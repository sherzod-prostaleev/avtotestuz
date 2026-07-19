import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

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
class SessionResultsScreen extends StatelessWidget {
  const SessionResultsScreen({required this.result, super.key});

  final SessionResult result;

  static ({IconData icon, Color Function(ColorScheme) color}) _statusVisual(
    String status,
  ) {
    return switch (status) {
      'passed' => (
          icon: Icons.check_circle,
          color: (scheme) => Colors.green.shade600,
        ),
      'failed' => (icon: Icons.cancel, color: (scheme) => Colors.red.shade600),
      _ => (icon: Icons.info_outline, color: (scheme) => scheme.outline),
    };
  }

  static String _statusLabel(String status) => switch (status) {
    'passed' => 'O\'tdingiz',
    'failed' => 'O\'ta olmadingiz',
    'abandoned' => 'Sessiya tugallanmadi',
    _ => status,
  };

  static String _reasonLabel(String stoppedReason) => switch (stoppedReason) {
    'completed' => 'Barcha savollar yakunlandi',
    'time_up' => 'Vaqt tugadi',
    'too_many_errors' => 'Xatolar soni ko\'payib ketdi',
    _ => stoppedReason,
  };

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final visual = _statusVisual(result.status);
    final percent = result.total == 0
        ? 0
        : ((result.score / result.total) * 100).round();

    return Scaffold(
      key: const Key('session-results-screen'),
      appBar: AppBar(title: const Text('Natija')),
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(AppSpacing.lg),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(
                  visual.icon,
                  key: const Key('session-results-status-icon'),
                  color: visual.color(scheme),
                  size: 64,
                ),
                const SizedBox(height: AppSpacing.md),
                Text(
                  _statusLabel(result.status),
                  key: const Key('session-results-status'),
                  style: Theme.of(context).textTheme.headlineSmall,
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: AppSpacing.md),
                Text(
                  '${result.score} / ${result.total}',
                  key: const Key('session-results-score'),
                  style: Theme.of(context).textTheme.displaySmall,
                ),
                const SizedBox(height: AppSpacing.sm),
                Text(
                  '$percent%',
                  key: const Key('session-results-percent'),
                  style: Theme.of(context).textTheme.titleMedium?.copyWith(
                    color: scheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: AppSpacing.md),
                Text(
                  _reasonLabel(result.stoppedReason),
                  key: const Key('session-results-reason'),
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: AppSpacing.lg),
                FilledButton(
                  key: const Key('session-results-home-button'),
                  onPressed: () => context.go('/'),
                  child: const Text('Bosh sahifa'),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
