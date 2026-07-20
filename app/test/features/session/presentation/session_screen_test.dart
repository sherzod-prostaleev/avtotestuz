import 'package:avtotest_app/app/l10n/app_localizations.dart';
import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/billing/presentation/vip_required_screen.dart';
import 'package:avtotest_app/features/content/data/content_api.dart';
import 'package:avtotest_app/features/session/data/session_api.dart';
import 'package:avtotest_app/features/session/domain/session_models.dart';
import 'package:avtotest_app/features/session/presentation/session_controller.dart';
import 'package:avtotest_app/features/session/presentation/session_screen.dart';
import 'package:avtotest_app/shared/widgets/answer_option.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'session_fakes.dart';

/// Builds a real router with `/session` (the screen under test, given
/// [request] directly) and the real results route reading its
/// [SessionResult] from `extra` — the same wiring `app/lib/app/router.dart`
/// uses. Only the two network APIs are faked; the controller, screen and
/// router are all real.
Widget _wrap({
  required FakeSessionApi sessionApi,
  required FakeContentApi contentApi,
  required SessionStartRequest request,
}) {
  final router = GoRouter(
    initialLocation: '/session',
    routes: [
      GoRoute(
        path: '/',
        builder: (context, state) =>
            const Scaffold(body: Text('home', key: Key('home'))),
      ),
      GoRoute(
        path: '/session',
        builder: (context, state) => SessionScreen(request: request),
      ),
      GoRoute(
        path: sessionResultRoute,
        builder: (context, state) =>
            SessionResultView(result: state.extra! as SessionResult),
      ),
      GoRoute(
        path: vipRequiredRoute,
        builder: (context, state) => const VipRequiredScreen(),
      ),
    ],
  );
  return ProviderScope(
    overrides: [
      sessionApiProvider.overrideWithValue(sessionApi),
      contentApiProvider.overrideWithValue(contentApi),
    ],
    child: MaterialApp.router(
      // Pinned explicitly — without a `locale:`, Flutter's default
      // locale-resolution fallback picks `ru`, not `uz`, and these assertions
      // are uz-Latn ARB-string specific (see home_shell_test.dart's same
      // note).
      locale: const Locale('uz'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      routerConfig: router,
    ),
  );
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  testWidgets('shows a loading indicator while the session starts', (
    tester,
  ) async {
    final api = FakeSessionApi(
      startResult: fakeSummary(mode: 'variant', ids: ['q1']),
    );
    await tester.pumpWidget(
      _wrap(
        sessionApi: api,
        contentApi: FakeContentApi(),
        request: const SessionStartRequest(mode: 'variant'),
      ),
    );
    // First frame: start() hasn't resolved yet.
    expect(find.byKey(const Key('session-loading')), findsOneWidget);

    await tester.pumpAndSettle();
    expect(find.byKey(const Key('session-loading')), findsNothing);
  });

  testWidgets('renders the first question and its four options', (
    tester,
  ) async {
    final api = FakeSessionApi(
      startResult: fakeSummary(mode: 'variant', ids: ['q1', 'q2']),
    );
    await tester.pumpWidget(
      _wrap(
        sessionApi: api,
        contentApi: FakeContentApi(),
        request: const SessionStartRequest(mode: 'variant'),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Savol matni q1'), findsOneWidget);
    expect(find.text('Savol 1 / 2'), findsOneWidget);
    expect(find.byType(AnswerOption), findsNWidgets(4));
  });

  testWidgets(
    'variant mode shows immediate correct/incorrect feedback after answering',
    (tester) async {
      // The user picks a2, which is wrong; a3 is the correct one.
      final api = FakeSessionApi(
        startResult: fakeSummary(mode: 'variant', ids: ['q1']),
        answerFn: (questionId, answerId) => AnswerResult(
          recorded: true,
          correct: answerId == '$questionId-a3',
          correctAnswerId: '$questionId-a3',
        ),
      );
      await tester.pumpWidget(
        _wrap(
          sessionApi: api,
          contentApi: FakeContentApi(),
          request: const SessionStartRequest(mode: 'variant'),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const ValueKey('answer-option-B')));
      await tester.pumpAndSettle();

      // Chosen wrong option → incorrect icon; the real correct one → shown.
      expect(find.byIcon(Icons.cancel), findsOneWidget);
      expect(find.byIcon(Icons.check_circle_outline), findsOneWidget);
    },
  );

  testWidgets(
    'F2 selects and submits the SECOND option (F1-F4 wired in the real screen)',
    (tester) async {
      final api = FakeSessionApi(
        startResult: fakeSummary(mode: 'variant', ids: ['q1']),
      );
      await tester.pumpWidget(
        _wrap(
          sessionApi: api,
          contentApi: FakeContentApi(),
          request: const SessionStartRequest(mode: 'variant'),
        ),
      );
      await tester.pumpAndSettle();

      await tester.sendKeyEvent(LogicalKeyboardKey.f2);
      await tester.pumpAndSettle();

      expect(api.answerCalls.single.answerId, 'q1-a2');
    },
  );

  testWidgets('exam mode WITHHOLDS feedback (server sends correct:null)', (
    tester,
  ) async {
    final api = FakeSessionApi(
      startResult: fakeSummary(mode: 'exam', ids: ['q1', 'q2'],
          timeLimitSec: 1800),
      // Exam: the server withholds correctness.
      answerFn: (_, _) => const AnswerResult(recorded: true),
    );
    await tester.pumpWidget(
      _wrap(
        sessionApi: api,
        contentApi: FakeContentApi(),
        request: const SessionStartRequest(mode: 'exam'),
      ),
    );
    // Exam mode runs a periodic timer → pumpAndSettle would never settle;
    // pump explicitly instead.
    await tester.pump(); // start resolves
    await tester.pump(); // question loads

    await tester.tap(find.byKey(const ValueKey('answer-option-A')));
    await tester.pump();

    // No correctness icons anywhere — the UI never guesses what was withheld.
    expect(find.byIcon(Icons.check_circle), findsNothing);
    expect(find.byIcon(Icons.cancel), findsNothing);
    expect(find.byIcon(Icons.check_circle_outline), findsNothing);
    // The countdown chrome is present in exam mode.
    expect(find.byKey(const ValueKey('countdown-timer-label')), findsOneWidget);
    // The answer was still recorded.
    expect(api.answerCalls.single.questionId, 'q1');
  });

  testWidgets('exam timer reaching zero finishes and navigates to results', (
    tester,
  ) async {
    final api = FakeSessionApi(
      startResult: fakeSummary(mode: 'exam', ids: ['q1'], timeLimitSec: 3),
      finishResult: const SessionResult(
        status: 'failed',
        stoppedReason: 'time_up',
        score: 0,
        total: 1,
      ),
    );
    await tester.pumpWidget(
      _wrap(
        sessionApi: api,
        contentApi: FakeContentApi(),
        request: const SessionStartRequest(mode: 'exam'),
      ),
    );
    await tester.pump(); // start resolves
    await tester.pump(); // question loads

    // Exhaust the 3-second limit.
    await tester.pump(const Duration(seconds: 1));
    await tester.pump(const Duration(seconds: 1));
    await tester.pump(const Duration(seconds: 1));
    // Timer is cancelled inside finish(), so settling is safe now.
    await tester.pumpAndSettle();

    expect(api.finishCalls, 1);
    expect(find.byKey(const Key('session-result-view')), findsOneWidget);
    expect(find.text('Vaqt tugadi'), findsOneWidget);
  });

  testWidgets(
    'exam 3rd-wrong stop (stopped:true) auto-finishes to the results route',
    (tester) async {
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
      await tester.pumpWidget(
        _wrap(
          sessionApi: api,
          contentApi: FakeContentApi(),
          request: const SessionStartRequest(mode: 'exam'),
        ),
      );
      await tester.pump();
      await tester.pump();

      await tester.tap(find.byKey(const ValueKey('answer-option-A')));
      await tester.pumpAndSettle(); // stop→finish cancels the timer

      expect(find.byKey(const Key('session-result-view')), findsOneWidget);
      expect(find.text('Xatolar soni ko\'payib ketdi'), findsOneWidget);
    },
  );

  testWidgets(
    'SEAM TEST: real screen + real controller + real router — answering every '
    'question drives an actual navigation to the results route (not just a '
    'finished state object)',
    (tester) async {
      final api = FakeSessionApi(
        startResult: fakeSummary(mode: 'variant', ids: ['q1', 'q2']),
        answerFn: (questionId, answerId) => AnswerResult(
          recorded: true,
          correct: true,
          correctAnswerId: answerId,
        ),
        finishResult: const SessionResult(
          status: 'passed',
          stoppedReason: 'completed',
          score: 2,
          total: 2,
        ),
      );
      await tester.pumpWidget(
        _wrap(
          sessionApi: api,
          contentApi: FakeContentApi(),
          request: const SessionStartRequest(mode: 'variant', variantId: 'v1'),
        ),
      );
      await tester.pumpAndSettle();

      // Question 1: answer, then advance.
      expect(find.text('Savol 1 / 2'), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('answer-option-A')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('session-next-button')));
      await tester.pumpAndSettle();

      // Question 2 (last): answer, then finish.
      expect(find.text('Savol 2 / 2'), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('answer-option-A')));
      await tester.pumpAndSettle();
      expect(find.text('Yakunlash'), findsOneWidget);
      await tester.tap(find.byKey(const Key('session-next-button')));
      await tester.pumpAndSettle();

      // The crux: the app actually LANDED on the results route.
      expect(api.finishCalls, 1);
      expect(find.byKey(const Key('session-result-view')), findsOneWidget);
      expect(find.byKey(const Key('session-result-score')), findsOneWidget);
      expect(find.text('2 / 2'), findsOneWidget);
    },
  );

  testWidgets(
    'a vip_required start error navigates to the dedicated VipRequiredScreen '
    '(a paywall-style screen), NOT an inline session error view',
    (tester) async {
      final api = FakeSessionApi(
        startFailure: const Failure(code: 'vip_required', message: 'raw'),
      );
      await tester.pumpWidget(
        _wrap(
          sessionApi: api,
          contentApi: FakeContentApi(),
          request: const SessionStartRequest(mode: 'variant', variantId: 'v2'),
        ),
      );
      await tester.pumpAndSettle();

      // Landed on the real VIP screen — a distinct route, not inline copy on
      // the session screen.
      expect(find.byKey(const Key('vip-required-screen')), findsOneWidget);
      expect(find.byKey(const Key('session-error-view')), findsNothing);
      expect(
        find.text('Bu bo\'lim faqat obunachilar uchun'),
        findsOneWidget,
      );
    },
  );

  testWidgets(
    'a daily_limit_reached start error stays an inline message on the session '
    'screen (a different KIND of block than vip_required)',
    (tester) async {
      final api = FakeSessionApi(
        startFailure: const Failure(
          code: 'daily_limit_reached',
          message: 'raw',
        ),
      );
      await tester.pumpWidget(
        _wrap(
          sessionApi: api,
          contentApi: FakeContentApi(),
          request: const SessionStartRequest(mode: 'practice'),
        ),
      );
      await tester.pumpAndSettle();

      // Inline — NOT the VIP screen.
      expect(find.byKey(const Key('session-error-view')), findsOneWidget);
      expect(find.byKey(const Key('vip-required-screen')), findsNothing);
      expect(find.textContaining('bepul limitga'), findsOneWidget);
    },
  );

  testWidgets('the Next button is disabled until the question is answered', (
    tester,
  ) async {
    final api = FakeSessionApi(
      startResult: fakeSummary(mode: 'variant', ids: ['q1', 'q2']),
    );
    await tester.pumpWidget(
      _wrap(
        sessionApi: api,
        contentApi: FakeContentApi(),
        request: const SessionStartRequest(mode: 'variant'),
      ),
    );
    await tester.pumpAndSettle();

    final button = tester.widget<FilledButton>(
      find.byKey(const Key('session-next-button')),
    );
    expect(button.onPressed, isNull);
  });

  testWidgets(
    'SEAM TEST: a transient answer-submit failure keeps the REAL screen on '
    'the active session (no full-screen error) and shows a working retry '
    'banner that resubmits the same answer',
    (tester) async {
      final api = FakeSessionApi(
        startResult: fakeSummary(mode: 'variant', ids: ['q1', 'q2']),
        answerFailureCount: 1,
        answerFn: (questionId, answerId) => AnswerResult(
          recorded: true,
          correct: true,
          correctAnswerId: answerId,
        ),
      );
      await tester.pumpWidget(
        _wrap(
          sessionApi: api,
          contentApi: FakeContentApi(),
          request: const SessionStartRequest(mode: 'variant'),
        ),
      );
      await tester.pumpAndSettle();

      // First tap fails on the network. The screen must stay on the active
      // question — never the full-screen SessionError view — and show the
      // retry banner instead.
      await tester.tap(find.byKey(const ValueKey('answer-option-A')));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('session-error-view')), findsNothing);
      expect(find.text('Savol 1 / 2'), findsOneWidget);
      expect(find.byKey(const Key('answer-retry-banner')), findsOneWidget);
      expect(api.answerCalls.length, 1);
      // Not yet recorded as answered — the Next/Finish button stays disabled.
      final button = tester.widget<FilledButton>(
        find.byKey(const Key('session-next-button')),
      );
      expect(button.onPressed, isNull);

      // Tapping retry resubmits the exact same questionId/answerId, and this
      // time the API call succeeds.
      await tester.tap(find.byKey(const Key('answer-retry-button')));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('answer-retry-banner')), findsNothing);
      expect(api.answerCalls.length, 2);
      expect(api.answerCalls.last.questionId, 'q1');
      expect(api.answerCalls.last.answerId, 'q1-a1');
      // Feedback now renders — the retried answer was actually recorded.
      expect(find.byIcon(Icons.check_circle), findsOneWidget);
    },
  );
}
