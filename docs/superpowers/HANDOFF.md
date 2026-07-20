# AvtoTest — Session Handoff (2026-07-19)

> **⚠️ STALE — SUPERSEDED BY THE 2026-07-21 STACK PIVOT.** Everything below about
> Flutter (Plan 06/07 status, T6/T8/T9 next steps, Flutter environment setup,
> Flutter collision constraints) is obsolete: the Flutter frontend was **retired and
> deleted from the repo** (commit `c938664`; history in git via `git log -- app/`).
> The new frontend is **Next.js + TypeScript + Tailwind + shadcn/ui** and has NOT
> been started yet. The current source of truth is the repo-root
> **`AVTOTEST-MASTER-PROMPT.txt`** — start there, not here. Still valid below: the
> Go backend status (Plans 01–05 complete), the real-content import facts, the
> backend DB-test contention constraint (§"Two collision constraints", item 2), and
> the general working-style notes. Do NOT resume any Flutter task from this file.

> Paste this whole file's content (or tell the AI to read this file) as your first message in a new session. It is written to be self-contained: a fresh AI with zero memory of prior conversations should be able to pick up exactly where things left off.

## What AvtoTest is

A paid online driving-theory-exam prep platform for Uzbekistan, explicitly targeting "10-15x better" than existing competitors (onless.uz, osonprava.uz, avtoimtihon.uz). Backend: Go. Frontend: Flutter (web-first). Repo root: `/home/sher/Рабочий стол/avtotest` (note the Cyrillic + space in the path — **always double-quote it in shell commands, never backslash-escape**).

Full platform design lives at `docs/superpowers/specs/2026-07-17-avtotest-platform-master-design.md` (milestones M1-M7, decisions D1-D17). Currently working on **M1** (web learning core).

## User working style (important, read before doing anything)

- User communicates in Uzbek. Respond in Uzbek in chat; code/comments/commits/docs stay in English matching existing conventions.
- **Execution model: subagent-driven-development.** Don't implement directly yourself for any non-trivial task — dispatch a fresh implementer subagent per task (via the `Agent` tool), then a separate reviewer subagent that independently, adversarially re-verifies (never trusts the implementer's self-report at face value — re-derive claims, revert fixes to confirm tests actually discriminate, then restore). Fix→re-review if the reviewer finds anything Critical. Update `.superpowers/sdd/progress.md` after every task. Do this continuously without stopping to ask "should I continue?" between tasks — only stop for a genuine blocker or ambiguity.
- **Model-tier convention (explicit user instruction, supersedes any older 3-tier habit):** **Fable → Opus → Sonnet → Haiku**, strongest-logic to simplest. Fable = the single hardest-logic/highest-stakes piece (subtle concurrency/race fixes, final adversarial re-review of a Critical/data-corruption bug, deep algorithmic correctness like FSRS). Opus = meaningfully complex but a notch below (tricky multi-file integration, most whole-task reviews of complex features, real-data import+verification). Sonnet = the default workhorse for most implementer/reviewer dispatches. Haiku = genuinely mechanical/fully-specified work. Pick per-task, don't default everything to one tier.
- **Maximum parallelism requested**: user explicitly asked to split all parallelizable work across subagents, up to 15 concurrent agents. BUT: respect two collision constraints (below) — don't blindly parallelize file-colliding work.
- User gets frustrated by repeated interruptions for routine approvals — this project's `.claude/settings.json` already has broad allow-lists; don't add friction back.
- **This session hit a Claude Code usage/rate limit once already** (the user accidentally killed two in-flight agents when trying to check on this, then said "89% limit qoldi, shu limitga yarasha ishla" — work economically w.r.t. remaining usage). If a background agent's task-notification shows `status: killed` or `status: failed` with no committed work, just relaunch it fresh (check `git status`/`git diff` first — if nothing was written to disk, it's safe to redo from scratch, no special recovery needed). If a fresh new session is starting because of exactly this kind of limit/compaction, ask the user whether they want continued max-parallelism or a more conservative pace before resuming heavy dispatch — don't assume either way.

## Two collision constraints that limit parallelism

