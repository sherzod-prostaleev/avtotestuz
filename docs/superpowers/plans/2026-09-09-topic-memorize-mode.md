# Topic Memorize Mode ("Yodlash") Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a "Yodlash" (memorize) button to every topic card in Practice → Category that opens a read-only walk through the whole topic, one question at a time, with the correct answer already marked — no answering required, Next/Previous only.

**Architecture:** One new VIP-gated backend endpoint (`GET /categories/{code}/memorize`) built entirely from three existing sqlc queries (no new SQL, no migration) plus the existing `content.Handler.LoadQuestionDetails` composition already used by session question reads. One new frontend route reuses the existing `QuestionStage` component in a disabled, always-correct-shown mode. The practice topic list gets one small button per card; the card's click target changes from `<button>` to a `<div role="button">` so it can host a second, independent button.

**Tech Stack:** Go 1.26 (chi, pgx/v5, sqlc 1.31.1), Next.js 15 App Router + TypeScript + Tailwind, next-intl, Vitest + Testing Library.

## Global Constraints

- Go toolchain is local but not on PATH: prefix every `go`/`golangci-lint` command with `PATH="/home/sher/.local/go/bin:$HOME/go/bin:$PATH"`.
- Backend tests need `TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable"` and Postgres running (`docker compose up -d postgres redis` if not already up).
- Frontend (`node`, `npm`) runs locally without any PATH tricks. Before any frontend gate, run `rm -rf frontend/.next` first — a stale `.next` produces false-red `tsc`/`vitest` failures unrelated to this change.
- Never add new sqlc queries or migrations for this feature — three existing queries (`CountValidQuestionsInCategory`, `OrderedQuestionIDsByCategory`, `ListCorrectAnswerIDsForQuestions`) already cover it. If a task below seems to need a new query, stop and re-read the spec — that's a sign of scope drift.
- Every commit message ends with the attribution lines from the session's system reminder (`Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>` + `Claude-Session: ...`). Copy them from a recent commit in `git log` if unsure.
- Spec: `docs/superpowers/specs/2026-09-09-topic-memorize-mode-design.md` — re-read it if a task here is ambiguous.

---

## File Structure

**Backend (new):**
- `backend/internal/session/memorize.go` — `Service.CategoryMemorize`, pure data logic, no HTTP/content concerns.
- `backend/internal/session/memorize_test.go` — service-level tests.
- `backend/internal/session/memorize_handlers_test.go` — HTTP-level tests.

**Backend (modified):**
- `backend/internal/session/handlers.go` — new route, new response type, new handler function.

**Frontend (new):**
- `frontend/src/hooks/use-memorize.ts` — fetch + map hook.
- `frontend/src/hooks/use-memorize.test.ts`
- `frontend/src/app/[locale]/(app)/practice/memorize/[code]/page.tsx` — the viewer.
- `frontend/src/app/[locale]/(app)/practice/memorize/[code]/page.test.tsx`
- `frontend/src/app/[locale]/(kiosk)/station/practice/memorize/[code]/page.tsx` — kiosk re-export, no dedicated test (matches the existing `station/practice/page.tsx` convention).

**Frontend (modified):**
- `frontend/src/hooks/use-session-engine.ts` — export `toQuestionItem`, `QuestionDetailResponse`, `toSessionError`, `SessionError` (already-existing private helpers, made reusable).
- `frontend/src/app/[locale]/(app)/practice/page.tsx` — card root becomes a `<div role="button">`, new nested "Yodlash" button, new `handleMemorizeClick`, new `GraduationCap` import.
- `frontend/src/app/[locale]/(app)/practice/page.test.tsx` — fix 3 existing `.closest("button")` queries, add 2 new tests.
- `frontend/messages/uz-Latn.json`, `frontend/messages/uz-Cyrl.json`, `frontend/messages/ru.json` — `Practice.memorizeButton` + new `Memorize` namespace (5 keys), identical key sets across all three (enforced by `frontend/tests/unit/i18n-keysets.test.ts`).

---

### Task 1: Backend — `Service.CategoryMemorize`

**Files:**
- Create: `backend/internal/session/memorize.go`
- Create: `backend/internal/session/memorize_test.go`

**Interfaces:**
- Produces: `type MemorizeItem struct { QuestionID uuid.UUID; Position int; CorrectAnswerID uuid.UUID }` and `func (s *Service) CategoryMemorize(ctx context.Context, categoryID uuid.UUID) ([]MemorizeItem, error)` — consumed by Task 2's handler.

- [ ] **Step 1: Write the failing tests**

Create `backend/internal/session/memorize_test.go`:

```go
package session_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/db/sqlc"
)

func TestCategoryMemorizeReturnsWholeTopicInSourceOrder(t *testing.T) {
	q, svc, _ := seed(t)
	ctx := context.Background()

	catID, err := q.GetCategoryIDByCode(ctx, "signs")
	if err != nil {
		t.Fatalf("category lookup: %v", err)
	}
	total, err := q.CountValidQuestionsInCategory(ctx, catID)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if total == 0 {
		t.Fatal("fixture category 'signs' has no questions to test against")
	}

	items, err := svc.CategoryMemorize(ctx, catID)
	if err != nil {
		t.Fatalf("CategoryMemorize: %v", err)
	}
	if len(items) != int(total) {
		t.Fatalf("len(items)=%d want %d", len(items), total)
	}

	wantIDs, err := q.OrderedQuestionIDsByCategory(ctx, sqlc.OrderedQuestionIDsByCategoryParams{
		CategoryID: catID, Skip: 0, LimitCount: total,
	})
	if err != nil {
		t.Fatalf("ordered ids: %v", err)
	}

	for i, item := range items {
		if item.QuestionID != wantIDs[i] {
			t.Fatalf("items[%d].QuestionID=%s want %s", i, item.QuestionID, wantIDs[i])
		}
		if item.Position != i+1 {
			t.Fatalf("items[%d].Position=%d want %d", i, item.Position, i+1)
		}
		wantCorrect, err := q.GetCorrectAnswerID(ctx, item.QuestionID)
		if err != nil {
			t.Fatalf("correct answer for %s: %v", item.QuestionID, err)
		}
		if item.CorrectAnswerID != wantCorrect {
			t.Fatalf("items[%d].CorrectAnswerID=%s want %s", i, item.CorrectAnswerID, wantCorrect)
		}
	}
}

func TestCategoryMemorizeEmptyCategoryReturnsEmptyNotError(t *testing.T) {
	_, svc, _ := seed(t)
	items, err := svc.CategoryMemorize(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("CategoryMemorize: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("len(items)=%d want 0 for a category with no questions", len(items))
	}
}
```

- [ ] **Step 2: Run tests, verify they fail to compile**

Run:
```bash
cd "/home/sher/Рабочий стол/avtotest/backend" && PATH="/home/sher/.local/go/bin:$PATH" \
  TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
  go test ./internal/session/... -run TestCategoryMemorize -v -count=1
```
Expected: FAIL — `svc.CategoryMemorize undefined (type *session.Service has no field or method CategoryMemorize)`.

- [ ] **Step 3: Implement `CategoryMemorize`**

Create `backend/internal/session/memorize.go`:

```go
package session

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/db/sqlc"
)

// MemorizeItem is one question's position and correct answer within a
// topic's memorize view. Handler.categoryMemorize composes this with
// content.Handler's question detail the same way listSessionQuestions
// composes SessionQuestionAccess with it (see decorateSessionQuestion).
type MemorizeItem struct {
	QuestionID      uuid.UUID
	Position        int
	CorrectAnswerID uuid.UUID
}

// CategoryMemorize returns every valid question in categoryID, in the
// topic's fixed source order (see OrderedQuestionIDsByCategory), each
// already carrying its correct answer.
//
// Unlike ordered practice (orderedCategoryDraw), this reads the whole topic
// in one call and touches no practice_cursor, session, or answer history:
// the same topic returns the same list from question 1, request after
// request. It exists for the "Yodlash" memorize view, which is deliberately
// not a session — nothing here is answered, scored, or FSRS-scheduled.
func (s *Service) CategoryMemorize(ctx context.Context, categoryID uuid.UUID) ([]MemorizeItem, error) {
	total, err := s.Q.CountValidQuestionsInCategory(ctx, categoryID)
	if err != nil {
		return nil, err
	}
	if total == 0 {
		return []MemorizeItem{}, nil
	}

	ids, err := s.Q.OrderedQuestionIDsByCategory(ctx, sqlc.OrderedQuestionIDsByCategoryParams{
		CategoryID: categoryID, Skip: 0, LimitCount: total,
	})
	if err != nil {
		return nil, err
	}

	rows, err := s.Q.ListCorrectAnswerIDsForQuestions(ctx, ids)
	if err != nil {
		return nil, err
	}
	correctByID := make(map[uuid.UUID]uuid.UUID, len(rows))
	for _, row := range rows {
		correctByID[row.QuestionID] = row.AnswerID
	}

	out := make([]MemorizeItem, 0, len(ids))
	for i, id := range ids {
		correctAnswerID, ok := correctByID[id]
		if !ok {
			return nil, fmt.Errorf("category %s question %s has no correct answer", categoryID, id)
		}
		out = append(out, MemorizeItem{QuestionID: id, Position: i + 1, CorrectAnswerID: correctAnswerID})
	}
	return out, nil
}
```

- [ ] **Step 4: Run tests, verify they pass**

Run:
```bash
cd "/home/sher/Рабочий стол/avtotest/backend" && PATH="/home/sher/.local/go/bin:$PATH" \
  TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
  go test ./internal/session/... -run TestCategoryMemorize -v -count=1
```
Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/session/memorize.go backend/internal/session/memorize_test.go
git commit -m "$(cat <<'EOF'
feat(session): add CategoryMemorize, the data behind topic Yodlash mode

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_012CFQf5HhH5ZZrXRLnKfNwm
EOF
)"
```

---

### Task 2: Backend — HTTP endpoint `GET /categories/{code}/memorize`

**Files:**
- Modify: `backend/internal/session/handlers.go`
- Create: `backend/internal/session/memorize_handlers_test.go`

**Interfaces:**
- Consumes: `Service.CategoryMemorize` (Task 1), `Service.ResolveCategoryID` (existing), `Service.Billing.Status` (existing), `Handler.Content.LoadQuestionDetails` (existing), `shuffleSessionAnswers` (existing), `writeSessionError` (existing), `claimsOrUnauthorized` (existing).
- Produces: JSON array of `memorizeQuestionResponse` at `GET /categories/{code}/memorize?locale=xx` — each item is a `content.QuestionDetailDTO` plus `position` (int), `answered` (always `true`), `correct_answer_id` (string).

- [ ] **Step 1: Write the failing tests**

Create `backend/internal/session/memorize_handlers_test.go`:

```go
package session_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestCategoryMemorizeRequiresAuth(t *testing.T) {
	ts, _, _ := setupServer(t)
	status, _ := doReq(t, ts, http.MethodGet, "/categories/signs/memorize?locale=uz-Latn", "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", status)
	}
}

func TestCategoryMemorizeRequiresVIP(t *testing.T) {
	ts, tok, _ := setupServer(t)
	status, env := doReq(t, ts, http.MethodGet, "/categories/signs/memorize?locale=uz-Latn", tok, nil)
	if status != http.StatusPaymentRequired || env.Error == nil || env.Error.Code != "vip_required" {
		t.Fatalf("status=%d env=%+v want 402 vip_required", status, env)
	}
}

func TestCategoryMemorizeOverHTTP(t *testing.T) {
	ts, tok, q := setupServer(t)
	profile, err := q.GetProfileByPhone(context.Background(), "+998901234567")
	if err != nil {
		t.Fatal(err)
	}
	grantVIP(t, q, profile.ID)

	catID, err := q.GetCategoryIDByCode(context.Background(), "signs")
	if err != nil {
		t.Fatal(err)
	}
	total, err := q.CountValidQuestionsInCategory(context.Background(), catID)
	if err != nil {
		t.Fatal(err)
	}

	status, env := doReq(t, ts, http.MethodGet, "/categories/signs/memorize?locale=uz-Latn", tok, nil)
	if status != http.StatusOK {
		t.Fatalf("status=%d env=%+v", status, env)
	}
	var items []struct {
		ID              string `json:"id"`
		Position        int    `json:"position"`
		Answered        bool   `json:"answered"`
		CorrectAnswerID string `json:"correct_answer_id"`
		Answers         []struct {
			ID string `json:"id"`
		} `json:"answers"`
	}
	if err := json.Unmarshal(env.Data, &items); err != nil {
		t.Fatalf("json: %v data=%s", err, env.Data)
	}
	if len(items) != int(total) {
		t.Fatalf("len(items)=%d want %d", len(items), total)
	}
	for i, item := range items {
		if item.Position != i+1 {
			t.Fatalf("items[%d].Position=%d want %d", i, item.Position, i+1)
		}
		if !item.Answered {
			t.Fatalf("items[%d].Answered=false want true", i)
		}
		if item.CorrectAnswerID == "" {
			t.Fatalf("items[%d] missing correct_answer_id", i)
		}
		found := false
		for _, a := range item.Answers {
			if a.ID == item.CorrectAnswerID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("items[%d].CorrectAnswerID %s not among its own answers", i, item.CorrectAnswerID)
		}
	}
}

func TestCategoryMemorizeBogusCategoryCode(t *testing.T) {
	ts, tok, q := setupServer(t)
	profile, err := q.GetProfileByPhone(context.Background(), "+998901234567")
	if err != nil {
		t.Fatal(err)
	}
	grantVIP(t, q, profile.ID)

	status, env := doReq(t, ts, http.MethodGet, "/categories/does-not-exist/memorize?locale=uz-Latn", tok, nil)
	if status != http.StatusNotFound || env.Error == nil || env.Error.Code != "not_found" {
		t.Fatalf("status=%d env=%+v want 404 not_found", status, env)
	}
}

func TestCategoryMemorizeInvalidLocale(t *testing.T) {
	ts, tok, q := setupServer(t)
	profile, err := q.GetProfileByPhone(context.Background(), "+998901234567")
	if err != nil {
		t.Fatal(err)
	}
	grantVIP(t, q, profile.ID)

	status, env := doReq(t, ts, http.MethodGet, "/categories/signs/memorize?locale=nope", tok, nil)
	if status != http.StatusBadRequest || env.Error == nil || env.Error.Code != "invalid_locale" {
		t.Fatalf("status=%d env=%+v want 400 invalid_locale", status, env)
	}
}
```

- [ ] **Step 2: Run tests, verify they fail**

Run:
```bash
cd "/home/sher/Рабочий стол/avtotest/backend" && PATH="/home/sher/.local/go/bin:$PATH" \
  TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
  go test ./internal/session/... -run TestCategoryMemorize -v -count=1
