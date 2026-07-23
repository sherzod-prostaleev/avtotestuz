# Session Screen Redesign — Zero-Scroll Test Surface

**Date:** 2026-07-22
**Status:** Approved
**Scope:** `frontend/src/app/[locale]/(app)/session/[id]/page.tsx` and its child components

## Problem

Solving a ticket is the product's core loop, and it is currently the worst screen in the app.
Everything is stacked in one narrow column:

```
header → error banner → navigator (20 buttons) → question badge →
question text → image (max-h-[28rem] = 448px) → explanation blocks → 4 answers → sticky footer
```

On a 1080p laptop (~900px usable viewport) this stack is roughly 1600–2000px tall. The user must
scroll down on every single question to reach the answer options, then scroll back up to re-read
the question or inspect the image. For a 20-question exam under a 25-minute timer this is a
serious usability failure, not a cosmetic one.

Two root causes:

1. **Image and answers are sequential in one column.** The image consumes ~450px before the first
   answer is reachable.
2. **The explanation renders inline inside `QuestionCard`.** It has no height bound and grows with
   content — some explanations are 6–7 blocks long.

## Content measurements

Taken from the seeded production dataset (1235 questions), not assumptions:

| Metric | Value |
|---|---|
| Answers per question | 2 → 196 q, 3 → 638 q, 4 → 282 q, **5 → 119 q** |
| Question text length | avg 77, p95 151, **max 369** chars |
| Answer text length | p95 107, **max 414** chars |
| Questions with an image | **715 / 1235 (58%)** — 42% have none |

Two consequences drive the design:

- **42% of questions have no image.** A fixed two-column layout would leave half the screen empty
  on nearly half of all questions.
- **Max 5 answers, not 4.** The reference competitor UI shows F1–F4; we need F1–F5. (The existing
  keyboard handler already covers F1–F5 — verified in `page.tsx`.)

## Competitor reference (onless.uz)

Verified from screenshots in the repo root:

- `Снимок экрана_20260717_223252.png` — test screen: full-screen kiosk, question bar centered on
  top, answers in a ~35% left column (top-aligned, not stretched), image in the ~65% right column,
  question navigator as a centered pagination row at the bottom. Zero scrolling. F1–F4 badges on
  answers. Image zoom `−/+` in the header.
- `Снимок экрана_20260717_223315.png`, `..._223330.png` — explanation is a **separate full-screen
  overlay**, not part of the question flow. Left panel is fixed (image + "was this helpful?"
  feedback); the right panel scrolls independently through long content.

**Not verified:** how onless renders a question with no image, or one with 5 answers — no such
screenshot was available. The handling of those two cases below is our own decision, not a copy.

## Design

### Layout selection is content-driven

Not a fixed two-column grid. The stage picks its shape from the question:

**With image (58%)** — two columns:

```
┌──────────────────────┬─────────────────────────┐
│ Question text        │                         │
│ F1 ○ Answer 1        │        [ IMAGE ]        │
│ F2 ○ Answer 2        │     full height,        │
│ F3 ○ Answer 3        │     object-contain      │
│ F4 ○ Answer 4        │                         │
└──────────────────────┴─────────────────────────┘
  min 380px / max 520px         remaining space
```

**Without image (42%)** — single centered column, `max-w-3xl`, slightly larger type. Answers stay
in a single column: one column is faster to read and produces fewer misreads than a 2×2 grid.

### Zero-scroll mechanics

The page becomes a fixed-height flex column:

- `main`: `h-[100dvh] flex flex-col overflow-hidden`
- header / navigator / footer: `shrink-0`
- stage: `flex-1 min-h-0`
- image wrapper: `min-h-0`, `<img class="h-full w-full object-contain">`

`100dvh` rather than `100vh` so mobile browsers do not jump when the address bar collapses.

### Overflow strategy for long content

Absolute zero-scroll cannot be guaranteed for every input — a 369-char question plus five 414-char
answers on a 768px-tall laptop does not fit at normal type sizes. Clipping content is not
acceptable. Two-stage defence instead:

1. **Auto-compact.** When the question has ≥5 answers or its total text length exceeds a threshold,
   drop one density step: `p-4` → `p-3`, `text-sm` → `text-[13px]`, tighter gaps. Deterministic,
   worth roughly 80–100px.
2. **Contained scroll.** Only if it still does not fit, the answers column scrolls internally
   (`min-h-0` + `overflow-y-auto`). The page itself never scrolls, so the user never loses the
   question, the image, or the navigation.

In practice the p95 question (151-char stem, 107-char answers) fits with no scroll at all.

### Mobile

Single column, fits one viewport:

```
header → question text → image (max 35vh, tap to zoom) → answers → bottom nav
```

The existing zoom modal covers the case where 35vh is too small to read the image detail.

### Explanation moves out of the flow

`QuestionCard` stops rendering explanations. After answering, the flow shows only a compact result
plus two buttons: **Saqlash** (bookmark) and **Ekspert tahlili**. The latter opens
`ExplanationDialog` — a modal mirroring the competitor's structure: fixed left panel (question
image), independently scrolling right panel (explanation blocks). The height of the question flow
therefore never changes when an answer is submitted.

## Components

`session/[id]/page.tsx` is 732 lines with everything inline. Split it:

| Component | Responsibility | Status |
|---|---|---|
| `SessionHeader` | timer, locale, bookmark, fullscreen, exit, finish | extracted from page |
| `QuestionNavigator` | 1..N buttons, single row, horizontal scroll | extracted from page |
| `QuestionStage` | chooses one/two-column shape; owns the height budget | new |
| `ExplanationDialog` | explanation modal | new |
| `QuestionCard` | question text + image only | explanation removed |
| `AnswerOption` | unchanged | unchanged |

`page.tsx` keeps session state, the engine hook, keyboard handling, and data flow — it stops being
a layout file.

## Preserved behaviour

Timer, F1–F5 shortcuts, arrow-key navigation, bookmark, fullscreen toggle, image zoom modal,
question navigator, sidebar and app header (the app shell stays; only the session content area is
restructured).

## Testing

- `question-card.test.tsx`: explanation assertions move to a new `explanation-dialog.test.tsx`.
- New `question-stage.test.tsx`: two-column shape when `imageUrl` is present; single-column when
  absent; compact mode when answers ≥ 5.
- New assertion in the session page test: explanation is not in the document until the
  "Ekspert tahlili" button is activated.
- Existing session, navigator, and answer tests must keep passing unchanged.

## Out of scope

- Full kiosk mode (hiding the sidebar during a test) — considered and explicitly rejected by the
  product owner; the app shell stays.
- Image zoom `−/+` controls in the header (competitor has them; our tap-to-zoom modal already
  covers the need).
- Any backend or API change. This is a presentation-layer redesign only.
