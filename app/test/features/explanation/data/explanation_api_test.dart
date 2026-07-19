import 'dart:convert';
import 'dart:typed_data';

import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/explanation/data/explanation_api.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

/// A hand-rolled [HttpClientAdapter] that never touches the real network —
/// same pattern as `test/features/profile/data/profile_api_test.dart`.
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
  group('ExplanationApi.feedback', () {
    test('posts question_id/helpful and succeeds on {ok:true}', () async {
      final adapter = FakeAdapter(
        (options) async => jsonResponseBody({
          'data': {'ok': true},
        }, 200),
      );
      final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
        ..httpClientAdapter = adapter;
      final api = ExplanationApi(dio);

      final result = await api.feedback(questionId: 'q1', helpful: true);

      expect(adapter.requests.single.path, '/explanations/feedback');
      expect(adapter.requests.single.method, 'POST');
      expect(adapter.requests.single.data, {
        'question_id': 'q1',
        'helpful': true,
      });
      switch (result) {
        case Ok():
          break;
        case Err():
          fail('expected Ok');
      }
    });

    test('sends helpful:false verbatim', () async {
      final adapter = FakeAdapter(
        (options) async => jsonResponseBody({
          'data': {'ok': true},
        }, 200),
      );
      final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
        ..httpClientAdapter = adapter;
      final api = ExplanationApi(dio);

      await api.feedback(questionId: 'q2', helpful: false);

      expect(adapter.requests.single.data, {
        'question_id': 'q2',
        'helpful': false,
      });
    });

    test('maps a 404 not_found error envelope to Failure.code=not_found', () async {
      final adapter = FakeAdapter(
        (options) async => jsonResponseBody({
          'error': {'code': 'not_found', 'message': 'explanation not found'},
        }, 404),
      );
      final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
        ..httpClientAdapter = adapter;
      final api = ExplanationApi(dio);

      final result = await api.feedback(questionId: 'missing', helpful: true);

      switch (result) {
        case Ok():
          fail('expected Err');
        case Err(:final failure):
          expect(failure.code, 'not_found');
          expect(failure.message, 'explanation not found');
      }
    });
  });
}
