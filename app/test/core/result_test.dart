import 'package:avtotest_app/core/result.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('guard', () {
    test('wraps a successful call in Result.ok', () async {
      final result = await guard(() async => 'value');

      expect(result, isA<Ok<String>>());
      switch (result) {
        case Ok(:final data):
          expect(data, 'value');
        case Err():
          fail('expected Ok');
      }
    });

    test('maps a structured error envelope to a Failure with code/message',
        () async {
      final options = RequestOptions(path: '/x');
      final error = DioException(
        requestOptions: options,
        type: DioExceptionType.badResponse,
        response: Response(
          requestOptions: options,
          statusCode: 422,
          data: {
            'error': {'code': 'invalid_code', 'message': 'bad code'},
          },
        ),
      );

      final result = await guard<void>(() async => throw error);

      switch (result) {
        case Err(:final failure):
          expect(failure.code, 'invalid_code');
          expect(failure.message, 'bad code');
        case Ok():
          fail('expected Err');
      }
    });

    test('maps a connection failure to a network_error Failure', () async {
      final options = RequestOptions(path: '/x');
      final error = DioException(
        requestOptions: options,
        type: DioExceptionType.connectionError,
        message: 'Failed host lookup',
      );

      final result = await guard<void>(() async => throw error);

      switch (result) {
        case Err(:final failure):
          expect(failure.code, 'network_error');
          expect(failure.message, isNotEmpty);
        case Ok():
          fail('expected Err');
      }
    });

    test('maps connectionTimeout to a network_error Failure', () async {
      final options = RequestOptions(path: '/x');
      final error = DioException(
        requestOptions: options,
        type: DioExceptionType.connectionTimeout,
      );

      final result = await guard<void>(() async => throw error);

      switch (result) {
        case Err(:final failure):
          expect(failure.code, 'network_error');
        case Ok():
          fail('expected Err');
      }
    });

    test('maps an unstructured 5xx response to an unknown Failure', () async {
      final options = RequestOptions(path: '/x');
      final error = DioException(
        requestOptions: options,
        type: DioExceptionType.badResponse,
        response: Response(
          requestOptions: options,
          statusCode: 500,
          data: 'internal server error',
        ),
      );

      final result = await guard<void>(() async => throw error);

      switch (result) {
        case Err(:final failure):
          expect(failure.code, 'unknown');
        case Ok():
          fail('expected Err');
      }
    });

    test('maps a non-Dio exception to an unknown Failure', () async {
      final result = await guard<void>(() async => throw Exception('boom'));

      switch (result) {
        case Err(:final failure):
          expect(failure.code, 'unknown');
          expect(failure.message, contains('boom'));
        case Ok():
          fail('expected Err');
      }
    });
  });
}
