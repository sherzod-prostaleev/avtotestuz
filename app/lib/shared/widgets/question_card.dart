import 'package:flutter/material.dart';

import '../../app/theme/app_theme.dart';

/// Renders a single test question's prompt text and, when present, its
/// illustrative image (traffic sign photo/diagram, etc.). Deliberately does
/// NOT render answer options — those are [AnswerOption]/[AnswerOptionsGroup],
/// kept separate so callers can lay out question + options independently
/// (e.g. image above options on narrow screens, side-by-side on wide ones).
class QuestionCard extends StatelessWidget {
  const QuestionCard({required this.question, required this.imageUrl, super.key});

  /// The question's prompt text.
  final String question;

  /// Optional illustration URL. When null, no image (or space for one) is
  /// rendered at all — many questions are text-only.
  final String? imageUrl;

  @override
  Widget build(BuildContext context) {
    final imageUrl = this.imageUrl;
    return Card(
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppRadius.card),
      ),
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.md),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(question, style: Theme.of(context).textTheme.titleMedium),
            if (imageUrl != null) ...[
              const SizedBox(height: AppSpacing.md),
              ClipRRect(
                borderRadius: BorderRadius.circular(AppRadius.card),
                child: Image.network(
                  imageUrl,
                  // A broken/unreachable image must never break the
                  // question-answering flow — collapse to nothing instead of
                  // showing Flutter's default red error box.
                  errorBuilder: (context, error, stackTrace) =>
                      const SizedBox.shrink(),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
