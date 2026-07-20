import 'package:avtotest_app/app/l10n/app_localizations.dart';
import 'package:avtotest_app/features/session/domain/session_models.dart';
import 'package:avtotest_app/features/session/presentation/session_results_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

Widget _wrap(SessionResult result) {
  return MaterialApp(
    // Pinned explicitly — without a `locale:`, Flutter's default
    // locale-resolution fallback picks `ru`, not `uz`, and these assertions
    // are uz-Latn ARB-string specific (see home_shell_test.dart's same note).
    locale: const Locale('uz'),
    localizationsDelegates: AppLocalizations.localizationsDelegates,
    supportedLocales: AppLocalizations.supportedLocales,
    home: SessionResultsScreen(result: result),
  );
}

void main() {
  testWidgets('passed + completed renders the pass copy, score and reason', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(
        const SessionResult(
          status: 'passed',
          stoppedReason: 'completed',
          score: 18,
          total: 20,
        ),
      ),
    );

    expect(find.text('O\'tdingiz'), findsOneWidget);
    expect(find.text('18 / 20'), findsOneWidget);
    expect(find.text('90%'), findsOneWidget);
    expect(find.text('Barcha savollar yakunlandi'), findsOneWidget);
    expect(find.byIcon(Icons.check_circle), findsOneWidget);
  });

  testWidgets('failed + completed renders the fail copy', (tester) async {
    await tester.pumpWidget(
      _wrap(
        const SessionResult(
          status: 'failed',
          stoppedReason: 'completed',
          score: 10,
          total: 20,
        ),
      ),
    );

    expect(find.text('O\'ta olmadingiz'), findsOneWidget);
    expect(find.text('10 / 20'), findsOneWidget);
    expect(find.text('50%'), findsOneWidget);
    expect(find.text('Barcha savollar yakunlandi'), findsOneWidget);
    expect(find.byIcon(Icons.cancel), findsOneWidget);
  });

  testWidgets('failed + time_up (exam timer ran out) renders that reason', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(
        const SessionResult(
          status: 'failed',
          stoppedReason: 'time_up',
          score: 5,
          total: 20,
        ),
      ),
    );

    expect(find.text('O\'ta olmadingiz'), findsOneWidget);
    expect(find.text('Vaqt tugadi'), findsOneWidget);
  });

  testWidgets(
    'failed + too_many_errors (exam 3rd-error stop) renders that reason',
    (tester) async {
      await tester.pumpWidget(
        _wrap(
          const SessionResult(
            status: 'failed',
            stoppedReason: 'too_many_errors',
            score: 2,
            total: 20,
          ),
        ),
      );

      expect(find.text('O\'ta olmadingiz'), findsOneWidget);
      expect(find.text('Xatolar soni ko\'payib ketdi'), findsOneWidget);
    },
  );

  testWidgets('abandoned renders the abandoned copy distinctly', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(
        const SessionResult(
          status: 'abandoned',
          stoppedReason: 'too_many_errors',
          score: 1,
          total: 20,
        ),
      ),
    );

    expect(find.text('Sessiya tugallanmadi'), findsOneWidget);
    expect(find.byIcon(Icons.info_outline), findsOneWidget);
  });

  testWidgets('the home button navigates back to /', (tester) async {
    final router = GoRouter(
      initialLocation: '/results',
      routes: [
        GoRoute(
          path: '/',
          builder: (context, state) =>
              const Scaffold(body: Text('home', key: Key('home'))),
        ),
        GoRoute(
          path: '/results',
          builder: (context, state) => const SessionResultsScreen(
            result: SessionResult(
              status: 'passed',
              stoppedReason: 'completed',
              score: 20,
              total: 20,
            ),
          ),
        ),
      ],
    );
    await tester.pumpWidget(
      MaterialApp.router(
        locale: const Locale('uz'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        routerConfig: router,
      ),
    );

    await tester.tap(find.byKey(const Key('session-results-home-button')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('home')), findsOneWidget);
  });
}
