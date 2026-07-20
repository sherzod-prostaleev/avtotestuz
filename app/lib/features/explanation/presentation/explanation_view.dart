import 'package:flutter/material.dart';

import '../../../app/theme/app_theme.dart';
import '../../content/domain/question.dart';

/// Renders a verified explanation's ordered [ExplanationBlock]s with a
/// visually distinct treatment per block `type`, plus its `legalRefs` when
/// present.
///
/// Block content/shape comes straight from Plan 06 Task 1's `Question`
/// domain model (`ExplanationBlock`/`AnswerAnalysisItem`/`LegalRef` in
/// `features/content/domain/question.dart`) — this widget does not
/// redefine or re-parse anything, it only renders what's already been
/// decoded there.
///
/// Only ever shown a `Question.explanation` that is non-null, which per
/// Plan 05's server-side invariant (`GetVerifiedExplanation` only returns
/// `verified`-status rows) means the content is always real/verified — this
/// widget deliberately has no "draft"/"AI-QORALAMA"/"pending" state to
/// render, since that state can never actually reach this client.
///
/// `type` values confirmed against `backend/internal/explanation/draft.go`
/// (`TemplateDraftGenerator.Generate`, the only place block types are
/// currently produced): `intro`, `muhim`, `answer_analysis` are the ones
/// actually emitted by that generator today; `eslatma`/`ogohlantirish`/
/// `maslahat`/`xulosa` are the remaining values in master spec §13's
/// taxonomy, not yet emitted by the M1 stub generator but modeled here so
/// expert-authored/future-verified explanations using them render
/// correctly without a follow-up change. Any other/unrecognized `type`
/// falls back to a neutral, still-legible treatment rather than crashing or
/// rendering nothing.
class ExplanationView extends StatelessWidget {
  const ExplanationView({required this.blocks, this.legalRefs, super.key});

  /// Ordered explanation blocks, rendered in list order (order carries
  /// meaning — e.g. `intro` first, `xulosa` last — so this widget never
  /// reorders or groups them by type).
  final List<ExplanationBlock> blocks;

  /// Legal references (e.g. `{"code": "YHQ 6.13", "title": "..."}`). Null or
  /// empty omits the section entirely rather than rendering an empty
  /// heading.
  final List<LegalRef>? legalRefs;

  @override
  Widget build(BuildContext context) {
    final refs = legalRefs;
    return Column(
      key: const Key('explanation-view'),
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        for (final block in blocks)
          Padding(
            padding: const EdgeInsets.only(bottom: AppSpacing.sm),
            child: _ExplanationBlockView(block: block),
          ),
        if (refs != null && refs.isNotEmpty) _LegalRefsSection(refs: refs),
      ],
    );
  }
}

/// Per-type visual styling for one [ExplanationBlock] — background color,
/// icon, and foreground color all drawn from the existing `AppTheme`
/// `ColorScheme`/`AppColors` tonal roles (no ad-hoc hex literals), matching
/// the convention `shared/widgets/answer_option.dart` already established
/// for its own correct/incorrect/the-correct-one states.
///
/// Each type maps to a genuinely distinct accent so a "MUHIM" callout never
/// reads as interchangeable with "ESLATMA"/"OGOHLANTIRISH" — the mapping
/// follows the actual severity/tone of each type's Uzbek meaning:
/// `ogohlantirish` ("caution/warning") gets the strongest alarm-red, one
/// notch above `muhim` ("important")'s attention-amber; `eslatma`
/// ("reminder") is a quiet informational tone; `maslahat` ("tip/advice")
/// borrows the success-green pair (allowed here specifically because a tip
/// block only ever appears inside a *verified answer* explanation — a
/// correctness-adjacent context per the design system's own carve-out for
/// where green may be used outside `AnswerOption`); `xulosa` ("conclusion")
/// is the one block allowed to read as brand-accent, since it's a neutral
/// wrap-up rather than a correctness signal.
class _BlockStyle {
  const _BlockStyle({
    required this.icon,
    required this.background,
    required this.foreground,
    required this.label,
  });

  final IconData icon;
  final Color background;
  final Color foreground;
  final String label;

