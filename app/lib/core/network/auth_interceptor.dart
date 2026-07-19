import 'package:dio/dio.dart';

import 'token_storage.dart';

/// Attaches the stored access token to outgoing requests and transparently
/// refreshes + retries once on a 401.
///
/// Correctness-critical detail: [RequestOptions.extra]'s `'retried'` flag is
/// set *before* attempting the refresh, and checked on every 401. Without
/// this guard, a stale/invalid refresh token would send the app into an
/// infinite refresh-retry loop (refresh fails with 401 -> that failure is
/// itself intercepted -> refresh attempted again -> ...). With the guard,
/// each request gets at most one refresh+retry cycle, ever.
class AuthInterceptor extends Interceptor {
  AuthInterceptor({
    required this.tokenStorage,
    required this.refreshDio,
    required this.onSessionExpired,
  });

  final TokenStorage tokenStorage;

  /// A separate, plain [Dio] instance with no interceptor attached, used to
  /// call `/auth/refresh` (and to retry the original request) without
  /// recursively triggering this interceptor's own 401 handling.
  final Dio refreshDio;

  final Future<void> Function() onSessionExpired;

  @override
  Future<void> onRequest(
    RequestOptions options,
    RequestInterceptorHandler handler,
  ) async {
    final access = await tokenStorage.readAccess();
    if (access != null && access.isNotEmpty) {
      options.headers['Authorization'] = 'Bearer $access';
    }
    handler.next(options);
  }

  @override
  Future<void> onError(
    DioException err,
    ErrorInterceptorHandler handler,
  ) async {
    final alreadyRetried = err.requestOptions.extra['retried'] == true;
    if (err.response?.statusCode != 401 || alreadyRetried) {
      handler.next(err);
      return;
    }

    // Mark before attempting the refresh, not after: this is what bounds a
    // request to exactly one refresh+retry cycle, even if the refresh call
    // itself fails with a 401.
    err.requestOptions.extra['retried'] = true;

    try {
      final refreshToken = await tokenStorage.readRefresh();
      if (refreshToken == null) {
        throw StateError('No refresh token available.');
      }

      final refreshResponse = await refreshDio.post<Map<String, dynamic>>(
        '/auth/refresh',
        data: {'refresh_token': refreshToken},
      );
      final tokens = refreshResponse.data?['data'] as Map<String, dynamic>?;
      final newAccess = tokens?['access_token'] as String?;
      final newRefresh = tokens?['refresh_token'] as String?;
      if (newAccess == null || newRefresh == null) {
        throw StateError('Malformed refresh response.');
      }

      await tokenStorage.save(access: newAccess, refresh: newRefresh);

      final retryOptions = err.requestOptions;
      retryOptions.headers['Authorization'] = 'Bearer $newAccess';
      final retryResponse = await refreshDio.fetch(retryOptions);
      handler.resolve(retryResponse);
    } catch (_) {
      await tokenStorage.clear();
      await onSessionExpired();
      handler.next(err);
    }
  }
}
