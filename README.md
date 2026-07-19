# AvtoTest Platform

O'zbekiston YHQ nazariy imtihoniga tayyorlovchi onlayn maktab. Spec:
`docs/superpowers/specs/2026-07-17-avtotest-platform-master-design.md`.

## Dev boshlash

Talablar: Docker (compose bilan), Go 1.22+ (`~/.local/go`ga o'rnatilgan bo'lsa,
`export PATH=$HOME/.local/go/bin:$HOME/go/bin:$PATH`).

```bash
make up        # Postgres + Redis + MinIO (compose)
make seed      # [NAMUNA] sintetik namuna kontent (2 bilet, 40 savol, signlar katalogi)
make seed-real # haqiqiy avtoimtihon kontenti (61 bilet, 1235 savol) — pastdagi "Kontent"ga qarang
make run       # API :8080 (PORT env bilan o'zgartiriladi)
make check     # lint + testlar
```

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
- **Kategoriya**: manbada kategoriya maydoni yo'q, shuning uchun hamma savol
  bitta fallback kategoriya — `umumiy` ("Umumiy savollar") ostiga tushadi.
  Halol cheklov: kategoriya darajasidagi FSRS mastery breakdown (`/me/stats`)
  haqiqiy kategoriyalash qo'shilgunicha bitta bucketda ko'rinadi.
- **Izohlar**: 1219 ta savolda izoh bor (manba `comment` matnidan). `LegalRefs`
  bo'sh — manba matnida huquqiy iqtiboslar inline proza sifatida yozilgan,
  strukturaviy maydonlarga ajratib olinmagan (halol gap, soxta iqtibos emas).
