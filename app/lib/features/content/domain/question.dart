import 'package:freezed_annotation/freezed_annotation.dart';

import 'sign.dart';

part 'question.freezed.dart';

/// One answer option, matching `backend/internal/content/dto.go`'s
/// `AnswerDTO` exactly as embedded in a question's `answers[]` (confirmed by
/// reading `handlers.go`'s `getVariant`/`getQuestion`, which both populate
/// this shape from `ListAnswersByQuestionIDs`):
/// ```
/// { "id": string, "position": int, "text": string, "image_url": string | null }
/// ```
/// Deliberately has NO `is_correct`/`correct` field — the content endpoints
/// never send one (anti-cheat invariant, `content` package doc comment:
/// "It never exposes correctness fields — scoring is session-scoped").
@freezed
abstract class Answer with _$Answer {
  const factory Answer({
    required String id,
    required int position,
    required String text,
    String? imageUrl,
  }) = _Answer;
}

/// One `{code, title}` legal reference entry inside an explanation's
/// `legal_refs` array (`backend/internal/db/sqlc/content_read.sql.go`'s
/// `GetVerifiedExplanationRow.LegalRefs` is `json.RawMessage` server-side —
/// the underlying shape is confirmed via the seed fixture
/// `backend/seed/sample/data.json` and Plan 01's canonical
/// `CanonExplanation.LegalRefs []map[string]string`, e.g.
/// `{"code": "YHQ 0.0", "title": "..."}`).
@freezed
abstract class LegalRef with _$LegalRef {
  const factory LegalRef({required String code, required String title}) =
      _LegalRef;
}

/// One `{position, correct, text}` entry inside an `answer_analysis` block's
/// `items[]`, matching `backend/internal/explanation/draft.go`'s
/// `AnswerAnalysisItem` struct exactly:
/// ```
/// { "position": int, "correct": bool, "text": string }
/// ```
@freezed
abstract class AnswerAnalysisItem with _$AnswerAnalysisItem {
  const factory AnswerAnalysisItem({
    required int position,
    required bool correct,
    required String text,
  }) = _AnswerAnalysisItem;
}

/// One explanation block, matching `backend/internal/explanation/draft.go`'s
/// `Block` struct exactly:
/// ```
/// { "type": string, "text"?: string, "items"?: AnswerAnalysisItem[] }
/// ```
/// `type` is one of `intro`/`muhim`/`eslatma`/`ogohlantirish`/`maslahat`/
/// `answer_analysis`/`xulosa` (master spec §13) — modeled as a plain `String`
/// here rather than an enum since the backend doesn't constrain it to a
/// closed set at the wire level; `text` and `items` are mutually-optional
/// (a block has one or the other depending on `type`, per the Go struct's
/// `omitempty` tags), so both are nullable/empty-default rather than
/// required.
@freezed
abstract class ExplanationBlock with _$ExplanationBlock {
  const factory ExplanationBlock({
    required String type,
    String? text,
    @Default(<AnswerAnalysisItem>[]) List<AnswerAnalysisItem> items,
  }) = _ExplanationBlock;
}

/// A question's verified explanation, matching
/// `backend/internal/content/dto.go`'s `ExplanationDTO` exactly:
/// ```
/// { "legal_refs": LegalRef[], "blocks": ExplanationBlock[] }
/// ```
/// (both fields are `json.RawMessage` server-side; the element shapes above
/// are confirmed via `explanation/draft.go` and the seed fixture, not
/// guessed). Only ever present when
/// `GetVerifiedExplanation` found a `verified`-status row — drafted/
/// unverified explanations never reach this client at all (Plan 05
/// invariant), so there is no "draft"/"pending" state to model here.
@freezed
abstract class Explanation with _$Explanation {
  const factory Explanation({
    required List<LegalRef> legalRefs,
    required List<ExplanationBlock> blocks,
  }) = _Explanation;
}

/// A question, matching `backend/internal/content/dto.go`'s `QuestionDTO`
/// (embedded in a variant's `questions[]`, `handlers.go`'s `getVariant`) and
/// `QuestionDetailDTO` (`GET /questions/{id}`, `handlers.go`'s `getQuestion`)
/// shapes — one class covering both, since they differ only in which extra
/// fields are present:
/// ```
/// // embedded in GET /variants/{number} (QuestionDTO):
/// { "id": string, "position": int, "category_code": string, "text": string,
///   "image_url": string | null, "answers": Answer[] }
/// // NOTE: no "signs"/"explanation" keys at all in this shape.
///
/// // GET /questions/{id} (QuestionDetailDTO = QuestionDTO + ...):
/// { "id": string, "category_code": string, "text": string,
///   "image_url": string | null, "answers": Answer[],
///   "signs": SignChip[], "explanation": Explanation | null }
/// // NOTE: "position" is `json:"position,omitempty"` on QuestionDTO and is
/// // never set by getQuestion, so it is OMITTED entirely from this shape's
/// // JSON (not `null` — the key is simply absent).
/// ```
/// `position` is therefore nullable here: present (the question's 1-based
/// slot within its variant) when embedded in a variant, absent when fetched
/// standalone via `question()`. `signs` defaults to an empty list when the
/// key is absent (variant-embedded shape). `explanation` is nullable both
/// because the key can be absent (variant-embedded shape) and because the
/// backend sends a literal `null` when a question has no verified
/// explanation yet (`getQuestion`'s `detail.Explanation` stays `nil`).
///
/// Deliberately has NO `is_correct`/`correct_answer_id` field anywhere —
/// content endpoints never send one for the question or for any answer
/// (anti-cheat invariant, Plan 01) — so no such field is modeled here, on
/// [Answer], or anywhere else in this file.
@freezed
abstract class Question with _$Question {
  const factory Question({
    required String id,
    int? position,
    required String categoryCode,
    required String text,
    String? imageUrl,
    required List<Answer> answers,
    @Default(<Sign>[]) List<Sign> signs,
    Explanation? explanation,
  }) = _Question;
}
