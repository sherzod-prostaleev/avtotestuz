import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/result.dart';
import '../domain/session_models.dart';

/// Supplies the [SessionApi] the app's session-consuming controllers talk
/// to. Has no usable default — a real instance (backed by the same
/// configured [Dio] `main.dart` already built for `AuthApi`/`ProfileApi`/
/// `ContentApi`) is wired at the app root, same "no default, overridden at
/// the app root" seam as `contentApiProvider`. Tests override this directly
/// with a mock.
final sessionApiProvider = Provider<SessionApi>((ref) {
  throw UnimplementedError(
    'sessionApiProvider has no default — override it at the app root with '
    'a real SessionApi.',
  );
});

/// Thin [Dio] wrapper around the endpoints documented in the README's
/// "Sessiya / test yechish" section. Every call is wrapped in [guard] so raw
/// [DioException]s never leak past this layer (same convention as
/// `features/content/data/content_api.dart`).
///
/// Request/response shapes below are exactly as documented in the README —
/// this endpoint family (unlike Task 1's content DTOs) is fully spelled out
/// in prose, so no Go-source reading was needed to confirm field names:
/// ```
/// POST /sessions {mode, variant_id?, category_id?, sign_id?, locale, count?}
///   -> {id, mode, question_ids[], time_limit_sec, total, started_at}
/// POST /sessions/{id}/answers {question_id, answer_id}
///   -> {recorded, correct?, correct_answer_id?, stopped?, stop_reason?}
/// POST /sessions/{id}/finish  (no body)
///   -> {status, stopped_reason, score, total}
/// GET /sessions/{id}
///   -> {id, mode, total, status, stopped_reason, score?, started_at,
///       finished_at?, answers:[{question_id, position, answered, correct?}]}
/// GET /me/variants
///   -> [{number, question_count, unlocked, best_correct, attempts, completed_at?}]
/// ```
/// all wrapped in the standard envelope `{"data": ...}` /
/// `{"error": {"code": ..., "message": ...}}` (`backend/internal/httpx`).
///
/// Error-code mapping: [guard] (via `core/result.dart`'s
/// `_errorEnvelope`) already extracts the backend's `error.code` verbatim
/// into `Failure.code` — there is no separate mapping/rewriting step here,
/// deliberately, so every code the README documents for this endpoint family
/// (`invalid_request`, `vip_required` (402), `daily_limit_reached` (429,
/// `practice` only), `not_found`, `already_answered` (409),
/// `invalid_answer`, `session_finished` (409)) reaches the UI as its own
/// distinct, real code — none of them are collapsed into a generic
/// `"unknown"`/error-banner code. `vip_required`/`daily_limit_reached` in
/// particular must stay distinct since later screens (Variants,
/// Practice/Mistakes) react to them specifically (VIP upsell vs. a
/// "kunlik limitga yetdingiz" message).
///
/// Optional `POST /sessions` fields (`variant_id`/`category_id`/`sign_id`/
/// `count`) are omitted from the request body entirely when null, rather
/// than sent as explicit `null`s — mirroring `ContentApi.signs`'s
/// `group`/`query` convention.
class SessionApi {
  SessionApi(this._dio);

  final Dio _dio;

  Future<Result<SessionSummary>> start({
    required String mode,
    String? variantId,
    String? categoryId,
    String? signId,
    required String locale,
    int? count,
  }) {
    return guard(() async {
      final response = await _dio.post<Map<String, dynamic>>(
        '/sessions',
        data: {
          'mode': mode,
          'locale': locale,
          'variant_id': ?variantId,
          'category_id': ?categoryId,
          'sign_id': ?signId,
          'count': ?count,
        },
      );
      final data = response.data!['data'] as Map<String, dynamic>;
      return _sessionSummaryFromJson(data);
    });
  }

  Future<Result<AnswerResult>> answer({
    required String sessionId,
    required String questionId,
    required String answerId,
  }) {
    return guard(() async {
      final response = await _dio.post<Map<String, dynamic>>(
        '/sessions/$sessionId/answers',
        data: {'question_id': questionId, 'answer_id': answerId},
      );
      final data = response.data!['data'] as Map<String, dynamic>;
      return _answerResultFromJson(data);
    });
  }

