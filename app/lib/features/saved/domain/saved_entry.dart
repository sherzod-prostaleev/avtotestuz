import 'package:freezed_annotation/freezed_annotation.dart';

part 'saved_entry.freezed.dart';

/// One bookmarked-question row, matching `backend/internal/progress`'s
/// `savedItemDTO` exactly (confirmed by reading
/// `backend/internal/progress/handlers.go`'s `listSaved`, not just README
/// prose):
/// ```
/// { "question_id": string, "created_at": string (RFC3339) }
/// ```
/// Deliberately does NOT embed the question's own content (text/answers/
/// image) — `GET /me/saved` only ever returns the bookmark row itself; the
/// full `Question` is fetched separately per-id from `ContentApi.question`
/// (`GET /questions/{id}`), the same "list endpoint gives ids, content API
/// gives bodies" split `session_controller.dart`'s `sessionQuestionProvider`
/// already uses for `question_ids`.
@freezed
abstract class SavedEntry with _$SavedEntry {
  const factory SavedEntry({
    required String questionId,
    required DateTime createdAt,
  }) = _SavedEntry;
}
