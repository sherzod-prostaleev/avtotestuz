import 'dart:async';

import 'package:avtotest_app/app/l10n/app_localizations.dart';
import 'package:avtotest_app/app/locale/locale_provider.dart';
import 'package:avtotest_app/app/theme/theme_mode_provider.dart';
import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/auth/domain/auth_state.dart';
import 'package:avtotest_app/features/auth/presentation/auth_controller.dart';
import 'package:avtotest_app/features/home/presentation/home_shell.dart';
import 'package:avtotest_app/features/profile/domain/profile.dart';
import 'package:avtotest_app/features/profile/presentation/profile_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

final _testProfile = Profile(
  id: 'p1',
  phone: '+998901112233',
  name: 'Aziz Karimov',
  region: 'Toshkent',
  district: 'Chilonzor',
  localePref: 'uz-Latn',
  themePref: 'dark',
  referralCode: '',
  role: 'student',
  createdAt: DateTime(2024, 1, 2),
);

/// A [ProfileController] that resolves instantly with a fixed value —
/// never touches `profileApiProvider`'s real `UnimplementedError` seam.
class _FixedProfileController extends ProfileController {
  _FixedProfileController(this._value);

  final ({Profile profile, Entitlement entitlement}) _value;

  @override
  Future<({Profile profile, Entitlement entitlement})> build() async =>
      _value;
}

/// A [ProfileController] whose `build()` never resolves on its own — lets a
/// test observe the "still loading" state deterministically.
class _PendingProfileController extends ProfileController {
  final Completer<({Profile profile, Entitlement entitlement})> completer =
      Completer();

  @override
  Future<({Profile profile, Entitlement entitlement})> build() =>
      completer.future;
}

/// Fails on the first `build()` call (simulating a failed fetch), then
/// succeeds on any subsequent call — lets a test drive
/// "error state -> tap retry -> `ref.invalidate` reloads -> success".
class _ToggleProfileController extends ProfileController {
  int buildCalls = 0;

  @override
  Future<({Profile profile, Entitlement entitlement})> build() async {
    buildCalls++;
    if (buildCalls == 1) {
      throw const ProfileFetchFailure(
        Failure(code: 'internal', message: 'Server xatosi'),
      );
    }
    return (
      profile: _testProfile,
      entitlement: const Entitlement(active: true),
    );
  }
}

class _FakeAuthController extends AuthController {
  int logoutCalls = 0;

  @override
  AuthState build() => const AuthState.authenticated();

  @override
  Future<void> logout() async {
    logoutCalls++;
  }
}

/// Builds a [ProviderContainer] (rather than a bare `ProviderScope`) so
/// tests can `container.read(...)` provider state (e.g. `localeProvider`,
/// `themeModeProvider`) after interacting with the widget tree.
ProviderContainer _buildContainer({
  required ProfileController Function() profileController,
  AuthController Function()? authController,
}) {
  return ProviderContainer(
    overrides: [
      profileControllerProvider.overrideWith(profileController),
      if (authController != null)
        authControllerProvider.overrideWith(authController),
    ],
  );
}

