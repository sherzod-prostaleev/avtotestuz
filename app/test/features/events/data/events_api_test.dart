import 'dart:convert';
import 'dart:typed_data';

import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/events/data/events_api.dart';
import 'package:avtotest_app/features/events/domain/client_event.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

/// A hand-rolled [HttpClientAdapter] that never touches the real network —
/// same pattern as `test/features/content/data/content_api_test.dart`.
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
  group('EventsApi', () {
    group('logBatch', () {
      test('sends events and parses count from the response', () async {
        final adapter = FakeAdapter(
          (options) async =>
              jsonResponseBody({'data': {'ok': true, 'count': 2}}, 200),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = EventsApi(dio);

        final result = await api.logBatch([
          ClientEvent(
            name: 'view_question',
            props: {'question_id': 'q1'},
            ts: DateTime.utc(2026, 1, 1, 12),
          ),
          const ClientEvent(name: 'session_finish'),
        ]);

        expect(adapter.requests.single.path, '/events');
        final body = adapter.requests.single.data as Map<String, dynamic>;
        final events = body['events'] as List<dynamic>;
        expect(events, hasLength(2));
        expect(events[0], {
          'name': 'view_question',
          'props': {'question_id': 'q1'},
          'ts': '2026-01-01T12:00:00.000Z',
        });
        expect(events[1], {'name': 'session_finish'});

        switch (result) {
          case Ok(data: final count):
            expect(count, 2);
          case Err():
            fail('expected Ok');
        }
      });

      test('omits props/ts from the request when null', () async {
        final adapter = FakeAdapter(
          (options) async =>
              jsonResponseBody({'data': {'ok': true, 'count': 1}}, 200),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = EventsApi(dio);

        await api.logBatch(const [ClientEvent(name: 'view_question')]);

        final body = adapter.requests.single.data as Map<String, dynamic>;
        final events = body['events'] as List<dynamic>;
        expect(events.single, {'name': 'view_question'});
      });

      test('maps an invalid_request (400) error envelope through untouched',
          () async {
        final adapter = FakeAdapter(
          (options) async => jsonResponseBody({
            'error': {
              'code': 'invalid_request',
              'message': 'events batch must have 1-100 events',
            },
          }, 400),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = EventsApi(dio);

        final result = await api.logBatch(const []);

        switch (result) {
          case Err(failure: final failure):
            expect(failure.code, 'invalid_request');
          case Ok():
            fail('expected Err');
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
        final api = EventsApi(dio);

        final result = await api.logBatch(
          const [ClientEvent(name: 'view_question')],
        );

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
