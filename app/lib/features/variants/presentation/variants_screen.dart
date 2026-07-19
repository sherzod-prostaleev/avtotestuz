import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

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
/// re-implementing our own start-call/error-handling here, that distinct
/// `vip_required` copy (Task 4's `_ErrorView`, already reviewed clean) is
/// what actually renders, not a generic error banner reinvented at this
/// layer.
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
    final state = ref.watch(variantsControllerProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('Biletlar')),
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
                : 'Biletlar ro\'yxatini yuklab bo\'lmadi.',
            onRetry: () => ref.invalidate(variantsControllerProvider),
            retryLabel: 'Qayta urinish',
          ),
          data: (variants) => _VariantsGrid(variants: variants),
        ),
      ),
    );
  }
}

class _VariantsGrid extends StatelessWidget {
  const _VariantsGrid({required this.variants});

  final List<VariantStatus> variants;

  @override
  Widget build(BuildContext context) {
    if (variants.isEmpty) {
      return const EmptyState(
        key: Key('variants-empty-state'),
        message: 'Hozircha biletlar mavjud emas.',
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
      itemBuilder: (context, index) => _VariantCell(variant: variants[index]),
    );
  }
}

class _VariantCell extends StatelessWidget {
  const _VariantCell({required this.variant});

  final VariantStatus variant;

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
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          if (locked)
            const Icon(Icons.lock_outline, size: 20)
          else
            Text(
              '${variant.number}',
              style: Theme.of(context).textTheme.titleLarge,
            ),
          const SizedBox(height: AppSpacing.sm),
          Text(
            locked ? 'Yopiq' : '${variant.bestCorrect}/${variant.questionCount}',
            style: Theme.of(context).textTheme.bodySmall,
          ),
        ],
      ),
    );
  }
}
