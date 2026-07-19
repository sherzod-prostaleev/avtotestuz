import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/result.dart';

/// Supplies the [ExplanationApi] the app's explanation-feedback UI talks to.
/// Has no usable default — a real instance (backed by the same configured
/// [Dio] `main.dart` already built for `AuthApi`/`ProfileApi`/`ContentApi`)
/// is wired at the app root, same "no default, overridden at the app root"
/// seam as `contentApiProvider`. Tests override this directly with a mock.
final explanationApiProvider = Provider<ExplanationApi>((ref) {
  throw UnimplementedError(
    'explanationApiProvider has no default — override it at the app root '
    'with a real ExplanationApi.',
  );
});

/// Thin [Dio] wrapper around `backend/internal/explanation/handlers.go`'s
/// learner-feedback endpoint. This is deliberately the ONLY real HTTP call
/// on this class — explanation CONTENT itself is never fetched separately,
/// it arrives already embedded in `Question.explanation` from
/// `ContentApi.question` (`GET /questions/{id}`, Plan 07 Task 1). Only
/// `verified`-status explanations ever reach that endpoint in the first
/// place (`GetVerifiedExplanation`'s server-side invariant, Plan 05), so by
/// the time a caller has a `Question.explanation` to show a feedback button
/// next to, the explanation is guaranteed real/verified — never a draft.
///
/// Request/response shape confirmed by reading
/// `backend/internal/explanation/handlers.go` directly (not just README
/// prose):
/// ```
/// POST /explanations/feedback {"question_id": string, "helpful": bool}
///   -> {"ok": true}
/// ```
/// wrapped in the standard envelope `{"data": ...}` /
/// `{"error": {"code": ..., "message": ...}}` (`backend/internal/httpx`).
/// The handler's only named error is `not_found` (404, "explanation not
/// found") — surfaced by [guard] as `Failure(code: 'not_found', ...)`
/// automatically (the envelope's `error.code` passes straight through), so
/// no extra mapping is needed here beyond documenting that callers should
/// check `failure.code == 'not_found'` distinctly rather than treating it
/// as a generic error.
class ExplanationApi {
  ExplanationApi(this._dio);

  final Dio _dio;

  Future<Result<void>> feedback({
    required String questionId,
    required bool helpful,
  }) {
    return guard(() async {
      await _dio.post<Map<String, dynamic>>(
        '/explanations/feedback',
        data: {'question_id': questionId, 'helpful': helpful},
      );
    });
  }
}
