import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/result.dart';
import '../domain/stats.dart';
import '../domain/streak.dart';

/// Supplies the [ProgressApi] the app's stats/streak UI talks to. Has no
/// usable default — a real instance (backed by the same configured [Dio]
/// `main.dart` already built for `AuthApi`/`ProfileApi`/`ContentApi`/etc.) is
/// wired at the app root, the same "no default, overridden at the app root"
/// seam as `contentApiProvider`/`explanationApiProvider`. Tests override this
/// directly with a mock.
final progressApiProvider = Provider<ProgressApi>((ref) {
  throw UnimplementedError(
    'progressApiProvider has no default — override it at the app root with '
    'a real ProgressApi.',
  );
});

/// Thin [Dio] wrapper around the learner's own progress endpoints
/// (`GET /me/streak` from `backend/internal/progress/handlers.go`,
/// `GET /me/stats` from `backend/internal/learning/handlers.go`). Every call
/// is wrapped in [guard] so raw [DioException]s never leak past this layer
/// (same convention as `features/profile/data/profile_api.dart`).
///
/// Response shapes (confirmed by reading those handlers directly, not just
/// README prose), each inside the standard `{"data": ...}` envelope
/// (`backend/internal/httpx`):
/// ```
/// GET /me/streak
///   -> {"current": int, "best": int, "today_done": int, "daily_goal": int,
///       "last_active_date": string("YYYY-MM-DD") | null}
/// GET /me/stats
///   -> {"categories": [{"category_code": string, "mastery": float,
///        "seen": int, "correct": int}], "readiness_pct": int,
///       "due_count": int}
/// ```
class ProgressApi {
  ProgressApi(this._dio);

  final Dio _dio;

  Future<Result<Streak>> streak() {
    return guard(() async {
      final response = await _dio.get<Map<String, dynamic>>('/me/streak');
      final data = response.data!['data'] as Map<String, dynamic>;
      return _streakFromJson(data);
    });
  }

  Future<Result<Stats>> stats() {
    return guard(() async {
      final response = await _dio.get<Map<String, dynamic>>('/me/stats');
      final data = response.data!['data'] as Map<String, dynamic>;
      return _statsFromJson(data);
    });
  }
}

Streak _streakFromJson(Map<String, dynamic> json) {
  return Streak(
    current: (json['current'] as num).toInt(),
    best: (json['best'] as num).toInt(),
    todayDone: (json['today_done'] as num).toInt(),
    dailyGoal: (json['daily_goal'] as num).toInt(),
    // Kept as the raw backend "YYYY-MM-DD" UTC-day string — deliberately NOT
    // parsed into a local DateTime (see Streak's doc comment / core/utc_day).
    lastActiveDate: json['last_active_date'] as String?,
  );
}

Stats _statsFromJson(Map<String, dynamic> json) {
  final rawCategories = (json['categories'] as List<dynamic>?) ?? const [];
  return Stats(
    categories: rawCategories
        .map((e) => _categoryStatFromJson(e as Map<String, dynamic>))
        .toList(growable: false),
    readinessPct: (json['readiness_pct'] as num).toInt(),
    dueCount: (json['due_count'] as num).toInt(),
  );
}

CategoryStat _categoryStatFromJson(Map<String, dynamic> json) {
  return CategoryStat(
    categoryCode: json['category_code'] as String,
    // `mastery` is a Go float64; JSON may decode a whole value as int, so go
    // through `num` before `.toDouble()` rather than a bare `as double`.
    mastery: (json['mastery'] as num).toDouble(),
    seen: (json['seen'] as num).toInt(),
    correct: (json['correct'] as num).toInt(),
  );
}
