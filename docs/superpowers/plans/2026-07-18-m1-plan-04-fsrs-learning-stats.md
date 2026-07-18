# M1 Plan 04 — FSRS Learning Engine, Mastery Map, Exam-Readiness Stats

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Plan 03's placeholder Leitner mistake-bank with a real FSRS (Free Spaced Repetition Scheduler) memory model — per-question stability/difficulty/due-date scheduling, a due-questions queue with weak-area interleaving, a per-category mastery map, and an overall "exam readiness %" prediction — all server-side and unit-tested against hand-verified reference values of the published FSRS-4.5 algorithm.

**Architecture:** A new `internal/learning` package owns FSRS end to end: pure scheduling math (`fsrs.go`, no DB — mirrors `internal/session/rules.go`'s pure/DB split), a DB-integrated `Service` (`service.go`: `RecordReview`, `NextDue`, `Stats`), and HTTP handlers (`handlers.go`: `GET /learn/next`, `POST /learn/review`, `GET /me/stats`), mounted under `/api/v1` behind `auth.Required` exactly like `internal/session`. `internal/session.Service.SubmitAnswer` is modified to call `learning.Service.RecordReview` for every answered question (replacing Plan 03's ad-hoc `MarkQuestionWrong`/`MarkQuestionCorrectInMistakesMode` calls), so every session answer — in any of the 4 modes — now feeds the real FSRS schedule, not just mistakes-mode. The mistakes-mode bank query is redefined to select on FSRS's own `lapses > 0 AND due_at <= now()` instead of Plan 03's placeholder Leitner `state` flag.

**Tech Stack:** No new dependencies — same stack as Plans 01-03 (Go stdlib `math`, `time`; pgx/sqlc; chi).

## Global Constraints

