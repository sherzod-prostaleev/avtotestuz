import 'package:freezed_annotation/freezed_annotation.dart';

import 'question.dart';

part 'variant.freezed.dart';

/// A bilet (variant), covering both shapes the backend returns under the
/// `Variant` name — one class, fields nullable where a given shape omits
/// them (confirmed against `backend/internal/content/dto.go`,
/// `backend/internal/content/handlers.go`):
///
/// `GET /variants` (`data[]` items, `VariantListItemDTO`, `listVariants`;
/// no `locale=` param — variant metadata is locale-neutral per Plan 01):
/// ```
/// { "number": int, "question_count": int }
/// ```
/// -> `questionCount` set, `questions` null.
///
/// `GET /variants/{number}` (`data`, `VariantDetailDTO`, `getVariant`):
/// ```
/// { "number": int, "questions": Question[] }
/// ```
/// -> `questions` set (each item is the variant-embedded `Question` shape —
/// see `question.dart`'s doc comment), `questionCount` null (the backend
/// doesn't send a redundant count alongside the full list; callers needing
/// a count here can use `questions!.length`).
@freezed
abstract class Variant with _$Variant {
  const factory Variant({
    required int number,
    int? questionCount,
    List<Question>? questions,
  }) = _Variant;
}
