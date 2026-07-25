# AvtoTest Platform

O'zbekiston YHQ nazariy imtihoniga tayyorlovchi onlayn maktab. Spec:
`docs/superpowers/specs/2026-07-17-avtotest-platform-master-design.md`.

## Dev boshlash

Talablar: Docker (compose bilan), Go 1.26.5+ (`~/.local/go`ga o'rnatilgan bo'lsa,
`export PATH=$HOME/.local/go/bin:$HOME/go/bin:$PATH`).

```bash
make up        # Postgres + Redis + MinIO (compose)
make seed      # [NAMUNA] sintetik namuna kontent (2 bilet, 40 savol, signlar katalogi)
make seed-real # haqiqiy avtoimtihon kontenti (61 bilet, 1235 savol) — pastdagi "Kontent"ga qarang
make run       # API :8080 (PORT env bilan o'zgartiriladi)
make check     # lint + testlar
```

Sozlash: `backend/.env.example` — `config.Load()` o'qiydigan har bir env
o'zgaruvchisi default qiymati bilan. Devda hech biri majburiy emas (default'lar
`docker-compose.yml`ga mos); `ENV=staging|prod` esa bir nechta secret'ni
majburiy qiladi — faylning o'zida yozilgan.

### Testlar

```bash
make test          # -p 1 (bitta migratsiya/pool navbatda; kamroq resurs)
make test-parallel # to'liq parallel (~2.5x tezroq)
make test-db-reset # paketga xos test bazalarini o'chirish
make fe-check      # frontend lint + typecheck + vitest + build (CI frontend job)
make fe-e2e        # Playwright Chromium smoke (ixtiyoriy E2E_AUTH_TOKEN)
```

Ikkalasi ham **bir xil** natija beradi: `internal/testdb` har test paketiga
alohida Postgres bazasi, `internal/redisx` esa alohida Redis DB'sini beradi,
shuning uchun paketlar bir-birining ma'lumotini o'chirmaydi. `-p 1` to'g'rilik
sharti emas. Migratsiyani **joyida** o'zgartirsangiz `make test-db-reset`
qiling — hosila bazalar runlar orasida qayta ishlatiladi.

Sinash: `curl "localhost:8080/api/v1/variants/1?locale=uz-Latn"`

## Tuzilma

