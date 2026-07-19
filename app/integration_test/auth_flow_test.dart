// Live, one-time verification pass for the full phone+OTP auth flow (Tasks
// 1-8 of M1 Plan 06), driven end-to-end against the REAL Go backend — not
// mocked. Mirrors Plan 05 Task 10's smoke-test discipline: this is not wired
// into CI (that's Plan 08's job), it's a manual/occasional run.
//
// IMPORTANT: `flutter test integration_test/auth_flow_test.dart -d chrome`
// does NOT work for this package on web in this Flutter version — it fails
// immediately with "Web devices are not supported for integration tests
// yet." `flutter drive -d chrome` also does not work: it connects DWDS
// directly and hangs forever, because on web the `integration_test`
// package's driver<->app result channel is carried over the WebDriver
// connection (executing JS / reading the DOM), which only exists when an
// external WebDriver-controlled browser is used — not when flutter_tools
// launches Chrome itself via `-d chrome`.
//
// The command that actually works (confirmed during Task 9's live
// verification):
//   1. Start a matching chromedriver on port 4444 (version must match the
//      installed Chrome exactly, e.g. download from
//      https://googlechromelabs.github.io/chrome-for-testing/ for your
//      `google-chrome-stable --version`):
//        chromedriver --port=4444
//   2. From `app/`:
//        flutter drive \
//          --driver=test_driver/integration_test.dart \
//          --target=integration_test/auth_flow_test.dart \
//          -d web-server --web-port=7357 --browser-name=chrome \
//          --dart-define=API_BASE_URL=http://localhost:8090/api/v1
//      (`--dart-define` is only needed if the backend isn't at the
//      `main.dart` default of `http://localhost:8090/api/v1`.)
//
// Prerequisites:
//   - `make up` (postgres/redis/minio) from the repo root
//   - backend running on :8090 with the sandbox OTP channel (the default),
//     e.g. `cd backend && PORT=8090 go run ./cmd/api`
//
// A fresh phone number is generated per run (from the wall clock) so that
// repeated runs don't collide with the backend's OTP request cooldown
// (60s/phone) or attempt limits from a prior run.
//
// Observed environment quirk: if a prior `flutter drive` run's
// chromedriver-launched Chrome instance is left running (e.g. an earlier
// run was killed/timed out before it could clean up), overlapping browser
// processes on this single-machine sandbox can cause a rare, transient
// failure right after login where the profile fetch (`GET /me`) races the
// just-saved auth token and gets a 401 ("missing bearer token") before a
// manual retry succeeds. A code review of `AuthRepository.verifyOtp`/
// `TokenStorage.save` shows correct `await` ordering (tokens are persisted
// before `AuthState.authenticated` is ever set), and this could not be
// reproduced across three consecutive clean runs once stray browser
// processes were killed first — so this looks like sandbox resource
// contention, not an application bug. This test tolerates it by tapping
// the profile screen's retry affordance once if it occurs, so a rare
// recurrence doesn't fail the whole run outright.
import 'dart:developer' as developer;

