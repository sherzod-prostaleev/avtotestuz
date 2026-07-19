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
/// `ColorScheme` tonal roles (no ad-hoc hex literals), matching the
/// convention `shared/widgets/answer_option.dart` already established for
/// its own correct/incorrect/the-correct-one states.
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

  static _BlockStyle forType(String type, ColorScheme colorScheme) {
    switch (type) {
      case 'intro':
        return _BlockStyle(
          icon: Icons.info_outline,
          background: colorScheme.surface,
          foreground: colorScheme.onSurfaceVariant,
          label: 'INTRO',
        );
      case 'muhim':
        return _BlockStyle(
          icon: Icons.priority_high,
          background: colorScheme.errorContainer,
          foreground: colorScheme.onErrorContainer,
          label: 'MUHIM',
        );
      case 'eslatma':
        return _BlockStyle(
          icon: Icons.push_pin_outlined,
          background: colorScheme.secondaryContainer,
          foreground: colorScheme.onSecondaryContainer,
          label: 'ESLATMA',
        );
      case 'ogohlantirish':
        return _BlockStyle(
          icon: Icons.warning_amber_rounded,
          background: colorScheme.tertiaryContainer,
          foreground: colorScheme.onTertiaryContainer,
          label: 'OGOHLANTIRISH',
        );
      case 'maslahat':
        return _BlockStyle(
          icon: Icons.lightbulb_outline,
          background: colorScheme.primaryContainer,
          foreground: colorScheme.onPrimaryContainer,
          label: 'MASLAHAT',
        );
      case 'answer_analysis':
        return _BlockStyle(
          icon: Icons.fact_check_outlined,
          background: colorScheme.surfaceContainerLow,
          foreground: colorScheme.onSurface,
          label: 'JAVOB TAHLILI',
        );
      case 'xulosa':
        return _BlockStyle(
          icon: Icons.summarize_outlined,
          background: colorScheme.surfaceContainerHigh,
          foreground: colorScheme.onSurface,
          label: 'XULOSA',
        );
      default:
        // Unrecognized type: neutral fallback, still legible/distinct from
        // every known type above, never a crash.
        return _BlockStyle(
          icon: Icons.help_outline,
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
    final colorScheme = Theme.of(context).colorScheme;
    final style = _BlockStyle.forType(block.type, colorScheme);

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
                style: Theme.of(context).textTheme.labelLarge?.copyWith(
                  color: style.foreground,
                ),
              ),
            ],
          ),
          if (block.text != null) ...[
            const SizedBox(height: AppSpacing.sm),
            Text(block.text!, style: TextStyle(color: style.foreground)),
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
    final colorScheme = Theme.of(context).colorScheme;
    return Padding(
      key: Key('answer-analysis-item-${item.position}'),
      padding: const EdgeInsets.symmetric(vertical: 2),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(
            item.correct ? Icons.check_circle : Icons.cancel,
            key: Key('answer-analysis-item-icon-${item.position}'),
            color: item.correct ? colorScheme.primary : colorScheme.error,
            size: 18,
          ),
          const SizedBox(width: AppSpacing.sm),
          Expanded(
            child: Text(
              '${item.position}. ${item.text}',
              style: TextStyle(color: foreground),
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
    final colorScheme = Theme.of(context).colorScheme;
    return Container(
      key: const Key('explanation-legal-refs'),
      width: double.infinity,
      padding: const EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        border: Border.all(color: colorScheme.outlineVariant),
        borderRadius: BorderRadius.circular(AppRadius.card),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            'HUQUQIY ASOSLAR',
            style: Theme.of(context).textTheme.labelLarge,
          ),
          const SizedBox(height: AppSpacing.sm),
          for (final ref in refs)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 2),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    ref.code,
                    style: Theme.of(context).textTheme.labelMedium?.copyWith(
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                  const SizedBox(width: AppSpacing.sm),
                  Expanded(child: Text(ref.title)),
                ],
              ),
            ),
        ],
      ),
    );
  }
}
