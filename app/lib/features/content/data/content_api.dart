import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/result.dart';
import '../domain/category.dart';
import '../domain/question.dart';
import '../domain/sign.dart';
import '../domain/variant.dart';

/// Supplies the [ContentApi] the app's content-consuming controllers talk
/// to. Has no usable default — a real instance (backed by the same
/// configured [Dio] `main.dart` already built for `AuthApi`/`ProfileApi`) is
/// wired at the app root, same "no default, overridden at the app root" seam
/// as `authRepositoryProvider`/`profileApiProvider`. Tests override this
/// directly with a mock.
final contentApiProvider = Provider<ContentApi>((ref) {
  throw UnimplementedError(
    'contentApiProvider has no default — override it at the app root with '
    'a real ContentApi.',
  );
});

/// Thin [Dio] wrapper around `backend/internal/content/handlers.go`'s
/// read-only content endpoints. Every call is wrapped in [guard] so raw
/// [DioException]s never leak past this layer (same convention as
/// `features/auth/data/auth_api.dart`/`features/profile/data/profile_api.dart`).
///
/// Request/response shapes below were confirmed by reading
/// `backend/internal/content/handlers.go` and `dto.go` directly (not just
/// README prose):
/// ```
/// GET /categories?locale=          -> {"data": CategoryDTO[], "meta": {...}}
/// GET /variants                    -> {"data": VariantListItemDTO[]}       // no locale
/// GET /variants/{number}?locale=   -> {"data": VariantDetailDTO, "meta": {...}}
/// GET /signs?locale=&group=&q=     -> {"data": SignListItemDTO[]}
/// GET /signs/{code}?locale=        -> {"data": SignDetailDTO}
/// GET /questions/{id}?locale=      -> {"data": QuestionDetailDTO, "meta": {...}}
/// ```
/// all wrapped in the standard envelope `{"data": ...}` /
/// `{"error": {"code": ..., "message": ...}}` (`backend/internal/httpx`).
/// `meta` (locale/fallback info) is deliberately not modeled here — this
/// task's domain classes only need `data`.
///
/// `group`/`q` (`signs()`'s `group`/`query` params) are omitted from the
/// request entirely when null/absent, rather than sent as empty strings —
/// `listSigns`'s handler treats a present-but-empty `group`/`q` as "no
/// filter" via its SQL, but sending nothing is the more honest client-side
/// signal of "the caller didn't ask to filter."
class ContentApi {
  ContentApi(this._dio);

  final Dio _dio;

  Future<Result<List<Category>>> categories({required String locale}) {
    return guard(() async {
      final response = await _dio.get<Map<String, dynamic>>(
        '/categories',
        queryParameters: {'locale': locale},
      );
      final data = response.data!['data'] as List<dynamic>;
      return data
          .map((e) => _categoryFromJson(e as Map<String, dynamic>))
          .toList();
    });
  }

  Future<Result<List<Variant>>> variants() {
    return guard(() async {
      final response = await _dio.get<Map<String, dynamic>>('/variants');
      final data = response.data!['data'] as List<dynamic>;
      return data
          .map((e) => _variantListItemFromJson(e as Map<String, dynamic>))
          .toList();
    });
  }

  Future<Result<Variant>> variantByNumber(int n, {required String locale}) {
    return guard(() async {
      final response = await _dio.get<Map<String, dynamic>>(
        '/variants/$n',
        queryParameters: {'locale': locale},
      );
      final data = response.data!['data'] as Map<String, dynamic>;
      return _variantDetailFromJson(data);
    });
  }

  Future<Result<List<Sign>>> signs({
    String? group,
    String? query,
    required String locale,
  }) {
    return guard(() async {
      final response = await _dio.get<Map<String, dynamic>>(
        '/signs',
        queryParameters: {
          'locale': locale,
          'group': ?group,
          'q': ?query,
        },
      );
      final data = response.data!['data'] as List<dynamic>;
      return data
          .map((e) => _signListItemFromJson(e as Map<String, dynamic>))
          .toList();
    });
  }

  Future<Result<Sign>> signByCode(String code, {required String locale}) {
    return guard(() async {
      final response = await _dio.get<Map<String, dynamic>>(
        '/signs/$code',
        queryParameters: {'locale': locale},
      );
      final data = response.data!['data'] as Map<String, dynamic>;
      return _signDetailFromJson(data);
    });
  }

  Future<Result<Question>> question(String id, {required String locale}) {
    return guard(() async {
      final response = await _dio.get<Map<String, dynamic>>(
        '/questions/$id',
        queryParameters: {'locale': locale},
      );
      final data = response.data!['data'] as Map<String, dynamic>;
      return _questionDetailFromJson(data);
    });
  }
}

