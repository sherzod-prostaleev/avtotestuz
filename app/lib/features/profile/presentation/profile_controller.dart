import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/result.dart';
import '../data/profile_api.dart';
import '../domain/profile.dart';

/// Supplies the [ProfileApi] the controller talks to. Has no usable
/// default — a real instance (backed by the same configured [Dio] `main.dart`
/// already built for `AuthApi`) is wired at the app root, same "no default,
/// overridden at the app root" seam as `authRepositoryProvider`
/// (`features/auth/presentation/auth_controller.dart`). Tests override this
/// directly with a mock.
final profileApiProvider = Provider<ProfileApi>((ref) {
  throw UnimplementedError(
    'profileApiProvider has no default — override it at the app root with '
    'a real ProfileApi.',
  );
});

/// `retry: (_, _) => null` disables Riverpod 3's default automatic
/// retry-with-backoff for a failed `build()`. That default would silently
/// re-run `build()` a few times behind an unchanged `AsyncLoading` (with its
/// internal `retrying: true` flag) before ever surfacing `AsyncError` —
/// invisible to both this app's UI and to tests, and redundant with (and
/// slower/more confusing than) the explicit manual retry this app already
/// provides via `HomeShell`'s `EmptyState.onRetry` -> `ref.invalidate`. With
/// auto-retry disabled, a failure surfaces as `AsyncError` immediately,
/// exactly once per `build()` call, matching this task's "don't swallow one
/// failure and pretend success" requirement.
final profileControllerProvider = AsyncNotifierProvider<
    ProfileController, ({Profile profile, Entitlement entitlement})>(
  ProfileController.new,
  retry: (retryCount, error) => null,
);

/// Wraps a [Failure] from either the `/me` or `/me/entitlement` call so it
/// survives as the concrete error object behind [ProfileController]'s
/// `AsyncError` (rather than a generic/opaque exception) — lets UI code that
/// catches it (e.g. `HomeShell`'s error branch) show the backend's actual
/// `error.code`/`message` instead of a generic fallback.
class ProfileFetchFailure implements Exception {
  const ProfileFetchFailure(this.failure);

  final Failure failure;

  @override
  String toString() => failure.message;
}

/// Fetches the authenticated user's profile + VIP entitlement, concurrently.
///
/// Both `/me` and `/me/entitlement` calls are started before either is
/// awaited (so they run concurrently), but this deliberately does **not**
/// use `Future.wait([...])` over a single `List` — the two calls return
/// different `Result<T>` types (`Result<Profile>` vs `Result<Entitlement>`),
/// which don't share a useful common list element type. Starting both
/// futures up front and awaiting them in sequence gets the same concurrency
/// without that typing awkwardness.
///
/// If *either* call fails, [build] throws a [ProfileFetchFailure] rather
/// than substituting a default/partial value — that failure propagates as
/// this `AsyncNotifier`'s normal `AsyncError` state (standard `AsyncNotifier`
/// behavior for a future that completes with an error), so a caller can
/// never mistake a partially-failed fetch for a fully successful one.
class ProfileController
    extends AsyncNotifier<({Profile profile, Entitlement entitlement})> {
  ProfileApi get _api => ref.read(profileApiProvider);

  @override
  Future<({Profile profile, Entitlement entitlement})> build() async {
    final profileFuture = _api.fetchMe();
    final entitlementFuture = _api.fetchEntitlement();

    final profileResult = await profileFuture;
    final entitlementResult = await entitlementFuture;

    final profile = switch (profileResult) {
      Ok(data: final data) => data,
      Err(failure: final failure) => throw ProfileFetchFailure(failure),
    };
    final entitlement = switch (entitlementResult) {
      Ok(data: final data) => data,
      Err(failure: final failure) => throw ProfileFetchFailure(failure),
    };

    return (profile: profile, entitlement: entitlement);
  }
}
