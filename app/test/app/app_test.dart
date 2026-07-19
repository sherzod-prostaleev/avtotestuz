import 'package:avtotest_app/app/app.dart';
import 'package:avtotest_app/app/locale/locale_provider.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// A [LocaleNotifier] override that always returns a fixed [Locale] and
/// never hydrates from `shared_preferences` — this is what lets the tests
/// below deterministically pin the app to a given locale regardless of
/// whatever async hydration the real [LocaleNotifier] would otherwise kick
/// off.
class _FixedLocale extends LocaleNotifier {
  _FixedLocale(this._locale);

  final Locale _locale;

  @override
  Locale build() => _locale;
}

void main() {
  setUp(() {
    // themeModeProvider also hydrates from shared_preferences on first
    // read; give it a clean, synchronous in-memory store so the widget
    // tree doesn't depend on real platform storage.
    SharedPreferences.setMockInitialValues({});
  });

  testWidgets('renders the placeholder home inside a MaterialApp.router', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          localeProvider.overrideWith(() => _FixedLocale(const Locale('uz'))),
        ],
        child: const AvtotestApp(),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byType(MaterialApp), findsOneWidget);
    expect(find.byType(Scaffold), findsOneWidget);
    expect(find.text('AvtoTest'), findsOneWidget);
  });

  // The two tests below each pump a fresh ProviderScope with a different
  // localeProvider override and assert on the rendered appTitle string.
  // ru's ARB source translates appTitle to the Cyrillic "АвтоТест", while
  // uz's (uz-Latn) translates it to the Latin "AvtoTest" — a real,
  // non-degenerate signal that the correct AppLocalizations instance was
  // resolved for each locale, i.e. that theme/locale/router/DI actually
  // compose end-to-end and not just "it compiles".
  //
  // Deliberately two separate `testWidgets` (rather than two sequential
  // `pumpWidget` calls in one test): Riverpod's Notifier state is preserved
  // across a widget rebuild that reuses the same ProviderScope element, so
  // re-pumping a *different* localeProvider override into an already-built
  // tree does not reliably force a fresh notifier instance. A fresh
  // `testWidgets` gives each override a clean, freshly-built widget tree.
  testWidgets('renders the ru-localized appTitle when localeProvider is ru', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          localeProvider.overrideWith(() => _FixedLocale(const Locale('ru'))),
        ],
        child: const AvtotestApp(),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('АвтоТест'), findsOneWidget);
    expect(find.text('AvtoTest'), findsNothing);
  });

  testWidgets('renders the uz-localized appTitle when localeProvider is uz', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          localeProvider.overrideWith(() => _FixedLocale(const Locale('uz'))),
        ],
        child: const AvtotestApp(),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('AvtoTest'), findsOneWidget);
    expect(find.text('АвтоТест'), findsNothing);
  });
}