```
Expected: the four new HTTP tests FAIL with 404 (route not registered) — `TestCategoryMemorizeRequiresAuth` also fails because chi's default response for an unmatched route is a plain 404, not a 401, so it fails too (that is the point: no route exists yet).

- [ ] **Step 3: Add the route, response type, and handler**

In `backend/internal/session/handlers.go`, add the route inside `Routes` (after `r.Get("/sessions/{id}/questions/{questionID}", h.getSessionQuestion)`):

```go
	r.Get("/categories/{code}/memorize", h.categoryMemorize)
```

So the full `Routes` function reads:

```go
func (h *Handler) Routes(r chi.Router) {
	r.Post("/sessions", h.startSession)
	r.Post("/sessions/{id}/answers", h.submitAnswer)
	r.Post("/sessions/{id}/finish", h.finishSession)
	r.Get("/sessions/{id}", h.getSession)
	r.Get("/sessions/{id}/questions", h.listSessionQuestions)
	r.Get("/sessions/{id}/questions/{questionID}", h.getSessionQuestion)
	r.Get("/categories/{code}/memorize", h.categoryMemorize)
	r.Get("/me/practice-allowance", h.practiceAllowance)
	r.Get("/me/practice-progress", h.practiceProgress)
	r.Post("/me/practice-progress/reset", h.resetPracticeProgress)
	r.Get("/me/mock-eligibility", h.mockEligibility)
	r.Get("/me/sessions", h.listMySessions)
	r.Get("/me/variants", h.listVariantStatuses)
}
```

Then, right after the existing `needExplanations` function (which ends just before `type finishSessionResponse struct {`), insert:

```go
// memorizeShuffleSeed is a fixed namespace for shuffleSessionAnswers when
// building the memorize view, which has no real session id of its own. The
// shuffle only reorders options cosmetically (it never affects scoring), so
// any stable UUID works — it exists purely so a topic doesn't always show
// the correct answer in whatever position the source data happens to store
// it in.
var memorizeShuffleSeed = uuid.MustParse("00000000-0000-0000-0000-000000000001")

type memorizeQuestionResponse struct {
	content.QuestionDetailDTO
	Position        int    `json:"position"`
	Answered        bool   `json:"answered"`
	CorrectAnswerID string `json:"correct_answer_id"`
}

// categoryMemorize serves the whole topic at once, correct answers already
// disclosed — the one deliberate exception to every other session-scoped
// read in this file, which redacts correctness until answered or finished.
// VIP-gated exactly like "mistakes": Billing.Status already resolves a
// licensed classroom station as VIP too (see billing.StationVIPChecker), so
// no separate kiosk handling is needed here.
func (h *Handler) categoryMemorize(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsOrUnauthorized(w, r)
	if !ok {
		return
	}
	active, _, err := h.Svc.Billing.Status(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "entitlement check failed")
		return
	}
	if !active {
		httpx.Error(w, http.StatusPaymentRequired, "vip_required", "active entitlement required")
		return
	}
	loc, ok := i18n.Parse(r)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "invalid_locale", "locale must be one of uz-Latn, uz-Cyrl, ru, kaa")
		return
	}
	categoryID, err := h.Svc.ResolveCategoryID(r.Context(), chi.URLParam(r, "code"))
	if err != nil {
		writeSessionError(w, err)
		return
	}
	items, err := h.Svc.CategoryMemorize(r.Context(), categoryID)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	if h.Content == nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "question content is unavailable")
		return
	}

	ids := make([]uuid.UUID, len(items))
	for i, item := range items {
		ids[i] = item.QuestionID
	}
	details, fallback, err := h.Content.LoadQuestionDetails(r.Context(), ids, loc, true)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "question query failed")
		return
	}

	out := make([]memorizeQuestionResponse, 0, len(items))
	for _, item := range items {
		detail, ok := details[item.QuestionID]
		if !ok {
			httpx.Error(w, http.StatusInternalServerError, "internal", "question content missing")
			return
		}
		detail.Answers = shuffleSessionAnswers(detail.Answers, memorizeShuffleSeed, item.QuestionID)
		out = append(out, memorizeQuestionResponse{
			QuestionDetailDTO: detail,
			Position:          item.Position,
			Answered:          true,
			CorrectAnswerID:   item.CorrectAnswerID.String(),
		})
	}
	httpx.DataMeta(w, http.StatusOK, out, content.LocaleMeta{Locale: loc, Fallback: fallback})
}
```

- [ ] **Step 4: Run tests, verify they pass**

Run:
```bash
cd "/home/sher/Рабочий стол/avtotest/backend" && PATH="/home/sher/.local/go/bin:$PATH" \
  TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
  go test ./internal/session/... -count=1
```
Expected: PASS, entire `internal/session` package (this also re-runs every pre-existing session test — confirms nothing regressed).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/session/handlers.go backend/internal/session/memorize_handlers_test.go
git commit -m "$(cat <<'EOF'
feat(session): serve GET /categories/{code}/memorize for topic Yodlash mode

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_012CFQf5HhH5ZZrXRLnKfNwm
EOF
)"
```

---

### Task 3: Frontend — export reusable pieces from `use-session-engine.ts`, add `useMemorize`

**Files:**
- Modify: `frontend/src/hooks/use-session-engine.ts`
- Create: `frontend/src/hooks/use-memorize.ts`
- Create: `frontend/src/hooks/use-memorize.test.ts`

**Interfaces:**
- Consumes: `apiGet<T>` from `@/lib/api-client` (existing).
- Produces: `useMemorize(categoryCode: string, locale: string): { questions: SessionQuestionItem[]; loading: boolean; error: SessionError | null }` — consumed by Task 4's page.

- [ ] **Step 1: Export the four private helpers**

In `frontend/src/hooks/use-session-engine.ts`, change:

```ts
interface QuestionDetailResponse {
```
to:
```ts
export interface QuestionDetailResponse {
```

Change:
```ts
function toQuestionItem(
```
to:
```ts
export function toQuestionItem(
```

Change:
```ts
function toSessionError(err: unknown): SessionError {
```
to:
```ts
export function toSessionError(err: unknown): SessionError {
```

(`SessionError` and `SessionQuestionItem` are already `export interface` — no change needed there.)

- [ ] **Step 2: Run the existing hook test to confirm nothing broke**

Run:
```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npx vitest run src/hooks/use-session-engine.test.ts
```
Expected: PASS (exporting a previously-private function changes nothing about its behavior).

- [ ] **Step 3: Write the failing test for `useMemorize`**

Create `frontend/src/hooks/use-memorize.test.ts`:

