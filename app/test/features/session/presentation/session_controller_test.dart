import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/content/data/content_api.dart';
import 'package:avtotest_app/features/session/data/session_api.dart';
import 'package:avtotest_app/features/session/domain/session_models.dart';
import 'package:avtotest_app/features/session/domain/session_state.dart';
import 'package:avtotest_app/features/session/presentation/session_controller.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'session_fakes.dart';

/// Lets a fire-and-forget async (`_start`, `submitAnswer`, `finish`) settle.
Future<void> _flush() async {
  await Future<void>.delayed(Duration.zero);
  await Future<void>.delayed(Duration.zero);
}

ProviderContainer _container(SessionApi sessionApi, {ContentApi? contentApi}) {
  final container = ProviderContainer(
    overrides: [
      sessionApiProvider.overrideWithValue(sessionApi),
      contentApiProvider.overrideWithValue(contentApi ?? FakeContentApi()),
    ],
  );
  return container;
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    // The controller reads `localeProvider`, whose Notifier hydrates from
    // SharedPreferences — mock it so that hydration resolves cleanly.
    SharedPreferences.setMockInitialValues({});
  });

  test('build() immediately starts the session and reaches active', () async {
    final api = FakeSessionApi(
      startResult: fakeSummary(mode: 'variant', ids: ['q1', 'q2']),
    );
    final container = _container(api);
    addTearDown(container.dispose);
    const req = SessionStartRequest(mode: 'variant');
    container.listen(sessionControllerProvider(req), (_, _) {},
        fireImmediately: true);

    await _flush();

    final state = container.read(sessionControllerProvider(req));
    expect(state, isA<SessionActive>());
    final active = state as SessionActive;
    expect(active.summary.questionIds, ['q1', 'q2']);
    expect(active.currentIndex, 0);
    // Non-exam mode has no countdown.
    expect(active.remaining, isNull);
    expect(api.startCalls.single.mode, 'variant');
    // Locale is filled in by the controller from localeProvider, not the
    // caller — proving the caller can't pass a stale one.
    expect(api.startCalls.single.locale, 'uz-Latn');
  });

  test('exam mode seeds remaining from time_limit_sec', () async {
    final api = FakeSessionApi(
      startResult: fakeSummary(mode: 'exam', ids: ['q1'], timeLimitSec: 1800),
    );
    final container = _container(api);
    addTearDown(container.dispose);
    const req = SessionStartRequest(mode: 'exam');
    container.listen(sessionControllerProvider(req), (_, _) {},
        fireImmediately: true);

    await _flush();

    final active =
        container.read(sessionControllerProvider(req)) as SessionActive;
    expect(active.remaining, const Duration(seconds: 1800));
  });

  test('a vip_required failure on start surfaces as a distinct error code',
      () async {
    final api = FakeSessionApi(
      startFailure: const Failure(code: 'vip_required', message: 'Obuna kerak'),
    );
    final container = _container(api);
    addTearDown(container.dispose);
    const req = SessionStartRequest(mode: 'variant', variantId: 'v2');
    container.listen(sessionControllerProvider(req), (_, _) {},
        fireImmediately: true);

    await _flush();

    final state = container.read(sessionControllerProvider(req));
    expect(state, isA<SessionError>());
    expect((state as SessionError).failure.code, 'vip_required');
  });

  test('submitAnswer records the AnswerResult under its questionId', () async {
    final api = FakeSessionApi(
      startResult: fakeSummary(mode: 'variant', ids: ['q1', 'q2']),
      answerFn: (_, _) =>
          const AnswerResult(recorded: true, correct: true, correctAnswerId: 'x'),
    );
    final container = _container(api);
    addTearDown(container.dispose);
    const req = SessionStartRequest(mode: 'variant');
    container.listen(sessionControllerProvider(req), (_, _) {},
        fireImmediately: true);
    await _flush();

    await container
        .read(sessionControllerProvider(req).notifier)
        .submitAnswer('q1', 'q1-a1');
    await _flush();

    final active =
        container.read(sessionControllerProvider(req)) as SessionActive;
    expect(active.answered.containsKey('q1'), isTrue);
    expect(active.answered['q1']!.correct, isTrue);
    expect(api.answerCalls.single.questionId, 'q1');
  });

  test('answering the same question twice does not re-submit', () async {
    final api = FakeSessionApi(
      startResult: fakeSummary(mode: 'variant', ids: ['q1']),
    );
    final container = _container(api);
    addTearDown(container.dispose);
    const req = SessionStartRequest(mode: 'variant');
    container.listen(sessionControllerProvider(req), (_, _) {},
        fireImmediately: true);
    await _flush();

    final notifier = container.read(sessionControllerProvider(req).notifier);
    await notifier.submitAnswer('q1', 'q1-a1');
    await _flush();
    await notifier.submitAnswer('q1', 'q1-a2');
    await _flush();

    expect(api.answerCalls.length, 1);
  });

  test('a stopped:true answer (e.g. 3rd exam error) auto-finishes', () async {
    final api = FakeSessionApi(
      startResult: fakeSummary(mode: 'exam', ids: ['q1', 'q2', 'q3'],
          timeLimitSec: 1800),
      answerFn: (_, _) => const AnswerResult(
        recorded: true,
        stopped: true,
        stopReason: 'too_many_errors',
      ),
      finishResult: const SessionResult(
        status: 'failed',
        stoppedReason: 'too_many_errors',
        score: 0,
        total: 3,
      ),
    );
    final container = _container(api);
    addTearDown(container.dispose);
    const req = SessionStartRequest(mode: 'exam');
    container.listen(sessionControllerProvider(req), (_, _) {},
        fireImmediately: true);
    await _flush();

    await container
        .read(sessionControllerProvider(req).notifier)
        .submitAnswer('q1', 'q1-a1');
    await _flush();

    final state = container.read(sessionControllerProvider(req));
    expect(state, isA<SessionFinished>());
    expect((state as SessionFinished).result.stoppedReason, 'too_many_errors');
    expect(api.finishCalls, 1);
  });

  test('jumpTo moves currentIndex and clamps out-of-range jumps', () async {
    final api = FakeSessionApi(
      startResult: fakeSummary(mode: 'exam', ids: ['q1', 'q2', 'q3'],
          timeLimitSec: 1800),
    );
    final container = _container(api);
    addTearDown(container.dispose);
    const req = SessionStartRequest(mode: 'exam');
    container.listen(sessionControllerProvider(req), (_, _) {},
        fireImmediately: true);
    await _flush();

    final notifier = container.read(sessionControllerProvider(req).notifier);

    notifier.jumpTo(2);
    expect(
      (container.read(sessionControllerProvider(req)) as SessionActive)
          .currentIndex,
      2,
    );

    notifier.jumpTo(99); // ignored
    notifier.jumpTo(-1); // ignored
    expect(
      (container.read(sessionControllerProvider(req)) as SessionActive)
          .currentIndex,
      2,
    );
  });

  test('finish() moves to finished with the server result', () async {
    final api = FakeSessionApi(
      startResult: fakeSummary(mode: 'variant', ids: ['q1']),
      finishResult: const SessionResult(
        status: 'passed',
        stoppedReason: 'completed',
        score: 1,
        total: 1,
      ),
    );
    final container = _container(api);
    addTearDown(container.dispose);
    const req = SessionStartRequest(mode: 'variant');
    container.listen(sessionControllerProvider(req), (_, _) {},
        fireImmediately: true);
    await _flush();

    await container.read(sessionControllerProvider(req).notifier).finish();
    await _flush();

    final state = container.read(sessionControllerProvider(req));
    expect(state, isA<SessionFinished>());
    expect((state as SessionFinished).result.status, 'passed');
  });

  test(
    'a transient submitAnswer failure keeps the session active (not error) '
    'and surfaces a pending retry; retryPendingAnswer resubmits and succeeds',
    () async {
      final api = FakeSessionApi(
        startResult: fakeSummary(mode: 'variant', ids: ['q1', 'q2']),
        answerFailureCount: 1,
        answerFn: (_, _) => const AnswerResult(
          recorded: true,
          correct: true,
          correctAnswerId: 'q1-a1',
        ),
      );
      final container = _container(api);
      addTearDown(container.dispose);
      const req = SessionStartRequest(mode: 'variant');
      container.listen(sessionControllerProvider(req), (_, _) {},
          fireImmediately: true);
      await _flush();

      final notifier = container.read(sessionControllerProvider(req).notifier);
      await notifier.submitAnswer('q1', 'q1-a1');
      await _flush();

      // First call failed: the session must stay active, NOT collapse to
      // SessionUiState.error (that would strand the whole attempt over a
      // transient network blip).
      final afterFailure = container.read(sessionControllerProvider(req));
      expect(afterFailure, isA<SessionActive>());
      final activeAfterFailure = afterFailure as SessionActive;
      expect(activeAfterFailure.answered.containsKey('q1'), isFalse);
      // A retry affordance for the EXACT failed pair is present.
      expect(activeAfterFailure.pendingRetryQuestionId, 'q1');
      expect(activeAfterFailure.pendingRetryAnswerId, 'q1-a1');
      expect(api.answerCalls.length, 1);

      // Tapping retry resubmits the same questionId/answerId and this time
      // the API call succeeds.
      await notifier.retryPendingAnswer();
      await _flush();

      final afterRetry = container.read(sessionControllerProvider(req));
      expect(afterRetry, isA<SessionActive>());
      final activeAfterRetry = afterRetry as SessionActive;
      expect(activeAfterRetry.answered.containsKey('q1'), isTrue);
      expect(activeAfterRetry.answered['q1']!.correct, isTrue);
      // Pending retry is cleared once the resubmit succeeds.
      expect(activeAfterRetry.pendingRetryQuestionId, isNull);
      expect(activeAfterRetry.pendingRetryAnswerId, isNull);
      expect(api.answerCalls.length, 2);
      expect(api.answerCalls.last.questionId, 'q1');
      expect(api.answerCalls.last.answerId, 'q1-a1');
    },
  );

  test('SessionStartRequest value-equality makes it a stable family key', () {
    const a = SessionStartRequest(mode: 'variant', variantId: 'v1');
    const b = SessionStartRequest(mode: 'variant', variantId: 'v1');
    const c = SessionStartRequest(mode: 'variant', variantId: 'v2');
    expect(a, b);
    expect(a.hashCode, b.hashCode);
    expect(a, isNot(c));
  });
}
