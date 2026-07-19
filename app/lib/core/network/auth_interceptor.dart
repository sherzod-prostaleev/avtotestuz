import 'dart:async';

import 'package:dio/dio.dart';

import 'token_storage.dart';

/// Attaches the stored access token to outgoing requests and transparently
/// refreshes + retries once on a 401.
///
/// Correctness-critical detail #1: [RequestOptions.extra]'s `'retried'` flag
/// is set *before* attempting the refresh, and checked on every 401. Without
/// this guard, a stale/invalid refresh token would send the app into an
/// infinite refresh-retry loop (refresh fails with 401 -> that failure is
/// itself intercepted -> refresh attempted again -> ...). With the guard,
/// each request gets at most one refresh+retry cycle, ever.
///
/// Correctness-critical detail #2: refreshing is single-flight. When two (or
/// more) requests are in-flight concurrently and both receive a 401 around
/// the same time (e.g. two requests fired concurrently right as the access
/// token expires), only ONE of them may actually call `POST /auth/refresh`.
/// The backend rotates and revokes the refresh token on every use, so a
/// second concurrent call presenting the same (now-already-consumed) refresh
/// token is treated as a replay/compromise signal and revokes ALL sessions
/// for that user. [_refreshCompleter] makes every 401 that arrives while a
/// refresh is already underway await that SAME in-flight refresh instead of
/// starting its own; once it resolves (success or failure), each caller
/// independently retries its own original request (on success) or surfaces
/// the failure (on failure — but [onSessionExpired] and [TokenStorage.clear]
/// still only fire once, since they live inside the shared refresh, not in
/// each caller's branch).
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

  /// Non-null while a refresh call is in flight; shared by every concurrent
  /// 401 so at most one `/auth/refresh` request happens at a time. Reset to
  /// `null` once that refresh settles, so a later 401 can trigger a fresh
  /// one.
  ///
  /// This completes with a [_RefreshOutcome] value — never with an error.
  /// Deliberately: completing a shared [Completer] with an error (via
  /// [Completer.completeError]) and having *another* request's `await`
  /// pick it up crosses between two independent Dio requests' internal
  /// zones (Dio forks a zone per request for its chained-stack-trace
  /// support), and Dio's zone error handling treats that cross-zone error
  /// delivery as an unhandled/escaped error — even though it *is* being
  /// awaited and caught normally. That manifests as a second, spurious
  /// "Unhandled exception" and the joining request's `await` never
  /// resuming. Completing with a plain success/failure *value* instead
  /// sidesteps Dio's zone machinery entirely: each caller inspects the
  /// value and — if it's a failure — throws its own fresh exception from
  /// within its own call stack, which behaves like any ordinary exception.
  Completer<_RefreshOutcome>? _refreshCompleter;

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

    final String newAccess;
    try {
      // Joins the single in-flight refresh if one is already underway,
      // rather than starting a second concurrent `/auth/refresh` call.
      newAccess = await _refreshAccessToken();
    } catch (_) {
      // The refresh call itself failed (or returned something unusable): the
      // session really is invalid. Token-clearing and onSessionExpired have
      // already happened exactly once inside the shared refresh above, even
      // if multiple concurrent requests are hitting this catch block.
      handler.next(err);
      return;
    }

    // Refresh succeeded and the new tokens are already saved. From here on,
    // any failure is specific to *this* retried request (e.g. a transient
    // network blip) and must not be treated as a session-expiry: the
    // refreshed tokens remain valid and saved for future requests, and this
    // request's caller should simply see its own error.
    try {
      final retryOptions = err.requestOptions;
      retryOptions.headers['Authorization'] = 'Bearer $newAccess';
      final retryResponse = await refreshDio.fetch(retryOptions);
      handler.resolve(retryResponse);
    } on DioException catch (retryError) {
      handler.reject(retryError);
    } catch (retryError, stackTrace) {
      handler.reject(DioException(
        requestOptions: err.requestOptions,
        error: retryError,
        stackTrace: stackTrace,
      ));
    }
  }

  /// Returns a valid access token, performing (at most) one concurrent
  /// `/auth/refresh` call no matter how many requests call this
  /// simultaneously.
  ///
  /// If a refresh is already in flight, this joins it via the shared
  /// [_refreshCompleter]'s future instead of starting a new one. Otherwise it
  /// becomes the owner of a new refresh and kicks it off. Throws if the
  /// (possibly shared) refresh failed.
  Future<String> _refreshAccessToken() async {
    final inFlight = _refreshCompleter;
    final Future<_RefreshOutcome> outcomeFuture;
    if (inFlight != null) {
      outcomeFuture = inFlight.future;
    } else {
      final completer = Completer<_RefreshOutcome>();
      _refreshCompleter = completer;
      unawaited(_performRefresh(completer));
      outcomeFuture = completer.future;
    }

    final outcome = await outcomeFuture;
    if (outcome.accessToken != null) {
      return outcome.accessToken!;
    }
    // Re-throw as a fresh exception from within *this* caller's own call
    // stack, rather than propagating the original error object across the
    // shared completer as an error (see the doc comment on
    // [_refreshCompleter] for why that matters).
    Error.throwWithStackTrace(outcome.error!, outcome.stackTrace!);
  }

  /// Does the actual refresh-token-storage-read + `POST /auth/refresh` +
  /// token-save work, completing [completer] with a [_RefreshOutcome]
  /// (never with an error — see [_refreshCompleter]'s doc comment). On
  /// failure, clears stored tokens and calls [onSessionExpired] — exactly
  /// once, here, regardless of how many callers are awaiting this refresh.
  Future<void> _performRefresh(Completer<_RefreshOutcome> completer) async {
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
      final refreshedAccess = tokens?['access_token'] as String?;
      final refreshedRefresh = tokens?['refresh_token'] as String?;
      if (refreshedAccess == null || refreshedRefresh == null) {
        throw StateError('Malformed refresh response.');
      }

      await tokenStorage.save(
        access: refreshedAccess,
        refresh: refreshedRefresh,
      );
      completer.complete(_RefreshOutcome.success(refreshedAccess));
    } catch (error, stackTrace) {
      // The refresh call itself failed (or returned something unusable): the
      // session really is invalid, so log the user out. This runs once per
      // shared refresh, not once per waiting caller.
      await tokenStorage.clear();
      await onSessionExpired();
      completer.complete(_RefreshOutcome.failure(error, stackTrace));
    } finally {
      // Allow a later 401 (after this refresh has settled) to trigger a
      // fresh refresh cycle.
      _refreshCompleter = null;
    }
  }
}

/// The result of a (possibly shared) refresh attempt: either a fresh access
/// token, or the error that made the refresh fail. Deliberately a plain
/// value rather than something delivered via [Completer.completeError] — see
/// the doc comment on [AuthInterceptor._refreshCompleter].
class _RefreshOutcome {
  _RefreshOutcome.success(this.accessToken)
      : error = null,
        stackTrace = null;

  _RefreshOutcome.failure(this.error, this.stackTrace) : accessToken = null;

  final String? accessToken;
  final Object? error;
  final StackTrace? stackTrace;
}
