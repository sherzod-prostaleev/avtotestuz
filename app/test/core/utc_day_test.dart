import 'package:avtotest_app/core/utc_day.dart';
import 'package:avtotest_app/features/stats/presentation/streak_card.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('utcDayStart', () {
    test('truncates a UTC instant to its UTC calendar day', () {
      final d = utcDayStart(DateTime.utc(2026, 7, 20, 23, 59, 59));
      expect(d, DateTime.utc(2026, 7, 20));
      expect(d.isUtc, isTrue);
    });
  });

  group('parseUtcDate', () {
    test('parses YYYY-MM-DD as UTC midnight (never local)', () {
      final d = parseUtcDate('2026-07-20');
      expect(d, DateTime.utc(2026, 7, 20));
      expect(d!.isUtc, isTrue);
    });

    test('returns null for null/empty/malformed input', () {
      expect(parseUtcDate(null), isNull);
      expect(parseUtcDate(''), isNull);
      expect(parseUtcDate('2026-07'), isNull);
      expect(parseUtcDate('not-a-date'), isNull);
      expect(parseUtcDate('2026-13-40'), isNull);
    });
  });

  group('streakRelativeLabel — UTC-day boundary edge cases', () {
    test('null last-active date -> "hali faol emas"', () {
      expect(streakRelativeLabel(null), 'hali faol emas');
      expect(streakRelativeLabel(''), 'hali faol emas');
    });

    // The crux: at 01:00 local in UTC+5 it is already 2026-07-21 locally, but
    // it is still 2026-07-20 in UTC. The backend's streak/last_active_date is
    // UTC-day based, so an activity dated "2026-07-20" must read as "bugun",
    // NOT "kecha" — even though a naive local-time comparison would say the
    // 20th is "yesterday" once the local clock ticks past local midnight.
    test(
      'just past LOCAL midnight in UTC+5 but still "today" in UTC -> "bugun"',
      () {
        // 2026-07-20T20:30Z == 2026-07-21T01:30 in UTC+5 (local "tomorrow").
        final now = DateTime.utc(2026, 7, 20, 20, 30);
        expect(streakRelativeLabel('2026-07-20', now: now), 'bugun');
      },
    );

    test(
      'just past UTC midnight -> the previous UTC day is "kecha"',
      () {
        // 2026-07-21T00:30Z: UTC has already rolled to the 21st.
        final now = DateTime.utc(2026, 7, 21, 0, 30);
        expect(streakRelativeLabel('2026-07-20', now: now), 'kecha');
        expect(streakRelativeLabel('2026-07-21', now: now), 'bugun');
      },
    );

    test('two or more UTC days ago -> raw date string', () {
      final now = DateTime.utc(2026, 7, 21, 0, 30);
      expect(streakRelativeLabel('2026-07-19', now: now), '2026-07-19');
    });

    test('same UTC day at any time of day -> "bugun"', () {
      expect(
        streakRelativeLabel('2026-07-20', now: DateTime.utc(2026, 7, 20, 0, 1)),
        'bugun',
      );
      expect(
        streakRelativeLabel('2026-07-20', now: DateTime.utc(2026, 7, 20, 23, 59)),
        'bugun',
      );
    });
  });
}
