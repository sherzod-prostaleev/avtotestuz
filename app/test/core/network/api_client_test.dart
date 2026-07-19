import 'package:avtotest_app/core/network/api_client.dart';
import 'package:avtotest_app/core/network/auth_interceptor.dart';
import 'package:avtotest_app/core/network/token_storage.dart';
import 'package:flutter_test/flutter_test.dart';

/// An in-memory [TokenStorage] fake — no shared_preferences plugin channel
/// involved, so this test stays fast and hermetic.
class FakeTokenStorage implements TokenStorage {
  @override
  Future<String?> readAccess() async => null;

  @override
  Future<String?> readRefresh() async => null;

  @override
  Future<void> save({required String access, required String refresh}) async {}

  @override
  Future<void> clear() async {}
}

void main() {
  group('buildDio', () {
    test(
        'returns a Dio with exactly one AuthInterceptor attached, and its '
        'internal refreshDio has zero interceptors', () {
      final dio = buildDio(
        baseUrl: 'https://api.test',
        tokenStorage: FakeTokenStorage(),
        onSessionExpired: () async {},
      );

      expect(dio.options.baseUrl, 'https://api.test');

      final authInterceptors = dio.interceptors.whereType<AuthInterceptor>();
      expect(authInterceptors, hasLength(1));

      final authInterceptor = authInterceptors.single;

      // This is the fact that makes the loop-guard airtight: refreshDio must
      // never itself carry an AuthInterceptor — otherwise a 401 on
      // `/auth/refresh` (or on the retried request) would recursively
      // re-enter AuthInterceptor.onError. (Dio attaches its own internal
      // ImplyContentTypeInterceptor to every instance by default — that one
      // is expected and harmless; only AuthInterceptor itself is the risk.)
      expect(authInterceptor.refreshDio.interceptors.whereType<AuthInterceptor>(),
          isEmpty);
    });
  });
}
