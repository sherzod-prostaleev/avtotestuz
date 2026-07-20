import 'package:flutter/material.dart';

import '../../app/theme/app_theme.dart';

/// A single horizontal mastery/progress bar: a leading [label], a filled
/// track showing [value] (clamped to 0..1), and a trailing percentage.
///
/// Lives in `shared/widgets` per the master spec's §15 shared-widget list
/// (`MasteryBar`) — used by [StatsScreen]'s per-category breakdown, and
/// reusable anywhere a labelled 0..1 progress bar is needed. Pure display:
/// it renders whatever [value] it's given, computing no data itself.
class MasteryBar extends StatelessWidget {
  const MasteryBar({
    required this.label,
    required this.value,
    this.trailing,
    super.key,
  });

  /// Row label (e.g. a category name/code).
  final String label;

  /// Fill fraction in 0..1. Values outside that range are clamped.
  final double value;

  /// Optional secondary text shown under the bar (e.g. "12/20 to'g'ri").
  final String? trailing;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final clamped = value.clamp(0.0, 1.0);
    final pct = (clamped * 100).round();

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                label,
                overflow: TextOverflow.ellipsis,
                style: theme.textTheme.bodyMedium,
              ),
            ),
            const SizedBox(width: AppSpacing.sm),
            Text('$pct%', style: theme.textTheme.bodyMedium),
          ],
        ),
        const SizedBox(height: 4),
        ClipRRect(
          borderRadius: BorderRadius.circular(6),
          child: LinearProgressIndicator(
            value: clamped,
            minHeight: 8,
            backgroundColor: theme.colorScheme.surfaceContainerHighest,
          ),
        ),
        if (trailing != null) ...[
          const SizedBox(height: 4),
          Text(
            trailing!,
            style: theme.textTheme.bodySmall,
          ),
        ],
      ],
    );
  }
}
