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

/// Compact, reusable streak display: current streak (with a big flame icon),
/// the best-ever record, today's progress toward the daily goal, and the
/// UTC-day-relative last-active label. Rendered on [StatsScreen] and designed
/// to drop into other surfaces (e.g. a home header) unchanged.
///
/// Visual identity is deliberately loud (Duolingo-convention): a flame icon
/// and the day-count number are always in [AppColors.streak] orange, so the
/// streak reads as the daily hook it is meant to be. The best-streak record
/// rides a [AppColors.gold] pill (rank/achievement accent).
class StreakCard extends StatelessWidget {
  const StreakCard({required this.streak, this.now, super.key});

  final Streak streak;

  /// Injectable "now" for deterministic relative-label tests; defaults to the
  /// real clock in production.
  final DateTime? now;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final appColors = context.appColors;
    final streakColor = appColors.streak;
    final activeLabel = streakRelativeLabel(streak.lastActiveDate, now: now);

    final goal = streak.dailyGoal;
    final goalProgress = goal > 0
        ? (streak.todayDone / goal).clamp(0.0, 1.0)
        : 0.0;
    final goalReached = goal > 0 && streak.todayDone >= goal;

    return AppCard(
      key: const Key('streakCard'),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              // Flame badge — the streak's visual anchor.
              Container(
                width: 60,
                height: 60,
                alignment: Alignment.center,
                decoration: BoxDecoration(
                  color: streakColor.withValues(alpha: 0.16),
                  borderRadius: BorderRadius.circular(AppRadius.card),
                ),
                child: Icon(
                  Icons.local_fire_department_rounded,
                  color: streakColor,
                  size: 36,
                ),
              ),
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      '${streak.current} kun',
                      key: const Key('streakCurrentText'),
                      style: theme.textTheme.displaySmall?.copyWith(
                        color: streakColor,
                        height: 1,
                      ),
                    ),
                    const SizedBox(height: AppSpacing.xs),
                    Text(
                      'Oxirgi faollik: $activeLabel',
                      key: const Key('streakLastActiveText'),
                      style: theme.textTheme.bodySmall,
                    ),
                  ],
                ),
              ),
              // Best-ever record — gold achievement pill.
              _RecordPill(best: streak.best, color: appColors.gold),
            ],
          ),
          const SizedBox(height: AppSpacing.md),
          Row(
            children: [
              Text(
                'Bugun: ${streak.todayDone}/${streak.dailyGoal}',
                key: const Key('streakTodayText'),
                style: theme.textTheme.titleSmall,
              ),
              const Spacer(),
              if (goalReached)
                Icon(
                  Icons.check_circle_rounded,
                  size: 18,
                  color: appColors.success,
                ),
            ],
          ),
          const SizedBox(height: AppSpacing.sm),
          ClipRRect(
            borderRadius: BorderRadius.circular(999),
            child: LinearProgressIndicator(
              value: goalProgress,
              minHeight: 10,
              color: goalReached ? appColors.success : streakColor,
              backgroundColor: theme.colorScheme.surfaceContainerHighest,
            ),
          ),
        ],
      ),
    );
  }
}

/// The best-ever streak record, rendered as a compact gold trophy pill.
class _RecordPill extends StatelessWidget {
  const _RecordPill({required this.best, required this.color});

  final int best;
  final Color color;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppSpacing.sm,
        vertical: 6,
      ),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.15),
        borderRadius: BorderRadius.circular(AppRadius.chip),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.emoji_events_rounded, size: 16, color: color),
          const SizedBox(width: AppSpacing.xs),
          Text(
            'Rekord: $best',
            key: const Key('streakBestText'),
            style: theme.textTheme.labelMedium?.copyWith(color: color),
          ),
        ],
      ),
    );
  }
}
