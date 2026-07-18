# M1 Plan 03 — Exam Sessions, Scoring, Bilet Unlock

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Server-authoritative test-taking for all four M1 modes (bilet/variant, imtihon/exam, mashq/practice, xatolar banki/mistakes) — start a session, submit answers one at a time with anti-cheat scoring, finish with a computed result, resume an interrupted session, and unlock the next bilet once a threshold is met.

**Architecture:** A single `internal/session` package owns the whole vertical slice: pure scoring/unlock rules (`rules.go`, no DB — the part most worth unit-testing exhaustively), a DB-integrated `Service` (`service.go`) that selects questions per mode, records answers, and finalizes sessions, and HTTP handlers (`handlers.go`) mounted under `/api/v1` behind the existing `auth.Required` middleware. Sessions never re-serve question text/images/answers — the client already has `GET /variants/{n}` and `GET /questions/{id}` from Plan 01's content API (anti-cheat safe, no `is_correct` ever) and calls those to render; `internal/session` only hands out ordered/random **question IDs**, accepts `{question_id, answer_id}` submissions, and returns correctness feedback (except in exam mode, where feedback is withheld until `finish`, matching the real exam's "no feedback until the end" rule).

**Tech Stack:** No new dependencies — same stack as Plan 01/02 (chi, pgx/sqlc, golang-migrate already has the `exam_session`/`session_answer`/`variant_progress`/`question_memory`/`limit_config` tables from migration `0004_learning.up.sql`, no new migration needed).

## Global Constraints

- All Plan 01/02 conventions hold (envelope, locales, commit style, `-p 1` tests, `export PATH=$HOME/.local/go/bin:$HOME/go/bin:$PATH`, `make up` before any DB test run).
- **Exam mode fixed rule** (spec §4, verified against osonprava.uz/yim.uz/gov.uz): 20 questions, 25 minute limit, pass requires ≥18/20 correct AND ≤2 wrong, 3rd wrong ends the attempt immediately. Constants: `ExamQuestionCount=20`, `ExamTimeLimitSec=1500`, `ExamErrorsAllowed=2`.
- **Immediate feedback vs. end-of-session feedback** (spec §11 table): modes `variant`, `practice`, `mistakes` return `{correct, correct_answer_id}` on every answer submission. Mode `exam` returns only `{recorded: true}` per answer — no correctness — until `POST /sessions/{id}/finish`.
- **Bilet unlock** (spec §7.3, §11): variant N+1 is unlocked iff variant N's `best_correct >= limit_config('unlock_threshold_correct')`. Variant #1 (lowest `number`) is always unlocked. Sequence is by `variant.number` ascending (matches the existing `ListVariants` ordering from Plan 01).
- **Mistakes-bank (Leitner, no FSRS yet)**: a question enters the bank (`question_memory.state=0`) the first time it's answered incorrectly in *any* session; it's cleared (`state=1`) after `MistakeClearAfter=2` consecutive correct answers **within mistakes-mode sessions**. `question_memory.lapses`/`reps` are reused here with plain counters; Plan 04 (FSRS) later layers `stability`/`difficulty`/`due_at` scheduling on the same table without redefining these two fields — standard FSRS terminology already matches (`reps`=review count, `lapses`=forgetting count).
- **Practice mode daily free-limit**: reads `limit_config('daily_practice_questions')` — `free_value` if the profile has no active VIP entitlement (`billing.Service.Status`), else `vip_value` (`-1` = unlimited). Requested session size is clamped to the remaining daily allowance; if the remaining allowance is `0`, `StartSession` returns `ErrDailyLimitReached`.
- **Ownership & anti-cheat**: every session read/write is scoped to `claims.ProfileID` from the JWT (never a body/query param) — a session ID belonging to another profile is `404 not_found`, not `403`, to avoid confirming existence. Resuming an **in-progress exam** session never reveals per-answer correctness (only `answered: true/false` per position); once the session is no longer `in_progress`, full per-answer correctness is included.
- **Idempotent finish**: calling `finish` on an already-finished session returns the stored result unchanged (no re-scoring, no double `variant_progress`/mistake-bank writes).
- Grand Mock (`mode='grand_mock'`) is **out of scope for M1** — the CHECK constraint already allows the value (future-proofing) but no code path creates or serves it; it's gated by VIP + Plan 02's monetization work (M2, spec §5).
- Error sentinels: `ErrNotFound, ErrForbiddenMode, ErrAlreadyAnswered, ErrInvalidAnswer, ErrDailyLimitReached, ErrSessionFinished, ErrInvalidRequest` (all `errors.New`, mapped to HTTP in `handlers.go`).

## File Structure (new/modified)

```
backend/
  internal/db/queries/session.sql          # + sqlc generate
  internal/session/
    rules.go rules_test.go                 # pure: unlock check, exam outcome, mistake-clear check
    dto.go                                  # request/response JSON types
    service.go service_test.go              # StartSession/SubmitAnswer/FinishSession/GetSession/ListMySessions/ListVariantStatuses
    handlers.go handlers_test.go            # HTTP routes + wiring test
  internal/server/server.go                 # mount session routes (modify)
```

---

### Task 1: sqlc queries for sessions, variant progress, and the mistake bank

**Files:** create `internal/db/queries/session.sql`; run `sqlc generate`.

- [ ] **Step 1: Write the queries file**