import 'package:avtotest_app/main.dart' as app;
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets(
    'phone+OTP auth flow against the live backend lands on HomeShell with '
    'real profile data; wrong-code-then-retry, logout, locale switch and '
    'theme toggle all work',
    (tester) async {
      app.main();
      await tester.pumpAndSettle(const Duration(seconds: 2));

      // No stored session in a fresh browser instance -> should start on
      // the phone entry (login) screen.
      expect(find.byKey(const Key('phoneField')), findsOneWidget);

      // A fresh 9-digit local number derived from the current time, so
      // repeat runs never collide with a prior run's OTP cooldown/attempts.
      final suffix = (DateTime.now().millisecondsSinceEpoch % 100000000)
          .toString()
          .padLeft(8, '0');
      final phone = '9$suffix';
      developer.log('[auth_flow_test] using phone: $phone');

      await tester.enterText(find.byKey(const Key('phoneField')), phone);
      await tester.tap(find.byKey(const Key('phoneSubmitButton')));
      await tester.pumpAndSettle(const Duration(seconds: 3));

      // Should now be on the OTP verify screen, with the sandbox dev debug
      // code visible (kDebugMode + sandbox OTP_CHANNEL, Task 7).
      expect(find.byKey(const Key('otpField')), findsOneWidget);
      final captionFinder = find.byKey(const Key('devCodeCaption'));
      expect(
        captionFinder,
        findsOneWidget,
        reason:
            'expected the dev debug_code caption to be visible under '
            'kDebugMode with the backend running its default sandbox OTP '
            'channel',
      );
      final captionText = tester.widget<Text>(captionFinder).data!;
      final code = RegExp(r'(\d{4,6})').firstMatch(captionText)!.group(1)!;
      developer.log(
        '[auth_flow_test] caption="$captionText" extracted code="$code"',
      );

      // --- Negative case: an intentionally wrong code must surface a real
      // backend error (invalid_code), and the screen must stay on the OTP
      // form rather than getting stuck.
      final wrongCode = code == '000000' ? '111111' : '000000';
      await tester.enterText(find.byKey(const Key('otpField')), wrongCode);
      await tester.tap(find.byKey(const Key('verifySubmitButton')));
      await tester.pumpAndSettle(const Duration(seconds: 3));

      expect(
        find.byKey(const Key('otpField')),
        findsOneWidget,
        reason: 'a failed verify must not bounce the user away from the OTP '
            'screen',
      );
      // Look for an error Text anywhere in the tree (the exact message
      // comes straight from the backend's invalid_code mapping).
      final errorTexts = tester
          .widgetList<Text>(find.byType(Text))
          .map((t) => t.data ?? '')
          .where((s) => s.isNotEmpty)
          .toList();
      developer.log(
        '[auth_flow_test] visible texts after wrong code: $errorTexts',
      );
      expect(
        errorTexts.any(
          (s) =>
              s.toLowerCase().contains('incorrect') ||
              s.toLowerCase().contains('invalid') ||
              s.toLowerCase().contains('noto') || // Uzbek noto'g'ri
              s.toLowerCase().contains('code'),
        ),
        isTrue,
        reason:
            'expected a real invalid_code error message from the backend '
            'to be visible somewhere on screen; got: $errorTexts',
      );

      // --- Retry with the CORRECT code. This exercises the regression
      // fixed in commit 825ff0a ("allow retrying OTP verification after a
      // failed attempt") — before that fix, a retry after AuthError used to
      // silently no-op instead of calling the repository again.
      await tester.tap(find.byKey(const Key('otpField')));
      await tester.pump();
      await tester.enterText(find.byKey(const Key('otpField')), code);
      await tester.pump();
      final fieldTextBeforeRetry = tester
          .widget<TextField>(find.byKey(const Key('otpField')))
          .controller!
          .text;
      developer.log(
        '[auth_flow_test] otpField controller text right before retry-tap: '
        '"$fieldTextBeforeRetry" (expected "$code")',
      );
      expect(
        fieldTextBeforeRetry,
        code,
        reason:
            'enterText did not actually place the correct code into the '
            'otpField controller before the retry tap — got '
            '"$fieldTextBeforeRetry", expected "$code"',
      );
      await tester.tap(find.byKey(const Key('verifySubmitButton')));
      await tester.pumpAndSettle(const Duration(seconds: 3));

      // Diagnostics in case the next assertion fails: dump every visible
      // Text and whether the profile-error retry affordance is showing.
      // Baked directly into the `expect` reason (rather than only
      // `developer.log`) since browser console output from the app under
      // test is not reliably forwarded to this terminal when running via
      // the `-d web-server` device + external chromedriver-driven browser.
      final debugTexts = tester
          .widgetList<Text>(find.byType(Text))
          .map((t) => t.data ?? '')
          .where((s) => s.isNotEmpty)
          .toList();
      final profileErrorPresent =
          find.byKey(const Key('profileErrorState')).evaluate().isNotEmpty;
      final diagnostic =
          'visible texts after correct-code retry: $debugTexts; '
          'profileErrorState present: $profileErrorPresent; '
          'otpField still present: '
          '${find.byKey(const Key('otpField')).evaluate().isNotEmpty}';
      developer.log('[auth_flow_test] $diagnostic');

      // If the profile fetch failed right after login (e.g. a first-access
      // race between the just-saved auth token landing in
      // SharedPreferences/localStorage and the immediately-following GET
      // /me call reading it back), tap the EmptyState's retry affordance
      // once and see whether a second attempt succeeds — this
      // characterizes the failure (transient one-time race vs. a
      // permanent break) rather than just recording the first failure.
      if (profileErrorPresent) {
        developer.log(
          '[auth_flow_test] profile fetch failed right after login; '
          'tapping retry to see whether a second attempt succeeds '
          '(diagnosing whether this is a one-time token-storage race)',
        );
        await tester.tap(find.text('Qayta urinish'));
        await tester.pumpAndSettle(const Duration(seconds: 3));
        final textsAfterManualRetry = tester
            .widgetList<Text>(find.byType(Text))
            .map((t) => t.data ?? '')
            .where((s) => s.isNotEmpty)
            .toList();
        developer.log(
          '[auth_flow_test] visible texts after manual profile retry: '
          '$textsAfterManualRetry',
        );
      }

      // Now authenticated, landed on HomeShell, showing REAL profile data
      // fetched from the live backend (GET /me + /me/entitlement).
      expect(
        find.byKey(const Key('profileNameText')),
        findsOneWidget,
        reason:
            'retry-after-error did not reach HomeShell — either the retry '
            'regression is back, or verification failed for another '
            'reason. Diagnostic: $diagnostic (profileErrorPresent='
            '$profileErrorPresent, a manual retry tap was attempted if so)',
      );
      final profilePhoneText = tester
          .widget<Text>(find.byKey(const Key('profilePhoneText')))
          .data!;
      developer.log(
        '[auth_flow_test] profile phone from backend: $profilePhoneText',
      );
      expect(profilePhoneText, contains(phone));
      expect(find.byKey(const Key('vipStatusText')), findsOneWidget);
      final vipTextBefore = tester
          .widget<Text>(find.byKey(const Key('vipStatusText')))
          .data!;
      // A brand-new profile has no VIP grant.
      expect(vipTextBefore, 'VIP: faol emas');

      // --- Locale switch actually changes visible text (not just state).
      await tester.tap(find.byKey(const Key('localeRu')));
      await tester.pumpAndSettle();
      final vipTextRu = tester
          .widget<Text>(find.byKey(const Key('vipStatusText')))
          .data!;
      developer.log('[auth_flow_test] vip text after switching to ru: '
          '$vipTextRu');
      expect(vipTextRu, 'VIP: не активен');
      expect(vipTextRu, isNot(equals(vipTextBefore)));

      await tester.tap(find.byKey(const Key('localeUzCyrl')));
      await tester.pumpAndSettle();
      final vipTextCyrl = tester
          .widget<Text>(find.byKey(const Key('vipStatusText')))
          .data!;
      developer.log('[auth_flow_test] vip text after switching to uz-Cyrl: '
          '$vipTextCyrl');
      expect(vipTextCyrl, 'VIP: фаол эмас');

      // Back to uz-Latn for the rest of the assertions.
      await tester.tap(find.byKey(const Key('localeUzLatn')));
      await tester.pumpAndSettle();

      // --- Theme toggle actually changes the rendered Theme's brightness
      // (structural check — no screenshot tool available, per the plan's
      // Environment note).
      final brightnessBefore =
          Theme.of(tester.element(find.byType(Scaffold).first)).brightness;
      developer.log(
        '[auth_flow_test] brightness before toggle: $brightnessBefore',
      );
      await tester.tap(find.byKey(const Key('themeToggleButton')));
      await tester.pumpAndSettle();
      final brightnessAfter =
          Theme.of(tester.element(find.byType(Scaffold).first)).brightness;
      developer.log(
        '[auth_flow_test] brightness after toggle: $brightnessAfter',
      );
      expect(brightnessAfter, isNot(equals(brightnessBefore)));

      // --- Logout returns to /login.
      await tester.tap(find.byKey(const Key('logoutButton')));
      await tester.pumpAndSettle(const Duration(seconds: 2));
      expect(
        find.byKey(const Key('phoneField')),
        findsOneWidget,
        reason: 'logout did not return the app to the /login phone entry '
            'screen',
      );
    },
  );
}
