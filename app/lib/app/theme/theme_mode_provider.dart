import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// Persisted [ThemeMode] selection. Dark is the app's default (master spec
/// §16: "dark (default) + light") until the user explicitly picks otherwise.
final themeModeProvider = NotifierProvider<ThemeModeNotifier, ThemeMode>(
  ThemeModeNotifier.new,
);

/// Reads/writes the user's [ThemeMode] preference via `shared_preferences`
/// (web-compatible via localStorage — same storage trade-off documented for
/// [TokenStorage] in `core/network/token_storage.dart`).
///
/// [Notifier.build] must return synchronously, but `shared_preferences`'
/// first load is asynchronous. To reconcile the two, [build] returns
/// [ThemeMode.dark] immediately (the correct value whenever nothing is
/// stored yet) and kicks off an async hydration that overwrites [state]
/// with the real persisted value once it's available — a no-op when
/// nothing was stored, and a one-time correction otherwise.
class ThemeModeNotifier extends Notifier<ThemeMode> {
  static const _prefsKey = 'theme_mode';

  /// Set the instant [setThemeMode] is called, before any `await`. Guards
  /// against [_hydrate] clobbering an explicit user choice with stale
  /// persisted data if its async read resolves afterwards. This must be a
  /// defended invariant of our own code, not an accident of the relative
  /// timing of two competing `shared_preferences` reads.
  bool _hasUserOverride = false;

  @override
  ThemeMode build() {
    _hydrate();
    return ThemeMode.dark;
  }

  Future<void> _hydrate() async {
    final prefs = await SharedPreferences.getInstance();
    if (!ref.mounted) return;
    // If the user already made an explicit choice by the time this async
    // read resolves, that choice wins — never overwrite it with whatever
    // was persisted from a prior session.
    if (_hasUserOverride) return;
    state = _decode(prefs.getString(_prefsKey));
  }

  Future<void> setThemeMode(ThemeMode mode) async {
    _hasUserOverride = true;
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_prefsKey, _encode(mode));
    if (!ref.mounted) return;
    state = mode;
  }

  static String _encode(ThemeMode mode) => switch (mode) {
    ThemeMode.light => 'light',
    ThemeMode.dark => 'dark',
    ThemeMode.system => 'system',
  };

  static ThemeMode _decode(String? value) => switch (value) {
    'light' => ThemeMode.light,
    'system' => ThemeMode.system,
    _ => ThemeMode.dark,
  };
}
