# Imtihon simulyatsiyasi: 20/50 savol rejimlari va avto-o'tish

**Sana:** 2026-08-22
**Holat:** dizayn tasdiqlangan, implementatsiya kutilmoqda
**Tegishli kod:** `backend/internal/session/`, `frontend/src/app/[locale]/(session)/`,
`frontend/src/app/[locale]/(kiosk)/station/`, `frontend/src/components/exam/`

## 1. Muammo

### 1.1. Faqat bitta imtihon turi bor

O'zbekistonda YHQ imtihoni ikki xil topshiriladi:

| | Kim topshiradi | Savol | Vaqt | O'tish sharti |
|---|---|---|---|---|
| **Oddiy** | Birinchi marta prava olayotgan | 20 | 25 daqiqa | ≥18 to'g'ri (≤2 xato) |
| **Qayta topshirish** | Pravadan mahrum bo'lib qayta o'qiyotgan | 50 | 50 daqiqa | ≥46 to'g'ri (≤4 xato) |

Platformada faqat birinchisi bor. `backend/internal/session/rules.go` da
`ExamQuestionCount = 20`, `ExamTimeLimitSec = 25*60`, `ExamErrorsAllowed = 2`
konstantalari `StartSession` ning `case "exam"` shoxida qattiq yozilgan. Ikkinchi
auditoriya — huquqdan mahrum bo'lganlar, ya'ni qayta o'qishga **majburan** kelgan
va shuning uchun to'lovga eng tayyor segment — o'zi topshiradigan imtihonni
platformada umuman mashq qila olmaydi.

### 1.2. `EvaluateExam` sessiyaning o'z qoidasini o'qimaydi

`exam_session` jadvalida har sessiyaning `errors_allowed` ustuni bor va
`SubmitAnswer` (`service.go:599-603`) aynan shu ustundan o'qiydi — ya'ni
"n-xatoda to'xtash" allaqachon sessiyaga bog'langan. Lekin yakuniy baho beruvchi
`EvaluateExam` paket konstantasini ishlatadi:

```go
if correct >= total-ExamErrorsAllowed && wrong <= ExamErrorsAllowed {
```

Bu 20-likda to'g'ri, lekin 50-lik sessiya qo'shilishi bilan **jimgina noto'g'ri
javob beradi**: 50 savolda 3 ta xato qilgan o'quvchi (47/50 — haqiqiy imtihonda
o'tadi) `wrong <= 2` shartida yiqilgan deb belgilanadi. Ya'ni bu shunchaki
"yangi funksiya uchun kengaytirish" emas — 50-lik qo'shilishi bilan bu qatorlar
faol xatoga aylanadi.

### 1.3. Xato hisoblagichi frontendda qattiq yozilgan

`official-avtotest-exam-view.tsx:115`:

```tsx
const errorsAllowed = session.mode === "placement" ? 1 : 2;
```

Sessiyaning `errors_allowed` qiymati API javoblarida (`POST /sessions`,
`GET /sessions/{id}`) **umuman yo'q**, shuning uchun frontend uni rejim nomidan
taxmin qiladi. 50-lik sessiyada ekranda "0/2" deb turadi va o'quvchi 3-xatodan
keyin imtihon tugadi deb o'ylab, aslida davom etayotgan imtihonni tashlab
ketishi mumkin.

### 1.4. Tanlov ekrani yo'q

Barcha "Imtihon" havolalari (`sidebar.tsx:88,141`, `dashboard/page.tsx:200,563`,
`station/page.tsx:51`) to'g'ridan-to'g'ri `/session/start?mode=exam` ga boradi.
Bu sahifa hech narsa so'ramay, `useEffect` ichida darrov sessiya ochib,
`/session/{id}` ga `router.replace` qiladi. Ikki xil imtihon paydo bo'lgach,
foydalanuvchi qaysi birini topshirishini aytadigan joy yo'q.

### 1.5. Javob berilgandan keyin qo'lda o'tish kerak

