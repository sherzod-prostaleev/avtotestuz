import 'package:flutter/material.dart';

import '../../app/theme/app_theme.dart';

/// A minimal card shell built on the app's shape/spacing tokens
/// ([AppRadius.card]/[AppSpacing.md]) — establishes the pattern later, richer
/// cards (e.g. Plan 07's `QuestionCard`) will follow, kept deliberately
/// minimal here.
///
/// Optionally tappable via [onTap]. Set [enabled] to `false` for an honest
/// "coming soon" placeholder entry: renders at reduced opacity and ignores
/// [onTap] entirely, rather than looking interactive but silently doing
/// nothing on tap.
class AppCard extends StatelessWidget {
  const AppCard({super.key, required this.child, this.onTap, this.enabled = true});

  final Widget child;
  final VoidCallback? onTap;
  final bool enabled;

  @override
  Widget build(BuildContext context) {
    final padded = Opacity(
      opacity: enabled ? 1 : 0.5,
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.md),
        child: child,
      ),
    );
    // InkWell must live INSIDE Card (not wrap it): Card provides the
    // Material ancestor InkWell's ink-splash rendering requires. Wrapping
    // Card in an outer InkWell would put that Material below InkWell in the
    // tree, not above it, and InkWell would fail to find an ancestor.
    return Card(
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppRadius.card),
      ),
      child: (!enabled || onTap == null)
          ? padded
          : InkWell(
              borderRadius: BorderRadius.circular(AppRadius.card),
              onTap: onTap,
              child: padded,
            ),
    );
  }
}