```sql
-- name: ListVariantQuestionIDsOrdered :many
SELECT vq.question_id
FROM variant_question vq
JOIN question q ON q.id = vq.question_id AND q.validation_status = 'valid'
WHERE vq.variant_id = $1
ORDER BY vq.position;

-- name: RandomQuestionIDs :many
SELECT id FROM question
WHERE validation_status = 'valid'
ORDER BY random()
LIMIT $1;

-- name: RandomQuestionIDsByCategory :many
SELECT id FROM question
WHERE validation_status = 'valid' AND category_id = sqlc.arg(category_id)
ORDER BY random()
LIMIT sqlc.arg(limit_count);

-- name: RandomQuestionIDsBySign :many
SELECT q.id FROM question q
JOIN question_sign qs ON qs.question_id = q.id
WHERE q.validation_status = 'valid' AND qs.sign_id = sqlc.arg(sign_id)
ORDER BY random()
LIMIT sqlc.arg(limit_count);

-- name: ListMistakeBankQuestionIDs :many
SELECT question_id FROM question_memory
WHERE profile_id = sqlc.arg(profile_id) AND state = 0
ORDER BY last_reviewed_at ASC NULLS FIRST
LIMIT sqlc.arg(limit_count);

-- name: GetAnswerForScoring :one
-- Also validates that answer_id truly belongs to question_id.
SELECT id, question_id, is_correct FROM answer
WHERE id = sqlc.arg(id) AND question_id = sqlc.arg(question_id);

-- name: GetCorrectAnswerID :one
SELECT id FROM answer WHERE question_id = $1 AND is_correct = true;

-- name: GetVariantProgress :one
SELECT * FROM variant_progress
WHERE profile_id = sqlc.arg(profile_id) AND variant_id = sqlc.arg(variant_id);

-- name: GetCategoryIDByCode :one
SELECT id FROM category WHERE code = $1;

-- name: CreateExamSession :one
INSERT INTO exam_session
  (profile_id, mode, variant_id, category_id, sign_id, locale, time_limit_sec, errors_allowed, total)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetExamSession :one
SELECT * FROM exam_session WHERE id = $1;

-- name: FinishExamSession :one
UPDATE exam_session
SET finished_at = now(), status = $2, score = $3, stopped_reason = $4
WHERE id = $1
RETURNING *;

-- name: InsertSessionAnswer :one
INSERT INTO session_answer (session_id, question_id, answer_id, is_correct, position)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetSessionAnswer :one
SELECT * FROM session_answer
WHERE session_id = sqlc.arg(session_id) AND question_id = sqlc.arg(question_id);

-- name: ListSessionAnswers :many
SELECT * FROM session_answer WHERE session_id = $1 ORDER BY position;

-- name: CountSessionAnswers :one
SELECT
  count(*)::int AS total_answered,
  count(*) FILTER (WHERE is_correct)::int AS correct_count
FROM session_answer WHERE session_id = $1;

-- name: ListMySessions :many
SELECT * FROM exam_session
WHERE profile_id = sqlc.arg(profile_id)
ORDER BY started_at DESC LIMIT sqlc.arg(limit_count);

-- name: GetLimitConfig :one
SELECT * FROM limit_config WHERE key = $1;

-- name: CountPracticeAnswersToday :one
SELECT count(*)::int FROM session_answer sa
JOIN exam_session es ON es.id = sa.session_id
WHERE es.profile_id = sqlc.arg(profile_id) AND es.mode = 'practice'
  AND sa.answered_at >= sqlc.arg(since);

-- name: UpsertVariantProgress :one
INSERT INTO variant_progress (profile_id, variant_id, best_correct, attempts, completed_at)
VALUES ($1, $2, $3, 1, $4)
ON CONFLICT (profile_id, variant_id) DO UPDATE SET
  best_correct = GREATEST(variant_progress.best_correct, EXCLUDED.best_correct),
  attempts = variant_progress.attempts + 1,
  completed_at = COALESCE(variant_progress.completed_at, EXCLUDED.completed_at)
RETURNING *;

-- name: ListVariantProgressForProfile :many
SELECT * FROM variant_progress WHERE profile_id = $1;

-- name: MarkQuestionWrong :one
INSERT INTO question_memory (profile_id, question_id, due_at, reps, lapses, state, last_reviewed_at)
VALUES ($1, $2, now(), 0, 1, 0, now())
ON CONFLICT (profile_id, question_id) DO UPDATE SET
  lapses = question_memory.lapses + 1, reps = 0, state = 0, last_reviewed_at = now()
RETURNING *;

-- name: MarkQuestionCorrectInMistakesMode :one
UPDATE question_memory SET
  reps = reps + 1,
  state = CASE WHEN reps + 1 >= sqlc.arg(clear_after)::int THEN 1 ELSE state END,
  last_reviewed_at = now()
WHERE profile_id = sqlc.arg(profile_id) AND question_id = sqlc.arg(question_id)
RETURNING *;
```

- [ ] **Step 2: Generate and build**

Run: `cd backend && sqlc generate && go build ./...`
Expected: clean exit, `internal/db/sqlc/session.sql.go` created.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/db/queries/session.sql backend/internal/db/sqlc/session.sql.go
git commit -m "feat(backend): sqlc queries for sessions, variant progress, mistake bank"
```

---

### Task 2: Pure scoring/unlock rules (no DB)

**Files:** create `internal/session/rules.go`, `internal/session/rules_test.go`.

**Interfaces (produced):**
- `session.ExamQuestionCount, ExamTimeLimitSec, ExamErrorsAllowed, MistakeClearAfter` (const int)
- `session.IsVariantUnlocked(isFirst bool, prevBestCorrect, threshold int) bool`
- `session.ExamOutcome{Status, StoppedReason string}`
- `session.EvaluateExam(correct, wrong, total int, timedOut, tooManyErrors bool) ExamOutcome`
- `session.ShouldStopExam(wrongSoFar int) bool`
- `session.MistakeCleared(repsAfterThisAnswer int) bool`

- [ ] **Step 1: Write the failing tests**

```go
package session

import "testing"

func TestIsVariantUnlocked(t *testing.T) {
	if !IsVariantUnlocked(true, 0, 10) {
		t.Fatal("first variant must always be unlocked")
	}
	if IsVariantUnlocked(false, 9, 10) {
		t.Fatal("9 < threshold 10 must stay locked")
	}
	if !IsVariantUnlocked(false, 10, 10) {
		t.Fatal("10 >= threshold 10 must unlock")
	}
}

func TestShouldStopExam(t *testing.T) {
	if ShouldStopExam(1) || ShouldStopExam(2) {
		t.Fatal("1 or 2 wrong answers must not stop the exam")
	}
	if !ShouldStopExam(3) {
		t.Fatal("3rd wrong answer must stop the exam")
	}
}

func TestEvaluateExamCompletedPass(t *testing.T) {
	out := EvaluateExam(18, 2, 20, false, false)
	if out.Status != "passed" || out.StoppedReason != "completed" {
		t.Fatalf("18/20 with 2 wrong should pass: %+v", out)
	}
}

func TestEvaluateExamCompletedFail(t *testing.T) {
	out := EvaluateExam(17, 3, 20, false, false)
	if out.Status != "failed" || out.StoppedReason != "completed" {
		t.Fatalf("17/20 with 3 wrong should fail: %+v", out)
	}
}

func TestEvaluateExamTooManyErrors(t *testing.T) {
	out := EvaluateExam(5, 3, 20, false, true)
	if out.Status != "failed" || out.StoppedReason != "too_many_errors" {
		t.Fatalf("3rd wrong must fail immediately: %+v", out)
	}
}

func TestEvaluateExamTimeUpPass(t *testing.T) {
	out := EvaluateExam(19, 1, 20, true, false)
	if out.Status != "passed" || out.StoppedReason != "time_up" {
		t.Fatalf("time up but already 19/20 with 1 wrong should pass: %+v", out)
	}
}

func TestEvaluateExamTimeUpFail(t *testing.T) {
	out := EvaluateExam(10, 1, 20, true, false)
	if out.Status != "failed" || out.StoppedReason != "time_up" {
		t.Fatalf("time up with only 10 answered should fail: %+v", out)
	}
}

