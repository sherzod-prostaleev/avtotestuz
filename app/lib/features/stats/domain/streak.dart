import 'package:freezed_annotation/freezed_annotation.dart';

part 'streak.freezed.dart';

/// The authenticated user's daily-streak state, matching
/// `GET /api/v1/me/streak`'s response shape exactly as produced by
/// `backend/internal/progress/handlers.go`'s `streakDTO` (confirmed by
/// reading that file directly, not just README prose):
/// ```
/// {
///   "current": int,            // consecutive active UTC days (0 if the
///                              // streak lapsed — no answer yesterday/today)
///   "best": int,               // longest streak record
///   "today_done": int,         // questions answered today (UTC)
///   "daily_goal": int,         // fixed for M1; editable-goal UI is M3
///   "last_active_date": string | null   // "YYYY-MM-DD", null if never active
/// }
/// ```
///
/// [lastActiveDate] is kept as the raw backend `"YYYY-MM-DD"` **string**, not
/// a `DateTime`, on purpose: that date denotes a **UTC calendar day**
/// (`README.md` "Kunlik streak"), and parsing it into a local `DateTime`
/// would silently shift it across the UTC/local midnight boundary —
/// contradicting the backend's own documented semantics. Any relative-day
/// labelling ("bugun"/"kecha") is computed from this raw string via
/// `core/utc_day.dart`'s UTC-day helpers (see `streakRelativeLabel`), never
/// via local `DateTime` math.
@freezed
abstract class Streak with _$Streak {
  const factory Streak({
    required int current,
    required int best,
    required int todayDone,
    required int dailyGoal,
    String? lastActiveDate,
  }) = _Streak;
}
