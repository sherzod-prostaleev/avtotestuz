import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../app/theme/app_theme.dart';
import '../../../core/result.dart';
import '../data/explanation_api.dart';

/// A simple 👍/👎 toggle for "was this explanation helpful?" feedback,
/// calling `ExplanationApi.feedback` directly (no separate controller —
/// this widget's own state machine is small enough not to warrant one:
/// idle -> submitting -> submitted-thumbsUp/submitted-thumbsDown, or an
/// inline error/not-found message on failure).
///
/// The `not_found` case (backend's `explanation not found`, 404) is handled
/// gracefully with an inline "hali izoh mavjud emas" message rather than a
/// crash or a generic error banner — even though it shouldn't normally
/// occur, since this button is only ever shown next to a `Question` whose
/// `explanation` is already non-null (which per Plan 05's server-side
/// invariant means a verified explanation row already exists for that
/// question).
class ExplanationFeedbackButton extends ConsumerStatefulWidget {
  const ExplanationFeedbackButton({required this.questionId, super.key});

  final String questionId;

  @override
  ConsumerState<ExplanationFeedbackButton> createState() =>
      _ExplanationFeedbackButtonState();
}

enum _Status { idle, submitting, submittedUp, submittedDown, notFound, error }

class _ExplanationFeedbackButtonState
    extends ConsumerState<ExplanationFeedbackButton> {
  _Status _status = _Status.idle;
  String? _errorMessage;

  bool get _isBusyOrDone =>
      _status == _Status.submitting ||
      _status == _Status.submittedUp ||
      _status == _Status.submittedDown;

  Future<void> _submit(bool helpful) async {
    if (_isBusyOrDone) return;
    setState(() {
      _status = _Status.submitting;
      _errorMessage = null;
    });

    final api = ref.read(explanationApiProvider);
    final result = await api.feedback(
      questionId: widget.questionId,
      helpful: helpful,
    );

    if (!mounted) return;
    switch (result) {
      case Ok():
        setState(() {
          _status = helpful ? _Status.submittedUp : _Status.submittedDown;
        });
      case Err(:final failure):
        setState(() {
          if (failure.code == 'not_found') {
            _status = _Status.notFound;
          } else {
            _status = _Status.error;
            _errorMessage = failure.message;
          }
        });
    }
  }

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Row(
          children: [
            Text(
              'Izoh foydali bo\'ldimi?',
              style: Theme.of(context).textTheme.bodyMedium,
            ),
            const SizedBox(width: AppSpacing.sm),
            IconButton(
              key: const Key('explanation-feedback-thumbs-up'),
              onPressed: _isBusyOrDone ? null : () => _submit(true),
              icon: Icon(
                Icons.thumb_up,
                color: _status == _Status.submittedUp
                    ? colorScheme.primary
                    : colorScheme.onSurfaceVariant,
              ),
            ),
            IconButton(
              key: const Key('explanation-feedback-thumbs-down'),
              onPressed: _isBusyOrDone ? null : () => _submit(false),
              icon: Icon(
                Icons.thumb_down,
                color: _status == _Status.submittedDown
                    ? colorScheme.error
                    : colorScheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
        if (_status == _Status.submittedUp || _status == _Status.submittedDown)
          Padding(
            key: const Key('explanation-feedback-confirmation'),
            padding: const EdgeInsets.only(top: AppSpacing.sm),
            child: Text(
              'Fikringiz uchun rahmat!',
              style: Theme.of(
                context,
              ).textTheme.bodySmall?.copyWith(color: colorScheme.primary),
            ),
          ),
        if (_status == _Status.notFound)
          Padding(
            key: const Key('explanation-feedback-not-found'),
            padding: const EdgeInsets.only(top: AppSpacing.sm),
            child: Text(
              'Hali izoh mavjud emas.',
              style: Theme.of(
                context,
              ).textTheme.bodySmall?.copyWith(color: colorScheme.error),
            ),
          ),
        if (_status == _Status.error)
          Padding(
            key: const Key('explanation-feedback-error'),
            padding: const EdgeInsets.only(top: AppSpacing.sm),
            child: Text(
              _errorMessage ?? 'Xatolik yuz berdi. Qayta urinib ko\'ring.',
              style: Theme.of(
                context,
              ).textTheme.bodySmall?.copyWith(color: colorScheme.error),
            ),
          ),
      ],
    );
  }
}
