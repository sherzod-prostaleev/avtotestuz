import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/result.dart';
import 'saved_controller.dart';

/// A bookmark-style toggle wired to [savedControllerProvider]: filled
/// bookmark when [questionId] is saved, outline otherwise. Tapping calls
/// [SavedController.toggle], which itself picks save vs. unsave based on the
/// current state — this widget never has to know which one applies.
///
/// Disabled while [savedControllerProvider] hasn't resolved its initial
/// `GET /me/saved` fetch yet (`state.isLoading`) — tapping before the
/// controller knows the current saved set would race against its own
/// `build()`. A failed toggle (network error, etc.) surfaces as a brief
/// snackbar rather than silently flipping the icon to a value the backend
/// never confirmed (`SavedController.toggle` already leaves state untouched
/// on failure; this widget just reports it).
class SavedToggleButton extends ConsumerWidget {
  const SavedToggleButton({required this.questionId, super.key});

  final String questionId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(savedControllerProvider);
    final notifier = ref.read(savedControllerProvider.notifier);
    final isSaved = state.value?.any(
          (e) => e.questionId == questionId,
        ) ??
        false;

    return IconButton(
      key: ValueKey('saved-toggle-$questionId'),
      icon: Icon(isSaved ? Icons.bookmark : Icons.bookmark_border),
      tooltip: isSaved ? 'Saqlangandan olib tashlash' : 'Saqlash',
      onPressed: state.isLoading
          ? null
          : () async {
              final result = await notifier.toggle(questionId);
              if (result case Err(:final failure)) {
                if (!context.mounted) return;
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(content: Text(failure.message)),
                );
              }
            },
    );
  }
}
