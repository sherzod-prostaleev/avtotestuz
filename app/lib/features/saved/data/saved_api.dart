import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/result.dart';
import '../domain/saved_entry.dart';

/// Supplies the [SavedApi] the app's saved-questions controller talks to.
/// Has no usable default — a real instance (backed by the SAME configured
/// [Dio] `main.dart` already built for `AuthApi`/`ProfileApi`/`ContentApi`/
/// `ExplanationApi`) is wired at the app root, same "no default, overridden
/// at the app root" seam as `contentApiProvider`/`explanationApiProvider`.
/// Tests override this directly with a mock/fake.
final savedApiProvider = Provider<SavedApi>((ref) {
  throw UnimplementedError(
    'savedApiProvider has no default — override it at the app root with a '
    'real SavedApi.',
  );
});

/// Thin [Dio] wrapper around `backend/internal/progress/handlers.go`'s
/// saved-questions (bookmark) endpoints.
///
/// Request/response shapes below were confirmed by reading
/// `backend/internal/progress/handlers.go` directly (not just README §
/// "Saqlangan savollar" prose):
/// ```
/// GET    /me/saved              -> {"data": [{"question_id": string, "created_at": string}, ...]}
/// POST   /me/saved {question_id} -> {"data": {"ok": true}}
/// DELETE /me/saved/{question_id} -> {"data": {"ok": true}}
/// ```
/// all wrapped in the standard envelope `{"data": ...}` /
/// `{"error": {"code": ..., "message": ...}}` (`backend/internal/httpx`) and
/// funneled through [guard] so no raw [DioException] ever leaks past this
/// layer (same convention as `ContentApi`/`ExplanationApi`). Both `save`/
/// `unsave` are server-side idempotent (`Service.SaveQuestion`/
/// `Service.UnsaveQuestion` doc comments: saving an already-saved question,
/// or unsaving an already-removed/never-saved one, is not an error) — this
/// wrapper doesn't need to special-case either.
class SavedApi {
  SavedApi(this._dio);

  final Dio _dio;

  Future<Result<List<SavedEntry>>> list() {
    return guard(() async {
      final response = await _dio.get<Map<String, dynamic>>('/me/saved');
      final data = response.data!['data'] as List<dynamic>;
      return data
          .map((e) => _savedEntryFromJson(e as Map<String, dynamic>))
          .toList();
    });
  }

  Future<Result<void>> save(String questionId) {
    return guard(() async {
      await _dio.post<Map<String, dynamic>>(
        '/me/saved',
        data: {'question_id': questionId},
      );
    });
  }

  Future<Result<void>> unsave(String questionId) {
    return guard(() async {
      await _dio.delete<Map<String, dynamic>>('/me/saved/$questionId');
    });
  }
}

SavedEntry _savedEntryFromJson(Map<String, dynamic> json) {
  return SavedEntry(
    questionId: json['question_id'] as String,
    createdAt: DateTime.parse(json['created_at'] as String),
  );
}
