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
          AppCard(
            key: const Key('readinessCard'),
            child: Row(
              children: [
                Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('Imtihonga tayyorlik', style: theme.textTheme.bodyMedium),
                    const SizedBox(height: 4),
                    Text(
                      '${stats.readinessPct}%',
                      key: const Key('readinessPctText'),
                      style: theme.textTheme.headlineMedium,
                    ),
                  ],
                ),
                const Spacer(),
                Column(
                  crossAxisAlignment: CrossAxisAlignment.end,
                  children: [
                    Text('Takrorlash uchun', style: theme.textTheme.bodySmall),
                    const SizedBox(height: 4),
                    Text(
                      '${stats.dueCount}',
                      key: const Key('dueCountText'),
                      style: theme.textTheme.titleLarge,
                    ),
                  ],
                ),
              ],
            ),
          ),
          const SizedBox(height: AppSpacing.md),
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
                  trailing: '${c.correct}/${c.seen} to\'g\'ri',
                ),
              ),
            ),
        ],
      ),
    );
  }
}
