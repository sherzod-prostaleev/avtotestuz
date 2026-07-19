import 'dart:async';

import 'package:avtotest_app/app/l10n/app_localizations.dart';
import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/auth/domain/auth_state.dart';
import 'package:avtotest_app/features/auth/presentation/auth_controller.dart';
import 'package:avtotest_app/features/auth/presentation/otp_verify_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

/// A fake [AuthController] — never talks to a real `AuthRepository`/Dio.
/// Records `verifyOtp`/`requestOtp` calls and exposes [emit] so a test can
/// drive the same notifier instance through a sequence of states (e.g.
/// `otpRequested` -> `error`), which is what a real verify-then-fail flow
/// looks like.
class _FakeAuthController extends AuthController {
  _FakeAuthController(this._initial);

  final AuthState _initial;
  final List<String> verifyOtpCalls = [];
  final List<String> requestOtpCalls = [];
  Completer<void>? verifyOtpGate;

  @override
  AuthState build() => _initial;

  @override
  Future<void> verifyOtp(String code) async {
    verifyOtpCalls.add(code);
    final gate = verifyOtpGate;
    if (gate != null) await gate.future;
  }

  @override
  Future<void> requestOtp(String phone) async {
    requestOtpCalls.add(phone);
  }

  void emit(AuthState newState) => state = newState;
}

Widget _wrap(_FakeAuthController fake) {
  return ProviderScope(
    overrides: [authControllerProvider.overrideWith(() => fake)],
    child: MaterialApp.router(
      routerConfig: GoRouter(
        initialLocation: '/login/verify',
        routes: [
          GoRoute(
            path: '/login',
            builder: (context, state) => const Scaffold(body: Text('login')),
          ),
          GoRoute(
            path: '/login/verify',
            builder: (context, state) => const OtpVerifyScreen(),
          ),
        ],
      ),
      // Pinned explicitly — see the note in phone_entry_screen_test.dart:
      // without this, Flutter's default locale-resolution fallback picks
      // `ru` (supportedLocales' first entry), not `uz`, and these tests
      // assert on uz-Latn ARB strings.
      locale: const Locale('uz'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
    ),
  );
}

void main() {
  testWidgets(
    'shows the dev debug code caption when debugCode is set (kDebugMode is '
    'true under flutter test)',
    (tester) async {
      final fake = _FakeAuthController(
        const AuthState.otpRequested(phone: '901112233', debugCode: '123456'),
      );
      await tester.pumpWidget(_wrap(fake));
      await tester.pumpAndSettle();

      expect(find.text('Dev kod: 123456'), findsOneWidget);
    },
  );

  testWidgets('hides the dev debug code caption when debugCode is null', (
    tester,
  ) async {
    final fake = _FakeAuthController(
      const AuthState.otpRequested(phone: '901112233', debugCode: null),
    );
    await tester.pumpWidget(_wrap(fake));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('devCodeCaption')), findsNothing);
  });

  testWidgets('shows the phone number the code was requested for', (
    tester,
  ) async {
    final fake = _FakeAuthController(
      const AuthState.otpRequested(phone: '901112233', debugCode: null),
    );
    await tester.pumpWidget(_wrap(fake));
    await tester.pumpAndSettle();

    expect(find.textContaining('901112233'), findsOneWidget);
  });

  testWidgets('entering a code and submitting calls verifyOtp with that '
      'code', (tester) async {
    final fake = _FakeAuthController(
      const AuthState.otpRequested(phone: '901112233', debugCode: null),
    );
    await tester.pumpWidget(_wrap(fake));
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(const Key('otpField')), '654321');
    await tester.tap(find.byKey(const Key('verifySubmitButton')));
    await tester.pump();

    expect(fake.verifyOtpCalls, ['654321']);
  });

  testWidgets(
    'renders the failure message when a verify attempt transitions state '
    'to AuthError, while keeping the form (phone/otp field) usable via the '
    'cached phone from the prior otpRequested state',
    (tester) async {
      final fake = _FakeAuthController(
        const AuthState.otpRequested(phone: '901112233', debugCode: null),
      );
      await tester.pumpWidget(_wrap(fake));
      await tester.pumpAndSettle();

      fake.emit(
        const AuthState.error(
          Failure(code: 'invalid_code', message: 'Kod noto\'g\'ri'),
        ),
      );
      await tester.pump();

      expect(find.text("Kod noto'g'ri"), findsOneWidget);
      // The form itself must still be usable (phone context preserved),
      // not bounced back to /login.
      expect(find.byKey(const Key('otpField')), findsOneWidget);
      expect(find.textContaining('901112233'), findsOneWidget);
    },
  );

  testWidgets(
    'bounces back to /login when reached without ever going through '
    'otpRequested (e.g. a stale deep link)',
    (tester) async {
      final fake = _FakeAuthController(const AuthState.unauthenticated());
      await tester.pumpWidget(_wrap(fake));
      await tester.pumpAndSettle();

      expect(find.text('login'), findsOneWidget);
      expect(find.byType(OtpVerifyScreen), findsNothing);
    },
  );
}
