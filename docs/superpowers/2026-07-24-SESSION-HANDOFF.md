# SESSION HANDOFF — bu yerdan boshlang (yangilangan 2026-07-24, audit + M2-07 GRAND MOCK bilan M2 haqiqatan tugagach)

> Yangi sessiya (yoki boshqa AI) uchun: bu hujjat **aniq holat + keyingi aniq qadam**ni beradi. Avval buni o'qing, keyin ishlang. Bu hujjat repo'ga committed — Claude Code'ning session-memory tizimidan farqli, har qanday AI/vosita buni o'qiy oladi.

## 0. Maqsad (kontekst)
AvtoTest — O'zbekiston YHQ imtihoniga tayyorlovchi **pullik onlayn maktab-startap** (onless.uz/osonprava.uz analogi, "10-15x kuchli"). Go backend + Next.js frontend. Manba-hujjat: repo ildizida `AVTOTEST-MASTER-PROMPT.txt`. To'liq roadmap: `docs/superpowers/2026-07-24-roadmap-m2-to-admin.md`.

## 1. Audit qilingan holat (2026-07-24, tekshirilgan)
- Git: `main`. Ish daraxti **toza**. `git log --oneline -1` bilan aniq HEAD'ni tekshiring (bu qatorda hash yozilmaydi — hujjatning o'zi committed bo'lgani uchun har doim bir commit orqada qolar edi).
- Backend: `go build ./...` OK; `go test ./... -p 1 -count=1` — barcha paketlar **o'tadi** (billing incl. promo/referral race-testlari, session incl. grand_mock, payme, click, account va boshqa hammasi).
- Frontend: `npm run typecheck` OK, `npm run lint` toza (faqat oldindan mavjud `<img>`→`<Image/>` ogohlantirishlari); `npm run test` — **249/249 test o'tadi** (53 fayl).
- DB migratsiya: **16 ta** (`0001`...`0016`), dirty emas.
- Kontent: 1231 savol (3 til), 62 bilet, 285 belgi. Foydalanuvchi ma'lumoti pre-launch tozalangan.
- **`./run.sh`** repo ildizida — bitta buyruq bilan Docker infra + backend (:8090) + frontend (:3000)ni ishga tushiradi.

### 1.1 MUHIM — bu sessiyada to'liq audit o'tkazildi va 3 ta Critical pul-xavfsizlik xatosi topilib TUZATILDI
Oldingi sessiya/AI M2-05/06/09/10/11'ni "tugadi" deb belgilagan va build/testlar yashil bo'lgan, lekin **testlar buglarni tutmagan** (ba'zilarida hatto noto'g'ri mock-shape ishlatilgan). To'rtta mustaqil chuqur code-review (parallel subagentlar) + qo'lda tekshiruv orqali quyidagilar topildi va **hammasi tuzatildi, testlar bilan tasdiqlandi**:

1. **Promo-kod orqali cheksiz bepul VIP** (`internal/billing/checkout.go` `StartCheckout`) — tranzaksiya/row-lock yo'q edi, reviewer 10 ta parallel so'rovda 10/10 muvaffaqiyatli exploit qilib ko'rsatdi ("bir martalik" promo 10 marta bepul VIP berdi). **Tuzatildi**: `pool.Begin`+`SELECT...FOR UPDATE` (promo_code qatori qulflanadi), migratsiya 0016 (`promo_redemption_one_per_payment` backstop unique index), 2 ta yangi concurrency-testi (`checkout_race_test.go`). Qoldiq (past xavfli, hujjatlashtirilgan): to'liq pullik (nol bo'lmagan) checkout uchun N ta parallel HAQIQIY to'lovchi hali ham limitni oshib ketishi mumkin — bu alohida, kamroq og'ir muammo, keyingi ishga qoldirilgan.
2. **Referal dasturi — concurrent to'lovlarda 2x mukofot** (`internal/billing/referral.go`) — avval `GrantDays` chaqirilib, keyin status-guard yangilanardi (teskari tartib). **Tuzatildi**: bitta atomik `UPDATE...WHERE status='pending' RETURNING` (`ClaimPendingReferralForReferee`) — claim-then-grant. Yangi concurrency test: bug qaytarib tasdiqlangan (8 ta parallel to'lovda avval 3x grant chiqqan), tuzatilgach 1x, `-race` bilan 10x tasdiqlangan.
3. **To'lov tarixi UI har doim bo'sh ko'rinardi** (`payment-history-card.tsx`) — backend `{"data": [...]}` qaytaradi, frontend `{"data": {"items": [...]}}` deb kutgan edi. Test ham noto'g'ri shape bilan mock qilingan bo'lib, yolg'on ishonch bergan. **Tuzatildi**: to'g'ri tur (`PaymentItem[]`), test to'g'ri shape bilan qayta yozildi. Shu bilan birga: referal havolasi endi backend'ning haqiqiy `invite_url`sidan foydalanadi, `/r/[code]` redirect-sahifa qo'shildi (avval umuman yo'q edi, havola bosilganda 404 berardi), referal xato-xabarlari endi lokalizatsiya qilingan (`err.code` orqali, `promo-input.tsx`dagi naqshga o'xshab), va login sahifasi `?ref=CODE`ni ushlab, OTP-tasdiqdan keyin avtomatik biriktiradi (to'liq referal-havola oqimi endi ishlaydi).

