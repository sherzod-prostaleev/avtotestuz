import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../app/locale/locale_provider.dart';
import '../../../core/result.dart';
import '../../content/data/content_api.dart';
import '../../content/domain/sign.dart';

/// Wraps a [Failure] from `GET /signs` so it survives as the concrete error
/// object behind [SignsController]'s `AsyncError` — same pattern as
/// `VariantsFetchFailure` (`features/variants/presentation/variants_controller.dart`).
class SignsFetchFailure implements Exception {
  const SignsFetchFailure(this.failure);

  final Failure failure;

  @override
  String toString() => failure.message;
}

/// Free-tier browsable sign catalog (`GET /signs`, `ContentApi.signs`),
/// optionally filtered by `group`/`query`. Deliberately has NO vip-gating
/// logic anywhere in this controller or [SignsScreen] — per D13, signs are
/// explicitly a free-tier feature, unlike `variant`/`exam`/`mistakes`.
///
/// `retry: (_, _) => null` disables Riverpod 3's automatic retry-with-backoff
/// (same reasoning as `VariantsController`): a failure surfaces as
/// `AsyncError` immediately rather than being silently retried behind an
/// unchanged `AsyncLoading`.
final signsControllerProvider =
    AsyncNotifierProvider<SignsController, List<Sign>>(
      SignsController.new,
      retry: (retryCount, error) => null,
    );

class SignsController extends AsyncNotifier<List<Sign>> {
  /// The currently-applied filters. Deliberately plain mutable fields (not
  /// the provider's family key): the catalog re-filters in place rather than
  /// spinning up a new controller instance per filter combination.
  String? _group;
  String? _query;

  ContentApi get _api => ref.read(contentApiProvider);

  @override
  Future<List<Sign>> build() => _fetch();

  Future<List<Sign>> _fetch() async {
    final locale = localeToBackendCode(ref.read(localeProvider));
    final result = await _api.signs(
      group: _group,
      query: _query,
      locale: locale,
    );
    return switch (result) {
      Ok(:final data) => data,
      Err(:final failure) => throw SignsFetchFailure(failure),
    };
  }

  /// Applies a new group filter (`null`/empty clears it) and refetches.
  Future<void> setGroup(String? group) async {
    _group = (group == null || group.isEmpty) ? null : group;
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(_fetch);
  }

  /// Applies a new free-text query filter (empty clears it) and refetches.
  Future<void> setQuery(String query) async {
    _query = query.isEmpty ? null : query;
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(_fetch);
  }
}