Category _categoryFromJson(Map<String, dynamic> json) {
  return Category(
    code: json['code'] as String,
    name: json['name'] as String,
    sortOrder: json['sort_order'] as int,
  );
}

Variant _variantListItemFromJson(Map<String, dynamic> json) {
  return Variant(
    number: json['number'] as int,
    questionCount: json['question_count'] as int,
  );
}

Variant _variantDetailFromJson(Map<String, dynamic> json) {
  final questions = json['questions'] as List<dynamic>;
  return Variant(
    number: json['number'] as int,
    questions: questions
        .map((e) => _questionFromJson(e as Map<String, dynamic>))
        .toList(),
  );
}

Sign _signListItemFromJson(Map<String, dynamic> json) {
  return Sign(
    code: json['code'] as String,
    groupCode: json['group_code'] as String,
    name: json['name'] as String,
    imageUrl: json['image_url'] as String?,
    questionCount: json['question_count'] as int,
  );
}

Sign _signDetailFromJson(Map<String, dynamic> json) {
  final questionIds = json['question_ids'] as List<dynamic>;
  return Sign(
    code: json['code'] as String,
    groupCode: json['group_code'] as String,
    name: json['name'] as String,
    description: json['description'] as String,
    imageUrl: json['image_url'] as String?,
    questionIds: questionIds.map((e) => e as String).toList(),
  );
}

Sign _signChipFromJson(Map<String, dynamic> json) {
  return Sign(
    code: json['code'] as String,
    name: json['name'] as String,
    imageUrl: json['image_url'] as String?,
  );
}

/// Parses the variant-embedded `Question` shape (`QuestionDTO`): has
/// `position`, never has `signs`/`explanation` keys at all.
Question _questionFromJson(Map<String, dynamic> json) {
  final answers = json['answers'] as List<dynamic>;
  return Question(
    id: json['id'] as String,
    position: json['position'] as int?,
    categoryCode: json['category_code'] as String,
    text: json['text'] as String,
    imageUrl: json['image_url'] as String?,
    answers: answers
        .map((e) => _answerFromJson(e as Map<String, dynamic>))
        .toList(),
  );
}

/// Parses the standalone `GET /questions/{id}` shape (`QuestionDetailDTO`):
/// no `position` key (omitted server-side via `omitempty`), plus `signs`
/// and a nullable `explanation`.
Question _questionDetailFromJson(Map<String, dynamic> json) {
  final answers = json['answers'] as List<dynamic>;
  final signs = json['signs'] as List<dynamic>;
  final explanation = json['explanation'] as Map<String, dynamic>?;
  return Question(
    id: json['id'] as String,
    position: json['position'] as int?,
    categoryCode: json['category_code'] as String,
    text: json['text'] as String,
    imageUrl: json['image_url'] as String?,
    answers: answers
        .map((e) => _answerFromJson(e as Map<String, dynamic>))
        .toList(),
    signs: signs
        .map((e) => _signChipFromJson(e as Map<String, dynamic>))
        .toList(),
    explanation:
        explanation == null ? null : _explanationFromJson(explanation),
  );
}

Answer _answerFromJson(Map<String, dynamic> json) {
  return Answer(
    id: json['id'] as String,
    position: json['position'] as int,
    text: json['text'] as String,
    imageUrl: json['image_url'] as String?,
  );
}

Explanation _explanationFromJson(Map<String, dynamic> json) {
  final legalRefs = json['legal_refs'] as List<dynamic>;
  final blocks = json['blocks'] as List<dynamic>;
  return Explanation(
    legalRefs: legalRefs
        .map((e) => _legalRefFromJson(e as Map<String, dynamic>))
        .toList(),
    blocks: blocks
        .map((e) => _blockFromJson(e as Map<String, dynamic>))
        .toList(),
  );
}

LegalRef _legalRefFromJson(Map<String, dynamic> json) {
  return LegalRef(code: json['code'] as String, title: json['title'] as String);
}

ExplanationBlock _blockFromJson(Map<String, dynamic> json) {
  final items = json['items'] as List<dynamic>?;
  return ExplanationBlock(
    type: json['type'] as String,
    text: json['text'] as String?,
    items: items == null
        ? const []
        : items
            .map((e) => _answerAnalysisItemFromJson(e as Map<String, dynamic>))
            .toList(),
  );
}

AnswerAnalysisItem _answerAnalysisItemFromJson(Map<String, dynamic> json) {
  return AnswerAnalysisItem(
    position: json['position'] as int,
    correct: json['correct'] as bool,
    text: json['text'] as String,
  );
}
