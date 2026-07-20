import 'package:flutter/material.dart';

import '../../app/theme/app_theme.dart';

/// A single horizontal mastery/progress bar: an optional [icon] badge, a
/// leading [label], a rounded filled track showing [value] (clamped to 0..1)
/// whose color scales with the mastery level, and a trailing percentage.
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
    this.icon,
    super.key,
  });

  /// Row label (e.g. a category name/code).
  final String label;

  /// Fill fraction in 0..1. Values outside that range are clamped.
  final double value;

  /// Optional secondary text shown under the bar (e.g. "12/20 to'g'ri").
  final String? trailing;

  /// Optional leading icon shown in a small rounded badge (e.g. a per-category
  /// glyph). Omitted entirely when null.
  final IconData? icon;

  /// Mastery-scaled fill color: red at low mastery, gold/brand climbing
  /// through the middle, success-green once the category is well learned.
  /// Uses semantic roles only (never ad-hoc colors) so it stays correct in
  /// both light and dark.
  Color _fillColor(BuildContext context, double v) {
    final colorScheme = Theme.of(context).colorScheme;
    final appColors = context.appColors;
    if (v >= 0.75) return appColors.success;
    if (v >= 0.5) return colorScheme.primary;
    if (v >= 0.25) return appColors.gold;
    return colorScheme.error;
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final clamped = value.clamp(0.0, 1.0);
    final pct = (clamped * 100).round();
    final fill = _fillColor(context, clamped);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            if (icon != null) ...[
              Container(
                width: 34,
                height: 34,
                alignment: Alignment.center,
                decoration: BoxDecoration(
                  color: fill.withValues(alpha: 0.16),
                  borderRadius: BorderRadius.circular(AppRadius.chip),
                ),
                child: Icon(icon, size: 20, color: fill),
              ),
              const SizedBox(width: AppSpacing.sm),
            ],
            Expanded(
              child: Text(
                label,
                overflow: TextOverflow.ellipsis,
                style: theme.textTheme.titleSmall,
              ),
            ),
            const SizedBox(width: AppSpacing.sm),
            Text(
              '$pct%',
              style: theme.textTheme.titleSmall?.copyWith(
                color: fill,
                fontWeight: FontWeight.w700,
              ),
            ),
          ],
        ),
        const SizedBox(height: AppSpacing.sm),
        ClipRRect(
          borderRadius: BorderRadius.circular(999),
          child: LinearProgressIndicator(
            value: clamped,
            minHeight: 10,
            color: fill,
            backgroundColor: theme.colorScheme.surfaceContainerHighest,
          ),
        ),
        if (trailing != null) ...[
          const SizedBox(height: 6),
          Text(
            trailing!,
            style: theme.textTheme.bodySmall,
          ),
        ],
      ],
    );
  }
}
