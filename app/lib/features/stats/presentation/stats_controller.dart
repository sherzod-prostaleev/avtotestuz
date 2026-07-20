import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/result.dart';
import '../data/progress_api.dart';
import '../domain/stats.dart';
import '../domain/streak.dart';

/// The combined value [StatsController] exposes — the streak and the
/// aggregate stats together, since [StatsScreen] renders both on one screen
/// and there's no meaningful "half-loaded" state to show (either both
/// fetches succeed or the screen shows a single error/retry).
typedef StatsData = ({Streak streak, Stats stats});

/// Wraps a [Failure] from `GET /me/streak` or `GET /me/stats` so it survives
/// as the concrete error object behind [StatsController]'s `AsyncError` —
/// same pattern as `VariantsFetchFailure`/`ProfileFetchFailure`, letting the
/// screen surface the backend's actual `error.code`/`message` instead of a
/// generic fallback.
class StatsFetchFailure implements Exception {
  const StatsFetchFailure(this.failure);

  final Failure failure;

  @override
  String toString() => failure.message;
}

/// Fetches the authenticated user's streak + aggregate stats
/// (`GET /me/streak` + `GET /me/stats`) — the data behind [StatsScreen].
///
/// `retry: (_, _) => null` disables Riverpod 3's automatic retry-with-backoff
/// (same reasoning as `ProfileController`/`VariantsController`): a failure
/// surfaces as `AsyncError` immediately rather than being silently retried
/// behind an unchanged `AsyncLoading`.
final statsControllerProvider =
    AsyncNotifierProvider<StatsController, StatsData>(
      StatsController.new,
      retry: (retryCount, error) => null,
    );

class StatsController extends AsyncNotifier<StatsData> {
  ProgressApi get _api => ref.read(progressApiProvider);

  @override
  Future<StatsData> build() async {
    // Fire both requests concurrently (they're independent), then await both —
    // mirrors `ProfileController`'s two-concurrent-fetch idiom.
    final streakFuture = _api.streak();
    final statsFuture = _api.stats();
    final streakResult = await streakFuture;
    final statsResult = await statsFuture;

    final streak = switch (streakResult) {
      Ok(:final data) => data,
      Err(:final failure) => throw StatsFetchFailure(failure),
    };
    final stats = switch (statsResult) {
      Ok(:final data) => data,
      Err(:final failure) => throw StatsFetchFailure(failure),
    };
    return (streak: streak, stats: stats);
  }
}
