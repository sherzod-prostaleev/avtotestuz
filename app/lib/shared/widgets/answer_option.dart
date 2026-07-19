import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../app/theme/app_theme.dart';

/// Feedback state for an already-submitted answer.
///
/// `null` (no [AnswerFeedback] at all) means "not answered yet, no feedback
/// to show" — distinct from all three enum values, which all mean "the
/// session has already told us something about this option."
enum AnswerFeedback {
  /// This option was selected and is correct.
  correct,

  /// This option was selected and is wrong.
  incorrect,

  /// This option was NOT selected, but it is the correct one — shown
  /// alongside [incorrect] so the learner sees what they should have picked.
  theCorrectOne,
}

/// A single selectable answer row: a label chip ('A'..'D'), the answer text,
/// and (once answered) a distinct correct/incorrect/the-correct-one visual
/// treatment. Pure presentational widget — no keyboard handling here, that
/// lives one level up in [AnswerOptionsGroup] since F1-F4 needs a single
/// shortcut scope across all four options, not one per option.
class AnswerOption extends StatelessWidget {
  const AnswerOption({
    required this.label,
    required this.text,
    required this.selected,
    required this.onSelect,
    this.feedback,
    super.key,
  });

  /// 'A'..'D' (or similar) — also what's shown in the label chip.
  final String label;

  final String text;

  final bool selected;

  final VoidCallback onSelect;

  /// null when the question hasn't been answered yet (or feedback is
  /// withheld, e.g. exam mode before the session finishes).
  final AnswerFeedback? feedback;

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    final Color background;
    final Color foreground;
    final IconData? trailingIcon;
    switch (feedback) {
      case AnswerFeedback.correct:
        background = colorScheme.primaryContainer;
        foreground = colorScheme.onPrimaryContainer;
        trailingIcon = Icons.check_circle;
      case AnswerFeedback.incorrect:
        background = colorScheme.errorContainer;
        foreground = colorScheme.onErrorContainer;
        trailingIcon = Icons.cancel;
      case AnswerFeedback.theCorrectOne:
        background = colorScheme.tertiaryContainer;
        foreground = colorScheme.onTertiaryContainer;
        trailingIcon = Icons.check_circle_outline;
      case null:
        background = selected
            ? colorScheme.secondaryContainer
            : colorScheme.surface;
        foreground = selected
            ? colorScheme.onSecondaryContainer
            : colorScheme.onSurface;
        trailingIcon = null;
    }

    return Material(
      key: ValueKey('answer-option-$label'),
      color: background,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppRadius.card),
        side: BorderSide(
          color: selected ? colorScheme.primary : colorScheme.outlineVariant,
          width: selected ? 2 : 1,
        ),
      ),
      child: InkWell(
        borderRadius: BorderRadius.circular(AppRadius.card),
        onTap: onSelect,
        child: Padding(
          padding: const EdgeInsets.symmetric(
            horizontal: AppSpacing.md,
            vertical: AppSpacing.sm,
          ),
          child: Row(
            children: [
              CircleAvatar(
                radius: 14,
                backgroundColor: colorScheme.primary,
                foregroundColor: colorScheme.onPrimary,
                child: Text(label),
              ),
              const SizedBox(width: AppSpacing.sm),
              Expanded(
                child: Text(text, style: TextStyle(color: foreground)),
              ),
              if (trailingIcon != null) Icon(trailingIcon, color: foreground),
            ],
          ),
        ),
      ),
    );
  }
}

/// Wraps a set of (usually four) [AnswerOption]s and binds F1-F4 keyboard
/// shortcuts to them, in list order — the first option is F1, the second
/// F2, and so on. Built here (Task 2) so Task 4's session screen doesn't
/// have to reinvent the shortcut wiring; it just builds four [AnswerOption]s
/// (each already carrying its own `onSelect`) and hands them to this group.
///
/// Uses [CallbackShortcuts], Flutter's standard mechanism for binding a key
/// combination straight to a callback without a separate Actions/Intent
/// dance. [CallbackShortcuts] only receives key events that reach it via the
/// focus system, so this widget also establishes an autofocused [Focus] node
/// beneath it — without a focused descendant, F1-F4 would never bubble up
/// to the shortcut bindings at all.
class AnswerOptionsGroup extends StatelessWidget {
  const AnswerOptionsGroup({required this.options, super.key})
    : assert(
        options.length <= _shortcutKeys.length,
        'AnswerOptionsGroup only wires F1-F${_shortcutKeys.length}; '
        'got ${options.length} options',
      );

  /// The options to render, in F1..F4 order.
  final List<AnswerOption> options;

  static const _shortcutKeys = [
    LogicalKeyboardKey.f1,
    LogicalKeyboardKey.f2,
    LogicalKeyboardKey.f3,
    LogicalKeyboardKey.f4,
  ];

  @override
  Widget build(BuildContext context) {
    final bindings = <ShortcutActivator, VoidCallback>{
      for (var i = 0; i < options.length; i++)
        SingleActivator(_shortcutKeys[i]): options[i].onSelect,
    };

    return CallbackShortcuts(
      bindings: bindings,
      child: Focus(
        autofocus: true,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            for (final option in options)
              Padding(
                padding: const EdgeInsets.only(bottom: AppSpacing.sm),
                child: option,
              ),
          ],
        ),
      ),
    );
  }
}