```ts
import { renderHook, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import * as apiClient from "@/lib/api-client";
import { useMemorize } from "./use-memorize";

const sampleDetail = {
  id: "q-1",
  category_code: "signs",
  text: "Savol matni?",
  image_url: null,
  answers: [
    { id: "a-1", position: 1, text: "Variant A", image_url: null },
    { id: "a-2", position: 2, text: "Variant B", image_url: null },
  ],
  signs: [],
  explanation: null,
  position: 1,
  answered: true,
  correct_answer_id: "a-2",
};

describe("useMemorize", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("fetches the topic and maps each item into a session question item", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue([sampleDetail]);

    const { result } = renderHook(() => useMemorize("signs", "uz-Latn"));
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(apiClient.apiGet).toHaveBeenCalledWith("categories/signs/memorize?locale=uz-Latn");
    expect(result.current.error).toBeNull();
    expect(result.current.questions).toHaveLength(1);
    expect(result.current.questions[0].question).toBe("Savol matni?");
    expect(result.current.questions[0].correct_answer_id).toBe("a-2");
  });

  it("surfaces the server error code instead of the raw questions", async () => {
    vi.spyOn(apiClient, "apiGet").mockRejectedValue(
      new apiClient.ApiError("active entitlement required", "vip_required", 402)
    );

    const { result } = renderHook(() => useMemorize("signs", "uz-Latn"));
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.error).toEqual({
      code: "vip_required",
      message: "active entitlement required",
    });
    expect(result.current.questions).toEqual([]);
  });
});
```

- [ ] **Step 4: Run the test, verify it fails**

Run:
```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npx vitest run src/hooks/use-memorize.test.ts
```
Expected: FAIL — `Failed to resolve import "./use-memorize"`.

- [ ] **Step 5: Implement `useMemorize`**

Create `frontend/src/hooks/use-memorize.ts`:

```ts
"use client";

import { useCallback, useEffect, useState } from "react";
import { apiGet } from "@/lib/api-client";
import {
  toQuestionItem,
  toSessionError,
  type QuestionDetailResponse,
  type SessionError,
  type SessionQuestionItem,
} from "@/hooks/use-session-engine";

interface UseMemorizeResult {
  questions: SessionQuestionItem[];
  loading: boolean;
  error: SessionError | null;
}

/**
 * One VIP-gated topic, read-only and fully disclosed — see
 * GET /categories/{code}/memorize. This is not a session: nothing here is
 * answered, scored, or scheduled, so it shares only the one conversion
 * function with useSessionEngine, not the engine itself.
 */
export function useMemorize(categoryCode: string, locale: string): UseMemorizeResult {
  const [questions, setQuestions] = useState<SessionQuestionItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<SessionError | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const path = `categories/${encodeURIComponent(categoryCode)}/memorize?locale=${encodeURIComponent(locale)}`;
      const batch = await apiGet<QuestionDetailResponse[]>(path);
      setQuestions(batch.map((detail) => toQuestionItem(detail)));
    } catch (err) {
      setError(toSessionError(err));
      setQuestions([]);
    } finally {
      setLoading(false);
    }
  }, [categoryCode, locale]);

  useEffect(() => {
    void load();
  }, [load]);

  return { questions, loading, error };
}
```

- [ ] **Step 6: Run the test, verify it passes**

Run:
```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npx vitest run src/hooks/use-memorize.test.ts
```
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/hooks/use-session-engine.ts frontend/src/hooks/use-memorize.ts frontend/src/hooks/use-memorize.test.ts
git commit -m "$(cat <<'EOF'
feat(practice): add useMemorize, fetching a topic's Yodlash view

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_012CFQf5HhH5ZZrXRLnKfNwm
EOF
)"
```

---

### Task 4: Frontend — the memorize page + kiosk re-export

**Files:**
- Create: `frontend/src/app/[locale]/(app)/practice/memorize/[code]/page.tsx`
- Create: `frontend/src/app/[locale]/(app)/practice/memorize/[code]/page.test.tsx`
- Create: `frontend/src/app/[locale]/(kiosk)/station/practice/memorize/[code]/page.tsx`

**Interfaces:**
- Consumes: `useMemorize` (Task 3), `QuestionStage` + `AnswerState` (existing, `@/components/shared/question-stage`), `ExplanationDialog` (existing, `@/components/shared/explanation-dialog`), `resolveQuestionImageUrl` (existing, `@/lib/question-image`), translation keys `Memorize.*` (Task 6), `SessionStart.errorTitle`/`vipRequired`/`goToPremium`/`backToStation`/`backToPractice` (existing), `Session.exit`/`previous`/`next`/`networkError`/`genericError`/`zoomDialog`/`zoomedImageAlt`/`closeZoom` (existing), `Practice.memorizeButton` (Task 6).
- Produces: default export `MemorizePage({ kiosk?: boolean })`, consumed by the kiosk re-export and by Task 5's navigation target.

- [ ] **Step 1: Write the failing test**

Create `frontend/src/app/[locale]/(app)/practice/memorize/[code]/page.test.tsx`:

```tsx
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../../../../messages/uz-Latn.json";
import MemorizePage from "./page";
import { useMemorize } from "@/hooks/use-memorize";
import type { SessionQuestionItem } from "@/hooks/use-session-engine";

const navigation = vi.hoisted(() => ({ push: vi.fn() }));

vi.mock("next/navigation", () => ({
  useParams: () => ({ code: "signs" }),
  useRouter: () => ({ push: navigation.push }),
}));

vi.mock("@/hooks/use-memorize", () => ({ useMemorize: vi.fn() }));

function question(overrides: Partial<SessionQuestionItem> = {}): SessionQuestionItem {
  return {
    id: "q-1",
    question: "Qaysi belgi to'xtashni taqiqlaydi?",
    image_url: null,
    answers: [
      { id: "a-1", text: "3.27 belgisi" },
      { id: "a-2", text: "3.28 belgisi" },
    ],
    position: 1,
    answered: true,
    user_answer_id: null,
    correct_answer_id: "a-2",
    explanation: null,
    ...overrides,
  };
}

function renderPage(kiosk = false) {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <MemorizePage kiosk={kiosk} />
    </NextIntlClientProvider>
  );
}

const mockUseMemorize = vi.mocked(useMemorize);

describe("MemorizePage", () => {
  beforeEach(() => {
    navigation.push.mockReset();
    mockUseMemorize.mockReset();
  });

  it("shows a loading state while the topic is fetched", () => {
    mockUseMemorize.mockReturnValue({ questions: [], loading: true, error: null });
    renderPage();
    expect(screen.getByText(messages.Memorize.loading)).toBeInTheDocument();
  });

  it("marks the correct answer from the very first render, with no click needed", async () => {
    mockUseMemorize.mockReturnValue({ questions: [question()], loading: false, error: null });
    renderPage();

    const correctOption = (await screen.findByText("3.28 belgisi")).closest("button")!;
    expect(correctOption.querySelector('[data-testid="answer-correct-icon"]')).toBeTruthy();
  });

  it("advances with Keyingi and shows the finished screen after the last question", async () => {
    mockUseMemorize.mockReturnValue({
      questions: [question({ id: "q-1" }), question({ id: "q-2", correct_answer_id: "a-1" })],
      loading: false,
      error: null,
    });
    renderPage();

    expect(await screen.findByText("Savol 1 / 2")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Keyingisi/ }));
    expect(await screen.findByText("Savol 2 / 2")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Keyingisi/ }));
    expect(await screen.findByText(messages.Memorize.finishedTitle)).toBeInTheDocument();
  });

  it("sends a non-VIP user to premium on vip_required", () => {
    mockUseMemorize.mockReturnValue({
      questions: [],
      loading: false,
      error: { code: "vip_required", message: "active entitlement required" },
    });
    renderPage();

    fireEvent.click(screen.getByRole("button", { name: messages.SessionStart.goToPremium }));
    expect(navigation.push).toHaveBeenCalledWith("/uz-Latn/premium");
  });

  it("sends a kiosk vip_required user back to the station, never to premium", () => {
    mockUseMemorize.mockReturnValue({
      questions: [],
      loading: false,
      error: { code: "vip_required", message: "active entitlement required" },
    });
    renderPage(true);

    expect(
      screen.queryByRole("button", { name: messages.SessionStart.goToPremium })
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: messages.SessionStart.backToStation }));
    expect(navigation.push).toHaveBeenCalledWith("/uz-Latn/station");
  });
});
```

- [ ] **Step 2: Run the test, verify it fails**

Run:
```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npx vitest run "src/app/\[locale\]/(app)/practice/memorize/\[code\]/page.test.tsx"
```
Expected: FAIL — `Failed to resolve import "./page"`.

- [ ] **Step 3: Implement the page**

Create `frontend/src/app/[locale]/(app)/practice/memorize/[code]/page.tsx`:

```tsx
"use client";

import { useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { useParams, useRouter } from "next/navigation";
import { ChevronLeft, ChevronRight, LoaderCircle, X } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { useMemorize } from "@/hooks/use-memorize";
import { ExplanationDialog } from "@/components/shared/explanation-dialog";
import { QuestionStage } from "@/components/shared/question-stage";
import { resolveQuestionImageUrl } from "@/lib/question-image";

export interface MemorizePageProps {
  // Reused as-is under the login-free kiosk
  // (frontend/src/app/[locale]/(kiosk)/station/practice/memorize/[code]/page.tsx):
  // a licensed classroom station is treated as VIP by the same server check
  // a personal subscription uses (billing.StationVIPChecker), so this screen
  // behaves identically there — only the exit and vip_required destinations
  // differ (never a premium checkout link on a kiosk).
  kiosk?: boolean;
}

export default function MemorizePage({ kiosk = false }: MemorizePageProps = {}) {
  const params = useParams();
  const router = useRouter();
  const locale = useLocale();
  const t = useTranslations("Memorize");
  const sessionT = useTranslations("Session");
  const startT = useTranslations("SessionStart");
  const practiceT = useTranslations("Practice");
  const code = typeof params.code === "string" ? params.code : "";

  const { questions, loading, error } = useMemorize(code, locale);
  const [currentIndex, setCurrentIndex] = useState(0);
  const [zoomImageUrl, setZoomImageUrl] = useState<string | null>(null);
  const [explanationOpen, setExplanationOpen] = useState(false);

  const practiceHref = `/${locale}/${kiosk ? "station/practice" : "practice"}`;

  const goPrev = () => {
    setExplanationOpen(false);
    setCurrentIndex((i) => Math.max(0, i - 1));
  };
  const goNext = () => {
    setExplanationOpen(false);
    setCurrentIndex((i) => Math.min(questions.length, i + 1));
  };

  if (error) {
    let destination = practiceHref;
    let actionLabel = startT("backToPractice");
    let message = error.code === "network_error" ? sessionT("networkError") : sessionT("genericError");

    if (error.code === "vip_required") {
      if (kiosk) {
        destination = `/${locale}/station`;
        actionLabel = startT("backToStation");
      } else {
        destination = `/${locale}/premium`;
        actionLabel = startT("goToPremium");
      }
      message = startT("vipRequired");
    }

    return (
      <main className="page-shell-narrow flex min-h-[60vh] items-center justify-center">
        <Card className="w-full max-w-md border-destructive/40 bg-destructive/5 p-6 text-center">
          <p className="font-display text-lg font-bold text-destructive">{startT("errorTitle")}</p>
          <p className="mt-2 text-sm text-muted-foreground">{message}</p>
          <div className="sticky-cta-bar mt-5">
            <Button variant="game" className="w-full" onClick={() => router.push(destination)}>
              {actionLabel}
            </Button>
          </div>
        </Card>
      </main>
    );
  }

  if (loading) {
    return (
      <main className="page-shell-narrow flex min-h-[60vh] items-center justify-center">
        <Card className="flex items-center justify-center gap-2 p-8 text-muted-foreground">
          <LoaderCircle className="h-5 w-5 animate-spin text-accent" aria-hidden="true" />
          <span className="text-sm">{t("loading")}</span>
        </Card>
      </main>
    );
  }

  if (questions.length === 0) {
    return (
      <main className="page-shell-narrow flex min-h-[60vh] items-center justify-center">
        <Card className="w-full max-w-md p-6 text-center">
          <p className="font-display text-lg font-bold">{t("emptyTitle")}</p>
          <p className="mt-2 text-sm text-muted-foreground">{t("emptyBody")}</p>
          <div className="sticky-cta-bar mt-5">
            <Button variant="game" className="w-full" onClick={() => router.push(practiceHref)}>
              {startT("backToPractice")}
            </Button>
          </div>
        </Card>
      </main>
    );
  }

  const isFinished = currentIndex >= questions.length;

  if (isFinished) {
    return (
      <main className="page-shell-narrow flex min-h-[60vh] items-center justify-center">
        <Card className="w-full max-w-md p-6 text-center">
          <p className="font-display text-lg font-bold">{t("finishedTitle")}</p>
          <p className="mt-2 text-sm text-muted-foreground">{t("finishedBody")}</p>
          <div className="sticky-cta-bar mt-5">
            <Button variant="game" className="w-full" onClick={() => router.push(practiceHref)}>
              {startT("backToPractice")}
            </Button>
          </div>
        </Card>
      </main>
    );
  }

  const currentQuestion = questions[currentIndex];

  return (
    <main className="page-enter-fade session-shell flex flex-col gap-1 overflow-hidden bg-background px-2 pb-[max(0.35rem,env(safe-area-inset-bottom))] pt-[max(0.35rem,env(safe-area-inset-top))] sm:gap-3 sm:px-4 sm:py-3">
      <header className="session-header flex shrink-0 items-center justify-between gap-1.5 rounded-xl border border-border bg-card px-2 py-1.5 sm:gap-3 sm:rounded-2xl sm:p-3">
        <Button
          variant="outline"
          size="sm"
          className="h-9 min-h-9 gap-1 rounded-lg border-border px-2.5 text-xs font-extrabold transition-transform active:scale-95 sm:h-11 sm:min-h-11 sm:rounded-xl sm:px-4 sm:text-sm"
          aria-label={sessionT("exit")}
          onClick={() => router.push(practiceHref)}
        >
          <ChevronLeft className="h-4 w-4 shrink-0" aria-hidden="true" />
          <span>{sessionT("exit")}</span>
        </Button>
        <span className="truncate rounded-lg border border-accent/30 bg-accent/10 px-2 py-1 text-[11px] font-bold text-accent sm:px-3 sm:py-1.5 sm:text-xs">
          {practiceT("memorizeButton")}
        </span>
      </header>

      <Card className="session-content-card flex min-h-0 flex-1 flex-col gap-1 overflow-hidden p-1.5 sm:gap-3 sm:p-5">
        <div className="min-h-0 flex-1 overflow-hidden">
          <QuestionStage
            question={currentQuestion}
            questionNumber={currentIndex + 1}
            totalQuestions={questions.length}
            answered={true}
            disabled={true}
            onSelectAnswer={() => {}}
            answerStateFor={(answerId) =>
              currentQuestion.correct_answer_id === answerId ? "correct" : "neutral"
            }
            onZoomImage={() => setZoomImageUrl(resolveQuestionImageUrl(currentQuestion.image_url))}
            onOpenExplanation={() => setExplanationOpen(true)}
          />
        </div>
      </Card>

      <footer className="session-actions flex shrink-0 items-center justify-between gap-2 rounded-xl border border-border bg-card p-2 sm:rounded-2xl sm:p-2.5 shadow-raised-sm">
        <Button
          variant="outline"
          className="h-9 min-h-9 px-3 sm:h-11 sm:min-h-11 sm:px-5"
          disabled={currentIndex === 0}
          onClick={goPrev}
        >
          <ChevronLeft className="mr-1 h-4 w-4" aria-hidden="true" />
          <span className="hidden xs:inline sm:inline">{sessionT("previous")}</span>
        </Button>

        <Button
          variant="game"
          className="h-9 min-h-9 px-4 sm:h-11 sm:min-h-11 sm:px-6"
          onClick={goNext}
        >
          <span>{sessionT("next")}</span>
          <ChevronRight className="ml-1 h-4 w-4" aria-hidden="true" />
        </Button>
      </footer>

      <ExplanationDialog
        open={explanationOpen}
        onClose={() => setExplanationOpen(false)}
        questionNumber={currentIndex + 1}
        questionText={currentQuestion.question}
        imageUrl={resolveQuestionImageUrl(currentQuestion.image_url)}
        explanation={currentQuestion.explanation ?? null}
      />

      <AnimatePresence>
        {zoomImageUrl && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.15 }}
            role="dialog"
            aria-modal="true"
            aria-label={sessionT("zoomDialog")}
            onMouseDown={(event) => {
              if (event.target === event.currentTarget) setZoomImageUrl(null);
            }}
            className="fixed inset-0 z-50 flex items-end justify-center bg-black/85 p-0 backdrop-blur-sm sm:items-center sm:p-4"
          >
            <motion.div
              initial={{ opacity: 0, scale: 0.92 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.92 }}
              transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1] }}
              className="session-zoom-panel relative w-full max-w-5xl rounded-t-3xl bg-card p-3 sm:rounded-2xl sm:bg-transparent sm:p-0"
            >
              {/* Dynamic media URL is served by the backend. */}
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src={zoomImageUrl}
                alt={sessionT("zoomedImageAlt")}
                className="session-zoom-image w-full rounded-2xl object-contain"
              />
              <button
                type="button"
                onClick={() => setZoomImageUrl(null)}
                aria-label={sessionT("closeZoom")}
                className="absolute right-3 top-3 flex h-11 w-11 items-center justify-center rounded-full border border-border bg-card text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring sm:right-2 sm:top-2 sm:border-0 sm:bg-foreground/90 sm:text-background"
              >
                <X className="h-5 w-5" aria-hidden="true" />
              </button>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </main>
  );
}
```

- [ ] **Step 4: Run the test, verify it passes**

Run:
```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npx vitest run "src/app/\[locale\]/(app)/practice/memorize/\[code\]/page.test.tsx"
```
Expected: PASS (all 5 tests). If the `Memorize` translation keys don't exist yet (Task 6 not done), this will fail with `undefined` message text — do Task 6 before this step if working out of order, or accept a temporary red here if executing tasks strictly in order and fixing forward.

- [ ] **Step 5: Add the kiosk re-export**

Create `frontend/src/app/[locale]/(kiosk)/station/practice/memorize/[code]/page.tsx`:

```tsx
// Kiosk memorize entry point: /[locale]/station/practice/memorize/[code].
//
// Reuses the learner app's memorize page in kiosk mode: exit and the
// vip_required fallback push to /station/... instead of the login-gated
// learner routes. See MemorizePageProps in the imported module, and
// billing.StationVIPChecker for why a licensed station's Billing.Status
// already comes back active without any kiosk-specific code on the server.
import MemorizePage from "@/app/[locale]/(app)/practice/memorize/[code]/page";

