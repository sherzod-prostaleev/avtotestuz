import 'package:freezed_annotation/freezed_annotation.dart';

import '../../../core/result.dart';

part 'auth_state.freezed.dart';

/// UI-facing state of the phone+OTP auth flow, driven by
/// `presentation/auth_controller.dart`.
@freezed
sealed class AuthState with _$AuthState {
  /// App just started (or a fresh `ProviderContainer`/session) — whether a
  /// stored session exists hasn't been checked yet. Never meant to be
  /// displayed for long; `AuthController` resolves it in the background.
  const factory AuthState.unknown() = AuthUnknown;

  /// No valid session: no stored refresh token, or the user explicitly
  /// logged out.
  const factory AuthState.unauthenticated() = AuthUnauthenticated;

  /// An OTP was requested for [phone] and is awaiting `verifyOtp`.
  /// [debugCode] is only ever non-null in the backend's dev/sandbox channel
  /// (see `auth.Service.RequestOTP`) — never present against a real SMS
  /// channel.
  const factory AuthState.otpRequested({
    required String phone,
    String? debugCode,
  }) = AuthOtpRequested;

  /// A valid session exists (either just verified, or a stored refresh
  /// token was found at startup).
  const factory AuthState.authenticated() = AuthAuthenticated;

  /// The last auth operation failed. [failure] carries the backend's
  /// `error.code` (e.g. `invalid_code`, `expired_code`, `rate_limited`)
  /// untouched, so the UI can show a specific, distinct message per code.
  const factory AuthState.error(Failure failure) = AuthError;
}
