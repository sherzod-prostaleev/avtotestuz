import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../../app/l10n/app_localizations.dart';
import '../../../app/theme/app_theme.dart';

/// The route [VipRequiredScreen] is mounted at. Shared as a const so the
/// screen, the router and their tests all agree on the destination — the
/// same pattern `session_screen.dart` uses for [sessionResultRoute].
const vipRequiredRoute = '/vip-required';

/// Shown when `POST /sessions` rejects a start with `vip_required` (402) —
/// bilet #2+, `exam`, and `mistakes` are the gated modes (README's free-limit
/// rules). Reached by navigating to [vipRequiredRoute]; the session screen
/// routes here from its error state (see `session_screen.dart`) rather than
/// rendering the gate inline, so every entry path that can produce a 402
/// (Variants / Mistakes / any future exam entry) lands on one consistent
/// upsell surface.
///
/// **Deliberately an honest M1 placeholder, NOT a fake checkout.** Real
/// billing (Payme/Click sandbox, master spec D12) is M2 territory and simply
/// does not exist in this build yet. So this screen tells the truth — the
/// feature needs an active pass, purchasing isn't available in this version —
/// and offers a real way out (back to Home). It intentionally does not render
/// a price, a "Buy" button, or a payment form that would go nowhere: that
/// would be exactly the kind of half-built fake screen this codebase's own
/// conventions avoid (see how `_ComingSoonNav` was historically kept honest).
class VipRequiredScreen extends StatelessWidget {
  const VipRequiredScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);

    return Scaffold(
      key: const Key('vip-required-screen'),
      appBar: AppBar(title: Text(l10n.vipRequiredTitle)),
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(AppSpacing.lg),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Icon(
                  Icons.workspace_premium_outlined,
                  size: 64,
                  color: theme.colorScheme.primary,
                ),
                const SizedBox(height: AppSpacing.lg),
                Text(
                  l10n.vipRequiredHeadline,
                  textAlign: TextAlign.center,
                  style: theme.textTheme.headlineSmall,
                ),
                const SizedBox(height: AppSpacing.md),
                Text(
                  l10n.vipRequiredBody,
                  textAlign: TextAlign.center,
                  style: theme.textTheme.bodyMedium,
                ),
                const SizedBox(height: AppSpacing.md),
                Text(
                  l10n.vipRequiredPurchaseUnavailable,
                  textAlign: TextAlign.center,
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: AppSpacing.lg),
                FilledButton(
                  key: const Key('vip-required-home-button'),
                  onPressed: () => context.go('/'),
                  child: Text(l10n.homeButton),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