`session/[id]/page.tsx` da javob yuborilgach `currentIndex` o'zgarmaydi.
Keyingi savolga o'tish uchun: pastdagi raqam-tugmani bosish, yoki `ArrowRight`
(faqat klaviaturada), yoki oddiy UI'dagi "Keyingi" tugmasi. Imtihon UI'sida
(`OfficialAvtotestExamView`) **"Keyingi" tugmasining o'zi yo'q** — faqat raqam
pillari. Telefonda va sensorli kioskda bu har savolda pastdagi mayda raqamni
qidirib bosishni anglatadi; 50 savolli imtihonda bu 50 marta takrorlanadi.

## 2. Maqsad

- **50 savolli "qayta topshirish" imtihoni** qo'shilsin: 50 daqiqa, 4 xatoga
  ruxsat, 5-xatoda darrov to'xtash, 46/50 dan o'tish.
- Imtihon boshlanishidan oldin **rejim tanlash ekrani** chiqsin.
- Javob berilgach **avto-o'tish** ishlasin (900 ms).
- Uchala platformada ham: **web, mobil va kiosk**.
- Imtihon qoidasi (savol soni, vaqt, xato budjeti) **serverda hal qilinsin**;
  klient faqat qaysi turdagi imtihon so'ralayotganini aytsin.

### Maqsad emas

- **Yangi `mode` qatori qo'shilmaydi.** 50-lik ham `mode = "exam"` bo'lib qoladi
  (sabab 3.1-bo'limda). `grand_mock` ("Bosh imtihon") 20 tada qoladi —
  u tayyorlik darvozasi, imtihon turi emas.
- **Admin paneldan sozlash qilinmaydi.** 20/25/2 va 50/50/4 — davlat
  belgilagan qoidalar, biznes parametri emas. Ular kodda, testlangan holda
  turadi; `limit_config` ga chiqarilmaydi.
- **Avto-o'tishni foydalanuvchi sozlay olmaydi.** Har rejimda 900 ms, doimiy.
  Sozlama kerak bo'lsa keyingi iteratsiyada qo'shiladi.
- **Sertifikat/reyting mantiqiga tegilmaydi.** 50-lik imtihon ham boshqa
  imtihonlar kabi FSRS'ni oziqlantiradi va tarixga tushadi, lekin alohida
  sertifikat yoki alohida reyting kategoriyasi olmaydi.

## 3. Backend

### 3.1. Nega yangi rejim emas, balki `count` parametri

Ikki variant ko'rildi:

**A) Yangi `mode = "exam50"`.** `IsExamLike`, `finishInternal` dagi `switch`,
frontenddagi `SessionMode` union, `modeLabel` jadvali, analitika filtrlari,
`toSessionDetailResponse` — hammasi yangi qatorni bilishi kerak. Har bir
unutilgan joy 50-lik sessiyani "noma'lum rejim" holatiga tushiradi.

**B) `mode = "exam"` + `count` parametri (tanlangan).** Sessiya qatorida
allaqachon `total`, `time_limit_sec` va `errors_allowed` ustunlari bor — ya'ni
jadval imtihonlarning bir-biridan farq qiladigan hamma narsasini saqlashga
tayyor. `total = 20` va `total = 50` ikki turni to'liq ajratadi (tarix va
analitika uchun ham yetarli). `count` maydoni `startSessionBody` da
allaqachon bor va `req.Count` ga ulangan — yangi API maydoni kerak emas.

B tanlandi. Xavfsizlik sharti: `count` **whitelist** dan o'tadi, aks holda
foydalanuvchi `count=3` yuborib o'zi uchun yengil "imtihon" yasagan bo'lardi.

### 3.2. `rules.go` — konfiguratsiya jadvali