**M2-07 GRAND MOCK ham shu auditda topildi va to'g'ri qurildi** (pastga qarang, §2) — oldingi hujjat "M2 to'liq bitdi" deb yozgan edi, lekin M2-07 uchun faqat spec+plan bor edi, implementatsiya YO'Q edi. Bu safar to'g'ridan-to'g'ri qurildi.

## 2. Hozirgacha nima TUGADI

**M1 (backend+kontent+frontend asosiy) — TO'LIQ.**

**M2-01 (tarif modeli) — TUGADI, LIVE.** `GET /api/v1/tariffs`: Nexia(7 kun/24 900) / Gentra(30/59 900) / Malibu(75/109 900) + Matiz=bepul. 3 til, hisoblangan per-day/discount.

**M2-02 (Payme integratsiyasi, sandbox) — TO'LIQ TUGADI.** `internal/billing/payme/` — JSON-RPC 2.0 webhook, 6 metod, tranzaksiya+row-lock bilan xavfsiz.

**M2-03 (Click integratsiyasi, sandbox) — TO'LIQ TUGADI.** `internal/billing/click/` — form-urlencoded webhook, MD5 sign_string, tranzaksiya+row-lock bilan xavfsiz.

**M2-04 (to'lov tarixi, read-side) — TUGADI.** `GET /api/v1/me/payments?limit=N`.

**M2-05 (promo-kodlar, backend) — TUGADI + AUDIT'DA TUZATILDI.** `POST /api/v1/billing/promo/validate`, `POST /me/checkout`'ning `promo_code` parametri. Promo redemption endi `pool.Begin`+row-lock bilan xavfsiz (§1.1'ga qarang).

**M2-06 (referal dasturi, backend) — TUGADI + AUDIT'DA TUZATILDI.** `user_referral_code`/`referral` jadvallari (migratsiya 15). `GET /me/referral`, `POST /referral/apply`. Referee birinchi to'lov qilganda referrer +7 kun VIP oladi — endi claim-then-grant bilan xavfsiz (§1.1).

**M2-07 (GRAND MOCK — bosh imtihon simulyatsiyasi) — TUGADI (bu auditda birinchi marta qurildi).** Spec qayta yozildi: `docs/superpowers/specs/2026-07-24-m2-07-grand-mock-design.md`. **Muhim dizayn qarori**: bu YANGI subsystem emas — mavjud `internal/session` dvigatelining 5-rejimi sifatida qurilgan (`variant`/`exam`/`practice`/`mistakes` bilan bir qatorda), chunki DB CHECK constraint va `limit_config` (85% threshold) buni M1'dan beri kutib turgan edi. `StartSession`ning `switch req.Mode`iga `case "grand_mock":` qo'shildi (VIP + mastery≥85% eligibility, aks holda "exam" bilan bir xil: 20 savol/25 daqiqa/2 xato), va 6+1 joyda `row.Mode == "exam"` tekshiruvi yangi `IsExamLike()` helper orqali `grand_mock`ga ham tarqatildi (vaqt-tugashi, anti-cheat redaction, scoring). Yangi endpoint faqat bitta: `GET /me/mock-eligibility` (o'qish uchun) — start esa mavjud `POST /sessions {mode:"grand_mock"}`ning o'zi. Frontend: `GrandMockCard` (dashboard'da, qulflangan/ochiq holat), `GrandMockCertificateDialog` (confetti+sertifikat, `session/[id]` sahifasining mavjud exam-natija ekraniga qo'shimcha sifatida). Yangi migratsiya kerak bo'lmadi.

**M2-08 (tarif UI) — TUGADI.** `/premium` sahifasi `GET /tariffs`dan qayta qurilgan, haqiqiy "Sotib olish" tugmasi bilan.

**M2-09 (checkout oqimi UI, frontend) — TUGADI.** Provider picker (Payme/Click), promo-kod tekshirish, natija sahifalari (success/failure/pending + status polling). **Minor qoldiq**: backend `StartCheckout`ga hali `returnURL` uzatilmaydi (`billing/handlers.go`da hardcoded `""`) — demak Payme/Click haqiqiy to'lovdan keyin foydalanuvchini avtomatik qaytarmaydi, `/checkout/pending` sahifasi hozircha faqat qo'lda navigatsiya bilan yetib boriladi. Kichik, ixtiyoriy tuzatish — keyingi ishga qoldirildi.

**M2-10 (to'lov tarixi + referal UI, frontend) — TUGADI + AUDIT'DA TUZATILDI.** `ReferralCard` + `PaymentHistoryCard` (`/profile`). Barcha audit topilmalari (§1.1) tuzatildi: response-shape bug, referal havolasi, lokalizatsiya, `/r/[code]` route.

**M2-11 (mehmon-demo landing funnel, frontend) — TUGADI.** Landing'dagi `DemoQuestionBlock` — demo savol, izoh, ro'yxatdan o'tish chaqiruvi, "yana bitta savol" replay.

> **🎉 M2 (MONETIZATSIYA) BO'LIMI HAQIQATAN TO'LIQ BITDI** (audit + tuzatishlardan keyin, GRAND MOCK ham qo'shilib). Merchant sandbox kalitlari (Payme/Click) qo'yilsa, to'liq to'lov tizimi live rejimda ishlaydi.

## 3. Qoldiq — past-xavfli, kelajakka qoldirilgan (hujjatlashtirilgan, yashirilmagan)
- M2-09: checkout `returnURL` hali backend'dan uzatilmaydi (yuqoriga qarang).
- Promo: to'liq pullik (nol bo'lmagan summa) checkout'da N ta parallel HAQIQIY to'lovchi promo-limitni oshib ketishi mumkin (nol-summali holat tuzatildi, bu qolgan holat past xavfli — haqiqiy to'lov talab qiladi).
- Referal: retroaktiv biriktirishga qarshi himoya yo'q (eski foydalanuvchi ham istalgan vaqt do'st kodini biriktirib, keyingi to'lovini "referal" qilib ko'rsatishi mumkin) — anti-fraud kuchaytirilishi mumkin.
- `referral_attribution` jadvali (migratsiya 0003, fraud_flags bilan) ishlatilmay qolgan — M2-06 parallel yangi jadval qurgan. Tozalash yoki hujjatlashtirish kerak.
- M2-11: "keyingi savol" replay'i faqat 2 ta savol orasida almashadi (backend `demoQuestionCount=2`) — "ko'p-savolli" degan va'daga nisbatan zaifroq.

## 4. KEYINGI ANIQ QADAM (tavsiya)

Keyingi bosqich M4 (Growth / Retention):

1. **M4-01** (Growth & Engagement): Gamification / Streak va o'quv motivatsiyasi tizimi.

Tavsiya: **M4-01 (Gamification / Streak tizimi)** bilan davom ettirish.

Har biri: avval `superpowers:brainstorming` (agar spec hali yo'q bo'lsa) → `superpowers:writing-plans` → `superpowers:subagent-driven-development` bilan bajarish.

## 5. Operatsion faktlar (MUHIM — vaqt tejaydi)
- **Go PATH:** har `go`/`sqlc` buyrug'iga `export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"` prefiks.
- **sqlc generate:** `make generate` (repo ildizidan).
- **DB testlar:** `-p 1 -count=1` (bitta test-DB `avtotest_test`); `testdb.New(t)` migratsiya qo'llaydi + `Truncate` qiladi (endi process-wide shared pool, `sync.Once` orqali — haqiqiy concurrent-goroutine testlar yozish uchun qulay).
- **`pool.Exec` parametr bilan bir nechta SQL buyrug'ini QO'LLAMAYDI** (prepared statement) — parametrli insert'larni alohida `Exec`ga bo'ling.
- **Dev API restart:** `pkill -f "cmd/api"` ISHLATMANG — aniq PID (`ss -ltnp|grep :8090`) + `run_in_background`.
- **Infra:** `docker compose` (postgres:5432, redis:6379, minio:9000).
- **Payme/Click kalitlari:** ENV bo'sh — webhook kutilganidek rad etadi.
- **Money-critical kod naqshi** (endi barcha to'lov/entitlement yo'llarida qat'iy qo'llanilgan): `pool.Begin` + `SELECT...FOR UPDATE` (yoki `UPDATE...RETURNING` claim-pattern) + tx-bound `Service{Q: q}`. Yangi pul/entitlement kodi yozganda BOSHIDANOQ shu naqshni qo'llang — post-hoc tuzatish qimmatga tushadi (bu audit buni 3 marta isbotladi).

## 6. Ish uslubi (skill'lar)
Har Plan: `brainstorming` (spec) → `writing-plans` (reja) → TDD implementatsiya (`subagent-driven-development`) → whole-branch review → push. Mustaqil Plan'lar (yoki bitta Plan ichidagi fayllar jihatidan mustaqil task'lar) parallel subagentlarga (`Agent(isolation:"worktree", run_in_background:true)`) berish mumkin — integratsiya `git cherry-pick` yoki `git merge --ff-only` bilan. **Katta ishlarni "tugadi" deb belgilashdan oldin mustaqil chuqur audit/review o'tkazing** — build/test yashil bo'lishi yetarli emas, ayniqsa pul-bog'liq kodda (bu sessiya buni amalda isbotladi: 4/4 review Critical topdi).

## 7. Keyingi Plan'lar to'liq ro'yxati (roadmap'dan)
M2 tugadi. Keyingi: M4 (Growth) → M5/M6/M7 → **M3 (Super Admin) ENG OXIRIDA**. To'liq dekompozitsiya: `docs/superpowers/2026-07-24-roadmap-m2-to-admin.md`.