- `backend/` — Go API (chi + pgx/sqlc + golang-migrate)
- `docs/superpowers/specs|plans` — dizayn hujjatlari va rejalar
- Kontent importi: `backend/cmd/importer -data <dir> -verified`
  (canonical format: `data.json` + `images/`; barcha invariantlar tekshiriladi,
  buzilganlar quarantine bo'ladi — hech narsa taxmin qilinmaydi)

## Kontent (ma'lumotlar)

Ikki xil kontent to'plami mavjud. Importer **upsert** qiladi (jadvallarni
o'zi tozalamaydi) — ikkalasi ham bilet raqamlari 1 va 2'ni ishlatgani uchun
bir DB'da aralashtirilsa 1/2-biletlar bir-birini qoplaydi va yetim savollar
qoladi; shu sababli dev DB'da **bittasini** toza holda ishlating.

### Haqiqiy kontent — `avtoimtihon` (asosiy, `make seed-real`)

Foydalanuvchining o'ziga tegishli, litsenziyalangan haqiqiy imtihon kontenti
(spec D5; jonli scrape emas). Manba: `aaa/src/data/questions.{uz-Latn,uz-Cyrl,ru}.json`
+ `aaa/public/quiz-images/*.webp`.

- **1235 ta savol**, 3 tilda (uz-Latn / uz-Cyrl / ru), har biri **verified**.
- Javob soni har xil: 2/3/4/5 (haqiqiy manba qanday bo'lsa shunday — 4'ga
  majburlanmaydi). Shulardan 25 tasi to'g'ri javobi 5-o'rinda bo'lgan
  5-javobli savol (DB CHECK `position 1..5` — migration 0006 bilan kengaytirilgan).
- **61 ta bilet** (variant), har biri aynan 20 savol — ikki ketma-ket
  manba-tiketni (10+10) juftlab hosil qilingan (rasmiy imtihon 20 savol
  formatiga mos). Bu ataylab qilingan qaror.
- **15 ta savol hech qaysi biletga tayinlanmagan** (toq tiket qoldig'i:
  `avtoimtihon-1221..1235`) — ular baribir yaroqli, mustaqil savol sifatida
  mavjud (mashq / xatolar banki / FSRS ularda ishlaydi), faqat raqamli
  biletga kirmaydi.
- **Kategoriya**: manbada kategoriya maydoni yo'q edi, shuning uchun barcha
  1235 ta savol **13 ta tasdiqlangan kategoriya**ga taqsimlangan (izoh
  matnidagi PDD bob/ilova iqtiboslariga qarab qoidaga asoslangan klassifikator
  + iqtibossiz 390 ta savol uchun committed `assignments.json` override —
  `docs/superpowers/research/2026-07-21-category-taxonomy-proposal.md`):
  `road_signs_markings` (334), `priority_intersections` (138),
  `maneuvering_lane_position` (105), `accidents_first_aid_dynamics` (102),
  `vehicle_equipment_lighting` (94), `stopping_parking` (80),
  `pedestrians_public_transport` (67), `general_provisions_admin` (65),
  `overtaking_speed` (62), `traffic_signals_gestures` (56),
  `special_road_zones` (52), `towing_special_vehicles` (41),
  `cargo_passenger_carriage` (39). Fallback `umumiy` kategoriyasi endi
  ishlatilmaydi — regeneratsiya `-strict` bilan ishga tushadi va har bir
  savolga kategoriya biriktirilmasa muvaffaqiyatsiz tugaydi (nol fallback).
- **Izohlar**: 1219 ta savolda izoh bor (manba `comment` matnidan). `LegalRefs`
  bo'sh — manba matnida huquqiy iqtiboslar inline proza sifatida yozilgan,
  strukturaviy maydonlarga ajratib olinmagan (halol gap, soxta iqtibos emas).
- **Signlar katalogi bu importdan KELMAYDI** (manbada yo'l-belgi katalogi
  yo'q, faqat sahna-fotolar). Signlar API'si dev'da bo'sh qaytadi (yoki agar
  `make seed` alohida ishga tushirilgan bo'lsa, [NAMUNA] sign katalogi).
- Import Report'i: `categories=13 signs=0 images=715 · questions valid=1235
  quarantined=0 · variants stored=61 skipped=0` (`images` 715 — noyob
  fayllar soni; content-hash bo'yicha byte-bir xillari birlashib DB'da 682
  qatorga tushadi). 1 ta manba-rasm (`i120_9`) yo'q, u savol rasmsiz keladi.

Regeneratsiya + import (`backend/`dan):

```bash
go run ./cmd/convertavtoimtihon -src "/home/sher/Рабочий стол/aaa" -out seed/avtoimtihon \
  -assignments seed/avtoimtihon/assignments.json -strict
go run ./cmd/importer -data seed/avtoimtihon -verified
# yoki repo ildizidan: make seed-real
```

Toza real-only dev DB uchun avval jadvallarni tozalab oling (bu ish
qilinganida shunday qilingan) — aks holda mavjud [NAMUNA] fixture bilan
aralashadi.

### Sintetik namuna — `[NAMUNA]` fixture (`make seed`)

Backend test suite'lari va tez smoke uchun sun'iy namuna: 2 bilet, 40 savol,
4 kategoriya (`priority`/`rules`/`safety`/`signs`) va **4 signli katalog**
(2 guruh). `cmd/genfixture` generatsiya qiladi. Signlar API'sini dev'da
sinash uchun yagona ma'lumot manbai (haqiqiy import signlar bermaydi).

Ikkala `seed/` katalogi ham (`seed/sample/`, `seed/avtoimtihon/`)
gitignore'da — kod (converter + importer) va yuqoridagi regeneratsiya
buyruqlari commit qilinadi, 700+ rasm blob'i emas.

## API (M1 Plan 01 holati)

- `GET /healthz` — liveness (process up; dependency ping yo'q)
- `GET /readyz` — readiness (`checks.postgres` / `checks.redis`; wired bo'lsa ping, aks holda `skipped`; fail → 503)
- `GET /metrics` — process-local counters; **Prometheus text** by default, JSON via `Accept: application/json` or `?format=json`; optional Sentry SDK when `SENTRY_DSN` / `NEXT_PUBLIC_SENTRY_DSN` set (empty = no-op; no pager); probes excluded from counts
- **U-42 load-test smoke:** `make load-test` (k6; `deploy/load-test/`) — lokal/staging API smoke, **not** prod soak
- **U-44 backup/DR drill:** `make backup-pg` / `make backup-restore-drill` (`scripts/backup/`) — local compose only; RPO/RTO placeholders in runbook
- **Admin OpenAPI stub:** `docs/openapi/admin-v1.stub.yaml` (route catalog for `/admin/v1`, no schemas)
- `GET /api/v1/categories?locale=`
- `GET /api/v1/variants` · `GET /api/v1/variants/{n}?locale=`
- `GET /api/v1/signs?group=&q=&locale=` · `GET /api/v1/signs/{code}`
- `GET /api/v1/questions/{id}?locale=`

Javob konverti: `{"data":..., "meta":{...}}` yoki `{"error":{"code","message"}}`.
Kontent javoblarida to'g'ri javob maydonlari hech qachon qaytmaydi (anti-cheat).

Ops (ixtiyoriy, `OPS_ADMIN_TOKEN` berilganda): `GET|PATCH /api/v1/ops/payment-providers`
(+ `X-Ops-Token`) — to'lov provayderlari kill-switch. FE: `/{locale}/ops/providers`.
Tizim holati stub (token shart emas): FE `/{locale}/ops/health` → BFF `GET /api/ops/health`
(`/{healthz,readyz,metrics}` agregatsiya).

## Demo (public, auth talab qilinmaydi)

Landing sahifa uchun ro'yxatdan o'tmasdan javob berish mumkin bo'lgan
YAGONA haqiqiy savol yuzasi (rejalashtirilgan yagona qo'shimcha backend
yuzasi — boshqa hech narsa public/grading emas). To'liq stateless — hech
qanday user/learning jadvaliga (sessiya, FSRS, streak, events) yozuv
qilinmaydi, Redis rate-limit hisoblagichidan tashqari.

- `GET /api/v1/demo/question?locale=` → `GET /questions/{id}` bilan AYNAN
  bir xil DTO shakli (`content` paketining rendering yo'lidan qayta
  foydalaniladi — yangi shakl yo'q, to'g'ri javob maydonlari yo'q).
  Whitelist — 1-bilet (bepul bilet)ning **birinchi 5 ta savoli** (pozitsiya
  bo'yicha, kod konstantasi `demoQuestionCount = 5`, konfiguratsiya jadvali
  yo'q). Har so'rovda whitelist'dan tasodifiy bittasi qaytadi. 1-bilet
  mavjud bo'lmasa yoki savolsiz bo'lsa (bo'sh DB) — `not_found` (404),
  hech narsa o'ylab topilmaydi.
- `POST /api/v1/demo/answer {question_id, answer_id}` → `{correct,
  correct_answer_id}`. `question_id` whitelist'dan tashqarida bo'lsa —
  `not_found` (404, bu endpoint hech qachon butun savol banki uchun
  "to'g'ri javob oracle"iga aylanmaydi). `answer_id` `question_id`ga
  tegishli bo'lmasa — `invalid_answer` (400, sessiya paketidagi xuddi shu
  qoida/kod). IP bo'yicha rate limit — **60 so'rov/soat/IP** (auth/OTP
  paketidagi Redis pattern'i bilan bir xil); oshsa — `rate_limited` (429).

Xato kodlari: `not_found`, `invalid_answer`, `rate_limited` (429),
`invalid_request` (400, `question_id`/`answer_id` UUID emas). Shuningdek
content/session paketlaridagi umumiy kodlar ham qaytishi mumkin:
`invalid_locale` (400, GET'da), `invalid_body` (400, POST'da JSON buzilgan
bo'lsa).

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
`dev-secret-change-me`), `OTP_CHANNEL` (`sandbox`|`telegram`, default
`sandbox`), `TELEGRAM_GATEWAY_TOKEN`, `TELEGRAM_GATEWAY_URL` (default
`https://gatewayapi.telegram.org`) va Next.js BFF bilan umumiy, tasodifiy 32+
baytli `CLIENT_IP_ASSERTION_SECRET`. Devda oxirgi qiymat bo'sh qolishi mumkin:
backend imzosiz/spoof qilingan client-IP headerlarini e'tiborsiz qoldirib,
ulanish IP'ini xavfsiz fallback sifatida ishlatadi. `ENV=staging|prod`da secret
majburiy; frontendda ayni secret bilan `TRUSTED_PROXY_HOPS` ham sozlanadi.
Referal havolalari `PUBLIC_BASE_URL` (frontend origin'i, default
`http://localhost:3000`) ustiga quriladi. To'liq ro'yxat: `backend/.env.example`.

VIP grant (to'lovsiz, admin tomonidan): foydalanuvchi kamida bir marta OTP
orqali kirgan bo'lishi kerak (profil yaratilgan bo'lishi shart), so'ng:

```bash
cd backend && go run ./cmd/grantvip -phone 901112233 -days 30 -note "promo"
```

Grantlar stacking qilinadi — yangi grant `max(hozir, joriy tugash vaqti)`dan
boshlanadi (bir necha grant ketma-ket qo'shilsa, muddat cho'ziladi). Stacking
profil qatorini `FOR UPDATE` bilan qulflab hisoblanadi, shuning uchun `GrantDays`
har doim tranzaksiya ichida chaqirilishi kerak — aks holda qulf darhol bo'shab,
bir vaqtda tugagan ikki to'lov bir-birining kunini bosib ketadi.

## Sessiya / test yechish (M1 Plan 03 holati)

Test yechish 5 rejimda ishlaydi (`POST /api/v1/sessions`ning `mode` maydoni):

- **`variant`** (bilet) — bitta biletning 20 ta savoli, tayinlangan tartibda
  (`variant_id` majburiy). Faqat ochilgan biletlar yechilishi mumkin.
  `variant_id` — UUID YOKI `GET /variants`ning `number`i (masalan `"1"`)
  bo'lishi mumkin — content API biletlarga hech qachon UUID qaytarmaydi,
  faqat `number`; shuning uchun backend server tarafida UUID'ga o'zi
  rezolyutsiya qiladi (`not_found` — raqam topilmasa).
- **`exam`** (imtihon) — 20 ta tasodifiy savol, 25 daqiqa vaqt chegarasi
  (`time_limit_sec: 1500`), haqiqiy imtihon qoidasi: 3-xatoda darhol to'xtaydi
  (`≤2 xato` — real YHQ imtihoni kabi). Javob fikr-mulohazasi (`correct`,
  `correct_answer_id`) sessiya tugagunicha yashiriladi (anti-cheat).
- **`practice`** (mashq) — `category_id` yoki `sign_id`dan biri (aynan bittasi)
  bo'yicha tasodifiy savollar, `count` bilan so'raladi. Kunlik limit bor
  (`daily_practice_questions` — free: 10/kun, VIP: cheklanmagan); limitdan
  oshsa `daily_limit_reached` xatosi qaytadi. `category_id`/`sign_id` — UUID
  YOKI `GET /categories`/`GET /signs`ning `code`i (masalan `"signs"`,
  `"3.27"`) bo'lishi mumkin — content API kategoriya/belgilarga hech qachon
  UUID qaytarmaydi, shuning uchun backend kodni server tarafida UUID'ga
  o'zi rezolyutsiya qiladi (`not_found` — kod topilmasa).
- **`mistakes`** (xatolar banki) — foydalanuvchi noto'g'ri javob bergan
  savollardan tuzilgan shaxsiy to'plam (`count`, default 10).

- **`grand_mock`** (bosh imtihon simulyatsiyasi) — `exam` bilan bir xil qoidalar
  (20 savol / 25 daqiqa / ≤2 xato), lekin uchta shart bajarilganda ochiladi:
  VIP obuna, savol bankining kamida `grand_mock_min_studied_pct` (25%) qismi
  o'rganilgan, va mastery ≥ `grand_mock_threshold_pct` (85%). Hajm sharti
  mastery'ni o'ynashga qarshi: u bo'lmasa har kategoriyadan bittagina savolga
  to'g'ri javob berib "100% tayyor" ko'rinish mumkin edi. Ikkala chegara ham
  `limit_config`dan o'qiladi (kod konstantalari faqat zaxira).
  `GET /api/v1/me/mock-eligibility` — joriy holat va qancha qolganini qaytaradi;
  bloklovchi sabab VIP bo'lsa start `402 vip_required` (upsell), o'qish
  talablari bo'lsa `403 mock_not_eligible` beradi.

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

### Xatolar banki

Har qanday rejimda berilgan har bir javob (to'g'ri yoki noto'g'ri) FSRS
xotira jadvaliga (`question_memory`) yoziladi. `mistakes`-rejim endi fixed
"ketma-ket 2 marta to'g'ri" qoidasi (Leitner, M1 Plan 03) o'rniga **haqiqiy
FSRS rejalashtirishga** asoslanadi: bank — `lapses > 0 AND due_at <= now()`
bo'lgan savollar to'plami (pastdagi "FSRS o'quv dvigateli" bo'limiga
qarang). Savol bankdan FSRS `due_at` kelajakka siljigandan keyin (odatda
keyingi muvaffaqiyatli review'dan so'ng) chiqadi — endi "ketma-ket 2 marta"
degan qat'iy son yo'q, interval FSRS formulasi bilan hisoblanadi.

## FSRS o'quv dvigateli (M1 Plan 04 holati)

Har bir sessiya javobi (barcha 4 rejimda — `variant`, `exam`, `practice`,
`mistakes`) FSRS-4.5 (Free Spaced Repetition Scheduler) algoritmiga
uzatiladi: har bir savol uchun `stability` (xotirada necha kun turishi) va
`difficulty` (savolning qiyinligi, 1–10) saqlanadi, va shu ikkisidan
`due_at` — savol qachon **90% maqsadli eslab qolish ehtimoli**
(`DefaultDesiredRetention = 0.9`)ga tushishi hisoblab chiqiladi. To'g'ri
javob stability'ni oshiradi (interval uzayadi); noto'g'ri javob (`Again`)
"lapse" hisoblanadi — stability qayta tiklanadi va savol tez orada qayta
ko'rsatiladi.

### Endpointlar (Bearer talab qilinadi)

- `GET /api/v1/learn/next` → `[question_id, ...]` — hozir due bo'lgan
  savollar ro'yxati (`due_at <= now()`), kategoriyalar bo'yicha round-robin
  interleaving qilingan holda: `due_at` bo'yicha ASC tartiblanadi va har
  navbatda eng shoshilinch (eng ko'p kechikkan) savolga ega kategoriya
  birinchi chiqadi. Bu alohida "zaif kategoriyaga ustuvorlik" mexanizmi
  emas — sof `due_at` shoshilinchligiga asoslangan; amalda zaif
  kategoriyalar ko'proq due savol to'plagani uchun tabiiy ravishda ko'proq
  chiqadi, lekin bu aniq og'irlik bosqichi sifatida amalga oshirilmagan.
- `POST /api/v1/learn/review {question_id, rating}` → `{stability,
  difficulty, due_at, reps, lapses}` — savolni sessiyadan tashqarida qo'lda
  baholash (`rating`: `1`=Again, `2`=Hard, `3`=Good, `4`=Easy). Yaroqsiz
  `rating` — `invalid_rating` (400).
- `GET /api/v1/me/stats` → `{categories:[{category_code, mastery, seen,
  correct}], readiness_pct, due_count}` — har bir kontent-kategoriya
  bo'yicha mastery (0–1) va umumiy imtihonga tayyorlik foizi.

Tayyorlik foizi (`readiness_pct`) — kategoriyalar bo'yicha og'irliklangan
o'rtacha mastery (og'irlik — butun savollar bankidagi shu kategoriyaga
tegishli barcha yaroqli (valid) savollar soni, muayyan imtihon
bileti/variantining tarkibi emas), 0–100 oralig'ida butun songa
yaxlitlangan.

Eslatma: savol yangi ko'rilganda ham `due_at` kamida 1 kun keyinga
rejalashtiriladi (FSRS interval formulasi natijasi hech qachon 1 kundan
kam bo'lmaydi) — shuning uchun bitta sessiyadan keyin darhol `learn/next`
va `mistakes` banki odatda bo'sh (yoki deyarli bo'sh) qaytadi; bu kutilgan
holat, xato emas.

## Izohlar: AI-qoralama → tekshiruv → fikr-mulohaza (M1 Plan 05 holati)

**MUHIM: AI-qoralama hozircha shablon (stub), haqiqiy LLM integratsiyasi
EMAS.** `gendraft` savol matni/kategoriyasi/to'g'ri javobidan andozaviy
matn yig'adi (`explanation.TemplateDraftGenerator`) — natijadagi har bir
blok `[AI-QORALAMA]` prefiksi bilan belgilanadi, masalan:
`"[AI-QORALAMA] MUHIM: to'g'ri javob — ..."`. Haqiqiy LLM chaqiruvi M1'da
yo'q; ekspert tekshiruvidan o'tmagan (`status != "verified"`) izoh
foydalanuvchiga hech qachon ko'rsatilmaydi.

Oqim: admin `gendraft` bilan qoralama yaratadi (`status: "draft"`) → ekspert
`verifyexplanation` bilan tasdiqlaydi (`status: "verified"`, `verified_by`,
`verified_at` to'ldiriladi) → shundan keyingina izoh `GET
/api/v1/questions/{id}` javobidagi `explanation` maydonida ko'rinadi
(`GetVerifiedExplanation` — faqat verified holatdagilar qaytadi).
Tasdiqlanmagan savolda `explanation: null` bo'ladi.

CLI (admin-only, HTTP endpoint yo'q):

```bash
cd backend && go run ./cmd/gendraft -question <savol-uuid>
# → "draft created for question <uuid>"

cd backend && go run ./cmd/verifyexplanation -question <savol-uuid> -by <profil-uuid>
# → "verified explanation for question <uuid>"
# (draft mavjud bo'lmasa: "error: no draft exists — run gendraft first")
```

### Endpointlar (Bearer talab qilinadi)

- `POST /api/v1/explanations/feedback {question_id, helpful}` →
  `{ok: true}` — izoh foydali bo'lganmi degan fikr-mulohaza (faqat
  izoh mavjud bo'lsa; aks holda `not_found`).

Xato kodlari: `not_found` (404, savol uchun hali izoh yaratilmagan).

## Saqlangan savollar (M1 Plan 05 holati)

Foydalanuvchi istalgan savolni keyinroq qaytib ko'rish uchun
"saqlanganlar" ro'yxatiga qo'sha oladi (rejim/sessiyadan mustaqil).

### Endpointlar (Bearer talab qilinadi)

- `POST /api/v1/me/saved {question_id}` → `{ok: true}`
- `GET /api/v1/me/saved` → `[{question_id, created_at}]`
- `DELETE /api/v1/me/saved/{question_id}` → `{ok: true}`

## Kunlik streak (M1 Plan 05 holati)

Har bir sessiya javobi (rejimdan qat'i nazar) kunlik streak'ni yangilaydi.
Kun chegarasi **UTC** bo'yicha hisoblanadi (`todayUTC()`), lokal vaqt
zonasi emas — shuning uchun kun almashinuvi atrofida (masalan
`UTC+5`da soat 05:00 gacha) `last_active_date` kutilganidan bir kun orqada
ko'rinishi mumkin, bu — belgilangan xulq-atvor, xato emas.

- `GET /api/v1/me/streak` → `{current, best, today_done, daily_goal,
  last_active_date}`
  - `current` — uzluksiz faol kunlar soni (kecha yoki bugun javob
    berilmasa `0`ga tushadi)
  - `best` — eng uzun streak rekordi
  - `today_done` — bugun (UTC) javob berilgan savollar soni
  - `daily_goal` — kunlik maqsad (hozircha fixed qiymat; maqsadni
    tahrirlash UI'i M3'da rejalashtirilgan)
  - `last_active_date` — oxirgi faol kun (`"YYYY-MM-DD"`) yoki `null`
    (hali hech qachon javob berilmagan)

## Free-tier / VIP chegarasi (M1 Plan 05 holati)

Bepul (entitlement talab qilinmaydi):

- `variant` rejimi, **faqat 1-bilet** (`number == 1`)
- `practice` rejimi (kunlik limit bilan — yuqoridagi "Sessiya" bo'limiga
  qarang)

VIP talab qilinadi (`entitlement` faol bo'lishi kerak):

- `variant` rejimi, **2-bilet va undan keyingilari**
- `exam` rejimi
- `mistakes` rejimi

Faol entitlement bo'lmasa, `POST /api/v1/sessions` xatosi: `402
{"error":{"code":"vip_required","message":"active entitlement required"}}`.
VIP berish — yuqoridagi "Auth" bo'limidagi `grantvip` CLI orqali.

## Voqealar (events) logi (M1 Plan 05 holati)

Klient tomonidan batch holida analitika voqealari yuboriladi (masalan,
ekran ko'rilishi, tugma bosilishi).

- `POST /api/v1/events {events:[{name, props?, ts?}, ...]}` →
  `{ok: true, count: <n>}`
  - Bir so'rovda **1–100 ta** voqea bo'lishi kerak; bo'sh yoki 100tadan
    ortiq bo'lsa — `invalid_request` (400).
  - `props` — ixtiyoriy erkin JSON obyekt; `ts` — ixtiyoriy (berilmasa
    server vaqti — `now()` — ishlatiladi).

Xato kodlari: `invalid_request` (400, bo'sh yoki 100tadan ortiq batch).

## Frontend

**Stack-pivot (2026-07-21):** avvalgi Flutter web ilovasi (`app/`)
foydalanuvchi qarori bilan bekor qilindi va repodan olib tashlandi (to'liq
tarixi git'da — `git log -- app/`). Yangi frontend **Next.js + TypeScript +
Tailwind CSS + shadcn/ui** bilan quriladi va shu Go backend'ga ulanadi.
To'liq talablar, sahifama-sahifa dizayn spetsifikatsiyasi va o'tmish
saboqlari: repo ildizidagi `AVTOTEST-MASTER-PROMPT.txt`.

**Phase A (2026-07-22):** skelet + dizayn-tizim + 3 statik mockup sahifa
tayyor (`frontend/`, real API'ga ulanmagan). Ishga tushirish:
`cd frontend && npm install && npm run dev`. Tafsilot:
`docs/superpowers/specs/2026-07-22-nextjs-frontend-foundation-design.md`
va `frontend/README.md`.

**Phase B1 (2026-07-22):** real phone+OTP login, httpOnly-cookie sessions,
single-flight-refresh BFF proxy (`/api/proxy/[...path]`), and full next-intl
locale routing (uz-Latn/uz-Cyrl/ru) landed. Dashboard/exam-mockup content is
still mock data post-login — Phase B2 wires the real backend. Details:
`docs/superpowers/specs/2026-07-22-nextjs-frontend-phase-b1-auth-i18n-design.md`.
