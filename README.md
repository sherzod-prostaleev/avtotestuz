# AvtoTest Platform

O'zbekiston YHQ nazariy imtihoniga tayyorlovchi onlayn maktab. Spec:
`docs/superpowers/specs/2026-07-17-avtotest-platform-master-design.md`.

## Dev boshlash

Talablar: Docker (compose bilan), Go 1.22+ (`~/.local/go`ga o'rnatilgan bo'lsa,
`export PATH=$HOME/.local/go/bin:$HOME/go/bin:$PATH`).

```bash
make up      # Postgres + Redis + MinIO (compose)
make seed    # [NAMUNA] sample kontentni import qiladi
make run     # API :8080 (PORT env bilan o'zgartiriladi)
make check   # lint + testlar
```

Sinash: `curl "localhost:8080/api/v1/variants/1?locale=uz-Latn"`

## Tuzilma

- `backend/` — Go API (chi + pgx/sqlc + golang-migrate)
- `docs/superpowers/specs|plans` — dizayn hujjatlari va rejalar
- Kontent importi: `backend/cmd/importer -data <dir> -verified`
  (canonical format: `data.json` + `images/`; barcha invariantlar tekshiriladi,
  buzilganlar quarantine bo'ladi — hech narsa taxmin qilinmaydi)

## API (M1 Plan 01 holati)

- `GET /healthz`
- `GET /api/v1/categories?locale=`
- `GET /api/v1/variants` · `GET /api/v1/variants/{n}?locale=`
- `GET /api/v1/signs?group=&q=&locale=` · `GET /api/v1/signs/{code}`
- `GET /api/v1/questions/{id}?locale=`

Javob konverti: `{"data":..., "meta":{...}}` yoki `{"error":{"code","message"}}`.
Kontent javoblarida to'g'ri javob maydonlari hech qachon qaytmaydi (anti-cheat).

## Auth (M1 Plan 02 holati)

Login telefon raqami + OTP kod orqali (Telegram Gateway; dev/sandboxda kod
javobning o'zida `debug_code` sifatida qaytadi). Sessiya — JWT access token
(15 daq) + rotatsiyalanadigan refresh token (30 kun, bir marta ishlatiladigan,
qayta ishlatilsa — profilning barcha sessiyalari bekor qilinadi).

- `POST /api/v1/auth/otp/request {phone}` → `{channel, debug_code?}`
  (cooldown 60s/telefon, 5/soat/telefon, 20/soat/IP)
- `POST /api/v1/auth/otp/verify {phone, code}` → `{access_token, refresh_token}`
  (birinchi muvaffaqiyatli kirishda profil avtomatik yaratiladi)
- `POST /api/v1/auth/refresh {refresh_token}` → yangi `{access_token, refresh_token}`
- `POST /api/v1/auth/logout {refresh_token}` → `{ok: true}`
- `GET /api/v1/me` (Bearer) → `{profile, vip:{active, until}}`
- `PATCH /api/v1/me` (Bearer, partial body) → yangilangan profil
  (`birth_date`: `"YYYY-MM-DD"` yoki `null`)
- `GET /api/v1/me/entitlement` (Bearer) → `{active, until}`

Xato kodlari: `invalid_phone`, `rate_limited` (429), `invalid_code`,
`expired_code`, `too_many_attempts`, `invalid_refresh`, `refresh_reused` (401),
`unauthorized` (401, Bearer yo'q/yaroqsiz).

Env o'zgaruvchilari (`.env` yoki shell): `JWT_SECRET` (dev default
`dev-secret-change-me`), `OTP_CHANNEL` (`sandbox`|`telegram`|`sms`, default
`sandbox`), `TELEGRAM_GATEWAY_TOKEN`, `TELEGRAM_GATEWAY_URL` (default
`https://gatewayapi.telegram.org`). `sms` kanali hali sozlanmagan (Plan 05+).

VIP grant (to'lovsiz, admin tomonidan): foydalanuvchi kamida bir marta OTP
orqali kirgan bo'lishi kerak (profil yaratilgan bo'lishi shart), so'ng:

```bash
cd backend && go run ./cmd/grantvip -phone 901112233 -days 30 -note "promo"
```

Grantlar stacking qilinadi — yangi grant `max(hozir, joriy tugash vaqti)`dan
boshlanadi (bir necha grant ketma-ket qo'shilsa, muddat cho'ziladi).

## Sessiya / test yechish (M1 Plan 03 holati)

Test yechish 4 rejimda ishlaydi (`POST /api/v1/sessions`ning `mode` maydoni):

- **`variant`** (bilet) — bitta biletning 20 ta savoli, tayinlangan tartibda
  (`variant_id` majburiy). Faqat ochilgan biletlar yechilishi mumkin.
- **`exam`** (imtihon) — 20 ta tasodifiy savol, 25 daqiqa vaqt chegarasi
  (`time_limit_sec: 1500`), haqiqiy imtihon qoidasi: 3-xatoda darhol to'xtaydi
  (`≤2 xato` — real YHQ imtihoni kabi). Javob fikr-mulohazasi (`correct`,
  `correct_answer_id`) sessiya tugagunicha yashiriladi (anti-cheat).
- **`practice`** (mashq) — `category_id` yoki `sign_id`dan biri (aynan bittasi)
  bo'yicha tasodifiy savollar, `count` bilan so'raladi. Kunlik limit bor
  (`daily_practice_questions` — free: 10/kun, VIP: cheklanmagan); limitdan
  oshsa `daily_limit_reached` xatosi qaytadi.
- **`mistakes`** (xatolar banki) — foydalanuvchi noto'g'ri javob bergan
  savollardan tuzilgan shaxsiy to'plam (`count`, default 10).

Grand Mock (to'liq simulyatsiya rejimi) M1'da yo'q — M2/VIP-gated funksiya
sifatida rejalashtirilgan.

### Endpointlar (Bearer talab qilinadi)

- `POST /api/v1/sessions {mode, variant_id?, category_id?, sign_id?, locale, count?}`
  → `{id, mode, question_ids[], time_limit_sec, total, started_at}`
- `POST /api/v1/sessions/{id}/answers {question_id, answer_id}`
  → `{recorded, correct?, correct_answer_id?, stopped?, stop_reason?}`
  (`correct`/`correct_answer_id` exam rejimida sessiya tugamaguncha
  qaytmaydi; `stopped:true, stop_reason:"too_many_errors"` — 3-xato bilan
  imtihon avtomatik to'xtaganda)
- `POST /api/v1/sessions/{id}/finish` → `{status, stopped_reason, score, total}`
  (`status`: `"passed"` | `"failed"` | `"abandoned"`;
  `stopped_reason`: `"completed"` | `"time_up"` | `"too_many_errors"`)
- `GET /api/v1/sessions/{id}` → `{id, mode, total, status, stopped_reason,
  score?, started_at, finished_at?, answers:[{question_id, position,
  answered, correct?}]}` (`correct` in-progress exam sessiyada har bir javob
  uchun yashirilgan bo'ladi)
- `GET /api/v1/me/sessions?limit=` → o'tgan/joriy sessiyalar ro'yxati
  (`[{id, mode, status, score?, total, started_at, finished_at?}]`)
- `GET /api/v1/me/variants` → har bir biletning qulf holati
  (`[{number, question_count, unlocked, best_correct, attempts,
  completed_at?}]`)

Xato kodlari: `invalid_request` (400), `daily_limit_reached` (429, faqat
`practice` rejimida), `not_found` (404), `already_answered` (409, savolga
sessiya ichida qayta javob berilsa), `invalid_answer` (400, `answer_id`
berilgan `question_id`ga tegishli emas), `session_finished` (409, tugagan
sessiyaga javob/finish yuborilsa).

### Bilet ochilish qoidasi

Birinchi bilet har doim ochiq. Keyingi har bir bilet oldingisi
`variant`-rejimida yechilib, `best_correct >= unlock_threshold_correct`
(`limit_config`da sozlanadi, default **10/20**, free va VIP uchun bir xil)
ga yetgandagina ochiladi.

### Xatolar banki (Leitner qoidasi)

Har qanday rejimda noto'g'ri javob berilgan savol avtomatik xatolar
bankiga tushadi. `mistakes`-rejimida savolga ketma-ket **2 marta** to'g'ri
javob berilsa (`MistakeClearAfter`), savol bankdan olib tashlanadi; bitta
xato ketma-ketlikni qaytadan boshlaydi.
