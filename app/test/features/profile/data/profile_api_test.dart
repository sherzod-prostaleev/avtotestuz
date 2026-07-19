import 'dart:convert';
import 'dart:typed_data';

import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/profile/data/profile_api.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

/// A hand-rolled [HttpClientAdapter] that never touches the real network —
/// same pattern as `test/features/auth/data/auth_api_test.dart`.
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

void main() {
  group('ProfileApi', () {
    group('fetchMe', () {
      test('parses the nested profile object from a successful envelope',
          () async {
        final adapter = FakeAdapter(
          (options) async => jsonResponseBody({
            'data': {
              'profile': {
                'id': 'p1',
                'phone': '+998901112233',
                'name': 'Aziz Karimov',
                'region': 'Toshkent',
                'district': 'Chilonzor',
                'birth_date': '2000-05-14',
                'locale_pref': 'uz-Latn',
                'theme_pref': 'dark',
                'referral_code': 'REF123',
                'role': 'student',
                'created_at': '2024-01-02T03:04:05Z',
              },
              'vip': {'active': true, 'until': '2025-01-01T00:00:00Z'},
            },
          }, 200),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = ProfileApi(dio);

        final result = await api.fetchMe();

        expect(adapter.requests.single.path, '/me');
        switch (result) {
          case Ok(data: final profile):
            expect(profile.id, 'p1');
            expect(profile.phone, '+998901112233');
            expect(profile.name, 'Aziz Karimov');
            expect(profile.region, 'Toshkent');
            expect(profile.district, 'Chilonzor');
            expect(profile.birthDate, DateTime.parse('2000-05-14'));
            expect(profile.localePref, 'uz-Latn');
            expect(profile.themePref, 'dark');
            expect(profile.referralCode, 'REF123');
            expect(profile.role, 'student');
            expect(profile.createdAt, DateTime.parse('2024-01-02T03:04:05Z'));
          case Err():
            fail('expected Ok');
        }
      });

      test('birth_date null maps to a null Profile.birthDate', () async {
        final adapter = FakeAdapter(
          (options) async => jsonResponseBody({
            'data': {
              'profile': {
                'id': 'p1',
                'phone': '+998901112233',
                'name': 'Aziz Karimov',
                'region': 'Toshkent',
                'district': 'Chilonzor',
                'birth_date': null,
                'locale_pref': 'uz-Latn',
                'theme_pref': 'dark',
                'referral_code': '',
                'role': 'student',
                'created_at': '2024-01-02T03:04:05Z',
              },
              'vip': {'active': false, 'until': null},
            },
          }, 200),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = ProfileApi(dio);

        final result = await api.fetchMe();

        switch (result) {
          case Ok(data: final profile):
            expect(profile.birthDate, isNull);
            expect(profile.referralCode, '');
          case Err():
            fail('expected Ok');
        }
      });

      test('maps an unauthorized error envelope through untouched', () async {
        final adapter = FakeAdapter(
          (options) async => jsonResponseBody({
            'error': {'code': 'unauthorized', 'message': 'missing auth'},
          }, 401),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = ProfileApi(dio);

        final result = await api.fetchMe();

        switch (result) {
          case Err(failure: final failure):
            expect(failure.code, 'unauthorized');
          case Ok():
            fail('expected Err');
        }
      });
    });

    group('fetchEntitlement', () {
      test('parses an active entitlement with an until date', () async {
        final adapter = FakeAdapter(
          (options) async => jsonResponseBody({
            'data': {'active': true, 'until': '2025-06-01T00:00:00Z'},
          }, 200),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = ProfileApi(dio);

        final result = await api.fetchEntitlement();

        expect(adapter.requests.single.path, '/me/entitlement');
        switch (result) {
          case Ok(data: final entitlement):
            expect(entitlement.active, isTrue);
            expect(entitlement.until, DateTime.parse('2025-06-01T00:00:00Z'));
          case Err():
            fail('expected Ok');
        }
      });

      test('parses an inactive entitlement with a null until', () async {
        final adapter = FakeAdapter(
          (options) async => jsonResponseBody({
            'data': {'active': false, 'until': null},
          }, 200),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = ProfileApi(dio);

        final result = await api.fetchEntitlement();

        switch (result) {
          case Ok(data: final entitlement):
            expect(entitlement.active, isFalse);
            expect(entitlement.until, isNull);
          case Err():
            fail('expected Ok');
        }
      });

      test('maps a network failure to a network_error Failure', () async {
        final adapter = FakeAdapter(
          (options) async => throw DioException(
            requestOptions: options,
            type: DioExceptionType.connectionError,
            message: 'Failed host lookup',
          ),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = ProfileApi(dio);

        final result = await api.fetchEntitlement();

        switch (result) {
          case Err(failure: final failure):
            expect(failure.code, 'network_error');
          case Ok():
            fail('expected Err');
        }
      });
    });
  });
}
