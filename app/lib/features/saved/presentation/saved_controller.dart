import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../app/locale/locale_provider.dart';
import '../../../core/result.dart';
import '../../content/data/content_api.dart';
import '../../content/domain/question.dart';
import '../data/saved_api.dart';
import '../domain/saved_entry.dart';

/// Wraps a [Failure] from `GET /me/saved` so it survives as the concrete
/// error object behind [SavedController]'s `AsyncError` — same pattern as
/// `SignsFetchFailure`/`VariantsFetchFailure`.
class SavedFetchFailure implements Exception {
  const SavedFetchFailure(this.failure);

  final Failure failure;

  @override
  String toString() => failure.message;
}

/// The authenticated user's bookmarked-question list (`GET /me/saved`) plus
/// the save/unsave toggle every `QuestionCard` usage site (session screen,
/// [SavedScreen] itself) drives through [toggle].
///
/// `retry: (_, _) => null` disables Riverpod 3's automatic retry-with-backoff
/// (same reasoning as `SignsController`/`VariantsController`): a failure
/// surfaces as `AsyncError` immediately rather than being silently retried
/// behind an unchanged `AsyncLoading`.
final savedControllerProvider =
    AsyncNotifierProvider<SavedController, List<SavedEntry>>(
      SavedController.new,
      retry: (retryCount, error) => null,
    );

class SavedController extends AsyncNotifier<List<SavedEntry>> {
  SavedApi get _api => ref.read(savedApiProvider);

  @override
  Future<List<SavedEntry>> build() async {
    final result = await _api.list();
    return switch (result) {
      Ok(:final data) => data,
      Err(:final failure) => throw SavedFetchFailure(failure),
    };
  }

  /// Whether [questionId] is currently in the saved list. Returns `false`
  /// while the list hasn't loaded yet (or is in error) — deliberately not a
  /// tri-state, since a toggle button has nothing sensible to show for
  /// "unknown" beyond treating it as "not saved".
  bool isSaved(String questionId) {
    final entries = state.value;
    if (entries == null) return false;
    return entries.any((e) => e.questionId == questionId);
  }

  /// Saves [questionId] if it isn't currently saved, unsaves it otherwise.
  ///
  /// On success, the local list is updated in place (append/remove) rather
  /// than refetching the whole `GET /me/saved` list — cheaper, and keeps the
  /// rest of the list (and any other question's toggle button) stable during
  /// the round trip. On failure, the state is left untouched (the caller
  /// gets the [Result] back to surface an error if it wants to) — never
  /// silently flips the toggle to a value the backend didn't confirm.
  Future<Result<void>> toggle(String questionId) async {
    final wasSaved = isSaved(questionId);
    final result = wasSaved
        ? await _api.unsave(questionId)
        : await _api.save(questionId);

    if (result case Ok()) {
      final current = state.value ?? const <SavedEntry>[];
      state = AsyncValue.data(
        wasSaved
            ? current.where((e) => e.questionId != questionId).toList()
            : [
                ...current,
                SavedEntry(questionId: questionId, createdAt: DateTime.now()),
              ],
      );
    }
    return result;
  }
}

/// Fetches a single saved question's full content (text/answers/image) for
/// display on [SavedScreen] — same "list endpoint gives ids, content API
/// gives bodies" split as `session_controller.dart`'s `sessionQuestionProvider`,
/// since `GET /me/saved` only ever returns `{question_id, created_at}` rows.
///
/// Throws (→ [AsyncError]) on failure so the list item can show a retry
/// affordance rather than silently rendering an empty card.
final savedQuestionProvider = FutureProvider.autoDispose
    .family<Question, String>((ref, questionId) async {
      final api = ref.read(contentApiProvider);
      final locale = localeToBackendCode(ref.read(localeProvider));
      final result = await api.question(questionId, locale: locale);
      return switch (result) {
        Ok(:final data) => data,
        Err(:final failure) => throw Exception(failure.message),
      };
    });
