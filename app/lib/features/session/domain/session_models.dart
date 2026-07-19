import 'package:freezed_annotation/freezed_annotation.dart';

part 'session_models.freezed.dart';

/// `POST /api/v1/sessions`'s response shape, matching the README's
/// "Sessiya / test yechish" section exactly:
/// ```
/// { "id": string, "mode": string, "question_ids": string[],
///   "time_limit_sec": int, "total": int, "started_at": string(ISO8601) }
/// ```
/// `mode` is one of `variant`/`exam`/`practice`/`mistakes` — modeled as a
/// plain `String` (not a closed enum) since the README documents the set as
/// prose, not as a wire-level constraint the client should assume is
/// exhaustive/stable. `timeLimitSec` is documented without a `?` (unlike
/// `AnswerResult`'s genuinely-optional fields below), so it's required here
/// too — non-exam modes are expected to send some non-exam value (e.g. `0`)
/// rather than omitting the key.
@freezed
abstract class SessionSummary with _$SessionSummary {
  const factory SessionSummary({
    required String id,
    required String mode,
    required List<String> questionIds,
    required int timeLimitSec,
    required int total,
    required DateTime startedAt,
  }) = _SessionSummary;
}

/// `POST /api/v1/sessions/{id}/answers`'s response shape:
/// ```
/// { "recorded": bool, "correct"?: bool, "correct_answer_id"?: string,
///   "stopped"?: bool, "stop_reason"?: string }
/// ```
/// Anti-cheat: [correct]/[correctAnswerId] are genuinely nullable, not
/// defaulted to a sentinel — the backend withholds both in `exam` mode until
/// the session finishes (README: "Javob fikr-mulohazasi ... sessiya
/// tugagunicha yashiriladi"), so a `null` here must be rendered by the UI as
/// "no feedback yet," never inferred/guessed. [stopped] is defaulted to
/// `false` when absent (unlike the correctness fields, a missing/false
/// `stopped` carries no such ambiguity — "not stopped" is unambiguous).
/// [stopReason] (e.g. `"too_many_errors"` on the 3rd exam error) stays
/// nullable since it's only ever present when [stopped] is `true`.
@freezed
abstract class AnswerResult with _$AnswerResult {
  const factory AnswerResult({
    required bool recorded,
    bool? correct,
    String? correctAnswerId,
    @Default(false) bool stopped,
    String? stopReason,
  }) = _AnswerResult;
}

/// `POST /api/v1/sessions/{id}/finish`'s response shape:
/// ```
/// { "status": string, "stopped_reason": string, "score": int, "total": int }
/// ```
/// `status` is one of `"passed"`/`"failed"`/`"abandoned"`; `stoppedReason` is
/// one of `"completed"`/`"time_up"`/`"too_many_errors"` (README). Both
/// modeled as plain `String`s (not enums) for the same reason as
/// [SessionSummary.mode] above. Note the field name is `stopped_reason`
/// here (past participle) — distinct from [AnswerResult.stopReason]
/// (present tense) which is a different field on a different endpoint; don't
/// conflate the two when reading/writing JSON keys.
@freezed
abstract class SessionResult with _$SessionResult {
  const factory SessionResult({
    required String status,
    required String stoppedReason,
    required int score,
    required int total,
  }) = _SessionResult;
}

/// One entry in [SessionDetail.answers], matching the `answers[]` item shape
/// documented for `GET /api/v1/sessions/{id}`:
/// ```
/// { "question_id": string, "position": int, "answered": bool, "correct"?: bool }
/// ```
/// [correct] is nullable for the same anti-cheat reason as
/// [AnswerResult.correct] — "in-progress exam sessiyada har bir javob uchun
/// yashirilgan bo'ladi" (README).
@freezed
abstract class SessionAnswerRecord with _$SessionAnswerRecord {
  const factory SessionAnswerRecord({
    required String questionId,
    required int position,
    required bool answered,
    bool? correct,
  }) = _SessionAnswerRecord;
}

/// `GET /api/v1/sessions/{id}`'s response shape (used to resume an
/// in-progress session, per the task brief):
/// ```
/// { "id": string, "mode": string, "total": int, "status": string,
///   "stopped_reason": string, "score"?: int, "started_at": string(ISO8601),
///   "finished_at"?: string(ISO8601),
///   "answers": [{question_id, position, answered, correct?}, ...] }
/// ```
/// [score]/[finishedAt] are nullable since an in-progress (not-yet-finished)
/// session hasn't produced either yet.
@freezed
abstract class SessionDetail with _$SessionDetail {
  const factory SessionDetail({
    required String id,
    required String mode,
    required int total,
    required String status,
    required String stoppedReason,
    int? score,
    required DateTime startedAt,
    DateTime? finishedAt,
    required List<SessionAnswerRecord> answers,
  }) = _SessionDetail;
}

/// `GET /api/v1/me/variants`'s response shape (one entry per bilet):
/// ```
/// { "number": int, "question_count": int, "unlocked": bool,
///   "best_correct": int, "attempts": int, "completed_at"?: string(ISO8601) }
/// ```
@freezed
abstract class VariantStatus with _$VariantStatus {
  const factory VariantStatus({
    required int number,
    required int questionCount,
    required bool unlocked,
    required int bestCorrect,
    required int attempts,
    DateTime? completedAt,
  }) = _VariantStatus;
}
