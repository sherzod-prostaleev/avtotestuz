import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

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
    final state = ref.watch(signsControllerProvider);
    final notifier = ref.read(signsControllerProvider.notifier);

    return Scaffold(
      appBar: AppBar(title: const Text('Yo\'l belgilari')),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(AppSpacing.md),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              TextField(
                key: const Key('signs-query-field'),
                decoration: const InputDecoration(
                  labelText: 'Qidiruv',
                  prefixIcon: Icon(Icons.search),
                ),
                onChanged: notifier.setQuery,
              ),
              const SizedBox(height: AppSpacing.sm),
              TextField(
                key: const Key('signs-group-field'),
                decoration: const InputDecoration(
                  labelText: 'Guruh kodi (ixtiyoriy)',
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
                        : 'Belgilar ro\'yxatini yuklab bo\'lmadi.',
                    onRetry: () => ref.invalidate(signsControllerProvider),
                    retryLabel: 'Qayta urinish',
                  ),
                  data: (signs) => _SignsList(signs: signs),
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
  const _SignsList({required this.signs});

  final List<Sign> signs;

  @override
  Widget build(BuildContext context) {
    if (signs.isEmpty) {
      return const EmptyState(
        key: Key('signs-empty-state'),
        message: 'Hech qanday belgi topilmadi.',
      );
    }
    return ListView.separated(
      key: const Key('signs-list'),
      itemCount: signs.length,
      separatorBuilder: (context, index) =>
          const SizedBox(height: AppSpacing.sm),
      itemBuilder: (context, index) {
        final sign = signs[index];
        return Align(
          alignment: Alignment.centerLeft,
          child: SignChip(
            key: ValueKey('sign-chip-${sign.code}'),
            name: sign.name,
            imageUrl: sign.imageUrl,
          ),
        );
      },
    );
  }
}
