import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../app/l10n/app_localizations.dart';
import '../../../app/theme/app_theme.dart';
import '../../../shared/widgets/empty_state.dart';
import '../../../shared/widgets/sign_chip.dart';
import '../../content/domain/sign.dart';
import 'signs_controller.dart';

/// Browsable road-sign catalog (`GET /signs`, `signsControllerProvider`),
/// filterable by group code and free-text search. Explicitly a free-tier
/// screen — per D13, signs carry no VIP gating anywhere, unlike
/// `variant`/`exam`/`mistakes`, so there is deliberately no error-code
/// special-casing here beyond a plain fetch-failure retry.
class SignsScreen extends ConsumerWidget {
  const SignsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final state = ref.watch(signsControllerProvider);
    final notifier = ref.read(signsControllerProvider.notifier);

    return Scaffold(
      appBar: AppBar(title: Text(l10n.signsScreenTitle)),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(AppSpacing.md),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              TextField(
                key: const Key('signs-query-field'),
                decoration: InputDecoration(
                  labelText: l10n.searchLabel,
                  prefixIcon: const Icon(Icons.search_rounded),
                ),
                onChanged: notifier.setQuery,
              ),
              const SizedBox(height: AppSpacing.sm),
              TextField(
                key: const Key('signs-group-field'),
                decoration: InputDecoration(
                  labelText: l10n.groupCodeLabel,
                  prefixIcon: const Icon(Icons.filter_alt_outlined),
                ),
                onChanged: notifier.setGroup,
              ),
              const SizedBox(height: AppSpacing.md),
              Expanded(
                child: state.when(
                  loading: () => const Center(
                    key: Key('signs-loading'),
                    child: CircularProgressIndicator(),
                  ),
                  error: (error, stackTrace) => EmptyState(
                    key: const Key('signs-error-state'),
                    message: error is SignsFetchFailure
                        ? error.failure.message
                        : l10n.signsLoadError,
                    onRetry: () => ref.invalidate(signsControllerProvider),
                    retryLabel: l10n.retryButton,
                  ),
                  data: (signs) => _SignsList(signs: signs, l10n: l10n),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _SignsList extends StatelessWidget {
  const _SignsList({required this.signs, required this.l10n});

  final List<Sign> signs;
  final AppLocalizations l10n;

  @override
  Widget build(BuildContext context) {
    if (signs.isEmpty) {
      return EmptyState(
        key: const Key('signs-empty-state'),
        message: l10n.signsEmptyState,
      );
    }
    // A wrapping catalog grid (rather than one sign per row) reads like a
    // real browsable catalog and lets short/long sign names size themselves
    // naturally instead of every cell being forced to one uniform width.
    return SingleChildScrollView(
      key: const Key('signs-list'),
      child: Wrap(
        spacing: AppSpacing.sm,
        runSpacing: AppSpacing.sm,
        children: [
          for (final sign in signs)
            SignChip(
              key: ValueKey('sign-chip-${sign.code}'),
              name: sign.name,
              imageUrl: sign.imageUrl,
            ),
        ],
      ),
    );
  }
}
