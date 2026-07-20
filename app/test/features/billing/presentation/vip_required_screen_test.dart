import 'package:avtotest_app/app/l10n/app_localizations.dart';
import 'package:avtotest_app/features/billing/presentation/vip_required_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

/// Builds a real [GoRouter] with `/vip-required` (the screen under test) and a
/// `/` home target, so the "go home" affordance can be exercised as an actual
/// navigation — not just asserted to exist.
Widget _wrap() {
  final router = GoRouter(
    initialLocation: vipRequiredRoute,
    routes: [
      GoRoute(
        path: '/',
        builder: (context, state) =>
            const Scaffold(body: Text('home', key: Key('home'))),
      ),
      GoRoute(
        path: vipRequiredRoute,
        builder: (context, state) => const VipRequiredScreen(),
      ),
    ],
  );
  return MaterialApp.router(
    // Pinned to uz-Latn — the assertions below are ARB-string specific and
    // Flutter's default locale resolution would otherwise pick `ru`.
    locale: const Locale('uz'),
    localizationsDelegates: AppLocalizations.localizationsDelegates,
    supportedLocales: AppLocalizations.supportedLocales,
    routerConfig: router,
  );
}

void main() {
  testWidgets('renders the honest upsell copy (no fake checkout affordance)', (
    tester,
  ) async {
    await tester.pumpWidget(_wrap());
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('vip-required-screen')), findsOneWidget);
    // The headline and the "purchasing isn't available yet" honesty line.
    expect(find.text('Bu bo\'lim faqat obunachilar uchun'), findsOneWidget);
    expect(find.textContaining('mavjud emas'), findsOneWidget);
    // Explicitly NOT a fake checkout: no buy/pay button.
    expect(find.widgetWithText(FilledButton, 'Sotib olish'), findsNothing);
  });

  testWidgets('the "go home" button actually navigates to home', (
    tester,
  ) async {
    await tester.pumpWidget(_wrap());
    await tester.pumpAndSettle();

    // We start on the VIP screen, not home.
    expect(find.byKey(const Key('home')), findsNothing);

    await tester.tap(find.byKey(const Key('vip-required-home-button')));
    await tester.pumpAndSettle();

    // Real navigation happened — we're on home, off the VIP screen.
    expect(find.byKey(const Key('home')), findsOneWidget);
    expect(find.byKey(const Key('vip-required-screen')), findsNothing);
  });
}
