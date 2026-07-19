import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../app/locale/locale_provider.dart';
import '../../../core/result.dart';
import '../../content/data/content_api.dart';
import '../../content/domain/question.dart';
import '../data/session_api.dart';
import '../domain/session_state.dart';

/// Everything the caller must supply to *start* a session — the exact same
/// argument set `SessionApi.start` takes, minus `locale` (which the
/// controller reads from `localeProvider` itself so a caller can never pass a
/// stale/wrong locale).
///
/// This is deliberately the family key for [sessionControllerProvider] (see
/// its doc for the "why"): the controller is keyed by *how to start a
/// session*, not by an already-existing `sessionId`. Value-equality
/// (hand-written [==]/[hashCode], since freezed here would need its own
/// part-file/codegen for a tiny presentation-only type) makes it a valid
/// Riverpod family key.
class SessionStartRequest {
  const SessionStartRequest({
    required this.mode,
    this.variantId,
    this.categoryId,
    this.signId,
    this.count,
  });

  /// One of `variant`/`exam`/`practice`/`mistakes`.
  final String mode;
  final String? variantId;
  final String? categoryId;
  final String? signId;
  final int? count;

  @override
  bool operator ==(Object other) =>
      other is SessionStartRequest &&
      other.mode == mode &&
      other.variantId == variantId &&
      other.categoryId == categoryId &&
      other.signId == signId &&
      other.count == count;

  @override
  int get hashCode => Object.hash(mode, variantId, categoryId, signId, count);

  @override
  String toString() =>
      'SessionStartRequest(mode: $mode, variantId: $variantId, '
      'categoryId: $categoryId, signId: $signId, count: $count)';
}

/// Drives one test-taking session's UI-facing [SessionUiState].
///
/// **Design decision — the controller owns `start()`.** The provider is a
/// `family` keyed by a [SessionStartRequest] (a mode + params bundle), NOT by
/// an already-created `sessionId`. When the controller is first read its
/// [build] immediately calls `SessionApi.start(...)` itself. This is the shape
/// the plan explicitly steers toward: there is no window in which a screen can
/// mount "assuming a session already exists" while the caller forgot to
/// actually start one — mounting the screen *is* starting the session, so the
/// "did the caller start it or not?" ambiguity that produced silent
/// seam-bugs in Plan 06 cannot exist here. Callers (Variants / Practice /
/// Mistakes screens, Tasks 5/6) just build a [SessionStartRequest] and
/// navigate; they never call `start()` themselves, so `vip_required` /
/// `daily_limit_reached` surface through [SessionUiState.error] in exactly
/// one place rather than being re-implemented per entry screen.
///
/// Owns the exam-mode countdown `Timer.periodic` (decrementing
/// [SessionActive.remaining] once a second, calling [finish] at zero) and the
/// "server said stop" reaction (`AnswerResult.stopped` → [finish], e.g. the
/// 3rd wrong answer in exam mode). Non-exam modes have no timer and no
/// withheld feedback.
class SessionController extends Notifier<SessionUiState> {
  SessionController(this.request);

  /// The family argument, captured via the constructor (Riverpod 3.x passes
  /// the family key to the notifier factory, not to [build]).
  final SessionStartRequest request;

  Timer? _timer;

  @override
  SessionUiState build() {
    ref.onDispose(() => _timer?.cancel());
    _start();
    return const SessionUiState.loading();
  }

  Future<void> _start() async {
    final api = ref.read(sessionApiProvider);
    final locale = localeToBackendCode(ref.read(localeProvider));
    final result = await api.start(
      mode: request.mode,
      variantId: request.variantId,
      categoryId: request.categoryId,
      signId: request.signId,
      locale: locale,
      count: request.count,
    );
    if (!ref.mounted) return;
    switch (result) {
      case Ok(:final data):
        final exam = data.mode == 'exam';
        state = SessionUiState.active(
          summary: data,
          currentIndex: 0,
          answered: const {},
          remaining: exam ? Duration(seconds: data.timeLimitSec) : null,
        );
        if (exam) _startTimer();
      case Err(:final failure):
        state = SessionUiState.error(failure);
    }
  }

