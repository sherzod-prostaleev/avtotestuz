import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../features/auth/domain/auth_state.dart';
import '../features/auth/presentation/auth_controller.dart';
import '../features/auth/presentation/otp_verify_screen.dart';
import '../features/auth/presentation/phone_entry_screen.dart';
import 'l10n/app_localizations.dart';

/// Adapts `authControllerProvider`'s Riverpod state stream into the plain
/// [Listenable] that [GoRouter.refreshListenable] expects.
///
/// This is the piece that makes the auth guard below actually re-fire as
/// auth state changes, rather than only ever applying once at router
/// construction: `GoRouter`'s `redirect` callback re-evaluates *only* when
/// something notifies `refreshListenable` — a bare `ref.read` inside
/// `redirect` with nothing wired to `refreshListenable` would only ever run
/// redirect logic on the first build. Constructed inside `routerProvider`
/// (a plain `Provider`, so it can call `ref.listen`) and disposed via
/// `ref.onDispose` alongside the provider itself.
class _AuthRefreshListenable extends ChangeNotifier {
  _AuthRefreshListenable(Ref ref) {
    ref.listen<AuthState>(authControllerProvider, (previous, next) {
      notifyListeners();
    });
  }
}

/// App-wide router. Routes below `/login` are reachable regardless of auth
/// state; every other route requires [AuthState.authenticated]. See
/// [_authRedirect] for the guard logic and the class doc above for why
/// [GoRouter.refreshListenable] is wired the way it is.
final routerProvider = Provider<GoRouter>((ref) {
  final refreshListenable = _AuthRefreshListenable(ref);
  ref.onDispose(refreshListenable.dispose);

  final router = GoRouter(
    initialLocation: '/',
    refreshListenable: refreshListenable,
    redirect: (context, state) => _authRedirect(ref, state),
    routes: [
      GoRoute(path: '/', builder: (context, state) => const _PlaceholderHome()),
      GoRoute(
        path: '/login',
        builder: (context, state) => const PhoneEntryScreen(),
      ),
      GoRoute(
        path: '/login/verify',
        builder: (context, state) => const OtpVerifyScreen(),
      ),
    ],
  );
  ref.onDispose(router.dispose);
  return router;
});

/// Auth guard: reads the *current* `authControllerProvider` state (a plain
/// `ref.read` is correct here — re-evaluation on state changes is
/// `refreshListenable`'s job, not this callback's) and decides whether the
/// requested location is allowed.
///
/// - [AuthUnknown]: the stored-session check hasn't resolved yet. No
///   redirect — stay wherever was requested. This is intentionally
///   transient: `AuthController.build()` kicks off that check immediately
///   and it resolves near-instantly from local storage, so there's no
///   dedicated splash route in this foundation; whatever's at the current
///   path renders for that brief window.
/// - [AuthUnauthenticated], [AuthOtpRequested], [AuthError]: only `/login*`
///   paths are allowed (`AuthError` happens only from a failed
///   `requestOtp`/`verifyOtp` call, which only ever happens while already on
///   one of those paths) — redirect to `/login` from anywhere else.
/// - [AuthAuthenticated]: the inverse — redirect away from `/login*` to `/`.
String? _authRedirect(Ref ref, GoRouterState state) {
  final authState = ref.read(authControllerProvider);
  final onLoginFlow = state.matchedLocation.startsWith('/login');

  switch (authState) {
    case AuthUnknown():
      return null;
    case AuthUnauthenticated():
    case AuthOtpRequested():
    case AuthError():
      return onLoginFlow ? null : '/login';
    case AuthAuthenticated():
      return onLoginFlow ? '/' : null;
  }
}

/// Trivial placeholder home screen. Its only purpose is to render a
/// localized string so widget tests can assert that theme/locale/router/DI
/// actually compose, rather than merely "it compiles". Replaced by the real
/// home shell in a later task.
class _PlaceholderHome extends StatelessWidget {
  const _PlaceholderHome();

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Center(child: Text(AppLocalizations.of(context)!.appTitle)),
    );
  }
}
