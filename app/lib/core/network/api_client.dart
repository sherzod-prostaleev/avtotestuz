import 'package:dio/dio.dart';

import 'auth_interceptor.dart';
import 'token_storage.dart';

/// Thin wrapper around the configured [Dio] instance used for all backend
/// calls. Kept intentionally small — the interesting behavior lives in
/// [buildDio] and [AuthInterceptor].
class ApiClient {
  // ignore: prefer_initializing_formals
  ApiClient({required Dio dio}) : _dio = dio;

  final Dio _dio;

  Dio get dio => _dio;
}

/// Builds the app's main [Dio] instance: sets [BaseOptions.baseUrl] and
/// installs an [AuthInterceptor] that attaches the bearer token and handles
/// 401-triggered refresh + retry-once.
///
/// A second, plain `Dio` (no interceptors) is created internally for the
/// interceptor to use when calling `/auth/refresh` — this avoids recursive
/// 401 handling on the refresh call itself.
Dio buildDio({
  required String baseUrl,
  required TokenStorage tokenStorage,
  required Future<void> Function() onSessionExpired,
}) {
  final dio = Dio(BaseOptions(baseUrl: baseUrl));
  final refreshDio = Dio(BaseOptions(baseUrl: baseUrl));

  dio.interceptors.add(AuthInterceptor(
    tokenStorage: tokenStorage,
    refreshDio: refreshDio,
    onSessionExpired: onSessionExpired,
  ));

  return dio;
}
