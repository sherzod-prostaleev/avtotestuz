# Yodlash: memorize mode for topic tests

2026-09-09

## The problem

Practice → Category ("Mavzulashtirilgan testlar") lets a learner test
themselves on one topic, but there is no way to just *review* a topic — read
every question with the correct answer already visible, to memorize the
material before testing. Today the only way to see a correct answer is to
answer the question first (or finish the session).

## What this adds

A small **"Yodlash"** button on every topic card in Practice → Category. It
opens a read-only walk through *every* question in that topic, in source
order, one at a time, with the correct answer already marked. No answering
required — just Next / Previous.

This is a new, separate flow. It does not touch:
- the "Hammasi" ordered-practice cursor (`practice_cursor` /
  `orderedCategoryDraw`, see the ordered-category-practice design) — Yodlash
  always shows the whole topic from question 1, every time;
- scoring, the mistake bank, or FSRS scheduling — nothing is "answered";
- the daily practice allowance — this is not a practice draw.

## Access: VIP (personal or station-licensed)

Yodlash requires an active entitlement — same rule as the existing
"mistakes" (xatolar) mode: `billing.Service.Status(ctx, profileID)`.

That check already resolves station-licensed kiosk profiles as VIP
(`StationVIPChecker.ActiveStationVIP`, via `stationctx` set by
`auth.Required` when the request carries a station JWT) with no extra code —
so a driving-school kiosk with a live B2B license gets Yodlash for free,
exactly like a personal VIP subscriber. A kiosk whose station license has
lapsed gets the same `vip_required` rejection a personal account without VIP
would.

The button therefore shows on **both** the learner app and the kiosk
(`/practice` and `/station/practice`) — unlike the VIP-purchase banner
elsewhere on this page, which is hidden on kiosk because a walk-up student
cannot buy VIP. Yodlash isn't a purchase entry point; it either works
(licensed station or personal VIP) or shows a "not available" message with no
checkout link.

## Backend

### New endpoint

`GET /api/v1/categories/{code}/memorize?locale=xx`

Mounted in `session.Handler.Routes` (the `learnerAuth`-gated group), not
`content.Handler` — `content` documents that it never exposes correctness,
and this endpoint's whole purpose is to expose it, deliberately, to an
authenticated + entitled caller.

Handler:
1. `claimsOrUnauthorized` (existing helper).
2. `Billing.Status(ctx, claims.ProfileID)` → if not active, `402
   vip_required` (same error code and shape the frontend's `vip_required`
   handling already understands from the "mistakes" flow).
3. `i18n.Parse(r)` for locale (existing helper).
4. `h.Svc.ResolveCategoryID(ctx, chi.URLParam(r, "code"))` — existing helper,
   accepts a category code directly.
5. `h.Svc.CategoryMemorize(ctx, categoryID, locale)` → the question list.
6. `h.Content.LoadQuestionDetails` for question/answer/explanation content
   (existing helper, same one `listSessionQuestions` uses).

### New service method + query

One new sqlc query returns the whole topic in source order with each
question's correct answer id in the same round trip — no session, no
cursor, no per-question follow-up query:

```sql
-- name: OrderedQuestionsWithCorrectAnswerByCategory :many
-- The memorize view shows a whole topic at once, unlike ordered practice
-- (OrderedQuestionIDsByCategory) which walks it in slices via a stored
-- cursor. Same ordering, no cursor, no limit — every valid question in the
-- topic, once.
SELECT q.id, q.correct_answer_id
FROM question q
WHERE q.validation_status = 'valid'
  AND q.category_id = sqlc.arg(category_id)
ORDER BY NULLIF(regexp_replace(q.source_ext_id, '\D', '', 'g'), '')::bigint NULLS LAST,
         q.source_ext_id, q.id;
```

`Service.CategoryMemorize(ctx, categoryID, locale)`:
1. Run the query above.
2. Empty topic → return an empty list (not an error; an empty category is
   real for a not-yet-populated topic and the frontend just shows "no
   questions").
3. Defensively error if any row's `correct_answer_id` is null (mirrors the
   existing guard in `ListSessionQuestionAccesses` — a `valid` question is
   never supposed to be missing one).
4. Build the ordered id list, call `content.LoadQuestionDetails` for
   text/answers/explanation, then attach each question's `correct_answer_id`.
5. Shuffle each question's answers with the existing
   `shuffleSessionAnswers(answers, seedID, questionID)` (a fixed constant
   `seedID`, not a real session id — this is cosmetic ordering only, not
   anti-cheat, but reusing it avoids "the correct answer is always option A"
   and avoids writing a second shuffle).

