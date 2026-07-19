import 'dart:async';

import 'package:avtotest_app/app/l10n/app_localizations.dart';
import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/auth/domain/auth_state.dart';
import 'package:avtotest_app/features/auth/presentation/auth_controller.dart';
import 'package:avtotest_app/features/auth/presentation/phone_entry_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

/// A fake [AuthController] — never talks to a real `AuthRepository`/Dio.
/// Records every `requestOtp` call it receives (for assertions) and lets a
/// test pin its starting [AuthState] and optionally gate the call on a
/// [Completer] to simulate an in-flight request.
class _FakeAuthController extends AuthController {
  _FakeAuthController(this._initial);

  final AuthState _initial;
  final List<String> requestOtpCalls = [];
  Completer<void>? requestOtpGate;

  @override
  AuthState build() => _initial;

  @override
  Future<void> requestOtp(String phone) async {
    requestOtpCalls.add(phone);
    final gate = requestOtpGate;
    if (gate != null) await gate.future;
  }
}

Widget _wrap(_FakeAuthController fake) {
  return ProviderScope(
    overrides: [authControllerProvider.overrideWith(() => fake)],
    child: MaterialApp(
      // Pinned explicitly: without a `locale:`, Flutter's default locale
      // resolution falls back to `supportedLocales.first` when nothing
      // matches the test environment's platform locale — which is `ru`
      // (`AppLocalizations.supportedLocales`' declared order), not `uz`.
      // These tests assert on uz-Latn ARB strings, so the locale must be
      // pinned rather than left to that fallback.
      locale: const Locale('uz'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: const PhoneEntryScreen(),
    ),
  );
}

void main() {
  testWidgets(
    'shows an inline error for a malformed phone and does not call '
    'requestOtp',
    (tester) async {
      final fake = _FakeAuthController(const AuthState.unauthenticated());
      await tester.pumpWidget(_wrap(fake));

      await tester.enterText(find.byKey(const Key('phoneField')), '123');
      await tester.tap(find.byKey(const Key('phoneSubmitButton')));
      await tester.pump();

      expect(find.text("Telefon raqami noto'g'ri formatda"), findsOneWidget);
      expect(fake.requestOtpCalls, isEmpty);
    },
  );

  testWidgets('a bare 9-digit phone calls requestOtp with the exact string '
      'entered', (tester) async {
    final fake = _FakeAuthController(const AuthState.unauthenticated());
    await tester.pumpWidget(_wrap(fake));

    await tester.enterText(find.byKey(const Key('phoneField')), '901112233');
    await tester.tap(find.byKey(const Key('phoneSubmitButton')));
    await tester.pump();

    expect(fake.requestOtpCalls, ['901112233']);
  });

  testWidgets(
    'a full 998-prefixed 12-digit phone is also accepted, matching the '
    'backend\'s NormalizePhone (backend/internal/auth/phone.go)',
    (tester) async {
      final fake = _FakeAuthController(const AuthState.unauthenticated());
      await tester.pumpWidget(_wrap(fake));

      await tester.enterText(
        find.byKey(const Key('phoneField')),
        '998901112233',
      );
      await tester.tap(find.byKey(const Key('phoneSubmitButton')));
      await tester.pump();

      expect(fake.requestOtpCalls, ['998901112233']);
    },
  );

  testWidgets('disables the submit button while a request is in flight', (
    tester,
  ) async {
    final fake = _FakeAuthController(const AuthState.unauthenticated())
      ..requestOtpGate = Completer<void>();
    await tester.pumpWidget(_wrap(fake));

    await tester.enterText(find.byKey(const Key('phoneField')), '901112233');
    await tester.tap(find.byKey(const Key('phoneSubmitButton')));
    await tester.pump();

    final duringFlight = tester.widget<ElevatedButton>(
      find.byKey(const Key('phoneSubmitButton')),
    );
    expect(duringFlight.onPressed, isNull);

    fake.requestOtpGate!.complete();
    await tester.pumpAndSettle();

    final afterFlight = tester.widget<ElevatedButton>(
      find.byKey(const Key('phoneSubmitButton')),
    );
    expect(afterFlight.onPressed, isNotNull);
  });

  testWidgets('renders the failure message when state is AuthError', (
    tester,
  ) async {
    final fake = _FakeAuthController(
      const AuthState.error(
        Failure(code: 'invalid_phone', message: 'Bad phone, try again'),
      ),
    );
    await tester.pumpWidget(_wrap(fake));
    await tester.pump();

    expect(find.text('Bad phone, try again'), findsOneWidget);
  });
}
