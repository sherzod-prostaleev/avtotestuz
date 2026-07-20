import 'package:freezed_annotation/freezed_annotation.dart';

import '../../../core/result.dart';
import 'session_models.dart';

part 'session_state.freezed.dart';

/// UI-facing state of an in-progress (or just-finished) test-taking session,
/// driven by `presentation/session_controller.dart` (Task 4 — not built by
/// this task, which is data-layer only).
@freezed
sealed class SessionUiState with _$SessionUiState {
  /// The session is being started (or resumed) — no [SessionSummary] yet.
  const factory SessionUiState.loading() = SessionLoading;

  /// A session is active: [summary] is the `POST /sessions` (or `GET
  /// /sessions/{id}` resume) result, [currentIndex] is the 0-based index
  /// into `summary.questionIds` the user is currently on, [answered] holds
  /// every [AnswerResult] recorded so far keyed by `questionId` (insertion
  /// order == answer order, since `Map` in Dart preserves insertion order).
  /// [remaining] is the exam-mode countdown (owned/ticked by
  /// `SessionController`'s `Timer.periodic`, NOT by this state class or by
  /// `CountdownTimer` — see Task 2's widget doc) — `null` in every
  /// non-`exam` mode.
  ///
  /// [pendingRetryQuestionId]/[pendingRetryAnswerId] carry the exact
  /// `questionId`/`answerId` pair of the most recent `submitAnswer` call
  /// whose API request failed (a transient network blip, not a genuinely
  /// unrecoverable failure). Deliberately kept on `SessionActive` itself
  /// rather than collapsing to `SessionUiState.error`: a single failed
  /// answer-submit must never strand the whole in-progress session (in
  /// exam mode that would burn a VIP-gated/daily-limited attempt over a
  /// momentary hiccup). Both are `null` whenever there is nothing to
  /// retry; `SessionController.retryPendingAnswer` reads them, clears them,
  /// and resubmits.
  const factory SessionUiState.active({
    required SessionSummary summary,
    required int currentIndex,
    required Map<String, AnswerResult> answered,
    Duration? remaining,
    String? pendingRetryQuestionId,
    String? pendingRetryAnswerId,
  }) = SessionActive;

  /// The session was stopped before naturally finishing (e.g. `exam`'s 3rd
  /// wrong answer, or a client-side timer hitting zero) but hasn't yet been
  /// confirmed via `finish()` — [stopReason] mirrors
  /// [AnswerResult.stopReason]/`stopped_reason` (e.g. `"too_many_errors"`,
  /// `"time_up"`).
  const factory SessionUiState.stopped({
    required SessionSummary summary,
    required String stopReason,
  }) = SessionStopped;

  /// The session finished (naturally or via a stop-then-finish) — [result]
  /// is `POST .../finish`'s response.
  const factory SessionUiState.finished({required SessionResult result}) =
      SessionFinished;

  /// The last operation (start/answer/finish/resume) failed. [failure]
  /// carries the backend's `error.code` untouched (e.g. `vip_required`,
  /// `daily_limit_reached`), so the UI can react to specific codes rather
  /// than showing a generic error banner.
  const factory SessionUiState.error(Failure failure) = SessionError;
}
