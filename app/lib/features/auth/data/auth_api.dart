import 'package:dio/dio.dart';

import '../../../core/result.dart';

/// Thin [Dio] wrapper around the backend's phone+OTP auth endpoints
/// (`backend/internal/auth/handlers.go`). Every call is wrapped in
/// [guard] so raw [DioException]s never leak past this layer.
///
/// `POST /auth/refresh` is deliberately not wrapped here — Task 2's
/// `AuthInterceptor` already calls it transparently on a 401, so no other
/// code needs to invoke it directly.
///
/// Request/response shapes below were confirmed by reading
/// `backend/internal/auth/handlers.go` directly (not just README prose):
/// ```
/// POST /auth/otp/request {"phone": string}
///   -> {"channel": string, "debug_code"?: string}   // debug_code: dev+sandbox only
/// POST /auth/otp/verify  {"phone": string, "code": string}
///   -> {"access_token": string, "refresh_token": string}
/// POST /auth/logout      {"refresh_token": string}
///   -> {"ok": true}
/// ```
/// All wrapped in the standard envelope `{"data": ...}` /
/// `{"error": {"code": ..., "message": ...}}` (`backend/internal/httpx`).
///
/// Phone format: the backend's `auth.NormalizePhone` accepts loose digit
/// strings — either a bare 9-digit local number (e.g. `"901112233"`) or a
/// full `998XXXXXXXXX`/`+998XXXXXXXXX` form — and normalizes to
/// `+998XXXXXXXXX` server-side. This layer sends whatever string it's given
/// verbatim and lets the server validate/normalize; it does not
/// pre-validate or reformat the phone itself.
class AuthApi {
  AuthApi(this._dio);

  final Dio _dio;

  Future<Result<({String channel, String? debugCode})>> requestOtp(
    String phone,
  ) {
    return guard(() async {
      final response = await _dio.post<Map<String, dynamic>>(
        '/auth/otp/request',
        data: {'phone': phone},
      );
      final data = response.data!['data'] as Map<String, dynamic>;
      return (
        channel: data['channel'] as String,
        debugCode: data['debug_code'] as String?,
      );
    });
  }

  Future<Result<({String access, String refresh})>> verifyOtp({
    required String phone,
    required String code,
  }) {
    return guard(() async {
      final response = await _dio.post<Map<String, dynamic>>(
        '/auth/otp/verify',
        data: {'phone': phone, 'code': code},
      );
      final data = response.data!['data'] as Map<String, dynamic>;
      return (
        access: data['access_token'] as String,
        refresh: data['refresh_token'] as String,
      );
    });
  }

  Future<Result<void>> logout(String refreshToken) {
    return guard(() async {
      await _dio.post<Map<String, dynamic>>(
        '/auth/logout',
        data: {'refresh_token': refreshToken},
      );
    });
  }
}