  static _BlockStyle forType(
    String type,
    ColorScheme colorScheme,
    AppColors appColors,
  ) {
    switch (type) {
      case 'intro':
        return _BlockStyle(
          icon: Icons.info_rounded,
          background: colorScheme.surfaceContainerLow,
          foreground: colorScheme.onSurfaceVariant,
          label: 'INTRO',
        );
      case 'muhim':
        return _BlockStyle(
          icon: Icons.priority_high_rounded,
          background: colorScheme.tertiaryContainer,
          foreground: colorScheme.onTertiaryContainer,
          label: 'MUHIM',
        );
      case 'eslatma':
        return _BlockStyle(
          icon: Icons.push_pin_rounded,
          background: colorScheme.secondaryContainer,
          foreground: colorScheme.onSecondaryContainer,
          label: 'ESLATMA',
        );
      case 'ogohlantirish':
        return _BlockStyle(
          icon: Icons.warning_rounded,
          background: colorScheme.errorContainer,
          foreground: colorScheme.onErrorContainer,
          label: 'OGOHLANTIRISH',
        );
      case 'maslahat':
        return _BlockStyle(
          icon: Icons.lightbulb_rounded,
          background: appColors.successContainer,
          foreground: appColors.onSuccessContainer,
          label: 'MASLAHAT',
        );
      case 'answer_analysis':
        return _BlockStyle(
          icon: Icons.fact_check_rounded,
          background: colorScheme.surfaceContainerHigh,
          foreground: colorScheme.onSurface,
          label: 'JAVOB TAHLILI',
        );
      case 'xulosa':
        return _BlockStyle(
          icon: Icons.check_circle_outline_rounded,
          background: colorScheme.primaryContainer,
          foreground: colorScheme.onPrimaryContainer,
          label: 'XULOSA',
        );
      default:
        // Unrecognized type: neutral fallback, still legible/distinct from
        // every known type above, never a crash.
        return _BlockStyle(
          icon: Icons.help_outline_rounded,
          background: colorScheme.surfaceContainerLowest,
          foreground: colorScheme.onSurfaceVariant,
          label: type.toUpperCase(),
        );
    }
  }
}

class _ExplanationBlockView extends StatelessWidget {
  const _ExplanationBlockView({required this.block});

  final ExplanationBlock block;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final style = _BlockStyle.forType(
      block.type,
      theme.colorScheme,
      context.appColors,
    );

    return Container(
      key: Key('explanation-block-${block.type}'),
      width: double.infinity,
      padding: const EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: style.background,
        borderRadius: BorderRadius.circular(AppRadius.card),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              Icon(
                style.icon,
                key: Key('explanation-block-icon-${block.type}'),
                color: style.foreground,
              ),
              const SizedBox(width: AppSpacing.sm),
              Text(
                style.label,
                style: theme.textTheme.labelLarge?.copyWith(
                  color: style.foreground,
                ),
              ),
            ],
          ),
          if (block.text != null) ...[
            const SizedBox(height: AppSpacing.sm),
            Text(
              block.text!,
              style: theme.textTheme.bodyMedium?.copyWith(
                color: style.foreground,
              ),
            ),
          ],
          if (block.items.isNotEmpty) ...[
            const SizedBox(height: AppSpacing.sm),
            for (final item in block.items)
              _AnswerAnalysisItemView(item: item, foreground: style.foreground),
          ],
        ],
      ),
    );
  }
}

class _AnswerAnalysisItemView extends StatelessWidget {
  const _AnswerAnalysisItemView({required this.item, required this.foreground});

  final AnswerAnalysisItem item;
  final Color foreground;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;
    return Padding(
      key: Key('answer-analysis-item-${item.position}'),
      padding: const EdgeInsets.symmetric(vertical: 3),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(
            item.correct ? Icons.check_circle_rounded : Icons.cancel_rounded,
            key: Key('answer-analysis-item-icon-${item.position}'),
            color: item.correct ? colorScheme.primary : colorScheme.error,
            size: 18,
          ),
          const SizedBox(width: AppSpacing.sm),
          Expanded(
            child: Text(
              '${item.position}. ${item.text}',
              style: theme.textTheme.bodyMedium?.copyWith(color: foreground),
            ),
          ),
        ],
      ),
    );
  }
}

class _LegalRefsSection extends StatelessWidget {
  const _LegalRefsSection({required this.refs});

  final List<LegalRef> refs;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;
    return Container(
      key: const Key('explanation-legal-refs'),
      width: double.infinity,
      padding: const EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: colorScheme.surfaceContainerLow,
        border: Border.all(color: colorScheme.outlineVariant),
        borderRadius: BorderRadius.circular(AppRadius.card),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              Icon(
                Icons.gavel_rounded,
                size: 18,
                color: colorScheme.onSurfaceVariant,
              ),
              const SizedBox(width: AppSpacing.sm),
              Text('HUQUQIY ASOSLAR', style: theme.textTheme.labelLarge),
            ],
          ),
          const SizedBox(height: AppSpacing.sm),
          for (final ref in refs)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 3),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    ref.code,
                    style: theme.textTheme.labelMedium?.copyWith(
                      color: colorScheme.primary,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                  const SizedBox(width: AppSpacing.sm),
                  Expanded(
                    child: Text(ref.title, style: theme.textTheme.bodyMedium),
                  ),
                ],
              ),
            ),
        ],
      ),
    );
  }
}