export default function KioskMemorizePage() {
  return <MemorizePage kiosk />;
}
```

- [ ] **Step 6: Commit**

```bash
git add "frontend/src/app/[locale]/(app)/practice/memorize" "frontend/src/app/[locale]/(kiosk)/station/practice/memorize"
git commit -m "$(cat <<'EOF'
feat(practice): add the Yodlash memorize viewer page, learner + kiosk

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_012CFQf5HhH5ZZrXRLnKfNwm
EOF
)"
```

---

### Task 5: Frontend — "Yodlash" button on every topic card

**Files:**
- Modify: `frontend/src/app/[locale]/(app)/practice/page.tsx`
- Modify: `frontend/src/app/[locale]/(app)/practice/page.test.tsx`

**Interfaces:**
- Consumes: nothing new (uses `router.push`, already in scope).
- Produces: nothing new (this is a leaf UI change) — but the URL it navigates to, `/${locale}/practice/memorize/${code}` (or `/station/practice/memorize/${code}` on kiosk), must match Task 4's route exactly.

- [ ] **Step 1: Fix the 3 existing tests that assume the card is a `<button>`**

In `frontend/src/app/[locale]/(app)/practice/page.test.tsx`, the card root is about to change from `<button>` to `<div role="button">` (Step 3 below). Update the three places that query it:

Line ~97:
```ts
    fireEvent.click(screen.getByText("Umumiy qoidalar").closest("button")!);
```
becomes:
```ts
    fireEvent.click(screen.getByText("Umumiy qoidalar").closest('[role="button"]')!);
```

Line ~133:
```ts
    const card = screen.getByText("Umumiy qoidalar").closest("button")!;
```
becomes:
```ts
    const card = screen.getByText("Umumiy qoidalar").closest('[role="button"]')!;
```

Line ~259 (inside `describe("PracticePage kiosk mode", ...)`):
```ts
    fireEvent.click(screen.getByText("Umumiy qoidalar").closest("button")!);
```
becomes:
```ts
    fireEvent.click(screen.getByText("Umumiy qoidalar").closest('[role="button"]')!);
```

- [ ] **Step 2: Add the two new tests**

Add this test right after the "sorts categories by sort_order..." test (still inside `describe("PracticePage", ...)`):

```ts
  it("opens memorize mode from a topic card without also starting a practice session", async () => {
    mockEndpoints();
    renderWithIntl();
    await screen.findByText("Umumiy qoidalar");

    const card = screen.getByText("Umumiy qoidalar").closest('[role="button"]')!;
    fireEvent.click(within(card).getByRole("button", { name: messages.Practice.memorizeButton }));

    expect(pushMock).toHaveBeenCalledTimes(1);
    expect(pushMock).toHaveBeenCalledWith("/uz-Latn/practice/memorize/general_rules");
  });
```

Add this test inside `describe("PracticePage kiosk mode", ...)`, right after the "starts a category practice session on a kiosk-reachable session/start" test:

```ts
  it("opens memorize mode on a kiosk-reachable route", async () => {
    mockEndpoints();
    renderKiosk();
    await screen.findByText("Umumiy qoidalar");

    const card = screen.getByText("Umumiy qoidalar").closest('[role="button"]')!;
    fireEvent.click(within(card).getByRole("button", { name: messages.Practice.memorizeButton }));

    expect(pushMock).toHaveBeenCalledWith("/uz-Latn/station/practice/memorize/general_rules");
    expect(isKioskReachable(pushMock.mock.calls[0][0])).toBe(true);
  });
