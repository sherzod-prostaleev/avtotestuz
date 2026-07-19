# M1 Plan 07 — Flutter Test Flows Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The actual learning/testing screens on top of Plan 06's foundation — bilet grid, the 4 test-taking modes (variant/exam/practice/mistakes) with real anti-cheat-respecting UI (F1-F4 shortcuts, navigator, exam timer), explanation rendering, saved questions, streak/stats display, VIP-gating UI, and client-side event logging. This is what turns the "auth + empty shell" from Plan 06 into an actual usable product for the first time.

**Architecture:** Same feature-first Clean architecture as Plan 06 (`lib/features/<name>/{data,domain,presentation}`), same Riverpod + freezed + go_router conventions. New shared widgets land in `lib/shared/widgets` per master spec §15. Every new feature's data layer talks to the Go backend built in Plans 01-05 — read the actual Go handler/DTO source when a field shape isn't fully spelled out in `README.md`, never guess.

**Tech Stack:** Same as Plan 06 (Flutter 3.44.6, Riverpod, go_router, Dio, freezed/json_serializable, mocktail, integration_test).

**Plan sequence for M1:** 01-05 backend (complete) → 06 Flutter foundation (complete) → **07 Flutter test flows (this plan)** → 08 E2E+staging deploy.

## Environment note (carried over from Plan 06 — still applies)

```bash
export PATH="$HOME/.local/flutter/bin:$PATH"
export CHROME_EXECUTABLE=google-chrome-stable
```
**Use `dart analyze`, not `flutter analyze`** (LSP Content-Length bug on this repo's Cyrillic path — CI unaffected). **Use `flutter test --concurrency=1`, not bare `flutter test`** (concurrency quirk on this path — CI unaffected). No screenshot/browser-automation tool is available to agents — verification standard is `dart analyze` + `flutter test --concurrency=1` (unit + widget tests, real `WidgetTester` trees) + `flutter build web`, with one live/manual `integration_test` pass in the final task (not CI-wired), mirroring Plan 06 Task 9's precedent.

**Hard-won lesson from Plan 06, apply proactively in this plan**: repeatedly, a piece that was individually correct in isolation (a screen, a controller, an interceptor) broke only when combined with another individually-correct piece — an OTP retry that no-op'd after an error state, concurrent profile fetches racing a token refresh, a screen that never navigated after a successful action. Every task below that wires a new screen to its controller must include at least one test that exercises the REAL screen widget together with its REAL controller/router (fakes only at the network boundary), not two pieces tested only in isolation from each other. Task reviewers: treat "screen tested standalone with a fake controller" + "controller tested standalone with a fake repository" as **necessary but not sufficient** — look for the seam.

## Global Constraints

- Repo root: `/home/sher/Рабочий стол/avtotest` (Cyrillic + space — always double-quote, never backslash-escape). App: `app/`, package `avtotest_app`.
- Backend contract is authoritative in `README.md`'s "Sessiya", "FSRS o'quv dvigateli", "Izohlar", "Saqlangan savollar", "Kunlik streak", "Free-tier / VIP chegarasi", "Voqealar (events)" sections, and Plan 01's Content API section — but **content endpoint response shapes (categories/variants/signs/questions) aren't fully spelled out in the README, only their paths are** — read `backend/internal/content/{handlers,dto}.go` directly for exact field names before modeling `Category`/`Variant`/`Sign`/`Question` domain classes. Same discipline for anything session/explanation/progress-related that isn't 100% explicit in prose.
- API envelope unchanged: `{"data":...,"meta":{...}?}` / `{"error":{"code","message"}}`. Every new API wrapper uses `core/result.dart`'s `guard()` (Plan 06 Task 2), same as `AuthApi`/`ProfileApi`.
- **DI wiring, decided up front this time (Plan 06 had to patch this gap twice mid-execution — not repeating that here):** every new `*Api` class this plan introduces (`ContentApi`, `SessionApi`, `ExplanationApi`, `ProgressApi`, `EventsApi`) is constructed from the **same single `Dio` instance** already built once in `app/lib/main.dart` (the one shared by `AuthApi`/`ProfileApi` since Plan 06 Task 7/8) — never call `buildDio` again. Each gets its own provider override in the same `ProviderContainer(overrides: [...])` list in `main.dart`. The task that introduces each `*Api` is responsible for adding its own DI wiring line in `main.dart` in the same commit — don't defer this to a later task or leave a `throw UnimplementedError` seam unwired past the task that needs it live.
- Anti-cheat (backend-enforced, client must respect, never work around): in `exam` mode, `correct`/`correct_answer_id` are withheld by the server on every `POST /sessions/{id}/answers` response until the session finishes — the UI must not attempt to infer or locally compute correctness in exam mode; only show what the server actually returned. `question`/`answer` content endpoints never include `is_correct`/`correct_answer_id` either (Plan 01 invariant, still true).
- Locale: every content/session API call passes the current `localeProvider`'s backend code (via Plan 06 Task 4's `localeToBackendCode`) as the `locale` param/field — reuse that single mapping function, don't re-derive.
- VIP gating: `POST /sessions` can return `402 vip_required` for `variant` (bilet #2+), `exam`, and `mistakes` modes. `practice` can return `429 daily_limit_reached`. Both must be modeled as distinct `Failure.code` values the UI can react to specifically (a paywall-style screen for the former, a "kelaso keyin urinib ko'ring" message for the latter) — not collapsed into a generic error banner.
- Testing: `mocktail` for repository/API mocks in unit tests; real `WidgetTester`-built widget trees for screen tests; per the lesson above, at least one integration-style test per new screen+controller pairing exercising them together for real.
- Commits: conventional + Claude co-author trailer, direct to `main`, no branches (established convention).
- CI: no changes needed unless a task's own new test file requires it (e.g. a new codegen'd class needs `build_runner` — already wired into the `frontend` CI job since Plan 06 Task 2).

