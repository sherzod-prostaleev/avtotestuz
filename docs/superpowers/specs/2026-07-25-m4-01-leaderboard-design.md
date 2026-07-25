# M4-01: Leaderboard (Reyting) — Design Spec

Sana: 2026-07-25 · Milestone: M4 (Growth) · Plan: M4-01 · Qatlam: backend

## 1. Maqsad

Foydalanuvchilarni to'g'ri javob berishga undaydigan, real-time yangilanadigan reyting jadvali: kunlik, haftalik, oylik va all-time. Bepul va VIP foydalanuvchilar bir xil reytingda — bu bepul foydalanuvchini VIP sotib olishga (ko'proq mashq qilish uchun) undaydi. M4-02 (Leaderboard UI) va kelajakdagi M4-03/04 (Battle Arena) shu ustiga quriladi, lekin bu Plan ular bilan bog'liq emas — faqat backend, faqat reyting.

## 2. Asosiy dizayn qarori: Postgres = haqiqat manbai, Redis = tezkor reyting-keshi

`session_answer` jadvali (migratsiya 0004: `session_id, question_id, answer_id, is_correct, position, answered_at`, `exam_session.profile_id` orqali profilga bog'langan) **allaqachon** har bir to'g'ri javobni abadiy saqlaydi. Shuning uchun:

- **Yangi "voqealar jadvali" (event log) yaratilmaydi** — bu Postgres'da allaqachon bor ma'lumotni ikki marta saqlash bo'lardi.
- Redis sorted-set'lar **faqat hisoblangan/derived kesh** — yo'qolsa ham, `session_answer`dan istalgan vaqt qayta hisoblab tiklanadi (`RebuildPeriod`, pastga qarang).
- Yozish yo'li: `session.Service.SubmitAnswer` to'g'ri javobni Postgres'ga yozgandan keyin (bu allaqachon bor kod), yangi `leaderboard.Service.RecordPoint(ctx, profileID)` chaqiriladi — Redis pipeline orqali 4 ta davr-kalitiga bir vaqtda `ZINCRBY`.

Bu M2-07 (GRAND MOCK)'dagi saboqning davomi: avval mavjud sxemani o'rganib, ustiga qurish — parallel/dublikat ma'lumot ombori yaratmaslik.

## 3. Redis kalit sxemasi va ball formulasi

Kalitlar (barchasi `lb:` prefiksi bilan, UTC kun chegarasi — loyihada allaqachon qabul qilingan konventsiya, `internal/progress/service.go`dagi `todayUTC()`ga mos):
- `lb:daily:<YYYY-MM-DD>` — TTL 3 kun (kechagi natijalar qisqa vaqt ko'rinib turishi uchun).
- `lb:weekly:<YYYY-Www>` (ISO hafta) — TTL 3 hafta.
- `lb:monthly:<YYYY-MM>` — TTL 3 oy.
- `lb:alltime` — TTL yo'q.

**Ball va teng-ball tartibi (tie-break)**: Redis ZSET balli — `float64(points) - float64(answeredAtUnixNano)/1e19`. Bu:
- Butun son qismini (`points`) hech qachon o'zgartirmaydi (chegirma har doim `< 1`ga teng, chunki unix-nano/1e19 juda kichik).
- Teng ballda **birinchi erishgan** foydalanuvchi tepada turadi (kamroq chegirma — chunki vaqt o'tgan sari chegirma soni ORTADI, demak keyinroq erishgan pastroq balga ega bo'ladi).
- `RecordPoint` chaqirilganda profilning joriy umumiy balli emas, balki **shu voqeaning vaqt belgisi** ishlatiladi — ya'ni har safar ZINCRBY qilinganda score qayta hisoblanmaydi, faqat `points` qismi ortadi va oxirgi chegirma yangilanadi (Redis `ZADD` bilan to'g'ridan-to'g'ri emas, balki `ZINCRBY` bilan ballni oshirib, keyin **oxirgi voqea vaqtini alohida hash'da** saqlab, ranglashda ishlatish — bu yondashuv Redis'ning yagona buyruqda ikki narsani birlashtira olmasligi sababli quyidagicha amalga oshiriladi):

  To'g'ri va sodda yechim: har `RecordPoint` chaqiruvida **butun scoreni qayta hisoblab ZADD qilish** (ZINCRBY emas): `newScore = currentIntegerPoints + 1 - now/1e19`. Buning uchun avval `ZSCORE` bilan joriy ballni o'qib, butun qismini ajratib, +1 qilib, yangi tie-break bilan qayta yozish kerak — bitta `RecordPoint` chaqiruvida 4 kalit uchun 4 juft `ZSCORE`+`ZADD` (pipeline'da, network round-trip bitta). Bu ZINCRBY'dan biroz murakkabroq, lekin tie-break to'g'ri ishlashi uchun zarur va hajman arzon (kichik pipeline, millisekundlar).

  **Bilib turilgan, ataylab qabul qilingan cheklov**: `ZSCORE` (o'qish) va `ZADD` (yozish) atomik juftlik EMAS — agar bir profilning ikkita javobi millisekundlar farqida, parallel so'rovlar orqali kelsa (masalan ikki tab), ikkalasi ham bir xil eski ballni o'qib, ikkalasi ham shu ustiga +1 qilib yozishi mumkin — natijada +2 o'rniga +1 qo'shiladi (lost update). Bu pul emas, reyting balli — oqibat past (bitta ball yo'qolishi mumkin), va `RebuildPeriod` navbatdagi ishga tushishida (§5) Postgres asosida avtomatik tuzatiladi. Redis `WATCH`/`MULTI` yoki Lua-skript bilan to'liq atomiklashtirish mumkin, lekin bu darajadagi murakkablik past-xavfli reyting balli uchun asossiz — YAGNI.

## 4. Firibgarlikka qarshi: kunlik ball chegarasi

`limit_config`ga yangi qator: `leaderboard_daily_points` (`free_value=30, vip_value=100`). Printsip: VIP ham CHEKSIZ emas — aks holda reytingning yuqori qismi faqat "kim ko'proq vaqt sarflay oladi"ga aylanib qoladi, bu esa reytingning "kim ko'proq bilim egallayapti" degan asosiy ma'nosini yo'qqa chiqaradi. Mexanizm:
- Har `RecordPoint` chaqiruvida avval bugungi kunda profil nechta ball olganini bilish kerak — bu **Redis daily kalitning o'zidan** o'qiladi (`ZSCORE lb:daily:<bugun> profileID`, butun qismi = bugungi ball soni), qo'shimcha hisoblagich shart emas.
- Agar bugungi ball soni chegaraga yetgan bo'lsa, `RecordPoint` **hech narsa yozmaydi** (daily/weekly/monthly/alltime — barchasiga bir xil qoida, chunki agar kunlik chegaraga yetilgan bo'lsa, boshqa davrlarga ham qo'shilmasligi kerak, aks holda hafta/oy/all-time orqali chegara aylanib o'tiladi).
- Bu **faqat reytingga** ta'sir qiladi — streak (`Progress.RecordActivity`), mastery (FSRS), amaliyot kunlik limiti (`daily_practice_questions`) — bularning barchasi mustaqil, o'zgarishsiz davom etadi.

## 5. Tiklanish (rezilience): `RebuildPeriod`

`leaderboard.Service.RebuildPeriod(ctx, period Period, periodKey string, from, to time.Time) error` — berilgan davr oralig'ida `session_answer JOIN exam_session ON session_id=exam_session.id WHERE is_correct AND answered_at BETWEEN from AND to GROUP BY profile_id, date_trunc('day', answered_at)` so'rovini bajarib, **har bir profil-kun** juftligi uchun alohida hisoblaydi, so'ng har bir kunlik sonni o'sha profilning joriy kunlik chegarasiga (`leaderboard_daily_points`, VIP/free) kesib (cap qilib), keyin kesilgan kunlik sonlarni davr bo'yicha yig'ib, natijadagi har bir `(profileID, cappedTotal)` juftligi uchun mos Redis kalitiga `ZADD` qiladi (tie-break uchun profilning barcha kunlari bo'yicha **eng katta** `answered_at` ishlatiladi — Go xaritasi tartibidan mustaqil "max shu paytgacha" taqqoslash orqali).

**Nima uchun kunlik cap qayta qo'llanadi**: jonli yo'l (`RecordPoint`) profilning kunlik ball soni chegaraga (`leaderboard_daily_points`) yetgach yozishni to'xtatadi — bu farming'ga qarshi himoya. Agar `RebuildPeriod` oddiy `GROUP BY profile_id` bilan **butun davr** bo'yicha yig'sa (cap'siz), Redis yo'qolib qayta qurilgandan keyin chegaraga allaqachon yetgan profil **yuqoriroq, cap qilinmagan** ball bilan qaytadi — bu anti-fraud chegarani sukut bo'yicha yengib o'tadi. Shu sababli kunlik cap endi rebuild vaqtida ham qayta qo'llanadi (Task 8, 2026-07-25).

**Muhim cheklov (aniq tarixiy mosliksiz)**: cap profilning **joriy** (bugungi) VIP holati va `limit_config`dagi **joriy** cap qiymati bilan qo'llanadi — sxema tarixiy VIP holatini yoki tarixiy cap qiymatini saqlamaydi, shuning uchun bu aniq tarixiy proyeksiya emas, balki chegaralangan approksimatsiya. Bu shuni kafolatlaydi: rebuild **hech qachon** cap'siz portlash bermaydi (eng yomon holatda — profilning VIP holati yoki cap konfiguratsiyasi rebuild qilinayotgan oyna ichida o'zgargan bo'lsa, ozgina chetlanish bo'lishi mumkin — tor, past-xavfli, vaqt o'tishi bilan o'z-o'zidan tuzaladigan holat), lekin **bayt-baytiga** jonli holatdagi bilan bir xil ball kafolatlanmaydi agar shu oyna ichida VIP holati yoki cap qiymati o'zgargan bo'lsa.

Bu funksiya:
- Redis butunlay yo'qolgan/tozalangan holatda **to'liq tiklaydi** (ma'lumot yo'qolmaydi, chunki Postgres — asl manba; ball hisoblash yuqoridagi cap approksimatsiyasiga bo'ysunadi).
- Yangi CLI orqali qo'lda ishga tushiriladi: `cmd/rebuildleaderboard` (mavjud `cmd/grantvip`ning bir xil naqshi — flag'lar bilan, `db.Migrate`+`db.NewPool`+`redisx.New` ochib, keyin yopib chiqadi).
- **Yangi migratsiya kerak**: `session_answer`da hozircha `answered_at`/`is_correct` ustida indeks yo'q — davr bo'yicha `GROUP BY profile_id, date_trunc('day', answered_at)` so'rovi katta jadvalda sekin ishlaydi. Qo'shiladi: `CREATE INDEX session_answer_correct_answered_idx ON session_answer(answered_at) WHERE is_correct;` (qisman indeks — faqat to'g'ri javoblar, chunki faqat ular kerak).

## 6. API

### `GET /api/v1/leaderboard?period=daily|weekly|monthly|alltime` (auth majburiy)

Response:
```json
{
  "data": {
    "period": "weekly",
    "you": { "rank": 42, "score": 87, "name": "Foydalanuvchi #a3f1" },
    "top": [
      { "rank": 1, "name": "Aziz Karimov", "score": 340 },
      { "rank": 2, "name": "Foydalanuvchi #9c02", "score": 312 }
    ],
    "around_you": [
      { "rank": 41, "name": "...", "score": 88 },
      { "rank": 42, "name": "Foydalanuvchi #a3f1", "score": 87 },
      { "rank": 43, "name": "...", "score": 85 }
    ]
  }
}
```
- `top`: har doim top-10 (konstanta, `LeaderboardTopN = 10`).
- `you`: agar profil hali umuman ball to'plamagan bo'lsa (`ZSCORE` topilmasa), `rank: null, score: 0`.
- `around_you`: so'ragan profilning ±2 qo'shnisi (jami ≤5 qator); agar profil top-10 ichida bo'lsa, bu maydon bo'sh massiv (frontend `top`ni ko'rsatadi, alohida "atrofingiz" bloki shart emas).
- Ism ko'rsatish: `profile.name` bo'sh bo'lmasa — o'sha; bo'sh bo'lsa — `Foydalanuvchi #<profileID'ning birinchi 4 hex belgisi>` (barqaror, har safar bir xil chiqadi, chunki UUID o'zgarmaydi).

### Wiring
`internal/leaderboard/` — yangi paket (`handlers.go`, `service.go`, `rules.go` — tie-break/scoring sof funksiyalari uchun, mavjud `session/rules.go` naqshiga o'xshab). `server.go`ning mavjud `if deps.Pool != nil && deps.Redis != nil {` blokiga qo'shiladi (Redis allaqachon shu shart ostida bor — `auth.Limiter`/`demo.Handler` xuddi shu joyda ulanadi).

`session.Service`ga yangi ixtiyoriy `Leaderboard *leaderboard.Service` field qo'shiladi (nil bo'lishi mumkin — masalan testlarda ulanmasa, `SubmitAnswer` shunchaki chaqirmaydi). `SubmitAnswer`da `ans.IsCorrect` aniqlangan joyda (mavjud `s.Progress.RecordActivity` chaqiruvi yonida) — `if s.Leaderboard != nil && ans.IsCorrect { _ = s.Leaderboard.RecordPoint(ctx, profileID) }`. Xato bo'lsa **yutilmaydi, lekin javobni buzmaydi** — reyting side-effect, asosiy javob-yozish oqimini to'xtatmasligi kerak (xuddi `Progress.RecordActivity`ning joriy xatosi asosiy oqimni to'xtatgani kabi emas — aslida hozirgi kodda `RecordActivity` xatosi butun `SubmitAnswer`ni to'xtatadi; **leaderboard uchun buni ataylab boshqacha qilamiz**, chunki reyting mastery/streak kabi "muhim" emas — log qilinadi, lekin foydalanuvchi javobi baribir saqlanadi).

## 7. Testlash rejasi
- `internal/leaderboard/rules_test.go`: tie-break formula (teng ballda avvalroq erishgan yuqorida), kunlik chegara mantiqi — sof funksiyalar, Redis'siz.
- `internal/leaderboard/service_test.go` (`redisx.NewTest(t)` + `testdb.New(t)`): `RecordPoint` → 4 kalitga ham yoziladi; kunlik chegaraga yetgach yozilmasligi; `GetLeaderboard` top-N+you+around_you to'g'ri qaytarishi; `RebuildPeriod` Redis'ni tozalab qayta qurishi (determinizm testi — bu rezilience'ning isboti; §5da tushuntirilganidek, natija chegaraga yetmagan profillar uchun aniq bir xil bo'ladi, chegaraga yetgan profillar uchun esa "cap qilingan, cap'siz emas" — bayt-baytiga aynan bir xil emas, balki teng darajada himoyalangan; buni alohida tasdiqlaydi `TestRebuildPeriodAppliesDailyCap`, Task 8).
- `internal/session/service_test.go`ga integratsion test: to'g'ri javobdan keyin `leaderboard.Service` orqali ball haqiqatan qo'shilishi (nil bo'lsa esa `SubmitAnswer` xatosiz ishlashi — orqaga moslik).

## 8. Doiradan tashqari (keyingi Plan'larga)
- Leaderboard UI (M4-02).
- Battle Arena (M4-03/04) — alohida rating/medal tizimi, bu bilan bog'liq emas.
- Reytingda ko'rinishni yoqish/o'chirish (profil sozlamasi) — kelajakda alohida so'ralsa qo'shiladi.
- Redis konteynerida persistence-volume (production deploy vaqtida DevOps ishi, kod darajasida emas — `RebuildPeriod` buning tabiiy backstopi).