- **Signlar katalogi bu importdan KELMAYDI** (manbada yo'l-belgi katalogi
  yo'q, faqat sahna-fotolar). Signlar API'si dev'da bo'sh qaytadi (yoki agar
  `make seed` alohida ishga tushirilgan bo'lsa, [NAMUNA] sign katalogi).
- Import Report'i: `categories=1 signs=0 images=715 · questions valid=1235
  quarantined=0 · variants stored=61 skipped=0` (`images` 715 — noyob
  fayllar soni; content-hash bo'yicha byte-bir xillari birlashib DB'da 682
  qatorga tushadi). 1 ta manba-rasm (`i120_9`) yo'q, u savol rasmsiz keladi.

Regeneratsiya + import (`backend/`dan):

```bash
go run ./cmd/convertavtoimtihon -src "/home/sher/Рабочий стол/aaa" -out seed/avtoimtihon
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

## Flutter frontend (M1 Plan 06 holati)

`app/` — Flutter web ilovasi (Dart paket nomi `avtotest_app`): loyiha
skeleti, tarmoq qatlami (Dio + 401-refresh interceptor), dark-default
Material 3 dizayn, i18n (uz-Latn/uz-Cyrl/ru), go_router (auth guard bilan),
va telefon+OTP orqali to'liq auth oqimi (profil olish + home shell bilan).
Arxitektura — feature-first Clean (`lib/app` router/theme/l10n, `lib/core`
tarmoq/natija turlari, `lib/features/*` data/domain/presentation, `lib/
shared/widgets`), holat boshqaruvi — Riverpod, modellar — freezed +
json_serializable (generatsiya qilingan `*.freezed.dart`/`*.g.dart` fayllar
committed — sqlc'nikiga o'xshash konventsiya).

**MUHIM: bu — faqat fundament.** Auth (telefon+OTP), profil va home shell
qurilgan; variantlar/imtihon/mashq/xatolar/izohlar/saqlanganlar/statistika
ekranlari **hali yo'q** — bular M1 Plan 07'ning ishi. Home shell'dagi
tegishli nav bo'limlari ataylab "tez orada" placeholder sifatida ko'rsatilgan
(soxta ekranlar emas).

### O'rnatish va ishga tushirish

Flutter shu muhitda `~/.local/flutter`ga o'rnatilgan (stable 3.44.6, web
qo'llab-quvvatlash yoqilgan, Chrome — `google-chrome-stable`). Har bir
shell'da:

```bash
export PATH="$HOME/.local/flutter/bin:$PATH"
export CHROME_EXECUTABLE=google-chrome-stable
```

```bash
cd app
flutter pub get
flutter run -d chrome --dart-define=API_BASE_URL=http://localhost:8090/api/v1
```

(`API_BASE_URL` berilmasa, `main.dart`dagi standart qiymat —
`http://localhost:8090/api/v1` — ishlatiladi, ya'ni yuqoridagi "Dev
boshlash"dagi `PORT=8090` konventsiyasiga mos.)

### Muhim muhit xususiyatlari (shu checkout uchun)

Ushbu repo yo'li kirill harflar va bo'sh joy o'z ichiga oladi
(`/home/sher/Рабочий стол/avtotest`), bu ikkita muhit xatosi/xususiyatiga
olib keladi — kimdir shu checkout ustida ishlasa, buni bilishi kerak:

- **`flutter analyze` o'rniga `dart analyze` ishlating.** `flutter analyze`
  bu yo'lda qulaydi (`FormatException: Unterminated string`) — sabab:
  analysis-server LSP handshake'ning `Content-Length` sarlavhasi UTF-16
  code-unit soni bo'yicha hisoblanadi, UTF-8 bayt uzunligi emas, shuning
  uchun workspace yo'lida kirill harflar bo'lsa sarlavha kam hisoblanib,
  JSON payload kesiladi. `dart analyze` xuddi shu analyzer dvigatelidan
  foydalanadi va bir xil signal beradi — shu sabab har doim shu
  ishlatiladi. CI'ga ta'sir qilmaydi (GitHub Actions checkout yo'llari faqat
  ASCII).
- **`flutter test` o'rniga `flutter test --concurrency=1` ishlating.** Bare
  `flutter test` (standart concurrency bilan) shu yo'lda birinchi fayldan
  keyin sukut bilan noto'g'ri ishlaydi (testlarni kesib qo'yadi yoki
  parallel workerlar orasida test nomlarini aralashtirib yuboradi) —
  `--concurrency=1` esa toza, to'g'ri natija beradi. CI'ga ta'sir qilmaydi.

```bash
cd app
dart analyze                      # 0 issues bo'lishi kerak
flutter test --concurrency=1      # to'liq unit+widget test suite
flutter build web                 # web build muvaffaqiyatli bo'lishi kerak
```

### Qo'llab-quvvatlanadigan tillar

`uz-Latn` (standart/fallback), `uz-Cyrl`, `ru` — backend bilan bir xil uchta
til (`kaa` hali yoqilmagan). Home shell'dagi til chiplari orqali
almashtiriladi, tanlov `shared_preferences` orqali saqlanadi.

### Live auth-flow tekshiruvi (M1 Plan 06 Task 9)

`app/integration_test/auth_flow_test.dart` — haqiqiy backend'ga qarshi
ishlaydigan, headless Chrome orqali haydaladigan avtomatik integratsiya
testi (telefon kiritish → OTP so'rash → dev debug-kodni o'qish → noto'g'ri
kod bilan xato ko'rish → to'g'ri kod bilan qayta urinish → HomeShell'da
haqiqiy profil ma'lumotlarini ko'rish → til/mavzu almashtirish → chiqish).
CI'ga ulanmagan (Plan 05 Task 10'ning bir martalik smoke-test odatiga mos) —
Plan 08'da to'liq E2E CI keladi. Ishga tushirish uchun testning boshidagi
izohga qarang (`chromedriver` alohida jarayon sifatida kerak bo'ladi —
`flutter test -d chrome` va `flutter drive -d chrome` ikkalasi ham web uchun
integration_test'ni ishlata olmaydi).
