import '../../../core/network/token_storage.dart';
import '../../../core/result.dart';
import 'auth_api.dart';

/// Mediates between [AuthApi] (wire calls) and [TokenStorage] (local
/// session persistence) so `presentation/auth_controller.dart` never has to
/// touch either directly.
class AuthRepository {
  AuthRepository({required AuthApi api, required TokenStorage tokenStorage})
      // ignore: prefer_initializing_formals
      : _api = api,
        // ignore: prefer_initializing_formals
        _tokenStorage = tokenStorage;

  final AuthApi _api;
  final TokenStorage _tokenStorage;

  Future<Result<({String channel, String? debugCode})>> requestOtp(
    String phone,
  ) {
    return _api.requestOtp(phone);
  }

  /// On success, saves the returned access/refresh tokens via
  /// [TokenStorage.save] before returning — callers can rely on a stored
  /// session existing immediately after an `Ok` result.
  Future<Result<void>> verifyOtp({
    required String phone,
    required String code,
  }) async {
    final result = await _api.verifyOtp(phone: phone, code: code);
    switch (result) {
      case Ok(data: final tokens):
        await _tokenStorage.save(access: tokens.access, refresh: tokens.refresh);
        return const Result.ok(null);
      case Err(failure: final failure):
        return Result.err(failure);
    }
  }

  /// True if a refresh token is present locally — regardless of whether
  /// it's still valid server-side. Validity is only discovered the next
  /// time it's actually used (by `AuthInterceptor`'s refresh call).
  Future<bool> hasStoredSession() async {
    final refresh = await _tokenStorage.readRefresh();
    return refresh != null;
  }

  /// Calls the API best-effort, but **always** clears the local session —
  /// a user pressing "log out" must never end up stuck logged-in-locally
  /// just because the logout network call failed or timed out.
  Future<void> logout() async {
    try {
      final refresh = await _tokenStorage.readRefresh();
      if (refresh != null) {
        await _api.logout(refresh);
      }
    } catch (_) {
      // Best-effort: a network blip here must not block clearing the
      // local session below.
    } finally {
      await _tokenStorage.clear();
    }
  }
}
