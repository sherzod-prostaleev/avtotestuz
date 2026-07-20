import 'package:flutter/material.dart';

import '../../../app/theme/app_theme.dart';
import '../../../core/utc_day.dart';
import '../../../shared/widgets/app_card.dart';
import '../domain/streak.dart';

/// Returns a human, Uzbek relative label for a [Streak.lastActiveDate]
/// backend `"YYYY-MM-DD"` string, computed against **UTC calendar days** to
/// match the backend's UTC-day streak semantics (`core/utc_day.dart`).
///
/// Using UTC-day math here (rather than local `DateTime` comparison) is the
/// whole point: around the UTC/local midnight boundary — e.g. 01:00 local in
/// UTC+5, which is still the *previous* day in UTC — a naive local comparison
/// would label a still-"today"-in-UTC activity as "kecha", contradicting the
/// `current`/`today_done` numbers the same backend response carries. [now]
/// is injectable purely so this is deterministically unit-testable across
/// that boundary.
///
/// - `null`/empty last-active date -> "hali faol emas" (never active).
/// - same UTC day as [now] -> "bugun".
/// - exactly one UTC day before [now] -> "kecha".
/// - anything older (or, defensively, in the future) -> the raw
///   `"YYYY-MM-DD"` string, shown as-is rather than mislabelled.
String streakRelativeLabel(String? lastActiveDate, {DateTime? now}) {
  if (lastActiveDate == null || lastActiveDate.isEmpty) {
    return 'hali faol emas';
  }
  final last = parseUtcDate(lastActiveDate);
  if (last == null) return lastActiveDate;
  final today = utcDayStart(now ?? DateTime.now());
  final diffDays = today.difference(last).inDays;
  if (diffDays <= 0) return 'bugun';
  if (diffDays == 1) return 'kecha';
  return lastActiveDate;
}

/// Compact, reusable streak display: current streak (with a flame icon), the
/// best-ever record, today's progress toward the daily goal, and the
/// UTC-day-relative last-active label. Rendered on [StatsScreen] and designed
/// to drop into other surfaces (e.g. a home header) unchanged.
class StreakCard extends StatelessWidget {
  const StreakCard({required this.streak, this.now, super.key});

  final Streak streak;

  /// Injectable "now" for deterministic relative-label tests; defaults to the
  /// real clock in production.
  final DateTime? now;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final activeLabel = streakRelativeLabel(streak.lastActiveDate, now: now);

    return AppCard(
      key: const Key('streakCard'),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Icon(Icons.local_fire_department, color: Colors.deepOrange),
              const SizedBox(width: AppSpacing.sm),
              Text(
                '${streak.current} kun',
                key: const Key('streakCurrentText'),
                style: theme.textTheme.headlineSmall,
              ),
              const Spacer(),
              Text(
                'Rekord: ${streak.best}',
                key: const Key('streakBestText'),
                style: theme.textTheme.bodyMedium,
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.sm),
          Text(
            'Bugun: ${streak.todayDone}/${streak.dailyGoal}',
            key: const Key('streakTodayText'),
            style: theme.textTheme.bodyMedium,
          ),
          const SizedBox(height: 4),
          Text(
            'Oxirgi faollik: $activeLabel',
            key: const Key('streakLastActiveText'),
            style: theme.textTheme.bodySmall,
          ),
        ],
      ),
    );
  }
}
