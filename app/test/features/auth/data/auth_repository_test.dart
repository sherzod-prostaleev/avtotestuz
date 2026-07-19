import 'package:avtotest_app/core/network/token_storage.dart';
import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/auth/data/auth_api.dart';
import 'package:avtotest_app/features/auth/data/auth_repository.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class MockAuthApi extends Mock implements AuthApi {}

/// In-memory [TokenStorage] fake — no shared_preferences plugin channel
/// involved, so these tests stay fast and hermetic. Mirrors
/// `test/core/network/auth_interceptor_test.dart`'s `FakeTokenStorage`.
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
  late MockAuthApi api;
  late FakeTokenStorage tokenStorage;
  late AuthRepository repository;

  setUp(() {
    api = MockAuthApi();
    tokenStorage = FakeTokenStorage();
    repository = AuthRepository(api: api, tokenStorage: tokenStorage);
  });

  group('requestOtp', () {
    test('delegates straight through to AuthApi.requestOtp', () async {
      const okResult = Result<({String channel, String? debugCode})>.ok(
        (channel: 'sandbox', debugCode: '123456'),
      );
      when(() => api.requestOtp('901112233'))
          .thenAnswer((_) async => okResult);

      final result = await repository.requestOtp('901112233');

      expect(result, same(okResult));
      verify(() => api.requestOtp('901112233')).called(1);
    });
  });

  group('verifyOtp', () {
    test('saves tokens via TokenStorage on success', () async {
      when(() => api.verifyOtp(phone: '901112233', code: '654321'))
          .thenAnswer(
        (_) async => const Result<({String access, String refresh})>.ok(
          (access: 'access-1', refresh: 'refresh-1'),
        ),
      );

      final result =
          await repository.verifyOtp(phone: '901112233', code: '654321');

      expect(result, isA<Ok<void>>());
      expect(tokenStorage.saveCalls, 1);
      expect(await tokenStorage.readAccess(), 'access-1');
      expect(await tokenStorage.readRefresh(), 'refresh-1');
    });

    test('does not save tokens and propagates the Failure on invalid_code',
        () async {
      when(() => api.verifyOtp(phone: '901112233', code: '000000'))
          .thenAnswer(
        (_) async => const Result<({String access, String refresh})>.err(
          Failure(code: 'invalid_code', message: 'code is incorrect'),
        ),
      );

      final result =
          await repository.verifyOtp(phone: '901112233', code: '000000');

      expect(tokenStorage.saveCalls, 0);
      switch (result) {
        case Err(failure: final failure):
          expect(failure.code, 'invalid_code');
        case Ok():
          fail('expected Err');
      }
    });
  });

  group('hasStoredSession', () {
    test('is true when a refresh token is present', () async {
      tokenStorage = FakeTokenStorage(refresh: 'refresh-1');
      repository = AuthRepository(api: api, tokenStorage: tokenStorage);

      expect(await repository.hasStoredSession(), isTrue);
    });

    test('is false when no refresh token is stored', () async {
      expect(await repository.hasStoredSession(), isFalse);
    });
  });

  group('logout', () {
    test('calls the API with the stored refresh token and clears storage '
        'on success', () async {
      tokenStorage = FakeTokenStorage(access: 'access-1', refresh: 'refresh-1');
      repository = AuthRepository(api: api, tokenStorage: tokenStorage);
      when(() => api.logout('refresh-1'))
          .thenAnswer((_) async => const Result<void>.ok(null));

      await repository.logout();

      verify(() => api.logout('refresh-1')).called(1);
      expect(tokenStorage.clearCalls, 1);
      expect(await tokenStorage.readAccess(), isNull);
      expect(await tokenStorage.readRefresh(), isNull);
    });

    test('clears local tokens even when the API call throws (network blip)',
        () async {
      tokenStorage = FakeTokenStorage(access: 'access-1', refresh: 'refresh-1');
      repository = AuthRepository(api: api, tokenStorage: tokenStorage);
      when(() => api.logout('refresh-1')).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/auth/logout'),
          type: DioExceptionType.connectionError,
        ),
      );

      // Must not throw out of logout() itself...
      await repository.logout();

      // ...and the local session must be gone regardless.
      expect(tokenStorage.clearCalls, 1);
      expect(await tokenStorage.readAccess(), isNull);
      expect(await tokenStorage.readRefresh(), isNull);
    });

    test('clears local tokens even when the API call resolves to a Failure',
        () async {
      tokenStorage = FakeTokenStorage(access: 'access-1', refresh: 'refresh-1');
      repository = AuthRepository(api: api, tokenStorage: tokenStorage);
      when(() => api.logout('refresh-1')).thenAnswer(
        (_) async => const Result<void>.err(
          Failure(code: 'network_error', message: 'timed out'),
        ),
      );

      await repository.logout();

      expect(tokenStorage.clearCalls, 1);
      expect(await tokenStorage.readAccess(), isNull);
      expect(await tokenStorage.readRefresh(), isNull);
    });

    test('is a no-op API-wise (but still clears) when nothing is stored',
        () async {
      // tokenStorage already has no refresh token (default setUp).
      await repository.logout();

      verifyZeroInteractions(api);
      expect(tokenStorage.clearCalls, 1);
    });
  });
}
