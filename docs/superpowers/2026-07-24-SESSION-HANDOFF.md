# SESSION HANDOFF — bu yerdan boshlang (yangilangan 2026-07-25, ikkinchi audit raundi tugagach)

> Yangi sessiya (yoki boshqa AI) uchun: bu hujjat **aniq holat + keyingi aniq qadam**ni beradi. Avval buni o'qing, keyin ishlang. Bu hujjat repo'ga committed — Claude Code'ning session-memory tizimidan farqli, har qanday AI/vosita buni o'qiy oladi.

## 0. Maqsad (kontekst)
AvtoTest — O'zbekiston YHQ imtihoniga tayyorlovchi **pullik onlayn maktab-startap** (onless.uz/osonprava.uz analogi, "10-15x kuchli"). Go backend + Next.js frontend. Manba-hujjat: repo ildizida `AVTOTEST-MASTER-PROMPT.txt`. To'liq roadmap: `docs/superpowers/2026-07-24-roadmap-m2-to-admin.md`.

## 1. Audit qilingan holat (2026-07-25, tekshirilgan)
- Git: `main`. `git log --oneline -1` bilan aniq HEAD'ni tekshiring (bu qatorda hash yozilmaydi — hujjatning o'zi committed bo'lgani uchun har doim bir commit orqada qolar edi).
- Backend: `go build ./...` OK; `go vet ./...` toza; `gofmt -l .` toza; `make test` (`-p 1`) **hammasi o'tadi**; `make test-parallel` (izolyatsiya tufayli endi mumkin) ham **3/3 marta ketma-ket yashil**, 117s o'rniga ~44s.
- Frontend: `npm run typecheck` OK, `npm run lint` toza (faqat oldindan mavjud `<img>`→`<Image/>` ogohlantirishlari); `npm run test` — **266/266 test o'tadi** (54 fayl).
- `make generate` idempotent — sqlc kodi `.sql` fayllar bilan mos, drift yo'q.
- DB migratsiya: **19 ta** (`0001`...`0019`), dirty emas.
- Kontent: 1231 savol (3 til), 62 bilet, 285 belgi. Foydalanuvchi ma'lumoti pre-launch tozalangan.
- **`./run.sh`** repo ildizida — bitta buyruq bilan Docker infra + backend (:8090) + frontend (:3000)ni ishga tushiradi.

### 1.-1 IKKINCHI AUDIT RAUNDI (2026-07-25, kech) — 7 ta topilma tuzatildi, shundan 3 tasi pul-xavfsizligiga tegishli
Birinchi audit raundi (§1.1) o'z topilmalarining bir qismini "past-xavfli, keyinga" deb qoldirgan edi. Bu raund shu qoldiqni yopdi va **birinchi raund umuman ko'rmagan** yangi muammolarni topdi. Har biri test bilan qulflangan.

**Critical #1 — promo-limitni ketma-ket (concurrency'siz!) chetlab o'tish.** Birinchi raund `StartCheckout`dagi `FOR UPDATE` qulfini "limit himoyasi" deb hisoblagan va faqat parallel holatni xavf deb yozgan edi. Haqiqat qo'rqinchliroq: **`promo_redemption` qatori faqat to'lov tugaganda yoziladi**, `created` holatidagi to'lov esa hech qachon eskirmaydi — demak `max_uses=1, per_user_limit=1` kod bilan **hech qanday parallellik ishlatmasdan** 5 ta chegirmali checkout ketma-ket ochib, keyin beshtasini ham to'lash mumkin edi. **Tuzatildi**: haqiqiy himoya endi `ProcessPaymentGrant` ichida — promo qatori `FOR UPDATE` bilan qulflanadi va limit **qayta** tekshiriladi. Kod endi haqiqiy emas bo'lsa, grant **to'langan summaga proporsional** kesiladi (`proRatedDays`, har doim ≥1 kun) va sabab `entitlement.note`ga yoziladi — ya'ni pul yegan holat ham, bepul VIP ham chiqmaydi. `StartCheckout`dagi qulf endi nima uchun borligini (tez, to'g'ri UX-javob + nol-summali tarmoq uchun haqiqiy himoya) aniq izohlaydigan comment bilan qoldirildi, chunki uni yana "limit himoyasi" deb o'qish xato bo'lardi.