```

- [ ] **Step 3: Run the tests, verify they fail**

Run:
```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npx vitest run "src/app/\[locale\]/(app)/practice/page.test.tsx"
```
Expected: FAIL — the two new tests fail (`Unable to find an accessible element with the role "button" and name ...`), and possibly the 3 fixed-query tests too if the card is still a `<button>` (they'd now fail because `.closest('[role="button"]')` finds nothing — confirming the fix is needed together with Step 4, not before it).

- [ ] **Step 4: Add the `GraduationCap` import**

In `frontend/src/app/[locale]/(app)/practice/page.tsx`, add `GraduationCap` to the lucide-react import list:

```ts
import {
  AlignLeft,
  BookOpen,
  CalendarClock,
  CheckCircle2,
  Crown,
  GraduationCap,
  Image as ImageIcon,
  Layers,
  Play,
  RefreshCw,
  Signpost,
  BrainCircuit,
  TriangleAlert,
  Route,
  Siren,
  Car,
  Navigation,
  MapPin,
  Truck,
  HeartPulse,
} from "lucide-react";
```

- [ ] **Step 5: Add `handleMemorizeClick`**

Right after `handleCategoryClick` (defined at line ~318), add:

```ts
  const handleMemorizeClick = (catCode: string) => {
    const base = kiosk ? `/${locale}/station/practice` : `/${locale}/practice`;
    router.push(`${base}/memorize/${encodeURIComponent(catCode)}`);
  };
```

- [ ] **Step 6: Turn the card into a `<div role="button">` and add the nested button**

Replace the card's opening tag:
```tsx
                    <button
                      key={cat.code}
                      type="button"
                      onClick={() => handleCategoryClick(cat.code)}
                      className={`surface-raised-sm surface-interactive flex flex-col justify-between gap-2 rounded-2xl border border-border bg-background p-3 text-left hover:border-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                        kiosk ? "min-h-[8.25rem] p-4" : "min-h-[7rem]"
                      }`}
                    >
```
with:
```tsx
                    <div
                      key={cat.code}
                      role="button"
                      tabIndex={0}
                      onClick={() => handleCategoryClick(cat.code)}
                      onKeyDown={(event) => {
                        if (event.key === "Enter" || event.key === " ") {
                          event.preventDefault();
                          handleCategoryClick(cat.code);
                        }
                      }}
                      className={`surface-raised-sm surface-interactive flex cursor-pointer flex-col justify-between gap-2 rounded-2xl border border-border bg-background p-3 text-left hover:border-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                        kiosk ? "min-h-[8.25rem] p-4" : "min-h-[7rem]"
                      }`}
                    >
```

Replace the card's closing tag (right before the `);` / `})}` that ends the `.map(...)` callback):
```tsx
                    </button>
                  );
                })}
```
with:
```tsx
                      <button
                        type="button"
                        onClick={(event) => {
                          event.stopPropagation();
                          handleMemorizeClick(cat.code);
                        }}
                        className="mt-1 flex w-full items-center justify-center gap-1.5 rounded-lg border border-gold/30 bg-gold/10 px-2 py-1.5 text-[11px] font-bold text-gold transition-colors hover:bg-gold/20 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                      >
                        <GraduationCap aria-hidden="true" className="h-3.5 w-3.5" />
                        {t("memorizeButton")}
                      </button>
                    </div>
                  );
                })}
```

- [ ] **Step 7: Run the tests, verify they pass**

Run:
```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npx vitest run "src/app/\[locale\]/(app)/practice/page.test.tsx"
```
Expected: PASS (all tests in the file, old and new). If it fails on missing `messages.Practice.memorizeButton`, do Task 6 first.

- [ ] **Step 8: Commit**

```bash
git add "frontend/src/app/[locale]/(app)/practice/page.tsx" "frontend/src/app/[locale]/(app)/practice/page.test.tsx"
git commit -m "$(cat <<'EOF'
feat(practice): add a Yodlash button to every topic card

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_012CFQf5HhH5ZZrXRLnKfNwm
EOF
)"
```

---

### Task 6: Translations — `Practice.memorizeButton` + new `Memorize` namespace

**Files:**
- Modify: `frontend/messages/uz-Latn.json`
- Modify: `frontend/messages/uz-Cyrl.json`
- Modify: `frontend/messages/ru.json`

**Interfaces:**
- Produces: `Practice.memorizeButton`, `Memorize.loading`, `Memorize.emptyTitle`, `Memorize.emptyBody`, `Memorize.finishedTitle`, `Memorize.finishedBody` — consumed by Task 4 and Task 5. Key set must be byte-identical (names, not values) across all three files, enforced by `frontend/tests/unit/i18n-keysets.test.ts`.

- [ ] **Step 1: `uz-Latn.json`**

Find this line inside the `"Practice"` block:
```json
    "titleShort": "Mavzular"
  },
```
Replace with:
```json
    "titleShort": "Mavzular",
    "memorizeButton": "Yodlash"
  },
