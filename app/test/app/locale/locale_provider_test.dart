import 'dart:async';

import 'package:avtotest_app/app/locale/locale_provider.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shared_preferences_platform_interface/shared_preferences_platform_interface.dart';

/// A `shared_preferences` storage backend whose [getAll] and [setValue]
/// calls only resolve when the test explicitly completes their respective
/// gates. This lets a test pin down the exact moment `_hydrate`'s read
/// resolves relative to `setLocale`'s write, instead of hoping the two
/// happen to interleave in a particular order (which is exactly the kind of
/// implicit, timing-dependent assumption this regression test exists to
/// remove).
class _GatedSharedPreferencesStore extends SharedPreferencesStorePlatform {
  final Completer<Map<String, Object>> getAllGate =
      Completer<Map<String, Object>>();
  final Completer<bool> setValueGate = Completer<bool>();

  final Map<String, Object> _data = {};

  @override
  Future<Map<String, Object>> getAll() => getAllGate.future;

  @override
  Future<bool> setValue(String valueType, String key, Object value) {
    _data[key] = value;
    return setValueGate.future;
  }

  @override
  Future<bool> remove(String key) async {
    _data.remove(key);
    return true;
  }

  @override
  Future<bool> clear() async {
    _data.clear();
    return true;
  }
}

void main() {
  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  group('localeToBackendCode / backendCodeToLocale', () {
    test('ru round-trips', () {
      const locale = Locale('ru');
      expect(localeToBackendCode(locale), 'ru');
      expect(backendCodeToLocale('ru'), locale);
    });

    test('uz (uz-Latn) round-trips', () {
      const locale = Locale('uz');
      expect(localeToBackendCode(locale), 'uz-Latn');
      expect(backendCodeToLocale('uz-Latn'), locale);
    });

    test('uz-Cyrl round-trips', () {
      const locale = Locale.fromSubtags(
        languageCode: 'uz',
        scriptCode: 'Cyrl',
      );
      expect(localeToBackendCode(locale), 'uz-Cyrl');
      expect(backendCodeToLocale('uz-Cyrl'), locale);
    });
  });

  group('LocaleNotifier', () {
    test('defaults to Locale("uz") when nothing is stored', () async {
      final container = ProviderContainer();
      addTearDown(container.dispose);

      expect(container.read(localeProvider), const Locale('uz'));

      // Let the async hydration (which reads "nothing stored") settle too —
      // it must not disturb the already-correct default.
      await pumpEventQueue();
      expect(container.read(localeProvider), const Locale('uz'));
    });

    test('setLocale updates the provider state', () async {
      final container = ProviderContainer();
      addTearDown(container.dispose);

      await container
          .read(localeProvider.notifier)
          .setLocale(const Locale('ru'));

      expect(container.read(localeProvider), const Locale('ru'));
    });

    test(
      'setLocale persists so a fresh container reads back the new value '
      'instead of the default',
      () async {
        final container = ProviderContainer();
        addTearDown(container.dispose);

        await container
            .read(localeProvider.notifier)
            .setLocale(const Locale('ru'));

        // Simulate app restart: a brand new container/provider tree.
        final freshContainer = ProviderContainer();
        addTearDown(freshContainer.dispose);

        // Providers initialize lazily: this first read is what triggers
        // build() (and its background hydration). build() itself returns
        // the Locale('uz') placeholder synchronously, then hydrates
        // asynchronously from the now-persisted value — let that settle
        // before asserting, same as a real app would after a frame or two
        // on restart.
        freshContainer.read(localeProvider);
        await pumpEventQueue();

        expect(freshContainer.read(localeProvider), const Locale('ru'));
      },
    );

    test(
      'setLocale(uz-Cyrl) persists and is re-read on a fresh container',
      () async {
        final container = ProviderContainer();
        addTearDown(container.dispose);

        const uzCyrl = Locale.fromSubtags(
          languageCode: 'uz',
          scriptCode: 'Cyrl',
        );

        await container.read(localeProvider.notifier).setLocale(uzCyrl);

        final freshContainer = ProviderContainer();
        addTearDown(freshContainer.dispose);

        freshContainer.read(localeProvider);
        await pumpEventQueue();

        expect(freshContainer.read(localeProvider), uzCyrl);
      },
    );

    test(
      "setLocale called while _hydrate's read is still pending is not "
      'clobbered once that read resolves with a stale, previously '
      'persisted value',
      () async {
        // Use a store whose `getAll`/`setValue` only resolve when we say
        // so, so the ordering below is deterministic rather than relying
        // on however `shared_preferences` happens to schedule its
        // internal futures today.
        final store = _GatedSharedPreferencesStore();
        SharedPreferencesStorePlatform.instance = store;

        final container = ProviderContainer();
        addTearDown(container.dispose);

        // Reading the provider triggers build(), which returns
        // Locale('uz') synchronously and kicks off _hydrate() in the
        // background. _hydrate's `await SharedPreferences.getInstance()`
        // is now suspended on `store.getAllGate`, which is still
        // incomplete.
        expect(container.read(localeProvider), const Locale('uz'));

        // The user makes an explicit choice *while that read is still
        // pending*. `setLocale` sets `_hasUserOverride = true`
        // synchronously, before it ever awaits anything — so this line
        // takes effect immediately, well before _hydrate's read can
        // possibly resolve.
        final setFuture = container
            .read(localeProvider.notifier)
            .setLocale(const Locale('ru'));

        // Let the pending read resolve with a value simulating a prior
        // session's persisted choice. This unblocks both _hydrate's
        // and setLocale's `SharedPreferences.getInstance()` calls (they
        // share the same underlying instance future).
        store.getAllGate.complete({'flutter.locale': 'uz-Cyrl'});
        await pumpEventQueue();

        // At this checkpoint, _hydrate's read has resolved and its
        // continuation has had a chance to run, but setLocale's own
        // persistence write is still deliberately blocked on
        // `store.setValueGate` -- only its state assignment is pending.
        // A naive, unconditional `state = backendCodeToLocale(...)` in
        // _hydrate would still fire here and overwrite whatever is in
        // `state`. The guarded implementation must skip the write
        // entirely, leaving the state exactly as build() left it.
        expect(container.read(localeProvider), const Locale('uz'));

        // Now let setLocale's persistence write complete too.
        store.setValueGate.complete(true);
        await setFuture;
        await pumpEventQueue();

        // The user's explicit choice — not the stale persisted value —
        // must be the final state.
        expect(container.read(localeProvider), const Locale('ru'));
      },
    );
  });
}