**Critical #2 — parallel `GrandDays` grantlari bir-birini yo'q qilardi.** `GrantDays` mavjud entitlement oxirini o'qib, ustiga qo'shardi — qulfsiz. Ikki to'lov bir vaqtda tugasa, ikkisi ham bir xil "hozirgi oxir"ni o'qib, biri ikkinchisini bosib ketardi (mijoz pulini to'lab, kunini olmasdi). **Tuzatildi**: `LockProfileForGrant` (`SELECT ... FOR UPDATE` profil qatorida) — barcha grantlar profil bo'yicha ketma-ketlashadi. `cmd/grantvip` ham tranzaksiyaga o'raldi, aks holda qulf ma'nosiz bo'lardi (autocommit'da darhol bo'shaydi). Deadlock-tartibi izohlangan.

**#3 — Grand Mock 85% darvozasi o'ynab bo'ladigan edi.** `ReadinessPct` mastery foizi bo'lgani uchun **har kategoriyadan bittagina savolga** to'g'ri javob berib 100% "tayyorlik" ko'rsatish mumkin edi — 1200+ savollik bankdan 12 tasini bilib, "bosh imtihon"ni ochish. **Tuzatildi**: ikkinchi darvoza — hajm chegarasi. Migratsiya `0018` `limit_config`ga `grand_mock_min_studied_pct` (25%) qo'shadi; eligibility endi bankning kamida 25% (`CountValidQuestions` orqali absolyut songa aylantiriladi) o'rganilganini ham talab qiladi. Absolyut son emas, foiz — bank o'sganda chegara o'zi o'sadi.

**#4 — Grand Mock paywall VIP-upsell yo'lini yo'q qilardi.** VIP bo'lmagan foydalanuvchi `403 mock_not_eligible` olardi, ya'ni "o'qishing yetmadi" degan xabar — to'lash kerakligi hech qayerda ko'rinmasdi. **Tuzatildi**: VIP to'sqinlik qilsa `402 vip_required` (frontend allaqachon `/premium`ga yo'naltiradi), `403 mock_not_eligible` esa faqat o'qish-talablariga qoldi. `GrandMockCard` endi VIP holatida haqiqiy "VIP obunani faollashtirish" tugmasini `/premium`ga ko'rsatadi, hajm-chegarasida esa progress-bar **mastery emas, o'rganilgan savol sonini** kuzatadi (aks holda 100% mastery bilan qulflangan holatda bar to'la turardi). `session/start` sahifasi ham `mock_not_eligible`ni endi generic 500-xabar sifatida emas, o'z matni + dashboard'ga qaytish tugmasi bilan ko'rsatadi. Dizayn (gold, Trophy/Lock, progress) o'zgarmadi.

**#5 — referal kodi ikki joyda yo'qolardi.** (a) Tizimga allaqachon kirgan odam invite havolasini ochsa, middleware uni `/login`dan `/dashboard`ga uloqtirardi va **`?ref=` query'ni tashlab ketardi** — referal butunlay yo'qolardi. (b) `verify` sahifasi kodni **so'rov yuborishdan oldin** localStorage'dan o'chirardi, ya'ni bitta tarmoq uzilishi referalni (va taklif qiluvchining mukofotini) qaytarib bo'lmaydigan tarzda yo'q qilardi. **Tuzatildi**: butun oqim `lib/referral-storage.ts`da jamlandi — kod faqat server **qat'iy** javob berganda (`referral_not_found`/`referral_self`/`referral_already_applied`) o'chiriladi, tranzient xato (offline, 5xx, hali sessiya yo'q) da saqlanadi; yangi `ReferralCapture` komponenti autentifikatsiyalangan layout'da turib qolgan kodni qayta urinadi; middleware `?ref=`ni (faqat shuni) redirect orqali uzatadi. Bundan tashqari `ApplyReferralCode` endi unique-violation'ni boshqa DB xatolaridan ajratadi — avval **har qanday** xato "allaqachon biriktirilgan" deb ko'rsatilardi, ya'ni haqiqiy infratuzilma nosozligi yashirilardi.

**#6 — to'lov tarixi lokalizatsiyalanmagan + refund "xatolik" deb ko'rsatilardi.** `" so'm"` va `" kun"` kodga qattiq yozilgan edi (ru/uz-Cyrl foydalanuvchi ham lotin o'zbek matnini ko'rardi), `refunded` holati esa `default` tarmoqqa tushib **qizil "Xatolik"** sifatida ko'rinardi — pul qaytarilgani foydalanuvchiga "to'lov o'tmadi, qayta urin" deb o'qilardi. **Tuzatildi**: uchta tilga kalit qo'shildi, holat-badge'lar deklarativ jadvalga aylantirildi, `refunded` o'z ko'k badge'ini oldi.

