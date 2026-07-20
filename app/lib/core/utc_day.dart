/// UTC calendar-day helpers, mirroring the Go backend's `todayUTC()`
/// (`backend/internal/progress/service.go`: `time.Now().UTC().Truncate(24h)`).
///
/// The backend computes daily-streak boundaries in **UTC**, not the user's
/// local time zone (`README.md`'s "Kunlik streak" section documents this
/// explicitly: around a day boundary — e.g. before 05:00 in UTC+5 —
/// `last_active_date` can look "one day behind" local time, and that is the
/// intended behavior, not a bug). So any client-side relative-day reasoning
/// about streak dates MUST use these UTC-day functions rather than local
/// `DateTime` math, otherwise the label ("bugun"/"kecha") would contradict
/// the backend's own semantics around midnight.
library;

/// Truncates [instant] to the start (midnight) of its **UTC** calendar day,
/// the direct analogue of the backend's `todayUTC()`. Always returns a
/// UTC-flagged [DateTime] regardless of whether [instant] was local or UTC.
DateTime utcDayStart(DateTime instant) {
  final u = instant.toUtc();
  return DateTime.utc(u.year, u.month, u.day);
}

/// Parses a backend `"YYYY-MM-DD"` date string — which denotes a **UTC**
/// calendar day — into a UTC-midnight [DateTime]. Returns `null` for a
/// `null`/empty/malformed input rather than throwing, so callers can treat
/// "never active" uniformly.
///
/// Deliberately does NOT use `DateTime.parse`, which interprets a bare
/// date-only string as **local** midnight — exactly the local-time
/// contamination this module exists to avoid.
DateTime? parseUtcDate(String? yyyyMmDd) {
  if (yyyyMmDd == null || yyyyMmDd.isEmpty) return null;
  final parts = yyyyMmDd.split('-');
  if (parts.length != 3) return null;
  final year = int.tryParse(parts[0]);
  final month = int.tryParse(parts[1]);
  final day = int.tryParse(parts[2]);
  if (year == null || month == null || day == null) return null;
  if (month < 1 || month > 12 || day < 1 || day > 31) return null;
  return DateTime.utc(year, month, day);
}

/// Whole-day difference between two `"YYYY-MM-DD"` UTC dates
/// (`later - earlier`), or `null` if either can't be parsed. `0` means the
/// same UTC day, `1` means consecutive UTC days, etc.
int? utcDayDifference(String? earlier, String? later) {
  final a = parseUtcDate(earlier);
  final b = parseUtcDate(later);
  if (a == null || b == null) return null;
  return b.difference(a).inDays;
}