## File Structure (new additions this plan)

```
app/lib/
  features/
    content/
      data/content_api.dart
      domain/{category,variant,sign,question}.dart
    variants/
      presentation/{variants_screen,variants_controller}.dart
    signs/
      presentation/{signs_screen,signs_controller}.dart
    session/
      data/{session_api,session_repository}.dart
      domain/{session_models,session_state}.dart          # freezed
      presentation/{session_controller,session_screen,session_results_screen}.dart
    practice/
      presentation/practice_setup_screen.dart
    mistakes/
      presentation/mistakes_screen.dart
    explanation/
      data/explanation_api.dart
      domain/explanation_block.dart
      presentation/{explanation_view,explanation_feedback_button}.dart
    saved/
      data/saved_api.dart
      presentation/{saved_controller,saved_screen}.dart
    stats/
      data/progress_api.dart                                # streak + stats, shares "progress" naming w/ backend's internal/progress
      domain/{streak,stats}.dart
      presentation/{stats_screen,streak_card}.dart
    events/
      data/events_api.dart
      presentation/event_logger.dart                        # batch queue + flush
    billing/
      presentation/vip_required_screen.dart                 # M1: static upsell placeholder, real billing is M2
  shared/widgets/
    question_card.dart
    answer_option.dart          # incl. F1-F4 keyboard shortcut handling
    countdown_timer.dart
    question_navigator.dart     # 1-20 grid, answered/current/flagged states
    sign_chip.dart
    mastery_bar.dart
    result_ring.dart
app/integration_test/
  variant_session_flow_test.dart   # Task 11's live pass
```

---

### Task 1: Content data layer

**Files:** create `app/lib/features/content/data/content_api.dart`, `app/lib/features/content/domain/{category,variant,sign,question}.dart`, matching tests; modify `app/lib/main.dart` (DI wiring for `ContentApi`, reusing the shared `Dio`).

**Interfaces (produced):**
```dart
// domain/*.dart — freezed, fields confirmed from backend/internal/content/{handlers,dto}.go,
// not guessed. Question must NOT have any is_correct/correct_answer_id field (anti-cheat —
// the backend never sends it, so don't model a field that would always be null/absent and
// invite someone to wire it up "just in case").
@freezed class Category with _$Category { ... }
@freezed class Variant with _$Variant { ... }           // bilet metadata (number, question_count, etc.)
@freezed class Sign with _$Sign { ... }
@freezed class Question with _$Question { ... }        // includes the verified explanation when present (nullable)

// data/content_api.dart
class ContentApi {
  ContentApi(this._dio);
  Future<Result<List<Category>>> categories({required String locale});
  Future<Result<List<Variant>>> variants();               // no locale — variant metadata is locale-neutral per Plan01
  Future<Result<Variant>> variantByNumber(int n, {required String locale});
  Future<Result<List<Sign>>> signs({String? group, String? query, required String locale});
  Future<Result<Sign>> signByCode(String code, {required String locale});
  Future<Result<Question>> question(String id, {required String locale});
}
```

**Logic:** Thin Dio wrappers, `guard()`-wrapped, mirroring `ProfileApi`'s shape. Read `backend/internal/content/handlers.go` for exact query-param names (`locale=`, `group=`, `q=`) and `backend/internal/content/dto.go` (or wherever response structs live) for exact JSON field names on each type — in particular, confirm whether `Question`'s explanation field is nested (`explanation: {blocks: [...], legal_refs: [...]}`) or flat, and confirm `Block`'s shape (type/text/items — this feeds Task 7's explanation renderer, so get it right here since Task 7 depends on this model).

**Testing:** Unit tests with a fake Dio adapter (same pattern as `auth_api_test.dart`) covering success parsing for each endpoint and at least one 404/error mapping.

