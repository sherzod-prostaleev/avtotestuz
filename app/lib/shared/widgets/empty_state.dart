import 'package:flutter/material.dart';

import '../../app/theme/app_theme.dart';
import 'primary_button.dart';

/// A minimal "nothing to show" / "something went wrong" placeholder: a
/// centered message plus an optional retry action, shown instead of a blank
/// screen or a crash. [onRetry]/[retryLabel] are both optional — omit both
/// for a pure informational message with no action.
class EmptyState extends StatelessWidget {
  const EmptyState({
    super.key,
    required this.message,
    this.onRetry,
    this.retryLabel,
  });

  final String message;
  final VoidCallback? onRetry;
  final String? retryLabel;

  @override
  Widget build(BuildContext context) {
    final onRetry = this.onRetry;
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.lg),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              message,
              textAlign: TextAlign.center,
              style: Theme.of(context).textTheme.bodyLarge,
            ),
            if (onRetry != null) ...[
              const SizedBox(height: AppSpacing.md),
              PrimaryButton(
                label: retryLabel ?? 'Retry',
                onPressed: onRetry,
              ),
            ],
          ],
        ),
      ),
    );
  }
}
