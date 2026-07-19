import 'dart:convert';
import 'dart:typed_data';

import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/session/data/session_api.dart';
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

(Dio, FakeAdapter) buildDio(
  Future<ResponseBody> Function(RequestOptions options) handler,
) {
  final adapter = FakeAdapter(handler);
  final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
    ..httpClientAdapter = adapter;
  return (dio, adapter);
}

void main() {
  group('SessionApi', () {
    group('start', () {
      test('parses a SessionSummary and sends only the non-null fields',
          () async {
        final (dio, adapter) = buildDio(
          (options) async => jsonResponseBody({
            'data': {
              'id': 'sess-1',
              'mode': 'variant',
              'question_ids': ['q1', 'q2'],
              'time_limit_sec': 0,
              'total': 2,
              'started_at': '2026-07-18T10:00:00Z',
            },
          }, 200),
        );
        final api = SessionApi(dio);

        final result = await api.start(
          mode: 'variant',
          variantId: 'v1',
          locale: 'uz-Latn',
        );

        expect(adapter.requests.single.path, '/sessions');
        expect(
          adapter.requests.single.data,
          {'mode': 'variant', 'locale': 'uz-Latn', 'variant_id': 'v1'},
        );
        switch (result) {
          case Ok(data: final summary):
            expect(summary.id, 'sess-1');
            expect(summary.mode, 'variant');
            expect(summary.questionIds, ['q1', 'q2']);
            expect(summary.timeLimitSec, 0);
            expect(summary.total, 2);
            expect(
              summary.startedAt,
              DateTime.parse('2026-07-18T10:00:00Z'),
            );
          case Err():
            fail('expected Ok');
        }
      });

      test('parses an exam SessionSummary with a real time_limit_sec',
          () async {
        final (dio, _) = buildDio(
          (options) async => jsonResponseBody({
            'data': {
              'id': 'sess-2',
              'mode': 'exam',
              'question_ids': List.generate(20, (i) => 'q$i'),
              'time_limit_sec': 1500,
              'total': 20,
              'started_at': '2026-07-18T10:00:00Z',
            },
          }, 200),
        );
        final api = SessionApi(dio);

        final result = await api.start(mode: 'exam', locale: 'uz-Latn');

        switch (result) {
          case Ok(data: final summary):
            expect(summary.mode, 'exam');
            expect(summary.timeLimitSec, 1500);
            expect(summary.questionIds, hasLength(20));
          case Err():
            fail('expected Ok');
        }
      });

      test('maps a 402 vip_required error to a distinct Failure.code',
          () async {
        final (dio, _) = buildDio(
          (options) async => jsonResponseBody({
            'error': {
              'code': 'vip_required',
              'message': 'active entitlement required',
            },
          }, 402),
        );
        final api = SessionApi(dio);

        final result =
            await api.start(mode: 'exam', locale: 'uz-Latn');

        switch (result) {
          case Err(failure: final failure):
            expect(failure.code, 'vip_required');
          case Ok():
            fail('expected Err');
        }
      });

      test(
          'maps a 429 daily_limit_reached error to a distinct Failure.code, '
          'not collapsed with vip_required', () async {
        final (dio, _) = buildDio(
          (options) async => jsonResponseBody({
            'error': {
              'code': 'daily_limit_reached',
              'message': 'daily practice limit reached',
            },
          }, 429),
        );
        final api = SessionApi(dio);

        final result = await api.start(
          mode: 'practice',
          categoryId: 'signs',
          count: 10,
          locale: 'uz-Latn',
        );

        switch (result) {
          case Err(failure: final failure):
            expect(failure.code, 'daily_limit_reached');
            expect(failure.code, isNot('vip_required'));
          case Ok():
            fail('expected Err');
        }
      });

      test('maps a not_found (404) error envelope through untouched',
          () async {
        final (dio, _) = buildDio(
          (options) async => jsonResponseBody({
            'error': {'code': 'not_found', 'message': 'variant not found'},
          }, 404),
        );
        final api = SessionApi(dio);

        final result = await api.start(
          mode: 'variant',
          variantId: 'does-not-exist',
          locale: 'uz-Latn',
        );

        switch (result) {
          case Err(failure: final failure):
            expect(failure.code, 'not_found');
          case Ok():
            fail('expected Err');
        }
      });

      test('maps an invalid_request (400) error envelope through untouched',
          () async {
        final (dio, _) = buildDio(
          (options) async => jsonResponseBody({
            'error': {
              'code': 'invalid_request',
              'message': 'exactly one of category_id/sign_id required',
            },
          }, 400),
        );
        final api = SessionApi(dio);

        final result = await api.start(mode: 'practice', locale: 'uz-Latn');

        switch (result) {
          case Err(failure: final failure):
            expect(failure.code, 'invalid_request');
          case Ok():
            fail('expected Err');
        }
      });
    });

    group('answer', () {
      test('parses an immediate-feedback AnswerResult (variant/practice/'
          'mistakes)', () async {
        final (dio, adapter) = buildDio(
          (options) async => jsonResponseBody({
            'data': {
              'recorded': true,
              'correct': false,
              'correct_answer_id': 'a2',
            },
          }, 200),
        );
        final api = SessionApi(dio);

        final result = await api.answer(
          sessionId: 'sess-1',
          questionId: 'q1',
          answerId: 'a1',
        );

        expect(adapter.requests.single.path, '/sessions/sess-1/answers');
        expect(
          adapter.requests.single.data,
          {'question_id': 'q1', 'answer_id': 'a1'},
        );
        switch (result) {
          case Ok(data: final answer):
            expect(answer.recorded, isTrue);
            expect(answer.correct, isFalse);
            expect(answer.correctAnswerId, 'a2');
            expect(answer.stopped, isFalse);
            expect(answer.stopReason, isNull);
          case Err():
            fail('expected Ok');
        }
      });

      test(
          'parses a withheld-feedback AnswerResult (exam mode) — correct/'
          'correct_answer_id genuinely null, not defaulted', () async {
        final (dio, _) = buildDio(
          (options) async => jsonResponseBody({
            'data': {'recorded': true},
          }, 200),
        );
        final api = SessionApi(dio);

        final result = await api.answer(
          sessionId: 'sess-2',
          questionId: 'q1',
          answerId: 'a1',
        );

        switch (result) {
          case Ok(data: final answer):
            expect(answer.recorded, isTrue);
            expect(answer.correct, isNull);
            expect(answer.correctAnswerId, isNull);
            expect(answer.stopped, isFalse);
          case Err():
            fail('expected Ok');
        }
      });

      test('parses a stopped:true AnswerResult (3rd exam error)', () async {
        final (dio, _) = buildDio(
          (options) async => jsonResponseBody({
            'data': {
              'recorded': true,
              'stopped': true,
              'stop_reason': 'too_many_errors',
            },
          }, 200),
        );
        final api = SessionApi(dio);

        final result = await api.answer(
          sessionId: 'sess-2',
          questionId: 'q3',
          answerId: 'a1',
        );

        switch (result) {
          case Ok(data: final answer):
            expect(answer.correct, isNull);
            expect(answer.stopped, isTrue);
            expect(answer.stopReason, 'too_many_errors');
          case Err():
            fail('expected Ok');
        }
      });

      test('maps an already_answered (409) error envelope through untouched',
          () async {
        final (dio, _) = buildDio(
          (options) async => jsonResponseBody({
            'error': {
              'code': 'already_answered',
              'message': 'question already answered in this session',
            },
          }, 409),
        );
        final api = SessionApi(dio);

        final result = await api.answer(
          sessionId: 'sess-1',
          questionId: 'q1',
          answerId: 'a1',
        );

        switch (result) {
          case Err(failure: final failure):
            expect(failure.code, 'already_answered');
          case Ok():
            fail('expected Err');
        }
      });

      test('maps an invalid_answer (400) error envelope through untouched',
          () async {
        final (dio, _) = buildDio(
          (options) async => jsonResponseBody({
            'error': {
              'code': 'invalid_answer',
              'message': 'answer_id does not belong to question_id',
            },
          }, 400),
        );
        final api = SessionApi(dio);

        final result = await api.answer(
          sessionId: 'sess-1',
          questionId: 'q1',
          answerId: 'wrong-question-answer',
        );

        switch (result) {
          case Err(failure: final failure):
            expect(failure.code, 'invalid_answer');
          case Ok():
            fail('expected Err');
        }
      });

      test('maps a session_finished (409) error envelope through untouched',
          () async {
        final (dio, _) = buildDio(
          (options) async => jsonResponseBody({
            'error': {
              'code': 'session_finished',
              'message': 'session already finished',
            },
          }, 409),
        );
        final api = SessionApi(dio);

        final result = await api.answer(
          sessionId: 'sess-1',
          questionId: 'q1',
          answerId: 'a1',
        );

        switch (result) {
          case Err(failure: final failure):
            expect(failure.code, 'session_finished');
          case Ok():
            fail('expected Err');
        }
      });
    });

    group('finish', () {
      test('parses a passed SessionResult', () async {
        final (dio, adapter) = buildDio(
          (options) async => jsonResponseBody({
            'data': {
              'status': 'passed',
              'stopped_reason': 'completed',
              'score': 18,
              'total': 20,
            },
          }, 200),
        );
        final api = SessionApi(dio);

        final result = await api.finish('sess-1');

        expect(adapter.requests.single.path, '/sessions/sess-1/finish');
        switch (result) {
          case Ok(data: final r):
            expect(r.status, 'passed');
            expect(r.stoppedReason, 'completed');
            expect(r.score, 18);
            expect(r.total, 20);
          case Err():
            fail('expected Ok');
        }
      });

      test('parses a failed/too_many_errors SessionResult', () async {
        final (dio, _) = buildDio(
          (options) async => jsonResponseBody({
            'data': {
              'status': 'failed',
              'stopped_reason': 'too_many_errors',
              'score': 5,
              'total': 20,
            },
          }, 200),
        );
        final api = SessionApi(dio);

        final result = await api.finish('sess-2');

        switch (result) {
          case Ok(data: final r):
            expect(r.status, 'failed');
            expect(r.stoppedReason, 'too_many_errors');
          case Err():
            fail('expected Ok');
        }
      });

      test('maps a session_finished (409) error envelope through untouched',
          () async {
        final (dio, _) = buildDio(
          (options) async => jsonResponseBody({
            'error': {
              'code': 'session_finished',
              'message': 'session already finished',
            },
          }, 409),
        );
        final api = SessionApi(dio);

        final result = await api.finish('sess-1');

        switch (result) {
          case Err(failure: final failure):
            expect(failure.code, 'session_finished');
          case Ok():
            fail('expected Err');
        }
      });
    });

    group('get', () {
      test('parses an in-progress SessionDetail (resume) with hidden '
          'correctness', () async {
        final (dio, adapter) = buildDio(
          (options) async => jsonResponseBody({
            'data': {
              'id': 'sess-2',
              'mode': 'exam',
              'total': 20,
              'status': 'in_progress',
              'stopped_reason': '',
              'started_at': '2026-07-18T10:00:00Z',
              'answers': [
                {
                  'question_id': 'q1',
                  'position': 1,
                  'answered': true,
                  'correct': null,
                },
                {
                  'question_id': 'q2',
                  'position': 2,
                  'answered': false,
                },
              ],
            },
          }, 200),
        );
        final api = SessionApi(dio);

        final result = await api.get('sess-2');

        expect(adapter.requests.single.path, '/sessions/sess-2');
        switch (result) {
          case Ok(data: final detail):
            expect(detail.id, 'sess-2');
            expect(detail.mode, 'exam');
            expect(detail.status, 'in_progress');
            expect(detail.score, isNull);
            expect(detail.finishedAt, isNull);
            expect(detail.answers, hasLength(2));
            expect(detail.answers[0].questionId, 'q1');
            expect(detail.answers[0].answered, isTrue);
            expect(detail.answers[0].correct, isNull);
            expect(detail.answers[1].answered, isFalse);
          case Err():
            fail('expected Ok');
        }
      });

      test('parses a finished SessionDetail with score/finished_at', () async {
        final (dio, _) = buildDio(
          (options) async => jsonResponseBody({
            'data': {
              'id': 'sess-1',
              'mode': 'variant',
              'total': 2,
              'status': 'passed',
              'stopped_reason': 'completed',
              'score': 2,
              'started_at': '2026-07-18T10:00:00Z',
              'finished_at': '2026-07-18T10:05:00Z',
              'answers': [
                {
                  'question_id': 'q1',
                  'position': 1,
                  'answered': true,
                  'correct': true,
                },
                {
                  'question_id': 'q2',
                  'position': 2,
                  'answered': true,
                  'correct': true,
                },
              ],
            },
          }, 200),
        );
        final api = SessionApi(dio);

        final result = await api.get('sess-1');

        switch (result) {
          case Ok(data: final detail):
            expect(detail.status, 'passed');
            expect(detail.score, 2);
            expect(
              detail.finishedAt,
              DateTime.parse('2026-07-18T10:05:00Z'),
            );
            expect(detail.answers[0].correct, isTrue);
          case Err():
            fail('expected Ok');
        }
      });

      test('maps a not_found (404) error envelope through untouched',
          () async {
        final (dio, _) = buildDio(
          (options) async => jsonResponseBody({
            'error': {'code': 'not_found', 'message': 'session not found'},
          }, 404),
        );
        final api = SessionApi(dio);

        final result = await api.get('does-not-exist');

        switch (result) {
          case Err(failure: final failure):
            expect(failure.code, 'not_found');
          case Ok():
            fail('expected Err');
        }
      });
    });

    group('myVariants', () {
      test('parses a list of VariantStatus, including a locked bilet',
          () async {
        final (dio, adapter) = buildDio(
          (options) async => jsonResponseBody({
            'data': [
              {
                'number': 1,
                'question_count': 20,
                'unlocked': true,
                'best_correct': 18,
                'attempts': 2,
                'completed_at': '2026-07-17T09:00:00Z',
              },
              {
                'number': 2,
                'question_count': 20,
                'unlocked': false,
                'best_correct': 0,
                'attempts': 0,
              },
            ],
          }, 200),
        );
        final api = SessionApi(dio);

        final result = await api.myVariants();

        expect(adapter.requests.single.path, '/me/variants');
        switch (result) {
          case Ok(data: final variants):
            expect(variants, hasLength(2));
            expect(variants[0].number, 1);
            expect(variants[0].unlocked, isTrue);
            expect(variants[0].bestCorrect, 18);
            expect(
              variants[0].completedAt,
              DateTime.parse('2026-07-17T09:00:00Z'),
            );
            expect(variants[1].number, 2);
            expect(variants[1].unlocked, isFalse);
            expect(variants[1].completedAt, isNull);
          case Err():
            fail('expected Ok');
        }
      });
    });

    test('maps a network failure to a network_error Failure', () async {
      final (dio, _) = buildDio(
        (options) async => throw DioException(
          requestOptions: options,
          type: DioExceptionType.connectionError,
          message: 'Failed host lookup',
        ),
      );
      final api = SessionApi(dio);

      final result = await api.myVariants();

      switch (result) {
        case Err(failure: final failure):
          expect(failure.code, 'network_error');
        case Ok():
          fail('expected Err');
      }
    });
  });
}