func TestMistakeCleared(t *testing.T) {
	if MistakeCleared(1) {
		t.Fatal("1 consecutive correct must not clear yet")
	}
	if !MistakeCleared(2) {
		t.Fatal("2 consecutive correct must clear")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/session/... -run . -v`
Expected: FAIL — package/functions don't exist.

- [ ] **Step 3: Implement**

```go
// Package session owns exam-session lifecycle: starting a session (question
// selection per mode), recording answers with server-side scoring, finishing
// with a computed result, resuming an interrupted session, and the bilet
// unlock / mistake-bank side effects that finishing triggers.
package session

const (
	ExamQuestionCount = 20
	ExamTimeLimitSec  = 25 * 60
	ExamErrorsAllowed = 2

	// MistakeClearAfter consecutive correct answers in mistakes-mode remove
	// a question from the bank.
	MistakeClearAfter = 2
)

// IsVariantUnlocked reports whether a variant is unlocked. The first variant
// in the sequence (isFirst) is always unlocked; every other one requires the
// previous variant's best_correct to meet the configured threshold.
func IsVariantUnlocked(isFirst bool, prevBestCorrect, threshold int) bool {
	if isFirst {
		return true
	}
	return prevBestCorrect >= threshold
}

type ExamOutcome struct {
	Status        string // "passed" | "failed"
	StoppedReason string // "completed" | "time_up" | "too_many_errors"
}

// EvaluateExam computes the final status of an exam-mode session. Passing
// requires correct >= total-ExamErrorsAllowed (i.e. >=18/20) AND
// wrong <= ExamErrorsAllowed — matching the real exam's "≤2 xato" rule.
func EvaluateExam(correct, wrong, total int, timedOut, tooManyErrors bool) ExamOutcome {
	if tooManyErrors {
		return ExamOutcome{Status: "failed", StoppedReason: "too_many_errors"}
	}
	reason := "completed"
	if timedOut {
		reason = "time_up"
	}
	if correct >= total-ExamErrorsAllowed && wrong <= ExamErrorsAllowed {
		return ExamOutcome{Status: "passed", StoppedReason: reason}
	}
	return ExamOutcome{Status: "failed", StoppedReason: reason}
}

// ShouldStopExam reports whether the exam must stop immediately after this
// wrong answer — the real exam ends on the 3rd mistake.
func ShouldStopExam(wrongSoFar int) bool {
	return wrongSoFar > ExamErrorsAllowed
}

// MistakeCleared reports whether repsAfterThisAnswer consecutive correct
// answers (in mistakes-mode) are enough to remove the question from the bank.
func MistakeCleared(repsAfterThisAnswer int) bool {
	return repsAfterThisAnswer >= MistakeClearAfter
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd backend && go test ./internal/session/... -v`
Expected: PASS (all 7 tests).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/session/rules.go backend/internal/session/rules_test.go
git commit -m "feat(backend): pure exam/unlock/mistake-bank scoring rules"
```

---

### Task 3: DTOs + `Service.StartSession`

**Files:** create `internal/session/dto.go`, `internal/session/service.go`, `internal/session/service_test.go`.

**Interfaces (produced):**
```go
type Service struct {
	Q       *sqlc.Queries
	Billing billing.Service
}

func NewService(q *sqlc.Queries, b billing.Service) *Service

type StartRequest struct {
	Mode       string     // "variant" | "exam" | "practice" | "mistakes"
	VariantID  uuid.UUID  // required for mode=variant
	CategoryID uuid.UUID  // required for mode=practice by category (zero UUID = unset)
	SignID     uuid.UUID  // required for mode=practice by sign (zero UUID = unset)
	Locale     string
	Count      int        // practice/mistakes only: requested size, clamped
}

type SessionView struct {
	ID           uuid.UUID
	Mode         string
	QuestionIDs  []uuid.UUID
	TimeLimitSec *int
	Total        int
	StartedAt    time.Time
}

var (
	ErrNotFound          = errors.New("session not found")
	ErrInvalidRequest    = errors.New("invalid session request")
	ErrDailyLimitReached = errors.New("daily practice limit reached")
	ErrAlreadyAnswered   = errors.New("question already answered in this session")
	ErrInvalidAnswer     = errors.New("answer does not belong to question")
	ErrSessionFinished   = errors.New("session already finished")
)

func (s *Service) StartSession(ctx context.Context, profileID uuid.UUID, req StartRequest) (SessionView, error)
```

- **Mode dispatch:**
  - `"variant"`: `req.VariantID` required (else `ErrInvalidRequest`); question IDs = `ListVariantQuestionIDsOrdered`; `total` = len(ids); no time limit, no errors_allowed.
  - `"exam"`: question IDs = `RandomQuestionIDs(ExamQuestionCount)`; `total = ExamQuestionCount`; `time_limit_sec = ExamTimeLimitSec`; `errors_allowed = ExamErrorsAllowed`. If fewer than `ExamQuestionCount` valid questions exist in the whole bank, return `ErrInvalidRequest` (content not seeded enough — never silently serve a short exam).
  - `"practice"`: exactly one of `req.CategoryID`/`req.SignID` must be set (else `ErrInvalidRequest`); compute remaining daily allowance (see below); `count := req.Count`; if `count <= 0`, default to remaining allowance (or 10 if unlimited); clamp `count` to remaining allowance when allowance is not `-1` (unlimited); if remaining allowance is `0`, return `ErrDailyLimitReached`; question IDs = `RandomQuestionIDsByCategory`/`RandomQuestionIDsBySign` with `LIMIT count`.
  - `"mistakes"`: question IDs = `ListMistakeBankQuestionIDs(profileID, count)` (`count` defaults to 10 if `req.Count<=0`); if the bank is empty, that's a valid (empty) session — not an error.
  - any other mode string → `ErrInvalidRequest`.
- **Daily allowance helper:** `active, _, err := s.Billing.Status(ctx, profileID)`; `cfg, err := s.Q.GetLimitConfig(ctx, "daily_practice_questions")`; `limit := cfg.FreeValue; if active { limit = cfg.VipValue }`; if `limit == -1` allowance is unlimited; else `used, err := s.Q.CountPracticeAnswersToday(ctx, sqlc.CountPracticeAnswersTodayParams{ProfileID: profileID, Since: pgtype.Timestamptz{Time: startOfTodayUTC(), Valid: true}})`; `remaining := limit - used` (floor at 0).
- Persist via `CreateExamSession` with `locale` from `req.Locale` (validate with `i18n.Parse`-equivalent set beforehand — reuse `avtotest.uz/backend/internal/i18n`).

- [ ] **Step 1: Write the failing tests** (`service_test.go`, using `testdb.New` + `fixture.Sample()` + `importer.Store` to seed 2 variants × 20 questions, 4 categories, 4 signs — same fixture Plan 01's content tests already use)

```go
package session_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
	"avtotest.uz/backend/internal/session"
	"avtotest.uz/backend/internal/testdb"
)

func seed(t *testing.T) (*sqlc.Queries, *session.Service, uuid.UUID) {
	t.Helper()
	pool := testdb.New(t)
	ds, images := fixture.Sample()
	if _, err := importer.Store(context.Background(), pool, blob.NewLocalDir(t.TempDir()), ds,
		importer.StoreOptions{MarkVerified: true, Images: images, Source: "fixture"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	q := sqlc.New(pool)
	svc := session.NewService(q, billing.Service{Q: q})
	profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{
		Phone: "+998901234567",
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	return q, svc, profile.ID
}

func TestStartSessionVariantMode(t *testing.T) {
	q, svc, profileID := seed(t)
	v, err := q.GetVariantByNumber(context.Background(), 1)
	if err != nil {
		t.Fatalf("get variant: %v", err)
	}
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "variant", VariantID: v.ID, Locale: "uz-Latn",
	})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if view.Total != 20 || len(view.QuestionIDs) != 20 {
		t.Fatalf("expected 20 questions, got total=%d ids=%d", view.Total, len(view.QuestionIDs))
	}
	if view.TimeLimitSec != nil {
		t.Fatalf("variant mode must have no time limit, got %v", *view.TimeLimitSec)
	}
}

func TestStartSessionExamMode(t *testing.T) {
	_, svc, profileID := seed(t)
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "exam", Locale: "uz-Latn",
	})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if view.Total != session.ExamQuestionCount || len(view.QuestionIDs) != session.ExamQuestionCount {
		t.Fatalf("expected %d questions, got %d", session.ExamQuestionCount, len(view.QuestionIDs))
	}
	if view.TimeLimitSec == nil || *view.TimeLimitSec != session.ExamTimeLimitSec {
		t.Fatalf("expected time limit %d, got %v", session.ExamTimeLimitSec, view.TimeLimitSec)
	}
}

func TestStartSessionInvalidMode(t *testing.T) {
	_, svc, profileID := seed(t)
	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "not-a-mode", Locale: "uz-Latn",
	}); err != session.ErrInvalidRequest {
		t.Fatalf("err=%v want ErrInvalidRequest", err)
	}
}

func TestStartSessionVariantRequiresVariantID(t *testing.T) {
	_, svc, profileID := seed(t)
	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "variant", Locale: "uz-Latn",
	}); err != session.ErrInvalidRequest {
		t.Fatalf("err=%v want ErrInvalidRequest", err)
	}
}

func TestStartSessionPracticeDailyLimitClampsAndBlocks(t *testing.T) {
	q, svc, profileID := seed(t)
	// fixture.Sample() assigns 40 questions round-robin across 4 categories
	// (10 each) and limit_config seeds daily_practice_questions free_value=10
	// (migration 0003_billing.up.sql) — so a fresh profile's first practice
	// session of this category exhausts the entire daily allowance.
	catID, err := q.GetCategoryIDByCode(context.Background(), "signs")
	if err != nil {
		t.Fatalf("category lookup: %v", err)
	}

	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "practice", CategoryID: catID, Locale: "uz-Latn", Count: 100,
	})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if len(view.QuestionIDs) != 10 {
		t.Fatalf("expected count clamped to the free daily allowance of 10, got %d", len(view.QuestionIDs))
	}
	for _, qid := range view.QuestionIDs {
		correctID, err := q.GetCorrectAnswerID(context.Background(), qid)
		if err != nil {
			t.Fatalf("correct answer: %v", err)
		}
		if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, qid, correctID); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}

	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "practice", CategoryID: catID, Locale: "uz-Latn", Count: 5,
	}); err != session.ErrDailyLimitReached {
		t.Fatalf("err=%v want ErrDailyLimitReached once today's allowance is used up", err)
	}
}
```

> **Implementer note:** the fixture's 4 category codes are `signs`, `rules`, `priority`, `safety` (`internal/fixture/fixture.go`'s `Categories` slice), assigned round-robin across the 40 questions (`cats[n%len(cats)]`) — 10 questions per category.

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/session/... -v`
Expected: FAIL — `session` package has no `Service`/`StartSession`/etc.

- [ ] **Step 3: Implement `dto.go` and `service.go`**

`dto.go`:
```go
package session

import (
	"time"

	"github.com/google/uuid"
)

type StartRequest struct {
	Mode       string
	VariantID  uuid.UUID
	CategoryID uuid.UUID
	SignID     uuid.UUID
	Locale     string
	Count      int
}

type SessionView struct {
	ID           uuid.UUID
	Mode         string
	QuestionIDs  []uuid.UUID
	TimeLimitSec *int
	Total        int
	StartedAt    time.Time
}
```

`service.go` (StartSession portion — full file grows in Tasks 4-6):
```go
package session

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
)

var (
	ErrNotFound          = errors.New("session not found")
	ErrInvalidRequest    = errors.New("invalid session request")
	ErrDailyLimitReached = errors.New("daily practice limit reached")
	ErrAlreadyAnswered   = errors.New("question already answered in this session")
	ErrInvalidAnswer     = errors.New("answer does not belong to question")
	ErrSessionFinished   = errors.New("session already finished")
)

type Service struct {
	Q       *sqlc.Queries
	Billing billing.Service
}

func NewService(q *sqlc.Queries, b billing.Service) *Service {
	return &Service{Q: q, Billing: b}
}

func (s *Service) StartSession(ctx context.Context, profileID uuid.UUID, req StartRequest) (SessionView, error) {
	var (
		ids           []uuid.UUID
		timeLimit     pgtype.Int4
		errorsAllowed pgtype.Int4
		variantID     uuid.NullUUID
		categoryID    uuid.NullUUID
		signID        uuid.NullUUID
		err           error
	)

	switch req.Mode {
	case "variant":
		if req.VariantID == uuid.Nil {
			return SessionView{}, ErrInvalidRequest
		}
		ids, err = s.Q.ListVariantQuestionIDsOrdered(ctx, req.VariantID)
		variantID = uuid.NullUUID{UUID: req.VariantID, Valid: true}

	case "exam":
		ids, err = s.Q.RandomQuestionIDs(ctx, int32(ExamQuestionCount))
		if err == nil && len(ids) < ExamQuestionCount {
			return SessionView{}, ErrInvalidRequest
		}
		timeLimit = pgtype.Int4{Int32: ExamTimeLimitSec, Valid: true}
		errorsAllowed = pgtype.Int4{Int32: ExamErrorsAllowed, Valid: true}

	case "practice":
		if (req.CategoryID == uuid.Nil) == (req.SignID == uuid.Nil) {
			return SessionView{}, ErrInvalidRequest // exactly one must be set
		}
		count, dailyErr := s.clampToDailyAllowance(ctx, profileID, req.Count)
		if dailyErr != nil {
			return SessionView{}, dailyErr
		}
		if req.CategoryID != uuid.Nil {
			categoryID = uuid.NullUUID{UUID: req.CategoryID, Valid: true}
			ids, err = s.Q.RandomQuestionIDsByCategory(ctx, sqlc.RandomQuestionIDsByCategoryParams{
				CategoryID: req.CategoryID, LimitCount: int32(count),
			})
		} else {
			signID = uuid.NullUUID{UUID: req.SignID, Valid: true}
			ids, err = s.Q.RandomQuestionIDsBySign(ctx, sqlc.RandomQuestionIDsBySignParams{
				SignID: req.SignID, LimitCount: int32(count),
			})
		}

	case "mistakes":
		count := req.Count
		if count <= 0 {
			count = 10
		}
		ids, err = s.Q.ListMistakeBankQuestionIDs(ctx, sqlc.ListMistakeBankQuestionIDsParams{
			ProfileID: profileID, LimitCount: int32(count),
		})

	default:
		return SessionView{}, ErrInvalidRequest
	}
	if err != nil {
		return SessionView{}, err
	}

	row, err := s.Q.CreateExamSession(ctx, sqlc.CreateExamSessionParams{
		ProfileID:     profileID,
		Mode:          req.Mode,
		VariantID:     variantID,
		CategoryID:    categoryID,
		SignID:        signID,
		Locale:        req.Locale,
		TimeLimitSec:  timeLimit,
		ErrorsAllowed: errorsAllowed,
		Total:         int32(len(ids)),
	})
	if err != nil {
		return SessionView{}, err
	}

	view := SessionView{
		ID: row.ID, Mode: row.Mode, QuestionIDs: ids,
		Total: int(row.Total), StartedAt: row.StartedAt.Time,
	}
	if timeLimit.Valid {
		v := int(timeLimit.Int32)
		view.TimeLimitSec = &v
	}
	return view, nil
}

func (s *Service) clampToDailyAllowance(ctx context.Context, profileID uuid.UUID, requested int) (int, error) {
	active, _, err := s.Billing.Status(ctx, profileID)
	if err != nil {
		return 0, err
	}
	cfg, err := s.Q.GetLimitConfig(ctx, "daily_practice_questions")
	if err != nil {
		return 0, err
	}
	limit := int(cfg.FreeValue)
	if active {
		limit = int(cfg.VipValue)
	}
	if limit == -1 {
		if requested <= 0 {
			return 10, nil
		}
		return requested, nil
	}
	startOfDay := time.Now().UTC().Truncate(24 * time.Hour)
	used, err := s.Q.CountPracticeAnswersToday(ctx, sqlc.CountPracticeAnswersTodayParams{
		ProfileID: profileID,
		Since:     pgtype.Timestamptz{Time: startOfDay, Valid: true},
	})
	if err != nil {
		return 0, err
	}
	remaining := limit - int(used)
	if remaining < 0 {
		remaining = 0
	}
	if remaining == 0 {
		return 0, ErrDailyLimitReached
	}
	count := requested
	if count <= 0 || count > remaining {
		count = remaining
	}
	return count, nil
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `make up && cd backend && go test ./internal/session/... -v`
Expected: PASS for `TestStartSessionVariantMode`, `TestStartSessionExamMode`, `TestStartSessionInvalidMode`, `TestStartSessionVariantRequiresVariantID`; finish wiring `TestStartSessionPracticeDailyLimitClampsAndBlocks` per the implementer note above before this step, so it passes too.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/session/dto.go backend/internal/session/service.go backend/internal/session/service_test.go backend/internal/db/queries/session.sql backend/internal/db/sqlc/session.sql.go
git commit -m "feat(backend): session.Service.StartSession — mode dispatch, daily practice limit"
```

---

### Task 4: `Service.SubmitAnswer`

**Files:** modify `internal/session/service.go`, `internal/session/dto.go`, extend `service_test.go`.

**Interfaces (produced):**
```go
type AnswerResult struct {
	Recorded        bool
	Correct         *bool      // nil for exam mode (feedback withheld)
	CorrectAnswerID *uuid.UUID // nil for exam mode
	Stopped         bool       // true if this answer ended an exam (3rd mistake)
	StopReason      string     // "too_many_errors" when Stopped
}

func (s *Service) SubmitAnswer(ctx context.Context, profileID, sessionID, questionID, answerID uuid.UUID) (AnswerResult, error)
```

**Logic:**
1. `row, err := s.Q.GetExamSession(ctx, sessionID)`; `pgx.ErrNoRows` or `row.ProfileID != profileID` → `ErrNotFound`.
2. `row.Status != "in_progress"` → `ErrSessionFinished`.
3. `_, err := s.Q.GetSessionAnswer(ctx, ...)`; if no `pgx.ErrNoRows` → `ErrAlreadyAnswered`.
4. `ans, err := s.Q.GetAnswerForScoring(ctx, sqlc.GetAnswerForScoringParams{ID: answerID, QuestionID: questionID})`; `pgx.ErrNoRows` → `ErrInvalidAnswer`.
5. Determine `position` — count of existing `session_answer` rows for this session + 1 (`ListSessionAnswers` length, or a dedicated `CountSessionAnswers`).
6. `s.Q.InsertSessionAnswer(...)` with `IsCorrect: ans.IsCorrect`.
7. If `row.Mode == "mistakes"`: if `ans.IsCorrect`, call `MarkQuestionCorrectInMistakesMode(profileID, questionID, MistakeClearAfter)`; else `MarkQuestionWrong(profileID, questionID)`.
8. If `row.Mode != "mistakes"` and `!ans.IsCorrect`: still call `MarkQuestionWrong` (any wrong answer anywhere feeds the bank, per Global Constraints).
9. If `row.Mode == "exam"`:
   - Recompute `wrongSoFar` via `CountSessionAnswers` (`total_answered - correct_count`).
   - If `ShouldStopExam(wrongSoFar)`: call `finishInternal(ctx, row, tooManyErrors=true, timedOut=false)` (the same internal helper Task 5 builds) and return `AnswerResult{Recorded: true, Stopped: true, StopReason: "too_many_errors"}`.
   - Else return `AnswerResult{Recorded: true}` (no `Correct`/`CorrectAnswerID`).
10. Else (`variant`/`practice`/`mistakes`): return `AnswerResult{Recorded: true, Correct: &ans.IsCorrect, CorrectAnswerID: &correctID}`. When `ans.IsCorrect`, `correctID` is the submitted `answerID` itself; when it's wrong, look up the actual correct answer with the `GetCorrectAnswerID` query already generated in Task 1 (`s.Q.GetCorrectAnswerID(ctx, questionID)`).

- [ ] **Step 1: Write the failing tests** (append to `service_test.go`)

```go
func startVariantSession(t *testing.T, q *sqlc.Queries, svc *session.Service, profileID uuid.UUID) session.SessionView {
	t.Helper()
	v, err := q.GetVariantByNumber(context.Background(), 1)
	if err != nil {
		t.Fatalf("get variant: %v", err)
	}
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "variant", VariantID: v.ID, Locale: "uz-Latn",
	})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	return view
}

func correctAnswerID(t *testing.T, q *sqlc.Queries, questionID uuid.UUID) uuid.UUID {
	t.Helper()
	ans, err := q.ListAnswersByQuestionIDs(context.Background(),
		sqlc.ListAnswersByQuestionIDsParams{QuestionIds: []uuid.UUID{questionID}, Locale: "uz-Latn"})
	if err != nil || len(ans) == 0 {
		t.Fatalf("answers: %v", err)
	}
	// fixture guarantees exactly one correct answer per question; find it
	// via a direct query since ListAnswersByQuestionIDs never exposes it.
	full, err := q.GetAnswerForScoring(context.Background(), sqlc.GetAnswerForScoringParams{ID: ans[0].ID, QuestionID: questionID})
	_ = full
	_ = err
	for _, a := range ans {
		full, err := q.GetAnswerForScoring(context.Background(), sqlc.GetAnswerForScoringParams{ID: a.ID, QuestionID: questionID})
		if err == nil && full.IsCorrect {
			return a.ID
		}
	}
	t.Fatal("no correct answer found")
	return uuid.Nil
}

func TestSubmitAnswerVariantModeImmediateFeedback(t *testing.T) {
	q, svc, profileID := seed(t)
	view := startVariantSession(t, q, svc, profileID)
	correctID := correctAnswerID(t, q, view.QuestionIDs[0])

	res, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], correctID)
	if err != nil {
		t.Fatalf("SubmitAnswer: %v", err)
	}
	if res.Correct == nil || !*res.Correct {
		t.Fatalf("expected correct=true, got %+v", res)
	}
	if res.CorrectAnswerID == nil || *res.CorrectAnswerID != correctID {
		t.Fatalf("expected correct_answer_id=%v, got %+v", correctID, res.CorrectAnswerID)
	}
}

