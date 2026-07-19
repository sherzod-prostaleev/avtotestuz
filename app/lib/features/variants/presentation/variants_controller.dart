import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/result.dart';
import '../../session/data/session_api.dart';
import '../../session/domain/session_models.dart';

/// Wraps a [Failure] from `GET /me/variants` so it survives as the concrete
/// error object behind [VariantsController]'s `AsyncError` — same pattern as
/// `ProfileFetchFailure` (`features/profile/presentation/profile_controller.dart`),
/// letting the screen show the backend's actual `error.code`/`message`
/// instead of a generic fallback.
class VariantsFetchFailure implements Exception {
  const VariantsFetchFailure(this.failure);

  final Failure failure;

  @override
  String toString() => failure.message;
}

/// Fetches the authenticated user's per-bilet unlock/progress status
/// (`GET /me/variants`) — the data behind [VariantsScreen]'s grid.
///
/// `retry: (_, _) => null` disables Riverpod 3's automatic retry-with-backoff
/// (same reasoning as `ProfileController`): a failure surfaces as `AsyncError`
/// immediately rather than being silently retried behind an unchanged
/// `AsyncLoading`.
final variantsControllerProvider =
    AsyncNotifierProvider<VariantsController, List<VariantStatus>>(
      VariantsController.new,
      retry: (retryCount, error) => null,
    );

class VariantsController extends AsyncNotifier<List<VariantStatus>> {
  SessionApi get _api => ref.read(sessionApiProvider);

  @override
  Future<List<VariantStatus>> build() async {
    final result = await _api.myVariants();
    return switch (result) {
      Ok(:final data) => data,
      Err(:final failure) => throw VariantsFetchFailure(failure),
    };
  }
}