- All Plan 01-03 conventions hold (envelope, locales, commit style, `-p 1` tests, `export PATH=$HOME/.local/go/bin:$HOME/go/bin:$PATH`, `make up` before any DB test run).
- **Algorithm version:** FSRS-4.5 (19 weights), matching the widely-published open-source reference (cross-checked against `github.com/open-spaced-repetition` docs and a from-scratch Rust walkthrough — see task 1 for the exact formulas). Default weights (index 0-18):
  ```
  w = [0.40255, 1.18385, 3.173, 15.69105, 7.1949, 0.5345, 1.4604, 0.0046, 1.54575,
       0.1192, 1.01925, 1.9395, 0.11, 0.29605, 2.2698, 0.2315, 2.9898, 0.51655, 0.6621]
  ```
  Constants: `FACTOR = 19.0/81.0`, `DECAY = -0.5`. Default desired retention: `0.9` (90%), not configurable in M1 (YAGNI — spec doesn't call for a tunable target).
- **Rating scale:** `Again=1, Hard=2, Good=3, Easy=4` (`learning.Rating`, standard FSRS 4-point grading). `POST /learn/review` accepts an explicit rating from the client (real self-assessed spaced-repetition UX). Session-embedded reviews (from `internal/session.SubmitAnswer`, which only has a binary correct/incorrect signal) map `correct → Good`, `incorrect → Again` — a deliberate simplification stated here explicitly, not a spec gap.
- **Simplified 2-state model:** the full FSRS spec distinguishes New/Learning/Review/Relearning (4 states) for sub-day learning-step scheduling. M1 deliberately collapses this to 2 states — **new** (no `question_memory` row yet: use the `S0`/`D0` initial formulas) and **reviewed** (row exists: use the update formulas) — because this app reviews YHQ exam questions at most a few times a day, not the multi-times-per-day flashcard cadence FSRS's Learning sub-states exist for. The `question_memory.state` column stores the outcome of the *most recent* review only: `0` if the last rating was `Again`, `1` otherwise. This is a deliberate scope decision, not an oversight — call it out if a future plan needs true 4-state learning steps.
- **`reps`/`lapses` become monotonic counters**, redefining Plan 03's placeholder semantics (which reset `reps` to 0 on a wrong answer, using it as a "consecutive correct" counter — that placeholder is being replaced by this plan, exactly as Plan 03 anticipated: *"Plan 04 (FSRS) later layers stability/difficulty/due_at scheduling on the same table without redefining these two fields"* — this plan refines that statement: `reps` = total number of reviews ever recorded (increments every review, never resets); `lapses` = total number of `Again` ratings ever recorded (increments only on failure, never resets). This is standard FSRS terminology and does not conflict with the scheduling math, which depends only on `stability`/`difficulty`/`due_at`, not on `reps`/`lapses` — those two are counters for reporting, not scheduling inputs.
- **Mistakes-bank query redefinition:** Plan 03's `ListMistakeBankQuestionIDs` selected `WHERE state = 0` (its own placeholder "active mistake" flag, cleared by a Leitner "2 consecutive correct in mistakes-mode" rule). This plan replaces that placeholder with the real signal: a question is in the bank iff it has been forgotten at least once (`lapses > 0`) AND is currently due for review (`due_at <= now()`). FSRS's own scheduling naturally removes a question from this set once a successful review pushes `due_at` into the future — no separate "clear after N consecutive corrects" counter is needed anymore, so Plan 03's `MistakeClearAfter`/`MistakeCleared` (in `internal/session/rules.go`) are removed in this plan's Task 6, not left dangling.
- **Category mastery formula:** `mastery = correct / seen` (simple ratio, recomputed and stored on every review of a question in that category). `seen`/`correct` increment on every `RecordReview` call for a question in that category (`correct` increments unless the rating was `Again`).
- **Exam-readiness % formula:** weighted average of category mastery, weighted by each category's total valid-question count (categories the profile has never touched contribute `mastery=0` but still count in the weighted denominator — this rewards both breadth of coverage and depth of mastery): `readiness = round(100 * Σ(category.mastery × category.question_count) / Σ(category.question_count))`.
- **Streak is out of scope for this plan.** `streak` (daily goal/current/best) is untouched — it belongs to a later plan (M1's own milestone breakdown groups it with explanations-AI-draft/saved-questions/event-logging, not FSRS). `GET /me/stats`'s response in this plan has no `streak` field; that's intentional, not a gap.
- **`GET /learn/next` scope:** returns only currently-**due** reviews (`question_memory.due_at <= now()` for the profile) — it does NOT introduce brand-new, never-attempted questions into the curriculum; that's what session-based `practice`/`variant` modes (Plan 03) already do. Interleaves across categories (round-robin by ascending due-urgency) rather than returning one category's due items as a contiguous block.
- **No new migrations** — `question_memory` and `category_mastery` already exist in full from Plan 01's `0004_learning.up.sql`; this plan only changes how they're read/written.
- Error sentinels (new, in `internal/learning`): `ErrInvalidRating` (`errors.New`, mapped to HTTP 400 `invalid_rating`).

## File Structure (new/modified)

```
backend/
  internal/db/queries/
    learning.sql                          # new: question_memory/category_mastery queries + sqlc generate
    session.sql                           # modify: ListMistakeBankQuestionIDs redefinition; remove MarkQuestionWrong/MarkQuestionCorrectInMistakesMode
  internal/learning/
    fsrs.go fsrs_test.go                  # pure: Rating, Card, Review()
    service.go service_test.go            # RecordReview, NextDue, Stats
    handlers.go handlers_test.go          # GET /learn/next, POST /learn/review, GET /me/stats
  internal/session/
    rules.go rules_test.go                # modify: remove MistakeClearAfter/MistakeCleared
    service.go service_test.go            # modify: SubmitAnswer calls learning.Service.RecordReview
  internal/server/server.go               # mount learning routes (modify)
```

---

### Task 1: Pure FSRS algorithm (no DB)

**Files:** create `internal/learning/fsrs.go`, `internal/learning/fsrs_test.go`.

**Interfaces (produced):**
- `learning.Rating` (`int`): `Again=1, Hard=2, Good=3, Easy=4`
- `learning.Card{Stability, Difficulty float64; DueAt, LastReviewedAt time.Time; Reps, Lapses int; State int16}`
- `learning.Card.IsNew() bool` — true iff `LastReviewedAt.IsZero()`
- `learning.DefaultDesiredRetention = 0.9`
- `learning.Review(c Card, rating Rating, now time.Time, desiredRetention float64) Card` — pure, no side effects

**Exact formulas (FSRS-4.5, cross-verified against `github.com/open-spaced-repetition` project docs and an independent from-scratch walkthrough; weights indexed 0-18 as in Global Constraints):**

- Retrievability (forgetting curve), elapsed days `t` since last review: `R(t, S) = (1 + FACTOR·t/S)^DECAY`
- Initial stability (first review, grade `G`): `S0(G) = w[G-1]`
- Initial difficulty (first review, grade `G`): `D0(G) = clamp(w[4] - e^(w[5]·(G-1)) + 1, 1, 10)`
- Next interval (days) for target retention `r`: `I(r, S) = (S/FACTOR)·(r^(1/DECAY) - 1)` — note this identity: `I(0.9, S) == S` exactly, by construction of `FACTOR` (a good structural sanity check, not a coincidence).
- Stability after a **successful** review (`G ∈ {Hard=2, Good=3, Easy=4}`), given current `S`, `D`, and retrievability `R` at review time:
  ```
  t_d = 11 - D
  t_s = S^(-w[9])
  t_r = e^(w[10]·(1-R)) - 1
  h   = w[15] if G == Hard else 1
  b   = w[16] if G == Easy else 1
  S'  = S · (1 + t_d · t_s · t_r · h · b · e^(w[8]))
  ```
- Stability after a **failed** review (`G == Again`):
  ```
  d_f = D^(-w[12])
  s_f = (S+1)^(w[13]) - 1
  r_f = e^(w[14]·(1-R))
  S'  = min(w[11] · d_f · s_f · r_f, S)   -- forgetting never increases stability
  ```
- Difficulty update (any review after the first), given current `D` and grade `G`:
  ```
  ΔD(G) = -w[6]·(G-3)
  D'(D,G) = D + ΔD(G)·((10-D)/9)
  D'' = clamp(w[7]·D0(4) + (1-w[7])·D'(D,G), 1, 10)
  ```

**Hand-verified reference values** (computed independently with the exact weights above — use these as test fixtures, not fabricated numbers):

| Case | Expected |
|---|---|
| `S0(Again)` | `0.40255` |
| `S0(Hard)` | `1.18385` |
| `S0(Good)` | `3.173` |
| `S0(Easy)` | `15.69105` |
| `D0(Again)` | `7.1949` |
| `D0(Hard)` | `6.4883` (±0.0005) |
| `D0(Good)` | `5.2824` (±0.0005) |
| `D0(Easy)` | `3.2245` (±0.0005) |
| `I(0.9, S)` for any `S` | `== S` exactly (±1e-9) |
| 2nd review, `Good` at `t=3` days after a first `Good` review (`S=3.173, D=5.28243`) | `S' ≈ 10.73893` (±0.001), `D' ≈ 5.27297` (±0.001) |
| Then a 3rd review, `Again` at `t=5` days after that (`S=10.73893, D=5.27297`) | `S' ≈ 1.94435` (±0.001), `D' ≈ 6.79057` (±0.001) |

- [ ] **Step 1: Write the failing tests**

```go
package learning

import (
	"math"
	"testing"
	"time"
)

func approxEqual(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %v, want %v (tol %v)", name, got, want, tol)
	}
}

func TestReviewFirstTimeInitialValues(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		rating   Rating
		wantS    float64
		wantD    float64
	}{
		{Again, 0.40255, 7.1949},
		{Hard, 1.18385, 6.4883},
		{Good, 3.173, 5.2824},
		{Easy, 15.69105, 3.2245},
	}
	for _, c := range cases {
		card := Review(Card{}, c.rating, now, DefaultDesiredRetention)
		approxEqual(t, "stability", card.Stability, c.wantS, 1e-4)
		approxEqual(t, "difficulty", card.Difficulty, c.wantD, 5e-4)
		if card.Reps != 1 {
			t.Errorf("Reps = %d, want 1", card.Reps)
		}
		if c.rating == Again && card.Lapses != 1 {
			t.Errorf("Lapses = %d, want 1 for Again", card.Lapses)
		}
		if c.rating != Again && card.Lapses != 0 {
			t.Errorf("Lapses = %d, want 0 for non-Again", card.Lapses)
		}
		wantState := int16(1)
		if c.rating == Again {
			wantState = 0
		}
		if card.State != wantState {
			t.Errorf("State = %d, want %d", card.State, wantState)
		}
		if !card.DueAt.After(now) {
			t.Errorf("DueAt %v must be after review time %v", card.DueAt, now)
		}
	}
}

func TestIntervalAtDesiredRetentionEqualsStability(t *testing.T) {
	// I(r, S) == S exactly when r == the target retention FACTOR was derived
	// from (0.9) — a structural identity, not a coincidence.
	for _, s := range []float64{0.4, 3.173, 15.69, 50.0} {
		got := interval(0.9, s)
		approxEqual(t, "interval", got, s, 1e-9)
	}
}

func TestReviewSecondTimeGoodIncreaseStability(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	first := Review(Card{}, Good, now, DefaultDesiredRetention)
	second := Review(first, Good, now.AddDate(0, 0, 3), DefaultDesiredRetention)
	approxEqual(t, "stability after 2nd Good", second.Stability, 10.73893, 1e-3)
	approxEqual(t, "difficulty after 2nd Good", second.Difficulty, 5.27297, 1e-3)
	if second.Reps != 2 {
		t.Errorf("Reps = %d, want 2", second.Reps)
	}
	if second.Lapses != 0 {
		t.Errorf("Lapses = %d, want 0", second.Lapses)
	}
	if second.State != 1 {
		t.Errorf("State = %d, want 1", second.State)
	}

	third := Review(second, Again, now.AddDate(0, 0, 3+5), DefaultDesiredRetention)
	approxEqual(t, "stability after fail", third.Stability, 1.94435, 1e-3)
	approxEqual(t, "difficulty after fail", third.Difficulty, 6.79057, 1e-3)
	if third.Reps != 3 {
		t.Errorf("Reps = %d, want 3", third.Reps)
	}
	if third.Lapses != 1 {
		t.Errorf("Lapses = %d, want 1", third.Lapses)
	}
	if third.State != 0 {
		t.Errorf("State = %d, want 0", third.State)
	}
	if third.Stability >= second.Stability {
		t.Errorf("a forgotten review must not increase stability: got %v, was %v", third.Stability, second.Stability)
	}
}

func TestReviewStabilityOrderingByGrade(t *testing.T) {
	// Easy > Good > Hard > Again for a first review's initial stability —
	// higher confidence grades must produce longer initial intervals.
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	again := Review(Card{}, Again, now, DefaultDesiredRetention)
	hard := Review(Card{}, Hard, now, DefaultDesiredRetention)
	good := Review(Card{}, Good, now, DefaultDesiredRetention)
	easy := Review(Card{}, Easy, now, DefaultDesiredRetention)
	if !(again.Stability < hard.Stability && hard.Stability < good.Stability && good.Stability < easy.Stability) {
		t.Fatalf("expected strictly increasing stability Again<Hard<Good<Easy, got %v %v %v %v",
			again.Stability, hard.Stability, good.Stability, easy.Stability)
	}
}

func TestCardIsNew(t *testing.T) {
	var c Card
	if !c.IsNew() {
		t.Fatal("zero-value Card must be new")
	}
	c.LastReviewedAt = time.Now()
	if c.IsNew() {
		t.Fatal("Card with LastReviewedAt set must not be new")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/learning/... -v`
Expected: FAIL — package/functions don't exist.

- [ ] **Step 3: Implement**

```go
// Package learning implements FSRS-4.5 (Free Spaced Repetition Scheduler) —
// the memory model behind due-question scheduling, weak-area detection, and
// exam-readiness prediction. fsrs.go is pure (no DB); service.go integrates
// it with question_memory/category_mastery.
package learning

import (
	"math"
	"time"
)

type Rating int

const (
	Again Rating = 1
	Hard  Rating = 2
	Good  Rating = 3
	Easy  Rating = 4
)

// DefaultDesiredRetention is the target recall probability FSRS schedules
// reviews for (90%) — not configurable in M1.
const DefaultDesiredRetention = 0.9

// w holds the 19 FSRS-4.5 default weights (see plan Global Constraints for
// provenance).
var w = [19]float64{
	0.40255, 1.18385, 3.173, 15.69105, 7.1949, 0.5345, 1.4604, 0.0046, 1.54575,
	0.1192, 1.01925, 1.9395, 0.11, 0.29605, 2.2698, 0.2315, 2.9898, 0.51655, 0.6621,
}

const (
	factor = 19.0 / 81.0
	decay  = -0.5
)

type Card struct {
	Stability      float64
	Difficulty     float64
	DueAt          time.Time
	LastReviewedAt time.Time
	Reps           int
	Lapses         int
	State          int16 // 0 = last rating was Again, 1 = otherwise
}

// IsNew reports whether this card has never been reviewed.
func (c Card) IsNew() bool { return c.LastReviewedAt.IsZero() }

func clamp(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

func s0(g Rating) float64 { return w[g-1] }

func d0(g Rating) float64 {
	return clamp(w[4]-math.Exp(w[5]*float64(g-1))+1, 1, 10)
}

func retrievability(t, s float64) float64 {
	return math.Pow(1+factor*t/s, decay)
}

func interval(r, s float64) float64 {
	return (s / factor) * (math.Pow(r, 1/decay) - 1)
}

func stabilitySuccess(s, d, r float64, g Rating) float64 {
	td := 11 - d
	ts := math.Pow(s, -w[9])
	tr := math.Exp(w[10]*(1-r)) - 1
	h := 1.0
	if g == Hard {
		h = w[15]
	}
	b := 1.0
	if g == Easy {
		b = w[16]
	}
	return s * (1 + td*ts*tr*h*b*math.Exp(w[8]))
}

func stabilityFail(s, d, r float64) float64 {
	df := math.Pow(d, -w[12])
	sf := math.Pow(s+1, w[13]) - 1
	rf := math.Exp(w[14] * (1 - r))
	forgotten := w[11] * df * sf * rf
	return math.Min(forgotten, s)
}

func difficultyUpdate(d float64, g Rating) float64 {
	deltaD := -w[6] * float64(g-3)
	dPrime := d + deltaD*((10-d)/9)
	return clamp(w[7]*d0(Easy)+(1-w[7])*dPrime, 1, 10)
}

// Review computes the next Card state after grading a review at time now,
// targeting desiredRetention (e.g. DefaultDesiredRetention).
func Review(c Card, rating Rating, now time.Time, desiredRetention float64) Card {
	var newStability, newDifficulty float64
	if c.IsNew() {
		newStability = s0(rating)
		newDifficulty = d0(rating)
	} else {
		t := now.Sub(c.LastReviewedAt).Hours() / 24
		r := retrievability(t, c.Stability)
		if rating == Again {
			newStability = stabilityFail(c.Stability, c.Difficulty, r)
		} else {
			newStability = stabilitySuccess(c.Stability, c.Difficulty, r, rating)
		}
		newDifficulty = difficultyUpdate(c.Difficulty, rating)
	}

	days := interval(desiredRetention, newStability)
	if days < 1 {
		days = 1
	}
	state := int16(1)
	lapses := c.Lapses
	if rating == Again {
		state = 0
		lapses++
	}

	return Card{
		Stability:      newStability,
		Difficulty:     newDifficulty,
		DueAt:          now.Add(time.Duration(math.Round(days)) * 24 * time.Hour),
		LastReviewedAt: now,
		Reps:           c.Reps + 1,
		Lapses:         lapses,
		State:          state,
	}
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd backend && go test ./internal/learning/... -v`
Expected: PASS (all tests).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/learning/fsrs.go backend/internal/learning/fsrs_test.go
git commit -m "feat(backend): FSRS-4.5 pure scheduling algorithm"
```

---

### Task 2: sqlc queries for learning + mistakes-bank redefinition

**Files:** create `internal/db/queries/learning.sql`; modify `internal/db/queries/session.sql` (redefine `ListMistakeBankQuestionIDs`, remove `MarkQuestionWrong`/`MarkQuestionCorrectInMistakesMode`); run `sqlc generate`.

- [ ] **Step 1: Write `learning.sql`**

```sql
-- name: GetQuestionMemory :one
SELECT * FROM question_memory
WHERE profile_id = sqlc.arg(profile_id) AND question_id = sqlc.arg(question_id);

-- name: UpsertQuestionMemory :one
INSERT INTO question_memory
  (profile_id, question_id, stability, difficulty, due_at, last_reviewed_at, reps, lapses, state)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (profile_id, question_id) DO UPDATE SET
  stability        = EXCLUDED.stability,
  difficulty       = EXCLUDED.difficulty,
  due_at           = EXCLUDED.due_at,
  last_reviewed_at = EXCLUDED.last_reviewed_at,
  reps             = EXCLUDED.reps,
  lapses           = EXCLUDED.lapses,
  state            = EXCLUDED.state
RETURNING *;

-- name: ListDueQuestions :many
SELECT qm.question_id, q.category_id
FROM question_memory qm
JOIN question q ON q.id = qm.question_id AND q.validation_status = 'valid'
WHERE qm.profile_id = sqlc.arg(profile_id) AND qm.due_at <= now()
ORDER BY qm.due_at ASC
LIMIT sqlc.arg(limit_count);

-- name: CountDueQuestions :one
SELECT count(*)::int FROM question_memory
WHERE profile_id = $1 AND due_at <= now();

-- name: GetQuestionCategoryID :one
SELECT category_id FROM question WHERE id = $1;

-- name: UpsertCategoryMastery :one
INSERT INTO category_mastery (profile_id, category_id, mastery, seen, correct, updated_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (profile_id, category_id) DO UPDATE SET
  mastery    = EXCLUDED.mastery,
  seen       = EXCLUDED.seen,
  correct    = EXCLUDED.correct,
  updated_at = now()
RETURNING *;

-- name: GetCategoryMastery :one
SELECT * FROM category_mastery
WHERE profile_id = sqlc.arg(profile_id) AND category_id = sqlc.arg(category_id);

-- name: ListCategoryMasteryForProfile :many
SELECT * FROM category_mastery WHERE profile_id = $1;

-- name: CountValidQuestionsByCategory :many
SELECT category_id, count(*)::int AS question_count
FROM question WHERE validation_status = 'valid'
GROUP BY category_id;
```

- [ ] **Step 2: Modify `session.sql`**

Replace the existing `ListMistakeBankQuestionIDs` query:
```sql
-- name: ListMistakeBankQuestionIDs :many
SELECT question_id FROM question_memory
WHERE profile_id = sqlc.arg(profile_id) AND lapses > 0 AND due_at <= now()
ORDER BY due_at ASC
LIMIT sqlc.arg(limit_count);
```

Delete the `MarkQuestionWrong` and `MarkQuestionCorrectInMistakesMode` query blocks entirely — they're superseded by `learning.Service.RecordReview` (Task 3) and will have no remaining Go callers after Task 6 removes their call sites in `internal/session/service.go`.

- [ ] **Step 3: Generate and build**

Run: `cd backend && sqlc generate && go build ./...`
Expected: build FAILS at this point — `internal/session/service.go` still calls the now-deleted `MarkQuestionWrong`/`MarkQuestionCorrectInMistakesMode` sqlc functions. **This is expected and correct** — Task 6 fixes the call sites. Confirm the failure is exactly these two missing symbols (not something else), then stop here; do not patch `session.go` in this task.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/db/queries/learning.sql backend/internal/db/queries/session.sql backend/internal/db/sqlc/
git commit -m "feat(backend): sqlc queries for FSRS memory, mastery map, mistakes-bank redefinition"
```

(This commit intentionally leaves `internal/session` non-compiling until Task 6 — the plan is executed task-by-task by a single continuous line of work, not independently mergeable checkpoints, since Tasks 2-6 are one tightly-coupled refactor of a shared table's meaning. Task 3-5 add the new package; Task 6 fixes the compile break in the same PR-equivalent unit of work before final verification.)

---

### Task 3: `learning.Service.RecordReview`

**Files:** create `internal/learning/dto.go`, `internal/learning/service.go`, `internal/learning/service_test.go`.

**Interfaces (produced):**
```go
type Service struct {
	Q *sqlc.Queries
}

func NewService(q *sqlc.Queries) *Service

var ErrInvalidRating = errors.New("invalid rating")

func (s *Service) RecordReview(ctx context.Context, profileID, questionID uuid.UUID, rating Rating) (Card, error)
```

**Logic:**
1. `rating` must be one of `Again/Hard/Good/Easy` (1-4) — else `ErrInvalidRating`.
2. `row, err := s.Q.GetQuestionMemory(ctx, ...)`; `pgx.ErrNoRows` → treat as a zero-value `Card{}` (new); any other error propagates.
3. Convert the DB row (if found) to a `Card{Stability, Difficulty, DueAt: row.DueAt.Time, LastReviewedAt: row.LastReviewedAt.Time, Reps: int(row.Reps), Lapses: int(row.Lapses), State: row.State}`.
4. `updated := Review(card, rating, time.Now(), DefaultDesiredRetention)`.
5. `s.Q.UpsertQuestionMemory(ctx, ...)` with all of `updated`'s fields (`LastReviewedAt`/`DueAt` as `pgtype.Timestamptz{Time: ..., Valid: true}`).
6. Fetch `catID, err := s.Q.GetQuestionCategoryID(ctx, questionID)`.
7. Fetch current mastery: `m, err := s.Q.GetCategoryMastery(ctx, ...)`; `pgx.ErrNoRows` → `seen=0, correct=0`.
8. `seen := m.Seen + 1`; `correct := m.Correct`; if `rating != Again` then `correct++`.
9. `mastery := float32(correct) / float32(seen)`.
10. `s.Q.UpsertCategoryMastery(ctx, ...)`.
11. Return `updated`.

- [ ] **Step 1: Write the failing tests**

```go
package learning_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
	"avtotest.uz/backend/internal/learning"
	"avtotest.uz/backend/internal/testdb"
)

func seed(t *testing.T) (*sqlc.Queries, *learning.Service, uuid.UUID, []uuid.UUID) {
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
		t.Fatalf("get variant: %v", err)
	}
	qids, err := q.ListVariantQuestionIDsOrdered(context.Background(), v.ID)
	if err != nil || len(qids) == 0 {
		t.Fatalf("question ids: %v %d", err, len(qids))
	}
	svc := learning.NewService(q)
	return q, svc, profile.ID, qids
}

func TestRecordReviewFirstTimeCreatesRow(t *testing.T) {
	q, svc, profileID, qids := seed(t)
	card, err := svc.RecordReview(context.Background(), profileID, qids[0], learning.Good)
	if err != nil {
		t.Fatalf("RecordReview: %v", err)
	}
	if card.Reps != 1 || card.Lapses != 0 {
		t.Fatalf("unexpected card: %+v", card)
	}

	row, err := q.GetQuestionMemory(context.Background(), sqlc.GetQuestionMemoryParams{ProfileID: profileID, QuestionID: qids[0]})
	if err != nil {
		t.Fatalf("GetQuestionMemory: %v", err)
	}
	if row.Reps != 1 {
		t.Fatalf("stored reps = %d, want 1", row.Reps)
	}
}

func TestRecordReviewUpdatesCategoryMastery(t *testing.T) {
	q, svc, profileID, qids := seed(t)
	catID, err := q.GetQuestionCategoryID(context.Background(), qids[0])
	if err != nil {
		t.Fatalf("category: %v", err)
	}

	if _, err := svc.RecordReview(context.Background(), profileID, qids[0], learning.Good); err != nil {
		t.Fatalf("review 1: %v", err)
	}
	m, err := q.GetCategoryMastery(context.Background(), sqlc.GetCategoryMasteryParams{ProfileID: profileID, CategoryID: catID})
	if err != nil {
		t.Fatalf("mastery: %v", err)
	}
	if m.Seen != 1 || m.Correct != 1 {
		t.Fatalf("mastery after 1 correct = %+v", m)
	}

	if _, err := svc.RecordReview(context.Background(), profileID, qids[1], learning.Again); err != nil {
		t.Fatalf("review 2: %v", err)
	}
	catID2, err := q.GetQuestionCategoryID(context.Background(), qids[1])
	if err != nil {
		t.Fatalf("category 2: %v", err)
	}
	if catID2 == catID {
		m2, err := q.GetCategoryMastery(context.Background(), sqlc.GetCategoryMasteryParams{ProfileID: profileID, CategoryID: catID})
		if err != nil {
			t.Fatalf("mastery: %v", err)
		}
		if m2.Seen != 2 || m2.Correct != 1 {
			t.Fatalf("mastery after 1 correct + 1 wrong (same category) = %+v", m2)
		}
	}
}

func TestRecordReviewInvalidRating(t *testing.T) {
	_, svc, profileID, qids := seed(t)
	if _, err := svc.RecordReview(context.Background(), profileID, qids[0], learning.Rating(99)); err != learning.ErrInvalidRating {
		t.Fatalf("err=%v want ErrInvalidRating", err)
	}
}

func TestRecordReviewSecondTimeUsesStoredState(t *testing.T) {
	q, svc, profileID, qids := seed(t)
	first, err := svc.RecordReview(context.Background(), profileID, qids[0], learning.Good)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := svc.RecordReview(context.Background(), profileID, qids[0], learning.Good)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if second.Stability <= first.Stability {
		t.Fatalf("a 2nd successful review must increase stability further: first=%v second=%v", first.Stability, second.Stability)
	}
	if second.Reps != 2 {
		t.Fatalf("Reps = %d, want 2", second.Reps)
	}
	row, err := q.GetQuestionMemory(context.Background(), sqlc.GetQuestionMemoryParams{ProfileID: profileID, QuestionID: qids[0]})
	if err != nil {
		t.Fatalf("GetQuestionMemory: %v", err)
	}
	if row.Reps != 2 {
		t.Fatalf("stored reps = %d, want 2", row.Reps)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/learning/... -v`
Expected: FAIL — `Service`/`RecordReview` undefined.

- [ ] **Step 3: Implement** `dto.go` (none needed beyond what `fsrs.go` already exports — this file can be skipped if there's nothing to add yet; `service.go` grows a `SessionSummary`-style DTO in Task 4) and `service.go` per the Logic section above.

- [ ] **Step 4: Run to verify it passes**

Run: `make up && cd backend && go test ./internal/learning/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/learning/service.go backend/internal/learning/service_test.go
git commit -m "feat(backend): learning.Service.RecordReview — FSRS persistence + mastery update"
```

---

### Task 4: `learning.Service.NextDue` (interleaving) + `Stats` (readiness %)

**Files:** modify `internal/learning/service.go`, create `internal/learning/dto.go` (if not already present from Task 3), extend `service_test.go`.

**Interfaces (produced):**
```go
func (s *Service) NextDue(ctx context.Context, profileID uuid.UUID, limit int) ([]uuid.UUID, error)

type CategoryStat struct {
	CategoryCode string
	Mastery      float64
	Seen, Correct int
}

type Stats struct {
	Categories   []CategoryStat
	ReadinessPct int
	DueCount     int
}

func (s *Service) Stats(ctx context.Context, profileID uuid.UUID) (Stats, error)
```

**`NextDue` logic** (round-robin interleave across categories by ascending due-urgency):
1. `limit <= 0` → default 20.
2. `rows, err := s.Q.ListDueQuestions(ctx, sqlc.ListDueQuestionsParams{ProfileID: profileID, LimitCount: int32(limit*3)})` — over-fetch a working set (each row has `QuestionID`, `CategoryID`), already ordered by `due_at ASC`.
3. Group `rows` by `CategoryID`, preserving first-appearance order of each category (this order reflects which category has the single most-overdue item).
4. Round-robin: repeatedly take one question from each category (in that first-appearance order) until `limit` is reached or every category's group is exhausted. This guarantees no long run of same-category questions when multiple categories have due items.
5. Return the resulting `[]uuid.UUID`, length `<= limit`.

**`Stats` logic:**
1. `categories, err := s.Q.ListCategoryMasteryForProfile(ctx, profileID)` → map `category_id -> {mastery, seen, correct}`.
2. `counts, err := s.Q.CountValidQuestionsByCategory(ctx)` → map `category_id -> question_count`, and the full set of category IDs that exist in the catalog (categories the profile never touched get `mastery=0` in the weighted sum below).
3. `catInfo, err := s.Q.ListCategories(ctx, "uz-Latn")` (existing Plan 01 query) — used only to get every category's `code`+`id` for the response; loop over these (not just the ones the profile has touched) so untouched categories appear with `mastery=0`.
4. For each category: `mastery = 0` if untouched, else the stored value; build `CategoryStat{CategoryCode: code, Mastery: mastery, Seen: seen, Correct: correct}`.
5. `readiness := round(100 * Σ(mastery_i × count_i) / Σ(count_i))` over all categories with `count_i > 0` (a category with zero valid questions contributes nothing to either sum, avoiding a divide-by-zero on an empty catalog edge case — if the total is 0, `readiness = 0`).
6. `dueCount, err := s.Q.CountDueQuestions(ctx, profileID)`.
7. Return `Stats{Categories: ..., ReadinessPct: readiness, DueCount: int(dueCount)}`.

- [ ] **Step 1: Write the failing tests**

```go
func TestNextDueInterleavesCategories(t *testing.T) {
	q, svc, profileID, qids := seed(t)
	ctx := context.Background()

	// force qids[0..3] due now by recording an Again review (short interval,
	// but not necessarily <= now — instead, directly backdate due_at via a
	// second review call is unreliable for "due now" determinism, so record
	// a review then verify interleaving structurally: at least 2 distinct
	// categories appear among the first few results if the fixture's 4
	// categories are represented in the first 8 questions.
	for _, qid := range qids[:8] {
		if _, err := svc.RecordReview(ctx, profileID, qid, learning.Again); err != nil {
			t.Fatalf("record review: %v", err)
		}
	}
	// Again reviews schedule due_at at least 1 day out (interval floor), so
	// nothing is "due now" yet in a real clock — this test instead verifies
	// NextDue with a directly-manipulated past-due row via SQL to keep it
	// deterministic and independent of wall-clock timing assumptions.
	// (Left for the implementer: use q.UpsertQuestionMemory or a direct
	// pool.Exec to backdate due_at into the past for at least 2 categories'
	// worth of questions from qids[:8], then call svc.NextDue and assert
	// the returned slice's category sequence — fetched via
	// q.GetQuestionCategoryID per ID — does not have more than 2 consecutive
	// same-category entries when 2+ categories have due items.)
	t.Skip("scaffolding above documents intent; implementer fills in the deterministic due_at backdating and the interleave assertion — see plan Task 4 note")
}

func TestStatsReadinessWeightedByQuestionCount(t *testing.T) {
	q, svc, profileID, qids := seed(t)
	ctx := context.Background()

	stats, err := svc.Stats(ctx, profileID)
	if err != nil {
		t.Fatalf("Stats (fresh profile): %v", err)
	}
	if stats.ReadinessPct != 0 {
		t.Fatalf("fresh profile readiness = %d, want 0", stats.ReadinessPct)
	}
	if len(stats.Categories) != 4 {
		t.Fatalf("expected 4 categories (fixture), got %d", len(stats.Categories))
	}

	// answer all 10 questions in the "signs" category correctly (fixture:
	// 40 questions / 4 categories = 10 each, round-robin assignment)
	signsCatID, err := q.GetCategoryIDByCode(ctx, "signs")
	if err != nil {
		t.Fatalf("category lookup: %v", err)
	}
	answered := 0
	for _, qid := range qids {
		catID, err := q.GetQuestionCategoryID(ctx, qid)
		if err != nil {
			t.Fatalf("question category: %v", err)
		}
		if catID != signsCatID {
			continue
		}
		if _, err := svc.RecordReview(ctx, profileID, qid, learning.Good); err != nil {
			t.Fatalf("review: %v", err)
		}
		answered++
	}
	if answered == 0 {
		t.Fatal("fixture must contain at least one 'signs' question reachable from variant 1")
	}

	stats, err = svc.Stats(ctx, profileID)
	if err != nil {
		t.Fatalf("Stats (after reviews): %v", err)
	}
	if stats.ReadinessPct <= 0 {
		t.Fatalf("readiness should be > 0 after mastering part of one category, got %d", stats.ReadinessPct)
	}
	if stats.ReadinessPct >= 100 {
		t.Fatalf("readiness should be < 100 since only 1 of 4 categories was touched, got %d", stats.ReadinessPct)
	}
}

func TestStatsDueCount(t *testing.T) {
	_, svc, profileID, qids := seed(t)
	ctx := context.Background()
	stats, err := svc.Stats(ctx, profileID)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.DueCount != 0 {
		t.Fatalf("fresh profile due count = %d, want 0", stats.DueCount)
	}
	if _, err := svc.RecordReview(ctx, profileID, qids[0], learning.Again); err != nil {
		t.Fatalf("review: %v", err)
	}
	// an Again review schedules due_at >=1 day in the future (interval
	// floor), so it is NOT immediately due — this call only proves Stats
	// doesn't error after a review exists, not that DueCount increments;
	// DueCount's positive case is exercised via the same due_at-backdating
	// technique noted in TestNextDueInterleavesCategories.
}
```

> **Implementer note (both `TestNextDueInterleavesCategories` and `TestStatsDueCount`'s positive case):** the cleanest deterministic way to test "due now" behavior without depending on FSRS's real interval math (which always schedules >= 1 day out) is to directly manipulate `due_at` into the past after calling `RecordReview`, using the pool directly, e.g.:
> ```go
> _, err := pool.Exec(ctx, `UPDATE question_memory SET due_at = now() - interval '1 hour' WHERE profile_id = $1 AND question_id = $2`, profileID, qid)
> ```
> (`pool` is the `*pgxpool.Pool` from `testdb.New(t)`, already in scope in `seed`'s caller — thread it through or re-derive via `q`'s underlying connection if `seed` doesn't already expose it; adjust `seed`'s return signature to also return `pool` if needed.) Use this to build a deterministic "N questions across 2+ categories are due now" fixture, then assert `NextDue`'s returned category sequence (via `GetQuestionCategoryID` per returned ID) never has more than `ceil(N/numCategories)+1`-ish consecutive same-category entries — or more simply, assert that among the first `2*numCategoriesWithDueItems` results, every represented category appears at least once (proves round-robin, not block-grouping). Write the real assertions — do not leave `t.Skip` in the committed test.

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/learning/... -run "NextDue|Stats" -v`
Expected: FAIL — `NextDue`/`Stats` undefined.

- [ ] **Step 3: Implement** `NextDue` and `Stats` in `service.go`, `CategoryStat`/`Stats` in `dto.go`, per the Logic section above. Replace the test scaffolding's `t.Skip` with the real due_at-backdating + assertion approach from the implementer note.

- [ ] **Step 4: Run to verify it passes**

Run: `cd backend && go test ./internal/learning/... -v`
Expected: PASS (all tests, Tasks 1-4, no skips).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/learning/
git commit -m "feat(backend): learning.Service.NextDue interleaving and Stats readiness %"
```

---

### Task 5: HTTP handlers + server wiring

**Files:** create `internal/learning/handlers.go`, `internal/learning/handlers_test.go`; modify `internal/server/server.go`.

**Routes (all behind `auth.Required`):**
- `GET /learn/next?limit=` → `200 [question_id, question_id, ...]` (array of UUID strings — the client fetches full question detail via the existing `GET /questions/{id}` content endpoint, same "IDs only" pattern `internal/session` already established)
- `POST /learn/review {question_id, rating}` → `200 {stability, difficulty, due_at, reps, lapses}` (`rating`: integer 1-4)
- `GET /me/stats` → `200 {categories:[{category_code, mastery, seen, correct}], readiness_pct, due_count}`

**Error mapping:** `ErrInvalidRating` → 400 `invalid_rating`.

- [ ] **Step 1: Write the failing test** (mirrors `internal/session/handlers_test.go`'s pattern: real `testdb` + fixture seed, `chi.NewRouter()`, mount `learning.Handler` behind `auth.Required`, issue a JWT via `auth.IssueAccess`)

```go
package learning_test

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
	"avtotest.uz/backend/internal/fixture"
	"avtotest.uz/backend/internal/importer"
	"avtotest.uz/backend/internal/learning"
	"avtotest.uz/backend/internal/testdb"
)

const handlerSecret = "test-secret"

func setupHandlerServer(t *testing.T) (*httptest.Server, string, []string) {
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
		t.Fatalf("get variant: %v", err)
	}
	qids, err := q.ListVariantQuestionIDsOrdered(context.Background(), v.ID)
	if err != nil || len(qids) == 0 {
		t.Fatalf("question ids: %v", err)
	}
	tok, err := auth.IssueAccess([]byte(handlerSecret), profile.ID, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	svc := learning.NewService(q)
	r := chi.NewRouter()
	h := &learning.Handler{Svc: svc}
	h.Routes(r.With(auth.Required([]byte(handlerSecret))))

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	ids := make([]string, len(qids))
	for i, id := range qids {
		ids[i] = id.String()
	}
	return ts, tok, ids
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

func TestPostLearnReviewAndStats(t *testing.T) {
	ts, tok, qids := setupHandlerServer(t)

	body, _ := json.Marshal(map[string]any{"question_id": qids[0], "rating": 3})
	status, env := doReq(t, ts, http.MethodPost, "/learn/review", tok, body)
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, env.Data)
	}
	var out struct {
		Stability float64 `json:"stability"`
		Reps      int     `json:"reps"`
	}
	if err := json.Unmarshal(env.Data, &out); err != nil {
		t.Fatal(err)
	}
	if out.Reps != 1 || out.Stability <= 0 {
		t.Fatalf("unexpected review response: %+v", out)
	}

	status, env = doReq(t, ts, http.MethodPost, "/learn/review", tok, []byte(`{"question_id":"`+qids[0]+`","rating":99}`))
	if status != http.StatusBadRequest || env.Error == nil || env.Error.Code != "invalid_rating" {
		t.Fatalf("expected 400 invalid_rating, got status=%d env=%+v", status, env)
	}

	status, env = doReq(t, ts, http.MethodGet, "/me/stats", tok, nil)
	if status != http.StatusOK {
		t.Fatalf("stats status=%d", status)
	}
	var stats struct {
		ReadinessPct int `json:"readiness_pct"`
		DueCount     int `json:"due_count"`
		Categories   []struct {
			CategoryCode string  `json:"category_code"`
			Mastery      float64 `json:"mastery"`
		} `json:"categories"`
	}
	if err := json.Unmarshal(env.Data, &stats); err != nil {
		t.Fatal(err)
	}
	if len(stats.Categories) != 4 {
		t.Fatalf("expected 4 categories, got %d", len(stats.Categories))
	}
}

func TestLearnRoutesRequireAuth(t *testing.T) {
	ts, _, _ := setupHandlerServer(t)
	resp, err := ts.Client().Get(ts.URL + "/me/stats")
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

Run: `cd backend && go test ./internal/learning/... -run "LearnReview|RequireAuth" -v`
Expected: FAIL — `learning.Handler` undefined.

- [ ] **Step 3: Implement `handlers.go`** (mirror `internal/session/handlers.go`'s style exactly: `Handler{Svc *Service}`, `Routes(r chi.Router)`, `auth.FromContext` for `profileID`, a `writeLearningError` switch). Then modify `internal/server/server.go`: inside the existing `if deps.Pool != nil && deps.Redis != nil` block, after the `session.Handler` mounting, add:

```go
lh := &learning.Handler{Svc: learning.NewService(deps.Queries)}
lh.Routes(api.With(auth.Required([]byte(cfg.JWTSecret))))
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd backend && go build ./... && go test ./internal/learning/... ./internal/server/... -v`
Expected: PASS. (`internal/session` is still expected to fail to build until Task 6 — do not attempt to fix it here.)

- [ ] **Step 5: Commit**

```bash
git add backend/internal/learning/handlers.go backend/internal/learning/handlers_test.go backend/internal/server/server.go
git commit -m "feat(backend): learning http endpoints — learn/next, learn/review, me/stats"
```

---

### Task 6: Integrate FSRS into session answers; retire the Leitner placeholder

**Files:** modify `internal/session/service.go`, `internal/session/rules.go`, `internal/session/rules_test.go`; extend/adjust `internal/session/service_test.go` only if a test directly depended on the old `Mark*` behavior (search first — Plan 03's test suite has no functional test of a real `mistakes`-mode session's bank side effects, only `rules_test.go`'s direct `TestMistakeCleared` unit test of the now-removed pure function).

**Changes:**

1. **`internal/session/rules.go`**: remove the `MistakeClearAfter` constant and the `MistakeCleared` function — both are superseded by FSRS's own `due_at`-based scheduling (Global Constraints already explains why). Update the package doc comment's task-list reference if it mentions mistake-clearing.

2. **`internal/session/rules_test.go`**: remove `TestMistakeCleared` (the function it tested no longer exists).

3. **`internal/session/service.go`**: give `Service` a new field `Learning *learning.Service` (add to the struct and to `NewService`'s parameter list — update every existing caller: `internal/server/server.go`'s session-handler construction, and every test's `session.NewService(...)` call across `service_test.go`/`handlers_test.go`, which will need one more constructor argument). In `SubmitAnswer`, replace the whole block:
   ```go
   if row.Mode == "mistakes" {
       if ans.IsCorrect {
           if _, err := s.Q.MarkQuestionCorrectInMistakesMode(...); err != nil { ... }
       } else {
           if _, err := s.Q.MarkQuestionWrong(...); err != nil { ... }
       }
   } else if !ans.IsCorrect {
       if _, err := s.Q.MarkQuestionWrong(...); err != nil { ... }
   }
   ```
   with a single unconditional call that fires for **every** mode (not just mistakes/wrong-only — every answered question now feeds the real FSRS schedule):
   ```go
   rating := learning.Good
   if !ans.IsCorrect {
       rating = learning.Again
   }
   if _, err := s.Learning.RecordReview(ctx, profileID, questionID, rating); err != nil {
       return AnswerResult{}, err
   }
   ```
   Place this right after the `InsertSessionAnswer` call (same position in the flow the old block occupied), before the exam-mode 3rd-mistake check.

- [ ] **Step 1: Update `rules.go`/`rules_test.go`** — delete `MistakeClearAfter`/`MistakeCleared` and their test.

- [ ] **Step 2: Update `service.go`** — add the `Learning *learning.Service` field, update `NewService`, replace the mistake-bank block in `SubmitAnswer` as shown above. Import `avtotest.uz/backend/internal/learning`.

- [ ] **Step 3: Update every test call site** across `internal/session/service_test.go` and `internal/session/handlers_test.go` that calls `session.NewService(...)` — each needs a `learning.Service` instance passed in, e.g.:
   ```go
   learningSvc := learning.NewService(q)
   svc := session.NewService(sqlc.New(pool), pool, session.Limiter{...}, sender, secret, env, learningSvc)
   ```
   (adjust to match `NewService`'s actual current parameter order — read `service.go` first; insert the new parameter in whatever position keeps the signature readable, e.g. appended at the end, and update every call site consistently, including `internal/server/server.go`'s construction of the session service).

- [ ] **Step 4: Build and run the full session + learning suites**

Run: `cd backend && go build ./... && go test ./internal/session/... ./internal/learning/... ./internal/server/... -p 1 -count=1 -v`
Expected: PASS — this is the first point since Task 2 where `internal/session` compiles again. Confirm zero regressions in Plan 03's existing tests (unlock, exam scoring, resume redaction, ownership, etc. — none of that logic changed, only the mistake-bank internals).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/session/ backend/internal/server/server.go
git commit -m "refactor(backend): route session answers through FSRS RecordReview, retire Leitner placeholder"
```

---

### Task 7: Full verification + docs

- [ ] **Step 1:** `make check` (lint + `-p 1` full suite) green — this is the first point the ENTIRE repo (not just `internal/learning`/`internal/session`) is guaranteed consistent again; run it from the repo root.

- [ ] **Step 2: Live smoke** (PORT=8090, same established pattern as Plans 02/03): sign in via OTP sandbox flow to get a Bearer token, then:
  1. `POST /api/v1/sessions {"mode":"variant","variant_id":"<v1 id>","locale":"uz-Latn"}` → answer all 20 questions (mix of correct/incorrect, using the DB to know which answer is correct per question, same technique as Plan 03's smoke test) → `POST /api/v1/sessions/{id}/finish`.
  2. `GET /api/v1/me/stats` → confirm `readiness_pct > 0`, at least one category shows `mastery > 0`, `due_count` reflects the reviewed questions (likely `0` immediately after, since FSRS schedules >= 1 day out — note this in the report, it's expected).
  3. `POST /api/v1/learn/review {"question_id":"<any answered id>","rating":3}` → confirm a `{stability, difficulty, due_at, reps, lapses}` response with `reps >= 2` (it was already reviewed once via the session answer).
  4. `GET /api/v1/learn/next` → confirm it returns `[]` (nothing due yet, since everything was just scheduled >= 1 day out) — this is the correct, expected result; record it as such, not as a bug.
  5. `POST /api/v1/sessions {"mode":"mistakes","locale":"uz-Latn"}` → confirm this reflects the new `lapses > 0 AND due_at <= now()` definition — since nothing is due yet, expect an empty or near-empty bank, consistent with step 4's reasoning. Record actual output.
  Record all outputs.

- [ ] **Step 3: README** — add a "FSRS o'quv dvigateli" section to `README.md` (mirrors the "Auth"/"Sessiya" sections' style): the FSRS model in one paragraph (stability/difficulty/due-date, 90% target retention), the 3 endpoints with request/response shapes, the readiness-% formula, and an explicit note that `mistakes`-mode and `GET /learn/next` now both key off real `due_at` scheduling rather than a fixed "N consecutive corrects" rule. Commit:

```bash
git add README.md
git commit -m "docs: FSRS learning engine, mastery map, and stats endpoint reference"
```

## Self-Review

1. **Spec coverage:** §12 (FSRS xotira modeli, `GET /learn/next`, weak-area fokus, interleaving, kategoriya mastery-map, "imtihonga tayyorlik %") → Tasks 1-5; §7.3 schema (`question_memory`, `category_mastery` — already migrated, no new migration) → Tasks 2-3; §14 API list (`GET /learn/next`, `POST /learn/review`) → Task 5; `GET /me/stats` (mastery, tayyorlik %) → Tasks 4-5, with streak explicitly out of scope per Global Constraints (a later plan's concern, not a gap in this one).
2. **Placeholders:** the one explicit implementer-note pattern in Task 4 (due_at backdating for a deterministic "due now" test) is not a placeholder in the forbidden sense — it hands the implementer the exact SQL to run and the exact assertion shape, because a hardcoded example value would be wrong for a real-clock-dependent scheduling test. No other placeholders found.
3. **Type consistency:** `Rating`, `Card`, `Service`, `CategoryStat`, `Stats` names and fields are used identically across Tasks 1-5; `ErrInvalidRating` defined once (Task 3) and reused in Task 5's HTTP mapping; the `Service.Learning` field/`NewService` signature change in Task 6 is applied consistently to every caller (service, tests, server wiring) in one task, avoiding a partially-migrated state surviving past that task's commit.
