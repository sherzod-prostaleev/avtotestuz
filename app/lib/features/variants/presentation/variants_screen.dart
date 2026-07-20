import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../app/l10n/app_localizations.dart';
import '../../../app/theme/app_theme.dart';
import '../../../shared/widgets/app_card.dart';
import '../../../shared/widgets/empty_state.dart';
import '../../session/domain/session_models.dart';
import '../../session/presentation/session_controller.dart';
import 'variants_controller.dart';

/// The bilet grid — one cell per [VariantStatus] from `GET /me/variants`
/// (`variantsControllerProvider`). Locked bilets (`unlocked: false`) render
/// visually disabled and non-tappable; unlocked ones are tappable.
///
/// **Design decision — this screen never calls `SessionApi.start` itself.**
/// A tap on an unlocked bilet just builds a [SessionStartRequest] and
/// navigates to `/session` (the route Task 4's [SessionScreen] owns) — the
/// *controller* reached there is the one that actually calls `start()` (see
/// `SessionController`'s doc: "the controller owns `start()`"). This matters
/// for the VIP-gating requirement below: `unlocked: true` here is only a
/// client-side *hint* from the last-fetched variants list, not the
/// authoritative entitlement check — the server's `POST /sessions` response
/// is the source of truth. So a bilet that *looks* unlocked can still come
/// back `vip_required` (e.g. a stale list, or bilet #2+ business rules the
/// server enforces regardless of what this list said) — and because we
/// route through the real `SessionScreen`/`SessionController` rather than
/// re-implementing our own start-call/error-handling here, that 402 lands on
/// the shared paywall-style `VipRequiredScreen` (Task 9) via
/// `SessionScreen`'s error handling, not a generic error banner reinvented
/// at this layer.
///
/// Known contract gap (not fixed here, flagged for whoever owns the backend
/// contract next): [VariantStatus] only carries a bilet `number`, never a
/// UUID — but `POST /sessions`' `variant_id` field is a UUID
/// (`backend/internal/session/handlers.go`'s `startSessionBody.VariantID
/// *uuid.UUID`), and no content/session endpoint currently returns a
/// variant's UUID by number. This screen passes `number.toString()` as
/// `variantId` (matching the literal interface the plan specifies), but a
/// real backend round-trip will 400 (`invalid_body`, a UUID parse failure)
/// until the backend either accepts a bilet number or exposes the UUID
/// somewhere the client can read it.
class VariantsScreen extends ConsumerWidget {
  const VariantsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final state = ref.watch(variantsControllerProvider);

    return Scaffold(
      appBar: AppBar(title: Text(l10n.variantsScreenTitle)),
      body: SafeArea(
        child: state.when(
          loading: () => const Center(
            key: Key('variants-loading'),
            child: CircularProgressIndicator(),
          ),
          error: (error, stackTrace) => EmptyState(
            key: const Key('variants-error-state'),
            message: error is VariantsFetchFailure
                ? error.failure.message
                : l10n.variantsLoadError,
            onRetry: () => ref.invalidate(variantsControllerProvider),
            retryLabel: l10n.retryButton,
          ),
          data: (variants) => _VariantsGrid(variants: variants, l10n: l10n),
        ),
      ),
    );
  }
}

class _VariantsGrid extends StatelessWidget {
  const _VariantsGrid({required this.variants, required this.l10n});

  final List<VariantStatus> variants;
  final AppLocalizations l10n;

  @override
  Widget build(BuildContext context) {
    if (variants.isEmpty) {
      return EmptyState(
        key: const Key('variants-empty-state'),
        message: l10n.variantsEmptyState,
      );
    }
    return GridView.builder(
      key: const Key('variants-grid'),
      padding: const EdgeInsets.all(AppSpacing.md),
      gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: 4,
        mainAxisSpacing: AppSpacing.sm,
        crossAxisSpacing: AppSpacing.sm,
        childAspectRatio: 0.85,
      ),
      itemCount: variants.length,
      itemBuilder: (context, index) => _VariantCell(variant: variants[index], l10n: l10n),
    );
  }
}

class _VariantCell extends StatelessWidget {
  const _VariantCell({required this.variant, required this.l10n});

  final VariantStatus variant;
  final AppLocalizations l10n;

  void _onTap(BuildContext context) {
    context.push(
      '/session',
      extra: SessionStartRequest(
        mode: 'variant',
        variantId: variant.number.toString(),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final locked = !variant.unlocked;
    return AppCard(
      key: ValueKey('variant-cell-${variant.number}'),
      enabled: !locked,
      onTap: locked ? null : () => _onTap(context),
      child: locked
          ? _LockedCellBody(l10n: l10n)
          : _UnlockedCellBody(variant: variant),
    );
  }
}

/// A locked bilet: a muted circular badge around the lock glyph (not just
/// dimmed text) plus the "Yopiq" label — reads as a distinct locked *state*,
/// not a disabled/broken cell. [AppCard]'s own `enabled: false` opacity dip
/// still applies on top of this from the parent.
class _LockedCellBody extends StatelessWidget {
  const _LockedCellBody({required this.l10n});

  final AppLocalizations l10n;

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return Column(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        Container(
          width: 36,
          height: 36,
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            color: colorScheme.surfaceContainerHighest,
          ),
          alignment: Alignment.center,
          child: Icon(
            Icons.lock_outline,
            size: 18,
            color: colorScheme.onSurfaceVariant,
          ),
        ),
        const SizedBox(height: AppSpacing.xs),
        Text(
          l10n.lockedLabel,
          textAlign: TextAlign.center,
          style: Theme.of(context).textTheme.bodySmall,
        ),
      ],
    );
  }
}

/// An unlocked bilet: the bilet number plus a small score chip. The chip is
/// neutral for never-attempted bilets (nothing to show off yet), a
/// success-tinted chip for an attempted bilet's best score, and a
/// [AppColorsX.appColors]`.gold` chip once that best score is a near-perfect
/// run — gold is reserved app-wide for rank/best-score highlights, so it
/// signals "this one you've basically nailed" rather than just "attempted".
class _UnlockedCellBody extends StatelessWidget {
  const _UnlockedCellBody({required this.variant});

  final VariantStatus variant;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final attempted = variant.attempts > 0;
    final ratio = variant.questionCount > 0
        ? variant.bestCorrect / variant.questionCount
        : 0.0;

    final Color chipBackground;
    final Color chipForeground;
    if (!attempted) {
      chipBackground = theme.colorScheme.surfaceContainerHighest;
      chipForeground = theme.colorScheme.onSurfaceVariant;
    } else if (ratio >= 0.9) {
      chipBackground = context.appColors.gold.withValues(alpha: 0.18);
      chipForeground = context.appColors.gold;
    } else {
      chipBackground = context.appColors.successContainer;
      chipForeground = context.appColors.onSuccessContainer;
    }

    return Column(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        Text('${variant.number}', style: theme.textTheme.titleLarge),
        const SizedBox(height: AppSpacing.xs),
        Container(
          padding: const EdgeInsets.symmetric(
            horizontal: AppSpacing.sm,
            vertical: 2,
          ),
          decoration: BoxDecoration(
            color: chipBackground,
            borderRadius: BorderRadius.circular(AppRadius.chip),
          ),
          child: Text(
            '${variant.bestCorrect}/${variant.questionCount}',
            style: theme.textTheme.labelSmall?.copyWith(color: chipForeground),
          ),
        ),
      ],
    );
  }
}