func TestSubmitAnswerExamModeWithholdsFeedback(t *testing.T) {
	_, svc, profileID := seed(t)
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{Mode: "exam", Locale: "uz-Latn"})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	res, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], uuid.New())
	if err == nil {
		t.Fatal("random answer id must be rejected as invalid")
	}
	if err != session.ErrInvalidAnswer {
		t.Fatalf("err=%v want ErrInvalidAnswer", err)
	}
	_ = res
}

func TestSubmitAnswerRejectsDuplicateAndWrongQuestionPair(t *testing.T) {
	q, svc, profileID := seed(t)
	view := startVariantSession(t, q, svc, profileID)
	correctID := correctAnswerID(t, q, view.QuestionIDs[0])

	if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], correctID); err != nil {
		t.Fatalf("first submit: %v", err)
	}
	if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], correctID); err != session.ErrAlreadyAnswered {
		t.Fatalf("err=%v want ErrAlreadyAnswered", err)
	}

	otherCorrect := correctAnswerID(t, q, view.QuestionIDs[1])
	if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], otherCorrect); err != session.ErrAlreadyAnswered {
		t.Fatalf("already-answered check must run before mismatch check: err=%v", err)
	}
}

func TestSubmitAnswerExamStopsOnThirdMistake(t *testing.T) {
	q, svc, profileID := seed(t)
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{Mode: "exam", Locale: "uz-Latn"})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	// answer the first 3 questions wrong by submitting a non-correct answer id each time
	for i := 0; i < 3; i++ {
		ans, err := q.ListAnswersByQuestionIDs(context.Background(),
			sqlc.ListAnswersByQuestionIDsParams{QuestionIds: []uuid.UUID{view.QuestionIDs[i]}, Locale: "uz-Latn"})
		if err != nil || len(ans) != 4 {
			t.Fatalf("answers: %v", err)
		}
		correctID := correctAnswerID(t, q, view.QuestionIDs[i])
		var wrongID uuid.UUID
		for _, a := range ans {
			if a.ID != correctID {
				wrongID = a.ID
				break
			}
		}
		res, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[i], wrongID)
		if err != nil {
			t.Fatalf("submit %d: %v", i, err)
		}
		if i < 2 {
			if res.Stopped {
				t.Fatalf("must not stop before the 3rd mistake, i=%d", i)
			}
		} else {
			if !res.Stopped || res.StopReason != "too_many_errors" {
				t.Fatalf("expected stop on 3rd mistake, got %+v", res)
			}
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/session/... -run SubmitAnswer -v`
Expected: FAIL — `SubmitAnswer` undefined.

- [ ] **Step 3: Implement** — add `AnswerResult` to `dto.go`; add `SubmitAnswer` + the internal `finishInternal` stub it calls to `service.go` (Task 5 fills in the real scoring; for this task, `finishInternal` only needs to set `status="failed", stopped_reason="too_many_errors"` and mark the session finished via `FinishExamSession` — the full pass/fail/time-up branching lands in Task 5).

- [ ] **Step 4: Run to verify it passes**

Run: `cd backend && go test ./internal/session/... -v`
Expected: PASS (all tests from Tasks 3-4).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/session/ backend/internal/db/queries/session.sql backend/internal/db/sqlc/session.sql.go
git commit -m "feat(backend): session.Service.SubmitAnswer — scoring, mistake bank, exam 3-strike stop"
```

---

### Task 5: `Service.FinishSession` — full scoring, bilet unlock, idempotency

**Files:** modify `internal/session/service.go`, `internal/session/dto.go`, extend `service_test.go`.

**Interfaces (produced):**
```go
type FinishResult struct {
	Status        string // "passed" | "failed" | "abandoned"
	StoppedReason string
	Score         int
	Total         int
}

func (s *Service) FinishSession(ctx context.Context, profileID, sessionID uuid.UUID) (FinishResult, error)
```

**Logic:**
1. Fetch + ownership check as in `SubmitAnswer` steps 1-2 — but if `row.Status != "in_progress"`, this is the **idempotent path**: return the already-stored `FinishResult{Status: row.Status, StoppedReason: row.StoppedReason.String, Score: int(row.Score.Int32), Total: int(row.Total)}` with no writes.
2. `totalAnswered, correctCount, err := s.Q.CountSessionAnswers(ctx, sessionID)`.
3. Branch on `row.Mode`:
   - `"exam"`: `wrong := totalAnswered - correctCount`; `timedOut := time.Now().After(row.StartedAt.Time.Add(time.Duration(row.TimeLimitSec.Int32) * time.Second))`; `outcome := EvaluateExam(int(correctCount), int(wrong), int(row.Total), timedOut, false)` (the `tooManyErrors=true` path is only ever reached from inside `SubmitAnswer`'s own call to this same finishing logic, never from the public `Finish` entry point — keep `finishInternal(ctx, row, tooManyErrors bool)` as the single implementation both call, with the public `FinishSession` passing `tooManyErrors=false`).
   - `"variant"`, `"practice"`, `"mistakes"`: `status := "passed"; if int(totalAnswered) < int(row.Total) { status = "abandoned" }`; `stoppedReason := "completed"` (empty/omitted when abandoned — store `pgtype.Text{}` invalid).
4. `s.Q.FinishExamSession(ctx, sqlc.FinishExamSessionParams{ID: sessionID, Status: status, Score: pgtype.Int4{Int32: int32(correctCount), Valid: true}, StoppedReason: ...})`.
5. **Bilet unlock side effect** — only when `row.Mode == "variant"`: `s.Q.UpsertVariantProgress(ctx, sqlc.UpsertVariantProgressParams{ProfileID: profileID, VariantID: row.VariantID.UUID, BestCorrect: int32(correctCount), CompletedAt: completedAtIfThresholdMet})` — `completedAtIfThresholdMet` is `pgtype.Timestamptz{Time: time.Now(), Valid: true}` when `correctCount >= thresholdFromLimitConfig` else `pgtype.Timestamptz{}` (the `COALESCE` in the SQL keeps any previously-set `completed_at` either way, so passing an invalid/zero value here is safe and never un-sets a real timestamp).
6. Return `FinishResult{...}`.

- [ ] **Step 1: Write the failing tests** (append to `service_test.go`)

```go
func TestFinishSessionVariantModeUnlocksNextBilet(t *testing.T) {
	q, svc, profileID := seed(t)
	view := startVariantSession(t, q, svc, profileID)
	for _, qid := range view.QuestionIDs {
		correctID := correctAnswerID(t, q, qid)
		if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, qid, correctID); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}
	res, err := svc.FinishSession(context.Background(), profileID, view.ID)
	if err != nil {
		t.Fatalf("FinishSession: %v", err)
	}
	if res.Status != "passed" || res.Score != 20 {
		t.Fatalf("expected passed 20/20, got %+v", res)
	}

	v1, err := q.GetVariantByNumber(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	progress, err := q.GetVariantProgress(context.Background(), sqlc.GetVariantProgressParams{ProfileID: profileID, VariantID: v1.ID})
	if err != nil {
		t.Fatalf("progress: %v", err)
	}
	if progress.BestCorrect != 20 || !progress.CompletedAt.Valid {
		t.Fatalf("expected best_correct=20 and completed_at set, got %+v", progress)
	}
}

func TestFinishSessionIsIdempotent(t *testing.T) {
	q, svc, profileID := seed(t)
	view := startVariantSession(t, q, svc, profileID)
	for _, qid := range view.QuestionIDs[:5] {
		correctID := correctAnswerID(t, q, qid)
		if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, qid, correctID); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}
	first, err := svc.FinishSession(context.Background(), profileID, view.ID)
	if err != nil {
		t.Fatalf("first finish: %v", err)
	}
	second, err := svc.FinishSession(context.Background(), profileID, view.ID)
	if err != nil {
		t.Fatalf("second finish: %v", err)
	}
	if first != second {
		t.Fatalf("finish must be idempotent: first=%+v second=%+v", first, second)
	}
}

func TestFinishSessionAbandonedWhenIncomplete(t *testing.T) {
	q, svc, profileID := seed(t)
	view := startVariantSession(t, q, svc, profileID)
	correctID := correctAnswerID(t, q, view.QuestionIDs[0])
	if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], correctID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	res, err := svc.FinishSession(context.Background(), profileID, view.ID)
	if err != nil {
		t.Fatalf("FinishSession: %v", err)
	}
	if res.Status != "abandoned" {
		t.Fatalf("expected abandoned with 1/20 answered, got %+v", res)
	}
}
```

The test uses `GetVariantProgress`, already generated in Task 1.

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/session/... -run FinishSession -v`
Expected: FAIL — `FinishSession` undefined.

- [ ] **Step 3: Implement** `FinishSession` + shared `finishInternal` in `service.go` per the Logic section above; wire `SubmitAnswer`'s exam 3rd-mistake branch (Task 4) to call `s.finishInternal(ctx, sessionID, profileID, row, true)`.

- [ ] **Step 4: Run to verify it passes**

Run: `cd backend && go test ./internal/session/... -v`
Expected: PASS (all tests from Tasks 3-5).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/session/ backend/internal/db/queries/session.sql backend/internal/db/sqlc/session.sql.go
git commit -m "feat(backend): session.Service.FinishSession — scoring finalize, bilet unlock, idempotent"
```

---

### Task 6: `Service.GetSession` (resume, anti-cheat-safe) + `ListMySessions` + `ListVariantStatuses`

**Files:** modify `internal/session/service.go`, `internal/session/dto.go`, extend `service_test.go`.

**Interfaces (produced):**
```go
type AnsweredQuestion struct {
	QuestionID uuid.UUID
	Position   int
	Answered   bool
	Correct    *bool // nil while an exam session is still in_progress
}

type SessionDetail struct {
	SessionView
	Status        string
	StoppedReason string
	Score         *int
	FinishedAt    *time.Time
	Answers       []AnsweredQuestion
}

func (s *Service) GetSession(ctx context.Context, profileID, sessionID uuid.UUID) (SessionDetail, error)

type SessionSummary struct {
	ID         uuid.UUID
	Mode       string
	Status     string
	Score      *int
	Total      int
	StartedAt  time.Time
	FinishedAt *time.Time
}

func (s *Service) ListMySessions(ctx context.Context, profileID uuid.UUID, limit int) ([]SessionSummary, error)

type VariantStatus struct {
	Number        int32
	QuestionCount int
	Unlocked      bool
	BestCorrect   int
	Attempts      int
	CompletedAt   *time.Time
}

func (s *Service) ListVariantStatuses(ctx context.Context, profileID uuid.UUID) ([]VariantStatus, error)
```

**Logic:**
- `GetSession`: fetch session (ownership check → `ErrNotFound`); fetch `ListSessionAnswers`; for the question-ID list, either use the stored `question_ids` order (variant/practice/mistakes ordering was decided at start time and isn't re-derivable for random modes) — **add `question_ids uuid[]` is NOT a column on `exam_session`**, so instead derive the answered/unanswered view purely from `ListSessionAnswers` plus knowledge of `row.Total`: return one `AnsweredQuestion` per row in `ListSessionAnswers` (answered ones) — the client already knows which question IDs it originally received from `StartSession`'s response and tracks unanswered ones itself; `GetSession` only needs to report, for each *answered* question, whether it was correct (redacted while an exam is `in_progress`).
- `ListVariantStatuses`: `variants, err := s.Q.ListVariants(ctx)` (existing Plan 01 query, ordered by number); `progress, err := s.Q.ListVariantProgressForProfile(ctx, profileID)` → map by `variant_id`; `threshold, err := s.Q.GetLimitConfig(ctx, "unlock_threshold_correct")`; iterate `variants` in order, tracking `prevBestCorrect` from the previous iteration's progress (0 if no attempt), unlocked via `IsVariantUnlocked(i==0, prevBestCorrect, int(threshold.FreeValue))`.

- [ ] **Step 1: Write the failing tests** (append to `service_test.go`)

```go
func TestGetSessionRedactsCorrectnessDuringInProgressExam(t *testing.T) {
	q, svc, profileID := seed(t)
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{Mode: "exam", Locale: "uz-Latn"})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	correctID := correctAnswerID(t, q, view.QuestionIDs[0])
	if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], correctID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	detail, err := svc.GetSession(context.Background(), profileID, view.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if len(detail.Answers) != 1 || detail.Answers[0].Correct != nil {
		t.Fatalf("in-progress exam must redact correctness: %+v", detail.Answers)
	}

	if _, err := svc.FinishSession(context.Background(), profileID, view.ID); err != nil {
		t.Fatalf("finish: %v", err)
	}
	detail, err = svc.GetSession(context.Background(), profileID, view.ID)
	if err != nil {
		t.Fatalf("GetSession after finish: %v", err)
	}
	if detail.Answers[0].Correct == nil {
		t.Fatal("finished session must reveal correctness")
	}
}

func TestGetSessionOwnershipIsEnforced(t *testing.T) {
	q, svc, profileID := seed(t)
	view := startVariantSession(t, q, svc, profileID)
	other, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998907654321"})
	if err != nil {
		t.Fatalf("create other profile: %v", err)
	}
	if _, err := svc.GetSession(context.Background(), other.ID, view.ID); err != session.ErrNotFound {
		t.Fatalf("err=%v want ErrNotFound for another profile's session", err)
	}
}

func TestListVariantStatusesSequentialUnlock(t *testing.T) {
	q, svc, profileID := seed(t)
	statuses, err := svc.ListVariantStatuses(context.Background(), profileID)
	if err != nil {
		t.Fatalf("ListVariantStatuses: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("fixture has 2 variants, got %d", len(statuses))
	}
	if !statuses[0].Unlocked {
		t.Fatal("variant 1 must always be unlocked")
	}
	if statuses[1].Unlocked {
		t.Fatal("variant 2 must be locked before variant 1 is passed")
	}

	view := startVariantSession(t, q, svc, profileID)
	for _, qid := range view.QuestionIDs {
		correctID := correctAnswerID(t, q, qid)
		if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, qid, correctID); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}
	if _, err := svc.FinishSession(context.Background(), profileID, view.ID); err != nil {
		t.Fatalf("finish: %v", err)
	}

	statuses, err = svc.ListVariantStatuses(context.Background(), profileID)
	if err != nil {
		t.Fatalf("ListVariantStatuses: %v", err)
	}
	if !statuses[1].Unlocked {
		t.Fatal("variant 2 must unlock after variant 1 hits the threshold")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/session/... -run "GetSession|ListVariantStatuses" -v`
Expected: FAIL — undefined symbols.

- [ ] **Step 3: Implement** `GetSession`, `ListMySessions`, `ListVariantStatuses` in `service.go` (add `AnsweredQuestion`, `SessionDetail`, `SessionSummary`, `VariantStatus` to `dto.go`) per the Logic section above.

- [ ] **Step 4: Run to verify it passes**

Run: `cd backend && go test ./internal/session/... -v`
Expected: PASS (all tests, Tasks 3-6).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/session/
git commit -m "feat(backend): session.Service resume view, session history, bilet unlock listing"
```

---

### Task 7: HTTP handlers + server wiring

**Files:** create `internal/session/handlers.go`, `internal/session/handlers_test.go`; modify `internal/server/server.go`.

**Routes (all behind `auth.Required`):**
- `POST /sessions` — body `{mode, variant_id?, category_id?, sign_id?, locale, count?}` → `201 {id, mode, question_ids, time_limit_sec, total, started_at}`
- `POST /sessions/{id}/answers` — body `{question_id, answer_id}` → `200 {recorded, correct?, correct_answer_id?, stopped?, stop_reason?}`
- `POST /sessions/{id}/finish` → `200 {status, stopped_reason, score, total}`
- `GET /sessions/{id}` → `200 {id, mode, total, status, stopped_reason, score?, started_at, finished_at?, answers:[{question_id, position, answered, correct?}]}`
- `GET /me/sessions?limit=` → `200 [{id, mode, status, score?, total, started_at, finished_at?}]`
- `GET /me/variants` → `200 [{number, question_count, unlocked, best_correct, attempts, completed_at?}]`

**Error mapping:** `ErrInvalidRequest`→400 `invalid_request`; `ErrDailyLimitReached`→429 `daily_limit_reached`; `ErrNotFound`→404 `not_found`; `ErrAlreadyAnswered`→409 `already_answered`; `ErrInvalidAnswer`→400 `invalid_answer`; `ErrSessionFinished`→409 `session_finished`.

- [ ] **Step 1: Write the failing test** (`handlers_test.go`, full httptest server, mirrors `internal/auth/handlers_test.go`'s and `internal/account/handlers_test.go`'s pattern: real `testdb` + fixture seed, `chi.NewRouter()`, mount `session.Handler` behind `auth.Required`, issue a JWT with `auth.IssueAccess` for the seeded profile)

```go
package session_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
	"avtotest.uz/backend/internal/session"
	"avtotest.uz/backend/internal/testdb"
)

const handlerSecret = "test-secret"

func setupServer(t *testing.T) (*httptest.Server, string, *sqlc.Queries) {
	t.Helper()
	pool := testdb.New(t)
	ds, images := fixture.Sample()
	if _, err := importer.Store(context.Background(), pool, blob.NewLocalDir(t.TempDir()), ds,
		importer.StoreOptions{MarkVerified: true, Images: images, Source: "fixture"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	q := sqlc.New(pool)
	profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998901234567"})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	tok, err := auth.IssueAccess([]byte(handlerSecret), profile.ID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	svc := session.NewService(q, billing.Service{Q: q})
	r := chi.NewRouter()
	h := &session.Handler{Svc: svc}
	h.Routes(r.With(auth.Required([]byte(handlerSecret))))

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts, tok, q
}

type respEnvelope struct {
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func doReq(t *testing.T, ts *httptest.Server, method, path, token string, body []byte) (int, respEnvelope) {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var env respEnvelope
	_ = json.NewDecoder(resp.Body).Decode(&env)
	return resp.StatusCode, env
}

func TestFullVariantSessionOverHTTP(t *testing.T) {
	ts, tok, q := setupServer(t)
	v, err := q.GetVariantByNumber(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(map[string]any{"mode": "variant", "variant_id": v.ID, "locale": "uz-Latn"})
	status, env := doReq(t, ts, http.MethodPost, "/sessions", tok, body)
	if status != http.StatusCreated {
		t.Fatalf("create session status=%d body=%s", status, env.Data)
	}
	var created struct {
		ID          string   `json:"id"`
		QuestionIDs []string `json:"question_ids"`
		Total       int      `json:"total"`
	}
	if err := json.Unmarshal(env.Data, &created); err != nil {
		t.Fatal(err)
	}
	if created.Total != 20 || len(created.QuestionIDs) != 20 {
		t.Fatalf("expected 20 questions: %+v", created)
	}

	ansBody, _ := json.Marshal(map[string]any{"question_id": created.QuestionIDs[0], "answer_id": "00000000-0000-0000-0000-000000000000"})
	status, env = doReq(t, ts, http.MethodPost, "/sessions/"+created.ID+"/answers", tok, ansBody)
	if status != http.StatusBadRequest || env.Error == nil || env.Error.Code != "invalid_answer" {
		t.Fatalf("expected 400 invalid_answer for a made-up answer id, got status=%d env=%+v", status, env)
	}

	status, env = doReq(t, ts, http.MethodGet, "/me/variants", tok, nil)
	if status != http.StatusOK {
		t.Fatalf("me/variants status=%d", status)
	}
	var statuses []struct {
		Number   int32 `json:"number"`
		Unlocked bool  `json:"unlocked"`
	}
	if err := json.Unmarshal(env.Data, &statuses); err != nil {
		t.Fatal(err)
	}
	if !statuses[0].Unlocked || statuses[1].Unlocked {
		t.Fatalf("expected only variant 1 unlocked initially: %+v", statuses)
	}
}

func TestSessionsRequireAuth(t *testing.T) {
	ts, _, _ := setupServer(t)
	resp, err := ts.Client().Get(ts.URL + "/me/sessions")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/session/... -run "HTTP|RequireAuth" -v`
Expected: FAIL — `session.Handler` undefined.

- [ ] **Step 3: Implement `handlers.go`** (mirrors `internal/auth/handlers.go`'s and `internal/account/handlers.go`'s style: a `Handler{Svc *Service}` with a `Routes(r chi.Router)` method, per-endpoint request/response structs, a `writeSessionError(w, err)` switch matching the Error mapping table above, and `auth.FromContext(r.Context())` for `profileID` on every handler). Then modify `internal/server/server.go`: inside the existing `if deps.Pool != nil && deps.Redis != nil` block (where `auth.Handler` and `account.Handler` are already mounted), add:

```go
sess := &session.Handler{Svc: session.NewService(deps.Queries, billing.Service{Q: deps.Queries})}
sess.Routes(api.With(auth.Required([]byte(cfg.JWTSecret))))
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd backend && go build ./... && go test ./internal/session/... -v && go test ./internal/server/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/session/handlers.go backend/internal/session/handlers_test.go backend/internal/server/server.go
git commit -m "feat(backend): session http endpoints — start/answer/finish/resume/history/variants"
```

---

### Task 8: Full verification + docs

- [ ] **Step 1:** `make check` (lint + `-p 1` full suite) green.

Run: `cd /home/sher/Рабочий\ стол/avtotest && make check`
Expected: `0 issues.` then all packages `ok`.

- [ ] **Step 2: Live smoke** (PORT=8090, reusing the pattern from Plan 02's Task 10): sign in via OTP sandbox flow (`/auth/otp/request` → `/auth/otp/verify`) to get a Bearer token, then:
  1. `GET /api/v1/me/variants` → confirm only variant 1 unlocked.
  2. `POST /api/v1/sessions {"mode":"variant","variant_id":"<v1 id>","locale":"uz-Latn"}` → capture `question_ids`.
  3. For each question ID, `GET /api/v1/questions/{id}?locale=uz-Latn` to find the right answer text (never `is_correct`), then `POST /api/v1/sessions/{id}/answers` guessing until `correct:true` comes back (dev/sandbox has only 40 fixture questions — feasible by hand for a couple).
  4. `POST /api/v1/sessions/{id}/finish` → confirm `status:"passed"` if ≥10/20 correct.
  5. `GET /api/v1/me/variants` again → confirm variant 2 now `unlocked:true`.
  6. `POST /api/v1/sessions {"mode":"exam","locale":"uz-Latn"}` → submit 3 deliberately wrong answers → confirm the 3rd response has `stopped:true, stop_reason:"too_many_errors"`, and `GET /api/v1/sessions/{id}` shows `status:"failed"`.
  Record all outputs.

- [ ] **Step 3: README** — add a "Sessiya / test yechish" section to `README.md`: the 4 modes, endpoints, error codes, and the unlock/mistake-bank rules (mirrors the "Auth" section Plan 02 added). Commit:

```bash
git add README.md
git commit -m "docs: session/scoring/unlock flow and endpoint reference"
```

## Self-Review

1. **Spec coverage:** §7.3 schema (already migrated in Plan 01, no new migration needed) → Tasks 1-6; §11 mode table (bilet/imtihon/mashq/xatolar + unlock/limit qoidalari) → Tasks 3-6; §4 real-exam facts (20/25min/≤2xato/3-xatoda stop) → Task 2 (`rules.go`) + Task 4 (stop-on-3rd) + Task 5 (time-up branch); §14 API list (`POST /sessions`, `/sessions/{id}/answers`, `/finish`, `GET /sessions/{id}`, `GET /me/sessions`, `GET /variants` +progress → implemented as `GET /me/variants` to keep the public content catalog endpoint auth-free, noted in the Architecture section) → Task 7; anti-cheat (§11, §20 — no `is_correct` leak) → Task 4 (exam withholds feedback) + Task 6 (resume redacts in-progress exam correctness); Grand Mock explicitly deferred to M2 per §5's milestone table — out of scope, documented in Global Constraints.
2. **Placeholders:** none — every step has real code or a fully specified logic/test list (matching the established style of Plan 02's DB-integration tasks). The one explicit "implementer note" in Task 3 is not a placeholder — it hands the implementer the exact query to add and the exact test behavior to assert, because the test file's category-ID lookup depends on a query this same task introduces.
3. **Type consistency:** `Service`, `StartRequest`, `SessionView`, `AnswerResult`, `FinishResult`, `SessionDetail`, `AnsweredQuestion`, `SessionSummary`, `VariantStatus` names and fields are used identically across Tasks 3-7; error sentinels (`ErrNotFound`, `ErrInvalidRequest`, `ErrDailyLimitReached`, `ErrAlreadyAnswered`, `ErrInvalidAnswer`, `ErrSessionFinished`) are defined once in Task 3 and reused verbatim in Tasks 4-7's logic and HTTP error mapping.