**#7 (yakuniy review'da topildi) — to'lov yozuvi nima sotilganini saqlamasdi.** `payment` faqat `amount_uzs` va `tariff_id`ni saqlardi, `tariff.days`/`tariff.price_uzs` esa **jonli** jadvaldan qayta o'qilardi. `created` to'lov hech qachon eskirmagani uchun checkout va webhook orasidagi oyna cheksiz: tarif narxi/muddati o'zgartirilsa, allaqachon boshlangan xaridlar qiymati **jimgina qayta yozilardi** (to'liq narx to'lagan mijoz narx oshgandan keyin kamroq kun olardi), va pro-rating muzlatilgan summani siljigan narxga bo'lardi. **Tuzatildi**: migratsiya `0019` `payment`ga `tariff_days_snapshot`/`tariff_price_uzs_snapshot` (NOT NULL, mavjud qatorlar backfill qilingan) qo'shadi; `StartCheckout` ularni yozadi, completion faqat snapshot'ni o'qiydi (`ListMyPayments` ham — tarix sotilgan muddatni ko'rsatishi kerak). Test: tarif checkout'dan **keyin** 30 kun/59 900 → 15 kun/119 800 ga o'zgartiriladi, mijoz baribir 30 kun oladi (tuzatishsiz test 15 kun bilan yiqiladi — empirik tekshirildi).

**Qo'shimcha (kichik, ammo hujjatsiz qolgan narsalar):** referal invite havolasi `https://avtotest.uz`ni qattiq yozardi — endi `PUBLIC_BASE_URL` konfiguratsiyasi (dev'da localhost:3000, shuning uchun oqim nihoyat lokal test qilinadi). `backend/.env.example` yaratildi: `config.Load()` o'qiydigan **har bir** env o'zgaruvchisi default qiymati va staging/prod'da nima majburiy bo'lishi bilan yozilgan (avval bu bilim faqat kodda edi).

### 1.-2 TEST INFRATUZILMASI — jim buzilish manbai yopildi (#38)
Muammo: barcha DB-test paketlari **bitta** `avtotest_test` bazasini bo'lishardi va har `testdb.New` deyarli hamma jadvalni `TRUNCATE` qilardi. To'g'rilik `-p 1` flagini eslab qolishga bog'liq edi; oddiy `go test ./...` paketlarni bir-birining fixture'ini o'chirishga majbur qilardi va bu **deadlock / foreign-key xatolari** ko'rinishida chiqardi — ya'ni tekshirilayotgan kodda haqiqiy concurrency bug bordek. Soatlar yo'qotadigan turdagi yolg'on signal.

