import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../app/l10n/app_localizations.dart';
import '../../../app/theme/app_theme.dart';
import '../../../shared/widgets/empty_state.dart';
import '../../../shared/widgets/question_card.dart';
import '../domain/saved_entry.dart';
import 'saved_controller.dart';
import 'saved_toggle_button.dart';

/// Lists every question the user has bookmarked (`GET /me/saved`, via
/// [savedControllerProvider]). Each row fetches its own full [Question] body
/// lazily (`savedQuestionProvider`, per-id) — `GET /me/saved` only returns
/// `{question_id, created_at}`, never the question's own text/answers/image.
class SavedScreen extends ConsumerWidget {
  const SavedScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(savedControllerProvider);

    return Scaffold(
      key: const Key('saved-screen'),
      appBar: AppBar(title: const Text('Saqlangan savollar')),
      body: SafeArea(
        child: state.when(
          loading: () => const Center(
            key: Key('saved-loading'),
            child: CircularProgressIndicator(),
          ),
          error: (error, stackTrace) => EmptyState(
            key: const Key('saved-error-state'),
            message: error is SavedFetchFailure
                ? error.failure.message
                : 'Saqlangan savollarni yuklab bo\'lmadi.',
            onRetry: () => ref.invalidate(savedControllerProvider),
            retryLabel: 'Qayta urinish',
          ),
          data: (entries) => _SavedList(entries: entries),
        ),
      ),
    );
  }
}

class _SavedList extends StatelessWidget {
  const _SavedList({required this.entries});

  final List<SavedEntry> entries;

  @override
  Widget build(BuildContext context) {
    if (entries.isEmpty) {
      return const _SavedEmptyState();
    }
    return ListView.separated(
      key: const Key('saved-list'),
      padding: const EdgeInsets.all(AppSpacing.md),
      itemCount: entries.length,
      separatorBuilder: (context, index) =>
          const SizedBox(height: AppSpacing.md),
      itemBuilder: (context, index) => _SavedListItem(entry: entries[index]),
    );
  }
}

/// A dedicated (not the shared [EmptyState]) empty-state for "nothing
/// bookmarked yet" — the shared widget has no icon slot, and this state
/// specifically calls for one plus a second line of encouraging copy (not
/// an error, so no retry action either). The original plain message text is
/// preserved verbatim as the headline; the added subtitle is the only new
/// user-facing string this screen introduces.
class _SavedEmptyState extends StatelessWidget {
  const _SavedEmptyState();

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;
    final l10n = AppLocalizations.of(context)!;
    return Center(
      key: const Key('saved-empty-state'),
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.lg),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              padding: const EdgeInsets.all(AppSpacing.lg),
              decoration: BoxDecoration(
                color: colorScheme.primaryContainer,
                shape: BoxShape.circle,
              ),
              child: Icon(
                Icons.bookmark_border_rounded,
                size: 40,
                color: colorScheme.onPrimaryContainer,
              ),
            ),
            const SizedBox(height: AppSpacing.lg),
            Text(
              'Hali hech qanday savol saqlanmagan.',
              textAlign: TextAlign.center,
              style: theme.textTheme.titleMedium,
            ),
            const SizedBox(height: AppSpacing.sm),
            Text(
              l10n.savedEmptyHint,
              textAlign: TextAlign.center,
              style: theme.textTheme.bodyMedium?.copyWith(
                color: colorScheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _SavedListItem extends ConsumerWidget {
  const _SavedListItem({required this.entry});

  final SavedEntry entry;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final questionAsync = ref.watch(savedQuestionProvider(entry.questionId));

    return questionAsync.when(
      loading: () => const Padding(
        padding: EdgeInsets.all(AppSpacing.md),
        child: Center(child: CircularProgressIndicator()),
      ),
      // A single broken/unreachable question must not break the whole list.
      error: (error, stackTrace) => const SizedBox.shrink(),
      data: (question) => Stack(
        key: ValueKey('saved-item-${entry.questionId}'),
        children: [
          QuestionCard(question: question.text, imageUrl: question.imageUrl),
          Positioned(
            top: AppSpacing.sm,
            right: AppSpacing.sm,
            child: SavedToggleButton(questionId: entry.questionId),
          ),
        ],
      ),
    );
  }
}
