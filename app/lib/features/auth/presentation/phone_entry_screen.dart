import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../app/l10n/app_localizations.dart';
import '../../../app/theme/app_theme.dart';
import '../domain/auth_state.dart';
import 'auth_controller.dart';

/// Validates a phone string against the backend's accepted formats
/// (`backend/internal/auth/phone.go`'s `NormalizePhone`, confirmed by
/// reading that file directly — commit `308583c`): after stripping every
/// non-digit character, the result must be either a bare 9-digit local
/// number (e.g. `"901112233"`) or a 12-digit `998`-prefixed number (e.g.
/// `"998901112233"`, with or without a leading `+`, since `+` is stripped).
/// Anything else is rejected client-side before ever reaching the server.
bool isValidUzPhone(String raw) {
  final digits = raw.replaceAll(RegExp(r'[^0-9]'), '');
  if (digits.length == 9) return true;
  if (digits.length == 12 && digits.startsWith('998')) return true;
  return false;
}

/// First screen of the phone+OTP auth flow: collects a phone number and
/// requests an OTP for it via [AuthController.requestOtp]. Reachable at
/// `/login` (`app/lib/app/router.dart`).
class PhoneEntryScreen extends ConsumerStatefulWidget {
  const PhoneEntryScreen({super.key});

  @override
  ConsumerState<PhoneEntryScreen> createState() => _PhoneEntryScreenState();
}

class _PhoneEntryScreenState extends ConsumerState<PhoneEntryScreen> {
  final _phoneController = TextEditingController();

  /// Inline validation error, shown under the field. Distinct from
  /// [AuthError] (the backend/network failure surfaced via
  /// `authControllerProvider`'s state) — this one never leaves the device.
  String? _formError;

  /// Tracks the single in-flight `requestOtp` call this screen may have
  /// started, so the submit button can disable itself for exactly the
  /// lifetime of that one request. `AuthState` (Task 6's domain type) has no
  /// "submitting" variant of its own — extending the sealed union just for a
  /// transient UI affordance would ripple into every existing exhaustive
  /// switch over it for little benefit. This flag is set synchronously
  /// immediately before the `await` and cleared in a `finally` immediately
  /// after, so it stays mechanically 1:1 with that specific request and
  /// cannot drift the way an independently-guessed loading flag could.
  bool _submitting = false;

  @override
  void dispose() {
    _phoneController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final phone = _phoneController.text.trim();
    final l10n = AppLocalizations.of(context)!;
    if (!isValidUzPhone(phone)) {
      setState(() => _formError = l10n.phoneInvalidError);
      return;
    }
    setState(() {
      _formError = null;
      _submitting = true;
    });
    try {
      await ref.read(authControllerProvider.notifier).requestOtp(phone);
      // requestOtp only updates authControllerProvider's state — nothing
      // about `/login` -> `/login/verify` is automatic: the router's
      // guard (`_authRedirect` in `app/lib/app/router.dart`) deliberately
      // does NOT redirect away from `/login*` while AuthOtpRequested
      // (that's what lets OtpVerifyScreen itself stay put through its own
      // requestOtp-triggered resends), so without this explicit
      // navigation a successful OTP request would silently leave the user
      // stuck on this screen. Found via Task 9's live smoke test against
      // the real backend — no widget/unit test caught it because they
      // exercise this screen and the router guard in isolation, never
      // together with a real requestOtp state transition.
      if (mounted && ref.read(authControllerProvider) is AuthOtpRequested) {
        context.go('/login/verify');
      }
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final authState = ref.watch(authControllerProvider);
    final serverError = authState is AuthError ? authState.failure.message : null;

    return Scaffold(
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 420),
              child: Padding(
                padding: const EdgeInsets.all(AppSpacing.lg),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Text(
                      l10n.appTitle,
                      textAlign: TextAlign.center,
                      style: theme.textTheme.titleLarge?.copyWith(
                        color: scheme.primary,
                        letterSpacing: 0.5,
                      ),
                    ),
                    const SizedBox(height: AppSpacing.lg),
                    const _TaglineBadge(),
                    const SizedBox(height: AppSpacing.md),
                    Text(
                      l10n.authHeadline,
                      textAlign: TextAlign.center,
                      style: theme.textTheme.headlineMedium,
                    ),
                    const SizedBox(height: AppSpacing.sm),
                    Text(
                      l10n.phoneEntrySubtitle,
                      textAlign: TextAlign.center,
                      style: theme.textTheme.bodyMedium?.copyWith(
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                    const SizedBox(height: AppSpacing.xl),
                    TextField(
                      key: const Key('phoneField'),
                      controller: _phoneController,
                      enabled: !_submitting,
                      keyboardType: TextInputType.phone,
                      decoration: InputDecoration(
                        labelText: l10n.phoneLabel,
                        prefixIcon: const Icon(Icons.phone_rounded),
                        errorText: _formError,
                      ),
                      onSubmitted: (_) => _submit(),
                    ),
                    if (serverError != null) ...[
                      const SizedBox(height: AppSpacing.sm),
                      Text(
                        serverError,
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: scheme.error,
                        ),
                      ),
                    ],
                    const SizedBox(height: AppSpacing.lg),
                    _SlabButton(
                      buttonKey: const Key('phoneSubmitButton'),
                      onPressed: _submitting ? null : _submit,
                      loading: _submitting,
                      label: l10n.continueButton,
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// Small star badge above the hero headline (matches the reference screens'
/// "⭐ tagline" pill).
class _TaglineBadge extends StatelessWidget {
  const _TaglineBadge();

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    return Align(
      alignment: Alignment.center,
      child: Container(
        padding: const EdgeInsets.symmetric(
          horizontal: AppSpacing.md,
          vertical: AppSpacing.sm,
        ),
        decoration: BoxDecoration(
          color: scheme.primaryContainer,
          borderRadius: BorderRadius.circular(AppRadius.chip),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.star_rounded, size: 18, color: scheme.primary),
            const SizedBox(width: AppSpacing.xs),
            Flexible(
              child: Text(
                AppLocalizations.of(context)!.authTagline,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: theme.textTheme.labelMedium?.copyWith(
                  color: scheme.onPrimaryContainer,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// A pill [ElevatedButton] with the reference "3D press slab" shadow beneath
/// it. Kept as a real [ElevatedButton] child (carrying [buttonKey]) so
/// enabled/disabled semantics and the existing structural tests are unchanged.
class _SlabButton extends StatelessWidget {
  const _SlabButton({
    required this.buttonKey,
    required this.onPressed,
    required this.label,
    this.loading = false,
  });

  final Key buttonKey;
  final VoidCallback? onPressed;
  final String label;
  final bool loading;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final enabled = onPressed != null && !loading;
    final hsl = HSLColor.fromColor(scheme.primary);
    final slabColor = enabled
        ? hsl.withLightness((hsl.lightness - 0.18).clamp(0.0, 1.0)).toColor()
        : scheme.outlineVariant;
    return DecoratedBox(
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(999),
        boxShadow: [BoxShadow(color: slabColor, offset: const Offset(0, 4))],
      ),
      child: ElevatedButton(
        key: buttonKey,
        onPressed: loading ? null : onPressed,
        child: loading
            ? const SizedBox(
                height: 18,
                width: 18,
                child: CircularProgressIndicator(strokeWidth: 2),
              )
            : Text(label),
      ),
    );
  }
}
