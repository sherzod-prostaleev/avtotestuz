import 'package:flutter/material.dart';
import 'package:google_fonts/google_fonts.dart';

/// Material 3 [ThemeData] for the app.
///
/// Visual direction (confirmed with the user against real competitor
/// screenshots — see `photo_*.jpg`/`Снимок экрана_*.png` in the repo root,
/// gitignored): a mix of "premium structure" (generous spacing, a clean
/// modern type scale, restrained motion) and "vibrant/gamified energy"
/// (a vivid signature accent, pill-shaped buttons with a solid 3D press
/// slab, Duolingo-style semantic colors — green=correct, red=incorrect,
/// orange=streak, gold=rank/XP — kept distinct from the brand accent so
/// "correct answer" never gets confused with "brand color"). Dark is the
/// default surface (matches every reference screenshot); light is a real,
/// fully-themed alternative, not an afterthought.
class AppTheme {
  AppTheme._();

  /// Our own signature accent — a vivid indigo-violet. Deliberately NOT
  /// green (that's OsonPrava's whole brand) and not the muted slate-blue
  /// this file used before the 2026-07-20 design pass — bright enough to
  /// read as energetic/modern, saturated enough to anchor a real brand
  /// identity.
  static const Color _brandSeed = Color(0xFF6C63FF);

  static ThemeData light() => _themeFrom(
    ColorScheme.fromSeed(
      seedColor: _brandSeed,
      brightness: Brightness.light,
      primary: const Color(0xFF5A50E0),
      onPrimary: Colors.white,
      secondary: const Color(0xFF0EA5B8),
      tertiary: const Color(0xFFC77700),
      error: const Color(0xFFD53F51),
      surface: const Color(0xFFF7F7FC),
      onSurface: const Color(0xFF161B2E),
    ),
    AppColors.light,
  );

  static ThemeData dark() => _themeFrom(
    ColorScheme.fromSeed(
      seedColor: _brandSeed,
      brightness: Brightness.dark,
      primary: const Color(0xFF8B84FF),
      onPrimary: const Color(0xFF160F45),
      secondary: const Color(0xFF22D3EE),
      tertiary: const Color(0xFFF5A524),
      error: const Color(0xFFFB5A6E),
      surface: const Color(0xFF121A2E),
      onSurface: const Color(0xFFF2F4FA),
    ),
    AppColors.dark,
  );

