import 'dart:convert';
import 'dart:typed_data';

import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/saved/data/saved_api.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

/// A hand-rolled [HttpClientAdapter] that never touches the real network —
/// same pattern as `test/features/explanation/data/explanation_api_test.dart`.
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
  group('SavedApi.list', () {
    test('parses a non-empty list of {question_id, created_at}', () async {
      final adapter = FakeAdapter(
        (options) async => jsonResponseBody({
          'data': [
            {'question_id': 'q1', 'created_at': '2026-01-01T00:00:00Z'},
            {'question_id': 'q2', 'created_at': '2026-01-02T00:00:00Z'},
          ],
        }, 200),
      );
      final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
        ..httpClientAdapter = adapter;
      final api = SavedApi(dio);

      final result = await api.list();

      expect(adapter.requests.single.path, '/me/saved');
      expect(adapter.requests.single.method, 'GET');
      switch (result) {
        case Ok(:final data):
          expect(data, hasLength(2));
          expect(data[0].questionId, 'q1');
          expect(data[0].createdAt, DateTime.parse('2026-01-01T00:00:00Z'));
          expect(data[1].questionId, 'q2');
        case Err():
          fail('expected Ok');
      }
    });

    test('parses an empty list without error', () async {
      final adapter = FakeAdapter(
        (options) async => jsonResponseBody({'data': <dynamic>[]}, 200),
      );
      final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
        ..httpClientAdapter = adapter;
      final api = SavedApi(dio);

      final result = await api.list();

      switch (result) {
        case Ok(:final data):
          expect(data, isEmpty);
        case Err():
          fail('expected Ok');
      }
    });

    test('a server error surfaces as Err with the backend code/message', () async {
      final adapter = FakeAdapter(
        (options) async => jsonResponseBody({
          'error': {'code': 'internal', 'message': 'unexpected error'},
        }, 500),
      );
      final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
        ..httpClientAdapter = adapter;
      final api = SavedApi(dio);

      final result = await api.list();

      switch (result) {
        case Ok():
          fail('expected Err');
        case Err(:final failure):
          expect(failure.code, 'internal');
          expect(failure.message, 'unexpected error');
      }
    });
  });

  group('SavedApi.save', () {
    test('posts question_id and succeeds on {ok:true}', () async {
      final adapter = FakeAdapter(
        (options) async => jsonResponseBody({
          'data': {'ok': true},
        }, 200),
      );
      final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
        ..httpClientAdapter = adapter;
      final api = SavedApi(dio);

      final result = await api.save('q1');

      expect(adapter.requests.single.path, '/me/saved');
      expect(adapter.requests.single.method, 'POST');
      expect(adapter.requests.single.data, {'question_id': 'q1'});
      switch (result) {
        case Ok():
          break;
        case Err():
          fail('expected Ok');
      }
    });
  });

  group('SavedApi.unsave', () {
    test('sends DELETE to /me/saved/{question_id} and succeeds', () async {
      final adapter = FakeAdapter(
        (options) async => jsonResponseBody({
          'data': {'ok': true},
        }, 200),
      );
      final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
        ..httpClientAdapter = adapter;
      final api = SavedApi(dio);

      final result = await api.unsave('q1');

      expect(adapter.requests.single.path, '/me/saved/q1');
      expect(adapter.requests.single.method, 'DELETE');
      switch (result) {
        case Ok():
          break;
        case Err():
          fail('expected Ok');
      }
    });

    test('an error response surfaces as Err', () async {
      final adapter = FakeAdapter(
        (options) async => jsonResponseBody({
          'error': {'code': 'unknown', 'message': 'boom'},
        }, 500),
      );
      final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
        ..httpClientAdapter = adapter;
      final api = SavedApi(dio);

      final result = await api.unsave('q1');

      switch (result) {
        case Ok():
          fail('expected Err');
        case Err(:final failure):
          expect(failure.code, 'unknown');
      }
    });
  });
}
