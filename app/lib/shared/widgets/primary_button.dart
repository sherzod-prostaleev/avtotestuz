import 'package:flutter/material.dart';

/// The app's default primary action button — a thin wrapper around
/// [ElevatedButton] (kept as the actual interactive element so hit-testing,
/// enabled/disabled semantics, and existing structural tests all still work
/// unchanged) plus a solid offset "3D press" shadow underneath it, matching
/// the pill-button look from the app's reference screenshots (repo-root
/// `photo_*.jpg`, gitignored — real competitor screens, not invented).
///
/// The shadow is a zero-blur [BoxShadow] rather than a second stacked
/// widget: Flutter paints box-shadows outside the box's own layout bounds
/// without affecting its size, so this container's geometry is exactly the
/// [ElevatedButton]'s own natural size — no [Stack]/[Positioned] sizing
/// ambiguity for callers (or tests) that measure/tap this widget.
///
/// Also knows how to show an inline loading spinner (in place of [label])
/// while [loading] is true, during which [onPressed] is ignored so a second
/// tap can't double-submit.
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
    final scheme = Theme.of(context).colorScheme;
    final enabled = onPressed != null && !loading;
    final slabColor = enabled
        ? _darken(scheme.primary, 0.18)
        : scheme.outlineVariant;

    return DecoratedBox(
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(999),
        boxShadow: [BoxShadow(color: slabColor, offset: const Offset(0, 4))],
      ),
      child: ElevatedButton(
        onPressed: loading ? null : onPressed,
        child: loading
            ? const SizedBox(
                height: 18,
                width: 18,
                child: CircularProgressIndicator(strokeWidth: 2),
              )
            : Text(label),
      ),
    );
  }
}

Color _darken(Color color, double amount) {
  final hsl = HSLColor.fromColor(color);
  return hsl.withLightness((hsl.lightness - amount).clamp(0.0, 1.0)).toColor();
}