  static ThemeData _themeFrom(ColorScheme colorScheme, AppColors appColors) {
    final textTheme = _textTheme(colorScheme);
    final isDark = colorScheme.brightness == Brightness.dark;
    final surfaceHigh = isDark
        ? const Color(0xFF1B2440)
        : const Color(0xFFECEDF6);
    final outline = isDark ? const Color(0xFF2A3355) : const Color(0xFFDDE0EE);

    return ThemeData(
      useMaterial3: true,
      brightness: colorScheme.brightness,
      colorScheme: colorScheme,
      scaffoldBackgroundColor: colorScheme.surface,
      textTheme: textTheme,
      extensions: [appColors],
      splashFactory: InkSparkle.splashFactory,
      appBarTheme: AppBarTheme(
        backgroundColor: colorScheme.surface,
        foregroundColor: colorScheme.onSurface,
        surfaceTintColor: Colors.transparent,
        elevation: 0,
        centerTitle: false,
        titleTextStyle: textTheme.titleLarge,
        iconTheme: IconThemeData(color: colorScheme.onSurface),
      ),
      iconTheme: IconThemeData(color: colorScheme.onSurface),
      dividerTheme: DividerThemeData(color: outline, thickness: 1, space: 1),
      cardTheme: CardThemeData(
        color: surfaceHigh,
        elevation: 0,
        surfaceTintColor: Colors.transparent,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppRadius.card),
          side: BorderSide(color: outline),
        ),
      ),
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          backgroundColor: colorScheme.primary,
          foregroundColor: colorScheme.onPrimary,
          disabledBackgroundColor: surfaceHigh,
          disabledForegroundColor: colorScheme.onSurface.withValues(alpha: 0.38),
          elevation: 0,
          shadowColor: Colors.transparent,
          padding: const EdgeInsets.symmetric(horizontal: 28, vertical: 16),
          shape: const StadiumBorder(),
          textStyle: textTheme.labelLarge?.copyWith(fontSize: 16),
        ),
      ),
      filledButtonTheme: FilledButtonThemeData(
        style: FilledButton.styleFrom(
          backgroundColor: colorScheme.primary,
          foregroundColor: colorScheme.onPrimary,
          padding: const EdgeInsets.symmetric(horizontal: 28, vertical: 16),
          shape: const StadiumBorder(),
          textStyle: textTheme.labelLarge?.copyWith(fontSize: 16),
        ),
      ),
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: OutlinedButton.styleFrom(
          foregroundColor: colorScheme.onSurface,
          side: BorderSide(color: outline, width: 1.5),
          padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 14),
          shape: const StadiumBorder(),
          textStyle: textTheme.labelLarge,
        ),
      ),
      textButtonTheme: TextButtonThemeData(
        style: TextButton.styleFrom(
          foregroundColor: colorScheme.primary,
          shape: const StadiumBorder(),
          textStyle: textTheme.labelLarge,
        ),
      ),
      iconButtonTheme: IconButtonThemeData(
        style: IconButton.styleFrom(
          backgroundColor: surfaceHigh,
          foregroundColor: colorScheme.onSurface,
          shape: const StadiumBorder(),
        ),
      ),
      chipTheme: ChipThemeData(
        backgroundColor: surfaceHigh,
        selectedColor: colorScheme.primaryContainer,
        side: BorderSide(color: outline),
        shape: const StadiumBorder(),
        labelStyle: textTheme.labelLarge,
        padding: const EdgeInsets.symmetric(
          horizontal: AppSpacing.sm,
          vertical: 4,
        ),
      ),
      segmentedButtonTheme: SegmentedButtonThemeData(
        style: ButtonStyle(
          shape: const WidgetStatePropertyAll(StadiumBorder()),
          side: WidgetStatePropertyAll(BorderSide(color: outline)),
          backgroundColor: WidgetStateProperty.resolveWith(
            (states) => states.contains(WidgetState.selected)
                ? colorScheme.primary
                : surfaceHigh,
          ),
          foregroundColor: WidgetStateProperty.resolveWith(
            (states) => states.contains(WidgetState.selected)
                ? colorScheme.onPrimary
                : colorScheme.onSurface,
          ),
        ),
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: surfaceHigh,
        contentPadding: const EdgeInsets.symmetric(
          horizontal: AppSpacing.md,
          vertical: AppSpacing.md,
        ),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(AppRadius.field),
          borderSide: BorderSide(color: outline),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(AppRadius.field),
          borderSide: BorderSide(color: outline),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(AppRadius.field),
          borderSide: BorderSide(color: colorScheme.primary, width: 2),
        ),
      ),
      progressIndicatorTheme: ProgressIndicatorThemeData(
        color: colorScheme.primary,
        linearTrackColor: surfaceHigh,
        circularTrackColor: surfaceHigh,
      ),
      snackBarTheme: SnackBarThemeData(
        backgroundColor: surfaceHigh,
        contentTextStyle: textTheme.bodyMedium,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppRadius.field),
        ),
        behavior: SnackBarBehavior.floating,
      ),
    );
  }

  static TextTheme _textTheme(ColorScheme colorScheme) {
    final onSurface = colorScheme.onSurface;
    // Baloo 2: bold, rounded, playful — headings/titles/big numbers (the
    // "energetic" half of the brief). Manrope: clean, highly legible modern
    // sans — body copy (the "premium structure" half). Bundled via
    // google_fonts (TTFs fetched once, cached) rather than hand-sourced
    // asset files.
    final display = GoogleFonts.baloo2TextTheme();
    final body = GoogleFonts.manropeTextTheme();

    return TextTheme(
      displayLarge: display.displayLarge?.copyWith(
        fontSize: 57,
        fontWeight: FontWeight.w700,
        color: onSurface,
      ),
      displayMedium: display.displayMedium?.copyWith(
        fontSize: 45,
        fontWeight: FontWeight.w700,
        color: onSurface,
      ),
      displaySmall: display.displaySmall?.copyWith(
        fontSize: 36,
        fontWeight: FontWeight.w700,
        color: onSurface,
      ),
      headlineLarge: display.headlineLarge?.copyWith(
        fontSize: 32,
        fontWeight: FontWeight.w700,
        color: onSurface,
      ),
      headlineMedium: display.headlineMedium?.copyWith(
        fontSize: 28,
        fontWeight: FontWeight.w700,
        color: onSurface,
      ),
      headlineSmall: display.headlineSmall?.copyWith(
        fontSize: 24,
        fontWeight: FontWeight.w700,
        color: onSurface,
      ),
      titleLarge: display.titleLarge?.copyWith(
        fontSize: 22,
        fontWeight: FontWeight.w700,
        color: onSurface,
      ),
      titleMedium: body.titleMedium?.copyWith(
        fontSize: 16,
        fontWeight: FontWeight.w600,
        color: onSurface,
      ),
      titleSmall: body.titleSmall?.copyWith(
        fontSize: 14,
        fontWeight: FontWeight.w600,
        color: onSurface,
      ),
      bodyLarge: body.bodyLarge?.copyWith(
        fontSize: 16,
        fontWeight: FontWeight.w400,
        color: onSurface,
      ),
      bodyMedium: body.bodyMedium?.copyWith(
        fontSize: 14,
        fontWeight: FontWeight.w400,
        color: onSurface,
      ),
      bodySmall: body.bodySmall?.copyWith(
        fontSize: 12,
        fontWeight: FontWeight.w400,
        color: colorScheme.onSurfaceVariant,
      ),
      labelLarge: body.labelLarge?.copyWith(
        fontSize: 14,
        fontWeight: FontWeight.w700,
        color: onSurface,
      ),
      labelMedium: body.labelMedium?.copyWith(
        fontSize: 12,
        fontWeight: FontWeight.w600,
        color: onSurface,
      ),
      labelSmall: body.labelSmall?.copyWith(
        fontSize: 11,
        fontWeight: FontWeight.w600,
        color: colorScheme.onSurfaceVariant,
      ),
    );
  }
}

