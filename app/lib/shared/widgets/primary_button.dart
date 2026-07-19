import 'package:flutter/material.dart';

/// The app's default primary action button — a thin wrapper around
/// [ElevatedButton] that also knows how to show an inline loading spinner
/// (in place of [label]) while [loading] is true, during which [onPressed]
/// is ignored so a second tap can't double-submit. Establishes the "keep it
/// to a few constructor params" pattern the richer Plan 07 widgets
/// (`QuestionCard`, `AnswerOption`, etc.) will follow.
class PrimaryButton extends StatelessWidget {
  const PrimaryButton({
    super.key,
    required this.label,
    required this.onPressed,
    this.loading = false,
  });

  final String label;
  final VoidCallback? onPressed;
  final bool loading;

  @override
  Widget build(BuildContext context) {
    return ElevatedButton(
      onPressed: loading ? null : onPressed,
      child: loading
          ? const SizedBox(
              height: 18,
              width: 18,
              child: CircularProgressIndicator(strokeWidth: 2),
            )
          : Text(label),
    );
  }
}
