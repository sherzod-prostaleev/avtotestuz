# Imtihon 20/50 rejimlari va avto-o'tish — implementatsiya rejasi

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Imtihon simulyatsiyasiga 50 savolli "qayta topshirish" turini, undan oldin chiqadigan rejim tanlash ekranini va javobdan keyingi avto-o'tishni qo'shish — web, mobil va kioskda.

**Architecture:** 50-lik alohida `mode` emas: `mode="exam"` + `count` parametri, server tomonda whitelist orqali qoidalar to'plamiga (`ExamConfig`) o'giriladi va sessiya qatoridagi mavjud `total` / `time_limit_sec` / `errors_allowed` ustunlariga yoziladi. Frontendda tanlov ekrani umumiy komponent bo'lib, web va kiosk sahifalari uni `kiosk` prop bilan qayta ishlatadi (loyihaning mavjud kiosk naqshi). Avto-o'tish `session/[id]/page.tsx` da bir joyda amalga oshiriladi, chunki ikkala UI ham (`OfficialAvtotestExamView` va oddiy UI) `currentIndex` ni shu sahifadan oladi.

**Tech Stack:** Go 1.26 (chi, pgx, sqlc), Next.js 15 App Router + TypeScript + Tailwind, next-intl, Vitest + Testing Library, Playwright.

**Spec:** `docs/superpowers/specs/2026-08-22-exam-20-50-autoadvance-design.md`

## Global Constraints

- **Mashinada `go` o'rnatilmagan.** Barcha Go buyruqlari Docker orqali (quyida har qadamda to'liq buyruq berilgan). `make check` / `make test` to'g'ridan-to'g'ri ishlamaydi.
- **Docker image `golang:1.27` bo'lishi shart.** `golang:1.26` ichida Go 1.26.5 keladi, `backend/go.mod` esa `go 1.26.7` talab qiladi — natijada har buyruq `go.mod requires go >= 1.26.7` bilan yiqiladi.
- **Postgres va redis ishlab turishi kerak:** `docker compose up -d postgres redis` (hozir ishlab turibdi).
- **Backend to'liq test to'plami ~10 daqiqadan uzoq.** Har qadamda faqat kerakli paket/test yugurtiriladi; to'liq to'plam faqat oxirida va `run_in_background: true` bilan.
- **Imtihon qoidalari kodda, o'zgarmas:** oddiy = 20 savol / 1500 s / 2 xato; qayta topshirish = 50 savol / 3000 s / 4 xato. `limit_config` ga chiqarilmaydi.
- **Avto-o'tish kechikishi: 900 ms**, barcha rejimlarda, sozlanmaydi.
- **Uchala til majburiy:** `frontend/messages/uz-Latn.json`, `uz-Cyrl.json`, `ru.json` — yangi kalit uchalasiga ham qo'shiladi, aks holda `next-intl` ishlab turgan tilda kalit nomini ko'rsatadi.
- **Orqaga moslik:** `?mode=exam` `count` siz 20-lik berishda davom etadi.
- **Fixture o'zgartirilmaydi:** `fixture.Sample()` 40 savolda qoladi (`internal/importer/store_test.go:27,36,55` aynan 40/160 ni tekshiradi). 50-lik testlar uchun yangi `fixture.SampleSized(n)` qo'shiladi.

---

## Fayllar tuzilishi

**Backend — yangi:** yo'q (mavjud fayllarga qo'shiladi)

| Fayl | Mas'uliyat |
|---|---|
| `backend/internal/session/rules.go` | `ExamConfig`, `ExamConfigFor`, parametrli `EvaluateExam` |
| `backend/internal/session/service.go` | `StartSession` `case "exam"`, `finishInternal` chaqiruvi, `GetSession` DTO |
| `backend/internal/session/dto.go` | `SessionView.ErrorsAllowed` |
| `backend/internal/session/handlers.go` | JSON javoblarga `errors_allowed` |
| `backend/internal/fixture/fixture.go` | `SampleSized(n)` — 50+ savol talab qiladigan testlar uchun |

**Frontend — yangi:**

| Fayl | Mas'uliyat |
|---|---|
| `frontend/src/lib/session-navigation.ts` | `hasAnswer`, `nextUnansweredIndex`, `AUTO_ADVANCE_MS` — sof mantiq, UI'siz |
| `frontend/src/lib/session-navigation.test.ts` | shu mantiqning testlari |
| `frontend/src/components/exam/exam-mode-picker.tsx` | 20/50 tanlash ekrani (web + kiosk umumiy) |
| `frontend/src/components/exam/exam-mode-picker.test.tsx` | tanlov ekranining testlari |
| `frontend/src/app/[locale]/(app)/exam/page.tsx` | `/[locale]/exam` — o'quvchi marshruti |
| `frontend/src/app/[locale]/(kiosk)/station/exam/page.tsx` | `/[locale]/station/exam` — kiosk marshruti |

**Frontend — o'zgaradi:**

| Fayl | Nima |
|---|---|
| `frontend/src/hooks/use-session-engine.ts` | `SessionState.errors_allowed` |
| `frontend/src/components/exam/official-avtotest-exam-view.tsx` | HUD sessiyadan o'qisin; pill scroll barcha o'lchamlarda |
| `frontend/src/app/[locale]/(session)/session/[id]/page.tsx` | avto-o'tish; `hasAnswer` ni lib'dan import |
| `frontend/src/lib/protected-segments.ts` | `"exam"` qatori |
| `frontend/src/components/layout/sidebar.tsx` | imtihon havolalari → `/exam` |
| `frontend/src/app/[locale]/(app)/dashboard/page.tsx` | imtihon havolalari → `/exam` |
| `frontend/src/app/[locale]/(kiosk)/station/page.tsx` | imtihon havolasi → `station/exam` |
| `frontend/messages/{uz-Latn,uz-Cyrl,ru}.json` | `ExamPicker` bo'limi + `Dashboard` matnlari |

---

## Task 1: Imtihon qoidalari jadvali va parametrli baholash

`EvaluateExam` hozir paket konstantasini o'qiydi. 50-lik qo'shilishidan **oldin** bu tuzatilishi kerak, aks holda 47/50 olgan o'quvchi yiqilgan deb belgilanadi.

**Files:**
- Modify: `backend/internal/session/rules.go`
- Modify: `backend/internal/session/service.go:842-846`
- Test: `backend/internal/session/rules_test.go`

**Interfaces:**
- Produces: `session.ExamConfig{QuestionCount, TimeLimitSec, ErrorsAllowed int}`;
  `session.ExamConfigFor(count int) (ExamConfig, bool)`;
  `session.EvaluateExam(correct, wrong, total, errorsAllowed int, timedOut, tooManyErrors bool) ExamOutcome`;
  konstantalar `ExamRestoreQuestionCount = 50`, `ExamRestoreTimeLimitSec = 3000`, `ExamRestoreErrorsAllowed = 4`.
- Consumes: mavjud `ExamQuestionCount`, `ExamTimeLimitSec`, `ExamErrorsAllowed`, `ExamOutcome`.

- [ ] **Step 1: Mavjud `EvaluateExam` testlarini yangi imzoga o'tkaz va yangi testlarni yoz**

`backend/internal/session/rules_test.go` da 5 ta mavjud `TestEvaluateExam*` funksiyasi 5-argumentli imzoga o'tadi (`ExamErrorsAllowed` qo'shiladi), va oxiriga yangi testlar qo'shiladi.

Mavjud beshtasini shunday almashtir:

```go
func TestEvaluateExamCompletedPass(t *testing.T) {
	out := EvaluateExam(18, 2, 20, ExamErrorsAllowed, false, false)
	if out.Status != "passed" || out.StoppedReason != "completed" {
		t.Fatalf("18/20 with 2 wrong should pass: %+v", out)
	}
}

func TestEvaluateExamCompletedFail(t *testing.T) {
	out := EvaluateExam(17, 3, 20, ExamErrorsAllowed, false, false)
	if out.Status != "failed" || out.StoppedReason != "completed" {
		t.Fatalf("17/20 with 3 wrong should fail: %+v", out)
	}
}

func TestEvaluateExamTooManyErrors(t *testing.T) {
	out := EvaluateExam(5, 3, 20, ExamErrorsAllowed, false, true)
	if out.Status != "failed" || out.StoppedReason != "too_many_errors" {
		t.Fatalf("3rd wrong must fail immediately: %+v", out)
	}
}

func TestEvaluateExamTimeUpPass(t *testing.T) {
	out := EvaluateExam(19, 1, 20, ExamErrorsAllowed, true, false)
	if out.Status != "passed" || out.StoppedReason != "time_up" {
		t.Fatalf("time up but already 19/20 with 1 wrong should pass: %+v", out)
	}
}

func TestEvaluateExamTimeUpFail(t *testing.T) {
	out := EvaluateExam(10, 1, 20, ExamErrorsAllowed, true, false)
	if out.Status != "failed" || out.StoppedReason != "time_up" {
		t.Fatalf("time up with only 10 answered should fail: %+v", out)
	}
}
```

Faylning oxiriga qo'sh:

