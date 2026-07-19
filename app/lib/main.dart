import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'app/app.dart';
import 'core/network/api_client.dart';
import 'core/network/token_storage.dart';
import 'features/auth/data/auth_api.dart';
import 'features/auth/data/auth_repository.dart';
import 'features/auth/presentation/auth_controller.dart';
import 'features/content/data/content_api.dart';
import 'features/profile/data/profile_api.dart';
import 'features/profile/presentation/profile_controller.dart';

/// Default dev backend base URL — matches this plan's Global Constraints
/// (`PORT=8090` convention) and Task 9's live-verification
/// `--dart-define=API_BASE_URL=...` override.
const _defaultApiBaseUrl = 'http://localhost:8090/api/v1';

void main() {
  // Real DI wiring for `authRepositoryProvider` (Task 6 left it as
  // `throw UnimplementedError` — a deliberate seam closed here, the first
  // point in the app where auth UI becomes reachable). This must run before
  // `authControllerProvider` is ever read (i.e. before the router/screens
  // mount), so it happens in `main()` rather than inside a widget further
  // down the tree.
  //
  // Building the real `AuthRepository` requires an `onSessionExpired`
  // callback that can invalidate `authControllerProvider` — which in turn
  // requires a `ProviderContainer` reference. A widget-level
  // `ProviderScope(overrides: [...])` can't supply that (its overrides list
  // is built before any container/ref exists), so the container is built
  // explicitly here instead, with `onSessionExpired` closing over it. The
  // closure only ever *calls* `container` (on a future 401-refresh
  // failure), never during its own construction, so capturing a not-yet-
  // initialized `late` local is safe.
  late final ProviderContainer container;

  final tokenStorage = SharedPrefsTokenStorage();
  final dio = buildDio(
    baseUrl: const String.fromEnvironment(
      'API_BASE_URL',
      defaultValue: _defaultApiBaseUrl,
    ),
    tokenStorage: tokenStorage,
    onSessionExpired: () async {
      // The session is gone (refresh itself failed/was rejected): drop the
      // controller's in-memory state so it re-checks `hasStoredSession()`
      // (tokens were already cleared by the interceptor) and lands on
      // `AuthUnauthenticated` — which `routerProvider`'s guard then
      // correctly redirects to `/login` for, since `_AuthRefreshListenable`
      // is notified as part of the same rebuild that follows invalidation.
      container.invalidate(authControllerProvider);
    },
  );
  final authRepository = AuthRepository(
    api: AuthApi(dio),
    tokenStorage: tokenStorage,
  );

  // Real DI wiring for `profileApiProvider` (Task 8's gap-closing note,
  // same pattern as `authRepositoryProvider` above): reuses the SAME `dio`
  // instance built above for `AuthApi` rather than calling `buildDio` a
  // second time — a second `Dio` would mean two separate token/interceptor
  // states (e.g. two independent refresh-token races) drifting
  // independently of each other.
  final profileApi = ProfileApi(dio);

  // Real DI wiring for `contentApiProvider` (Plan 07 Task 1), reusing the
  // SAME shared `dio` instance as `AuthApi`/`ProfileApi` above — never a
  // second `buildDio` call, per this plan's Global Constraints.
  final contentApi = ContentApi(dio);

  container = ProviderContainer(
    overrides: [
      authRepositoryProvider.overrideWithValue(authRepository),
      profileApiProvider.overrideWithValue(profileApi),
      contentApiProvider.overrideWithValue(contentApi),
    ],
  );

  runApp(
    UncontrolledProviderScope(container: container, child: const AvtotestApp()),
  );
}
