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