```go
func TestExamConfigForStandard(t *testing.T) {
	cfg, ok := ExamConfigFor(ExamQuestionCount)
	if !ok {
		t.Fatal("20 must be a valid exam size")
	}
	if cfg.QuestionCount != 20 || cfg.TimeLimitSec != 25*60 || cfg.ErrorsAllowed != 2 {
		t.Fatalf("standard exam config = %+v", cfg)
	}
}

func TestExamConfigForRestore(t *testing.T) {
	cfg, ok := ExamConfigFor(ExamRestoreQuestionCount)
	if !ok {
		t.Fatal("50 must be a valid exam size")
	}
	if cfg.QuestionCount != 50 || cfg.TimeLimitSec != 50*60 || cfg.ErrorsAllowed != 4 {
		t.Fatalf("restore exam config = %+v", cfg)
	}
}

// An absent count is how every pre-existing ?mode=exam link and saved
// bookmark arrives; it must keep meaning the standard 20-question exam.
func TestExamConfigForZeroDefaultsToStandard(t *testing.T) {
	cfg, ok := ExamConfigFor(0)
	if !ok {
		t.Fatal("an unspecified count must fall back to the standard exam")
	}
	if cfg.QuestionCount != ExamQuestionCount {
		t.Fatalf("zero count = %d questions, want %d", cfg.QuestionCount, ExamQuestionCount)
	}
}

// Only 20 and 50 are real exams. Anything else must be refused rather than
// silently honoured, or a client could ask for a 3-question "exam" and pass it.
func TestExamConfigForRejectsUnknownSizes(t *testing.T) {
	for _, count := range []int{-1, 1, 3, 19, 21, 30, 49, 51, 100, 1000} {
		if _, ok := ExamConfigFor(count); ok {
			t.Fatalf("count=%d must not be a valid exam size", count)
		}
	}
}

// The 46/50 pass bar is the whole point of the restore exam: 4 mistakes still
// pass, the 5th does not. Before errorsAllowed was a parameter this evaluated
// against the constant 2 and failed a 47/50 run that really passes.
func TestEvaluateExamRestorePassBar(t *testing.T) {
	cases := []struct {
		name    string
		correct int
		wrong   int
		want    string
	}{
		{"50/50 passes", 50, 0, "passed"},
		{"47/50 passes", 47, 3, "passed"},
		{"46/50 passes", 46, 4, "passed"},
		{"45/50 fails", 45, 5, "failed"},
		{"40/50 fails", 40, 10, "failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := EvaluateExam(tc.correct, tc.wrong, 50, ExamRestoreErrorsAllowed, false, false)
			if out.Status != tc.want {
				t.Fatalf("%d correct / %d wrong = %q, want %q", tc.correct, tc.wrong, out.Status, tc.want)
			}
		})
	}
}

func TestShouldStopForErrorsRestore(t *testing.T) {
	for _, wrong := range []int{1, 2, 3, 4} {
		if ShouldStopForErrors(wrong, ExamRestoreErrorsAllowed) {
			t.Fatalf("%d wrong must not stop the restore exam", wrong)
		}
	}
	if !ShouldStopForErrors(5, ExamRestoreErrorsAllowed) {
		t.Fatal("5th wrong must stop the restore exam")
	}
}
```

- [ ] **Step 2: Testlarni yugurtirib, kompilyatsiya xatosi bilan yiqilishini tasdiqla**

```bash
cd "/home/sher/Рабочий стол/avtotest" && docker run --rm --network host \
  -v "$PWD":/src -v avtotest-gomod:/go/pkg/mod -w /src/backend \
  golang:1.27 go test ./internal/session/ -run 'TestExamConfigFor|TestEvaluateExam|TestShouldStopForErrors' -count=1 2>&1 | tail -20
```

Kutilgan: FAIL — `undefined: ExamConfigFor`, `undefined: ExamRestoreErrorsAllowed`, va `too many arguments in call to EvaluateExam`.

- [ ] **Step 3: `rules.go` ni yoz**

`backend/internal/session/rules.go` da `const` blokiga qo'sh (mavjud `ExamErrorsAllowed = 2` dan keyin, `PlacementQuestionCount` dan oldin):

```go
	// The restore exam is what a driver who lost their licence re-sits: 50
	// questions in 50 minutes, passing at 46/50. It is the same exam pipeline
	// as the standard one, only a different rule set — see ExamConfigFor.
	ExamRestoreQuestionCount = 50
	ExamRestoreTimeLimitSec  = 50 * 60
	ExamRestoreErrorsAllowed = 4
```

`IsExamLike` dan keyin qo'sh:

```go
// ExamConfig is one exam variety's complete rule set: how many questions are
// drawn, how long the candidate has, and how many mistakes are survivable.
type ExamConfig struct {
	QuestionCount int
	TimeLimitSec  int
	ErrorsAllowed int
}

// ExamConfigFor maps a requested exam size onto its official rule set.
//
// Only 20 (standard) and 50 (restore) are real exams. 0 means "unspecified"
// and yields the standard exam, which is what keeps every pre-existing
// ?mode=exam link working. Every other size is refused: the count arrives
// from the client, so an open-ended value would let a caller request a
// 3-question "exam" and pass it trivially.
func ExamConfigFor(count int) (ExamConfig, bool) {
	switch count {
	case 0, ExamQuestionCount:
		return ExamConfig{
			QuestionCount: ExamQuestionCount,
			TimeLimitSec:  ExamTimeLimitSec,
			ErrorsAllowed: ExamErrorsAllowed,
		}, true
	case ExamRestoreQuestionCount:
		return ExamConfig{
			QuestionCount: ExamRestoreQuestionCount,
			TimeLimitSec:  ExamRestoreTimeLimitSec,
			ErrorsAllowed: ExamRestoreErrorsAllowed,
		}, true
	default:
		return ExamConfig{}, false
	}
}
```

`EvaluateExam` ni almashtir (izohi bilan birga):

```go
// EvaluateExam computes the final status of an exam-mode session. Passing
// requires correct >= total-errorsAllowed AND wrong <= errorsAllowed — i.e.
// >=18/20 on the standard exam and >=46/50 on the restore exam.
//
// errorsAllowed is a parameter rather than the ExamErrorsAllowed constant
// because the two exam varieties have different budgets and the session row
// carries its own errors_allowed. Reading the constant here while
// ShouldStopForErrors read the column would let a session stop under one rule
// and be graded under another.
func EvaluateExam(correct, wrong, total, errorsAllowed int, timedOut, tooManyErrors bool) ExamOutcome {
	if tooManyErrors {
		return ExamOutcome{Status: "failed", StoppedReason: "too_many_errors"}
	}
	reason := "completed"
	if timedOut {
		reason = "time_up"
	}
	if correct >= total-errorsAllowed && wrong <= errorsAllowed {
		return ExamOutcome{Status: "passed", StoppedReason: reason}
	}
	return ExamOutcome{Status: "failed", StoppedReason: reason}
}
```

- [ ] **Step 4: Yagona chaqiruvchini yangila**

`backend/internal/session/service.go` da `finishInternal` ichidagi `case "exam", "grand_mock":` shoxini almashtir:

```go
	case "exam", "grand_mock":
		wrong := totalAnswered - correctCount
		// Same fallback SubmitAnswer uses: sessions created before
		// errors_allowed existed carry NULL and are graded as standard exams.
		errorsAllowed := ExamErrorsAllowed
		if row.ErrorsAllowed.Valid {
			errorsAllowed = int(row.ErrorsAllowed.Int32)
		}
		outcome := EvaluateExam(correctCount, wrong, int(row.Total), errorsAllowed, timedOut, tooManyErrors)
		status = outcome.Status
		reason = outcome.StoppedReason
```

- [ ] **Step 5: Testlar o'tishini tasdiqla**

```bash
cd "/home/sher/Рабочий стол/avtotest" && docker run --rm --network host \
  -v "$PWD":/src -v avtotest-gomod:/go/pkg/mod -w /src/backend \
  golang:1.27 go test ./internal/session/ -run 'TestExamConfigFor|TestEvaluateExam|TestShouldStopForErrors' -count=1 -v 2>&1 | tail -40
```

Kutilgan: `ok  avtotest.uz/backend/internal/session` — barcha `TestExamConfigFor*`, `TestEvaluateExam*`, `TestShouldStopForErrors*` PASS.

- [ ] **Step 6: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add backend/internal/session/rules.go backend/internal/session/rules_test.go backend/internal/session/service.go && git commit -m "feat(session): make the exam pass bar follow the session's own error budget

