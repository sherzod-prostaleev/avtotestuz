import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../../app/l10n/app_localizations.dart';
import '../../../app/theme/app_theme.dart';
import '../domain/session_models.dart';

/// The real, complete results screen every mode (`variant`/`exam`/`practice`/
/// `mistakes`) lands on after [SessionResult] comes back from
/// `POST /sessions/{id}/finish` — superseding Task 4's minimal
/// `SessionResultView` placeholder (`session_screen.dart`) now that results
/// are core to every mode, not an afterthought.
///
/// Renders [SessionResult.status] (`passed`/`failed`/`abandoned`) and
/// [SessionResult.stoppedReason] (`completed`/`time_up`/`too_many_errors`)
/// with distinct, sensible copy for every combination the backend can
/// actually send — never a raw/blank fallback for an expected value.
///
/// Each status is a genuinely different *moment*, not just different text:
/// passing is a celebratory success-green moment (distinct from the brand
/// accent — "passed" must read as "correct/good", the same reason the answer
/// options use green for correctness); failing is calm and encouraging (never
/// punishing — the copy nudges the learner to try again); abandoning is a
/// neutral, informational note.
class SessionResultsScreen extends StatelessWidget {
  const SessionResultsScreen({required this.result, super.key});

  final SessionResult result;

  /// The full visual + copy treatment for a given status, resolved against the
  /// live [ColorScheme]/[AppColors] so both light and dark themes stay correct.
  static _ResultVisual _visualFor(
    String status,
    ColorScheme scheme,
    AppColors appColors,
    AppLocalizations l10n,
  ) {
    return switch (status) {
      'passed' => _ResultVisual(
          icon: Icons.check_circle,
          accent: appColors.success,
          container: appColors.successContainer,
          onContainer: appColors.onSuccessContainer,
          reasonIcon: Icons.emoji_events_rounded,
          label: l10n.sessionStatusPassedLabel,
          message: l10n.sessionResultsPassedMessage,
        ),
      'failed' => _ResultVisual(
          icon: Icons.cancel,
          accent: scheme.error,
          container: scheme.errorContainer,
          onContainer: scheme.onErrorContainer,
          reasonIcon: Icons.flag_outlined,
          label: l10n.sessionStatusFailedLabel,
          message: l10n.sessionResultsFailedMessage,
        ),
      _ => _ResultVisual(
          icon: Icons.info_outline,
          accent: scheme.onSurfaceVariant,
          container: scheme.surfaceContainerHighest,
          onContainer: scheme.onSurfaceVariant,
          reasonIcon: Icons.flag_outlined,
          label: l10n.sessionResultsAbandonedLabel,
          message: l10n.sessionResultsAbandonedMessage,
        ),
    };
  }

  static String _reasonLabel(AppLocalizations l10n, String stoppedReason) =>
      switch (stoppedReason) {
        'completed' => l10n.sessionReasonCompletedLabel,
        'time_up' => l10n.sessionReasonTimeUpLabel,
        'too_many_errors' => l10n.sessionReasonTooManyErrorsLabel,
        _ => stoppedReason,
      };

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final visual = _visualFor(result.status, scheme, context.appColors, l10n);
    final percent = result.total == 0
        ? 0
        : ((result.score / result.total) * 100).round();