- [ ] Steps 1-4 (test-first).
- [ ] **Step 5: DI wiring.** In `main.dart`, construct `ContentApi(dio)` (same shared `dio`) and override its provider alongside the existing ones.
- [ ] **Step 6: Commit.**
  ```bash
  git add app/lib/features/content/ app/lib/main.dart app/test/features/content/
  git commit -m "feat(frontend): content data layer — categories/variants/signs/questions"
  ```

---

### Task 2: Shared question-answering widgets

**Files:** create `app/lib/shared/widgets/{question_card,answer_option,countdown_timer,question_navigator}.dart` and matching widget tests.

**Interfaces (produced):**
```dart
class QuestionCard extends StatelessWidget {
  const QuestionCard({required this.question, required this.imageUrl, super.key});
  // renders question text + optional image; no answer options (those are separate, see below)
}

class AnswerOption extends StatelessWidget {
  const AnswerOption({
    required this.label,          // 'A'..'D' or similar, drives the F1-F4 mapping
    required this.text,
    required this.selected,
    required this.onSelect,
    this.feedback,                // null (no feedback yet) | AnswerFeedback.correct | .incorrect | .theCorrectOne
    super.key,
  });
}
enum AnswerFeedback { correct, incorrect, theCorrectOne }

class CountdownTimer extends StatelessWidget {
  const CountdownTimer({required this.remaining, required this.total, super.key});
  // pure display widget — the actual ticking/countdown LOGIC lives in Task 4's
  // SessionController (a Timer.periodic there), NOT inside this widget. This widget
  // just renders a given Duration; keep the two concerns separate so the countdown
  // logic is unit-testable without pumping widget frames.
}

class QuestionNavigator extends StatelessWidget {
  const QuestionNavigator({
    required this.total,
    required this.currentIndex,
    required this.answeredIndices,      // Set<int>
    required this.onJump,               // void Function(int index)
    super.key,
  });
  // 1..N grid (per master spec §11: "1-20 navigator, javob berilgan/berilmagan/belgilangan")
}
```