EvaluateExam graded every exam against the ExamErrorsAllowed constant while
ShouldStopForErrors already read the session row's errors_allowed column. With
one exam variety the two agreed; adding the 50-question restore exam would
have made them disagree and failed a 47/50 run that really passes.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01MTBBDEFbFgB4hkj6u1jg7g"
```

---

## Task 2: 50 savolli imtihonni boshlash va `errors_allowed` ni API'ga chiqarish

**Files:**
- Modify: `backend/internal/fixture/fixture.go`
- Modify: `backend/internal/session/service.go:211-224` (`case "exam"`), `service.go:367-375` (`SessionView` qurilishi), `service.go:985-1000` (`GetSession` DTO)
- Modify: `backend/internal/session/dto.go:27-34`
- Modify: `backend/internal/session/handlers.go:93-100` va `:449-460`
- Test: `backend/internal/session/service_test.go`, `backend/internal/session/handlers_test.go`

**Interfaces:**
- Consumes: Task 1'ning `ExamConfigFor`, `ExamRestoreQuestionCount`, `ExamRestoreTimeLimitSec`, `ExamRestoreErrorsAllowed`.
- Produces: `fixture.SampleSized(questionCount int) (importer.Dataset, importer.MapSource)`;
  `session.SessionView.ErrorsAllowed *int`;
  JSON maydoni `"errors_allowed"` `POST /sessions` va `GET /sessions/{id}` javoblarida.

- [ ] **Step 1: `fixture.SampleSized` ni yoz**

`fixture.Sample()` 40 ta savol beradi, ya'ni 50 talik tanlov undan chiqmaydi. `Sample()` ning o'zini kattalashtirib bo'lmaydi — `internal/importer/store_test.go:27,36,55` aynan 40 va 160 raqamlarini tekshiradi.

`backend/internal/fixture/fixture.go` da `Sample` ni shu ikkitaga ajrat (mavjud tanaga tegmasdan, faqat 40 ni parametrga aylantirib):

```go
// Sample returns 40 questions (2 variants × 20), 4 categories, 2 sign groups,
// 4 signs, 1 explanation, with tiny PNG images for signs and every 4th question.
func Sample() (importer.Dataset, importer.MapSource) {
	return SampleSized(40)
}

// SampleSized is Sample with a caller-chosen question count, for tests that
// need a bank bigger than one exam draw — the 50-question restore exam cannot
// be started against Sample's 40. Variants are built in blocks of 20, so
// questionCount/20 bilets are produced and any remainder stays variant-less
// (valid: importer.Validate checks questions individually).
func SampleSized(questionCount int) (importer.Dataset, importer.MapSource) {
```

Va mavjud tananing ikki joyini parametrga bog'la:

```go
	for n := 1; n <= questionCount; n++ {
```

```go
	for v := 1; v <= questionCount/20; v++ {
```

Qolgan hamma narsa (kategoriyalar, belgilar, izoh) o'zgarishsiz qoladi.

- [ ] **Step 2: Yangi testlarni yoz**

`backend/internal/session/service_test.go` ning `seed` helperidan keyin qo'sh:

```go
// seedBig is seed with a question bank large enough to draw a 50-question
// restore exam from. fixture.Sample only carries 40 questions, and
// importer/store_test.go asserts that exact number, so the bank is widened
// here instead of there.
func seedBig(t *testing.T) (*sqlc.Queries, *session.Service, uuid.UUID) {
	t.Helper()
	pool := testdb.New(t)
	ds, images := fixture.SampleSized(60)
	if _, err := importer.Store(context.Background(), pool, blob.NewLocalDir(t.TempDir()), ds,
		importer.StoreOptions{MarkVerified: true, Images: images, Source: "fixture"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	q := sqlc.New(pool)
	svc := session.NewService(q, pool, billing.Service{Q: q}, learning.NewService(q), progress.NewService(q))
	profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{
		Phone: "+998901234567",
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	return q, svc, profile.ID
}
```

`TestStartSessionExamMode` dan keyin qo'sh:

```go
func TestStartSessionRestoreExamMode(t *testing.T) {
	q, svc, profileID := seedBig(t)
	grantVIP(t, q, profileID)
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "exam", Locale: "uz-Latn", Count: session.ExamRestoreQuestionCount,
	})
	if err != nil {
		t.Fatalf("StartSession restore exam: %v", err)
	}
	if view.Total != session.ExamRestoreQuestionCount || len(view.QuestionIDs) != session.ExamRestoreQuestionCount {
		t.Fatalf("expected %d questions, got total=%d ids=%d",
			session.ExamRestoreQuestionCount, view.Total, len(view.QuestionIDs))
	}
	if view.TimeLimitSec == nil || *view.TimeLimitSec != session.ExamRestoreTimeLimitSec {
		t.Fatalf("expected time limit %d, got %v", session.ExamRestoreTimeLimitSec, view.TimeLimitSec)
	}
	if view.ErrorsAllowed == nil || *view.ErrorsAllowed != session.ExamRestoreErrorsAllowed {
		t.Fatalf("expected errors allowed %d, got %v", session.ExamRestoreErrorsAllowed, view.ErrorsAllowed)
	}
}

// The standard exam must keep reporting its own budget, so the client HUD
// stops guessing it from the mode name.
func TestStartSessionStandardExamReportsErrorBudget(t *testing.T) {
	q, svc, profileID := seed(t)
	grantVIP(t, q, profileID)
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "exam", Locale: "uz-Latn",
	})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if view.ErrorsAllowed == nil || *view.ErrorsAllowed != session.ExamErrorsAllowed {
		t.Fatalf("expected errors allowed %d, got %v", session.ExamErrorsAllowed, view.ErrorsAllowed)
	}
}

// A count the official exam does not have must be refused, not rounded or
// honoured — otherwise a client picks its own difficulty.
func TestStartSessionExamRejectsUnofficialCount(t *testing.T) {
	q, svc, profileID := seedBig(t)
	grantVIP(t, q, profileID)
	for _, count := range []int{3, 30, 49, 51} {
		if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
			Mode: "exam", Locale: "uz-Latn", Count: count,
		}); err != session.ErrInvalidRequest {
			t.Fatalf("count=%d err=%v want ErrInvalidRequest", count, err)
		}
	}
}
```

- [ ] **Step 3: Testlarni yugurtirib yiqilishini tasdiqla**

```bash
cd "/home/sher/Рабочий стол/avtotest" && docker run --rm --network host \
  -v "$PWD":/src -v avtotest-gomod:/go/pkg/mod -w /src/backend \
  -e TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
  golang:1.27 go test ./internal/session/ -p 1 -run 'TestStartSession.*Exam' -count=1 2>&1 | tail -20
```

Kutilgan: FAIL — `view.ErrorsAllowed undefined` (kompilyatsiya).

- [ ] **Step 4: `SessionView` ga `ErrorsAllowed` qo'sh**

`backend/internal/session/dto.go`:

```go
type SessionView struct {
	ID           uuid.UUID
	Mode         string
	QuestionIDs  []uuid.UUID
	TimeLimitSec *int
	// ErrorsAllowed is the session's own mistake budget (2 on the standard
	// exam, 4 on the restore exam, 1 on placement). nil for untimed modes.
	// The client HUD reads this instead of inferring a budget from the mode.
	ErrorsAllowed *int
	Total         int
	StartedAt     time.Time
}
```

- [ ] **Step 5: `StartSession` `case "exam"` ni almashtir**

`backend/internal/session/service.go`:

```go
	case "exam":
		cfg, ok := ExamConfigFor(req.Count)
		if !ok {
			return SessionView{}, ErrInvalidRequest
		}
		active, _, statusErr := s.Billing.Status(ctx, profileID)
		if statusErr != nil {
			return SessionView{}, statusErr
		}
		if !active {
			return SessionView{}, ErrRequiresVIP
		}
		ids, err = s.Q.RandomQuestionIDs(ctx, int32(cfg.QuestionCount))
		if err == nil && len(ids) < cfg.QuestionCount {
			return SessionView{}, ErrInvalidRequest
		}
		timeLimit = pgtype.Int4{Int32: int32(cfg.TimeLimitSec), Valid: true}
		errorsAllowed = pgtype.Int4{Int32: int32(cfg.ErrorsAllowed), Valid: true}
```

Shu funksiyaning oxiridagi `view` qurilishiga `timeLimit` bloki yonига qo'sh:

```go
	if errorsAllowed.Valid {
		v := int(errorsAllowed.Int32)
		view.ErrorsAllowed = &v
	}
```

- [ ] **Step 6: `GetSession` ham `errors_allowed` qaytarsin**

`backend/internal/session/service.go` da `GetSession` ichidagi `if row.TimeLimitSec.Valid { ... }` blokidan keyin qo'sh:

```go
	if row.ErrorsAllowed.Valid {
		v := int(row.ErrorsAllowed.Int32)
		detail.ErrorsAllowed = &v
	}
```

- [ ] **Step 7: JSON javoblariga maydonni qo'sh**

`backend/internal/session/handlers.go` da `startSessionResponse`:

```go
type startSessionResponse struct {
	ID            string    `json:"id"`
	Mode          string    `json:"mode"`
	QuestionIDs   []string  `json:"question_ids"`
	TimeLimitSec  *int      `json:"time_limit_sec"`
	ErrorsAllowed *int      `json:"errors_allowed"`
	Total         int       `json:"total"`
	StartedAt     time.Time `json:"started_at"`
}
```

`toStartSessionResponse` ga `ErrorsAllowed: v.ErrorsAllowed,` qatorini qo'sh.

`sessionDetailResponse` da `TimeLimitSec` qatoridan keyin qo'sh:

```go
	ErrorsAllowed        *int                  `json:"errors_allowed,omitempty"`
```

`toSessionDetailResponse` ga `ErrorsAllowed: d.ErrorsAllowed,` qatorini qo'sh.

- [ ] **Step 8: Testlar o'tishini tasdiqla**

```bash
cd "/home/sher/Рабочий стол/avtotest" && docker run --rm --network host \
  -v "$PWD":/src -v avtotest-gomod:/go/pkg/mod -w /src/backend \
  -e TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
  golang:1.27 go test ./internal/session/ ./internal/importer/ ./internal/fixture/... -p 1 -count=1 2>&1 | tail -20
