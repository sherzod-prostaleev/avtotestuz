import 'dart:convert';
import 'dart:typed_data';

import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/content/data/content_api.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

/// A hand-rolled [HttpClientAdapter] that never touches the real network —
/// same pattern as `test/features/auth/data/auth_api_test.dart` and
/// `test/features/profile/data/profile_api_test.dart`.
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
  group('ContentApi', () {
    group('categories', () {
      test('parses a list of categories and sends locale=', () async {
        final adapter = FakeAdapter(
          (options) async => jsonResponseBody({
            'data': [
              {'code': 'signs', 'name': 'Yo\'l belgilari', 'sort_order': 1},
              {'code': 'traffic', 'name': 'Harakat qoidalari', 'sort_order': 2},
            ],
            'meta': {'locale': 'uz-Latn', 'fallback': false},
          }, 200),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = ContentApi(dio);

        final result = await api.categories(locale: 'uz-Latn');

        expect(adapter.requests.single.path, '/categories');
        expect(
          adapter.requests.single.queryParameters,
          {'locale': 'uz-Latn'},
        );
        switch (result) {
          case Ok(data: final categories):
            expect(categories, hasLength(2));
            expect(categories[0].code, 'signs');
            expect(categories[0].name, "Yo'l belgilari");
            expect(categories[0].sortOrder, 1);
            expect(categories[1].code, 'traffic');
          case Err():
            fail('expected Ok');
        }
      });
    });

    group('variants', () {
      test('parses a list of variants without sending locale', () async {
        final adapter = FakeAdapter(
          (options) async => jsonResponseBody({
            'data': [
              {'number': 1, 'question_count': 20},
              {'number': 2, 'question_count': 20},
            ],
          }, 200),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = ContentApi(dio);

        final result = await api.variants();

        expect(adapter.requests.single.path, '/variants');
        expect(adapter.requests.single.queryParameters, isEmpty);
        switch (result) {
          case Ok(data: final variants):
            expect(variants, hasLength(2));
            expect(variants[0].number, 1);
            expect(variants[0].questionCount, 20);
            expect(variants[0].questions, isNull);
          case Err():
            fail('expected Ok');
        }
      });
    });

    group('variantByNumber', () {
      test('parses a variant detail with embedded questions', () async {
        final adapter = FakeAdapter(
          (options) async => jsonResponseBody({
            'data': {
              'number': 1,
              'questions': [
                {
                  'id': 'q1',
                  'position': 1,
                  'category_code': 'signs',
                  'text': 'Bu belgi nimani anglatadi?',
                  'image_url': 'https://media.test/q1.png',
                  'answers': [
                    {
                      'id': 'a1',
                      'position': 1,
                      'text': 'Tezlikni cheklash',
                      'image_url': null,
                    },
                    {
                      'id': 'a2',
                      'position': 2,
                      'text': "To'xtash taqiqlangan",
                      'image_url': null,
                    },
                  ],
                },
              ],
            },
            'meta': {'locale': 'uz-Latn', 'fallback': false},
          }, 200),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = ContentApi(dio);

        final result = await api.variantByNumber(1, locale: 'uz-Latn');

        expect(adapter.requests.single.path, '/variants/1');
        expect(
          adapter.requests.single.queryParameters,
          {'locale': 'uz-Latn'},
        );
        switch (result) {
          case Ok(data: final variant):
            expect(variant.number, 1);
            expect(variant.questionCount, isNull);
            expect(variant.questions, hasLength(1));
            final q = variant.questions!.single;
            expect(q.id, 'q1');
            expect(q.position, 1);
            expect(q.categoryCode, 'signs');
            expect(q.text, 'Bu belgi nimani anglatadi?');
            expect(q.imageUrl, 'https://media.test/q1.png');
            expect(q.answers, hasLength(2));
            expect(q.answers[0].id, 'a1');
            expect(q.answers[0].position, 1);
            expect(q.answers[1].text, "To'xtash taqiqlangan");
            // Variant-embedded questions never carry signs/explanation.
            expect(q.signs, isEmpty);
            expect(q.explanation, isNull);
          case Err():
            fail('expected Ok');
        }
      });

      test('maps a not_found (404) error envelope through untouched',
          () async {
        final adapter = FakeAdapter(
          (options) async => jsonResponseBody({
            'error': {'code': 'not_found', 'message': 'variant not found'},
          }, 404),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = ContentApi(dio);

        final result = await api.variantByNumber(999, locale: 'uz-Latn');

        switch (result) {
          case Err(failure: final failure):
            expect(failure.code, 'not_found');
          case Ok():
            fail('expected Err');
        }
      });
    });

    group('signs', () {
      test('parses a list of signs and sends group/q only when provided',
          () async {
        final adapter = FakeAdapter(
          (options) async => jsonResponseBody({
            'data': [
              {
                'code': '1.1',
                'group_code': 'warning',
                'name': 'Temiryo\'l kesishmasi',
                'image_url': 'https://media.test/1.1.png',
                'question_count': 3,
              },
            ],
          }, 200),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = ContentApi(dio);

        final result = await api.signs(
          group: 'warning',
          query: 'temir',
          locale: 'uz-Latn',
        );

        expect(adapter.requests.single.path, '/signs');
        expect(
          adapter.requests.single.queryParameters,
          {'locale': 'uz-Latn', 'group': 'warning', 'q': 'temir'},
        );
        switch (result) {
          case Ok(data: final signs):
            expect(signs, hasLength(1));
            expect(signs.single.code, '1.1');
            expect(signs.single.groupCode, 'warning');
            expect(signs.single.questionCount, 3);
            expect(signs.single.questionIds, isNull);
          case Err():
            fail('expected Ok');
        }
      });

      test('omits group/q from the request when not provided', () async {
        final adapter = FakeAdapter(
          (options) async => jsonResponseBody({'data': <dynamic>[]}, 200),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = ContentApi(dio);

        await api.signs(locale: 'uz-Latn');

        expect(
          adapter.requests.single.queryParameters,
          {'locale': 'uz-Latn'},
        );
      });
    });

    group('signByCode', () {
      test('parses a sign detail with question_ids', () async {
        final adapter = FakeAdapter(
          (options) async => jsonResponseBody({
            'data': {
              'code': '1.1',
              'group_code': 'warning',
              'name': 'Temiryo\'l kesishmasi',
              'description': 'Shlagbaumsiz temir yo\'l kesishmasi haqida ogohlantiradi.',
              'image_url': 'https://media.test/1.1.png',
              'question_ids': ['q1', 'q2'],
            },
          }, 200),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = ContentApi(dio);

        final result = await api.signByCode('1.1', locale: 'uz-Latn');

        expect(adapter.requests.single.path, '/signs/1.1');
        switch (result) {
          case Ok(data: final sign):
            expect(sign.code, '1.1');
            expect(sign.description, "Shlagbaumsiz temir yo'l kesishmasi haqida ogohlantiradi.");
            expect(sign.questionIds, ['q1', 'q2']);
            expect(sign.questionCount, isNull);
          case Err():
            fail('expected Ok');
        }
      });

      test('maps a not_found (404) error envelope through untouched',
          () async {
        final adapter = FakeAdapter(
          (options) async => jsonResponseBody({
            'error': {'code': 'not_found', 'message': 'sign not found'},
          }, 404),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = ContentApi(dio);

        final result = await api.signByCode('9.9', locale: 'uz-Latn');

        switch (result) {
          case Err(failure: final failure):
            expect(failure.code, 'not_found');
          case Ok():
            fail('expected Err');
        }
      });
    });

    group('question', () {
      test('parses a question detail with signs and a verified explanation',
          () async {
        final adapter = FakeAdapter(
          (options) async => jsonResponseBody({
            'data': {
              'id': 'q1',
              'category_code': 'signs',
              'text': 'Bu belgi nimani anglatadi?',
              'image_url': null,
              'answers': [
                {'id': 'a1', 'position': 1, 'text': 'A', 'image_url': null},
                {'id': 'a2', 'position': 2, 'text': 'B', 'image_url': null},
              ],
              'signs': [
                {'code': '1.1', 'name': 'Temiryo\'l kesishmasi', 'image_url': null},
              ],
              'explanation': {
                'legal_refs': [
                  {'code': 'YHQ 1.1', 'title': "Yo'l belgilari to'g'risida"},
                ],
                'blocks': [
                  {'type': 'intro', 'text': 'Kirish matni'},
                  {
                    'type': 'answer_analysis',
                    'items': [
                      {'position': 1, 'correct': false, 'text': "Noto'g'ri"},
                      {'position': 2, 'correct': true, 'text': "To'g'ri"},
                    ],
                  },
                ],
              },
            },
            'meta': {'locale': 'uz-Latn', 'fallback': false},
          }, 200),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = ContentApi(dio);

        final result = await api.question('q1', locale: 'uz-Latn');

        expect(adapter.requests.single.path, '/questions/q1');
        switch (result) {
          case Ok(data: final question):
            expect(question.id, 'q1');
            expect(question.position, isNull);
            expect(question.signs, hasLength(1));
            expect(question.signs.single.code, '1.1');
            expect(question.signs.single.groupCode, isNull);
            expect(question.explanation, isNotNull);
            expect(question.explanation!.legalRefs.single.code, 'YHQ 1.1');
            expect(question.explanation!.blocks, hasLength(2));
            expect(question.explanation!.blocks[0].type, 'intro');
            expect(question.explanation!.blocks[0].text, 'Kirish matni');
            expect(question.explanation!.blocks[1].type, 'answer_analysis');
            expect(question.explanation!.blocks[1].items, hasLength(2));
            expect(question.explanation!.blocks[1].items[1].correct, isTrue);
          case Err():
            fail('expected Ok');
        }
      });

      test('explanation is null when the question has none yet', () async {
        final adapter = FakeAdapter(
          (options) async => jsonResponseBody({
            'data': {
              'id': 'q2',
              'category_code': 'signs',
              'text': 'Savol matni',
              'image_url': null,
              'answers': <dynamic>[],
              'signs': <dynamic>[],
              'explanation': null,
            },
          }, 200),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = ContentApi(dio);

        final result = await api.question('q2', locale: 'uz-Latn');

        switch (result) {
          case Ok(data: final question):
            expect(question.explanation, isNull);
            expect(question.signs, isEmpty);
          case Err():
            fail('expected Ok');
        }
      });

      test('maps a not_found (404) error envelope through untouched',
          () async {
        final adapter = FakeAdapter(
          (options) async => jsonResponseBody({
            'error': {'code': 'not_found', 'message': 'question not found'},
          }, 404),
        );
        final dio = Dio(BaseOptions(baseUrl: 'https://api.test'))
          ..httpClientAdapter = adapter;
        final api = ContentApi(dio);

        final result = await api.question('does-not-exist', locale: 'uz-Latn');

        switch (result) {
          case Err(failure: final failure):
            expect(failure.code, 'not_found');
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
        final api = ContentApi(dio);

        final result = await api.question('q1', locale: 'uz-Latn');

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