Response shape mirrors `sessionQuestionDetailResponse` minus the
session-specific fields (`answered`, `user_answer_id`, `correct`):

```go
type memorizeQuestionResponse struct {
    content.QuestionDetailDTO
    Position        int    `json:"position"`
    CorrectAnswerID string `json:"correct_answer_id"`
}
```

## Frontend

### Topic card button

`frontend/src/app/[locale]/(app)/practice/page.tsx`, in the category card
grid (currently one `<button>` per card handling the whole-card click):

- Change the card root from `<button>` to a `<div role="button" tabIndex={0}
  onClick={...} onKeyDown={...}>` so it can host a second, independent
  interactive element (a `<button>` cannot legally nest inside a
  `<button>`).
- Add a small icon button (e.g. `GraduationCap`, distinct from the
  `BrainCircuit` icon already used for the spaced-repetition tab) in a
  corner of the card. `onClick` calls `e.stopPropagation()` then navigates to
  the memorize route — it must not also trigger the card's own
  `handleCategoryClick`.
- If `!allowance?.unlimited` (the existing VIP signal already loaded via
  `usePracticeAllowance`), show a small gold crown badge on the button, same
  visual language as the existing `allowanceUpgrade` treatment — a preview,
  not a block; the actual gate is server-side (station-licensed kiosk users
  are also `!unlimited`-looking on the frontend today in some paths, so the
  button must still be clickable and let the server decide).
- Shown identically for `kiosk` and non-kiosk (unlike the VIP-purchase
  banner, which is `!kiosk`-gated elsewhere on this page).
- Navigates to `/${locale}/${kiosk ? "station/practice" : "practice"}/memorize/${code}`.

### New route: the memorize viewer

New page, e.g.
`frontend/src/app/[locale]/(app)/practice/memorize/[code]/page.tsx`, plus a
thin kiosk re-export under `(kiosk)/station/practice/memorize/[code]/page.tsx`
(same pattern as the existing kiosk practice page — reuse the component,
pass `kiosk`).

Not a session — no `useSessionEngine`. A small dedicated hook fetches
`categories/${code}/memorize?locale=${locale}` once, maps each item to the
existing `SessionQuestionItem` shape (export the current private
`toQuestionItem`/`QuestionDetailResponse` from `use-session-engine.ts` and
reuse it — same mapping, no duplication), and holds `currentIndex` locally.

Rendering reuses `QuestionStage` as-is:
- `answerStateFor={(answerId) => question.correct_answer_id === answerId ? "correct" : "neutral"}`
- `disabled={true}` (answer options are inert — no click handler does
  anything)
- `onSelectAnswer={() => {}}`
- `onZoomImage` / `onOpenExplanation` wired the same as the normal session
  page (`ExplanationDialog` reused as-is)
- `answered={true}` so the explanation affordance behaves the same as an
  already-answered question elsewhere

Chrome around `QuestionStage`:
- Progress label "`{index+1} / {total}`"
- **Oldingi** / **Keyingi** buttons only (no submit, no per-answer feedback
  flow)
- Reaching the end shows a small "Mavzu tugadi" state with a button back to
  `/practice` (or `/station/practice` on kiosk)
- Exit control back to the topic list, matching the existing session page's
  exit affordance

### VIP-required error handling

On a `402 vip_required` response, mirror the exact pattern already in
`session/start/page.tsx`:
- Non-kiosk: message + button to `/${locale}/premium`
- Kiosk: message + button back to `/${locale}/station` (no checkout link —
  matches the existing "no VIP entry point on kiosk" rule elsewhere on this
  page)

### Translations

New keys needed in all three message files (`uz-Latn.json`, `uz-Cyrl.json`,
`ru.json`):
- `Practice.memorizeButton` (short label, fits a small corner button)
- A small set under a `Memorize` namespace (or reuse `Session` where it
  already fits): loading, error, empty-topic, "topic finished" state,
  prev/next labels, vip-required message + CTA labels (can reuse
  `Session.vipRequired` / `Session.goToPremium` / `Session.backToStation` if
  those exact keys already exist from the session/start flow — verify during
  implementation rather than assuming, and add new keys only where nothing
  fits).

## Out of scope

- No changes to the "Hammasi" ordered practice or its cursor.
- No new session mode, no `exam_session` row, no answer persistence.
- No offline/download support for the memorize list.
- No admin/CMS surface for this feature — it's a pure read view over
  existing content.