```

Kutilgan: `ok avtotest.uz/backend/internal/session`, `ok avtotest.uz/backend/internal/importer` — `SampleSized` refaktori 40 ta savolli testlarni buzmagan.

- [ ] **Step 9: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add backend/internal/session/ backend/internal/fixture/fixture.go && git commit -m "feat(session): add the 50-question restore exam and report its error budget

mode=exam now takes count=20 (standard) or count=50 (restore, 50 min, 46/50 to
pass); every other size is refused so a client cannot pick its own difficulty.
An absent count still means 20, keeping saved ?mode=exam links working.

errors_allowed is now on the start and detail responses: the exam HUD was
inferring it from the mode name and would have shown '0/2' during a 50-question
exam that really allows 4.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01MTBBDEFbFgB4hkj6u1jg7g"
```

---

## Task 3: Frontend — xato budjeti sessiyadan o'qilsin, pill'lar desktopda ham scroll bo'lsin

**Files:**
- Modify: `frontend/src/hooks/use-session-engine.ts`
- Modify: `frontend/src/components/exam/official-avtotest-exam-view.tsx:115` va `:351`
- Test: `frontend/src/components/exam/official-avtotest-exam-view.test.tsx`

**Interfaces:**
- Consumes: Task 2'ning `"errors_allowed"` JSON maydoni.
- Produces: `SessionState.errors_allowed: number | null`.

- [ ] **Step 1: Failing test yoz**

`frontend/src/components/exam/official-avtotest-exam-view.test.tsx` oxiriga qo'sh (faylning mavjud `renderExam`/`examSession` helperlaridan foydalan — agar nomlari boshqacha bo'lsa, o'sha fayldagi mavjud helperga moslashtir):

```tsx
it("shows the session's own error budget, not one guessed from the mode", () => {
  render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <OfficialAvtotestExamView
        session={{ ...examSession(), errors_allowed: 4, total: 50 }}
        currentIndex={0}
        onSelectIndex={() => {}}
        onSelectAnswer={() => {}}
        onFinish={() => {}}
        submitting={false}
        finishing={false}
        exitHref="/uz-Latn/dashboard"
      />
    </NextIntlClientProvider>
  );

  expect(screen.getByText(/0\s*\/\s*4/)).toBeInTheDocument();
});

it("falls back to the standard budget when the server did not send one", () => {
  render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <OfficialAvtotestExamView
        session={{ ...examSession(), errors_allowed: null }}
        currentIndex={0}
        onSelectIndex={() => {}}
        onSelectAnswer={() => {}}
        onFinish={() => {}}
        submitting={false}
        finishing={false}
        exitHref="/uz-Latn/dashboard"
      />
    </NextIntlClientProvider>
  );

  expect(screen.getByText(/0\s*\/\s*2/)).toBeInTheDocument();
});
```

- [ ] **Step 2: Testni yugurtirib yiqilishini tasdiqla**

```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npx vitest run src/components/exam/official-avtotest-exam-view.test.tsx 2>&1 | tail -25
```

Kutilgan: FAIL — `errors_allowed` `SessionState` da yo'q (TS xatosi) va "0/4" topilmaydi.

- [ ] **Step 3: `SessionState` ga maydonni qo'sh**

`frontend/src/hooks/use-session-engine.ts`:

`SessionState` interfeysiga `time_limit_sec` dan keyin:

```ts
  /**
   * The session's own mistake budget (2 standard exam, 4 restore exam,
   * 1 placement). null for untimed modes and for sessions created before the
   * backend reported it. The exam HUD reads this instead of inferring a
   * budget from the mode name.
   */
  errors_allowed: number | null;
```

`StartSessionResponse` interfeysiga:

```ts
  errors_allowed: number | null;
```

`SessionDetailResponse` interfeysiga:

```ts
  errors_allowed?: number | null;
```

`startSession` ichidagi `const state: SessionState = {` bloki `time_limit_sec` qatoridan keyin:

```ts
          errors_allowed: created.errors_allowed ?? null,
```

`fetchSessionState` ichidagi qaytarish blokida `time_limit_sec: timeLimitSec,` dan keyin:

```ts
    errors_allowed: detail.errors_allowed ?? null,
```

- [ ] **Step 4: Exam view HUD'ini sessiyadan o'qi**

`frontend/src/components/exam/official-avtotest-exam-view.tsx:115` ni almashtir:

```tsx
  // The server sends the session's real budget; the mode-name guess below is
  // only a fallback for sessions created before it did.
  const errorsAllowed = session.errors_allowed ?? (session.mode === "placement" ? 1 : 2);
```

- [ ] **Step 5: Pill ro'yxatini desktopda ham scroll qil**

Xuddi shu faylda raqam-pill konteynerini (`:351`) almashtir — `max-lg:` prefikslari olib tashlanadi, chunki 50 ta pill desktopga ham sig'maydi (50 × 38px ≈ 1900px):

```tsx
        {/* Question number pills — 50 of them overflow even a 1920px row, so
            the track scrolls at every size; 20 still fit and show no scrollbar. */}
        <div className="flex min-w-0 flex-1 items-center gap-1.5 overflow-x-auto scrollbar-none max-lg:gap-1">
```

- [ ] **Step 6: Testlar o'tishini tasdiqla**

```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npx vitest run src/components/exam/ src/hooks/use-session-engine.test.ts 2>&1 | tail -25
```

Kutilgan: barcha testlar PASS.

- [ ] **Step 7: Typecheck**

```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npm run typecheck 2>&1 | tail -15
```

