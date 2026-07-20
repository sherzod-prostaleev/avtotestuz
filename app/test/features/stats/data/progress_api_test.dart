import 'dart:convert';
import 'dart:typed_data';

import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/stats/data/progress_api.dart';
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

ProgressApi _apiWith(FakeAdapter adapter) {
  final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
    ..httpClientAdapter = adapter;
  return ProgressApi(dio);
}

void main() {
  group('ProgressApi.streak', () {
    test('parses a full streak with a last_active_date', () async {
      final adapter = FakeAdapter(
        (options) async => jsonResponseBody({
          'data': {
            'current': 5,
            'best': 12,
            'today_done': 3,
            'daily_goal': 10,
            'last_active_date': '2026-07-20',
          },
        }, 200),
      );
      final api = _apiWith(adapter);

      final result = await api.streak();

      expect(adapter.requests.single.path, '/me/streak');
      switch (result) {
        case Ok(data: final streak):
          expect(streak.current, 5);
          expect(streak.best, 12);
          expect(streak.todayDone, 3);
          expect(streak.dailyGoal, 10);
          expect(streak.lastActiveDate, '2026-07-20');
        case Err():
          fail('expected Ok, got $result');
      }
    });

    test('parses a never-active streak (null last_active_date)', () async {
      final adapter = FakeAdapter(
        (options) async => jsonResponseBody({
          'data': {
            'current': 0,
            'best': 0,
            'today_done': 0,
            'daily_goal': 10,
            'last_active_date': null,
          },
        }, 200),
      );
      final api = _apiWith(adapter);

      final result = await api.streak();

      switch (result) {
        case Ok(data: final streak):
          expect(streak.current, 0);
          expect(streak.lastActiveDate, isNull);
        case Err():
          fail('expected Ok, got $result');
      }
    });

    test('maps a server error envelope to Result.err', () async {
      final adapter = FakeAdapter(
        (options) async => jsonResponseBody({
          'error': {'code': 'unauthorized', 'message': 'missing auth'},
        }, 401),
      );
      final api = _apiWith(adapter);

      final result = await api.streak();

      switch (result) {
        case Ok():
          fail('expected Err, got $result');
        case Err(failure: final f):
          expect(f.code, 'unauthorized');
      }
    });
  });

  group('ProgressApi.stats', () {
    test('parses categories, readiness_pct and due_count', () async {
      final adapter = FakeAdapter(
        (options) async => jsonResponseBody({
          'data': {
            'categories': [
              {
                'category_code': 'signs',
                'mastery': 0.75,
                'seen': 40,
                'correct': 30,
              },
              // `mastery` sent as a whole number (int on the wire) must still
              // parse as a double.
              {
                'category_code': 'traffic',
                'mastery': 1,
                'seen': 10,
                'correct': 10,
              },
            ],
            'readiness_pct': 62,
            'due_count': 4,
          },
        }, 200),
      );
      final api = _apiWith(adapter);

      final result = await api.stats();

      expect(adapter.requests.single.path, '/me/stats');
      switch (result) {
        case Ok(data: final stats):
          expect(stats.categories, hasLength(2));
          expect(stats.categories[0].categoryCode, 'signs');
          expect(stats.categories[0].mastery, 0.75);
          expect(stats.categories[0].seen, 40);
          expect(stats.categories[0].correct, 30);
          expect(stats.categories[1].mastery, 1.0);
          expect(stats.readinessPct, 62);
          expect(stats.dueCount, 4);
        case Err():
          fail('expected Ok, got $result');
      }
    });

    test('parses an empty categories list (fresh account)', () async {
      final adapter = FakeAdapter(
        (options) async => jsonResponseBody({
          'data': {
            'categories': <dynamic>[],
            'readiness_pct': 0,
            'due_count': 0,
          },
        }, 200),
      );
      final api = _apiWith(adapter);

      final result = await api.stats();

      switch (result) {
        case Ok(data: final stats):
          expect(stats.categories, isEmpty);
          expect(stats.readinessPct, 0);
          expect(stats.dueCount, 0);
        case Err():
          fail('expected Ok, got $result');
      }
    });

    test('maps a server error envelope to Result.err', () async {
      final adapter = FakeAdapter(
        (options) async => jsonResponseBody({
          'error': {'code': 'internal', 'message': 'unexpected error'},
        }, 500),
      );
      final api = _apiWith(adapter);

      final result = await api.stats();

      switch (result) {
        case Ok():
          fail('expected Err, got $result');
        case Err(failure: final f):
          expect(f.code, 'internal');
      }
    });
  });
}
