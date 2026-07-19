import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/content/data/content_api.dart'
    show contentApiProvider;
import 'package:avtotest_app/features/session/data/session_api.dart';
import 'package:avtotest_app/features/session/domain/session_models.dart';
import 'package:avtotest_app/features/session/presentation/session_controller.dart';
import 'package:avtotest_app/features/session/presentation/session_screen.dart';
import 'package:avtotest_app/features/variants/presentation/variants_controller.dart';
import 'package:avtotest_app/features/variants/presentation/variants_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../session/presentation/session_fakes.dart';

/// Builds a real router with `/` (the [VariantsScreen] under test),
/// `/session` (the real [SessionScreen], same wiring `router.dart` uses) and
/// the results route — only the two network APIs (`SessionApi`/`ContentApi`)
/// are faked; the [VariantsScreen], its [VariantsController], [SessionScreen]
/// and [SessionController] are all real, so a tap genuinely exercises the
/// seam described in `variants_screen.dart`'s doc comment.
Widget _wrap({required FakeSessionApi sessionApi, FakeContentApi? contentApi}) {
  final router = GoRouter(
    initialLocation: '/',
    routes: [
      GoRoute(path: '/', builder: (context, state) => const VariantsScreen()),
      GoRoute(
        path: '/session',
        builder: (context, state) =>
            SessionScreen(request: state.extra! as SessionStartRequest),
      ),
      GoRoute(
        path: sessionResultRoute,
        builder: (context, state) =>
            SessionResultView(result: state.extra! as SessionResult),
      ),
    ],
  );
  return ProviderScope(
    overrides: [
      sessionApiProvider.overrideWithValue(sessionApi),
      contentApiProvider.overrideWithValue(contentApi ?? FakeContentApi()),
    ],
    child: MaterialApp.router(routerConfig: router),
  );
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  testWidgets('renders one cell per bilet, locked ones show a lock icon', (
    tester,
  ) async {
    final api = FakeSessionApi(
      variantsResult: const [
        VariantStatus(
          number: 1,
          questionCount: 20,
          unlocked: true,
          bestCorrect: 15,
          attempts: 1,
        ),
        VariantStatus(
          number: 2,
          questionCount: 20,
          unlocked: false,
          bestCorrect: 0,
          attempts: 0,
        ),
      ],
    );
    await tester.pumpWidget(_wrap(sessionApi: api));
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey('variant-cell-1')), findsOneWidget);
    expect(find.byKey(const ValueKey('variant-cell-2')), findsOneWidget);
    expect(find.byIcon(Icons.lock_outline), findsOneWidget);
  });

  testWidgets('a locked bilet is not tappable — tapping it starts nothing', (
    tester,
  ) async {
    final api = FakeSessionApi(
      variantsResult: const [
        VariantStatus(
          number: 2,
          questionCount: 20,
          unlocked: false,
          bestCorrect: 0,
          attempts: 0,
        ),
      ],
    );
    await tester.pumpWidget(_wrap(sessionApi: api));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const ValueKey('variant-cell-2')));
    await tester.pumpAndSettle();

    // Still on the variants grid — no navigation, no start() call.
    expect(find.byKey(const Key('variants-grid')), findsOneWidget);
    expect(api.startCalls, isEmpty);
  });

  testWidgets(
    'SEAM TEST: tapping an unlocked bilet calls start() with its number and '
    'navigates to the real session screen',
    (tester) async {
      final api = FakeSessionApi(
        variantsResult: const [
          VariantStatus(
            number: 1,
            questionCount: 1,
            unlocked: true,
            bestCorrect: 0,
            attempts: 0,
          ),
        ],
        startResult: fakeSummary(mode: 'variant', ids: ['q1']),
      );
      await tester.pumpWidget(_wrap(sessionApi: api));
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const ValueKey('variant-cell-1')));
      await tester.pumpAndSettle();

      expect(api.startCalls.single.mode, 'variant');
      expect(api.startCalls.single.variantId, '1');
      // Actually landed on the real session screen, not just a state change.
      expect(find.text('Savol matni q1'), findsOneWidget);
    },
  );

  testWidgets(
    'SEAM TEST: a bilet that looks unlocked but whose start() call comes '
    'back vip_required surfaces the distinct upsell copy, not a generic '
    'error banner',
    (tester) async {
      final api = FakeSessionApi(
        variantsResult: const [
          VariantStatus(
            number: 2,
            questionCount: 20,
            // The client's cached list says unlocked, but the server is the
            // authority at start() time — see variants_screen.dart's doc.
            unlocked: true,
            bestCorrect: 0,
            attempts: 0,
          ),
        ],
        startFailure: const Failure(code: 'vip_required', message: 'raw'),
      );
      await tester.pumpWidget(_wrap(sessionApi: api));
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const ValueKey('variant-cell-2')));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('session-error-view')), findsOneWidget);
      expect(find.textContaining('obuna kerak'), findsOneWidget);
    },
  );

  testWidgets('shows a loading indicator while variants are fetched', (
    tester,
  ) async {
    final api = FakeSessionApi(variantsResult: const []);
    await tester.pumpWidget(_wrap(sessionApi: api));
    expect(find.byKey(const Key('variants-loading')), findsOneWidget);
    await tester.pumpAndSettle();
  });

  testWidgets('a failed /me/variants fetch shows an error state with retry', (
    tester,
  ) async {
    final api = FakeSessionApi(
      variantsFailure: const Failure(code: 'unknown', message: 'boom'),
    );
    await tester.pumpWidget(_wrap(sessionApi: api));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('variants-error-state')), findsOneWidget);
    expect(find.text('boom'), findsOneWidget);
  });
}