  Future<Result<SessionResult>> finish(String sessionId) {
    return guard(() async {
      final response = await _dio.post<Map<String, dynamic>>(
        '/sessions/$sessionId/finish',
      );
      final data = response.data!['data'] as Map<String, dynamic>;
      return _sessionResultFromJson(data);
    });
  }

  Future<Result<SessionDetail>> get(String sessionId) {
    return guard(() async {
      final response = await _dio.get<Map<String, dynamic>>(
        '/sessions/$sessionId',
      );
      final data = response.data!['data'] as Map<String, dynamic>;
      return _sessionDetailFromJson(data);
    });
  }

  Future<Result<List<VariantStatus>>> myVariants() {
    return guard(() async {
      final response = await _dio.get<Map<String, dynamic>>('/me/variants');
      final data = response.data!['data'] as List<dynamic>;
      return data
          .map((e) => _variantStatusFromJson(e as Map<String, dynamic>))
          .toList();
    });
  }
}

SessionSummary _sessionSummaryFromJson(Map<String, dynamic> json) {
  final questionIds = json['question_ids'] as List<dynamic>;
  return SessionSummary(
    id: json['id'] as String,
    mode: json['mode'] as String,
    questionIds: questionIds.map((e) => e as String).toList(),
    // Genuinely nullable: the backend sends `null` for every non-exam mode
    // (see SessionSummary's doc comment). A hard `as int` here was the real
    // crash site of the live "type 'Null' is not a subtype of type 'int'"
    // bug — the model was made nullable but this cast was missed.
    timeLimitSec: json['time_limit_sec'] as int?,
    total: json['total'] as int,
    startedAt: DateTime.parse(json['started_at'] as String),
  );
}

AnswerResult _answerResultFromJson(Map<String, dynamic> json) {
  return AnswerResult(
    recorded: json['recorded'] as bool,
    correct: json['correct'] as bool?,
    correctAnswerId: json['correct_answer_id'] as String?,
    stopped: json['stopped'] as bool? ?? false,
    stopReason: json['stop_reason'] as String?,
  );
}

SessionResult _sessionResultFromJson(Map<String, dynamic> json) {
  return SessionResult(
    status: json['status'] as String,
    stoppedReason: json['stopped_reason'] as String,
    score: json['score'] as int,
    total: json['total'] as int,
  );
}

SessionDetail _sessionDetailFromJson(Map<String, dynamic> json) {
  final answers = json['answers'] as List<dynamic>;
  final finishedAt = json['finished_at'] as String?;
  return SessionDetail(
    id: json['id'] as String,
    mode: json['mode'] as String,
    total: json['total'] as int,
    status: json['status'] as String,
    stoppedReason: json['stopped_reason'] as String,
    score: json['score'] as int?,
    startedAt: DateTime.parse(json['started_at'] as String),
    finishedAt: finishedAt == null ? null : DateTime.parse(finishedAt),
    answers: answers
        .map((e) => _sessionAnswerRecordFromJson(e as Map<String, dynamic>))
        .toList(),
  );
}

SessionAnswerRecord _sessionAnswerRecordFromJson(Map<String, dynamic> json) {
  return SessionAnswerRecord(
    questionId: json['question_id'] as String,
    position: json['position'] as int,
    answered: json['answered'] as bool,
    correct: json['correct'] as bool?,
  );
}

VariantStatus _variantStatusFromJson(Map<String, dynamic> json) {
  final completedAt = json['completed_at'] as String?;
  return VariantStatus(
    number: json['number'] as int,
    questionCount: json['question_count'] as int,
    unlocked: json['unlocked'] as bool,
    bestCorrect: json['best_correct'] as int,
    attempts: json['attempts'] as int,
    completedAt: completedAt == null ? null : DateTime.parse(completedAt),
  );
}
