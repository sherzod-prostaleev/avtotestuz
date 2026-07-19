import 'package:flutter/material.dart';

import '../../app/theme/app_theme.dart';

/// A 1..N grid of question cells (per master spec §11: exam-mode navigator)
/// with three visually distinct states: current, answered, and unanswered.
/// Tapping a cell calls [onJump] with that cell's (zero-based) index.
///
/// "Flagged"/"belgilangan" is deliberately omitted — the master spec lists
/// it as optional, and there's no real flag-toggle control anywhere yet
/// (Task 4 doesn't wire one either); adding a flag visual with no way to
/// actually set it would be a dead control, so it's left out until a task
/// actually wires a toggle.
class QuestionNavigator extends StatelessWidget {
  const QuestionNavigator({
    required this.total,
    required this.currentIndex,
    required this.answeredIndices,
    required this.onJump,
    super.key,
  });

  final int total;
  final int currentIndex;
  final Set<int> answeredIndices;
  final ValueChanged<int> onJump;

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    return GridView.builder(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: 5,
        mainAxisSpacing: AppSpacing.sm,
        crossAxisSpacing: AppSpacing.sm,
      ),
      itemCount: total,
      itemBuilder: (context, index) {
        final isCurrent = index == currentIndex;
        final isAnswered = answeredIndices.contains(index);

        final Color background;
        final Color foreground;
        if (isCurrent) {
          background = colorScheme.primary;
          foreground = colorScheme.onPrimary;
        } else if (isAnswered) {
          background = colorScheme.secondaryContainer;
          foreground = colorScheme.onSecondaryContainer;
        } else {
          background = colorScheme.surfaceContainerHighest;
          foreground = colorScheme.onSurfaceVariant;
        }

        return Material(
          key: ValueKey('question-navigator-cell-$index'),
          color: background,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(AppRadius.card / 2),
          ),
          child: InkWell(
            borderRadius: BorderRadius.circular(AppRadius.card / 2),
            onTap: () => onJump(index),
            child: Center(
              child: Text(
                '${index + 1}',
                style: TextStyle(
                  color: foreground,
                  fontWeight: isCurrent ? FontWeight.bold : FontWeight.normal,
                ),
              ),
            ),
          ),
        );
      },
    );
  }
}