Kutilgan: xatosiz chiqadi. (Agar `SessionState` obyekti qurilgan boshqa test/fayl `errors_allowed` yo'qligidan yiqilsa, o'sha joylarga `errors_allowed: null` qo'sh.)

- [ ] **Step 8: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add frontend/src/hooks/use-session-engine.ts frontend/src/components/exam/ && git commit -m "fix(exam): read the mistake budget from the session instead of the mode name

The HUD hardcoded 2 for every non-placement exam, so a 50-question restore
exam would have shown '0/2' and a learner would have abandoned a run that was
still alive. The pill track now scrolls at every width too: 50 pills overflow
a 1920px row.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01MTBBDEFbFgB4hkj6u1jg7g"
```

---

## Task 4: Avto-o'tish

**Files:**
- Create: `frontend/src/lib/session-navigation.ts`
- Create: `frontend/src/lib/session-navigation.test.ts`
- Modify: `frontend/src/app/[locale]/(session)/session/[id]/page.tsx`
- Test: `frontend/src/app/[locale]/(session)/session/[id]/page.test.tsx`

**Interfaces:**
- Produces: `AUTO_ADVANCE_MS = 900`;
  `hasAnswer(question: SessionQuestionItem): boolean`;
  `nextUnansweredIndex(questions: SessionQuestionItem[], from: number, justAnsweredId?: string): number` — javobsiz savol topilmasa `-1`.

- [ ] **Step 1: Sof mantiq uchun failing test yoz**

`frontend/src/lib/session-navigation.test.ts` yarat:

```ts
import { describe, expect, it } from "vitest";
import type { SessionQuestionItem } from "@/hooks/use-session-engine";
import { AUTO_ADVANCE_MS, hasAnswer, nextUnansweredIndex } from "@/lib/session-navigation";

function q(id: string, answered = false): SessionQuestionItem {
  return {
    id,
    question: `savol ${id}`,
    image_url: null,
    answers: [{ id: `${id}-a`, text: "javob" }],
    answered,
    user_answer_id: answered ? `${id}-a` : null,
  };
}

describe("hasAnswer", () => {
  it("counts a recorded answer id even when the answered flag is missing", () => {
    expect(hasAnswer({ ...q("1"), answered: undefined, user_answer_id: "x" })).toBe(true);
  });

  it("is false for an untouched question", () => {
    expect(hasAnswer(q("1"))).toBe(false);
  });
});

describe("nextUnansweredIndex", () => {
  it("moves to the next question when it is still unanswered", () => {
    expect(nextUnansweredIndex([q("1"), q("2"), q("3")], 0)).toBe(1);
  });

  it("skips over questions already answered", () => {
    expect(nextUnansweredIndex([q("1"), q("2", true), q("3")], 0)).toBe(2);
  });

  it("wraps back to an earlier gap when nothing is left ahead", () => {
    expect(nextUnansweredIndex([q("1"), q("2", true), q("3", true)], 1)).toBe(0);
  });

  it("returns -1 when every question is answered", () => {
    expect(nextUnansweredIndex([q("1", true), q("2", true)], 0)).toBe(-1);
  });

  // The caller schedules the hop straight after a submit, while the question
  // just answered may not have landed in the array yet.
  it("treats the just-answered question as answered", () => {
    expect(nextUnansweredIndex([q("1"), q("2", true)], 0, "1")).toBe(-1);
  });

  it("stays put when there are no questions at all", () => {
    expect(nextUnansweredIndex([], 0)).toBe(-1);
  });
});

describe("AUTO_ADVANCE_MS", () => {
  it("holds the graded answer on screen long enough to read", () => {
    expect(AUTO_ADVANCE_MS).toBe(900);
  });
});
```

- [ ] **Step 2: Testni yugurtirib yiqilishini tasdiqla**

```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npx vitest run src/lib/session-navigation.test.ts 2>&1 | tail -15
```

Kutilgan: FAIL — `Failed to resolve import "@/lib/session-navigation"`.

- [ ] **Step 3: Mantiqni yoz**

`frontend/src/lib/session-navigation.ts` yarat:

```ts
import type { SessionQuestionItem } from "@/hooks/use-session-engine";

/**
 * How long a graded answer stays on screen before the runner hops to the next
 * question. Long enough to register the green/red state, short enough that a
 * 50-question exam does not feel like 50 pauses.
 */
export const AUTO_ADVANCE_MS = 900;

/**
 * Whether this question already carries the learner's answer. `answered` is
 * the server's flag; `user_answer_id` covers the window where an optimistic
 * local update has recorded the choice but the flag has not been refreshed.
 */
export function hasAnswer(question: SessionQuestionItem): boolean {
  return question.answered === true || Boolean(question.user_answer_id);
}

/**
 * Index of the next question still waiting for an answer: forward from `from`
 * first, then wrapping to the start so a learner who jumped around is carried
 * back to the gaps they left. Returns -1 when nothing is left, which is the
 * caller's signal to stay put and let the auto-finish effect close the session.
 *
 * `justAnsweredId` is counted as answered even if the questions array has not
 * been refreshed yet — the hop is scheduled immediately after a submit.
 */
export function nextUnansweredIndex(
  questions: SessionQuestionItem[],
  from: number,
  justAnsweredId?: string
): number {
  const answered = (question: SessionQuestionItem) =>
    hasAnswer(question) || question.id === justAnsweredId;

  for (let i = from + 1; i < questions.length; i++) {
    if (!answered(questions[i])) return i;
  }
  for (let i = 0; i < Math.min(from + 1, questions.length); i++) {
    if (!answered(questions[i])) return i;
  }
  return -1;
}
```

- [ ] **Step 4: Testlar o'tishini tasdiqla**

```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npx vitest run src/lib/session-navigation.test.ts 2>&1 | tail -15
```

Kutilgan: 8 test PASS.

- [ ] **Step 5: Sahifa uchun failing test yoz**

`frontend/src/app/[locale]/(session)/session/[id]/page.test.tsx` oxiriga qo'sh. Fayl boshidagi mavjud `question()`, `activeSession()`, `mockEngine(session, overrides)` va `renderPage()` helperlaridan foydalanadi.

Pill tugmalari `Session.questionNavLabel` = `"{number}-savol: {status}"` shabloni bilan topiladi; status qiymatlari: `joriy`, `javob berilgan`, `javob berilmagan`, `to'g'ri`, `xato`. Javob tugmalari `question()` fixture'ining matni bilan — `3.27 belgisi` va `3.28 belgisi`.

```tsx
describe("SessionPage auto-advance", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    navigation.push.mockReset();
    vi.mocked(useSessionEngine).mockReset();
    vi.spyOn(apiClient, "apiGet").mockResolvedValue([] as never);
    vi.spyOn(apiClient, "apiPost").mockResolvedValue({ ok: true } as never);
  });

  it("moves to the next unanswered question after the grading pause", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const submitAnswer = vi.fn().mockResolvedValue({ recorded: true, correct: true });
      mockEngine(
        activeSession({
          questions: [question(), question({ id: "q-2", question: "Ikkinchi savol" })],
          total: 2,
        }),
        { submitAnswer }
      );
      renderPage();

      fireEvent.click(screen.getByRole("button", { name: /3\.27 belgisi/ }));
      await waitFor(() => expect(submitAnswer).toHaveBeenCalled());
      expect(screen.getByRole("button", { name: "1-savol: joriy" })).toBeInTheDocument();

      await vi.advanceTimersByTimeAsync(900);

      await waitFor(() =>
        expect(screen.getByRole("button", { name: "2-savol: joriy" })).toHaveAttribute(
          "aria-current",
          "step"
        )
      );
    } finally {
      vi.useRealTimers();
    }
  });

  it("does not advance when the answer stopped the session", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const submitAnswer = vi.fn().mockResolvedValue({
        recorded: true,
        correct: false,
        stopped: true,
        stop_reason: "too_many_errors",
      });
      mockEngine(
        activeSession({
          questions: [question(), question({ id: "q-2", question: "Ikkinchi savol" })],
          total: 2,
        }),
        { submitAnswer }
      );
      renderPage();

      fireEvent.click(screen.getByRole("button", { name: /3\.27 belgisi/ }));
      await waitFor(() => expect(submitAnswer).toHaveBeenCalled());
      await vi.advanceTimersByTimeAsync(2000);

      expect(screen.getByRole("button", { name: "1-savol: joriy" })).toHaveAttribute(
        "aria-current",
        "step"
      );
    } finally {
      vi.useRealTimers();
    }
  });

  it("cancels the hop when the learner navigates first", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const submitAnswer = vi.fn().mockResolvedValue({ recorded: true, correct: true });
      mockEngine(
        activeSession({
          questions: [
            question(),
            question({ id: "q-2", question: "Ikkinchi savol" }),
            question({ id: "q-3", question: "Uchinchi savol" }),
          ],
          total: 3,
        }),
        { submitAnswer }
      );
      renderPage();

      fireEvent.click(screen.getByRole("button", { name: /3\.27 belgisi/ }));
      await waitFor(() => expect(submitAnswer).toHaveBeenCalled());

      fireEvent.click(screen.getByRole("button", { name: "3-savol: javob berilmagan" }));
      await vi.advanceTimersByTimeAsync(2000);

      expect(screen.getByRole("button", { name: "3-savol: joriy" })).toHaveAttribute(
        "aria-current",
        "step"
      );
    } finally {
      vi.useRealTimers();
    }
  });
});
```

- [ ] **Step 6: Testni yugurtirib yiqilishini tasdiqla**

```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npx vitest run "src/app/[locale]/(session)/session/[id]/page.test.tsx" -t "auto-advance" 2>&1 | tail -25
```

Kutilgan: FAIL — birinchi test 900 ms dan keyin ham 1-savolda qoladi.

- [ ] **Step 7: Sahifaga avto-o'tishni qo'sh**

`frontend/src/app/[locale]/(session)/session/[id]/page.tsx`:

**(a)** Import qo'sh (mavjud `@/lib/question-image` importidan keyin):

```tsx
import { AUTO_ADVANCE_MS, hasAnswer, nextUnansweredIndex } from "@/lib/session-navigation";
```

**(b)** Fayl ichidagi lokal `hasAnswer` funksiyasini **o'chir** (`:96-98`) — endi lib'dan keladi.

**(c)** `activeChipRef` deklaratsiyasidan keyin qo'sh:

```tsx
  const autoAdvanceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const cancelAutoAdvance = useCallback(() => {
    if (autoAdvanceRef.current !== null) {
      clearTimeout(autoAdvanceRef.current);
      autoAdvanceRef.current = null;
    }
  }, []);

  // A pending hop must never outlive the screen that scheduled it.
  useEffect(() => cancelAutoAdvance, [cancelAutoAdvance]);
```

**(d)** `goToQuestion` ni almashtir — har qanday qo'lda navigatsiya kutilayotgan sakrashni bekor qiladi:

```tsx
  const goToQuestion = useCallback(
    (index: number) => {
      cancelAutoAdvance();
      setCurrentIndex(index);
    },
    [cancelAutoAdvance]
  );
```

**(e)** `handleSelectAnswer` ichida, `trackEvent("answer", answerProps);` dan keyin va `if (response.stopped)` blokidan **oldin** qo'sh:

```tsx
      // Auto-advance: hold the graded answer on screen briefly, then move to
      // the next gap. Skipped when this answer ended the session — that screen
      // is the result, not another question.
      if (response.recorded && !response.stopped) {
        const target = nextUnansweredIndex(
          session.questions,
          session.questions.findIndex((item) => item.id === questionId),
          questionId
        );
        if (target >= 0) {
          cancelAutoAdvance();
          autoAdvanceRef.current = setTimeout(() => {
            autoAdvanceRef.current = null;
            setCurrentIndex(target);
          }, AUTO_ADVANCE_MS);
        }
      }
```

va `handleSelectAnswer` ning `useCallback` bog'liqliklariga `cancelAutoAdvance` qo'sh.

**(f)** `handleFinish` ning boshiga, `setFinishing(true);` dan oldin qo'sh:

```tsx
    cancelAutoAdvance();
```

va uning bog'liqliklariga ham `cancelAutoAdvance` qo'sh.

**(g)** Izoh oynasi ochilganda o'g'irlanmasin — `QuestionStage` ning `onOpenExplanation` propini almashtir:

```tsx
              onOpenExplanation={() => {
                cancelAutoAdvance();
                setExplanationOpen(true);
              }}
```

- [ ] **Step 8: Testlar o'tishini tasdiqla**

```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npx vitest run "src/app/[locale]/(session)/session/[id]/page.test.tsx" src/lib/session-navigation.test.ts 2>&1 | tail -25
```

Kutilgan: barcha testlar PASS — jumladan fayldagi mavjud testlar ham (lokal `hasAnswer` olib tashlanganidan keyin xatti-harakat o'zgarmagan bo'lishi kerak).

- [ ] **Step 9: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add frontend/src/lib/session-navigation.ts frontend/src/lib/session-navigation.test.ts "frontend/src/app/[locale]/(session)/session/[id]/page.tsx" "frontend/src/app/[locale]/(session)/session/[id]/page.test.tsx" && git commit -m "feat(session): advance to the next question on its own after an answer

The runner sat on the answered question until the learner found the right
number pill at the bottom of the screen — 50 times in a restore exam, on a
phone or a classroom touchscreen. It now holds the graded answer for 900ms and
hops to the next gap, cancelling if the learner navigates, opens an
explanation, finishes, or the answer ended the session.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01MTBBDEFbFgB4hkj6u1jg7g"
```