Tuzatish: har test **paketi** o'z fizik bazasini oladi (`internal/testdb` paket katalogidan nom chiqaradi: `internal/billing` → `avtotest_test_internal_billing`). `CREATE DATABASE` advisory-lock ostida ketma-ketlashtiriladi (aks holda `template1` bo'yicha to'qnashuv), identifikator 63 baytdan oshsa hash qo'shiladi (kesish ikki paketni bitta bazaga qo'shib, muammoni qaytarardi). Nozik nuqta: `pool_max_conns` faqat pgxpool DSN'iga qo'shiladi — golang-migrate noma'lum parametrni serverga runtime-sozlama sifatida uzatib, ulanishni yiqitadi.

**Va shu izolyatsiya darhol ikkinchi, mustaqil muammoni ochib berdi**: `internal/auth`ning OTP-cooldown testi parallel rejimda yiqildi — sababi Redis. `redisx.NewTest` har paketda **bitta** Redis DB'sini `FLUSHDB` qilardi. Tuzatildi: paket → Redis DB indeksi aniq jadval bilan taqsimlanadi (`testDBByPackage`), ro'yxatda yo'q paket jim bo'lishish o'rniga baland ovozda yiqiladi. Redis'da mantiqiy baza soni qat'iy (default 16) bo'lgani uchun hash bilan to'qnashuvsizlikni kafolatlab bo'lmaydi — shuning uchun aniq jadval. Jadval invariantlari (indekslar takrorlanmasin, 0 ishlatilmasin — u dev bazasi, 0-15 oralig'ida bo'lsin) test bilan qulflangan. Umumiy paket-identifikatori `internal/testenv`ga ajratildi.

Natija: `make test` (`-p 1`) va yangi `make test-parallel` **bir xil** natija beradi; parallel ~44s vs 117s, 3/3 marta yashil. `-p 1` endi to'g'rilik sharti emas, faqat resurs tanlovi. `make test-db-reset` — hosila bazalarni tozalash (migratsiya joyida o'zgartirilsa kerak bo'ladi).

### 1.0 M4-01 (Leaderboard) — TUGADI, shu sessiyada qurildi va auditlandi
Spec: `docs/superpowers/specs/2026-07-25-m4-01-leaderboard-design.md`. Plan: `docs/superpowers/plans/2026-07-25-m4-01-leaderboard.md` (8 task — 7 asosiy + audit'da topilgan 1 qo'shimcha).

**Dizayn**: `session_answer` (allaqachon bor jadval) — haqiqat manbai; Redis sorted-set — faqat tezkor, to'liq tiklanadigan kesh (yangi paket `internal/leaderboard/`). `GET /leaderboard?period=daily|weekly|monthly|alltime` (auth): top-10 + sizning o'rningiz + atrofingiz. Kunlik ball chegarasi (`leaderboard_daily_points`: 30 bepul/100 VIP) — farming'ga qarshi. `cmd/rebuildleaderboard` — Redis yo'qolsa to'liq tiklash CLI'si.

**Ish jarayonida topilib TUZATILGAN muhim narsalar** (bu ish uslubining o'zi ishlayotganini isbotlaydi):
- Task 3 implementeri o'zining TDD testida haqiqiy matematik xato topdi (`EncodeScore` tie-break formulasi `-` o'rniga `+` bo'lishi kerak edi — `floor(N-f)` har doim `N-1` beradi) — testni "moslashtirib qo'yish" o'rniga to'g'ri xabar berdi, tuzatildi, mustaqil reviewer arifmetikasi bilan qayta tasdiqladi.
- Task 4'ning `RebuildPeriod`'i eski Redis a'zolarni tozalamas edi (faqat qo'shar, o'chirmas edi) — review topib tuzatdi.
- **Eng muhimi — yakuniy whole-branch review'da topildi**: `RebuildPeriod` kunlik ball chegarasini qo'llamas edi, ya'ni Redis yo'qolib qayta tiklansa, chegaraga yetgan foydalanuvchi "cheklovsiz" (yuqoriroq) ball bilan qaytar edi — anti-fraud chegarani soqov qoldirar edi. Bu alohida 8-task sifatida tuzatildi: kunlik-guruhlab, har kunni joriy VIP holati+joriy chegaraga kesib, keyin yig'ish. Aniq tarixiy moslik emas (joriy holat/chegara bilan approksimatsiya), lekin cheksiz portlashni butunlay yo'q qiladi. Shu jarayonda yana bitta kichik reviewer topilmasi (`date_trunc` aniq UTC belgilanmagan edi) ham tuzatildi.
- Bonus: to'liq `go test ./...` ishga tushirilganda M4-01'dan oldingi (Task 1 migratsiyasidan qolgan) bitta hardcoded `limit_config` qator-soni testi buzilgani topildi va tuzatildi — bu hech qaysi task-review'da ko'rinmagan edi, chunki reviewerlar odatda faqat o'z paketini ishga tushiradi.

**Qoldiq (past-xavfli, hujjatlashtirilgan)**: rebuild'ning cap-approksimatsiyasi (yuqoriga qarang, §3'da ham) aniq tarixiy moslikni kafolatlamaydi — VIP holati/cap qiymati rebuild oynasi ichida o'zgargan bo'lsa ozgina chetlanish mumkin, past-xavfli va o'z-o'zidan tuzaladigan.

### 1.1 MUHIM — bu sessiyada to'liq audit o'tkazildi va 3 ta Critical pul-xavfsizlik xatosi topilib TUZATILDI
Oldingi sessiya/AI M2-05/06/09/10/11'ni "tugadi" deb belgilagan va build/testlar yashil bo'lgan, lekin **testlar buglarni tutmagan** (ba'zilarida hatto noto'g'ri mock-shape ishlatilgan). To'rtta mustaqil chuqur code-review (parallel subagentlar) + qo'lda tekshiruv orqali quyidagilar topildi va **hammasi tuzatildi, testlar bilan tasdiqlandi**:

1. **Promo-kod orqali cheksiz bepul VIP** (`internal/billing/checkout.go` `StartCheckout`) — tranzaksiya/row-lock yo'q edi, reviewer 10 ta parallel so'rovda 10/10 muvaffaqiyatli exploit qilib ko'rsatdi ("bir martalik" promo 10 marta bepul VIP berdi). **Tuzatildi**: `pool.Begin`+`SELECT...FOR UPDATE` (promo_code qatori qulflanadi), migratsiya 0016 (`promo_redemption_one_per_payment` backstop unique index), 2 ta yangi concurrency-testi (`checkout_race_test.go`). ~~Qoldiq: nol bo'lmagan checkout uchun N ta parallel to'lovchi limitni oshib ketishi mumkin~~ — **bu baho xato edi va ikkinchi raundda yopildi**: muammo parallellikni umuman talab qilmasdi va shuning uchun "past xavfli" emas edi. §1.-1 Critical #1'ga qarang.
2. **Referal dasturi — concurrent to'lovlarda 2x mukofot** (`internal/billing/referral.go`) — avval `GrantDays` chaqirilib, keyin status-guard yangilanardi (teskari tartib). **Tuzatildi**: bitta atomik `UPDATE...WHERE status='pending' RETURNING` (`ClaimPendingReferralForReferee`) — claim-then-grant. Yangi concurrency test: bug qaytarib tasdiqlangan (8 ta parallel to'lovda avval 3x grant chiqqan), tuzatilgach 1x, `-race` bilan 10x tasdiqlangan.
3. **To'lov tarixi UI har doim bo'sh ko'rinardi** (`payment-history-card.tsx`) — backend `{"data": [...]}` qaytaradi, frontend `{"data": {"items": [...]}}` deb kutgan edi. Test ham noto'g'ri shape bilan mock qilingan bo'lib, yolg'on ishonch bergan. **Tuzatildi**: to'g'ri tur (`PaymentItem[]`), test to'g'ri shape bilan qayta yozildi. Shu bilan birga: referal havolasi endi backend'ning haqiqiy `invite_url`sidan foydalanadi, `/r/[code]` redirect-sahifa qo'shildi (avval umuman yo'q edi, havola bosilganda 404 berardi), referal xato-xabarlari endi lokalizatsiya qilingan (`err.code` orqali, `promo-input.tsx`dagi naqshga o'xshab), va login sahifasi `?ref=CODE`ni ushlab, OTP-tasdiqdan keyin avtomatik biriktiradi. **Diqqat**: bu oqimning o'zi ham to'liq ishlamas edi (ikki joyda kod yo'qolardi) — ikkinchi raundda tuzatildi, §1.-1 #5.

