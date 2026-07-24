# M2-07: GRAND MOCK (Bosh Imtihon Simulyatsiyasi) Design Spec

> **QAYTA YOZILDI (audit natijasida).** Avvalgi versiya GRAND MOCK'ni alohida `mock` paket/endpoint sifatida loyihalagan edi. Audit paytida aniqlandiki, mavjud `session` dvigateli (backend) GRAND MOCK'ni **M1'dan boshlab kutib turgan**: `exam_session.mode` CHECK constraint'ida `'grand_mock'` allaqachon ro'yxatda (migratsiya 0004), `limit_config`'da `grand_mock_threshold_pct = 85` allaqachon urug'langan (migratsiya 0003), va frontend'ning `SessionMode` turi ham xuddi shu kengaytirishni kutmoqda. Demak GRAND MOCK — **yangi subsystem emas, balki mavjud "exam" rejimining eligibility-gate + mukofot-teatri bilan boyitilgan varianti**. Quyidagi dizayn shuni aniq talab qiladi: mavjud kodni takrorlamaslik, faqat kengaytirish.

## 1. Umumiy Ko'rinish va Maqsad
GRAND MOCK — AvtoTest platformasidagi eng mas'uliyatli imtihon rejimi, DYXX shartlarini 100% aks ettiradi. Faqat **o'rtacha bilim ko'rsatgichi ≥85%** (`mastery_percent`) hamda **VIP obunasi faol** bo'lgan foydalanuvchilar kirish huquqiga ega.

## 2. Biznes Qoidalari (mavjud "exam" qoidalari bilan BAB — bir xil)
`backend/internal/session/rules.go`dagi konstantalar («exam» rejimi uchun M1'da yozilgan) GRAND MOCK uchun ham **aynan shu holicha** qo'llaniladi — yangi konstanta kerak emas:
- `ExamQuestionCount = 20` (barcha kategoriyalardan tasodifiy).
- `ExamTimeLimitSec = 25*60`.
- `ExamErrorsAllowed = 2` (3-xatoda `ShouldStopExam` orqali darhol to'xtaydi — "Yiqildi").
- `EvaluateExam(correct, wrong, total, timedOut, tooManyErrors)` — pass/fail hisoblash logikasi.

**Kirish sharti (yangi, faqat GRAND MOCK uchun)**:
- `mastery_percent >= 85` — `learning.Service.Stats(ctx, profileID)` orqali hisoblanadi (`internal/learning/service.go:184`, allaqachon mavjud, o'zgartirish shart emas — faqat chaqiriladi).
- `is_vip == true` — `billing.Service.Status(ctx, profileID)` orqali (allaqachon "exam" rejimi xuddi shu tekshiruvni qiladi, `service.go:161`).

## 3. Backend — Mavjud `session` paketini kengaytirish (YANGI paket EMAS)

`session.Service` allaqachon `Learning *learning.Service` va `Billing billing.Service` fieldlariga ega (`NewService(q, b, l, p)`, `service.go:34,41`) — eligibility tekshiruvi uchun hech qanday yangi wiring kerak emas.

### 3.1 `StartSession`ga yangi mode (`service.go:139`dagi `switch req.Mode`ga qo'shiladi)
```go
case "grand_mock":
    active, _, statusErr := s.Billing.Status(ctx, profileID)
    if statusErr != nil {
        return SessionView{}, statusErr
    }
    if !active {
        return SessionView{}, ErrMockNotEligible // VIP ham, alohida ErrRequiresVIP emas — sabab bitta xabarda ko'rsatiladi
    }
    stats, statsErr := s.Learning.Stats(ctx, profileID)
    if statsErr != nil {
        return SessionView{}, statsErr
    }
    if stats.ReadinessPct < MockMasteryThreshold { // ReadinessPct is int (0-100); threshold = 85, rules.go'da konstanta
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

Yangi sentinel xato (`service.go:22` blokiga qo'shiladi): `ErrMockNotEligible = errors.New("grand mock requires 85% mastery and active VIP")`. `writeSessionError` (`handlers.go:464`)ga yangi case: 403, code `mock_not_eligible`.

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
- **Response**: `{"eligible": bool, "mastery_percent": float64, "min_required_percent": 85.0, "is_vip": bool, "reason": "mastery_too_low"|"vip_required"|null}`.
- Implementatsiya: yangi `Service.MockEligibility(ctx, profileID)` metodi — `s.Learning.Stats` + `s.Billing.Status`ni chaqiradi, taqqoslaydi, DTO qaytaradi. Yangi DB so'rov kerak emas.

### 3.4 Konfetti/sertifikat uchun signal
`FinishResult`da GRAND MOCK bo'lib `status == "passed"` bo'lgan holatni frontend ajrata olishi kerak — bu allaqachon mavjud `mode` va `status` fieldlaridan olinadi (`FinishResult`ga qarang, `service.go`), yangi field shart emas.

## 4. Frontend — mavjud sessiya dvigatelini kengaytirish

- `frontend/src/hooks/use-session-engine.ts`: `SessionMode` turiga `"grand_mock"` qo'shiladi (`"variant" | "exam" | "practice" | "mistakes" | "grand_mock"`).
- `frontend/src/app/[locale]/(session)/session/[id]/page.tsx`: mavjud `isExam = session.mode === "exam"` (163-qator atrofi) o'rniga `isExamLike = session.mode === "exam" || session.mode === "grand_mock"` — timer/anti-cheat UI GRAND MOCK uchun ham avtomatik ishlaydi. `session.mode === "grand_mock" && status === "passed"` holatida qo'shimcha: `canvas-confetti` + sertifikat dialogi (yangi, faqat shu holat uchun).
- **YANGI** `GrandMockCard` komponenti (`frontend/src/components/mock/grand-mock-card.tsx` — audit vaqtida topilgan avvalgi reja shu yo'lni to'g'ri tanlagan): `GET /me/mock-eligibility`ni chaqiradi; qulflangan holatda progress-bar (`72% / 85%`) + qulf belgisi; ochilgan holatda "Bosh Imtihonni Boshlash" tugmasi — bu tugma **mavjud session-start hook/oqimini** (`POST /sessions {mode: "grand_mock"}`) chaqiradi, YANGI start-oqimi qurmaydi.
- Dashboard/Practice sahifasiga `GrandMockCard` joylashtiriladi (oltin VIP gradient, spec §4.1 talabiga mos).

## 5. Anti-Fraud eslatma
Eligibility (mastery+VIP) — server tomonda `StartSession`ning o'zida tekshiriladi (client faqat UI-holat uchun `/me/mock-eligibility`dan foydalanadi) — shuning uchun frontend tekshiruvini chetlab o'tib to'g'ridan-to'g'ri `POST /sessions {mode:"grand_mock"}` chaqirish ham xavfsiz (403 qaytaradi agar shart bajarilmasa).

## 6. Verifikatsiya
- Backend: mavjud `internal/session` test faylida (yoki yangi `mock_test.go`da, lekin **`session` paketining ichida**, alohida paket emas) — eligibility rad etish (mastery<85, VIP yo'q), eligibility o'tish + session yaratilishi (mode=grand_mock, 20 savol, 25 min), 3-xatoda to'xtash, `EvaluateExam` orqali pass/fail (mavjud exam-testlarining aynan retseptidan foydalanib, faqat mode="grand_mock").
- Frontend: `grand-mock-card.test.tsx` (qulflangan/ochiq holatlar), `/session/[id]` sahifasining mavjud testiga grand_mock uchun bitta muvaffaqiyatli-parcourse testi qo'shiladi.