**Logic:**
- `AnswerOption`'s F1-F4 keyboard shortcut handling (master spec §11/§16: "F1–F4 javob klavishalari"): use a `Focus`/`Shortcuts`/`CallbackShortcuts` widget (Flutter's standard keyboard-shortcut mechanism) mapping `LogicalKeyboardKey.f1`..`f4` to selecting options A-D — this belongs at the screen level (Task 4's `SessionScreen`, which owns 4 `AnswerOption`s at once and needs a single shortcut scope), not duplicated inside each individual `AnswerOption`. Build the shortcut-handling wrapper here as a small reusable widget (e.g. `AnswerOptionsGroup` wrapping 4 `AnswerOption`s + the `Shortcuts` mapping) so Task 4 doesn't have to reinvent it, but keep the actual test of "F1 selects option A" at this task's level since it's a pure widget/keyboard-simulation test (`tester.sendKeyEvent(LogicalKeyboardKey.f1)`).
- `QuestionNavigator`'s cell states (per master spec: answered/unanswered/current, "belgilangan"/flagged is optional — only add a flag/bookmark visual state if you also wire a real flag toggle somewhere in Task 4; otherwise omit "flagged" rather than building a control with no effect).

**Testing:** Widget tests per component — `QuestionCard` renders given text/image; `AnswerOption` renders selected/feedback states distinctly and calls `onSelect`; `AnswerOptionsGroup`'s F1-F4 keys genuinely select the right option (real `tester.sendKeyEvent`, not a mocked keyboard handler); `CountdownTimer` renders a given duration correctly (including a visually-distinct "low time" state if you add one — keep it simple); `QuestionNavigator` renders answered/current states and calls `onJump` with the right index.

- [ ] Steps 1-4.
- [ ] **Step 5: Commit.**
  ```bash
  git add app/lib/shared/widgets/ app/test/shared/widgets/
  git commit -m "feat(frontend): shared question-answering widgets (card, answer options, timer, navigator)"
  ```

---

### Task 3: Session data layer

**Files:** create `app/lib/features/session/data/{session_api,session_repository}.dart`, `app/lib/features/session/domain/{session_models,session_state}.dart`, matching tests; modify `app/lib/main.dart` (DI wiring).

**Interfaces (produced):**
```dart
// domain/session_models.dart — freezed, matching README's Sessiya section exactly
@freezed class SessionSummary with _$SessionSummary { ... }   // POST /sessions response shape
@freezed class AnswerResult with _$AnswerResult { ... }        // POST .../answers response shape
@freezed class SessionResult with _$SessionResult { ... }      // POST .../finish response shape
@freezed class SessionDetail with _$SessionDetail { ... }      // GET /sessions/{id} (resume) response shape

// data/session_api.dart
class SessionApi {
  Future<Result<SessionSummary>> start({required String mode, String? variantId, String? categoryId,
    String? signId, required String locale, int? count});
  Future<Result<AnswerResult>> answer({required String sessionId, required String questionId, required String answerId});
  Future<Result<SessionResult>> finish(String sessionId);
  Future<Result<SessionDetail>> get(String sessionId);
  Future<Result<List<VariantStatus>>> myVariants();          // GET /me/variants
}

// domain/session_state.dart
@freezed
sealed class SessionUiState with _$SessionUiState {
  const factory SessionUiState.loading() = SessionLoading;
  const factory SessionUiState.active({
    required SessionSummary summary,
    required int currentIndex,
    required Map<String, AnswerResult> answered,   // questionId -> result, in the order answered
    Duration? remaining,                            // exam mode only; null otherwise
  }) = SessionActive;
  const factory SessionUiState.stopped({required SessionSummary summary, required String stopReason}) = SessionStopped;
  const factory SessionUiState.finished({required SessionResult result}) = SessionFinished;
  const factory SessionUiState.error(Failure failure) = SessionError;
}
```

**Logic:**
- Map the README's error codes to distinct `Failure.code` values the UI reacts to specifically: `vip_required` (402), `daily_limit_reached` (429), `invalid_request`/`not_found`/`already_answered`/`invalid_answer`/`session_finished` — don't collapse these, Task 5/6/9 need to react differently to at least `vip_required` and `daily_limit_reached`.
- `SessionController` (presentation layer, built in Task 4 alongside the screen — NOT in this task, since this task is data-layer only) will own the actual exam countdown `Timer.periodic` and the "3rd error stops the exam" reaction to `AnswerResult.stopped`/`stopReason` — this task just needs `AnswerResult` to correctly surface those fields so the controller can react to them.
- Anti-cheat reminder: `AnswerResult.correct`/`correctAnswerId` are nullable in the domain model specifically because the backend withholds them in exam mode until the session finishes — model this as genuinely nullable, don't default them to a sentinel that could be mistaken for a real value.

**Testing:** Unit tests for `SessionApi` (parsing each endpoint's response shape, mapping each named error code to the right `Failure.code`) with a fake Dio adapter — no controller/UI in this task.

- [ ] Steps 1-4.
- [ ] **Step 5: DI wiring** in `main.dart` (shared `dio`).
- [ ] **Step 6: Commit.**
  ```bash
  git add app/lib/features/session/data/ app/lib/features/session/domain/ app/lib/main.dart app/test/features/session/data/
  git commit -m "feat(frontend): session data layer — start/answer/finish/resume across all 4 modes"
  ```

---

### Task 4: Session screen (test-taking UI) + controller

**Files:** create `app/lib/features/session/presentation/{session_controller,session_screen}.dart`, matching tests; modify `app/lib/app/router.dart` (add a session route, e.g. `/session/:id`).

**Interfaces (produced):**
```dart
final sessionControllerProvider = NotifierProvider.family<SessionController, SessionUiState, String /* sessionId, or a start-request token — see Logic */>(...);
// (exact family-arg shape is your call — could key by an already-started sessionId if the
// screen always receives one from a prior "start" call made by the caller, e.g. Variants/
// Practice/Mistakes screens; OR the controller itself could own the start() call given a
// mode+params request. Pick whichever avoids the seam-bug pattern most: a screen that
// mounts assuming a session already exists, when the caller forgot to actually start one
// first, is exactly the shape of bug this plan is watching for — make the contract between
// "whoever navigates here" and "this controller" impossible to get wrong silently, e.g. by
// having the controller's own build() take a StartRequest and call SessionApi.start()
// itself, so there's no "did the caller start it or not" ambiguity.)

class SessionController extends Notifier<SessionUiState> {
  Future<void> submitAnswer(String questionId, String answerId);
  Future<void> finish();
  void jumpTo(int index);        // exam mode navigator
}
```

**Logic:**
- Handle all 4 modes' real behavioral differences in ONE screen (per master spec, they share the same question-answering mechanics, differing only in chrome/rules):
  - `exam`: `CountdownTimer` ticking down from `time_limit_sec`, `QuestionNavigator` showing all 20, feedback WITHHELD on each answer (server sends `correct: null`), auto-stop+navigate-to-results on `stopped: true` (either `stop_reason: "too_many_errors"` after the 3rd wrong answer, or the timer hitting zero — client-side timer hitting zero should call `finish()`, matching the backend's own time-based stop).
  - `variant`/`practice`/`mistakes`: immediate feedback shown per answer (server sends real `correct`/`correct_answer_id`), no navigator grid needed (or a simpler one — your call, but don't force exam-only chrome onto these modes).
  - F1-F4 keyboard shortcuts active in all modes (use Task 2's `AnswerOptionsGroup`).
- **Apply this plan's core lesson explicitly here**: after `submitAnswer` succeeds and the session naturally reaches its last question (or `finish()` is called), the screen must actually navigate to the results screen (Task 5) — don't just update `SessionUiState.finished` and assume some other layer handles navigation; write a test that drives the REAL `SessionScreen` + REAL `SessionController` (fake only `SessionApi`) through a full mock session (answer all N questions) and asserts the app actually lands on the results route, not just that the state object is correct in isolation.
- Anti-cheat: never store/display a `correct`/`correctAnswerId` the server didn't actually send for the current mode — if `SessionUiState.active.answered[questionId].correct` is null in exam mode, the UI must render "no feedback yet," never infer/guess.

**Testing:** Widget tests covering: exam mode timer ticks and eventually triggers finish; exam mode withholds feedback on each answer; a 3rd wrong answer in exam mode triggers the stop-and-navigate-to-results path; variant/practice mode shows immediate feedback; F1-F4 selects and submits the right answer; the full-session-to-results navigation test described above (the seam test). Use a fake `SessionApi`/`SessionController` state sequence, not a real backend.

- [ ] Steps 1-4.
- [ ] **Step 5: Router.** Add the session route to `router.dart`.
- [ ] **Step 6: Commit.**
  ```bash
  git add app/lib/features/session/presentation/session_controller.dart app/lib/features/session/presentation/session_screen.dart app/lib/app/router.dart app/test/features/session/presentation/
  git commit -m "feat(frontend): session screen — exam timer/navigator, variant/practice/mistakes immediate feedback"
  ```

---

### Task 5: Session results screen + Variants (bilet) screen

**Files:** create `app/lib/features/session/presentation/session_results_screen.dart`, `app/lib/features/variants/presentation/{variants_screen,variants_controller}.dart`, matching tests; modify `app/lib/app/router.dart`, `app/lib/features/home/presentation/home_shell.dart` (wire the "variants" placeholder nav entry to the real screen).

**Interfaces (produced):**
```dart
final variantsControllerProvider = AsyncNotifierProvider<VariantsController, List<VariantStatus>>(...);
// fetches GET /me/variants
```

**Logic:**
- `SessionResultsScreen`: shows `SessionResult` (score/total, status passed/failed/abandoned, stop reason) — a real screen, not a placeholder, since results are core to every mode.
- `VariantsScreen`: a grid of bilets (per master spec: "bilet grid + mastery ring" concept from §16, though the mastery-ring visual specifically is Task 8's stats concern — here just render each bilet's unlock/lock state, best score, attempts, per `VariantStatus`), locked ones visually disabled, unlocked ones tappable → calls `SessionApi.start(mode: 'variant', variantId: ...)` → navigates to the session screen with the returned session id. Bilet #2+ tapped by a non-VIP user should surface the `vip_required` error from `SessionApi.start` distinctly (e.g. route to Task 9's VIP-required screen) rather than a generic error banner.
- Wire `HomeShell`'s "coming soon" variants placeholder (Plan 06 Task 8) to actually navigate to `VariantsScreen` now.

**Testing:** `VariantsController` unit test (mocked `SessionApi`/a variants-fetch API — success/failure). `VariantsScreen` widget test: locked bilets disabled/non-tappable; unlocked bilet tap calls `start()` and navigates; a VIP-gated bilet tap surfaces the specific `vip_required` path (the seam test — real screen + real controller, fake only the API). `SessionResultsScreen` widget test for each `status`/`stopReason` combination rendering sensibly.

- [ ] Steps 1-4.
- [ ] **Step 5: Commit.**
  ```bash
  git add app/lib/features/session/presentation/session_results_screen.dart app/lib/features/variants/ app/lib/app/router.dart app/lib/features/home/presentation/home_shell.dart app/test/features/session/presentation/session_results_screen_test.dart app/test/features/variants/
  git commit -m "feat(frontend): session results screen + variants (bilet) screen"
  ```

---

### Task 6: Practice setup + Signs catalog + Mistakes bank screens

**Files:** create `app/lib/features/practice/presentation/practice_setup_screen.dart`, `app/lib/features/signs/presentation/{signs_screen,signs_controller}.dart`, `app/lib/features/mistakes/presentation/mistakes_screen.dart`, matching tests; modify `app/lib/app/router.dart`, `app/lib/features/home/presentation/home_shell.dart` (wire "practice"/"mistakes" placeholders).

**Logic:**
- `PracticeSetupScreen`: pick exactly one of category-or-sign (per backend's "aynan bittasi" constraint — enforce this client-side too, don't just hope the backend rejects a bad request) + a count input, then `SessionApi.start(mode: 'practice', ...)` → session screen. On `daily_limit_reached` (429), show a specific "kunlik limitga yetdingiz" message (not a generic error) — per D13 this is expected/normal for free-tier users, not a bug, so the copy should read as informational, not alarming.
- `SignsScreen`: browsable catalog (`ContentApi.signs`, optional `group`/`query` filter) — per D13 this is explicitly **free-tier**, no VIP gating logic needed here at all.
- `MistakesScreen`: simple entry point — a count picker (default 10 per README) + start button calling `SessionApi.start(mode: 'mistakes', count: ...)`. Non-VIP tapping this should surface `vip_required` distinctly (same as variant #2+ in Task 5).
- Wire `HomeShell`'s "practice"/"mistakes" placeholders to these screens.

**Testing:** Widget tests per screen (mocked controllers/APIs): practice setup enforces the one-of category/sign rule and shows the daily-limit message distinctly on 429; signs catalog renders a fetched list and filters by group/query; mistakes screen surfaces `vip_required` distinctly for a non-VIP profile (seam test, real screen + real logic, fake API).

- [ ] Steps 1-4.
- [ ] **Step 5: Commit.**
  ```bash
  git add app/lib/features/practice/ app/lib/features/signs/ app/lib/features/mistakes/ app/lib/app/router.dart app/lib/features/home/presentation/home_shell.dart app/test/features/practice/ app/test/features/signs/ app/test/features/mistakes/
  git commit -m "feat(frontend): practice setup, signs catalog, mistakes bank screens"
  ```

---

### Task 7: Explanation rendering + feedback

**Files:** create `app/lib/features/explanation/data/explanation_api.dart`, `app/lib/features/explanation/domain/explanation_block.dart` (if not already fully covered by Task 1's `Question.explanation` field — check first, don't duplicate), `app/lib/features/explanation/presentation/{explanation_view,explanation_feedback_button}.dart`, matching tests; modify `app/lib/main.dart` (DI wiring for feedback POST), session screen/results screen (render explanations where applicable).

**Interfaces (produced):**
```dart
// data/explanation_api.dart — only the feedback endpoint is a real HTTP call
// (POST /explanations/feedback {question_id, helpful} -> {ok:true}, 404 not_found
// mapped to a distinct Failure.code); explanation CONTENT itself comes from
// Task 1's Question.explanation field (GET /questions/{id}), not a separate fetch.
class ExplanationApi {
  Future<Result<void>> feedback({required String questionId, required bool helpful});
}

class ExplanationView extends StatelessWidget {
  const ExplanationView({required this.blocks, this.legalRefs, super.key});
  // renders ordered blocks (intro/muhim/eslatma/ogohlantirish/maslahat/javob-tahlili/xulosa
  // per master spec §13) with distinct visual treatment per block type (a "MUHIM" callout
  // should look different from "ESLATMA" — check master spec's CalloutBlock concept),
  // sign-code references rendered as SignChip (Task 2... actually add SignChip here if
  // Task 2 didn't need it — check, don't duplicate)
}
```

**Logic:**
- **Explicit stub-awareness**: drafted-but-unverified explanations never reach the client at all (`GetVerifiedExplanation` only returns `verified` rows server-side — Plan 05's invariant), so the UI never needs to handle a "draft"/"AI-QORALAMA" state — if `Question.explanation` is present, it's real/verified content, render it plainly. Don't add speculative UI for draft/pending states that can never actually reach this client.
- Where to show it: in `variant`/`practice`/`mistakes` modes, show the explanation immediately after each answered question (inline, expandable, or on a per-question detail — your call, keep it simple for this foundation-level pass); in `exam` mode, explanations are naturally only relevant after `finish()` (anti-cheat — showing them mid-exam would leak correctness), so wire them into the results screen's per-question review instead, if such a review exists yet — if Task 5's results screen doesn't have a per-question breakdown, keep this task's exam-mode explanation display minimal (e.g. just note "javdob tahlili tugagandan keyin" / defer full per-question review to a later plan) rather than over-building a feature not yet scoped.
- `ExplanationFeedbackButton`: a simple 👍/👎-style toggle calling `ExplanationApi.feedback`, showing the 404 `not_found` case as "hali izoh mavjud emas" if it somehow occurs (shouldn't happen if the button is only shown when `Question.explanation` is non-null, but the API can still theoretically 404 — handle it, don't crash).

**Testing:** Widget tests for `ExplanationView` rendering each block type distinctly; `ExplanationFeedbackButton` calls the API with the right `helpful` value and shows a confirmation/error state.

- [ ] Steps 1-4.
- [ ] **Step 5: DI wiring** for `ExplanationApi`.
- [ ] **Step 6: Commit.**
  ```bash
  git add app/lib/features/explanation/ app/lib/main.dart app/lib/features/session/ app/test/features/explanation/
  git commit -m "feat(frontend): explanation rendering + feedback"
  ```

---

### Task 8: Saved questions + streak + stats

**Files:** create `app/lib/features/saved/{data,presentation}/*`, `app/lib/features/stats/{data,domain,presentation}/*`, matching tests; modify `app/lib/main.dart` (DI wiring), `app/lib/app/router.dart`, `app/lib/features/home/presentation/home_shell.dart` (wire "stats" placeholder + add a streak display + a save-toggle on `QuestionCard` usages).

**Interfaces (produced):**
```dart
// data/saved_api.dart — GET/POST/DELETE /me/saved
// data/progress_api.dart — GET /me/streak, GET /me/stats
@freezed class Streak with _$Streak { ... }   // current/best/todayDone/dailyGoal/lastActiveDate
@freezed class Stats with _$Stats { ... }     // categories:[{categoryCode, mastery, seen, correct}], readinessPct, dueCount

final savedControllerProvider = ...;   // list + toggle-save/unsave
final statsControllerProvider = AsyncNotifierProvider<StatsController, ({Streak streak, Stats stats})>(...);
```

**Logic:**
- Streak's `lastActiveDate` is UTC-day-based per the backend (README explicitly documents this can look "one day behind" around local midnight in UTC+5) — if you display it as a raw date, don't silently convert it to local time in a way that contradicts the backend's own documented semantics; if you show a relative label ("bugun"/"kecha"), compute that relative-ness using the SAME UTC-day logic the backend uses (reuse or mirror Plan 06's locale/date-handling conventions — check if there's already a UTC-day-truncation helper anywhere in `core/` before writing a new one).
- `StatsScreen`: `MasteryBar` (Task 2... check, add here if not already built) per category + a `ResultRing`-style readiness % + due count + the streak display. This is explicitly the last "coming soon" placeholder in `HomeShell` to become real.
- Save-toggle: add a bookmark-style icon button to `QuestionCard`/`AnswerOptionsGroup`'s usage sites (session screen, results screen) wired to `savedControllerProvider`, plus a dedicated `SavedScreen` listing everything saved (reachable from `StatsScreen` or `HomeShell` — your call on exact nav placement, just make it reachable).

**Testing:** `StatsController`/`SavedController` unit tests (mocked APIs, success/empty/error). `StatsScreen` widget test rendering categories/readiness/streak from fetched data. Save-toggle widget test: tapping it calls save/unsave and reflects the new state visually (seam test with the real toggle widget + real controller, fake API).

- [ ] Steps 1-4.
- [ ] **Step 5: DI wiring + router + HomeShell wiring.**
- [ ] **Step 6: Commit.**
  ```bash
  git add app/lib/features/saved/ app/lib/features/stats/ app/lib/main.dart app/lib/app/router.dart app/lib/features/home/presentation/home_shell.dart app/lib/shared/widgets/ app/test/features/saved/ app/test/features/stats/
  git commit -m "feat(frontend): saved questions, streak display, stats screen"
  ```

---

### Task 9: VIP-gating UI + free-limit UX polish

**Files:** create `app/lib/features/billing/presentation/vip_required_screen.dart`, matching tests; modify wherever `vip_required`/`daily_limit_reached` are currently shown as generic errors (Tasks 5/6) to route to/render this screen instead.

**Logic:**
- `VipRequiredScreen`: a static, honest M1-scoped upsell placeholder — no real billing exists yet (that's M2, D12: Payme/Click sandbox). Explain in Uzbek that this feature requires an active pass, note that pricing/purchase isn't available yet in this build, and offer a way back (not a dead end). This is NOT a fake checkout flow — don't build a payment form that goes nowhere, that would be a "half-built fake screen" of exactly the kind this plan's Task 8 (Plan 06) was careful to avoid for its own placeholders.
- Sweep every `SessionApi.start()` call site (variants, practice-adjacent mistakes-start, exam entry if one exists) to confirm `vip_required` routes here consistently, not handled ad-hoc differently per screen.

**Testing:** Widget test confirming the screen renders and its "go back" affordance works. A router/navigation test confirming at least one real gated action (e.g. tapping a locked-but-technically-unlockable bilet as a non-VIP profile) actually lands here end-to-end (seam test again).

- [ ] Steps 1-4.
- [ ] **Step 5: Commit.**
  ```bash
  git add app/lib/features/billing/ app/lib/app/router.dart app/lib/features/variants/ app/lib/features/mistakes/ app/test/features/billing/
  git commit -m "feat(frontend): VIP-required upsell screen, consistent 402 routing"
  ```

---

### Task 10: Event logging client

**Files:** create `app/lib/features/events/data/events_api.dart`, `app/lib/features/events/presentation/event_logger.dart`, matching tests; modify `app/lib/main.dart` (DI wiring), session screen (log `view_question`/`answer`/`session_finish` at minimum).

**Interfaces (produced):**
```dart
class EventsApi {
  Future<Result<int>> logBatch(List<ClientEvent> events);   // POST /events, returns count
}
@freezed class ClientEvent with _$ClientEvent { const factory ClientEvent({required String name, Map<String,dynamic>? props, DateTime? ts}) = _ClientEvent; }

class EventLogger {
  void log(String name, {Map<String,dynamic>? props});   // enqueues, does not block the caller
  Future<void> flush();                                    // sends whatever's queued (1-100 cap per backend)
}
final eventLoggerProvider = Provider<EventLogger>((ref) => ...);
```

**Logic:**
- Client-side batching: queue events locally, flush periodically (e.g. every N seconds via a `Timer.periodic`) AND on a natural boundary (session finish, app going to background if you want to be thorough — but don't over-build lifecycle handling not asked for; periodic + on-finish is enough for this foundation pass). Respect the backend's 1-100 batch cap — if the queue exceeds 100, split into multiple `logBatch` calls, don't just fail or silently drop events past 100.
- `log()` must never throw or block UI interaction — a failed flush should be swallowed (maybe logged to console in debug mode) and retried later, not surfaced as a user-facing error; event logging is not supposed to affect the learning experience if the network hiccups.
- Wire at minimum: `view_question` (when a question is displayed), `answer` (when one is submitted, with `props: {question_id, correct?}` — omit `correct` in exam mode since the client itself doesn't know it yet), `session_finish` (on `finish()`).

**Testing:** `EventLogger` unit tests: batches correctly, respects the 100-cap by splitting, swallows a failed flush without throwing and retries on the next flush cycle, `log()` returns immediately without waiting for the network call.

- [ ] Steps 1-4.
- [ ] **Step 5: DI wiring + session-screen call sites.**
- [ ] **Step 6: Commit.**
  ```bash
  git add app/lib/features/events/ app/lib/main.dart app/lib/features/session/ app/test/features/events/
  git commit -m "feat(frontend): client-side batch event logging"
  ```

---

### Task 11: Full verification + live test-flow check + docs

- [ ] **Step 1:** `cd "/home/sher/Рабочий стол/avtotest/app" && dart analyze` (0 issues), `flutter test --concurrency=1` (all pass), `flutter build web` (succeeds).

- [ ] **Step 2: Live verification** (mirrors Plan 06 Task 9's precedent — one-time, not CI-wired): start the backend (`make up` + `PORT=8090 go run ./cmd/api`), run a real `integration_test/variant_session_flow_test.dart` via `flutter drive` (same `chromedriver`+`-d web-server --browser-name=chrome` recipe Plan 06 Task 9 established — reuse it, don't rediscover it) driving: log in (reuse the auth flow) → open Variants → start bilet #1 (always free) → answer all 20 questions (mix of correct/incorrect) → view an explanation → toggle save on a question → finish the session → see results → check Stats screen reflects the attempt → check Streak incremented. If time-constrained, a manual pass is an acceptable fallback — but say so explicitly, with real commands/responses recorded, per this plan's established honesty standard.

- [ ] **Step 3: README.** Extend the "Flutter frontend" section (or add a new one) covering: which screens now exist (variants/practice/signs/mistakes/session/results/explanation/saved/stats), the VIP-gating UI behavior, event logging, and an explicit note on what's STILL not built (Grand Mock, real billing/checkout, admin — all later milestones).

- [ ] **Step 4: Commit.**
  ```bash
  git add README.md app/integration_test/
  git commit -m "docs: Flutter test-flow screens dev setup + live verification"
  ```

## Self-Review

1. **Spec coverage:** master spec §23 steps 9-11 (exam simulator UI, bilet/mashq/xatolar flows, explanation-render+AI-draft-awareness+saved+streak, free-limit+entitlement+event-logging) are covered by Tasks 3-10. §11's mode table (bilet/imtihon/mashq/xatolar) is fully represented in one shared session screen (Task 4), not 4 duplicated screens. §13's explanation block types are covered by Task 7. §15's shared widget list (`QuestionCard`, `AnswerOption`+F1-F4, `CountdownTimer`, `QuestionGrid`, `MasteryBar`, `ResultRing`, `SignChip`, `CalloutBlock`) is covered across Tasks 2/7/8.
2. **Placeholders:** `VipRequiredScreen` (Task 9) is an honest, non-dead-end upsell placeholder, explicitly not a fake checkout — called out the same way Plan 06 Task 8 called out its own placeholders. Grand Mock, real billing, admin are explicitly out of scope (later milestones), not silently missing pieces of this plan.
3. **Known scope boundary, not a gap:** per-question review-after-exam-finish (Task 7's note) is deliberately minimal if not already scoped by Task 5's results screen — don't over-build a feature this plan didn't fully spec just because it's adjacent.
4. **DI wiring discipline**: unlike Plan 06 (which needed two mid-execution patches), this plan decides up front that every new `*Api` reuses the single shared `Dio` and gets wired in the SAME task that introduces it — task reviewers should treat a `throw UnimplementedError` DI seam left unwired past its own task as a blocking finding, not a deferred one.
5. **Integration-seam discipline** (Plan 06's biggest recurring lesson): every task that wires a screen to a controller includes an explicit instruction to test them together, not just each in isolation — task reviewers should actively look for the shape of bug this produced repeatedly in Plan 06 (a real screen+controller pairing that's never actually driven together in any test) before approving.