  /// Exam-mode countdown. Ticks once a second; when it would cross zero it
  /// cancels itself and calls [finish] — matching the backend's own
  /// time-based `time_up` stop, so the client and server agree on when an
  /// exam ends rather than the client silently running past a server-expired
  /// session.
  void _startTimer() {
    _timer?.cancel();
    _timer = Timer.periodic(const Duration(seconds: 1), (timer) {
      final current = state;
      if (current is! SessionActive || current.remaining == null) {
        timer.cancel();
        return;
      }
      final next = current.remaining! - const Duration(seconds: 1);
      if (next <= Duration.zero) {
        timer.cancel();
        state = current.copyWith(remaining: Duration.zero);
        finish();
      } else {
        state = current.copyWith(remaining: next);
      }
    });
  }

  /// Records an answer for [questionId]. No-op if the session isn't active or
  /// this question was already answered (so a stray F-key / double-tap after
  /// answering can't re-submit). In exam mode a `stopped: true` response
  /// (e.g. the 3rd wrong answer) triggers an automatic [finish].
  Future<void> submitAnswer(String questionId, String answerId) async {
    final current = state;
    if (current is! SessionActive) return;
    if (current.answered.containsKey(questionId)) return;

    final api = ref.read(sessionApiProvider);
    final result = await api.answer(
      sessionId: current.summary.id,
      questionId: questionId,
      answerId: answerId,
    );
    if (!ref.mounted) return;
    final afterCall = state;
    if (afterCall is! SessionActive) return;

    switch (result) {
      case Ok(:final data):
        state = afterCall.copyWith(
          answered: {...afterCall.answered, questionId: data},
        );
        if (data.stopped) {
          // Server stopped the session (too many errors / time). Collapse
          // straight to finish() — there is no distinct stopped-vs-finished
          // UI in this screen, so we don't dwell on SessionUiState.stopped
          // (it flickers to results anyway); we go straight to the finished
          // state that the screen navigates on.
          await finish();
        }
      case Err(:final failure):
        state = SessionUiState.error(failure);
    }
  }

  /// Moves the exam navigator (or any mode's "next"/"previous") to [index].
  /// Clamped to the question range; a no-op outside an active session.
  void jumpTo(int index) {
    final current = state;
    if (current is! SessionActive) return;
    if (index < 0 || index >= current.summary.questionIds.length) return;
    state = current.copyWith(currentIndex: index);
  }

  /// Finishes the session (cancelling any exam timer) and moves to
  /// [SessionUiState.finished] with the server's result — the state the
  /// screen watches to navigate to the results route.
  Future<void> finish() async {
    _timer?.cancel();
    final current = state;
    final sessionId = switch (current) {
      SessionActive(:final summary) => summary.id,
      SessionStopped(:final summary) => summary.id,
      _ => null,
    };
    if (sessionId == null) return;

    final api = ref.read(sessionApiProvider);
    final result = await api.finish(sessionId);
    if (!ref.mounted) return;
    switch (result) {
      case Ok(:final data):
        state = SessionUiState.finished(result: data);
      case Err(:final failure):
        state = SessionUiState.error(failure);
    }
  }
}

/// The session controller, keyed by *how to start* the session (see
/// [SessionController]'s doc). `autoDispose` so a finished/abandoned session's
/// controller (and its timer) is torn down once the screen is popped, and a
/// fresh attempt at the same request starts a genuinely new session rather
/// than resurrecting a finished one.
final sessionControllerProvider = NotifierProvider.autoDispose
    .family<SessionController, SessionUiState, SessionStartRequest>(
      SessionController.new,
    );

/// Fetches a single question's full content (text/answers/image) for display.
/// Kept separate from [sessionControllerProvider] because
/// [SessionUiState.active] carries only `question_ids` (from
/// `POST /sessions`) — the actual question bodies come from the content API,
/// a different network boundary, fetched lazily per question as the user
/// reaches it.
///
/// Throws (→ [AsyncError]) on failure so the screen can show a retry affordance
/// rather than silently rendering an empty question.
final sessionQuestionProvider = FutureProvider.autoDispose
    .family<Question, String>((ref, questionId) async {
      final api = ref.read(contentApiProvider);
      final locale = localeToBackendCode(ref.read(localeProvider));
      final result = await api.question(questionId, locale: locale);
      return switch (result) {
        Ok(:final data) => data,
        Err(:final failure) => throw Exception(failure.message),
      };
    });
