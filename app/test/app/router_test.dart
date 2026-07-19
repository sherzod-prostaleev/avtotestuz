import 'package:avtotest_app/app/l10n/app_localizations.dart';
import 'package:avtotest_app/app/router.dart';
import 'package:avtotest_app/features/auth/domain/auth_state.dart';
import 'package:avtotest_app/features/auth/presentation/auth_controller.dart';
import 'package:avtotest_app/features/auth/presentation/otp_verify_screen.dart';
import 'package:avtotest_app/features/auth/presentation/phone_entry_screen.dart';
import 'package:avtotest_app/features/profile/domain/profile.dart';
import 'package:avtotest_app/features/profile/presentation/profile_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

/// A fake [AuthController] — never talks to a real `AuthRepository`/Dio.
/// [emit] drives the *same* notifier instance through a sequence of states,
/// which is exactly what's needed to prove the router's redirect guard
/// actually re-fires on state changes rather than only applying once at
/// construction.
class _FakeAuthController extends AuthController {
  _FakeAuthController(this._initial);

  final AuthState _initial;

  @override
  AuthState build() => _initial;

  void emit(AuthState newState) => state = newState;
}

/// A fake [ProfileController] so `HomeShell` (rendered at `/` once
/// authenticated) never touches `profileApiProvider`'s real
/// `UnimplementedError` default in these router-focused tests — these tests
/// only care about redirect behavior, not profile data, so a fixed
/// instantly-resolved value is all that's needed.
class _FakeProfileController extends ProfileController {
  @override
  Future<({Profile profile, Entitlement entitlement})> build() async {
    return (
      profile: Profile(
        id: 'p1',
        phone: '+998901112233',
        name: 'Test User',
        region: 'Toshkent',
        district: 'Chilonzor',
        localePref: 'uz-Latn',
        themePref: 'dark',
        referralCode: '',
        role: 'user',
        createdAt: DateTime(2024, 1, 1),
      ),
      entitlement: const Entitlement(active: false),
    );
  }
}

void main() {
  testWidgets(
    'redirects re-fire on auth state changes on the SAME router instance '
    '(not just once at construction): unauthenticated on / redirects to '
    '/login, then flipping to authenticated redirects /login -> / without '
    'rebuilding the whole widget tree from scratch',
    (tester) async {
      final fake = _FakeAuthController(const AuthState.unauthenticated());
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => fake),
          profileControllerProvider.overrideWith(_FakeProfileController.new),
        ],
      );
      addTearDown(container.dispose);

      // Read routerProvider exactly once — the whole point of this test is
      // to prove redirects re-evaluate on THIS SAME GoRouter instance, not
      // that a fresh one gets built each time auth state changes.
      final router = container.read(routerProvider);

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp.router(
            routerConfig: router,
            // Pinned explicitly: without a `locale:`, Flutter's default
            // locale-resolution fallback picks `ru`
            // (`AppLocalizations.supportedLocales`'s first entry), not
            // `uz` — and the 'AvtoTest' assertions below are uz-Latn
            // specific.
            locale: const Locale('uz'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
          ),
        ),
      );
      await tester.pumpAndSettle();

      // Started unauthenticated, requested '/': the guard must redirect to
      // /login.
      expect(
        router.routerDelegate.currentConfiguration.uri.toString(),
        '/login',
      );
      expect(find.byType(PhoneEntryScreen), findsOneWidget);

      // Flip auth state on the SAME notifier instance the router's
      // refreshListenable is listening to (no new ProviderContainer, no
      // re-reading routerProvider) -- this is the crux of the property
      // this test exists to prove.
      fake.emit(const AuthState.authenticated());
      await tester.pumpAndSettle();

      // Still the exact same GoRouter object...
      expect(container.read(routerProvider), same(router));
      // ...and it redirected /login -> / on its own, driven by
      // refreshListenable firing (wired via `ref.listen` inside
      // `_AuthRefreshListenable` in app/lib/app/router.dart) -- not because
      // a new router/widget tree was built from scratch.
      expect(router.routerDelegate.currentConfiguration.uri.toString(), '/');
      expect(find.byType(PhoneEntryScreen), findsNothing);
      expect(find.text('AvtoTest'), findsOneWidget);
    },
  );

  testWidgets('AuthAuthenticated on /login redirects away to /', (
    tester,
  ) async {
    final fake = _FakeAuthController(const AuthState.authenticated());
    final container = ProviderContainer(
      overrides: [
        authControllerProvider.overrideWith(() => fake),
        profileControllerProvider.overrideWith(_FakeProfileController.new),
      ],
    );
    addTearDown(container.dispose);
    final router = container.read(routerProvider)..go('/login');

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp.router(
          routerConfig: router,
          // Pinned explicitly: without a `locale:`, Flutter's default
          // locale-resolution fallback picks `ru`
          // (`AppLocalizations.supportedLocales`'s first entry), not `uz`.
          locale: const Locale('uz'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('AvtoTest'), findsOneWidget);
    expect(find.byType(PhoneEntryScreen), findsNothing);
  });

  testWidgets(
    'AuthOtpRequested is allowed to stay on /login/verify (no redirect)',
    (tester) async {
      final fake = _FakeAuthController(
        const AuthState.otpRequested(phone: '901112233'),
      );
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => fake),
          profileControllerProvider.overrideWith(_FakeProfileController.new),
        ],
      );
      addTearDown(container.dispose);
      final router = container.read(routerProvider)..go('/login/verify');

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp.router(
            routerConfig: router,
            // Pinned explicitly: without a `locale:`, Flutter's default
            // locale-resolution fallback picks `ru`
            // (`AppLocalizations.supportedLocales`'s first entry), not
            // `uz` — and the 'AvtoTest' assertions below are uz-Latn
            // specific.
            locale: const Locale('uz'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.byType(OtpVerifyScreen), findsOneWidget);
    },
  );

  testWidgets('AuthUnknown does not redirect away from the requested route', (
    tester,
  ) async {
    final fake = _FakeAuthController(const AuthState.unknown());
    final container = ProviderContainer(
      overrides: [
        authControllerProvider.overrideWith(() => fake),
        profileControllerProvider.overrideWith(_FakeProfileController.new),
      ],
    );
    addTearDown(container.dispose);
    final router = container.read(routerProvider);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp.router(
          routerConfig: router,
          // Pinned explicitly: without a `locale:`, Flutter's default
          // locale-resolution fallback picks `ru`
          // (`AppLocalizations.supportedLocales`'s first entry), not `uz`.
          locale: const Locale('uz'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
        ),
      ),
    );
    await tester.pumpAndSettle();

    // No redirect happened yet: still on the initial '/' location.
    expect(router.routerDelegate.currentConfiguration.uri.toString(), '/');
  });
}
