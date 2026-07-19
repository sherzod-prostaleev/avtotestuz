import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:avtotest_app/core/network/auth_interceptor.dart';
import 'package:avtotest_app/core/network/token_storage.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

/// A hand-rolled [HttpClientAdapter] that never touches the real network.
/// Every call to [fetch] is recorded and answered by [handler].
class FakeAdapter implements HttpClientAdapter {
  FakeAdapter(this.handler);

  final Future<ResponseBody> Function(RequestOptions options) handler;
  final List<RequestOptions> requests = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    return handler(options);
  }

  @override
  void close({bool force = false}) {}
}

ResponseBody jsonResponseBody(Object body, int statusCode) {
  return ResponseBody.fromString(
    jsonEncode(body),
    statusCode,
    headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    },
  );
}

/// An in-memory [TokenStorage] fake — no shared_preferences plugin channel
/// involved, so these tests stay fast and hermetic.
class FakeTokenStorage implements TokenStorage {
  FakeTokenStorage({String? access, String? refresh})
      // ignore: prefer_initializing_formals
      : _access = access,
        // ignore: prefer_initializing_formals
        _refresh = refresh;

  String? _access;
  String? _refresh;
  int clearCalls = 0;
  int saveCalls = 0;

  @override
  Future<String?> readAccess() async => _access;

  @override
  Future<String?> readRefresh() async => _refresh;

  @override
  Future<void> save({required String access, required String refresh}) async {
    saveCalls++;
    _access = access;
    _refresh = refresh;
  }

  @override
  Future<void> clear() async {
    clearCalls++;
    _access = null;
    _refresh = null;
  }
}

