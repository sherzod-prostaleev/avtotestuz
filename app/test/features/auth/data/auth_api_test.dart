import 'dart:convert';
import 'dart:typed_data';

import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/auth/data/auth_api.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

/// A hand-rolled [HttpClientAdapter] that never touches the real network —
/// same pattern as `test/core/network/auth_interceptor_test.dart`. Every
/// call to [fetch] is recorded and answered by [handler].
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
  group('AuthApi', () {
    group('requestOtp', () {
      test('sends {phone} and maps a successful envelope', () async {
        final adapter = FakeAdapter(
          (options) async => jsonResponseBody({
            'data': {'channel': 'sandbox', 'debug_code': '123456'},
          }, 200),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = AuthApi(dio);

        final result = await api.requestOtp('901112233');

        expect(adapter.requests, hasLength(1));
        expect(adapter.requests.single.path, '/auth/otp/request');
        expect(adapter.requests.single.data, {'phone': '901112233'});
        switch (result) {
          case Ok(data: final data):
            expect(data.channel, 'sandbox');
            expect(data.debugCode, '123456');
          case Err():
            fail('expected Ok');
        }
      });

      test('debug_code is null when the backend omits it (non-sandbox)',
          () async {
        final adapter = FakeAdapter(
          (options) async => jsonResponseBody({
            'data': {'channel': 'sms'},
          }, 200),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = AuthApi(dio);

        final result = await api.requestOtp('901112233');

        switch (result) {
          case Ok(data: final data):
            expect(data.channel, 'sms');
            expect(data.debugCode, isNull);
          case Err():
            fail('expected Ok');
        }
      });

      test('maps a rate_limited (429) error envelope through untouched',
          () async {
        final adapter = FakeAdapter(
          (options) async => jsonResponseBody({
            'error': {
              'code': 'rate_limited',
              'message': 'too many requests, try again later',
            },
          }, 429),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = AuthApi(dio);

        final result = await api.requestOtp('901112233');

        switch (result) {
          case Err(failure: final failure):
            expect(failure.code, 'rate_limited');
          case Ok():
            fail('expected Err');
        }
      });

      test('maps an invalid_phone error envelope through untouched',
          () async {
        final adapter = FakeAdapter(
          (options) async => jsonResponseBody({
            'error': {
              'code': 'invalid_phone',
              'message': 'phone number is not a valid Uzbekistan number',
            },
          }, 400),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = AuthApi(dio);

        final result = await api.requestOtp('not-a-phone');

        switch (result) {
          case Err(failure: final failure):
            expect(failure.code, 'invalid_phone');
          case Ok():
            fail('expected Err');
        }
      });
    });

    group('verifyOtp', () {
      test('sends {phone, code} and maps tokens from a successful envelope',
          () async {
        final adapter = FakeAdapter(
          (options) async => jsonResponseBody({
            'data': {
              'access_token': 'access-1',
              'refresh_token': 'refresh-1',
            },
          }, 200),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = AuthApi(dio);

        final result =
            await api.verifyOtp(phone: '901112233', code: '654321');

        expect(adapter.requests.single.path, '/auth/otp/verify');
        expect(
          adapter.requests.single.data,
          {'phone': '901112233', 'code': '654321'},
        );
        switch (result) {
          case Ok(data: final tokens):
            expect(tokens.access, 'access-1');
            expect(tokens.refresh, 'refresh-1');
          case Err():
            fail('expected Ok');
        }
      });

      for (final code in ['invalid_code', 'expired_code', 'too_many_attempts']) {
        test('maps a $code error envelope through untouched', () async {
          final adapter = FakeAdapter(
            (options) async => jsonResponseBody({
              'error': {'code': code, 'message': 'irrelevant for this test'},
            }, 400),
          );
          final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
            ..httpClientAdapter = adapter;
          final api = AuthApi(dio);

          final result =
              await api.verifyOtp(phone: '901112233', code: '000000');

          switch (result) {
            case Err(failure: final failure):
              expect(failure.code, code);
            case Ok():
              fail('expected Err');
          }
        });
      }
    });

    group('logout', () {
      test('sends {refresh_token} and returns Ok on a successful envelope',
          () async {
        final adapter = FakeAdapter(
          (options) async => jsonResponseBody({
            'data': {'ok': true},
          }, 200),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = AuthApi(dio);

        final result = await api.logout('refresh-1');

        expect(adapter.requests.single.path, '/auth/logout');
        expect(adapter.requests.single.data, {'refresh_token': 'refresh-1'});
        expect(result, isA<Ok<void>>());
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
        final api = AuthApi(dio);

        final result = await api.logout('refresh-1');

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
