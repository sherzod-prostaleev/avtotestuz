# SESSION HANDOFF — bu yerdan boshlang (yangilangan 2026-07-24, M2-10 va M2 to'liq tugagach)

> Yangi sessiya (yoki boshqa AI) uchun: bu hujjat **aniq holat + keyingi aniq qadam**ni beradi. Avval buni o'qing, keyin ishlang. Bu hujjat repo'ga committed — Claude Code'ning session-memory tizimidan farqli, har qanday AI/vosita buni o'qiy oladi.

## 0. Maqsad (kontekst)
AvtoTest — O'zbekiston YHQ imtihoniga tayyorlovchi **pullik onlayn maktab-startap** (onless.uz/osonprava.uz analogi, "10-15x kuchli"). Go backend + Next.js frontend. Manba-hujjat: repo ildizida `AVTOTEST-MASTER-PROMPT.txt`. To'liq roadmap: `docs/superpowers/2026-07-24-roadmap-m2-to-admin.md`.

## 1. Audit qilingan holat (2026-07-24, tekshirilgan)
- Git: `main`, barcha yangi imkoniyatlar (M2-06 backend referal, M2-10 to'lov tarixi va referal UI) committed. Ish daraxti **toza**.
- Backend: `go build ./...` OK; `go test ./... -p 1 -count=1` — barcha 28 paket **o'tadi** (billing, referral, payme, click, account, va boshqa hammasi).
- Frontend: `npm run typecheck` OK; `npm run test` — **243/243 test o'tadi** (52 fayl); lint'da faqat oldindan mavjud `<img>`→`<Image/>` ogohlantirishlari (bu sessiyaga aloqasi yo'q).
- DB migratsiya: **version 15**, dirty emas.
- Kontent: 1231 savol (3 til), 62 bilet, 285 belgi. Foydalanuvchi ma'lumoti pre-launch tozalangan.
- **`./run.sh`** repo ildizida — bitta buyruq bilan Docker infra + backend (:8090) + frontend (:3000)ni ishga tushiradi (Ctrl+C to'xtatadi, infra ishlab qoladi; `--stop-infra` bilan uni ham to'xtatadi).

## 2. Hozirgacha nima TUGADI

**M1 (backend+kontent+frontend asosiy) — TO'LIQ.**

**M2-01 (tarif modeli) — TUGADI, LIVE.** `GET /api/v1/tariffs`: Nexia(7 kun/24 900) / Gentra(30/59 900) / Malibu(75/109 900) + Matiz=bepul. 3 til, hisoblangan per-day/discount.

**M2-02 (Payme integratsiyasi, sandbox) — TO'LIQ TUGADI.** Spec: `docs/superpowers/specs/2026-07-24-m2-02-payme-design.md`. `internal/billing/payme/` — JSON-RPC 2.0 webhook (`POST /api/v1/billing/payme`, Basic-auth, doim HTTP 200), 6 metod (CheckPerform/Create/Perform+GrantDays/Cancel/Check/GetStatement). Money-critical fix (non-atomic write + concurrent double-grant) topilib **keyin** tuzatilgan (post-hoc) — migratsiya 12+13.

**M2-03 (Click integratsiyasi, sandbox) — TO'LIQ TUGADI.** Spec: `docs/superpowers/specs/2026-07-24-m2-03-click-design.md` (Click LLC'ning rasmiy GitHub kutubxonasidan tasdiqlangan protokol). `internal/billing/click/` — form-urlencoded webhook (`POST /api/v1/billing/click`, MD5 `sign_string` o'zi autentifikatsiya, faqat 2 metod: Prepare/Complete). M2-02'ning saboqlari **boshidanoq** qo'llanildi (tranzaksiya+row-lock birinchi commitdan bor) — whole-branch review birinchi urinishdayoq toza o'tdi, qo'shimcha fix kerak bo'lmadi. Migratsiya 14.

**M2-04 (to'lov tarixi, read-side) — TUGADI.** Spec: `docs/superpowers/specs/2026-07-24-m2-04-payment-history-design.md`. `GET /api/v1/me/payments?limit=N` (auth) — `internal/account` paketiga qo'shildi, yangi migratsiya yo'q. Barcha statusdagi to'lovlarni (paid/failed/canceled/...) tarif nomi bilan (locale-fallback) qaytaradi, `created_at DESC`, default limit 20.

**M2-05 (promo-kodlar, backend) — TUGADI.** Spec: `docs/superpowers/specs/2026-07-24-m2-05-promo-codes-design.md`. Plan: `docs/superpowers/plans/2026-07-24-m2-05-promo-codes.md`. `POST /api/v1/billing/promo/validate` (auth) — promo-kodni va tarif kodi bo'yicha chegirma (`percent`, `fixed`, `days`) va anti-fraud cheklovlarni (`active`, `valid_from/to`, `max_uses`, `per_user_limit`) tekshiradi. `POST /api/v1/me/checkout` `promo_code` parametrini qabul qiladi. Agar promo-kod bilan narx 0 so'm bo'lsa (100% chegirma yoki 0 so'mlik promo), checkout provayderga redirect qilmasdan to'lovni darhol `paid` qiladi, VIP entitlement grant etadi, va `promo_redemption` yozuvini yaratadi.

**M2-06 (referal dasturi, backend) — TUGADI.** Spec: `docs/superpowers/specs/2026-07-24-m2-06-referral-backend-design.md`. Plan: `docs/superpowers/plans/2026-07-24-m2-06-referral-backend.md`. `user_referral_code` va `referral` jadvallari (migratsiya 15). `GET /api/v1/me/referral` (referal kod yaratadi/oladi va taklif statistikasi: total_invited, total_rewarded, bonus_days_earned), `POST /api/v1/referral/apply` (referal kodni biriktiradi, self-referral va takroriy biriktirish anti-fraud cheklovlari bilan). Referal taklif qilgan foydalanuvchiga referee birinchi marta haqiqiy to'lov qilganda avtomatik +7 kunlik VIP entitlement beriladi va stat rewarded ga o'tkaziladi.

**M2-08 (tarif UI) — TUGADI.** Spec: `docs/superpowers/specs/2026-07-24-m2-08-tariff-ui-design.md`. `/premium` sahifasi statikdan qayta qurildi: Matiz (bepul, frontend-only static) + Nexia/Gentra/Malibu (`GET /tariffs`dan), kunlik-narx freyming, eski narx+tejash%, "Ommabop" badge highlight.

**M2-09 (checkout oqimi UI, frontend) — TUGADI.** Spec: `docs/superpowers/specs/2026-07-24-m2-09-checkout-ui-design.md`. Plan: `docs/superpowers/plans/2026-07-24-m2-09-checkout-ui.md`. `/premium` sahifasi kengaytirildi: `ProviderPicker` (Payme/Click brend tanlovi) va `PromoInput` (promo-kod tekshirish va dinamik narx prevyusi). Promo 100% chegirma bergan holatda (`free: true`) to'lov darhol bepul faollashtirilib `/checkout/success?free=true` sahifasiga o'tiladi. Natija sahifalari: `/checkout/success` (VIP nishon/mashqlarga o'tish), `/checkout/failure` (qayta urinish), `/checkout/pending` (automatik status polling).

**M2-10 (to'lov tarixi + referal UI, frontend) — TUGADI.** Spec: `docs/superpowers/specs/2026-07-24-m2-10-payment-history-referral-ui-design.md`. Plan: `docs/superpowers/plans/2026-07-24-m2-10-payment-history-referral-ui.md`. Profil sahifasiga (`/profile`) `ReferralCard` (referal havola va kodni bir marta bosishda nusxalash, takliflar va mukofotlar statistikasi, do'st kodingizni biriktirish) hamda `PaymentHistoryCard` (o'tgan to'lovlar jadvali: Payme/Click icon, status badge, so'mdagi narx va olingan VIP tarif) komponentlari qo'shildi va testlar bilan 100% verifikatsiya qilindi.

**M2-11 (mehmon-demo landing funnel, frontend) — TUGADI.** Spec: `docs/superpowers/specs/2026-07-24-m2-11-guest-demo-design.md`. Plan: `docs/superpowers/plans/2026-07-24-m2-11-guest-demo.md`. Landing sahifasida (`/`) `DemoQuestionBlock` kengaytirildi: mehmon foydalanuvchi demo savolni yechgach, unga to'g'ri/xato izohi, FSRS aqlli xatolar bankining afzalliklari va ro'yxatdan o'tish chaqiruvi (`/login`) ko'rsatiladi. Mehmon ro'yxatdan o'tmasdan "Yana bitta savol sinab ko'rish" tugmasi orqali bir nechta demo savolni ketma-ket yechib ko'rishi mumkin.

> **🎉 ROADMAPDA MONETIZATSIYA (M2) BO'LIMI TO'LIQ BITDI!** (Tariflar, Payme, Click, to'lov tarixi backend+UI, promo-kodlar, referal backend+UI, tarif UI, checkout oqimi va landing demo funnel bitdi. Merchant sandbox kalitlari qo'yilsa, to'liq to'lov tizimi live rejimda ishlaydi.)

## 3. Roadmap'dagi asl tavsiya vs. haqiqiy bajarilish tartibi

`docs/superpowers/2026-07-24-roadmap-m2-to-admin.md` bo'lim 9 (Wave jadvali) asl tavsiyasi: Wave 0 (M2-01) → Wave 1 (M2-02 Payme, M2-08 UI, M2-05 promo, M2-11 demo) → Wave 2 (M2-04 tarix, M2-09 checkout) → Wave 3 (M2-03 Click, M2-06 referal, M2-07 GRAND MOCK, M2-10 tarix UI).

**Bajarildi**: M2 bo'limidagi barcha rejalar (Wave 0, Wave 1, Wave 2, Wave 3 elementlari) **TO'LIQ BITDI**.

## 4. KEYINGI ANIQ QADAM (tavsiya)

Keyingi bosqich M4 (Growth / Retention):

1. **M4-01** (Growth & Engagement): Gamification / Streak va o'quv motivatsiyasi tizimi.

Tavsiya: **M4-01 (Gamification / Streak tizimi)** bilan davom ettirish.

Har biri: avval `superpowers:brainstorming` (agar spec hali yo'q bo'lsa) → `superpowers:writing-plans` → `superpowers:subagent-driven-development` bilan bajarish.

## 5. Operatsion faktlar (MUHIM — vaqt tejaydi)
- **Go PATH:** har `go`/`sqlc` buyrug'iga `export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"` prefiks (interaktiv bo'lmagan PATH'da yo'q).
- **sqlc generate:** `make generate` (repo ildizidan).
- **DB testlar:** `-p 1 -count=1` (bitta test-DB `avtotest_test`); `testdb.New(t)` migratsiya qo'llaydi + `Truncate` qiladi. Testlar o'z fixture'ini insert qiladi.
- **`pool.Exec` parametr bilan bir nechta SQL buyrug'ini QO me O'LLAMAYDI** (prepared statement) — parametrli insert'larni alohida `Exec`ga bo'ling.
- **Dev API restart:** `pkill -f "cmd/api"` KENG pattern shell'ni o'ldiradi (exit 144) — o'rniga `ss -ltnp | grep :8090` bilan aniq PID topib kill qiling; yangi binarni `run_in_background` bilan ishga tushiring.
- **Infra:** `docker compose` (postgres:5432, redis:6379, minio:9000) ishlab turibdi; backend compose'da EMAS.
- **Payme kalitlari:** ENV bo'sh, webhook -32504 qaytaradi (kutilgan).
- **Click kalitlari:** ENV bo'sh (`CLICK_SERVICE_ID`/`CLICK_MERCHANT_ID`/`CLICK_SECRET_KEY`), webhook -1 (SIGN CHECK FAILED) qaytaradi (kutilgan). Click webhook Payme'dan farqli — Basic-Auth emas, `sign_string` (MD5) o'zi autentifikatsiya; so'rov `application/x-www-form-urlencoded` (JSON fallback bilan), javob flat JSON (JSON-RPC emas).

## 6. Ish uslubi (skill'lar)
Har Plan: `brainstorming` (spec) → `writing-plans` (reja) → TDD implementatsiya (`subagent-driven-development`) → whole-branch review → push. Mustaqil Plan'lar (yoki bitta Plan ichidagi fayllar jihatidan mustaqil task'lar) parallel subagentlarga berish mumkin.

## 7. Keyingi Plan'lar to'liq ro'yxati (roadmap'dan)
M2-06 (BE, referal, 04'ga bog'liq) · M2-07 (BE+FE, GRAND MOCK) · M2-10 (FE, tarix/referal UI, 04+06'ga bog'liq). Har biri roadmapda (`docs/superpowers/2026-07-24-roadmap-m2-to-admin.md`, bo'lim 2 va 9) dekompozitsiya qilingan. M2 tugagach: M4 (Growth) → M5/M6/M7 → **M3 (Super Admin) ENG OXIRIDA**.
