import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'l10n/app_localizations.dart';
import 'locale/locale_provider.dart';
import 'router.dart';
import 'theme/app_theme.dart';
import 'theme/theme_mode_provider.dart';

/// App root. Wires together the four foundation pieces built in Tasks 1-4:
/// theme ([AppTheme]/[themeModeProvider]), locale ([localeProvider]),
/// routing ([routerProvider]), and the Riverpod DI root — with nothing else
/// layered on top yet.
class AvtotestApp extends ConsumerWidget {
  const AvtotestApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final router = ref.watch(routerProvider);
    final themeMode = ref.watch(themeModeProvider);
    final locale = ref.watch(localeProvider);
    return MaterialApp.router(
      routerConfig: router,
      theme: AppTheme.light(),
      darkTheme: AppTheme.dark(),
      themeMode: themeMode,
      locale: locale,
      supportedLocales: AppLocalizations.supportedLocales,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
    );
  }
}
