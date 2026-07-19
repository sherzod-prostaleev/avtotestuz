import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'l10n/app_localizations.dart';

/// App-wide router. No auth guard/redirect logic yet — Task 7 adds the
/// `/login` routes and the guard; this task only proves the router/theme/
/// locale wiring works end-to-end via a single placeholder route.
final routerProvider = Provider<GoRouter>((ref) {
  return GoRouter(
    initialLocation: '/',
    routes: [
      GoRoute(path: '/', builder: (context, state) => const _PlaceholderHome()),
    ],
  );
});

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
