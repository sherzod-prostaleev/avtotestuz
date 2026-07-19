import 'package:avtotest_app/app/theme/theme_mode_provider.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  group('ThemeModeNotifier', () {
    test('defaults to ThemeMode.dark when nothing is stored', () async {
      final container = ProviderContainer();
      addTearDown(container.dispose);

      expect(container.read(themeModeProvider), ThemeMode.dark);

      // Let the async hydration (which reads "nothing stored") settle too —
      // it must not disturb the already-correct default.
      await pumpEventQueue();
      expect(container.read(themeModeProvider), ThemeMode.dark);
    });

    test('setThemeMode updates the provider state', () async {
      final container = ProviderContainer();
      addTearDown(container.dispose);

      await container.read(themeModeProvider.notifier).setThemeMode(
        ThemeMode.light,
      );

      expect(container.read(themeModeProvider), ThemeMode.light);
    });

    test(
      'setThemeMode persists so a fresh container reads back the new value '
      'instead of the default',
      () async {
        final container = ProviderContainer();
        addTearDown(container.dispose);

        await container.read(themeModeProvider.notifier).setThemeMode(
          ThemeMode.light,
        );

        // Simulate app restart: a brand new container/provider tree.
        final freshContainer = ProviderContainer();
        addTearDown(freshContainer.dispose);

        // Providers initialize lazily: this first read is what triggers
        // build() (and its background hydration). build() itself returns
        // the ThemeMode.dark placeholder synchronously, then hydrates
        // asynchronously from the now-persisted value — let that settle
        // before asserting, same as a real app would after a frame or two
        // on restart.
        freshContainer.read(themeModeProvider);
        await pumpEventQueue();

        expect(freshContainer.read(themeModeProvider), ThemeMode.light);
      },
    );

    test('setThemeMode(ThemeMode.system) persists and is re-read', () async {
      final container = ProviderContainer();
      addTearDown(container.dispose);

      await container.read(themeModeProvider.notifier).setThemeMode(
        ThemeMode.system,
      );

      final freshContainer = ProviderContainer();
      addTearDown(freshContainer.dispose);

      freshContainer.read(themeModeProvider);
      await pumpEventQueue();

      expect(freshContainer.read(themeModeProvider), ThemeMode.system);
    });
  });
}
