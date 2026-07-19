import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/result.dart';
import '../data/auth_repository.dart';
import '../domain/auth_state.dart';

/// Supplies the [AuthRepository] the controller talks to. Has no usable
/// default: a real instance (backed by a configured [Dio]/[TokenStorage])
/// is wired at the app root once networking config (base URL, etc.) is
/// assembled there — out of this task's scope. Tests override this
/// directly with a mock/fake repository.
final authRepositoryProvider = Provider<AuthRepository>((ref) {
  throw UnimplementedError(
    'authRepositoryProvider has no default — override it at the app root '
    'with a real AuthRepository.',
  );
});

final authControllerProvider = NotifierProvider<AuthController, AuthState>(
  AuthController.new,
);

/// Drives the phone+OTP auth flow's UI-facing state.
///
/// [build] must return synchronously, but checking for a stored session
/// (`AuthRepository.hasStoredSession`) is inherently async. To reconcile
/// the two, [build] returns [AuthState.unknown] immediately and kicks off
/// a background check that resolves to `authenticated`/`unauthenticated`
/// once it settles — the same "return a placeholder now, hydrate async"
/// shape already used by `ThemeModeNotifier`/`LocaleNotifier`
/// (`app/lib/app/theme/theme_mode_provider.dart`,
/// `app/lib/app/locale/locale_provider.dart`).
///
/// Those two guard against a competing *explicit* action (`setThemeMode`/
/// `setLocale`) racing ahead of their async hydration with a boolean flag
/// set synchronously the instant that action starts. Here the guard is
/// simpler because `AuthState` is a sealed class: [_checkStoredSession]
/// only ever writes `state` if it is *still* [AuthUnknown] at the moment
/// its async read resolves. If `requestOtp`/`verifyOtp`/`logout` (or
/// anything else) has already moved `state` past `unknown` by then, that
/// background determination is stale and must be dropped rather than
/// clobbering the more-advanced state.
class AuthController extends Notifier<AuthState> {
  AuthRepository get _repository => ref.read(authRepositoryProvider);

  /// Phone the current OTP flow is for, tracked independently of the
  /// publicly-exposed [AuthState] union. A failed `verifyOtp` moves `state`
  /// to [AuthError] — a different union case with no `phone` field — so
  /// [verifyOtp] cannot rely on `state is AuthOtpRequested` to find the
  /// phone to retry against; it reads this field instead. Set on a
  /// successful [requestOtp] (overwriting any previous value), and cleared
  /// once the flow concludes in a way that shouldn't allow further retries
  /// (reaching [AuthAuthenticated], or [logout]).
  String? _pendingPhone;

  @override
  AuthState build() {
    _checkStoredSession();
    return const AuthState.unknown();
  }

  Future<void> _checkStoredSession() async {
    final hasSession = await _repository.hasStoredSession();
    if (!ref.mounted) return;
    // Stale-write guard: only apply this determination if nothing else
    // has already advanced state past `unknown` in the meantime.
    if (state is! AuthUnknown) return;
    state = hasSession
        ? const AuthState.authenticated()
        : const AuthState.unauthenticated();
  }

  Future<void> requestOtp(String phone) async {
    final result = await _repository.requestOtp(phone);
    if (!ref.mounted) return;
    switch (result) {
      case Ok(data: final data):
        _pendingPhone = phone;
        state = AuthState.otpRequested(
          phone: phone,
          debugCode: data.debugCode,
        );
      case Err(failure: final failure):
        state = AuthState.error(failure);
    }
  }

  /// Uses [_pendingPhone] rather than the phone from the current
  /// [AuthOtpRequested] state, so a retry still works after a failed
  /// `verifyOtp` has moved `state` to [AuthError] — a no-op only if no OTP
  /// flow has ever been started (nothing to verify against).
  Future<void> verifyOtp(String code) async {
    final phone = _pendingPhone;
    if (phone == null) return;
    final result = await _repository.verifyOtp(phone: phone, code: code);
    if (!ref.mounted) return;
    switch (result) {
      case Ok():
        _pendingPhone = null;
        state = const AuthState.authenticated();
      case Err(failure: final failure):
        state = AuthState.error(failure);
    }
  }

  Future<void> logout() async {
    await _repository.logout();
    if (!ref.mounted) return;
    _pendingPhone = null;
    state = const AuthState.unauthenticated();
  }
}