/// Semantic colors outside Material 3's [ColorScheme] roles (which has no
/// "success" or "streak"/"gold" slot). Kept as a real [ThemeExtension] (not
/// static constants) so light/dark both get correct, theme-aware values and
/// `ThemeData.lerp` (e.g. during theme-mode transitions) interpolates them
/// smoothly instead of jump-cutting.
///
/// Deliberately distinct from [ColorScheme.primary] (the brand accent): a
/// correct-answer highlight must never read as "brand color", it must read
/// as "correct" — same reasoning Duolingo/most quiz apps use green
/// specifically for that, independent of their own brand hue.
@immutable
class AppColors extends ThemeExtension<AppColors> {
  const AppColors({
    required this.success,
    required this.onSuccess,
    required this.successContainer,
    required this.onSuccessContainer,
    required this.streak,
    required this.gold,
  });

  final Color success;
  final Color onSuccess;
  final Color successContainer;
  final Color onSuccessContainer;

  /// Streak-flame orange (Duolingo-convention: streak is always orange,
  /// distinct from success-green and brand-accent).
  final Color streak;

  /// Rank/medal/XP gold accent (leaderboards, best-score highlights).
  final Color gold;

  static const light = AppColors(
    success: Color(0xFF1B9C63),
    onSuccess: Colors.white,
    successContainer: Color(0xFFD3F3E2),
    onSuccessContainer: Color(0xFF0B4A2C),
    streak: Color(0xFFE8730A),
    gold: Color(0xFFB8860B),
  );

  static const dark = AppColors(
    success: Color(0xFF34D399),
    onSuccess: Color(0xFF04321F),
    successContainer: Color(0xFF184E37),
    onSuccessContainer: Color(0xFFBBF7D8),
    streak: Color(0xFFFF8A3D),
    gold: Color(0xFFFFD166),
  );

  @override
  AppColors copyWith({
    Color? success,
    Color? onSuccess,
    Color? successContainer,
    Color? onSuccessContainer,
    Color? streak,
    Color? gold,
  }) {
    return AppColors(
      success: success ?? this.success,
      onSuccess: onSuccess ?? this.onSuccess,
      successContainer: successContainer ?? this.successContainer,
      onSuccessContainer: onSuccessContainer ?? this.onSuccessContainer,
      streak: streak ?? this.streak,
      gold: gold ?? this.gold,
    );
  }

  @override
  AppColors lerp(ThemeExtension<AppColors>? other, double t) {
    if (other is! AppColors) return this;
    return AppColors(
      success: Color.lerp(success, other.success, t)!,
      onSuccess: Color.lerp(onSuccess, other.onSuccess, t)!,
      successContainer: Color.lerp(successContainer, other.successContainer, t)!,
      onSuccessContainer:
          Color.lerp(onSuccessContainer, other.onSuccessContainer, t)!,
      streak: Color.lerp(streak, other.streak, t)!,
      gold: Color.lerp(gold, other.gold, t)!,
    );
  }
}

/// Ergonomic accessor: `context.appColors.success` instead of
/// `Theme.of(context).extension<AppColors>()!`.
///
/// Falls back to [AppColors.dark] rather than throwing when the current
/// [ThemeData] has no [AppColors] extension registered — many existing
/// widget tests deliberately build a bare `MaterialApp(home: ...)` without
/// the app's real theme (isolating the widget under test from app-wide
/// wiring), and a hard null-check here would break every one of them for a
/// widget that merely *can* render a success/streak/gold color, not just
/// ones that exercise that exact state this render.
extension AppColorsX on BuildContext {
  AppColors get appColors =>
      Theme.of(this).extension<AppColors>() ?? AppColors.dark;
}

/// Named spacing constants for consistent layout rhythm.
class AppSpacing {
  AppSpacing._();

  static const double xs = 4;
  static const double sm = 8;
  static const double md = 16;
  static const double lg = 24;
  static const double xl = 32;
}

/// Named corner-radius constants for consistent shape language. Generously
/// rounded across the board — matches the reference screenshots' soft,
/// pill-leaning shape language (nothing in this app should render with
/// sharp/boxy corners).
class AppRadius {
  AppRadius._();

  static const double card = 20;
  static const double field = 16;
  static const double chip = 14;
}