    return Scaffold(
      key: const Key('session-results-screen'),
      appBar: AppBar(title: Text(l10n.sessionResultTitle)),
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(AppSpacing.lg),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 440),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  // Hero badge — a tinted disc holding the status icon, the
                  // single strongest signal of "how did I do".
                  Center(
                    child: Container(
                      width: 104,
                      height: 104,
                      decoration: BoxDecoration(
                        color: visual.container,
                        shape: BoxShape.circle,
                        border: Border.all(color: visual.accent, width: 2),
                      ),
                      child: Icon(
                        visual.icon,
                        key: const Key('session-results-status-icon'),
                        color: visual.accent,
                        size: 56,
                      ),
                    ),
                  ),
                  const SizedBox(height: AppSpacing.md),
                  Text(
                    visual.label,
                    key: const Key('session-results-status'),
                    style: theme.textTheme.headlineMedium?.copyWith(
                      color: visual.accent,
                    ),
                    textAlign: TextAlign.center,
                  ),
                  const SizedBox(height: AppSpacing.sm),
                  Text(
                    visual.message,
                    style: theme.textTheme.bodyLarge?.copyWith(
                      color: scheme.onSurfaceVariant,
                    ),
                    textAlign: TextAlign.center,
                  ),
                  const SizedBox(height: AppSpacing.lg),
                  // Score "moment": the big Baloo-2 number, framed in a tinted
                  // card so it reads as the headline result, with the percent
                  // as a supporting pill beneath.
                  _ScoreCard(
                    score: result.score,
                    total: result.total,
                    percent: percent,
                    visual: visual,
                  ),
                  const SizedBox(height: AppSpacing.md),
                  // Why the session ended — supporting context, kept quiet.
                  _ReasonRow(
                    icon: visual.reasonIcon,
                    label: _reasonLabel(l10n, result.stoppedReason),
                  ),
                  const SizedBox(height: AppSpacing.lg),
                  FilledButton(
                    key: const Key('session-results-home-button'),
                    onPressed: () => context.go('/'),
                    child: Text(l10n.homeButton),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// The framed score headline: `score / total` in a big rounded number, plus a
/// percent pill. Tinted with the current status accent so a pass, a fail and
/// an abandon each carry their own colour all the way through.
class _ScoreCard extends StatelessWidget {
  const _ScoreCard({
    required this.score,
    required this.total,
    required this.percent,
    required this.visual,
  });

  final int score;
  final int total;
  final int percent;
  final _ResultVisual visual;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppSpacing.lg,
        vertical: AppSpacing.md,
      ),
      decoration: BoxDecoration(
        color: visual.container.withValues(alpha: 0.55),
        borderRadius: BorderRadius.circular(AppRadius.card),
        border: Border.all(color: visual.accent.withValues(alpha: 0.35)),
      ),
      child: Column(
        children: [
          Text(
            '$score / $total',
            key: const Key('session-results-score'),
            style: theme.textTheme.displayMedium?.copyWith(
              color: visual.onContainer,
            ),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: AppSpacing.sm),
          Container(
            padding: const EdgeInsets.symmetric(
              horizontal: AppSpacing.md,
              vertical: AppSpacing.xs,
            ),
            decoration: BoxDecoration(
              color: visual.accent,
              borderRadius: BorderRadius.circular(AppRadius.chip),
            ),
            child: Text(
              '$percent%',
              key: const Key('session-results-percent'),
              style: theme.textTheme.titleMedium?.copyWith(
                color: theme.colorScheme.surface,
                fontWeight: FontWeight.w700,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// A quiet, centered "why it ended" line with a leading icon.
class _ReasonRow extends StatelessWidget {
  const _ReasonRow({required this.icon, required this.label});

  final IconData icon;
  final String label;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final muted = theme.colorScheme.onSurfaceVariant;
    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        Icon(icon, size: 18, color: muted),
        const SizedBox(width: AppSpacing.sm),
        Flexible(
          child: Text(
            label,
            key: const Key('session-results-reason'),
            textAlign: TextAlign.center,
            style: theme.textTheme.bodyMedium?.copyWith(color: muted),
          ),
        ),
      ],
    );
  }
}

/// Immutable per-status visual + copy bundle resolved against the live theme.
class _ResultVisual {
  const _ResultVisual({
    required this.icon,
    required this.accent,
    required this.container,
    required this.onContainer,
    required this.reasonIcon,
    required this.label,
    required this.message,
  });

  final IconData icon;
  final Color accent;
  final Color container;
  final Color onContainer;
  final IconData reasonIcon;
  final String label;
  final String message;
}
