import 'package:flutter/material.dart';

import '../../app/theme/app_theme.dart';

/// A PURE display widget: renders a given [remaining]/[total] pair as a
/// "M:SS" (or "H:MM:SS" once past an hour) label plus a progress bar.
///
/// Deliberately contains no `Timer.periodic`/ticking logic whatsoever — the
/// actual countdown (decrementing [remaining] over time, stopping at zero,
/// etc.) is Task 4's `SessionController`'s job. Keeping this widget a dumb
/// renderer of whatever [Duration] it's handed is what keeps that countdown
/// logic unit-testable without ever pumping widget frames.
class CountdownTimer extends StatelessWidget {
  const CountdownTimer({required this.remaining, required this.total, super.key});

  final Duration remaining;
  final Duration total;

  /// Below this, the timer renders in a visually distinct "low time" state
  /// (error color + warning icon) — purely a rendering decision, not a
  /// behavioral one; nothing here reacts to time passing on its own.
  static const Duration _lowTimeThreshold = Duration(seconds: 60);

  bool get _isLow => remaining > Duration.zero && remaining <= _lowTimeThreshold;

  double get _progress {
    if (total.inMilliseconds <= 0) return 0;
    final value = remaining.inMilliseconds / total.inMilliseconds;
    return value.clamp(0.0, 1.0);
  }

  /// Formats as "M:SS", or "H:MM:SS" once the duration reaches an hour.
  /// e.g. `Duration(seconds: 89)` -> "1:29".
  static String format(Duration duration) {
    final totalSeconds = duration.inSeconds < 0 ? 0 : duration.inSeconds;
    final hours = totalSeconds ~/ 3600;
    final minutes = (totalSeconds % 3600) ~/ 60;
    final seconds = totalSeconds % 60;
    final secondsStr = seconds.toString().padLeft(2, '0');
    if (hours > 0) {
      final minutesStr = minutes.toString().padLeft(2, '0');
      return '$hours:$minutesStr:$secondsStr';
    }
    return '$minutes:$secondsStr';
  }

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    final color = _isLow ? colorScheme.error : colorScheme.primary;

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (_isLow) ...[
              Icon(
                Icons.warning_amber_rounded,
                key: const ValueKey('countdown-timer-low-icon'),
                color: colorScheme.error,
                size: 20,
              ),
              const SizedBox(width: AppSpacing.sm),
            ],
            Text(
              format(remaining),
              key: const ValueKey('countdown-timer-label'),
              style: Theme.of(
                context,
              ).textTheme.headlineSmall?.copyWith(color: color, fontWeight: FontWeight.bold),
            ),
          ],
        ),
        const SizedBox(height: AppSpacing.sm),
        SizedBox(
          width: 120,
          child: LinearProgressIndicator(
            value: _progress,
            color: color,
            backgroundColor: colorScheme.surfaceContainerHighest,
          ),
        ),
      ],
    );
  }
}
