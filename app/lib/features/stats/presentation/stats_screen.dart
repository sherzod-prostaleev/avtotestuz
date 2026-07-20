import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../app/theme/app_theme.dart';
import '../../../shared/widgets/app_card.dart';
import '../../../shared/widgets/empty_state.dart';
import '../../../shared/widgets/mastery_bar.dart';
import '../domain/stats.dart';
import '../domain/streak.dart';
import 'stats_controller.dart';
import 'streak_card.dart';

/// The learner's own progress dashboard — the real screen behind
/// `HomeShell`'s last remaining "coming soon" nav placeholder (`navStats`).
///
/// Renders, from [statsControllerProvider] (`GET /me/streak` + `GET /me/stats`):
/// a [StreakCard] (current/best/today/last-active), an overall exam-readiness
/// percentage + due-count, and a per-category [MasteryBar] breakdown.
class StatsScreen extends ConsumerWidget {
  const StatsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(statsControllerProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('Statistika')),
      body: SafeArea(
        child: state.when(
          loading: () => const Center(
            key: Key('stats-loading'),
            child: CircularProgressIndicator(),
          ),
          error: (error, stackTrace) => EmptyState(
            key: const Key('stats-error-state'),
            message: error is StatsFetchFailure
                ? error.failure.message
                : 'Statistikani yuklab bo\'lmadi',
            onRetry: () => ref.invalidate(statsControllerProvider),
            retryLabel: 'Qayta urinish',
          ),
          data: (data) => _StatsBody(streak: data.streak, stats: data.stats),
        ),
      ),
    );
  }
}

/// Infers a sensible glyph for a category code. Known codes get a specific
/// icon; anything else falls back to a generic category glyph rather than
/// rendering nothing — keeps the breakdown visually consistent as the backend
/// adds categories we don't yet special-case.
IconData categoryIcon(String code) {
  switch (code.toLowerCase()) {
    case 'signs':
      return Icons.signpost_rounded;
    case 'traffic':
      return Icons.traffic_rounded;
    case 'markings':
    case 'marking':
      return Icons.horizontal_rule_rounded;
    case 'rules':
      return Icons.gavel_rounded;
    case 'safety':
      return Icons.health_and_safety_rounded;
    case 'medical':
    case 'first_aid':
      return Icons.medical_services_rounded;
    case 'penalties':
    case 'fines':
      return Icons.receipt_long_rounded;
    default:
      return Icons.category_rounded;
  }
}

class _StatsBody extends StatelessWidget {
  const _StatsBody({required this.streak, required this.stats});

  final Streak streak;
  final Stats stats;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return SingleChildScrollView(
      key: const Key('stats-body'),
      padding: const EdgeInsets.all(AppSpacing.md),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          StreakCard(streak: streak),
          const SizedBox(height: AppSpacing.md),
          _ReadinessCard(readinessPct: stats.readinessPct),
          const SizedBox(height: AppSpacing.md),
          _DueCard(dueCount: stats.dueCount),
          const SizedBox(height: AppSpacing.lg),
          Text(
            'Kategoriyalar bo\'yicha',
            style: theme.textTheme.titleMedium,
          ),
          const SizedBox(height: AppSpacing.sm),
          if (stats.categories.isEmpty)
            const Padding(
              key: Key('stats-empty-categories'),
              padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
              child: Text('Hali ma\'lumot yo\'q — birinchi sessiyani boshlang'),
            )
          else
            ...stats.categories.map(
              (c) => Padding(
                key: Key('masteryBar-${c.categoryCode}'),
                padding: const EdgeInsets.only(bottom: AppSpacing.md),
                child: MasteryBar(
                  label: c.categoryCode,
                  value: c.mastery,
                  icon: categoryIcon(c.categoryCode),
                  trailing: '${c.correct}/${c.seen} to\'g\'ri',
                ),
              ),
            ),
        ],
      ),
    );
  }
}

/// Exam-readiness hero: a big Baloo2 percentage inside a progress ring. Reads
/// as gold once the learner is genuinely exam-ready (a high-achievement
/// state), and as brand accent while still climbing.
class _ReadinessCard extends StatelessWidget {
  const _ReadinessCard({required this.readinessPct});

  final int readinessPct;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final appColors = context.appColors;
    final clamped = (readinessPct / 100).clamp(0.0, 1.0);
    final isReady = readinessPct >= 80;
    final accent = isReady ? appColors.gold : theme.colorScheme.primary;

    return AppCard(
      key: const Key('readinessCard'),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Imtihonga tayyorlik',
                  style: theme.textTheme.titleMedium,
                ),
                const SizedBox(height: AppSpacing.xs),
                Text(
                  isReady
                      ? 'Imtihonga tayyorsiz'
                      : 'Mashqni davom ettiring',
                  style: theme.textTheme.bodySmall,
                ),
              ],
            ),
          ),
          const SizedBox(width: AppSpacing.md),
          SizedBox(
            width: 96,
            height: 96,
            child: Stack(
              alignment: Alignment.center,
              children: [
                SizedBox.expand(
                  child: CircularProgressIndicator(
                    value: clamped,
                    strokeWidth: 9,
                    strokeCap: StrokeCap.round,
                    color: accent,
                    backgroundColor:
                        theme.colorScheme.surfaceContainerHighest,
                  ),
                ),
                Text(
                  '$readinessPct%',
                  key: const Key('readinessPctText'),
                  style: theme.textTheme.headlineSmall?.copyWith(
                    color: accent,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// Cards currently due for review — a clear, highlighted stat with its own
/// icon so the number reads as an actionable "do this next" prompt.
class _DueCard extends StatelessWidget {
  const _DueCard({required this.dueCount});

  final int dueCount;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final primary = theme.colorScheme.primary;

    return AppCard(
      child: Row(
        children: [
          Container(
            width: 48,
            height: 48,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color: primary.withValues(alpha: 0.14),
              borderRadius: BorderRadius.circular(AppRadius.chip),
            ),
            child: Icon(Icons.history_rounded, color: primary),
          ),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Text(
              'Takrorlash uchun',
              style: theme.textTheme.titleMedium,
            ),
          ),
          Text(
            '$dueCount',
            key: const Key('dueCountText'),
            style: theme.textTheme.headlineMedium?.copyWith(color: primary),
          ),
        ],
      ),
    );
  }
}
