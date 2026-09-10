# Biletlar → «Tozalash»

2026-09-10. A control on the bilet grid that wipes every result the learner has
built up, while the set of open and locked bilets stays exactly as they left it.
Ships to all three surfaces: the learner app on desktop and phone, and the
classroom kiosk.

## The conflict this design exists to solve

Bilet unlock is derived from progress. `IsVariantUnlocked` (`rules.go`) opens #1
for everyone and #N (N>1) only for a VIP whose bilet #N−1 has a `completed_at`.
So "delete the progress" and "do not change what is open" are, on the face of
it, contradictory:

| Profile | Effect of deleting `variant_progress` |
|---|---|
| Kiosk station (`bypass_variant_progress = TRUE`) | none — every bilet already open |
| Free learner | none — only #1 was ever open |
| **VIP learner** | **every bilet past #1 re-locks** ❌ |

## Resolution: an unlock ceiling

Migration **0073** adds `profile.variant_unlock_ceiling int NOT NULL DEFAULT 0`.
Before deleting anything, the reset records how far the completion chain had
walked. `VariantPrevGateSatisfied(number, bypass, ceiling)` then accepts a bilet
at or below that number in place of "the previous bilet was completed".

Three properties make this safe:

- **Default 0 is a no-op.** No bilet number is ≤ 0, so every existing profile
  keeps precisely today's behaviour until it first clears.
- **The ceiling never substitutes for payment.** `IsVariantUnlocked` still ANDs
  `isVIP`. A free learner with a stored ceiling of 11 still sees `vip_required`.
- **The ceiling is computed ignoring VIP.** Reading "currently unlocked" would
  see a lapsed learner's bilets as locked and freeze a ceiling of 1, destroying
  their place permanently. Reading the completion chain instead means renewing
  puts them back where they were.

Writes go through `GREATEST(variant_unlock_ceiling, $2)`, so a second clear —
which necessarily computes a short chain, the first having emptied the table —
cannot lower it.

## The two gates must not drift

The unlock rule lives in two places that compute `prevCompleted` differently:
`ListVariantStatuses` walks the ordered list, `StartSession` looks up the single
previous bilet. A ceiling honoured by only one would render an **open tile that
answers `variant_locked` when tapped**. Both now call the one
`VariantPrevGateSatisfied`, and `bypassVariantProgress` became
`variantUnlockOverrides`, returning bypass and ceiling from the same profile
read. `TestStartSessionAcceptsBiletUnlockedOnlyByTheCeiling` is the test that
fails if they ever separate again.

## Scope

`POST /me/variants/reset` deletes **only** `variant_progress` rows for the
caller. `exam_session`, `question_memory`, `category_mastery`, the streak and
the leaderboard are untouched — the control lives in the bilet section and
clears bilets, not the learner's study record
(`TestResetVariantProgressLeavesSessionHistoryAlone`).

Both writes — raising the ceiling and deleting the rows — run in **one
transaction**. Landing the delete without the ceiling is exactly the outcome
this feature promises cannot happen.

The response is `{cleared, unlock_ceiling}`, so the confirmation quotes the
number the server actually removed.

## UI

The control sits in the header beside the search field: a labelled button from
`md` up, and its 44×44 icon twin on phones, using the same box as the search
toggle. It is disabled whenever no bilet carries a result, and while the grid is
still loading (a count quoted before the data lands would be a lie).

Confirmation is a modal (`components/ui/confirm-dialog.tsx`) — a bottom sheet on
phones, centred from `sm:` up, following the `/signs` detail modal. It adds what
a destructive confirmation needs and a browsable sheet does not: initial focus
on **Cancel** (so Enter, or a TV remote's OK, lands on the safe choice), a Tab
trap, Escape to cancel, focus restored to the opener, and a scroll lock.
`window.confirm` is deliberately not used — a native modal blocks the kiosk's
automation channel and leaves a classroom screen nobody can dismiss remotely.

The outcome message renders **directly under the header**, not down with the
other notices near the grid. A classroom TV is 720px tall; everything from the
filter chips down is below the fold, and a confirmation nobody sees is no
confirmation. `tickets-clear.spec.ts` asserts `toBeInViewport({ratio: 1})` on
it, at 390×844, 1280×720 and 1440×900.

Afterwards the page re-reads the grid from the server rather than zeroing it
locally: the response says how many rows went, but only the server can say which
bilets are open now, and that is precisely the part a learner would notice being
wrong.

## Verification

- Go: 8 integration tests + `TestVariantPrevGateSatisfied` +
  `TestResetVariantProgressOverHTTP`; whole `go test ./...` green.
- Frontend: 777 unit tests (17 on the tickets page, 4 on the hook).
- E2E: 8 new Playwright tests across the three viewports, run ×3 for flake.

Kiosk note: `KioskChrome` (the floating language/theme bar) overlaps the header
controls at phone width. Pre-existing, and it also covers the search toggle;
station PCs run at ≥1024px, where it sits clear. The phone e2e rows hide it,
since they stand in for the learner phone layout, whose top bar is `sticky` and
therefore in flow.
