import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../features/auth/domain/auth_state.dart';
import '../features/auth/presentation/auth_controller.dart';
import '../features/auth/presentation/otp_verify_screen.dart';
import '../features/auth/presentation/phone_entry_screen.dart';
import '../features/home/presentation/home_shell.dart';
import '../features/session/domain/session_models.dart';
import '../features/session/presentation/session_controller.dart';
import '../features/session/presentation/session_screen.dart';

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
      GoRoute(path: '/', builder: (context, state) => const HomeShell()),
      GoRoute(
        path: '/login',
        builder: (context, state) => const PhoneEntryScreen(),
      ),
      GoRoute(
        path: '/login/verify',
        builder: (context, state) => const OtpVerifyScreen(),
      ),
      // The session screen is reached by navigating with a
      // [SessionStartRequest] as `extra` — the controller starts the session
      // itself (see [SessionController]), so callers (Variants/Practice/
      // Mistakes) never pre-start one and hand over an id. A missing/wrong
      // `extra` is a programming error (nobody should reach `/session`
      // directly), surfaced as an inline message rather than a crash.
      GoRoute(
        path: '/session',
        builder: (context, state) {
          final request = state.extra;
          if (request is! SessionStartRequest) {
            return const _MissingExtraScreen(
              message: 'Sessiyani boshlash uchun maʼlumot topilmadi.',
            );
          }
          return SessionScreen(request: request);
        },
      ),
      // Results route the session screen navigates to on finish. Task 4 wires
      // the minimal [SessionResultView]; Task 5 replaces this with the full
      // results screen. The [SessionResult] is passed as `extra`.
      GoRoute(
        path: sessionResultRoute,
        builder: (context, state) {
          final result = state.extra;
          if (result is! SessionResult) {
            return const _MissingExtraScreen(
              message: 'Natija maʼlumoti topilmadi.',
            );
          }
          return SessionResultView(result: result);
        },
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

/// Shown when a route that expects a typed `extra` (e.g. `/session`,
/// `/session/result`) is reached without it — a deep link or a caller bug.
/// Offers a way home rather than dead-ending or throwing a cast error.
class _MissingExtraScreen extends StatelessWidget {
  const _MissingExtraScreen({required this.message});

  final String message;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: Center(
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(message, textAlign: TextAlign.center),
                const SizedBox(height: 16),
                FilledButton(
                  onPressed: () => context.go('/'),
                  child: const Text('Bosh sahifa'),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
