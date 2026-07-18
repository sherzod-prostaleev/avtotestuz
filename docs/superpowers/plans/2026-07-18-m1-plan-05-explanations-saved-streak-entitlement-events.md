# M1 Plan 05 — Explanation Drafts, Saved Questions, Streak, Free-Tier Entitlement Gating, Event Logging

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close out the remaining M1 learning-core features from the master spec that aren't covered by Plans 01-04: an AI-draft→verify→feedback workflow for explanations (draft generation stubbed — no live LLM key yet), saved/bookmarked questions, daily streak tracking, enforcement of the free-tier boundary the spec always specified but no prior plan implemented (bilet #1 + limited practice + signs catalog free; everything else requires an active VIP entitlement), and batch client event ingestion for analytics.

**Architecture:** Three new small packages, each following the established `rules.go`(pure)/`service.go`(DB)/`handlers.go`(HTTP) split used by `internal/session` and `internal/learning`:
- `internal/explanation` — `AIDraftGenerator` interface + a template-based stub implementation (real LLM integration deferred until an API key/budget is decided — explicitly out of scope here, confirmed with the user), a `Service` for creating drafts, marking verified, and recording "helpful?" feedback, and one HTTP endpoint for feedback (draft-generation and verify are admin-only CLI actions in M1, matching the spec's "model in M1, UI in M3" note for the review queue).
- `internal/progress` — saved questions (bookmarks) and streak tracking together, since both are simple per-profile counters/sets with no complex business logic (unlike FSRS or scoring, which each warranted their own package).
- `internal/events` — batch event ingestion (`POST /events`), a thin write-only layer over the existing partitioned `event` table.

Two existing packages get small, targeted modifications: `internal/session` gains free/VIP gating in `StartSession` (variant #2+, exam, and mistakes modes now require an active entitlement; variant #1 and practice remain free, matching D13) and calls into `internal/progress` to bump the daily streak on every answered question, mirroring how Plan 04 wired FSRS into the same call site.

**Tech Stack:** No new dependencies — same stack as Plans 01-04. No new migrations — `explanation`/`explanation_translation`/`explanation_feedback`/`saved_question`/`streak`/`event`/`limit_config` all already exist from Plan 01's `0001_content.up.sql`/`0002_identity.up.sql`/`0003_billing.up.sql`/`0004_learning.up.sql`/`0005_system.up.sql`.

## Global Constraints

- All Plan 01-04 conventions hold (envelope, locales, commit style, `-p 1` tests, `export PATH=$HOME/.local/go/bin:$HOME/go/bin:$PATH`, `make up` before any DB test run, double-quote the repo path in shell commands — never backslash-escape its space, that triggers an unrelated extra confirmation prompt).
- **AI-draft is stubbed, not a real LLM call** — confirmed with the user (no API key/budget decided yet). `TemplateDraftGenerator` produces a clearly-marked placeholder (`[AI-QORALAMA]` prefix, matching the fixture's `[NAMUNA]` convention for "not real content") built from the question's text, correct answer text, and category — structurally correct (right block shape, right locale, `status='draft'`) but not real legal analysis. The `AIDraftGenerator` interface is designed so a real LLM-backed implementation can be swapped in later without touching callers.
- **Draft-gen and verify are CLI-only in M1** (`cmd/gendraft`, `cmd/verifyexplanation`) — per spec §13, the review/quality-queue *UI* is M3's job; M1 only needs the data model and a way to actually move rows through `draft→pending→verified` so the pipeline is testable end-to-end. Feedback (`POST /explanations/feedback`) IS end-user-facing (the "Foydali bo'ldimi?" vote a learner casts) and gets a real HTTP endpoint.
- **Draft generation scope**: uz-Latn only for the stub (the other two required locales get real professional translation later, not machine-stub text pretending to be Cyrillic/Russian legal analysis — stubbing all three would be actively misleading, not just placeholder).
- **Free-tier boundary** (spec D13, never enforced by a prior plan): free profiles (no active entitlement) may access **bilet/variant #1 only**, **practice mode** (already daily-limited via `limit_config`), and the public signs catalog (already unauthenticated). **Variant #2+, `exam` mode, and `mistakes` mode all require an active entitlement** — `session.Service.StartSession` checks `Billing.Status` before dispatching those. New sentinel `ErrRequiresVIP`, mapped to **402 Payment Required**, code `vip_required` (a deliberate, correct use of 402 for "pay to unlock" — not a generic 403).
- **Streak update rule** (classic day-based streak, no hidden state beyond the `streak` table): on any answered question (any session mode, mirroring Plan 04's `RecordReview` call site in `SubmitAnswer`), bump the caller's streak:
  - no existing row, or `last_active_date` is today already → `today_done++`, `current`/`best`/`last_active_date` unchanged (first call of the day additionally sets `last_active_date=today, current=max(current,1), best=max(best,current)` — see Task 4's exact pure logic).
  - `last_active_date` was yesterday → `current++`, `today_done=1`, `last_active_date=today`, `best=max(best,current)`.
  - `last_active_date` was any earlier day (a gap) or the row is brand new → `current=1`, `today_done=1`, `last_active_date=today`, `best=max(best,1)`.
  - `daily_goal` defaults from `limit_config('daily_goal_default')` (10) when a streak row is first created; not otherwise touched by the bump logic in this plan (goal *editing* is a future UI concern).
- **Event logging is authenticated-only in M1** — `POST /events` requires a Bearer token and always writes `profile_id` (never `anon_id`); anonymous/pre-login event capture is explicitly deferred, not silently dropped-and-forgotten (documented in the README task).
- Error sentinels: `explanation.ErrNotFound` (question has no explanation to give feedback on); `session.ErrRequiresVIP` (new, added to the existing sentinel list in `internal/session/service.go`).

## File Structure (new/modified)

```
backend/
  internal/db/queries/
    explanation.sql                          # new: draft/verify/feedback queries + sqlc generate
    progress.sql                              # new: saved_question + streak queries + sqlc generate
    events.sql                                # new: event insert query + sqlc generate
    session.sql                               # modify: + GetVariantByID
  internal/explanation/
    draft.go draft_test.go                    # pure: AIDraftGenerator interface, TemplateDraftGenerator, Block types
    service.go service_test.go                # CreateDraft, Verify, RecordFeedback
    handlers.go handlers_test.go              # POST /explanations/feedback
  internal/progress/
    rules.go rules_test.go                    # pure: BumpStreak
    service.go service_test.go                # SaveQuestion/Unsave/ListSaved, RecordActivity (streak bump), GetStreak
    handlers.go handlers_test.go              # GET/POST/DELETE /me/saved, GET /me/streak
  internal/events/
    service.go service_test.go                # LogBatch
    handlers.go handlers_test.go              # POST /events
  internal/session/
    service.go service_test.go                # modify: VIP gating in StartSession; RecordActivity call in SubmitAnswer
  cmd/gendraft/main.go                        # new: admin CLI, generates a draft for a question
  cmd/verifyexplanation/main.go               # new: admin CLI, marks a draft/pending translation verified
  internal/server/server.go                   # mount explanation/progress/events routes (modify)
```

---

### Task 1: sqlc queries — explanations, saved questions, streak, events, variant-by-id

**Files:** create `internal/db/queries/explanation.sql`, `internal/db/queries/progress.sql`, `internal/db/queries/events.sql`; modify `internal/db/queries/session.sql` (add `GetVariantByID`); run `sqlc generate`.

- [ ] **Step 1: Write `explanation.sql`**

```sql
-- name: GetExplanationTranslationForFeedback :one
-- Used only to validate that a question has an explanation before recording
-- feedback against it — returns the explanation_id regardless of locale/status,
-- since feedback is about the question's explanation as a whole, not one locale.
SELECT e.id AS explanation_id
FROM explanation e
WHERE e.question_id = sqlc.arg(question_id)
LIMIT 1;

-- name: UpsertExplanationFeedback :exec
INSERT INTO explanation_feedback (profile_id, explanation_id, helpful)
VALUES ($1, $2, $3)
ON CONFLICT (profile_id, explanation_id) DO UPDATE SET helpful = EXCLUDED.helpful;

-- name: GetQuestionForDraft :one
SELECT q.id, q.category_id, c.code AS category_code,
       COALESCE(qt.text, qft.text, '') AS question_text
FROM question q
JOIN category c ON c.id = q.category_id
LEFT JOIN question_translation qt
       ON qt.question_id = q.id AND qt.locale = 'uz-Latn' AND qt.status = 'verified'
LEFT JOIN question_translation qft
       ON qft.question_id = q.id AND qft.locale = 'uz-Latn'
WHERE q.id = sqlc.arg(id);

-- name: GetCorrectAnswerTextForDraft :one
SELECT COALESCE(at.text, aft.text, '') AS answer_text
FROM answer a
LEFT JOIN answer_translation at
       ON at.answer_id = a.id AND at.locale = 'uz-Latn' AND at.status = 'verified'
LEFT JOIN answer_translation aft
       ON aft.answer_id = a.id AND aft.locale = 'uz-Latn'
WHERE a.question_id = sqlc.arg(question_id) AND a.is_correct = true;

-- name: InsertDraftExplanation :one
INSERT INTO explanation (question_id, legal_refs)
VALUES ($1, '[]'::jsonb)
ON CONFLICT (question_id) DO UPDATE SET legal_refs = explanation.legal_refs
RETURNING id;

-- name: InsertDraftTranslation :exec
INSERT INTO explanation_translation (explanation_id, locale, blocks, status, source)
VALUES ($1, $2, $3, 'draft', 'ai-stub')
ON CONFLICT (explanation_id, locale) DO UPDATE
  SET blocks = EXCLUDED.blocks, status = 'draft', source = 'ai-stub';

-- name: GetExplanationTranslationByExplanationAndLocale :one
SELECT * FROM explanation_translation
WHERE explanation_id = sqlc.arg(explanation_id) AND locale = sqlc.arg(locale);

-- name: VerifyExplanationTranslation :exec
UPDATE explanation_translation
SET status = 'verified', verified_by = sqlc.arg(verified_by), verified_at = now()
WHERE explanation_id = sqlc.arg(explanation_id) AND locale = sqlc.arg(locale);

-- name: GetExplanationIDByQuestionID :one
SELECT id FROM explanation WHERE question_id = $1;
```

- [ ] **Step 2: Write `progress.sql`**

```sql
-- name: SaveQuestion :exec
INSERT INTO saved_question (profile_id, question_id)
VALUES ($1, $2)
ON CONFLICT (profile_id, question_id) DO NOTHING;

-- name: UnsaveQuestion :exec
DELETE FROM saved_question WHERE profile_id = sqlc.arg(profile_id) AND question_id = sqlc.arg(question_id);

-- name: ListSavedQuestions :many
SELECT question_id, created_at FROM saved_question
WHERE profile_id = $1
ORDER BY created_at DESC;

-- name: GetStreak :one
SELECT * FROM streak WHERE profile_id = $1;

-- name: UpsertStreak :one
INSERT INTO streak (profile_id, current, best, last_active_date, daily_goal, today_done)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (profile_id) DO UPDATE SET
  current = EXCLUDED.current,
  best = EXCLUDED.best,
  last_active_date = EXCLUDED.last_active_date,
  daily_goal = EXCLUDED.daily_goal,
  today_done = EXCLUDED.today_done
RETURNING *;
```

- [ ] **Step 3: Write `events.sql`**

```sql
-- name: InsertEvent :exec
INSERT INTO event (profile_id, name, props, ts)
VALUES ($1, $2, $3, $4);
```

- [ ] **Step 4: Add to `session.sql`**

```sql
-- name: GetVariantByID :one
SELECT * FROM variant WHERE id = $1;
```

- [ ] **Step 5: Generate and build**

Run: `cd backend && sqlc generate && go build ./...`
Expected: clean exit (no deliberate breakage this time — every query in this task is purely additive, nothing existing was deleted or renamed).

- [ ] **Step 6: Commit**

```bash
git add backend/internal/db/queries/explanation.sql backend/internal/db/queries/progress.sql backend/internal/db/queries/events.sql backend/internal/db/queries/session.sql backend/internal/db/sqlc/
git commit -m "feat(backend): sqlc queries for explanation drafts, saved questions, streak, events"
```

---

### Task 2: Pure AI-draft stub (no DB)

**Files:** create `internal/explanation/draft.go`, `internal/explanation/draft_test.go`.

**Interfaces (produced):**
```go
type Block struct {
	Type string `json:"type"`            // "intro" | "muhim" | "eslatma" | "ogohlantirish" | "maslahat" | "answer_analysis"
	Text string `json:"text,omitempty"`
	Items []AnswerAnalysisItem `json:"items,omitempty"` // only for "answer_analysis"
}
type AnswerAnalysisItem struct {
	Position int    `json:"position"`
	Correct  bool   `json:"correct"`
	Text     string `json:"text"`
}
type DraftInput struct {
	QuestionText      string
	CategoryCode      string
	CorrectAnswerText string
}
// AIDraftGenerator produces a first-pass explanation for expert review.
// TemplateDraftGenerator is the M1 stub; a real LLM-backed implementation
// can satisfy the same interface later without touching any caller.
type AIDraftGenerator interface {
	Generate(in DraftInput) []Block
}
type TemplateDraftGenerator struct{}
func (TemplateDraftGenerator) Generate(in DraftInput) []Block
```

- [ ] **Step 1: Write the failing tests**

```go
package explanation

import (
	"strings"
	"testing"
)

func TestTemplateDraftGeneratorProducesMarkedPlaceholder(t *testing.T) {
	g := TemplateDraftGenerator{}
	blocks := g.Generate(DraftInput{
		QuestionText:      "Svetofor sariq rangda yonganda nima qilish kerak?",
		CategoryCode:      "rules",
		CorrectAnswerText: "To'xtash chizig'i oldida to'xtash",
	})
	if len(blocks) == 0 {
		t.Fatal("expected at least one block")
	}
	var hasIntro, hasMuhim, hasAnalysis bool
	for _, b := range blocks {
		switch b.Type {
		case "intro":
			hasIntro = true
			if !containsMarker(b.Text) {
				t.Errorf("intro block must carry the [AI-QORALAMA] marker, got %q", b.Text)
			}
		case "muhim":
			hasMuhim = true
			if !containsMarker(b.Text) {
				t.Errorf("muhim block must carry the [AI-QORALAMA] marker, got %q", b.Text)
			}
		case "answer_analysis":
			hasAnalysis = true
		}
	}
	if !hasIntro || !hasMuhim || !hasAnalysis {
		t.Fatalf("expected intro+muhim+answer_analysis blocks, got %+v", blocks)
	}
}

func TestTemplateDraftGeneratorNeverPanicsOnEmptyInput(t *testing.T) {
	g := TemplateDraftGenerator{}
	blocks := g.Generate(DraftInput{})
	if len(blocks) == 0 {
		t.Fatal("even empty input should produce a non-empty placeholder structure")
	}
}

func containsMarker(s string) bool {
	return strings.Contains(s, aiMarker)
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/explanation/... -v`
Expected: FAIL — package/functions don't exist.

- [ ] **Step 3: Implement**

```go
// Package explanation owns the AI-draft → expert-verify → learner-feedback
// pipeline for per-question legal explanations. draft.go is pure (no DB);
// service.go integrates it with the explanation/explanation_translation/
// explanation_feedback tables.
package explanation

import "fmt"

const aiMarker = "[AI-QORALAMA]"

type AnswerAnalysisItem struct {
	Position int    `json:"position"`
	Correct  bool   `json:"correct"`
	Text     string `json:"text"`
}

type Block struct {
	Type  string               `json:"type"`
	Text  string               `json:"text,omitempty"`
	Items []AnswerAnalysisItem `json:"items,omitempty"`
}

type DraftInput struct {
	QuestionText      string
	CategoryCode      string
	CorrectAnswerText string
}

// AIDraftGenerator produces a first-pass explanation for expert review.
// TemplateDraftGenerator is the M1 stub — clearly marked, not real legal
// analysis; a real LLM-backed implementation can satisfy this same
// interface later without touching any caller.
type AIDraftGenerator interface {
	Generate(in DraftInput) []Block
}

type TemplateDraftGenerator struct{}

func (TemplateDraftGenerator) Generate(in DraftInput) []Block {
	qText := in.QuestionText
	if qText == "" {
		qText = "(savol matni mavjud emas)"
	}
	correct := in.CorrectAnswerText
	if correct == "" {
		correct = "(to'g'ri javob matni mavjud emas)"
	}
	cat := in.CategoryCode
	if cat == "" {
		cat = "umumiy"
	}

	return []Block{
		{Type: "intro", Text: fmt.Sprintf("%s Ushbu savol \"%s\" kategoriyasiga tegishli: %s", aiMarker, cat, qText)},
		{Type: "muhim", Text: fmt.Sprintf("%s MUHIM: to'g'ri javob — %s", aiMarker, correct)},
		{Type: "answer_analysis", Items: []AnswerAnalysisItem{
			{Position: 1, Correct: false, Text: aiMarker + " ekspert tahlili kutilmoqda"},
			{Position: 2, Correct: false, Text: aiMarker + " ekspert tahlili kutilmoqda"},
			{Position: 3, Correct: false, Text: aiMarker + " ekspert tahlili kutilmoqda"},
			{Position: 4, Correct: false, Text: aiMarker + " ekspert tahlili kutilmoqda"},
		}},
	}
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd backend && go test ./internal/explanation/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/explanation/draft.go backend/internal/explanation/draft_test.go
git commit -m "feat(backend): AI-draft stub generator for explanations"
```

---

### Task 3: `explanation.Service` — CreateDraft, Verify, RecordFeedback

**Files:** create `internal/explanation/service.go`, `internal/explanation/service_test.go`.

**Interfaces (produced):**
```go
type Service struct {
	Q         *sqlc.Queries
	Generator AIDraftGenerator
}
func NewService(q *sqlc.Queries, g AIDraftGenerator) *Service

var ErrNotFound = errors.New("explanation not found")

// CreateDraft generates and stores a uz-Latn draft for questionID (status='draft').
func (s *Service) CreateDraft(ctx context.Context, questionID uuid.UUID) error

// Verify marks the uz-Latn translation for questionID as verified.
func (s *Service) Verify(ctx context.Context, questionID uuid.UUID, verifiedBy uuid.UUID) error

// RecordFeedback upserts a helpful/not-helpful vote; ErrNotFound if the
// question has no explanation yet.
func (s *Service) RecordFeedback(ctx context.Context, profileID, questionID uuid.UUID, helpful bool) error
```

**Logic:**
- `CreateDraft`: `q, err := s.Q.GetQuestionForDraft(ctx, questionID)`; `correct, err := s.Q.GetCorrectAnswerTextForDraft(ctx, questionID)` (a question with no correct answer yet is a data problem outside this task's scope — propagate the error, don't guess); `blocks := s.Generator.Generate(DraftInput{QuestionText: q.QuestionText, CategoryCode: q.CategoryCode, CorrectAnswerText: correct})`; marshal to `json.RawMessage`; `explID, err := s.Q.InsertDraftExplanation(ctx, questionID)`; `s.Q.InsertDraftTranslation(ctx, sqlc.InsertDraftTranslationParams{ExplanationID: explID, Locale: "uz-Latn", Blocks: blocksJSON})`.
- `Verify`: `explID, err := s.Q.GetExplanationIDByQuestionID(ctx, questionID)` (`pgx.ErrNoRows` → `ErrNotFound`); `s.Q.VerifyExplanationTranslation(ctx, sqlc.VerifyExplanationTranslationParams{ExplanationID: explID, Locale: "uz-Latn", VerifiedBy: uuid.NullUUID{UUID: verifiedBy, Valid: true}})`.
- `RecordFeedback`: `row, err := s.Q.GetExplanationTranslationForFeedback(ctx, questionID)` (`pgx.ErrNoRows` → `ErrNotFound`); `s.Q.UpsertExplanationFeedback(ctx, sqlc.UpsertExplanationFeedbackParams{ProfileID: profileID, ExplanationID: row.ExplanationID, Helpful: helpful})`.

- [ ] **Step 1: Write the failing tests** (using `testdb.New`/`fixture.Sample()`/`importer.Store` seeding, same pattern as `internal/session`/`internal/learning`'s test suites)

```go
package explanation_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/explanation"
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
	"avtotest.uz/backend/internal/testdb"
)

func seed(t *testing.T) (*sqlc.Queries, *explanation.Service, uuid.UUID, uuid.UUID) {
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
	v, err := q.GetVariantByNumber(context.Background(), 1)
	if err != nil {
		t.Fatalf("variant: %v", err)
	}
	qids, err := q.ListVariantQuestionIDsOrdered(context.Background(), v.ID)
	if err != nil || len(qids) == 0 {
		t.Fatalf("question ids: %v", err)
	}
	svc := explanation.NewService(q, explanation.TemplateDraftGenerator{})
	return q, svc, profile.ID, qids[0]
}

func TestCreateDraftStoresBlocks(t *testing.T) {
	q, svc, _, questionID := seed(t)
	ctx := context.Background()

	if err := svc.CreateDraft(ctx, questionID); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}

	explID, err := q.GetExplanationIDByQuestionID(ctx, questionID)
	if err != nil {
		t.Fatalf("explanation id: %v", err)
	}
	row, err := q.GetExplanationTranslationByExplanationAndLocale(ctx, sqlc.GetExplanationTranslationByExplanationAndLocaleParams{
		ExplanationID: explID, Locale: "uz-Latn",
	})
	if err != nil {
		t.Fatalf("translation: %v", err)
	}
	if row.Status != "draft" {
		t.Fatalf("status = %q, want draft", row.Status)
	}
	var blocks []explanation.Block
	if err := json.Unmarshal(row.Blocks, &blocks); err != nil {
		t.Fatalf("unmarshal blocks: %v", err)
	}
	if len(blocks) == 0 {
		t.Fatal("expected non-empty blocks")
	}
}

func TestVerifyMarksTranslationVerified(t *testing.T) {
	q, svc, profileID, questionID := seed(t)
	ctx := context.Background()

	if err := svc.CreateDraft(ctx, questionID); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	if err := svc.Verify(ctx, questionID, profileID); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	explID, err := q.GetExplanationIDByQuestionID(ctx, questionID)
	if err != nil {
		t.Fatalf("explanation id: %v", err)
	}
	row, err := q.GetExplanationTranslationByExplanationAndLocale(ctx, sqlc.GetExplanationTranslationByExplanationAndLocaleParams{
		ExplanationID: explID, Locale: "uz-Latn",
	})
	if err != nil {
		t.Fatalf("translation: %v", err)
	}
	if row.Status != "verified" || !row.VerifiedBy.Valid {
		t.Fatalf("expected verified status with verified_by set, got %+v", row)
	}
}

func TestVerifyWithoutDraftReturnsNotFound(t *testing.T) {
	_, svc, profileID, questionID := seed(t)
	if err := svc.Verify(context.Background(), questionID, profileID); err != explanation.ErrNotFound {
		t.Fatalf("err=%v want ErrNotFound", err)
	}
}

func TestRecordFeedbackRequiresExistingExplanation(t *testing.T) {
	_, svc, profileID, questionID := seed(t)
	if err := svc.RecordFeedback(context.Background(), profileID, questionID, true); err != explanation.ErrNotFound {
		t.Fatalf("err=%v want ErrNotFound", err)
	}
}

func TestRecordFeedbackUpsertsVote(t *testing.T) {
	_, svc, profileID, questionID := seed(t)
	ctx := context.Background()
	if err := svc.CreateDraft(ctx, questionID); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	if err := svc.RecordFeedback(ctx, profileID, questionID, true); err != nil {
		t.Fatalf("first feedback: %v", err)
	}
	// changing the vote must upsert, not error or duplicate
	if err := svc.RecordFeedback(ctx, profileID, questionID, false); err != nil {
		t.Fatalf("second feedback: %v", err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/explanation/... -v`
Expected: FAIL — `Service` undefined.

- [ ] **Step 3: Implement** `service.go` per the Logic section above.

- [ ] **Step 4: Run to verify it passes**

Run: `make up && cd backend && go test ./internal/explanation/... -p 1 -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/explanation/service.go backend/internal/explanation/service_test.go
git commit -m "feat(backend): explanation.Service — draft creation, verify, feedback"
```

---

### Task 4: `cmd/gendraft` + `cmd/verifyexplanation` CLIs; feedback HTTP endpoint

**Files:** create `cmd/gendraft/main.go`, `cmd/verifyexplanation/main.go`, `internal/explanation/handlers.go`, `internal/explanation/handlers_test.go`; modify `internal/server/server.go`.

**CLIs** (mirror `cmd/grantvip`'s established flag-based style):
- `gendraft -question <uuid>`: loads config, connects DB, calls `explanation.Service.CreateDraft`, prints `draft created for question <id>` or the error.
- `verifyexplanation -question <uuid> -by <profile-uuid>`: calls `explanation.Service.Verify`, prints `verified explanation for question <id>` or the error (`explanation.ErrNotFound` → clear "no draft exists — run gendraft first" message, matching `grantvip`'s "profile not found — user must sign in once first" style of actionable error text).

**HTTP route** (behind `auth.Required`): `POST /explanations/feedback {question_id, helpful}` → `200 {ok: true}`. Error mapping: `explanation.ErrNotFound` → 404 `not_found`.

- [ ] **Step 1: Write the failing test** (mirrors `internal/session/handlers_test.go`'s pattern)

```go
package explanation_test

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
	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/explanation"
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
	"avtotest.uz/backend/internal/testdb"
)

const handlerSecret = "test-secret"

func setupHandlerServer(t *testing.T) (*httptest.Server, string, string) {
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
	v, err := q.GetVariantByNumber(context.Background(), 1)
	if err != nil {
		t.Fatalf("variant: %v", err)
	}
	qids, err := q.ListVariantQuestionIDsOrdered(context.Background(), v.ID)
	if err != nil || len(qids) == 0 {
		t.Fatalf("question ids: %v", err)
	}
	svc := explanation.NewService(q, explanation.TemplateDraftGenerator{})
	tok, err := auth.IssueAccess([]byte(handlerSecret), profile.ID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	r := chi.NewRouter()
	h := &explanation.Handler{Svc: svc}
	h.Routes(r.With(auth.Required([]byte(handlerSecret))))

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts, tok, qids[0].String()
}

func TestFeedbackWithoutDraftReturns404(t *testing.T) {
	ts, tok, questionID := setupHandlerServer(t)
	body, _ := json.Marshal(map[string]any{"question_id": questionID, "helpful": true})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/explanations/feedback", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d want 404", resp.StatusCode)
	}
}

func TestFeedbackRequiresAuth(t *testing.T) {
	ts, _, questionID := setupHandlerServer(t)
	body, _ := json.Marshal(map[string]any{"question_id": questionID, "helpful": true})
	resp, err := ts.Client().Post(ts.URL+"/explanations/feedback", "application/json", bytes.NewReader(body))
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

Run: `cd backend && go test ./internal/explanation/... -run "Feedback" -v`
Expected: FAIL — `explanation.Handler` undefined.

- [ ] **Step 3: Implement** `handlers.go` (mirror `internal/session/handlers.go`'s `Handler{Svc}`/`Routes(r chi.Router)`/`decodeBody`/error-switch style exactly), `cmd/gendraft/main.go`, `cmd/verifyexplanation/main.go` (mirror `cmd/grantvip/main.go`'s structure). Modify `server.go`: mount `explanation.Handler` the same way `session.Handler`/`learning.Handler` are mounted, inside the existing `if deps.Pool != nil && deps.Redis != nil` block.

- [ ] **Step 4: Run to verify it passes**

Run: `cd backend && go build ./... && go test ./internal/explanation/... -p 1 -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/gendraft cmd/verifyexplanation backend/internal/explanation/handlers.go backend/internal/explanation/handlers_test.go backend/internal/server/server.go
git commit -m "feat(backend): explanation feedback endpoint, gendraft/verifyexplanation admin CLIs"
```

---

### Task 5: Pure streak logic (no DB)

**Files:** create `internal/progress/rules.go`, `internal/progress/rules_test.go`.

**Interfaces (produced):**
```go
type StreakState struct {
	Current, Best, TodayDone int
	LastActiveDate           *time.Time // nil = never active; compared by calendar date, not time
}

// BumpStreak applies one day's worth of activity to s, evaluated as of today
// (a date, time-of-day and location-independent — callers pass a UTC
// midnight-truncated time). Pure, no DB, no side effects.
func BumpStreak(s StreakState, today time.Time) StreakState
```

**Logic:** `today` is always truncated by the caller to a calendar day (UTC midnight) before being passed in — this function does simple date-equality comparison, no timezone logic of its own.
- `s.LastActiveDate == nil` (brand new): `Current=1, Best=max(s.Best,1), TodayDone=1, LastActiveDate=&today`.
- `sameDay(*s.LastActiveDate, today)`: `TodayDone++`; `Current`/`Best`/`LastActiveDate` unchanged (already counted today).
- `sameDay(*s.LastActiveDate, today.AddDate(0,0,-1))` (yesterday): `Current++`, `Best=max(s.Best,Current)`, `TodayDone=1`, `LastActiveDate=&today`.
- otherwise (a gap of 2+ days): `Current=1`, `Best=max(s.Best,1)`, `TodayDone=1`, `LastActiveDate=&today`.

- [ ] **Step 1: Write the failing tests**

```go
package progress

import (
	"testing"
	"time"
)

func day(offset int) time.Time {
	base := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	return base.AddDate(0, 0, offset)
}

func TestBumpStreakBrandNew(t *testing.T) {
	got := BumpStreak(StreakState{}, day(0))
	if got.Current != 1 || got.Best != 1 || got.TodayDone != 1 {
		t.Fatalf("brand new bump = %+v", got)
	}
	if got.LastActiveDate == nil || !got.LastActiveDate.Equal(day(0)) {
		t.Fatalf("LastActiveDate = %v, want %v", got.LastActiveDate, day(0))
	}
}

func TestBumpStreakSameDayOnlyIncrementsTodayDone(t *testing.T) {
	d := day(0)
	s := StreakState{Current: 3, Best: 5, TodayDone: 2, LastActiveDate: &d}
	got := BumpStreak(s, day(0))
	if got.Current != 3 || got.Best != 5 || got.TodayDone != 3 {
		t.Fatalf("same-day bump = %+v", got)
	}
}

func TestBumpStreakConsecutiveDayIncrementsCurrent(t *testing.T) {
	d := day(0)
	s := StreakState{Current: 3, Best: 5, TodayDone: 10, LastActiveDate: &d}
	got := BumpStreak(s, day(1))
	if got.Current != 4 || got.Best != 5 || got.TodayDone != 1 {
		t.Fatalf("consecutive-day bump = %+v", got)
	}
	if !got.LastActiveDate.Equal(day(1)) {
		t.Fatalf("LastActiveDate = %v, want %v", got.LastActiveDate, day(1))
	}
}

func TestBumpStreakConsecutiveDayCanExceedPriorBest(t *testing.T) {
	d := day(0)
	s := StreakState{Current: 5, Best: 5, TodayDone: 1, LastActiveDate: &d}
	got := BumpStreak(s, day(1))
	if got.Current != 6 || got.Best != 6 {
		t.Fatalf("new-best bump = %+v", got)
	}
}

func TestBumpStreakGapResetsCurrent(t *testing.T) {
	d := day(0)
	s := StreakState{Current: 7, Best: 7, TodayDone: 1, LastActiveDate: &d}
	got := BumpStreak(s, day(3))
	if got.Current != 1 || got.Best != 7 || got.TodayDone != 1 {
		t.Fatalf("gap bump = %+v", got)
	}
	if !got.LastActiveDate.Equal(day(3)) {
		t.Fatalf("LastActiveDate = %v, want %v", got.LastActiveDate, day(3))
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/progress/... -v`
Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Implement**

```go
// Package progress owns saved (bookmarked) questions and daily streak
// tracking — two simple per-profile engagement features grouped together
// since neither warrants the complexity of its own package the way FSRS
// (internal/learning) or scoring (internal/session) did. rules.go is pure;
// service.go integrates it with saved_question/streak.
package progress

import "time"

type StreakState struct {
	Current, Best, TodayDone int
	LastActiveDate           *time.Time
}

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

// BumpStreak applies one day's worth of activity to s, evaluated as of
// today (already truncated to a calendar day by the caller).
func BumpStreak(s StreakState, today time.Time) StreakState {
	if s.LastActiveDate == nil {
		best := s.Best
		if best < 1 {
			best = 1
		}
		return StreakState{Current: 1, Best: best, TodayDone: 1, LastActiveDate: &today}
	}
	if sameDay(*s.LastActiveDate, today) {
		s.TodayDone++
		return s
	}
	if sameDay(*s.LastActiveDate, today.AddDate(0, 0, -1)) {
		s.Current++
		if s.Current > s.Best {
			s.Best = s.Current
		}
		s.TodayDone = 1
		s.LastActiveDate = &today
		return s
	}
	best := s.Best
	if best < 1 {
		best = 1
	}
	return StreakState{Current: 1, Best: best, TodayDone: 1, LastActiveDate: &today}
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd backend && go test ./internal/progress/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/progress/rules.go backend/internal/progress/rules_test.go
git commit -m "feat(backend): pure daily-streak bump logic"
```

---

### Task 6: `progress.Service` — saved questions + streak persistence

**Files:** create `internal/progress/service.go`, `internal/progress/service_test.go`.

**Interfaces (produced):**
```go
type Service struct{ Q *sqlc.Queries }
func NewService(q *sqlc.Queries) *Service

func (s *Service) SaveQuestion(ctx context.Context, profileID, questionID uuid.UUID) error
func (s *Service) UnsaveQuestion(ctx context.Context, profileID, questionID uuid.UUID) error

type SavedItem struct {
	QuestionID uuid.UUID
	CreatedAt  time.Time
}
func (s *Service) ListSaved(ctx context.Context, profileID uuid.UUID) ([]SavedItem, error)

type StreakView struct {
	Current, Best, TodayDone, DailyGoal int
	LastActiveDate                      *time.Time
}
func (s *Service) GetStreak(ctx context.Context, profileID uuid.UUID) (StreakView, error)

// RecordActivity bumps the caller's streak for "one answered question
// today" — called once per answered question from internal/session.
func (s *Service) RecordActivity(ctx context.Context, profileID uuid.UUID) (StreakView, error)
```

**Logic:**
- `SaveQuestion`/`UnsaveQuestion`/`ListSaved`: thin wrappers over `SaveQuestion`/`UnsaveQuestion`/`ListSavedQuestions` sqlc calls.
- `GetStreak`: `row, err := s.Q.GetStreak(ctx, profileID)`; `pgx.ErrNoRows` → a zero-value `StreakView{DailyGoal: <from limit_config('daily_goal_default').FreeValue>}` (a profile that has never answered anything has no streak row yet — this is a normal, not-an-error state, matching how `billing.Service.Status` returns `false,nil,nil` for a fresh profile rather than erroring).
- `RecordActivity`: fetch current `streak` row (or zero-value `StreakState{}` if `pgx.ErrNoRows`) → convert to `progress.StreakState` → `updated := BumpStreak(state, todayUTC())` (where `todayUTC()` truncates `time.Now().UTC()` to midnight, same day-boundary convention `internal/session`'s daily-practice-limit already established) → if this is a brand-new row, use `limit_config('daily_goal_default').FreeValue` for `DailyGoal`, else keep the existing stored `daily_goal` unchanged (this function never edits a profile's chosen goal) → `s.Q.UpsertStreak(ctx, ...)` → return the resulting `StreakView`.

- [ ] **Step 1: Write the failing tests**

```go
package progress_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
	"avtotest.uz/backend/internal/progress"
	"avtotest.uz/backend/internal/testdb"
)

func seed(t *testing.T) (*sqlc.Queries, *progress.Service, uuid.UUID, uuid.UUID) {
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
	v, err := q.GetVariantByNumber(context.Background(), 1)
	if err != nil {
		t.Fatalf("variant: %v", err)
	}
	qids, err := q.ListVariantQuestionIDsOrdered(context.Background(), v.ID)
	if err != nil || len(qids) == 0 {
		t.Fatalf("question ids: %v", err)
	}
	return q, progress.NewService(q), profile.ID, qids[0]
}

func TestSaveListUnsaveQuestion(t *testing.T) {
	_, svc, profileID, questionID := seed(t)
	ctx := context.Background()

	if err := svc.SaveQuestion(ctx, profileID, questionID); err != nil {
		t.Fatalf("SaveQuestion: %v", err)
	}
	// saving twice must not error (idempotent)
	if err := svc.SaveQuestion(ctx, profileID, questionID); err != nil {
		t.Fatalf("SaveQuestion (repeat): %v", err)
	}

	items, err := svc.ListSaved(ctx, profileID)
	if err != nil {
		t.Fatalf("ListSaved: %v", err)
	}
	if len(items) != 1 || items[0].QuestionID != questionID {
		t.Fatalf("ListSaved = %+v", items)
	}

	if err := svc.UnsaveQuestion(ctx, profileID, questionID); err != nil {
		t.Fatalf("UnsaveQuestion: %v", err)
	}
	items, err = svc.ListSaved(ctx, profileID)
	if err != nil {
		t.Fatalf("ListSaved after unsave: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty after unsave, got %+v", items)
	}
}

func TestGetStreakFreshProfile(t *testing.T) {
	_, svc, profileID, _ := seed(t)
	view, err := svc.GetStreak(context.Background(), profileID)
	if err != nil {
		t.Fatalf("GetStreak: %v", err)
	}
	if view.Current != 0 || view.Best != 0 || view.DailyGoal != 10 {
		t.Fatalf("fresh streak = %+v, want DailyGoal=10 (limit_config default)", view)
	}
}

func TestRecordActivityFirstCallCreatesStreak(t *testing.T) {
	_, svc, profileID, _ := seed(t)
	view, err := svc.RecordActivity(context.Background(), profileID)
	if err != nil {
		t.Fatalf("RecordActivity: %v", err)
	}
	if view.Current != 1 || view.Best != 1 || view.TodayDone != 1 || view.DailyGoal != 10 {
		t.Fatalf("first activity = %+v", view)
	}
}

func TestRecordActivitySameDayOnlyBumpsTodayDone(t *testing.T) {
	_, svc, profileID, _ := seed(t)
	ctx := context.Background()
	if _, err := svc.RecordActivity(ctx, profileID); err != nil {
		t.Fatalf("first: %v", err)
	}
	view, err := svc.RecordActivity(ctx, profileID)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if view.Current != 1 || view.TodayDone != 2 {
		t.Fatalf("second same-day activity = %+v", view)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/progress/... -run "Save|Streak|Activity" -v`
Expected: FAIL — `Service` undefined.

- [ ] **Step 3: Implement** `service.go` per the Logic section above.

- [ ] **Step 4: Run to verify it passes**

Run: `make up && cd backend && go test ./internal/progress/... -p 1 -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/progress/service.go backend/internal/progress/service_test.go
git commit -m "feat(backend): progress.Service — saved questions and streak persistence"
```

---

### Task 7: HTTP handlers for saved questions and streak + server wiring

**Files:** create `internal/progress/handlers.go`, `internal/progress/handlers_test.go`; modify `internal/server/server.go`.

**Routes** (all behind `auth.Required`):
- `GET /me/saved` → `200 [{question_id, created_at}]`
- `POST /me/saved {question_id}` → `200 {ok: true}`
- `DELETE /me/saved/{question_id}` → `200 {ok: true}`
- `GET /me/streak` → `200 {current, best, today_done, daily_goal, last_active_date}`

- [ ] **Step 1: Write the failing test** (mirrors the established `internal/session`/`internal/learning` handler-test pattern: real `testdb` seed, `chi.NewRouter()`, `auth.Required` middleware, real JWT)

```go
package progress_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
	"avtotest.uz/backend/internal/progress"
	"avtotest.uz/backend/internal/testdb"
)

const handlerSecret = "test-secret"

func setupHandlerServer(t *testing.T) (*httptest.Server, string, string) {
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
	v, err := q.GetVariantByNumber(context.Background(), 1)
	if err != nil {
		t.Fatalf("variant: %v", err)
	}
	qids, err := q.ListVariantQuestionIDsOrdered(context.Background(), v.ID)
	if err != nil || len(qids) == 0 {
		t.Fatalf("question ids: %v", err)
	}
	tok, err := auth.IssueAccess([]byte(handlerSecret), profile.ID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	r := chi.NewRouter()
	h := &progress.Handler{Svc: progress.NewService(q)}
	h.Routes(r.With(auth.Required([]byte(handlerSecret))))

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts, tok, qids[0].String()
}

func doReq(t *testing.T, ts *httptest.Server, method, path, token string, body []byte) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, buf
}

func TestSavedQuestionsRoundtripOverHTTP(t *testing.T) {
	ts, tok, questionID := setupHandlerServer(t)

	body, _ := json.Marshal(map[string]string{"question_id": questionID})
	status, _ := doReq(t, ts, http.MethodPost, "/me/saved", tok, body)
	if status != http.StatusOK {
		t.Fatalf("POST /me/saved status=%d", status)
	}

	status, respBody := doReq(t, ts, http.MethodGet, "/me/saved", tok, nil)
	if status != http.StatusOK {
		t.Fatalf("GET /me/saved status=%d", status)
	}
	var env struct {
		Data []struct {
			QuestionID string `json:"question_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 1 || env.Data[0].QuestionID != questionID {
		t.Fatalf("saved list = %+v", env.Data)
	}

	status, _ = doReq(t, ts, http.MethodDelete, "/me/saved/"+questionID, tok, nil)
	if status != http.StatusOK {
		t.Fatalf("DELETE status=%d", status)
	}
}

func TestStreakRequiresAuth(t *testing.T) {
	ts, _, _ := setupHandlerServer(t)
	resp, err := ts.Client().Get(ts.URL + "/me/streak")
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

Run: `cd backend && go test ./internal/progress/... -run "SavedQuestionsRoundtrip|StreakRequiresAuth" -v`
Expected: FAIL — `progress.Handler` undefined.

- [ ] **Step 3: Implement** `handlers.go` (mirror the established `Handler{Svc}`/`Routes(r chi.Router)` pattern; `DELETE /me/saved/{question_id}` parses the UUID path param the same way `internal/session/handlers.go` does). Modify `server.go`: mount `progress.Handler` the same way the other handlers are mounted.

- [ ] **Step 4: Run to verify it passes**

Run: `cd backend && go build ./... && go test ./internal/progress/... -p 1 -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/progress/handlers.go backend/internal/progress/handlers_test.go backend/internal/server/server.go
git commit -m "feat(backend): saved-questions and streak http endpoints"
```

---

### Task 8: Free-tier VIP gating in sessions + streak activity wiring

**Files:** modify `internal/session/service.go`, `internal/session/service_test.go`, `internal/server/server.go`.

**Changes:**
1. `Service` gains a `Progress *progress.Service` field; `NewService`'s signature grows one parameter (append at the end, matching how Task 6 of Plan 04 appended `Learning`). Update every call site: `internal/server/server.go`, `internal/session/service_test.go`, `internal/session/handlers_test.go`.
2. New sentinel: `ErrRequiresVIP = errors.New("active entitlement required")`.
3. In `StartSession`'s mode switch:
   - `"variant"`: after fetching `v, err := s.Q.GetVariantByID(ctx, req.VariantID)` (replaces the current bare `ListVariantQuestionIDsOrdered` call with a fetch of the variant row first, so its `Number` is available), if `v.Number > 1`: `active, _, err := s.Billing.Status(ctx, profileID)`; if `err != nil` propagate; if `!active` return `ErrRequiresVIP`. Variant #1 never requires this check.
   - `"exam"`: at the very top of this case, the same `active, _, err := s.Billing.Status(...)` check — `!active` → `ErrRequiresVIP`.
   - `"mistakes"`: same check.
   - `"practice"`: unchanged — free tier explicitly includes limited daily practice (spec D13); no VIP check added here.
4. In `SubmitAnswer`, right after the existing `s.Learning.RecordReview(...)` call (added by Plan 04), add: `if _, err := s.Progress.RecordActivity(ctx, profileID); err != nil { return AnswerResult{}, err }` — every answered question also bumps the streak, mirroring how it already feeds FSRS.

- [ ] **Step 1: Write the failing tests** (append to `service_test.go`)

```go
func TestStartSessionVariantTwoRequiresVIP(t *testing.T) {
	q, svc, profileID := seed(t)
	v2, err := q.GetVariantByNumber(context.Background(), 2)
	if err != nil {
		t.Fatalf("get variant 2: %v", err)
	}
	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "variant", VariantID: v2.ID, Locale: "uz-Latn",
	}); err != session.ErrRequiresVIP {
		t.Fatalf("err=%v want ErrRequiresVIP", err)
	}
}

func TestStartSessionVariantOneNeverRequiresVIP(t *testing.T) {
	q, svc, profileID := seed(t)
	v1, err := q.GetVariantByNumber(context.Background(), 1)
	if err != nil {
		t.Fatalf("get variant 1: %v", err)
	}
	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "variant", VariantID: v1.ID, Locale: "uz-Latn",
	}); err != nil {
		t.Fatalf("variant 1 should always be accessible: %v", err)
	}
}

func TestStartSessionExamRequiresVIP(t *testing.T) {
	_, svc, profileID := seed(t)
	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "exam", Locale: "uz-Latn",
	}); err != session.ErrRequiresVIP {
		t.Fatalf("err=%v want ErrRequiresVIP", err)
	}
}

func TestStartSessionMistakesRequiresVIP(t *testing.T) {
	_, svc, profileID := seed(t)
	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "mistakes", Locale: "uz-Latn",
	}); err != session.ErrRequiresVIP {
		t.Fatalf("err=%v want ErrRequiresVIP", err)
	}
}

func TestStartSessionPracticeNeverRequiresVIP(t *testing.T) {
	q, svc, profileID := seed(t)
	catID, err := q.GetCategoryIDByCode(context.Background(), "signs")
	if err != nil {
		t.Fatalf("category: %v", err)
	}
	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "practice", CategoryID: catID, Locale: "uz-Latn", Count: 3,
	}); err != nil {
		t.Fatalf("practice should always be accessible (daily-limited, not VIP-gated): %v", err)
	}
}

func TestStartSessionExamWorksForVIPProfile(t *testing.T) {
	q, svc, profileID := seed(t)
	billingSvc := billing.Service{Q: q}
	if _, err := billingSvc.GrantDays(context.Background(), profileID, 7, "admin", "test", uuid.NullUUID{}); err != nil {
		t.Fatalf("grant vip: %v", err)
	}
	if _, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "exam", Locale: "uz-Latn",
	}); err != nil {
		t.Fatalf("VIP profile should be able to start exam: %v", err)
	}
}

func TestSubmitAnswerBumpsStreak(t *testing.T) {
	q, svc, profileID := seed(t)
	view := startVariantSession(t, q, svc, profileID)
	correctID := correctAnswerID(t, q, view.QuestionIDs[0])
	if _, err := svc.SubmitAnswer(context.Background(), profileID, view.ID, view.QuestionIDs[0], correctID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	streakView, err := svc.Progress.GetStreak(context.Background(), profileID)
	if err != nil {
		t.Fatalf("GetStreak: %v", err)
	}
	if streakView.Current != 1 || streakView.TodayDone != 1 {
		t.Fatalf("expected streak bumped after answering, got %+v", streakView)
	}
}
```

> **Implementer note:** these tests need `billing`/`progress`/`uuid` imports added to `service_test.go` if not already present, and the file's `seed()` helper must construct a `progress.Service` and pass it into `session.NewService(...)`'s new parameter — check the actual current signature and every call site (search first, same discipline Plan 04's Task 6 used: `grep -rn "session.NewService(" --include="*.go" .`) before editing.

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/session/... -run "RequiresVIP|NeverRequiresVIP|BumpsStreak" -v`
Expected: FAIL — `ErrRequiresVIP` undefined, `svc.Progress` undefined.

- [ ] **Step 3: Implement** the changes described above in `internal/session/service.go`, update every `session.NewService(...)` call site repo-wide, and add the `writeSessionError` mapping for `ErrRequiresVIP` → 402 `vip_required` in `internal/session/handlers.go`.

- [ ] **Step 4: Run to verify it passes**

Run: `cd backend && go build ./... && go test ./... -p 1 -count=1 -v 2>&1 | tail -60`
Expected: PASS across every package — this task touches a widely-used constructor, so a full-repo build+test is the right verification scope here (not a narrower one), same as Plan 04's Task 6.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/session/ backend/internal/server/server.go
git commit -m "feat(backend): free-tier VIP gating (variant 2+/exam/mistakes) and streak activity wiring"
```

---

### Task 9: `internal/events` — batch event ingestion

**Files:** create `internal/events/service.go`, `internal/events/service_test.go`, `internal/events/handlers.go`, `internal/events/handlers_test.go`; modify `internal/server/server.go`.

**Interfaces:**
```go
type Service struct{ Q *sqlc.Queries }
func NewService(q *sqlc.Queries) *Service

type Event struct {
	Name  string
	Props json.RawMessage // may be nil/empty — defaults to '{}' at the DB layer
	TS    *time.Time       // nil → now()
}

var ErrInvalidRequest = errors.New("invalid event batch")

// LogBatch inserts every event, all attributed to profileID. An empty
// batch or a batch with more than 100 events is ErrInvalidRequest (a
// sane per-request cap — clients batch periodically, not unboundedly).
func (s *Service) LogBatch(ctx context.Context, profileID uuid.UUID, events []Event) error
```

**HTTP route** (behind `auth.Required`): `POST /events {events: [{name, props?, ts?}]}` → `200 {ok: true, count: N}`. Error mapping: `ErrInvalidRequest` → 400 `invalid_request`.

- [ ] **Step 1: Write the failing tests**

```go
package events_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/events"
	"avtotest.uz/backend/internal/testdb"
)

func TestLogBatchInsertsEvents(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	profile, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: "+998901234567"})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	svc := events.NewService(q)

	err = svc.LogBatch(context.Background(), profile.ID, []events.Event{
		{Name: "view_question", Props: json.RawMessage(`{"question_id":"x"}`)},
		{Name: "session_finish"},
	})
	if err != nil {
		t.Fatalf("LogBatch: %v", err)
	}
}

func TestLogBatchRejectsEmpty(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	svc := events.NewService(q)
	if err := svc.LogBatch(context.Background(), uuid.New(), nil); err != events.ErrInvalidRequest {
		t.Fatalf("err=%v want ErrInvalidRequest", err)
	}
}

func TestLogBatchRejectsOversized(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	svc := events.NewService(q)
	big := make([]events.Event, 101)
	for i := range big {
		big[i] = events.Event{Name: "x"}
	}
	if err := svc.LogBatch(context.Background(), uuid.New(), big); err != events.ErrInvalidRequest {
		t.Fatalf("err=%v want ErrInvalidRequest", err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/events/... -v`
Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Implement** `service.go`: validate batch size (`0 < len(events) <= 100`, else `ErrInvalidRequest`), loop calling `s.Q.InsertEvent(ctx, sqlc.InsertEventParams{ProfileID: uuid.NullUUID{UUID: profileID, Valid: true}, Name: e.Name, Props: propsOrEmptyObject(e.Props), Ts: pgtype.Timestamptz{Time: tsOrNow(e.TS), Valid: true}})` for each event (propagate the first error, matching this codebase's established no-explicit-transaction style for multi-row writes — e.g. `internal/session`'s `SubmitAnswer` doesn't wrap its several sequential writes in a transaction either). Then `handlers.go` (mirror the established `Handler{Svc}`/`Routes`/`decodeBody` pattern) and the `server.go` mount.

- [ ] **Step 4: Run to verify it passes**

Run: `cd backend && go build ./... && go test ./internal/events/... -p 1 -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/events/ backend/internal/server/server.go
git commit -m "feat(backend): batch event ingestion endpoint"
```

---

### Task 10: Full verification + docs

- [ ] **Step 1:** `make check` (lint + `-p 1` full suite) green — run from the repo root.

- [ ] **Step 2: Live smoke** (PORT=8090, established pattern): sign in via OTP sandbox flow, then:
  1. `POST /api/v1/sessions {"mode":"exam",...}` (fresh free profile) → confirm `402 vip_required`.
  2. Run `grantvip` for the same phone → retry step 1 → confirm it now succeeds.
  3. `POST /api/v1/me/saved {"question_id":"<id>"}` → `GET /api/v1/me/saved` → confirm it appears → `DELETE /api/v1/me/saved/{id}` → confirm it's gone.
  4. `GET /api/v1/me/streak` → confirm `current=0` initially; answer a question in any session → `GET /api/v1/me/streak` again → confirm `current=1, today_done>=1`.
  5. Run `gendraft -question <id-with-no-explanation-yet>` → confirm it prints success. `POST /api/v1/explanations/feedback {"question_id":"<that id>","helpful":true}` (now succeeds, since a draft exists) → confirm `200`. Run `verifyexplanation -question <id> -by <profile-id>` → confirm success.
  6. `POST /api/v1/events {"events":[{"name":"smoke_test"}]}` → confirm `200 {ok:true,count:1}`.
  Record all outputs.

- [ ] **Step 3: README** — add sections for: explanations (draft/verify/feedback workflow, CLI usage, explicit "AI-draft is a stub, no real LLM yet" note), saved questions, streak, the free-tier VIP boundary (which modes require an entitlement now), and event logging. Mirror the existing Auth/Sessiya/FSRS sections' style. Commit:

```bash
git add README.md
git commit -m "docs: explanations, saved questions, streak, free-tier gating, event logging"
```

## Self-Review

1. **Spec coverage:** §13 (izohlar AI-draft→verify→feedback, model in M1/UI in M3) → Tasks 2-4; saved questions (§14 API list `GET/POST/DELETE /me/saved`) → Tasks 5-7; streak (§11 "kunlik maqsad+streak") → Tasks 5-8; D13 free-tier boundary (never enforced by a prior plan — a real gap this plan closes) → Task 8; §19/§22 event logging (`POST /events`) → Task 9. Streak's *goal-editing* UI and the explanation quality-review *UI* are both explicitly M3, not this plan — noted as intentional scope boundaries, not gaps.
2. **Placeholders:** none — every task has real code or a fully specified logic/test list, matching the established style of Plans 03-04's DB-integration tasks.
3. **Type consistency:** `explanation.Service`/`Block`/`AIDraftGenerator` names are used identically across Tasks 2-4; `progress.Service`/`StreakState`/`StreakView`/`SavedItem` across Tasks 5-8; `session.ErrRequiresVIP` and the `Progress *progress.Service` field introduced in Task 8 match how Plan 04's Task 6 handled the equivalent `Learning *learning.Service` addition (search-first discipline, single task updates every call site).
