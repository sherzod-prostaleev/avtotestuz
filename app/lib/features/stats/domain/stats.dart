import 'package:freezed_annotation/freezed_annotation.dart';

part 'stats.freezed.dart';

/// Per-category learning progress, one entry per content category, matching
/// `GET /api/v1/me/stats`'s `categories[]` entries as produced by
/// `backend/internal/learning/handlers.go`'s `categoryStatDTO` (confirmed by
/// reading that file directly):
/// ```
/// { "category_code": string, "mastery": float(0..1), "seen": int, "correct": int }
/// ```
@freezed
abstract class CategoryStat with _$CategoryStat {
  const factory CategoryStat({
    required String categoryCode,
    required double mastery,
    required int seen,
    required int correct,
  }) = _CategoryStat;
}

/// Overall learning statistics, matching `GET /api/v1/me/stats`'s response as
/// produced by `backend/internal/learning/handlers.go`'s `statsResponse`:
/// ```
/// {
///   "categories": categoryStatDTO[],
///   "readiness_pct": int,   // 0..100, weighted-average mastery -> exam readiness
///   "due_count": int        // FSRS cards currently due for review
/// }
/// ```
@freezed
abstract class Stats with _$Stats {
  const factory Stats({
    required List<CategoryStat> categories,
    required int readinessPct,
    required int dueCount,
  }) = _Stats;
}