void main() {
  group('AuthInterceptor', () {
    test('attaches Authorization bearer header on outgoing requests',
        () async {
      final tokenStorage = FakeTokenStorage(access: 'access-1', refresh: 'r');
      final mainAdapter = FakeAdapter(
        (options) async => jsonResponseBody({
          'data': {'ok': true},
        }, 200),
      );
      final refreshDio = Dio(BaseOptions(baseUrl: 'https://api.test'));
      final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
        ..httpClientAdapter = mainAdapter;
      dio.interceptors.add(AuthInterceptor(
        tokenStorage: tokenStorage,
        refreshDio: refreshDio,
        onSessionExpired: () async {},
      ));

      final response = await dio.get<Map<String, dynamic>>('/me');

      expect(response.statusCode, 200);
      expect(mainAdapter.requests, hasLength(1));
      expect(
        mainAdapter.requests.single.headers['Authorization'],
        'Bearer access-1',
      );
    });

    test('does not attach a header when no access token is stored',
        () async {
      final tokenStorage = FakeTokenStorage();
      final mainAdapter = FakeAdapter(
        (options) async => jsonResponseBody({
          'data': {'ok': true},
        }, 200),
      );
      final refreshDio = Dio(BaseOptions(baseUrl: 'https://api.test'));
      final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
        ..httpClientAdapter = mainAdapter;
      dio.interceptors.add(AuthInterceptor(
        tokenStorage: tokenStorage,
        refreshDio: refreshDio,
        onSessionExpired: () async {},
      ));

      await dio.get<Map<String, dynamic>>('/me');

      expect(
        mainAdapter.requests.single.headers.containsKey('Authorization'),
        isFalse,
      );
    });

    test('401 triggers exactly one refresh and retries the request once',
        () async {
      final tokenStorage =
          FakeTokenStorage(access: 'old-access', refresh: 'refresh-abc');
      var protectedCallCount = 0;
      var refreshCallCount = 0;
      var sessionExpiredCount = 0;

      final mainAdapter = FakeAdapter((options) async {
        protectedCallCount++;
        return jsonResponseBody({
          'error': {'code': 'unauthorized', 'message': 'token expired'},
        }, 401);
      });

      final refreshAdapter = FakeAdapter((options) async {
        if (options.path == '/auth/refresh') {
          refreshCallCount++;
          expect(options.data, {'refresh_token': 'refresh-abc'});
          return jsonResponseBody({
            'data': {
              'access_token': 'new-access',
              'refresh_token': 'new-refresh',
            },
          }, 200);
        }
        // This is the retried original request.
        expect(options.headers['Authorization'], 'Bearer new-access');
        return jsonResponseBody({
          'data': {'ok': true},
        }, 200);
      });

      final refreshDio = Dio(BaseOptions(baseUrl: 'https://api.test'))
        ..httpClientAdapter = refreshAdapter;
      final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
        ..httpClientAdapter = mainAdapter;
      dio.interceptors.add(AuthInterceptor(
        tokenStorage: tokenStorage,
        refreshDio: refreshDio,
        onSessionExpired: () async {
          sessionExpiredCount++;
        },
      ));

      final response = await dio.get<Map<String, dynamic>>('/protected');

      expect(response.statusCode, 200);
      expect(response.data, {
        'data': {'ok': true},
      });
      expect(protectedCallCount, 1);
      expect(refreshCallCount, 1);
      expect(sessionExpiredCount, 0);
      expect(await tokenStorage.readAccess(), 'new-access');
      expect(await tokenStorage.readRefresh(), 'new-refresh');
    });

    test(
        '401 with a failing refresh clears tokens, calls onSessionExpired '
        'exactly once, and does not loop', () async {
      final tokenStorage =
          FakeTokenStorage(access: 'old-access', refresh: 'stale-refresh');
      var protectedCallCount = 0;
      var refreshCallCount = 0;
      var sessionExpiredCount = 0;

      final mainAdapter = FakeAdapter((options) async {
        protectedCallCount++;
        return jsonResponseBody({
          'error': {'code': 'unauthorized', 'message': 'token expired'},
        }, 401);
      });

      final refreshAdapter = FakeAdapter((options) async {
        refreshCallCount++;
        return jsonResponseBody({
          'error': {
            'code': 'invalid_refresh',
            'message': 'refresh token is invalid or expired',
          },
        }, 401);
      });

      final refreshDio = Dio(BaseOptions(baseUrl: 'https://api.test'))
        ..httpClientAdapter = refreshAdapter;
      final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
        ..httpClientAdapter = mainAdapter;
      dio.interceptors.add(AuthInterceptor(
        tokenStorage: tokenStorage,
        refreshDio: refreshDio,
        onSessionExpired: () async {
          sessionExpiredCount++;
        },
      ));

      await expectLater(
        dio.get<Map<String, dynamic>>('/protected'),
        throwsA(isA<DioException>()),
      );

      // The critical correctness property: exactly one attempt at each step,
      // no infinite refresh/retry loop.
      expect(protectedCallCount, 1);
      expect(refreshCallCount, 1);
      expect(sessionExpiredCount, 1);
      expect(await tokenStorage.readAccess(), isNull);
      expect(await tokenStorage.readRefresh(), isNull);
    });

    test(
        'a successful refresh followed by an unrelated retry failure keeps '
        'the refreshed tokens, does not call onSessionExpired, and '
        'propagates the retry error', () async {
      final tokenStorage =
          FakeTokenStorage(access: 'old-access', refresh: 'refresh-abc');
      var protectedCallCount = 0;
      var refreshCallCount = 0;
      var retryCallCount = 0;
      var sessionExpiredCount = 0;

      final mainAdapter = FakeAdapter((options) async {
        protectedCallCount++;
        return jsonResponseBody({
          'error': {'code': 'unauthorized', 'message': 'token expired'},
        }, 401);
      });

      final refreshAdapter = FakeAdapter((options) async {
        if (options.path == '/auth/refresh') {
          refreshCallCount++;
          return jsonResponseBody({
            'data': {
              'access_token': 'new-access',
              'refresh_token': 'new-refresh',
            },
          }, 200);
        }
        // The retried original request fails for an unrelated reason (e.g.
        // a transient network error), not because the new token is bad.
        retryCallCount++;
        return jsonResponseBody({
          'error': {'code': 'server_error', 'message': 'boom'},
        }, 503);
      });

      final refreshDio = Dio(BaseOptions(baseUrl: 'https://api.test'))
        ..httpClientAdapter = refreshAdapter;
      final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
        ..httpClientAdapter = mainAdapter;
      dio.interceptors.add(AuthInterceptor(
        tokenStorage: tokenStorage,
        refreshDio: refreshDio,
        onSessionExpired: () async {
          sessionExpiredCount++;
        },
      ));

      try {
        await dio.get<Map<String, dynamic>>('/protected');
        fail('expected a DioException to propagate');
      } on DioException catch (e) {
        expect(e.response?.statusCode, 503);
      }

      expect(protectedCallCount, 1);
      expect(refreshCallCount, 1);
      expect(retryCallCount, 1);
      // The refresh succeeded and its tokens must remain saved — this was
      // NOT a session-expiry event.
      expect(sessionExpiredCount, 0);
      expect(tokenStorage.clearCalls, 0);
      expect(await tokenStorage.readAccess(), 'new-access');
      expect(await tokenStorage.readRefresh(), 'new-refresh');
    });

    test(
        'two concurrent 401s share a single in-flight refresh; both '
        'requests succeed via retry and onSessionExpired never fires',
        () async {
      final tokenStorage =
          FakeTokenStorage(access: 'old-access', refresh: 'refresh-abc');
      var refreshCallCount = 0;
      var sessionExpiredCount = 0;
      final mainCallPaths = <String>[];

      // Completes once BOTH protected endpoints have hit the main adapter
      // with their (expired-token) 401 — i.e. once both requests have
      // genuinely reached the interceptor's error handling concurrently.
      // Gating the refresh response on this (rather than on a fixed delay)
      // deterministically proves the second 401 arrives *while* the first
      // refresh is still in flight, instead of relying on timing.
      final bothProtectedArrived = Completer<void>();

      final mainAdapter = FakeAdapter((options) async {
        mainCallPaths.add(options.path);
        if (mainCallPaths.toSet().length == 2 &&
            !bothProtectedArrived.isCompleted) {
          bothProtectedArrived.complete();
        }
        return jsonResponseBody({
          'error': {'code': 'unauthorized', 'message': 'token expired'},
        }, 401);
      });

      final refreshAdapter = FakeAdapter((options) async {
        if (options.path == '/auth/refresh') {
          refreshCallCount++;
          expect(options.data, {'refresh_token': 'refresh-abc'});
          // Block until the second concurrent 401 has definitely arrived,
          // proving both requests joined this one in-flight refresh rather
          // than each kicking off their own.
          await bothProtectedArrived.future;
          return jsonResponseBody({
            'data': {
              'access_token': 'new-access',
              'refresh_token': 'new-refresh',
            },
          }, 200);
        }
        // A retried original request.
        expect(options.headers['Authorization'], 'Bearer new-access');
        return jsonResponseBody({
          'data': {'ok': true, 'path': options.path},
        }, 200);
      });

      final refreshDio = Dio(BaseOptions(baseUrl: 'https://api.test'))
        ..httpClientAdapter = refreshAdapter;
      final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
        ..httpClientAdapter = mainAdapter;
      dio.interceptors.add(AuthInterceptor(
        tokenStorage: tokenStorage,
        refreshDio: refreshDio,
        onSessionExpired: () async {
          sessionExpiredCount++;
        },
      ));

      final results = await Future.wait([
        dio.get<Map<String, dynamic>>('/protected1'),
        dio.get<Map<String, dynamic>>('/protected2'),
      ]);

      expect(results[0].statusCode, 200);
      expect(results[1].statusCode, 200);
      expect(refreshCallCount, 1,
          reason: '/auth/refresh must be called exactly once, not once per '
              'concurrent 401');
      expect(sessionExpiredCount, 0);
      expect(await tokenStorage.readAccess(), 'new-access');
      expect(await tokenStorage.readRefresh(), 'new-refresh');
    });

    test(
        'two concurrent 401s share a single in-flight refresh; if it fails, '
        'both requests reject and onSessionExpired fires exactly once',
        () async {
      final tokenStorage =
          FakeTokenStorage(access: 'old-access', refresh: 'stale-refresh');
      var refreshCallCount = 0;
      var sessionExpiredCount = 0;
      final mainCallPaths = <String>[];

      final bothProtectedArrived = Completer<void>();

      final mainAdapter = FakeAdapter((options) async {
        mainCallPaths.add(options.path);
        if (mainCallPaths.toSet().length == 2 &&
            !bothProtectedArrived.isCompleted) {
          bothProtectedArrived.complete();
        }
        return jsonResponseBody({
          'error': {'code': 'unauthorized', 'message': 'token expired'},
        }, 401);
      });

      final refreshAdapter = FakeAdapter((options) async {
        refreshCallCount++;
        await bothProtectedArrived.future;
        return jsonResponseBody({
          'error': {
            'code': 'refresh_reused',
            'message': 'refresh token already used; all sessions revoked',
          },
        }, 401);
      });

      final refreshDio = Dio(BaseOptions(baseUrl: 'https://api.test'))
        ..httpClientAdapter = refreshAdapter;
      final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
        ..httpClientAdapter = mainAdapter;
      dio.interceptors.add(AuthInterceptor(
        tokenStorage: tokenStorage,
        refreshDio: refreshDio,
        onSessionExpired: () async {
          sessionExpiredCount++;
        },
      ));

      final results = await Future.wait([
        dio.get<Map<String, dynamic>>('/protected1'),
        dio.get<Map<String, dynamic>>('/protected2'),
      ].map((f) => f.then<Object?>((r) => r).catchError((Object e) => e)));

      expect(results, everyElement(isA<DioException>()));
      expect(refreshCallCount, 1,
          reason: '/auth/refresh must be called exactly once, not once per '
              'concurrent 401');
      expect(sessionExpiredCount, 1,
          reason: 'onSessionExpired must fire exactly once for the shared '
              'refresh failure, not once per waiting caller');
      expect(await tokenStorage.readAccess(), isNull);
      expect(await tokenStorage.readRefresh(), isNull);
    });

    test('non-401 errors pass through unchanged without a refresh attempt',
        () async {
      final tokenStorage =
          FakeTokenStorage(access: 'old-access', refresh: 'refresh-abc');
      var refreshCallCount = 0;

      final mainAdapter = FakeAdapter(
        (options) async => jsonResponseBody({
          'error': {'code': 'server_error', 'message': 'boom'},
        }, 500),
      );
      final refreshAdapter = FakeAdapter((options) async {
        refreshCallCount++;
        return jsonResponseBody({
          'data': {'access_token': 'x', 'refresh_token': 'y'},
        }, 200);
      });

      final refreshDio = Dio(BaseOptions(baseUrl: 'https://api.test'))
        ..httpClientAdapter = refreshAdapter;
      final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
        ..httpClientAdapter = mainAdapter;
      dio.interceptors.add(AuthInterceptor(
        tokenStorage: tokenStorage,
        refreshDio: refreshDio,
        onSessionExpired: () async {},
      ));

      try {
        await dio.get<Map<String, dynamic>>('/protected');
        fail('expected a DioException to propagate');
      } on DioException catch (e) {
        expect(e.response?.statusCode, 500);
      }

      expect(refreshCallCount, 0);
    });
  });
}