Widget _appFor(ProviderContainer container) {
  return UncontrolledProviderScope(
    container: container,
    child: MaterialApp(
      // Pinned explicitly — same reasoning as `router_test.dart`/
      // `app_test.dart`: without a `locale:`, Flutter's default
      // locale-resolution fallback picks `ru`, not `uz`, and these
      // assertions are uz-Latn ARB-string specific. `HomeShell` reads
      // `localeProvider` itself for the locale-chip's `selected` state —
      // independent of this `MaterialApp.locale`, which only controls
      // which `AppLocalizations` gets resolved for rendering text.
      locale: const Locale('uz'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: const HomeShell(),
    ),
  );
}

void main() {
  setUp(() {
    // themeModeProvider/localeProvider both hydrate from shared_preferences
    // on first read; give them a clean, synchronous in-memory store.
    SharedPreferences.setMockInitialValues({});
  });

  testWidgets('renders the profile name/phone and VIP active status', (
    tester,
  ) async {
    final container = _buildContainer(
      profileController: () => _FixedProfileController(
        (profile: _testProfile, entitlement: const Entitlement(active: true)),
      ),
    );
    addTearDown(container.dispose);
    await tester.pumpWidget(_appFor(container));
    await tester.pumpAndSettle();

    expect(find.text('Aziz Karimov'), findsOneWidget);
    expect(find.text('+998901112233'), findsOneWidget);
    expect(find.text('VIP: faol'), findsOneWidget);
  });

  testWidgets('renders VIP inactive status for a non-VIP entitlement', (
    tester,
  ) async {
    final container = _buildContainer(
      profileController: () => _FixedProfileController(
        (
          profile: _testProfile,
          entitlement: const Entitlement(active: false),
        ),
      ),
    );
    addTearDown(container.dispose);
    await tester.pumpWidget(_appFor(container));
    await tester.pumpAndSettle();

    expect(find.text('VIP: faol emas'), findsOneWidget);
    expect(find.text('VIP: faol'), findsNothing);
  });

  testWidgets('shows a spinner while the profile is still loading', (
    tester,
  ) async {
    final container = _buildContainer(
      profileController: () => _PendingProfileController(),
    );
    addTearDown(container.dispose);
    await tester.pumpWidget(_appFor(container));
    await tester.pump();

    expect(find.byType(CircularProgressIndicator), findsWidgets);
    expect(find.text('Aziz Karimov'), findsNothing);
  });

  testWidgets(
    'shows a retry affordance on error, with the backend failure message; '
    'retrying reloads and succeeds',
    (tester) async {
      final toggle = _ToggleProfileController();
      final container = _buildContainer(profileController: () => toggle);
      addTearDown(container.dispose);
      await tester.pumpWidget(_appFor(container));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('profileErrorState')), findsOneWidget);
      expect(find.text('Server xatosi'), findsOneWidget);
      expect(find.text('Aziz Karimov'), findsNothing);

      await tester.tap(find.text('Qayta urinish'));
      await tester.pumpAndSettle();

      expect(toggle.buildCalls, 2);
      expect(find.text('Aziz Karimov'), findsOneWidget);
      expect(find.byKey(const Key('profileErrorState')), findsNothing);
    },
  );

  testWidgets(
    'tapping logout calls authControllerProvider.notifier.logout()',
    (tester) async {
      final auth = _FakeAuthController();
      final container = _buildContainer(
        profileController: () => _FixedProfileController(
          (
            profile: _testProfile,
            entitlement: const Entitlement(active: true),
          ),
        ),
        authController: () => auth,
      );
      addTearDown(container.dispose);
      await tester.pumpWidget(_appFor(container));
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('logoutButton')));
      await tester.pump();

      expect(auth.logoutCalls, 1);
    },
  );

  testWidgets(
    "tapping the uz-Cyrl locale chip actually changes localeProvider's "
    'value',
    (tester) async {
      final container = _buildContainer(
        profileController: () => _FixedProfileController(
          (
            profile: _testProfile,
            entitlement: const Entitlement(active: true),
          ),
        ),
      );
      addTearDown(container.dispose);
      await tester.pumpWidget(_appFor(container));
      await tester.pumpAndSettle();

      expect(container.read(localeProvider), const Locale('uz'));

      await tester.tap(find.byKey(const Key('localeUzCyrl')));
      await tester.pumpAndSettle();

      expect(
        container.read(localeProvider),
        const Locale.fromSubtags(languageCode: 'uz', scriptCode: 'Cyrl'),
      );
    },
  );

  testWidgets(
    'tapping the ru locale chip actually changes localeProvider\'s value',
    (tester) async {
      final container = _buildContainer(
        profileController: () => _FixedProfileController(
          (
            profile: _testProfile,
            entitlement: const Entitlement(active: true),
          ),
        ),
      );
      addTearDown(container.dispose);
      await tester.pumpWidget(_appFor(container));
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('localeRu')));
      await tester.pumpAndSettle();

      expect(container.read(localeProvider), const Locale('ru'));
    },
  );

  testWidgets(
    'tapping the theme toggle button flips themeModeProvider between '
    'dark and light',
    (tester) async {
      final container = _buildContainer(
        profileController: () => _FixedProfileController(
          (
            profile: _testProfile,
            entitlement: const Entitlement(active: true),
          ),
        ),
      );
      addTearDown(container.dispose);
      await tester.pumpWidget(_appFor(container));
      await tester.pumpAndSettle();

      expect(container.read(themeModeProvider), ThemeMode.dark);

      await tester.tap(find.byKey(const Key('themeToggleButton')));
      await tester.pumpAndSettle();

      expect(container.read(themeModeProvider), ThemeMode.light);
    },
  );

  testWidgets(
    'renders the real variants entry plus honest "coming soon" '
    'placeholders for practice/mistakes/stats, not fake screens',
    (tester) async {
      final container = _buildContainer(
        profileController: () => _FixedProfileController(
          (
            profile: _testProfile,
            entitlement: const Entitlement(active: true),
          ),
        ),
      );
      addTearDown(container.dispose);
      await tester.pumpWidget(_appFor(container));
      await tester.pumpAndSettle();

      // Variants (Plan 07 Task 5) is wired to the real screen now, so it no
      // longer shows the "coming soon" placeholder text — only the still-
      // unbuilt practice/mistakes/stats entries do.
      expect(find.byKey(const Key('navVariants')), findsOneWidget);
      expect(find.byKey(const Key('navPractice')), findsOneWidget);
      expect(find.byKey(const Key('navMistakes')), findsOneWidget);
      expect(find.byKey(const Key('navStats')), findsOneWidget);
      expect(find.text('Tez orada'), findsNWidgets(3));
    },
  );
}