---

## Task 5: Rejim tanlash ekrani

**Files:**
- Create: `frontend/src/components/exam/exam-mode-picker.tsx`
- Create: `frontend/src/components/exam/exam-mode-picker.test.tsx`
- Create: `frontend/src/app/[locale]/(app)/exam/page.tsx`
- Create: `frontend/src/app/[locale]/(kiosk)/station/exam/page.tsx`
- Modify: `frontend/src/lib/protected-segments.ts`
- Modify: `frontend/messages/uz-Latn.json`, `uz-Cyrl.json`, `ru.json`

**Interfaces:**
- Produces: `ExamModePicker({ kiosk }: { kiosk?: boolean })` — default export'lar `(app)/exam/page.tsx` va `(kiosk)/station/exam/page.tsx` da.
- Consumes: `useMeQuery()` (`@/hooks/use-me`) — VIP holati uchun; `session.ExamConfigFor` whitelist'iga mos `count=20|50`.

- [ ] **Step 1: Uchala tilga `ExamPicker` kalitlarini qo'sh**

`frontend/messages/uz-Latn.json` — yuqori darajadagi `"Session"` bo'limidan keyin qo'sh:

```json
  "ExamPicker": {
    "title": "Imtihon simulyatsiyasi",
    "subtitle": "Qaysi imtihonni topshirasiz?",
    "standardTitle": "Oddiy imtihon",
    "standardAudience": "Birinchi marta prava olayotganlar uchun",
    "restoreTitle": "Qayta topshirish",
    "restoreAudience": "Pravadan mahrum bo'lib qayta o'qiyotganlar uchun",
    "questionsMeta": "{count} savol · {minutes} daqiqa",
    "errorsMeta": "{count} xatogacha ruxsat",
    "stopMeta": "{n}-xatoda to'xtaydi",
    "start": "Boshlash",
    "vipLocked": "VIP obuna kerak",
    "keyboardHint": "Klaviaturada 1 yoki 2 tugmasini bosing"
  },
```

`frontend/messages/uz-Cyrl.json`:

```json
  "ExamPicker": {
    "title": "Имтиҳон симуляцияси",
    "subtitle": "Қайси имтиҳонни топширасиз?",
    "standardTitle": "Оддий имтиҳон",
    "standardAudience": "Биринчи марта права олаётганлар учун",
    "restoreTitle": "Қайта топшириш",
    "restoreAudience": "Правадан маҳрум бўлиб қайта ўқиётганлар учун",
    "questionsMeta": "{count} савол · {minutes} дақиқа",
    "errorsMeta": "{count} хатогача рухсат",
    "stopMeta": "{n}-хатода тўхтайди",
    "start": "Бошлаш",
    "vipLocked": "VIP обуна керак",
    "keyboardHint": "Клавиатурада 1 ёки 2 тугмасини босинг"
  },
```

`frontend/messages/ru.json`:

```json
  "ExamPicker": {
    "title": "Симуляция экзамена",
    "subtitle": "Какой экзамен вы сдаёте?",
    "standardTitle": "Обычный экзамен",
    "standardAudience": "Для тех, кто получает права впервые",
    "restoreTitle": "Пересдача",
    "restoreAudience": "Для лишённых прав и проходящих переобучение",
    "questionsMeta": "{count} вопросов · {minutes} минут",
    "errorsMeta": "До {count} ошибок",
    "stopMeta": "Остановка на {n}-й ошибке",
    "start": "Начать",
    "vipLocked": "Нужна VIP-подписка",
    "keyboardHint": "Нажмите 1 или 2 на клавиатуре"
  },
```

- [ ] **Step 2: Failing test yoz**

`frontend/src/components/exam/exam-mode-picker.test.tsx` yarat:

```tsx
import { render, screen, fireEvent } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, expect, it, vi, beforeEach } from "vitest";
import messages from "../../../messages/uz-Latn.json";
import { ExamModePicker } from "./exam-mode-picker";

const navigation = vi.hoisted(() => ({ push: vi.fn() }));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: navigation.push }),
}));

vi.mock("@/hooks/use-me", () => ({
  useMeQuery: vi.fn(() => ({ data: { vip: { active: true } }, isLoading: false })),
}));

function renderPicker(kiosk = false) {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <ExamModePicker kiosk={kiosk} />
    </NextIntlClientProvider>
  );
}

beforeEach(() => {
  navigation.push.mockReset();
});

describe("ExamModePicker", () => {
  it("offers both official exam varieties", () => {
    renderPicker();
    expect(screen.getByText(messages.ExamPicker.standardTitle)).toBeInTheDocument();
    expect(screen.getByText(messages.ExamPicker.restoreTitle)).toBeInTheDocument();
  });

  it("states each variety's real rules", () => {
    renderPicker();
    expect(screen.getByText("20 savol · 25 daqiqa")).toBeInTheDocument();
    expect(screen.getByText("50 savol · 50 daqiqa")).toBeInTheDocument();
    expect(screen.getByText("2 xatogacha ruxsat")).toBeInTheDocument();
    expect(screen.getByText("4 xatogacha ruxsat")).toBeInTheDocument();
  });

  it("starts a 20-question exam from the standard card", () => {
    renderPicker();
    fireEvent.click(screen.getByTestId("exam-mode-standard"));
    expect(navigation.push).toHaveBeenCalledWith("/uz-Latn/session/start?mode=exam&count=20");
  });

  it("starts a 50-question exam from the restore card", () => {
    renderPicker();
    fireEvent.click(screen.getByTestId("exam-mode-restore"));
    expect(navigation.push).toHaveBeenCalledWith("/uz-Latn/session/start?mode=exam&count=50");
  });

  // A classroom PC has a keyboard and often no usable mouse.
  it("starts an exam from the number keys", () => {
    renderPicker();
    fireEvent.keyDown(window, { key: "2" });
    expect(navigation.push).toHaveBeenCalledWith("/uz-Latn/session/start?mode=exam&count=50");
  });

  // Every kiosk destination must stay under /station/... — the kiosk browser
  // holds no auth cookie and bounces off the learner routes.
  it("keeps kiosk navigation inside the station namespace", () => {
    renderPicker(true);
    fireEvent.click(screen.getByTestId("exam-mode-restore"));
    expect(navigation.push).toHaveBeenCalledWith("/uz-Latn/station/session/start?mode=exam&count=50");
  });
});

describe("ExamModePicker without a subscription", () => {
  it("sends a learner without VIP to the tariffs instead of into an exam", async () => {
    const { useMeQuery } = await import("@/hooks/use-me");
    vi.mocked(useMeQuery).mockReturnValue({
      data: { vip: { active: false } },
      isLoading: false,
    } as ReturnType<typeof useMeQuery>);

    renderPicker();
    fireEvent.click(screen.getByTestId("exam-mode-standard"));
    expect(navigation.push).toHaveBeenCalledWith("/uz-Latn/premium");
  });
});
```

- [ ] **Step 3: Testni yugurtirib yiqilishini tasdiqla**

```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npx vitest run src/components/exam/exam-mode-picker.test.tsx 2>&1 | tail -20
```

Kutilgan: FAIL — `Failed to resolve import "./exam-mode-picker"`.

- [ ] **Step 4: Komponentni yoz**

`frontend/src/components/exam/exam-mode-picker.tsx` yarat:

```tsx
"use client";

import { useCallback, useEffect } from "react";
import { useLocale, useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { Award, ChevronRight, Clock, ListChecks, Lock, RotateCcw, XCircle } from "lucide-react";
import { useMeQuery } from "@/hooks/use-me";

/**
 * The two official exam varieties. `count` is what the backend whitelists
 * (session.ExamConfigFor) — it is the only thing the client gets to choose;
 * time limit and error budget are decided server-side from it. The numbers
 * shown here are copies for display and must match rules.go.
 */
const MODES = [
  {
    key: "standard",
    count: 20,
    minutes: 25,
    errors: 2,
    icon: Award,
    titleKey: "standardTitle",
    audienceKey: "standardAudience",
    accent: "border-emerald-500/50 hover:border-emerald-400 focus-visible:ring-emerald-400",
    badge: "bg-emerald-500/15 text-emerald-300",
    shortcut: "1",
  },
  {
    key: "restore",
    count: 50,
    minutes: 50,
    errors: 4,
    icon: RotateCcw,
    titleKey: "restoreTitle",
    audienceKey: "restoreAudience",
    accent: "border-amber-500/50 hover:border-amber-400 focus-visible:ring-amber-400",
    badge: "bg-amber-500/15 text-amber-300",
    shortcut: "2",
  },
] as const;

export interface ExamModePickerProps {
  /**
   * Reused as-is under the login-free kiosk
   * (frontend/src/app/[locale]/(kiosk)/station/exam/page.tsx): a walk-up
   * student has no learner routes to land on, so every destination stays
   * under /station/... and the VIP paywall is never offered.
   */
  kiosk?: boolean;
}

export function ExamModePicker({ kiosk = false }: ExamModePickerProps) {
  const t = useTranslations("ExamPicker");
  const locale = useLocale();
  const router = useRouter();
  const { data: me } = useMeQuery();

  // A station profile is always licensed, so the kiosk never shows the lock
  // and never has a /premium route to send anyone to.
  const locked = !kiosk && me?.vip?.active === false;

  const start = useCallback(
    (count: number) => {
      if (locked) {
        router.push(`/${locale}/premium`);
        return;
      }
      const base = kiosk ? `/${locale}/station/session/start` : `/${locale}/session/start`;
      router.push(`${base}?mode=exam&count=${count}`);
    },
    [kiosk, locale, locked, router]
  );

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      const mode = MODES.find((item) => item.shortcut === event.key);
      if (!mode) return;
      event.preventDefault();
      start(mode.count);
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [start]);

  return (
    <main className="relative flex min-h-screen flex-col items-center justify-center overflow-hidden bg-[#091726] px-4 py-10 text-white">
      {/* Same cube mesh the exam runner uses, so the chooser reads as its front door. */}
      <div
        className="pointer-events-none absolute inset-0"
        style={{
          opacity: 0.12,
          backgroundImage: `
            linear-gradient(135deg, #1a3a6a 25%, transparent 25%),
            linear-gradient(225deg, #1a3a6a 25%, transparent 25%),
            linear-gradient(315deg, #1a3a6a 25%, transparent 25%),
            linear-gradient(45deg,  #1a3a6a 25%, transparent 25%)
          `,
          backgroundSize: "20px 20px",
          backgroundPosition: "0 0, 0 10px, 10px -10px, -10px 0px",
        }}
        aria-hidden="true"
      />

      <div className="relative z-10 w-full max-w-4xl">
        <header className="text-center">
          <h1 className="font-display text-3xl font-black tracking-tight sm:text-4xl">{t("title")}</h1>
          <p className="mt-2 text-sm text-slate-300 sm:text-base">{t("subtitle")}</p>
        </header>

        <div className="mt-8 grid gap-4 sm:mt-10 lg:grid-cols-2 lg:gap-6">
          {MODES.map((mode) => {
            const Icon = mode.icon;
            return (
              <button
                key={mode.key}
                type="button"
                data-testid={`exam-mode-${mode.key}`}
                onClick={() => start(mode.count)}
                className={`group flex min-h-[14rem] flex-col rounded-2xl border-2 bg-[#0d2e4d]/80 p-5 text-left shadow-lg transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-[#091726] sm:p-6 ${mode.accent}`}
              >
                <div className="flex items-start justify-between gap-3">
                  <div className={`flex h-12 w-12 shrink-0 items-center justify-center rounded-xl ${mode.badge}`}>
                    <Icon className="h-6 w-6" aria-hidden="true" />
                  </div>
                  <span className="hidden rounded-md border border-[#2a4568] bg-[#081320] px-2 py-1 font-mono text-xs font-bold text-slate-300 lg:inline">
                    {mode.shortcut}
                  </span>
                </div>

                <h2 className="mt-4 font-display text-xl font-extrabold sm:text-2xl">{t(mode.titleKey)}</h2>
                <p className="mt-1 text-sm text-slate-300">{t(mode.audienceKey)}</p>

                <dl className="mt-4 space-y-1.5 text-sm font-semibold text-slate-200">
                  <div className="flex items-center gap-2">
                    <ListChecks className="h-4 w-4 shrink-0 text-slate-400" aria-hidden="true" />
                    <dd>{t("questionsMeta", { count: mode.count, minutes: mode.minutes })}</dd>
                  </div>
                  <div className="flex items-center gap-2">
                    <Clock className="h-4 w-4 shrink-0 text-slate-400" aria-hidden="true" />
                    <dd>{t("errorsMeta", { count: mode.errors })}</dd>
                  </div>
                  <div className="flex items-center gap-2">
                    <XCircle className="h-4 w-4 shrink-0 text-slate-400" aria-hidden="true" />
                    <dd>{t("stopMeta", { n: mode.errors + 1 })}</dd>
                  </div>
                </dl>

                <span className="mt-5 inline-flex min-h-14 w-full items-center justify-center gap-2 rounded-xl bg-[#183654] px-4 text-base font-extrabold transition-colors group-hover:bg-[#1f4a72]">
                  {locked ? (
                    <>
                      <Lock className="h-4 w-4" aria-hidden="true" />
                      {t("vipLocked")}
                    </>
                  ) : (
                    <>
                      {t("start")}
                      <ChevronRight className="h-5 w-5" aria-hidden="true" />
                    </>
                  )}
                </span>
              </button>
            );
          })}
        </div>

        <p className="mt-6 hidden text-center text-xs text-slate-400 lg:block">{t("keyboardHint")}</p>
      </div>
    </main>
  );
}
```

- [ ] **Step 5: Testlar o'tishini tasdiqla**

```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npx vitest run src/components/exam/exam-mode-picker.test.tsx 2>&1 | tail -20
```

Kutilgan: 7 test PASS.

- [ ] **Step 6: Ikkala marshrutni yarat**

`frontend/src/app/[locale]/(app)/exam/page.tsx`:

```tsx
// Exam variety chooser: /[locale]/exam.
//
// Every "Imtihon" entry point in the learner app lands here instead of
// starting a session outright, because there are now two official exams
// (20-question standard and 50-question restore) and the runner cannot guess
// which one the learner is preparing for.
import { ExamModePicker } from "@/components/exam/exam-mode-picker";

export default function ExamPage() {
  return <ExamModePicker />;
}
```

`frontend/src/app/[locale]/(kiosk)/station/exam/page.tsx`:

```tsx
// Kiosk exam chooser: /[locale]/station/exam.
//
// This is a distinct URL from the learner app's /[locale]/exam specifically so
// the middleware exemption in src/proxy.ts stays narrow (only "station" and
// everything under it is login-free) instead of unauthenticating /exam for
// every learner. It reuses the same component in kiosk mode, which keeps every
// destination under /station/... and never offers the VIP paywall — see
// ExamModePickerProps in the imported module.
import { ExamModePicker } from "@/components/exam/exam-mode-picker";