```go
const (
    ExamQuestionCount = 20
    ExamTimeLimitSec  = 25 * 60
    ExamErrorsAllowed = 2

    // Huquqdan mahrum bo'lganlar qayta topshiradigan imtihon.
    ExamRestoreQuestionCount = 50
    ExamRestoreTimeLimitSec  = 50 * 60
    ExamRestoreErrorsAllowed = 4
)

// ExamConfig — bitta imtihon turining to'liq qoidasi.
type ExamConfig struct {
    QuestionCount int
    TimeLimitSec  int
    ErrorsAllowed int
}

// ExamConfigFor rasmiy imtihon o'lchamini uning qoidalar to'plamiga o'giradi.
// Faqat 20 va 50 — haqiqiy imtihonlar; 0 esa "ko'rsatilmagan" degani va 20 ga
// tushadi (eski `?mode=exam` havolalari ishlashda davom etsin). Boshqa har
// qanday son rad etiladi, ya'ni klient o'zi uchun yengilroq imtihon yasay olmaydi.
func ExamConfigFor(count int) (ExamConfig, bool)
```

### 3.3. `EvaluateExam` sessiyaning xato budjetini oladi

```go
func EvaluateExam(correct, wrong, total, errorsAllowed int, timedOut, tooManyErrors bool) ExamOutcome
```

Shart o'zgarmaydi, faqat konstanta o'rniga parametr ishlatiladi:
`correct >= total-errorsAllowed && wrong <= errorsAllowed`.

`finishInternal` (`service.go:842`) chaqiruvchi tomon:

```go
case "exam", "grand_mock":
    errorsAllowed := ExamErrorsAllowed          // eski, NULL ustunli qatorlar uchun
    if row.ErrorsAllowed.Valid {
        errorsAllowed = int(row.ErrorsAllowed.Int32)
    }
```

Bu `SubmitAnswer:599-603` dagi mavjud naqshning aynan o'zi — natijada
"n-xatoda to'xtash" va "yakuniy baho" bitta manbadan oziqlanadi va bir-biriga
zid javob bera olmaydi.

### 3.4. `StartSession` — `case "exam"`

```go
case "exam":
    cfg, ok := ExamConfigFor(req.Count)
    if !ok {
        return SessionView{}, ErrInvalidRequest      // 400 invalid_request
    }
    // VIP tekshiruvi o'zgarishsiz qoladi
    ids, err = s.Q.RandomQuestionIDs(ctx, int32(cfg.QuestionCount))
    if err == nil && len(ids) < cfg.QuestionCount {
        return SessionView{}, ErrInvalidRequest
    }
    timeLimit = pgtype.Int4{Int32: int32(cfg.TimeLimitSec), Valid: true}
    errorsAllowed = pgtype.Int4{Int32: int32(cfg.ErrorsAllowed), Valid: true}
```

`case "grand_mock"` tegilmaydi (20 tada qoladi). Savol bazasida 1231 ta
tasdiqlangan savol bor, ya'ni 50 ta tasodifiy tanlash uchun zaxira yetarli.

### 3.5. API: `errors_allowed` maydoni qo'shiladi

`SessionView` ga `ErrorsAllowed *int` qo'shiladi va u `SessionDetail` ga
embed orqali o'z-o'zidan o'tadi. JSON tomonda:

- `startSessionResponse` → `"errors_allowed": 4` (null bo'lishi mumkin)
- `sessionDetailResponse` → `"errors_allowed": 4`

`time_limit_sec` bilan bir xil naqsh: `*int`, imtihon bo'lmagan rejimlarda
`null`. Frontend uni `SessionState.errors_allowed` sifatida o'qiydi va
xato-hisoblagichi endi taxmin qilmaydi.

## 4. Frontend: rejim tanlash ekrani

### 4.1. Marshrut

| Kontekst | Manzil | Fayl |
|---|---|---|
| Web / mobil | `/[locale]/exam` | `app/[locale]/(app)/exam/page.tsx` |
| Kiosk | `/[locale]/station/exam` | `app/[locale]/(kiosk)/station/exam/page.tsx` |

Kiosk sahifasi — mavjud kiosk naqshining aynan o'zi
(`station/session/start/page.tsx` da bo'lgani kabi): umumiy komponentni
`kiosk` prop bilan qayta ishlatadi, ichidagi har bir manzil `/station/...`
ostida qoladi. Umumiy komponent: `components/exam/exam-mode-picker.tsx`.

### 4.2. Ko'rinish

Imtihon UI'sining navy/asfalt palitrasida (`#091726` fon, `#0d2e4d` panellar,
`#22c55e` urg'u) — tanlov ekrani imtihonning o'zi bilan bir olamda tursin,
ilovaning oq/och kartalari bilan emas. Ikkita katta karta:

```
┌──────────────────────────┐  ┌──────────────────────────┐
│  1                       │  │  2                       │
│  ODDIY IMTIHON           │  │  QAYTA TOPSHIRISH        │
│                          │  │                          │
│  20 savol · 25 daqiqa    │  │  50 savol · 50 daqiqa    │
│  2 xatogacha ruxsat      │  │  4 xatogacha ruxsat      │
│  3-xatoda to'xtaydi      │  │  5-xatoda to'xtaydi      │
│                          │  │                          │
│  Birinchi marta prava    │  │  Pravadan mahrum bo'lib  │
│  olayotganlar uchun      │  │  qayta o'qiyotganlar     │
│                          │  │  uchun                   │
│  [ Boshlash ]            │  │  [ Boshlash ]            │
└──────────────────────────┘  └──────────────────────────┘
```

- **Desktop:** yonma-yon, teng kenglikda.
- **Mobil:** ustma-ust, to'liq kenglikda, har bir tegish maydoni ≥56px.
- **Kiosk:** desktop tartibi, lekin tugmalar kattaroq (sensorli ekran).
- **Klaviatura:** `1`/`2` darrov boshlaydi; `←`/`→` fokusni ko'chiradi,
  `Enter` tanlaydi. Kiosk klaviaturasi va sichqonchasiz ish uchun.
- **VIP qulfi:** obuna faol bo'lmasa kartada qulf belgisi va `/premium` ga
  yo'naltirish — bu mavjud `vip_required` oqimining ekran versiyasi, ya'ni
  foydalanuvchi 402 xatosiga urilgandan keyin emas, undan oldin ko'radi.
  Kioskda `/premium` yo'q, shuning uchun u yerda qulf ko'rsatilmaydi
  (stansiya profili doim litsenziyalangan).

### 4.3. Tanlovdan keyin

`/[locale]/session/start?mode=exam&count=20` yoki `count=50` ga o'tadi —
mavjud oqim, hech qanday yangi start mantiqi yozilmaydi.

### 4.4. Kirish nuqtalari yangilanadi

| Fayl | Nima o'zgaradi |
|---|---|
| `components/layout/sidebar.tsx:88,141` | `session/start?mode=exam` → `/exam` |
| `app/[locale]/(app)/dashboard/page.tsx:200` | "keyingi qadam" tavsiyasi → `/exam` |
| `app/[locale]/(app)/dashboard/page.tsx:563` | imtihon kartasi → `/exam` |
| `app/[locale]/(kiosk)/station/page.tsx:51` | `station/session/start?mode=exam` → `station/exam` |

**Orqaga moslik:** `?mode=exam` `count` siz ham ishlashda davom etadi va 20-lik
beradi. Ya'ni saqlangan bookmarklar, Telegram botidagi eski havolalar va
kioskdagi keshlangan sahifalar buzilmaydi.

### 4.5. `PROTECTED_SEGMENTS` ga qo'shish

`lib/protected-segments.ts` — proxy shu ro'yxat asosida cookiesiz
foydalanuvchini `/login` ga qaytaradi. `"exam"` qatori qo'shilishi **shart**:
aks holda tizimga kirmagan odam tanlov ekranini ko'radi, "Boshlash" ni bosadi
va faqat o'shanda login'ga uloqtiriladi.

Kiosk manzili bilan to'qnashuv yo'q: `matchesAny` faqat `/exam` va `/exam/...`
ga mos keladi, `/station/exam` esa ikkalasiga ham tushmaydi. Buni
`kiosk-path.test.tsx` allaqachon tekshiradi — u har bir kiosk manzili
`PROTECTED_SEGMENTS` dan tashqarida qolishini talab qiladi.

## 5. Frontend: avto-o'tish

### 5.1. Qayerda

`app/[locale]/(session)/session/[id]/page.tsx` — **bitta joyda**. Bu sahifa
`currentIndex` ni saqlaydi va uni ikkala UI'ga ham uzatadi: imtihon UI'siga
(`OfficialAvtotestExamView`) prop sifatida, oddiy UI'ga esa to'g'ridan-to'g'ri.
Kiosk va mobil aynan shu sahifani qayta ishlatadi. Demak bitta o'zgarish —
uchala platforma, ikkala UI.

### 5.2. Qoida

Javob muvaffaqiyatli yozilgach (`response.recorded === true`), **900 ms**
kutiladi, keyin keyingi javobsiz savolga o'tiladi:

1. Joriy savoldan **keyin** javobsiz savol bormi → o'shanga.
2. Yo'q bo'lsa, **oldinda** javobsiz savol bormi → birinchisiga.
3. Umuman javobsiz savol qolmagan bo'lsa → joyida qoladi. Bu holda mavjud
   **avto-yakunlash** effekti (`page.tsx:330-340`) ishga tushadi va sessiyani
   o'zi tugatadi.

To'g'ri javobda ham, xato javobda ham o'tadi — 900 ms davomida yashil yoki
qizil belgi ko'rinib turadi.

### 5.3. Bekor qilish shartlari

Kutish taymeri quyidagi hollarda bekor qilinadi (va boshqa o'rnatilmaydi):

- Foydalanuvchi o'zi boshqa savolga o'tsa (raqam-pill, `←`/`→`, "Keyingi").
- Izoh oynasi (`ExplanationDialog`) ochilsa — mashq rejimida izoh o'qilayotgan
  bo'lsa, ekran o'g'irlanmasligi kerak.
- Javob sessiyani to'xtatgan bo'lsa (`response.stopped` — 3- yoki 5-xato).
- Sessiya tugagan bo'lsa yoki komponent yechilsa.

Javob berishning o'zi muvaffaqiyatsiz bo'lsa (`response === null`, tarmoq
xatosi) taymer umuman o'rnatilmaydi — ekranda qayta urinish tugmasi qoladi.

### 5.4. Qamrov

Hamma rejimda: `exam`, `grand_mock`, `placement`, `variant`, `practice`,
`mistakes`, `review`. Mashq rejimlarida izoh o'qish imkoni 5.3-dagi izoh-oynasi
sharti bilan saqlanadi.

## 6. i18n

Uchala tilga (`uz-Latn`, `uz-Cyrl`, `ru`) yangi `ExamPicker` bo'limi:
sarlavha, ikkala kartaning nomi/tavsifi/meta qatorlari, "Boshlash", VIP qulfi
matni, klaviatura maslahati.

Mavjud kalitlar yangilanadi, chunki ular hozir "20 savol" deb qat'iy aytadi va
endi ikki tur bor:

| Kalit | Hozir (uz-Latn) |
|---|---|
| `Dashboard.examMeta` | `20 savol / 25 daqiqa` |
| `Dashboard.navExamDesc` | `20 savol, 25 daqiqa, 3-xatoda to'xtash` |

Ikkalasi ham imtihon turini tanlashga taklif qiladigan matnga o'zgaradi.

## 7. Testlar

**Go (`backend/internal/session/`)**

- `ExamConfigFor`: 20 → oddiy; 50 → qayta topshirish; 0 → 20 ga tushadi;
  3, 30, 100, manfiy → rad etiladi.
- `EvaluateExam` 4 xatoli budjet bilan: 46/50 o'tadi, 45/50 yiqiladi,
  47/50 o'tadi (1.2-dagi regressiyaning aynan o'zi).
- `StartSession` `count=50` bilan: 50 ta savol, `time_limit_sec = 3000`,
  `errors_allowed = 4` yozilishi.
- `StartSession` `count=30` bilan: 400 `invalid_request`.
- 50-lik sessiyada 5-xatoda `Stopped` qaytishi, 4-xatoda esa davom etishi.
- `errors_allowed` `POST /sessions` va `GET /sessions/{id}` javoblarida
  ko'rinishi.

**Vitest (`frontend/`)**

- Tanlov ekrani: ikkala karta chiqadi; 20-lik `count=20` bilan, 50-lik
  `count=50` bilan to'g'ri manzilga yo'naltiradi; `1`/`2` tugmalari ishlaydi.
- Avto-o'tish (fake timer): 900 ms dan keyin keyingi javobsiz savolga o'tadi;
  qo'lda navigatsiyada bekor bo'ladi; izoh oynasi ochiq bo'lsa o'tmaydi;
  `stopped` javobda o'tmaydi; oxirgi javobsiz savoldan keyin joyida qoladi.
- Xato hisoblagichi: 50-lik sessiyada "0/4" ko'rsatadi (hozirgi qattiq
  yozilgan "0/2" emas).
- `kiosk-path.test.tsx` kengaytiriladi: `/station/exam` dagi barcha manzillar
  `/station/...` ostida qolishi.

## 8. Xavflar

**8.1. Savol bazasi 50 tagacha yetmasa.** Bazada 1231 ta tasdiqlangan savol
bor, ya'ni amalda xavf yo'q. Lekin `StartSession` allaqachon
`len(ids) < cfg.QuestionCount` tekshiruvini qiladi va yarim-imtihon ochish
o'rniga 400 qaytaradi — bu xatti-harakat saqlanadi.

**8.2. Eski sessiyalar.** `errors_allowed` NULL bo'lgan eski qatorlar 2 ga
tushadi (3.3-dagi fallback). Migratsiya kerak emas.

**8.3. 50 ta raqam-pill desktopda sig'maydi.** Imtihon UI'sining pastki
panelida raqamlar gorizontal ro'yxatda turadi. Mobilda `overflow-x-auto` va
joriy pillni `scrollIntoView` qilish allaqachon bor, ya'ni 50 ta ham ishlaydi.
Desktopda esa bunday himoya yo'q: 50 × (32px pill + 6px oraliq) ≈ 1900px, ustiga
"Yakunlash" tugmasi, xato hisoblagichi va taymer — 1920px ekranga ham
sig'maydi, ya'ni panel siqilib yoki toshib ketadi.

Qaror: `overflow-x-auto` va `scrollbar-none` **barcha o'lchamlarda** yoqiladi
(hozirgi `max-lg:` prefiksi olib tashlanadi). 20-lik imtihonda ko'rinish
o'zgarmaydi, chunki 20 ta pill baribir sig'adi va scroll ko'rinmaydi.

## 9. Ish tartibi

1. Backend qoidalari: `ExamConfigFor`, parametrli `EvaluateExam`, chaqiruvchi
   tomonlarni tuzatish + testlar.
2. `StartSession` `case "exam"` + `errors_allowed` API maydoni + testlar.
3. Frontend: `SessionState.errors_allowed` va imtihon UI'sidagi qattiq
   yozilgan "2" ni almashtirish.
4. Avto-o'tish + testlar.
5. Tanlov ekrani (umumiy komponent + web va kiosk sahifalari) + testlar.
6. Kirish nuqtalarini yangilash + i18n (3 til).
7. `make check`, e2e va kioskda qo'lda ko'rib chiqish.

1–2 va 4 bir-biridan mustaqil; 3 esa 2 ga bog'liq; 5–6 esa 1–2 tugagach
ma'noli bo'ladi.