```

Find this line (end of the `"SessionStart"` block):
```json
    "backToStation": "Sinfxonaga qaytish"
  },
  "Saved": {
```
Replace with:
```json
    "backToStation": "Sinfxonaga qaytish"
  },
  "Memorize": {
    "loading": "Yuklanmoqda...",
    "emptyTitle": "Bu mavzuda hali savol yo'q",
    "emptyBody": "Iltimos, boshqa mavzuni tanlang.",
    "finishedTitle": "Mavzu tugadi!",
    "finishedBody": "Siz ushbu mavzudagi barcha savollarni ko'rib chiqdingiz."
  },
  "Saved": {
```

- [ ] **Step 2: `uz-Cyrl.json`**

Find the analogous end of the `"Practice"` block (the Cyrillic translation of `"titleShort": "Mavzular"`) and add `"memorizeButton": "Ёдлаш"` the same way as Step 1.

Find the analogous end of the `"SessionStart"` block (the Cyrillic translation of `"backToStation": "Sinfxonaga qaytish"`, immediately before `"Saved": {`) and insert:
```json
  "Memorize": {
    "loading": "Юкланмоқда...",
    "emptyTitle": "Бу мавзуда ҳали савол йўқ",
    "emptyBody": "Илтимос, бошқа мавзуни танланг.",
    "finishedTitle": "Мавзу тугади!",
    "finishedBody": "Сиз ушбу мавзудаги барча саволларни кўриб чиқдингиз."
  },
```

- [ ] **Step 3: `ru.json`**

Find the analogous end of the `"Practice"` block and add `"memorizeButton": "Запомнить"` the same way.

Find the analogous end of the `"SessionStart"` block (immediately before `"Saved": {`) and insert:
```json
  "Memorize": {
    "loading": "Загрузка...",
    "emptyTitle": "В этой теме пока нет вопросов",
    "emptyBody": "Пожалуйста, выберите другую тему.",
    "finishedTitle": "Тема пройдена!",
    "finishedBody": "Вы просмотрели все вопросы этой темы."
  },
```

- [ ] **Step 4: Validate JSON and key parity**

Run:
```bash
cd "/home/sher/Рабочий стол/avtotest" && python3 -c "
import json
for f in ['uz-Latn', 'uz-Cyrl', 'ru']:
    json.load(open(f'frontend/messages/{f}.json'))
print('all three parse OK')
"
```
Expected: `all three parse OK` (catches a stray comma/bracket before running the slower JS test suite).

Then run:
```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npx vitest run tests/unit/i18n-keysets.test.ts
```
Expected: PASS — proves the three files carry the exact same key set.

- [ ] **Step 5: Commit**

```bash
git add frontend/messages/uz-Latn.json frontend/messages/uz-Cyrl.json frontend/messages/ru.json
git commit -m "$(cat <<'EOF'
i18n(practice): add Yodlash button and memorize-view copy, all 3 locales

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_012CFQf5HhH5ZZrXRLnKfNwm
EOF
)"
```

---

### Task 7: Full-suite verification

No new code in this task — it exists to catch anything the per-task runs above missed (cross-package regressions, lint, typecheck, a full build).

- [ ] **Step 1: Full backend test suite**

Run in the background (takes ~20-25 minutes):
```bash
cd "/home/sher/Рабочий стол/avtotest/backend" && PATH="/home/sher/.local/go/bin:$PATH" \
  TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
  go test -p 1 ./... -count=1
```
Expected: every package `ok`, none `FAIL`.

- [ ] **Step 2: Backend lint**

Run:
```bash
cd "/home/sher/Рабочий стол/avtotest/backend" && PATH="/home/sher/.local/go/bin:$HOME/go/bin:$PATH" golangci-lint run
```
Expected: `0 issues`.

- [ ] **Step 3: Confirm no sqlc drift**

Run:
```bash
cd "/home/sher/Рабочий стол/avtotest" && git status --porcelain backend/internal/db/queries backend/internal/db/sqlc
```
Expected: empty output — this feature reused existing queries and must not have touched generated code. If this shows anything, something added a query outside this plan; investigate before proceeding.

- [ ] **Step 4: Frontend typecheck, lint, unit tests**

Run:
```bash
cd "/home/sher/Рабочий стол/avtotest" && rm -rf frontend/.next
cd frontend && npx tsc --noEmit
```
Expected: no errors.

Run:
```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npx eslint .
```
Expected: no errors.

Run:
```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npx vitest run
```
Expected: every test passes (this re-runs the whole suite, catching anything the per-task targeted runs missed — e.g. the middleware route-discovery test, the i18n parity test, and every existing practice/session test).

- [ ] **Step 5: Frontend production build**

Run:
```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npm run build
```
Expected: build succeeds. Afterward:
```bash
cd "/home/sher/Рабочий стол/avtotest" && git checkout -- frontend/next-env.d.ts
```
(the build touches this file harmlessly; discard the diff per existing project convention.)

- [ ] **Step 6: Fix forward**

If any of Steps 1-5 fail, fix the failure in the file it points to (not by weakening a test or adding `--no-verify`/`eslint-disable` blocks), re-run that one step, then re-run the full step once more to confirm. Do not commit broken code between fixes — amend the relevant task's commit is not allowed either (never amend per the global git rule); make a small follow-up commit instead, e.g. `fix(practice): ...`.

---

### Task 8: Manual browser verification

UI changes are not done until they've been exercised in a real browser against a real backend — typecheck and unit tests verify code correctness, not feature correctness.

- [ ] **Step 1: Bring up the stack**

```bash
cd "/home/sher/Рабочий стол/avtotest" && docker compose up -d --wait
```

- [ ] **Step 2: Start the backend**

```bash
cd "/home/sher/Рабочий стол/avtotest/backend" && PATH="/home/sher/.local/go/bin:$PATH" go run ./cmd/api
```
Run this with `run_in_background: true` — it's a long-lived server, not a one-shot command.

- [ ] **Step 3: Seed data if the dev DB is empty**

```bash
cd "/home/sher/Рабочий стол/avtotest" && make seed
```
Skip if `make seed` was already run in this environment before (check `docker compose exec postgres psql -U avtotest -d avtotest -c "select count(*) from question;"` — a non-zero count means seeding already happened).

- [ ] **Step 4: Start the frontend dev server**

```bash
cd "/home/sher/Рабочий стол/avtotest/frontend" && npm run dev
```
Run with `run_in_background: true`.

- [ ] **Step 5: Grant VIP to the test account**

Use whatever the project's existing dev login is (check `backend/.env.example` / `docs/superpowers/plans/*.md` for a seeded test phone number), or:
```bash
cd "/home/sher/Рабочий стол/avtotest/backend" && PATH="/home/sher/.local/go/bin:$PATH" go run ./cmd/seedadmin
```
only grants an admin account — for a learner VIP grant, use the admin UI (`/uz-Latn/admin`) to add entitlement days to the test profile, or call `billing.Service.GrantDays` via a short one-off script if no admin UI path is faster. The goal is one learner account with active VIP.

- [ ] **Step 6: Walk through the feature in Chrome**

Using the claude-in-chrome tools:
1. Navigate to `http://localhost:3000/uz-Latn/login`, sign in as the VIP test account.
2. Navigate to `http://localhost:3000/uz-Latn/practice`.
3. Confirm every topic card shows the new "Yodlash" button.
4. Click a card's "Yodlash" button (not the card itself) — confirm it does NOT start a normal practice session (URL should be `/uz-Latn/practice/memorize/<code>`, not `/uz-Latn/session/<id>`).
5. On the memorize screen, confirm the first question's correct answer is already shown highlighted green with a checkmark, before clicking anything.
6. Click "Keyingisi" a few times, confirm each new question also shows its correct answer pre-marked, and "Oldingisi" navigates back correctly.
7. Click through to the last question and one more "Keyingisi" — confirm the "Mavzu tugadi!" screen appears with a working "Mavzularga qaytish" button.
8. Sign in as (or switch to) a non-VIP account, click "Yodlash" on any card, confirm it redirects to `/uz-Latn/premium` with the VIP-required message (not a raw error page).
9. Take a screenshot of the memorize screen mid-topic (correct answer visibly marked) for the final report.

- [ ] **Step 7: Stop the background servers**

Once verified, stop the `go run ./cmd/api` and `npm run dev` background processes (or leave them running only if the user is continuing to use them — ask if unsure).

---

## Self-Review Notes

- **Spec coverage:** every section of `docs/superpowers/specs/2026-09-09-topic-memorize-mode-design.md` maps to a task above — backend endpoint (Tasks 1-2), frontend viewer (Tasks 3-4), topic card button (Task 5), translations (Task 6), kiosk parity (built into Tasks 4-5, verified in Task 8), out-of-scope items (no session/cursor/scoring changes) are structurally impossible here since Task 1's `CategoryMemorize` never touches `practice_cursor`, `exam_session`, or `session_answer`.
- **Placeholder scan:** no TBD/TODO; every step has complete, real code.
- **Type consistency:** `MemorizeItem` (Task 1) → consumed only inside `handlers.go` (Task 2), never crosses to the frontend. Frontend-facing shape is `memorizeQuestionResponse` (Task 2) → `QuestionDetailResponse` (existing, frontend) → `SessionQuestionItem` (existing) via `toQuestionItem` (Task 3) → consumed by `useMemorize` (Task 3) → `MemorizePage` (Task 4). Names match at every hop.
