# M2-07: GRAND MOCK (Bosh Imtihon Simulyatsiyasi) Design Spec

> **YANGILANISH (2026-07-25, ikkinchi audit raundi).** Quyidagi eligibility dizayni **ikki nuqtada o'zgardi**; kod haqiqat manbai, bu hujjatning §1/§2/§3.3 qismlari shu o'zgarishlarga moslandi:
>
> 1. **Hajm chegarasi qo'shildi.** "mastery ≥85%" yolg'iz yetarli emas edi: `ReadinessPct` — o'rtacha ko'rsatkich, shuning uchun **har kategoriyadan bittagina savolga** to'g'ri javob berib 100% "tayyorlik" chiqarish mumkin edi (1200+ savollik bankdan 12 tasi). Endi ikkinchi shart bor: savol bankining kamida `grand_mock_min_studied_pct` (25%, migratsiya `0018` bilan `limit_config`ga qo'shilgan) qismi o'rganilgan bo'lishi kerak. Foizda saqlanadi — bank o'sganda chegara o'zi o'sadi.
> 2. **VIP-to'siq alohida status kodini oldi.** Avval VIP bo'lmagan foydalanuvchi ham `403 mock_not_eligible` olardi, ya'ni "o'qishing yetmadi" — to'lash kerakligi hech qayerda ko'rinmasdi. Endi VIP bloklovchi sabab bo'lsa `402 vip_required` (frontend `/premium`ga yo'naltiradi), `403 mock_not_eligible` esa faqat o'qish-talablariga qoladi.
>
> Shu bilan birga `StartSession` eligibility mantiqini takrorlamaydi — `MockEligibility`ning o'zini chaqiradi, ya'ni UI va start bir xil qoidani ishlatadi (avval ikki joyda alohida yozilgan edi va ular ajralib ketishi mumkin edi).

> **QAYTA YOZILDI (audit natijasida).** Avvalgi versiya GRAND MOCK'ni alohida `mock` paket/endpoint sifatida loyihalagan edi. Audit paytida aniqlandiki, mavjud `session` dvigateli (backend) GRAND MOCK'ni **M1'dan boshlab kutib turgan**: `exam_session.mode` CHECK constraint'ida `'grand_mock'` allaqachon ro'yxatda (migratsiya 0004), `limit_config`'da `grand_mock_threshold_pct = 85` allaqachon urug'langan (migratsiya 0003), va frontend'ning `SessionMode` turi ham xuddi shu kengaytirishni kutmoqda. Demak GRAND MOCK — **yangi subsystem emas, balki mavjud "exam" rejimining eligibility-gate + mukofot-teatri bilan boyitilgan varianti**. Quyidagi dizayn shuni aniq talab qiladi: mavjud kodni takrorlamaslik, faqat kengaytirish.

## 1. Umumiy Ko'rinish va Maqsad
GRAND MOCK — AvtoTest platformasidagi eng mas'uliyatli imtihon rejimi, DYXX shartlarini 100% aks ettiradi. Kirish huquqi uchun uchta shart: **VIP obunasi faol**, savol bankining kamida **25%i o'rganilgan**, va **o'rtacha bilim ko'rsatgichi ≥85%** (`mastery_percent`). Uchinchi shart yolg'iz yetarli emas — nima uchun emasligi §2'da.

## 2. Biznes Qoidalari (mavjud "exam" qoidalari bilan BAB — bir xil)
`backend/internal/session/rules.go`dagi konstantalar («exam» rejimi uchun M1'da yozilgan) GRAND MOCK uchun ham **aynan shu holicha** qo'llaniladi — yangi konstanta kerak emas:
- `ExamQuestionCount = 20` (barcha kategoriyalardan tasodifiy).
- `ExamTimeLimitSec = 25*60`.
- `ExamErrorsAllowed = 2` (3-xatoda `ShouldStopExam` orqali darhol to'xtaydi — "Yiqildi").
- `EvaluateExam(correct, wrong, total, timedOut, tooManyErrors)` — pass/fail hisoblash logikasi.

**Kirish sharti (yangi, faqat GRAND MOCK uchun)** — tartib muhim, chunki qaytariladigan `reason` shu tartibda birinchi bajarilmagan shartga tegishli:
1. `is_vip == true` — `billing.Service.Status(ctx, profileID)` orqali (allaqachon "exam" rejimi xuddi shu tekshiruvni qiladi, `service.go:161`). Bajarilmasa: `reason="vip_required"`, start `402`.
2. **Hajm**: o'rganilgan alohida savollar soni ≥ `grand_mock_min_studied_pct` × (bankdagi valid savollar soni). Bajarilmasa: `reason="too_few_studied"`, start `403`.
3. `mastery_percent >= grand_mock_threshold_pct` (85) — `learning.Service.Stats(ctx, profileID)` orqali (`internal/learning/service.go:184`, allaqachon mavjud). Bajarilmasa: `reason="mastery_too_low"`, start `403`.

Ikkala chegara ham `limit_config`dan o'qiladi; `rules.go`dagi konstantalar faqat hujjat/zaxira qiymat, **haqiqat manbai DB**.

## 3. Backend — Mavjud `session` paketini kengaytirish (YANGI paket EMAS)

`session.Service` allaqachon `Learning *learning.Service` va `Billing billing.Service` fieldlariga ega (`NewService(q, b, l, p)`, `service.go:34,41`) — eligibility tekshiruvi uchun hech qanday yangi wiring kerak emas.

### 3.1 `StartSession`ga yangi mode (`service.go:139`dagi `switch req.Mode`ga qo'shiladi)
```go
case "grand_mock":
    // Eligibility mantiqi bu yerda TAKRORLANMAYDI: /me/mock-eligibility
    // bilan aynan bir manbadan o'qiladi, aks holda UI "ochiq" deb
    // ko'rsatib, start rad etishi (yoki teskarisi) mumkin bo'lardi.
    elig, eligErr := s.MockEligibility(ctx, profileID)
    if eligErr != nil {
        return SessionView{}, eligErr
    }
    if !elig.Eligible {
        // VIP — foydalanuvchi HOZIR hal qila oladigan yagona to'siq, shuning
        // uchun u alohida status kodini oladi (402 -> /premium upsell).
        if elig.Reason == MockReasonVIPRequired {
            return SessionView{}, ErrRequiresVIP
        }
        return SessionView{}, ErrMockNotEligible
    }
    ids, err = s.Q.RandomQuestionIDs(ctx, int32(ExamQuestionCount))
    if err == nil && len(ids) < ExamQuestionCount {
        return SessionView{}, ErrInvalidRequest
    }
    timeLimit = pgtype.Int4{Int32: ExamTimeLimitSec, Valid: true}
    errorsAllowed = pgtype.Int4{Int32: ExamErrorsAllowed, Valid: true}
```
(Amalda "exam" case'iga deyarli bir xil — faqat VIP-tekshiruv o'rniga eligibility-tekshiruv.)

Sentinel xatolar: `ErrMockNotEligible` → 403 `mock_not_eligible` (o'qish talablari), mavjud `ErrRequiresVIP` → 402 `vip_required` (obuna). Xabar matni ataylab ingliz tilida va umumiy — mijoz `reason` kodi va aniq raqamlarni `/me/mock-eligibility`dan oladi va o'zi lokalizatsiya qiladi.

**`POST /sessions` allaqachon `mode` qabul qiladi — yangi start-endpoint SHART EMAS.** Frontend shunchaki `mode: "grand_mock"` yuboradi, xuddi `exam`/`practice` kabi.

### 3.2 Boshqa 6 joyda `row.Mode == "exam"`ni `grand_mock`ga ham tarqatish
Bularning barchasi GRAND MOCK uchun "exam" bilan **bir xil xatti-harakat** talab qiladi (vaqt-tugashi, javob-yashirish anti-cheat, EvaluateExam scoring):
- `service.go:374` — vaqt tugashini majburiy tekshirish (`SubmitAnswer`).
- `service.go:410` — javob paytida izohni yashirish (redaction).
- `service.go:445` — darhol to'g'ri/xato feedback ko'rsatish.
- `service.go:525` — `SessionQuestionAccess.FeedbackAllowed` faqat tugagach.
- `service.go:569` — `FinishSession`da vaqt-tugashini hisoblash.
- `service.go:598` — `finishInternal`dagi `switch row.Mode { case "exam": ... }` — `EvaluateExam` chaqiruvi.
- `service.go:680` — `GetSession`dagi `redact` (in-progress paytida javoblarni yashirish).

**Tavsiya**: yangi kichik helper qo'shish, masalan `isExamLike(mode string) bool { return mode == "exam" || mode == "grand_mock" }`, va yuqoridagi 7 joyning barchasida `row.Mode == "exam"` o'rniga `isExamLike(row.Mode)` ishlatish — bitta joyda markazlashtirilgan, kelajakda uchinchi "exam-uslubidagi" rejim qo'shilsa ham oson kengayadi. `finishInternal`ning `switch`idagi `case "exam":` esa `case "exam", "grand_mock":`ga aylanadi (bu yerda `isExamLike` ishlatib bo'lmaydi, switch-case sintaksisi).

### 3.3 Yangi endpoint: eligibility ko'rish (UI uchun, faqat o'qish)
- **Endpoint**: `GET /api/v1/me/mock-eligibility` (`session.Handler.Routes`ga qo'shiladi, `handlers.go:25-34` ro'yxatiga).
- **Auth**: Required.
- **Response**: `{"eligible": bool, "mastery_percent": int, "min_required_percent": int, "questions_studied": int, "min_required_questions": int, "is_vip": bool, "reason": "vip_required"|"too_few_studied"|"mastery_too_low"|null}`. `min_required_questions` — foizdan hisoblangan **absolyut** son, chunki UI foydalanuvchiga "300 tadan 12 tasi" deb ko'rsatishi kerak, "25%" emas.
- Implementatsiya: yangi `Service.MockEligibility(ctx, profileID)` metodi — `s.Learning.Stats` + `s.Billing.Status`ni chaqiradi, taqqoslaydi, DTO qaytaradi. Yangi DB so'rov kerak emas.

### 3.4 Konfetti/sertifikat uchun signal
`FinishResult`da GRAND MOCK bo'lib `status == "passed"` bo'lgan holatni frontend ajrata olishi kerak — bu allaqachon mavjud `mode` va `status` fieldlaridan olinadi (`FinishResult`ga qarang, `service.go`), yangi field shart emas.

## 4. Frontend — mavjud sessiya dvigatelini kengaytirish

- `frontend/src/hooks/use-session-engine.ts`: `SessionMode` turiga `"grand_mock"` qo'shiladi (`"variant" | "exam" | "practice" | "mistakes" | "grand_mock"`).
- `frontend/src/app/[locale]/(session)/session/[id]/page.tsx`: mavjud `isExam = session.mode === "exam"` (163-qator atrofi) o'rniga `isExamLike = session.mode === "exam" || session.mode === "grand_mock"` — timer/anti-cheat UI GRAND MOCK uchun ham avtomatik ishlaydi. `session.mode === "grand_mock" && status === "passed"` holatida qo'shimcha: `canvas-confetti` + sertifikat dialogi (yangi, faqat shu holat uchun).
- **YANGI** `GrandMockCard` komponenti (`frontend/src/components/mock/grand-mock-card.tsx` — audit vaqtida topilgan avvalgi reja shu yo'lni to'g'ri tanlagan): `GET /me/mock-eligibility`ni chaqiradi; qulflangan holatda progress-bar + qulf belgisi; ochilgan holatda "Bosh Imtihonni Boshlash" tugmasi — bu tugma **mavjud session-start hook/oqimini** (`POST /sessions {mode: "grand_mock"}`) chaqiradi, YANGI start-oqimi qurmaydi.
  - Progress-bar **bloklovchi** shartni kuzatadi, mastery'ni emas: hajm sharti bajarilmagan bo'lsa `12 / 300 savol` ko'rsatiladi. Aks holda 100% mastery bilan qulflangan holatda bar to'la turib, foydalanuvchiga nima qilishi kerakligini aytmasdi.
  - `reason === "vip_required"` bo'lsa qo'shimcha **CTA tugmasi** `/premium`ga — bu foydalanuvchi shu zahoti hal qila oladigan yagona to'siq. Boshqa sabablarda tugma ko'rsatilmaydi (o'qishni "sotib olib" bo'lmaydi).
- Dashboard/Practice sahifasiga `GrandMockCard` joylashtiriladi (oltin VIP gradient, spec §4.1 talabiga mos).
- `session/start` sahifasi `mock_not_eligible`ni o'z matni + dashboard'ga qaytish tugmasi bilan ko'rsatadi (generic "xatolik" emas — foydalanuvchi qancha qolganini kartadan ko'radi).

## 5. Anti-Fraud eslatma
Eligibility — server tomonda `StartSession`ning o'zida tekshiriladi (client faqat UI-holat uchun `/me/mock-eligibility`dan foydalanadi) — shuning uchun frontend tekshiruvini chetlab o'tib to'g'ridan-to'g'ri `POST /sessions {mode:"grand_mock"}` chaqirish ham xavfsiz (402/403 qaytaradi agar shart bajarilmasa).

Hajm chegarasi (§2, shart 2) — aynan anti-fraud/anti-gaming chorasi: mastery yolg'iz o'ynab bo'ladigan ko'rsatkich edi (har kategoriyadan 1 savol → 100%).

## 6. Verifikatsiya
- Backend: mavjud `internal/session` test faylida (yoki yangi `mock_test.go`da, lekin **`session` paketining ichida**, alohida paket emas) — har uch shart uchun rad etish, eligibility o'tish + session yaratilishi (mode=grand_mock, 20 savol, 25 min), 3-xatoda to'xtash, `EvaluateExam` orqali pass/fail (mavjud exam-testlarining aynan retseptidan foydalanib, faqat mode="grand_mock").
  - Muhim: "mastery past" va "hajm kam" holatlarini **ajratib** test qilish kerak, aks holda bitta test ikkalasini ham qoniqtirib, hajm chegarasi ishlayotganini isbotlamaydi. Shuning uchun `studyQuestions` (ko'p savol, past reyting → hajm yetarli, mastery past) va `studyOnePerCategoryCorrectly` (aynan exploit: 100% mastery, hajm juda kam) helperlari alohida yozilgan.
- Frontend: `grand-mock-card.test.tsx` (qulflangan/ochiq holatlar, uchta `reason`, VIP CTA bor/yo'qligi, `min = 0` da nolga bo'linish bo'lmasligi), `/session/[id]` sahifasining mavjud testiga grand_mock uchun bitta muvaffaqiyatli-parcourse testi.
