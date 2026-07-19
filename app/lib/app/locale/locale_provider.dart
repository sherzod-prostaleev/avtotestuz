import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// Two-way mapping between Dart [Locale] objects and the backend's
/// `locale_code` wire values (`"uz-Latn"`, `"uz-Cyrl"`, `"ru"`).
///
/// Dart's `Locale.toString()` does **not** match the backend's format (e.g.
/// `Locale('uz').toString()` is `"uz"`, not `"uz-Latn"`), so every API call
/// site and persistence layer must go through these explicit functions
/// rather than re-deriving the string itself.
String localeToBackendCode(Locale locale) {
  if (locale.languageCode == 'ru') return 'ru';
  if (locale.languageCode == 'uz' && locale.scriptCode == 'Cyrl') {
    return 'uz-Cyrl';
  }
  if (locale.languageCode == 'uz') return 'uz-Latn';
  throw ArgumentError.value(locale, 'locale', 'Unsupported locale');
}

/// Inverse of [localeToBackendCode].
Locale backendCodeToLocale(String code) {
  switch (code) {
    case 'ru':
      return const Locale('ru');
    case 'uz-Cyrl':
      return const Locale.fromSubtags(languageCode: 'uz', scriptCode: 'Cyrl');
    case 'uz-Latn':
      return const Locale('uz');
    default:
      throw ArgumentError.value(code, 'code', 'Unsupported locale code');
  }
}

/// Persisted app [Locale] selection. `uz` (uz-Latn) is the default per the
/// backend's convention (M1 Global Constraints: "Locales exactly uz-Latn
/// (default/fallback), uz-Cyrl, ru").
final localeProvider = NotifierProvider<LocaleNotifier, Locale>(
  LocaleNotifier.new,
);

/// Reads/writes the user's [Locale] preference via `shared_preferences`,
/// storing the backend's wire-format string (via [localeToBackendCode] /
/// [backendCodeToLocale]) so the persisted representation and the API
/// representation are the same value.
///
/// [Notifier.build] must return synchronously, but `shared_preferences`'
/// first load is asynchronous. To reconcile the two, [build] returns
/// `Locale('uz')` immediately (the correct value whenever nothing is stored
/// yet) and kicks off an async hydration that overwrites [state] with the
/// real persisted value once it's available — a no-op when nothing was
/// stored, and a one-time correction otherwise.
class LocaleNotifier extends Notifier<Locale> {
  static const _prefsKey = 'locale';

  /// Set the instant [setLocale] is called, before any `await`. Guards
  /// against [_hydrate] clobbering an explicit user choice with stale
  /// persisted data if its async read resolves afterwards. This must be a
  /// defended invariant of our own code, not an accident of the relative
  /// timing of two competing `shared_preferences` reads.
  bool _hasUserOverride = false;

  @override
  Locale build() {
    _hydrate();
    return const Locale('uz');
  }

  Future<void> _hydrate() async {
    final prefs = await SharedPreferences.getInstance();
    if (!ref.mounted) return;
    // If the user already made an explicit choice by the time this async
    // read resolves, that choice wins — never overwrite it with whatever
    // was persisted from a prior session.
    if (_hasUserOverride) return;
    final stored = prefs.getString(_prefsKey);
    if (stored == null) return;
    state = backendCodeToLocale(stored);
  }

  Future<void> setLocale(Locale locale) async {
    _hasUserOverride = true;
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_prefsKey, localeToBackendCode(locale));
    if (!ref.mounted) return;
    state = locale;
  }
}