export default function KioskExamPage() {
  return <ExamModePicker kiosk />;
}
```

- [ ] **Step 7: `/exam` ni himoyalangan segmentlarga qo'sh**

`frontend/src/lib/protected-segments.ts` — `"dashboard"` dan keyin `"exam",` qatorini qo'sh (`"exam-mockup"` dan oldin, alfavit tartibi buzilmasin):

```ts
export const PROTECTED_SEGMENTS = [
  "dashboard",
  "exam",
  "exam-mockup",
```

`matchesAny` faqat `/exam` va `/exam/...` ga mos keladi, ya'ni kioskning `/station/exam` manzili qamrab olinmaydi.

- [ ] **Step 8: `ExamPicker` ni klient namespace ro'yxatiga qo'sh**

Bu **majburiy** — komponent testlari uni ushlamaydi, chunki ular
`NextIntlClientProvider` ga to'liq messages faylini uzatadi. Ro'yxatda
bo'lmasa, ishlab turgan sahifa xom kalitlar ekraniga aylanadi.

`frontend/src/i18n/namespaces.ts` da `APP_EXTRA` ichiga `"ExamMockup"` dan
keyin qo'sh (`KIOSK_NAMESPACES` uni `APP_NAMESPACES` dan meros oladi):

```ts
  // The 20/50 exam chooser at /exam; KIOSK_NAMESPACES inherits it for
  // /station/exam, which renders the same component.
  "ExamPicker",
```

`frontend/src/i18n/pick-messages.test.ts` dagi `describe("pickMessages")`
ichiga regressiya testi:

```ts
  // /exam and /station/exam render the same ExamModePicker. A namespace left
  // out here does not fail a component test — those wrap the full message file
  // — it ships a screen of raw keys, which is how this was nearly missed.
  it("ships the exam chooser strings to both shells that render it", () => {
    expect(APP_NAMESPACES).toContain("ExamPicker");
    expect(KIOSK_NAMESPACES).toContain("ExamPicker");
    expect(pickMessages(uzLatn, APP_NAMESPACES).ExamPicker).toEqual(uzLatn.ExamPicker);
  });
```

```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npx vitest run src/i18n/ 2>&1 | tail -10
```

Kutilgan: 5 test PASS.

- [ ] **Step 9: Kiosk marshrut testi avtomatik o'tishini tasdiqla**

`kiosk-path.test.tsx` fayl tizimini o'zi aylanib chiqadi, ya'ni yangi `station/exam/page.tsx` unga qo'lda qo'shilmaydi.

```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npx vitest run "src/app/[locale]/(kiosk)/" 2>&1 | tail -20
```

Kutilgan: barcha kiosk testlari PASS — `/station/exam` `PROTECTED_SEGMENTS` dan tashqarida.

- [ ] **Step 10: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add frontend/src/components/exam/exam-mode-picker.tsx frontend/src/components/exam/exam-mode-picker.test.tsx "frontend/src/app/[locale]/(app)/exam" "frontend/src/app/[locale]/(kiosk)/station/exam" frontend/src/lib/protected-segments.ts frontend/messages && git commit -m "feat(exam): let the learner pick which official exam they are sitting

Two exams exist now and the runner cannot guess which one a learner is
preparing for, so /exam (and /station/exam on the kiosk) asks first, in the
runner's own navy palette. Number keys pick a card for classroom PCs; a
learner without VIP is sent to the tariffs instead of into a 402.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01MTBBDEFbFgB4hkj6u1jg7g"
```

---

## Task 6: Kirish nuqtalarini tanlov ekraniga burish

**Files:**
- Modify: `frontend/src/components/layout/sidebar.tsx:88`, `:141`
- Modify: `frontend/src/app/[locale]/(app)/dashboard/page.tsx:200`, `:563`
- Modify: `frontend/src/app/[locale]/(kiosk)/station/page.tsx:51`
- Modify: `frontend/messages/uz-Latn.json`, `uz-Cyrl.json`, `ru.json`
- Test: `frontend/src/components/layout/sidebar.test.tsx`

**Interfaces:**
- Consumes: Task 5'ning `/[locale]/exam` va `/[locale]/station/exam` marshrutlari.

- [ ] **Step 1: Sidebar testini yangila va yangi tekshiruv qo'sh**

`frontend/src/components/layout/sidebar.test.tsx:125-134` da mavjud tekshiruv **aynan 2 ta** `session/start` havolasi bo'lishini talab qiladi — bu o'sha ikkita imtihon havolasi. Ular `/exam` ga ko'chgach bu 0 ga tushadi, ya'ni testning o'z mantiqi bo'yicha holat yaxshilanadi. Tekshiruvni va uning izohini almashtir:

```tsx
    // The image split lives inside Practice, not in the nav. Two sibling entries
    // differing only by a boolean cluttered the sidebar and pointed several nav
    // items at /session/start, which starts a session as a mount side effect.
    // No nav entry points there any more: the exam entries land on the chooser.
    expect(
      screen.queryAllByRole("link").filter((link) => link.getAttribute("href")?.includes("session/start"))
    ).toHaveLength(0);
```

Va shu `describe` blokining oxiriga yangi test qo'sh (fayldagi mavjud `renderWithIntl(localeCase)` va `localeCases` dan foydalanadi):

```tsx
  it.each(localeCases)("routes the exam entry to the variety chooser for $locale", (localeCase) => {
    renderWithIntl(localeCase);

    const examLinks = screen
      .queryAllByRole("link")
      .filter((link) => link.getAttribute("href")?.endsWith("/exam"));

    // One entry in the desktop sidebar list + one in the thumb-zone tabs.
    expect(examLinks).toHaveLength(2);
    for (const link of examLinks) {
      expect(link).toHaveAttribute("href", `/${localeCase.locale}/exam`);
    }
  });
```

- [ ] **Step 2: Testni yugurtirib yiqilishini tasdiqla**

```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npx vitest run src/components/layout/sidebar.test.tsx 2>&1 | tail -20
```

Kutilgan: FAIL — havolalar hali `session/start?mode=exam`.

- [ ] **Step 3: Havolalarni yangila**

`frontend/src/components/layout/sidebar.tsx` — ikkala joyda:

```tsx
    { href: `/${currentLocale}/exam`, label: t("navExam"), icon: Award },
```

```tsx
      href: `/${currentLocale}/exam`,
```

`frontend/src/app/[locale]/(app)/dashboard/page.tsx` — ikkala joyda `` `/${locale}/session/start?mode=exam` `` ni `` `/${locale}/exam` `` ga almashtir.

`frontend/src/app/[locale]/(kiosk)/station/page.tsx:51`:

```tsx
    href: "station/exam",
```

- [ ] **Step 4: Imtihon matnlarini ikki turga moslashtir**

Endi bitta "20 savol / 25 daqiqa" jumlasi noto'g'ri — ikki tur bor.

`frontend/messages/uz-Latn.json` `Dashboard` bo'limida:

```json
    "examMeta": "20 yoki 50 savol",
    "navExamDesc": "Rasmiy imtihon: 20 savollik yoki 50 savollik",
```

`frontend/messages/uz-Cyrl.json`:

```json
    "examMeta": "20 ёки 50 савол",
    "navExamDesc": "Расмий имтиҳон: 20 саволлик ёки 50 саволлик",
```

`frontend/messages/ru.json`:

```json
    "examMeta": "20 или 50 вопросов",
    "navExamDesc": "Официальный экзамен: 20 или 50 вопросов",
```

- [ ] **Step 5: Testlar o'tishini tasdiqla**

```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npx vitest run src/components/layout/ "src/app/[locale]/(app)/dashboard/" "src/app/[locale]/(kiosk)/" 2>&1 | tail -20
```

Kutilgan: barcha testlar PASS.

- [ ] **Step 6: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest" && git add frontend/src/components/layout/sidebar.tsx frontend/src/components/layout/sidebar.test.tsx "frontend/src/app/[locale]/(app)/dashboard/page.tsx" "frontend/src/app/[locale]/(kiosk)/station/page.tsx" frontend/messages && git commit -m "feat(exam): point every exam entry at the variety chooser

Sidebar, dashboard card, next-action tile and the kiosk home all opened a
20-question exam directly. They now land on the chooser, and the copy stops
promising '20 savol / 25 daqiqa' when two exams exist. ?mode=exam links that
are already saved still resolve to the standard exam.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01MTBBDEFbFgB4hkj6u1jg7g"
```

---

## Task 7: To'liq gate

**Files:** yo'q — faqat tekshiruv.

- [ ] **Step 1: Frontend to'liq gate**

```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npm run lint 2>&1 | tail -10 && npm run typecheck 2>&1 | tail -10 && npm run test 2>&1 | tail -15 && npm run build 2>&1 | tail -15
```

Kutilgan: `lint` toza, `typecheck` xatosiz, barcha vitest fayllari PASS, `build` muvaffaqiyatli va route ro'yxatida `/[locale]/exam` hamda `/[locale]/station/exam` ko'rinadi.

- [ ] **Step 2: Backend to'liq test to'plami**

**`run_in_background: true` bilan yugurtiring** — ~10 daqiqadan uzoq davom etadi.

```bash
cd "/home/sher/Рабочий стол/avtotest" && docker run --rm --network host \
  -v "$PWD":/src -v avtotest-gomod:/go/pkg/mod -w /src/backend \
  -e TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
  golang:1.27 go test -p 1 ./... -count=1
```

Kutilgan: barcha paketlar `ok` yoki `no test files`.

- [ ] **Step 3: Backend lint**

```bash
cd "/home/sher/Рабочий стол/avtotest" && docker run --rm -v "$PWD":/src \
  -v avtotest-gomod:/go/pkg/mod -v avtotest-golangci:/root/.cache -w /src/backend \
  golangci/golangci-lint:latest-alpine golangci-lint run 2>&1 | tail -20
```

Kutilgan: topilma yo'q. (`v2.6-alpine` tegini ishlatmang — Go versiya mos kelmasligi bilan yiqiladi.)

- [ ] **Step 4: Qo'lda ko'rib chiqish ro'yxati**

`npm run dev` bilan ishga tushirib, quyidagilarni tekshir:

1. `/uz-Latn/exam` — ikkala karta ko'rinadi, `1` va `2` tugmalari ishlaydi.
2. 50-lik boshlanganda: taymer 50:00 dan boshlanadi, HUD "0/4" ko'rsatadi, 50 ta pill scroll bo'ladi.
3. Javob berilgach ~1 soniyada keyingi savolga o'zi o'tadi; pill'ni bosib turib qo'lda o'tsang, avto-sakrash bekor bo'ladi.
4. Telefon kengligida (DevTools, 390px) tanlov ekrani ustma-ust chiqadi, tugmalar bosishga qulay.
5. `/uz-Latn/station/exam` — bir xil ekran, "Boshlash" `/station/session/start?...` ga olib boradi va VIP qulfi ko'rinmaydi.
6. Uchala tilda ham matnlar chiqadi (kalit nomi emas).

---

## Self-review qaydlari

- **Spec qamrovi:** 1.1→Task 2; 1.2→Task 1; 1.3→Task 3; 1.4→Task 5+6; 1.5→Task 4; spec §3.5→Task 2 Step 7; §4.5→Task 5 Step 7; §6→Task 5 Step 1 va Task 6 Step 4; §8.3→Task 3 Step 5.
- **Turlar izchilligi:** `ExamConfigFor` Task 1'da e'lon qilinib Task 2'da ishlatiladi; `hasAnswer`/`nextUnansweredIndex`/`AUTO_ADVANCE_MS` Task 4'da bir joyda e'lon qilinadi; `errors_allowed` Task 2'da (Go) va Task 3'da (TS) bir xil nomda.
- **Namespace tuzog'i (implementatsiyada topildi):** Task 5 Step 8 — `ExamPicker` ni
  `APP_NAMESPACES` ga qo'shmasa, sahifa xom kalitlar bilan chiqadi va **hech bir
  komponent testi buni ushlamaydi**. Shu sababli tekshiruv namespace ro'yxatining
  o'ziga yozildi.
- **Mavjud testga majburiy o'zgarish:** `sidebar.test.tsx:125-134` `session/start` havolalari sonini aynan 2 deb qotirib qo'ygan; Task 6 uni 0 ga o'zgartiradi. Bu yagona "eskisi yashil bo'lgani uchun yiqiladigan" joy — Task 6 Step 1'da aniq ko'rsatilgan.
- **Test selektorlari** mavjud fayllardan o'qib olindi va rejaga aynan ko'chirildi: pill'lar `"{number}-savol: {status}"` (`joriy` / `javob berilmagan` / …), javob tugmalari `3.27 belgisi`, `mockEngine(session, overrides)`.