1. **DI-wiring / shared-file consolidation pattern**: `app/lib/main.dart` (DI wiring), `app/lib/app/router.dart` (routes), and `app/lib/features/home/presentation/home_shell.dart` (home nav grid) are shared files every new Flutter feature touches. When dispatching 2+ Flutter feature tasks in parallel, **instruct each one to NOT edit these shared files directly** — instead have each report back the exact lines/diff it needs added, then consolidate all of them into one pass yourself (the orchestrating session), verify (`dart analyze` + `flutter test --concurrency=1`), and commit that consolidation separately. This has been done successfully several times already (see commits `a2d7d66`, `2d12176`).
2. **Backend DB-test contention**: running `go test ./... -p 1` from multiple concurrent shells/agents against the shared `avtotest_test` Postgres DB causes deadlocks/duplicate-key/FK-violation false failures (`testdb.Truncate` does a cross-table `TRUNCATE CASCADE`, and concurrent invocations collide — confirmed via isolated-worktree re-testing, not a real code bug). Avoid dispatching multiple backend Go agents that will run the full test suite at the same time.

## Environment setup (Flutter) — required for every Flutter shell command

```bash
export PATH="$HOME/.local/flutter/bin:$PATH"
export CHROME_EXECUTABLE=google-chrome-stable
```

- **Use `dart analyze`, NEVER `flutter analyze`** — the latter crashes on this repo's Cyrillic path due to an LSP Content-Length (UTF-16 vs UTF-8 byte count) bug in the bundled `analysis_server`. Not fixable (not our source). CI is unaffected (ASCII paths there).
- **Use `flutter test --concurrency=1`, NEVER bare `flutter test`** — misbehaves (silently truncates/misattributes results) under default concurrency on this Cyrillic path. CI unaffected.
- Flutter SDK lives at `~/.local/flutter`, stable channel 3.44.6, web support enabled.
- No screenshot/browser-automation tool is available to subagents by default — verification standard is `dart analyze` (0 issues) + `flutter test --concurrency=1` (all pass) + `flutter build web` (succeeds), with occasional one-off live `flutter drive` integration-test passes (recipe: `flutter drive --driver=test_driver/integration_test.dart --target=integration_test/<file>.dart -d web-server --web-port=<port> --browser-name=chrome` with a matching external `chromedriver` — `flutter test -d chrome` / plain `flutter drive -d chrome` do NOT work for web integration tests on this Flutter version).

## Architecture conventions (Flutter)

