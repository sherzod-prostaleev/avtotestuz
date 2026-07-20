import 'dart:async';

import 'package:google_fonts/google_fonts.dart';

/// Runs once before the whole `test/` suite (Flutter's standard hook for
/// this file name/location). Disables `google_fonts`' runtime network fetch
/// — this sandboxed test environment has no network access, and without
/// this, every widget test that builds `AppTheme.light()`/`dark()` throws
/// (google_fonts falls back to the platform default font silently instead,
/// which is exactly what a test needs; production builds still fetch/cache
/// the real fonts normally, this only affects `flutter test`).
Future<void> testExecutable(FutureOr<void> Function() testMain) async {
  GoogleFonts.config.allowRuntimeFetching = false;
  await testMain();
}