**M2-07 GRAND MOCK ham shu auditda topildi va to'g'ri qurildi** (pastga qarang, §2) — oldingi hujjat "M2 to'liq bitdi" deb yozgan edi, lekin M2-07 uchun faqat spec+plan bor edi, implementatsiya YO'Q edi. Bu safar to'g'ridan-to'g'ri qurildi.

## 2. Hozirgacha nima TUGADI

**M1 (backend+kontent+frontend asosiy) — TO'LIQ.**

**M2-01 (tarif modeli) — TUGADI, LIVE.** `GET /api/v1/tariffs`: Nexia(7 kun/24 900) / Gentra(30/59 900) / Malibu(75/109 900) + Matiz=bepul. 3 til, hisoblangan per-day/discount.

**M2-02 (Payme integratsiyasi, sandbox) — TO'LIQ TUGADI.** `internal/billing/payme/` — JSON-RPC 2.0 webhook, 6 metod, tranzaksiya+row-lock bilan xavfsiz.

**M2-03 (Click integratsiyasi, sandbox) — TO'LIQ TUGADI.** `internal/billing/click/` — form-urlencoded webhook, MD5 sign_string, tranzaksiya+row-lock bilan xavfsiz.

**M2-04 (to'lov tarixi, read-side) — TUGADI.** `GET /api/v1/me/payments?limit=N`.

**M2-05 (promo-kodlar, backend) — TUGADI + AUDIT'DA TUZATILDI.** `POST /api/v1/billing/promo/validate`, `POST /me/checkout`'ning `promo_code` parametri. Promo redemption endi `pool.Begin`+row-lock bilan xavfsiz (§1.1'ga qarang).

**M2-06 (referal dasturi, backend) — TUGADI + AUDIT'DA TUZATILDI.** `user_referral_code`/`referral` jadvallari (migratsiya 15). `GET /me/referral`, `POST /referral/apply`. Referee birinchi to'lov qilganda referrer +7 kun VIP oladi — endi claim-then-grant bilan xavfsiz (§1.1).

**M2-07 (GRAND MOCK — bosh imtihon simulyatsiyasi) — TUGADI (bu auditda birinchi marta qurildi).** Spec qayta yozildi: `docs/superpowers/specs/2026-07-24-m2-07-grand-mock-design.md`. **Muhim dizayn qarori**: bu YANGI subsystem emas — mavjud `internal/session` dvigatelining 5-rejimi sifatida qurilgan (`variant`/`exam`/`practice`/`mistakes` bilan bir qatorda), chunki DB CHECK constraint va `limit_config` (85% threshold) buni M1'dan beri kutib turgan edi. `StartSession`ning `switch req.Mode`iga `case "grand_mock":` qo'shildi (aks holda "exam" bilan bir xil: 20 savol/25 daqiqa/2 xato), va 6+1 joyda `row.Mode == "exam"` tekshiruvi yangi `IsExamLike()` helper orqali `grand_mock`ga ham tarqatildi (vaqt-tugashi, anti-cheat redaction, scoring). Yangi endpoint faqat bitta: `GET /me/mock-eligibility` (o'qish uchun) — start esa mavjud `POST /sessions {mode:"grand_mock"}`ning o'zi. Frontend: `GrandMockCard` (dashboard'da, qulflangan/ochiq holat), `GrandMockCertificateDialog` (confetti+sertifikat, `session/[id]` sahifasining mavjud exam-natija ekraniga qo'shimcha sifatida).

**Eligibility ikkinchi raundda qayta ishlandi** (§1.-1 #3/#4): faqat "VIP + mastery≥85%" yetarli emas edi — mastery o'ynab bo'ladigan edi va VIP-to'siq upsell'ni yashirardi. Endi uchta shart: VIP (yo'q bo'lsa `402`), bankning ≥25%i o'rganilgan (`grand_mock_min_studied_pct`, migratsiya `0018`), mastery ≥85%. `StartSession` bu tekshiruvni endi `MockEligibility`ning o'zidan chaqiradi, ya'ni start va UI **bir xil** mantiqni ishlatadi (avval ikki joyda takrorlangan edi).

**M2-08 (tarif UI) — TUGADI.** `/premium` sahifasi `GET /tariffs`dan qayta qurilgan, haqiqiy "Sotib olish" tugmasi bilan.

**M2-09 (checkout oqimi UI, frontend) — TUGADI.** Provider picker (Payme/Click), promo-kod tekshirish, natija sahifalari (success/failure/pending + status polling). **Minor qoldiq**: backend `StartCheckout`ga hali `returnURL` uzatilmaydi (`billing/handlers.go`da hardcoded `""`) — demak Payme/Click haqiqiy to'lovdan keyin foydalanuvchini avtomatik qaytarmaydi, `/checkout/pending` sahifasi hozircha faqat qo'lda navigatsiya bilan yetib boriladi. Kichik, ixtiyoriy tuzatish — keyingi ishga qoldirildi.

**M2-10 (to'lov tarixi + referal UI, frontend) — TUGADI + AUDIT'DA TUZATILDI.** `ReferralCard` + `PaymentHistoryCard` (`/profile`). Barcha audit topilmalari (§1.1) tuzatildi: response-shape bug, referal havolasi, lokalizatsiya, `/r/[code]` route.

**M2-11 (mehmon-demo landing funnel, frontend) — TUGADI.** Landing'dagi `DemoQuestionBlock` — demo savol, izoh, ro'yxatdan o'tish chaqiruvi, "yana bitta savol" replay.

> **🎉 M2 (MONETIZATSIYA) BO'LIMI HAQIQATAN TO'LIQ BITDI** (audit + tuzatishlardan keyin, GRAND MOCK ham qo'shilib). Merchant sandbox kalitlari (Payme/Click) qo'yilsa, to'liq to'lov tizimi live rejimda ishlaydi.

**M4-01 (Leaderboard, backend) — TUGADI.** §1.0'ga qarang — to'liq tafsilot shu yerda.

## 3. Qoldiq — past-xavfli, kelajakka qoldirilgan (hujjatlashtirilgan, yashirilmagan)
- M2-09: checkout `returnURL` hali backend'dan uzatilmaydi (yuqoriga qarang). `PUBLIC_BASE_URL` endi konfiguratsiyada bor, ya'ni buni ulash uchun kerakli qism tayyor.
- Referal: retroaktiv biriktirishga qarshi himoya yo'q (eski foydalanuvchi ham istalgan vaqt do'st kodini biriktirib, keyingi to'lovini "referal" qilib ko'rsatishi mumkin) — anti-fraud kuchaytirilishi mumkin. **E'tibor**: `ReferralCapture` autentifikatsiyalangan layout'da turgani uchun bu yo'l endi oldingiga qaraganda **ochiqroq** — ya'ni buni yopish avvalgidan muhimroq bo'ldi.
- `referral_attribution` jadvali (migratsiya 0003, fraud_flags bilan) ishlatilmay qolgan — M2-06 parallel yangi jadval qurgan. Tozalash yoki hujjatlashtirish kerak (yuqoridagi anti-fraud ishi uchun aynan shu jadval mo'ljallangan ko'rinadi).
- M2-11: "keyingi savol" replay'i faqat 2 ta savol orasida almashadi (backend `demoQuestionCount=2`) — "ko'p-savolli" degan va'daga nisbatan zaifroq.
- M4-01: `RebuildPeriod`ning cap-approksimatsiyasi aniq tarixiy moslikni kafolatlamaydi (§1.0'ga qarang) — past-xavfli.
- Promo pro-rating (§1.-1 Critical #1): limit tugagan kodda to'lov tugasa, kun to'langan summaga proporsional beriladi. Bu pul-neytral, lekin mijoz "30 kun" kutib 12 kun olishi mumkin — sabab `entitlement.note`da bor, ammo foydalanuvchiga **ko'rsatilmaydi** va avtomatik refund/xabar yo'q. Amalda kamdan-kam (kod aynan checkout va to'lov orasida tugashi kerak), lekin support-jarayon kerak bo'ladi.

## 3.1 MUHIM TUZATISH (2026-07-25) — oldingi hujjat xato yozgan edi
Avvalgi versiyada "M4-01 = Gamification/Streak tizimi" deb yozilgan edi — bu **XATO** edi, tekshirilmagan holda eski hujjatdan ko'chirilgan. Haqiqat: **streak/gamification allaqachon M1'da to'liq qurilgan** (`GET /me/streak`, `internal/progress` paketi, `RecordActivity` har javobdan keyin chaqiriladi, dashboard/stats/header/sidebar'da ko'rsatiladi). Roadmap'ning o'zida (`docs/superpowers/2026-07-24-roadmap-m2-to-admin.md`, bo'lim 3) haqiqiy **M4-01 — Leaderboard** (Redis sorted-set reyting) — bu shu sessiyada qurildi (§1.0). Xulosa: **agar biror hujjatda biror Plan nomi/tavsifi shubhali tuyulsa, roadmap manba-hujjatini tekshirmasdan ishonmang** — bu safar tekshirilmasdan davom etilganda, GRAND MOCK holatidagidek, xato ko'chib ketishi mumkin edi.

## 4. KEYINGI ANIQ QADAM (tavsiya)

M4 (Growth) roadmap jadvali (`docs/superpowers/2026-07-24-roadmap-m2-to-admin.md`, bo'lim 3):

| Plan | Nomi | Bog'liqlik | Holat |
|------|------|-----------|-------|
| M4-01 | Leaderboard (backend) | — | **TUGADI** (§1.0) |
| M4-02 | Leaderboard UI (frontend) | M4-01 | Navbatda |
| M4-03 | Battle Arena — realtime infra | M4-01? | Navbatda (katta, bir necha task) |
| M4-06 | Telegram bot — poydevor | — | Mustaqil, M4-02 bilan parallel qilish mumkin |

Tavsiya: **M4-02 (Leaderboard UI)** bilan davom ettirish — M4-01 backend allaqachon tayyor va `GET /leaderboard` orqali to'liq ishlaydi, frontend faqat shuni ko'rsatishi kerak. Muqobil: agar parallel ish xohlansa, **M4-06 (Telegram bot poydevori)** M4-01/02'ga bog'liq emas, alohida boshlash mumkin.

Har biri: avval `superpowers:brainstorming` (agar spec hali yo'q bo'lsa) → `superpowers:writing-plans` → `superpowers:subagent-driven-development` bilan bajarish. **Katta ishni "tugadi" deb belgilashdan oldin albatta whole-branch review o'tkazing** — M4-01'ning o'zida buni 2 marta muhim narsa topib isbotladi (RebuildPeriod'ning tozalamasligi, keyin uning cap-approksimatsiya yo'qligi) — ikkalasi ham faqat butun feature holisticha ko'rilganda ko'rindi, task-darajasidagi review'lar buni ko'ra olmadi.

## 5. Operatsion faktlar (MUHIM — vaqt tejaydi)
- **Go PATH:** har `go`/`sqlc` buyrug'iga `export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"` prefiks.
- **sqlc generate:** `make generate` (repo ildizidan).
- **DB testlar:** `make test` (`-p 1`) yoki `make test-parallel` — **ikkisi ham to'g'ri** (§1.-2). `testdb.New(t)` paketga xos bazani yaratadi/migratsiya qiladi + `Truncate` qiladi (process-wide pool, `sync.Once` — concurrent-goroutine testlar yozish uchun qulay). Migratsiyani **joyida** o'zgartirsangiz `make test-db-reset` qiling, aks holda eski hosila bazalar qayta ishlatiladi.
- **Redis testlar:** yangi paket `redisx.NewTest`dan foydalansa, `internal/redisx/testhelper.go`dagi `testDBByPackage`ga qo'shish kerak — aks holda test darhol tushunarli xato bilan yiqiladi (bu ataylab shunday, jim bo'lishish o'rniga).
- **`pool.Exec` parametr bilan bir nechta SQL buyrug'ini QO'LLAMAYDI** (prepared statement) — parametrli insert'larni alohida `Exec`ga bo'ling.
- **Dev API restart:** `pkill -f "cmd/api"` ISHLATMANG — aniq PID (`ss -ltnp|grep :8090`) + `run_in_background`.
- **Infra:** `docker compose` (postgres:5432, redis:6379, minio:9000).
- **Payme/Click kalitlari:** ENV bo'sh — webhook kutilganidek rad etadi.
- **ENV o'zgaruvchilari:** `backend/.env.example` — `config.Load()` o'qiydigan hammasi, default qiymati bilan. `ENV=staging|prod` qo'ysangiz `JWT_SECRET`, `CLIENT_IP_ASSERTION_SECRET` (≥32 bayt) va sandbox bo'lmagan `OTP_CHANNEL` majburiy, aks holda startup'da yiqiladi.
- **Money-critical kod naqshi** (endi barcha to'lov/entitlement yo'llarida qat'iy qo'llanilgan): `pool.Begin` + `SELECT...FOR UPDATE` (yoki `UPDATE...RETURNING` claim-pattern) + tx-bound `Service{Q: q}`. Yangi pul/entitlement kodi yozganda BOSHIDANOQ shu naqshni qo'llang — post-hoc tuzatish qimmatga tushadi (bu audit buni 3 marta isbotladi).

## 6. Ish uslubi (skill'lar)
Har Plan: `brainstorming` (spec) → `writing-plans` (reja) → TDD implementatsiya (`subagent-driven-development`) → whole-branch review → push. Mustaqil Plan'lar (yoki bitta Plan ichidagi fayllar jihatidan mustaqil task'lar) parallel subagentlarga (`Agent(isolation:"worktree", run_in_background:true)`) berish mumkin — integratsiya `git cherry-pick` yoki `git merge --ff-only` bilan. **Katta ishlarni "tugadi" deb belgilashdan oldin mustaqil chuqur audit/review o'tkazing** — build/test yashil bo'lishi yetarli emas, ayniqsa pul-bog'liq kodda (bu sessiya buni amalda isbotladi: 4/4 review Critical topdi).

## 7. Keyingi Plan'lar to'liq ro'yxati (roadmap'dan)
M2 tugadi. Keyingi: M4 (Growth) → M5/M6/M7 → **M3 (Super Admin) ENG OXIRIDA**. To'liq dekompozitsiya: `docs/superpowers/2026-07-24-roadmap-m2-to-admin.md`.