- Feature-first Clean architecture: `app/lib/features/<name>/{data,domain,presentation}`.
- Riverpod (Notifier/AsyncNotifier + providers) + freezed + json_serializable + go_router + Dio + mocktail for tests.
- Every `*Api` class is constructed from the SAME single shared `Dio` instance built once in `main.dart` — never call `buildDio` twice.
- API envelope: `{"data":...,"meta":{...}?}` success / `{"error":{"code","message"}}` failure — every wrapper uses `core/result.dart`'s `guard()`.
- Anti-cheat (backend-enforced, client must respect): in `exam` mode, `correct`/`correct_answer_id` are withheld by the server on every answer response until the session finishes. Content endpoints never send `is_correct`/`correct_answer_id` at all. **Never infer/compute correctness client-side** — if the field is null, render "no feedback yet."
- Locale: pass `localeProvider`'s backend code via `localeToBackendCode` (Plan 06 Task 4) — don't re-derive.
- VIP gating: `POST /sessions` can 402 `vip_required` (variant bilet #2+, exam, mistakes) or 429 `daily_limit_reached` (practice). Both must be distinct, reactable `Failure.code` values, not a generic error banner.

## Hard-won lessons (apply proactively, don't rediscover)

- **Seam testing discipline** (this plan's biggest recurring lesson): a screen tested standalone with a fake controller, plus a controller tested standalone with a fake repository, is **necessary but not sufficient**. Every task wiring a new screen to a controller must include a test driving the REAL screen + REAL controller together (fakes only at the network/API boundary) proving the actual end-to-end behavior (e.g. "after finishing a session, the app actually navigates to results" — not just "state object says finished"). This exact bug shape (individually-correct pieces that don't actually work combined) has appeared repeatedly: an OTP screen that built correct state but never called `context.go(...)`; a concurrent-401 double-refresh that silently revoked all sessions; etc.
- **Riverpod hydration-race pattern**: any Notifier/AsyncNotifier whose `build()` returns synchronously but has an async init step (reading shared_preferences, checking a stored session) risks background hydration completing AFTER a user action already advanced state, silently clobbering it. Fix: an explicit synchronous override-guard flag set as the first statement of the user-action method, checked by hydration before it writes state.
- **Dio Zone-forking gotcha**: Dio forks a `Zone` per interceptor call. Completing a shared `Completer` with `.completeError()` from a different zone than the one awaiting it can hang/swallow the error. Fix: complete with a plain value, never an error; each caller throws within its own zone via `Error.throwWithStackTrace`.
- **Concurrent-401 single-flight refresh**: multiple simultaneous 401s must share ONE `/auth/refresh` call (a `Completer`-based guard in `AuthInterceptor`) — the backend rotates+revokes refresh tokens per use, so two concurrent refreshes trip replay/compromise detection and revoke ALL of a user's sessions across devices. Already fixed (`app/lib/core/network/auth_interceptor.dart`).

## Status as of this handoff

### Backend (Go) — Plans 01-05: all complete.
Sessions/scoring/unlock, FSRS learning engine + stats, explanations/saved/streak/entitlement/events. Not re-detailed here — read `.superpowers/sdd/progress.md` (top of file) if you need specifics.

### Flutter Plan 06 (foundation) — complete.
Auth (OTP), theme, locale, profile, DI wiring, router with auth guard. All 9 tasks done, whole-branch review passed twice (first pass found+fixed a concurrent-401 Critical bug).

### Real content import (dedicated mini-plan, `docs/superpowers/plans/2026-07-19-real-content-import-avtoimtihon.md`) — complete.
1235 real licensed driving-exam questions (source: `/home/sher/Рабочий стол/aaa/`, the user's own prototype, rights confirmed clear) converted and imported:
- **Ticket-pairing decision (non-negotiable, user-confirmed)**: our bilets stay exactly 20 questions; the converter pairs two consecutive 10-question source "tickets" into one 20-question bilet variant → 61 full bilets + 15 leftover standalone questions (not in any named bilet).
- **Answer-count decision (non-negotiable, user-confirmed)**: never force a fixed answer count — real distribution is 2/3/4/5 answers (196/638/282/119), validator + storage + DB CHECK all now correctly accept 2-5 (was buggy at two different layers, both found and fixed — see below).
- Two real bugs found and fixed during this work: (1) validator only relaxed to 2-5 at the count-check gating level, not applied for non-4 counts (fixed, commit `e6d0a8b`); (2) **CRITICAL**: storage layer (`store.go`) silently dropped any answer at `position > 4` and the DB had `CHECK (position BETWEEN 1 AND 4)` — 25 real 5-answer questions would have had their correct answer silently deleted. Fixed via migration `0006_answer_position` + `store.go` widening to 1-5 (commit `41233a9`), independently adversarially re-reviewed clean (Fable-tier review: reverted the fix, confirmed the regression test genuinely fails for the predicted reason, restored, confirmed passes).
- Import executed and live-verified against the running API (commit `182ef21`): 61 variants, 1235 questions, 1219 explanations, fidelity spot-checked against source by id including a position-5-correct-answer question, image byte-verified as real webp. Dev DB was clean-truncated and reseeded with real content only (the `[NAMUNA]` fixture, including its 4 signs, is fully restorable any time via `make seed`; backend tests use a separate `avtotest_test` DB, unaffected). `make seed-real` target added.
- Known honest gaps, not bugs: single fallback category `umumiy` for all real content (no per-category tagging yet — future work), no signs catalog comes from this import, `LegalRefs` empty (citations are inline prose, not machine-extracted), the id-614 cross-locale 3-vs-4-answer anomaly was reconciled to the minimum (3) after confirming the correct answer survives in all locales.
- **Master spec was also updated** (`docs/superpowers/specs/2026-07-17-avtotest-platform-master-design.md`) with a Fable-tier competitive analysis of avtoimtihon.uz (commit `fb7a9ee`): F1-F4 → F1-F5 (dynamic answer count), image-zoom UI idea, guest-demo idea (D13/M2), PWA install idea (M6), and importantly the idea of using the source `comment` field as an AI-draft-explanation seed (§13) — noted here since it's a real, not-yet-implemented product idea worth remembering. **One correction was needed**: Fable's first pass had wrongly changed the bilet size from 20→10 (copying avtoimtihon.uz's own structure) because it wasn't briefed on the user's already-confirmed pairing decision — this was caught and reverted; **lesson for any future agent dispatch that might touch shared spec/architecture docs: always brief it with existing confirmed decisions on overlapping topics first, don't let it "discover" the wrong answer from a competitor site.**

### Flutter Plan 07 (test-taking screens) — `docs/superpowers/plans/2026-07-19-m1-plan-07-flutter-test-flows.md` — IN PROGRESS, this is the active work.

| Task | Status |
|---|---|
| T1 Content data layer (categories/variants/signs/questions) | ✅ complete |
| T2 Shared widgets (QuestionCard, AnswerOption+F1-F4, CountdownTimer, QuestionNavigator) | ✅ complete |
| T3 Session data layer (SessionApi, domain models) | ✅ complete |
| T4 Session screen + controller (all 4 modes, anti-cheat, seam test) | ✅ complete, reviewed Approved (commit `ec9186a`) |
| T5 Session results screen + Variants (bilet) screen | ✅ complete + router/home_shell consolidated (commits `b63e4c7`, `2d12176`) |
| T6 Practice setup + Signs catalog + Mistakes bank screens | ✅ code + router/home_shell consolidated (commits `b0c05c3`, `95627f4`), **but its review did NOT complete** — the reviewer agent hit the Claude Code session/usage limit mid-run (`status: failed`, "You've hit your session limit · resets 3:30am (Asia/Tashkent)") and produced no report at all. **Re-dispatch a fresh reviewer for T6 once capacity allows** — the review prompt used is recoverable from this session's own history if resumable, otherwise redispatch fresh using the Task 6 section of the plan + `git show b0c05c3`/`git show 95627f4` as the basis (see the "Specific things to verify" list this handoff's authoring session used: one-of category/sign enforcement, daily_limit_reached/vip_required distinction via SessionScreen's `_ErrorView`, the category/sign UUID-vs-code gap, SignChip usage, test-double duplication judgment call). Signs has no home-grid slot — reachable via an `IconButton` on `PracticeSetupScreen`'s app bar. **Known non-blocking gap** flagged by T6's own implementer (not yet independently verified by a review): `Category`/`Sign` domain models only expose `.code`, but backend `POST /sessions` expects UUIDs for `category_id`/`sign_id` — practice mode will likely 400 against a real backend until this content-contract gap is resolved (separate backend/content task, not a Flutter bug — needs its own follow-up task, not yet created). |
| T7 Explanation rendering + feedback | ✅ complete |
| T8 Saved questions + streak + stats | 🔶 **dispatched but made no progress before the underlying Claude Code process itself restarted** (its background-agent notification came back as `status: stopped`, "No completion record was found... previous session", meaning the harness process exited/restarted while it was running). `git status`/`find` confirmed **zero files were written to disk** for it — nothing to lose, safe to treat as never-started and redispatch fully fresh (do not try to resume by agent ID; that ID belongs to the now-gone process). Same shared-file caveat applies: it must report `main.dart`/`router.dart`/`home_shell.dart` lines rather than edit them directly (three shared files this time, not two, since T8 also needs DI wiring for new `SavedApi`/`ProgressApi`). |

**Session-limit note as of this handoff**: a Claude Code usage/rate limit was hit ("resets 3:30am Asia/Tashkent") for at least one subagent dispatch, and the underlying process also appears to have restarted once around the same time (see T8's row above). **If you are a fresh session reading this because of exactly this kind of interruption, check the current wall-clock time against the stated reset time before dispatching new heavyweight (Sonnet/Opus/Fable) subagents** — if still before reset, either wait, or continue with lighter-weight work (e.g. the orchestrating session's own direct file edits, like the router/home_shell consolidations already done by hand in this document's history) rather than spawning more agents that will also fail immediately.
| T9 VIP-gating UI + free-limit UX polish | ⬜ not started — **depends on T5/T6 landing** (sweeps their `vip_required`/`daily_limit_reached` call sites to route to a real upsell screen instead of ad-hoc handling) |
| T10 Event logging client | ✅ complete |
| T11 Full verification + live test-flow check + docs | ⬜ not started — final task, do last |

**Two Important (non-blocking) findings from T4's review, not yet acted on, worth fixing in a dedicated follow-up pass (not blocking, but real)**:
1. Mid-session answer-API failure (a transient network blip) moves the whole session to an error state with no retry — for exam mode specifically, this can strand an entire VIP-gated/limited attempt. Recommend a per-answer retry keeping the session active, not a full-session error.
2. The session screen (`session_screen.dart`, Task 4) uses hardcoded Uzbek strings instead of the app's existing ARB/`AppLocalizations` system (which auth/home screens already use) — a uz-Cyrl or ru user gets this core screen in Latin Uzbek regardless of their chosen locale. This is a real multi-locale regression on this one screen, worth a dedicated l10n sweep pass across Plan 07's screens (T4-T9 all likely share this shortcut, per each task's own file-scope discipline) rather than fixing piecemeal.

**Recommended order for what's left**, respecting the two collision constraints above:
1. Resolve/finish/review T6 (as detailed in the table above).
2. Once T6 is merged, dispatch **T8 and T9 sequentially, not in parallel** — T9 explicitly needs T5's and T6's actual `vip_required`/`daily_limit_reached` call sites to exist to "sweep" them consistently, and T8 touches the same session/results screens T9 also touches (save-toggle icon) → real risk of overlapping edits, not just a shared-file-name collision like router.dart. Do T8 first (touches saved/stats feature dirs + main.dart DI + router/home_shell, using the same skip-and-report pattern for the shared files), consolidate, then do T9.
3. T11 (final verification + live flutter-drive pass + README update) — last, after T8/T9 land.
4. Consider a dedicated small follow-up task for the two Important T4-review findings above (l10n sweep + per-answer-retry) — not urgent, but real; ask the user if/when to prioritize it relative to Plan 08.
5. After Plan 07 fully closes: **Plan 08 (E2E + staging deploy)** is the last M1 plan and has not been written yet — write it (via `superpowers:writing-plans` or by hand following this repo's existing plan-doc conventions) before executing.

## Where everything lives

- Master spec: `docs/superpowers/specs/2026-07-17-avtotest-platform-master-design.md`
- Plan 06 (Flutter foundation, done): `docs/superpowers/plans/2026-07-19-m1-plan-06-flutter-foundation.md`
- Plan 07 (Flutter test flows, active): `docs/superpowers/plans/2026-07-19-m1-plan-07-flutter-test-flows.md`
- Real content import mini-plan (done): `docs/superpowers/plans/2026-07-19-real-content-import-avtoimtihon.md`
- Durable task ledger (append after every task, full adversarial-review detail): `.superpowers/sdd/progress.md`
- Real content source (user's own prior prototype, rights confirmed clear): `/home/sher/Рабочий стол/aaa/src/data/questions.{uz-Latn,uz-Cyrl,ru}.json` + `aaa/public/quiz-images/*.webp`
- Converter output (gitignored, regenerate via `make seed-real` or `cmd/convertavtoimtihon`'s own header comment): `backend/seed/avtoimtihon/`
- This session's persistent cross-conversation memory (auto-loaded every new session, do not duplicate its content into new files): `/home/sher/.claude/projects/-home-sher--------------avtotest/memory/` — in particular `avtotest-workflow-preferences.md` (execution style, permission friction resolution, model-tier convention), `avtotest-platform-loyihasi.md` (stack/decisions/status), `osonprava-raqobat-tahlili.md` + `avtoimtihon-raqobat-tahlili.md` (competitor analyses).

## Immediate next step for whoever picks this up

1. `cd "/home/sher/Рабочий стол/avtotest" && git status && git log --oneline -5` — confirm you're looking at current state, not stale info from this document.
2. Check whether the T6 background agent (dispatched just before this handoff was written) has completed — if you're in a genuinely fresh session with no memory of dispatching it, there's no way to "resume" it by ID; instead inspect the untracked files it left (`app/lib/features/{mistakes,practice,signs}/`, `app/lib/shared/widgets/sign_chip.dart`) — run `dart analyze` and `flutter test --concurrency=1` from `app/` to see if what's on disk is complete/working, decide from there whether to finish it yourself, dispatch a fresh implementer to complete it, or treat it as done and move straight to reviewing + consolidating router/home_shell wiring + committing.
3. Read `.superpowers/sdd/progress.md`'s tail for the most recent entries (Plan 07 section) to cross-check against this document — it is the authoritative ledger, this handoff is a snapshot.
