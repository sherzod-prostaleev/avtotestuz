import 'package:freezed_annotation/freezed_annotation.dart';

part 'sign.freezed.dart';

/// A road sign, covering all three shapes the backend returns under the
/// `Sign` name — one class, fields nullable where a given shape omits them,
/// rather than three near-duplicate classes (confirmed against
/// `backend/internal/content/dto.go`, `backend/internal/content/handlers.go`):
///
/// `GET /signs` (`data[]` items, `SignListItemDTO`, `listSigns`):
/// ```
/// { "code": string, "group_code": string, "name": string,
///   "image_url": string | null, "question_count": int }
/// ```
/// -> `groupCode`/`imageUrl`/`questionCount` set, `description`/`questionIds` null.
///
/// `GET /signs/{code}` (`data`, `SignDetailDTO`, `getSign`):
/// ```
/// { "code": string, "group_code": string, "name": string,
///   "description": string, "image_url": string | null,
///   "question_ids": string[] }
/// ```
/// -> `groupCode`/`description`/`imageUrl`/`questionIds` set, `questionCount` null.
///
/// Nested inside a `Question`'s `signs` list (`SignChipDTO`, from
/// `GET /questions/{id}`'s `getQuestion`):
/// ```
/// { "code": string, "name": string, "image_url": string | null }
/// ```
/// -> only `code`/`name`/`imageUrl` set, everything else null.
@freezed
abstract class Sign with _$Sign {
  const factory Sign({
    required String code,
    required String name,
    String? groupCode,
    String? description,
    String? imageUrl,
    int? questionCount,
    List<String>? questionIds,
  }) = _Sign;
}
