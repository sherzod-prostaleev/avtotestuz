import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../app/l10n/app_localizations.dart';
import '../../../app/theme/app_theme.dart';
import '../../../shared/widgets/primary_button.dart';
import '../domain/auth_state.dart';
import 'auth_controller.dart';

const _resendCooldownSeconds = 30;

/// Second screen of the phone+OTP auth flow: collects the 6-digit code and
/// submits it via [AuthController.verifyOtp]. Reachable at `/login/verify`
/// (`app/lib/app/router.dart`) — meaningfully reachable only once
/// `authControllerProvider`'s state is [AuthOtpRequested] (set by
/// [PhoneEntryScreen] after a successful `requestOtp`); if a user somehow
/// lands here without that (e.g. a stale deep link), this screen bounces
/// back to `/login` rather than showing a dead-end phone-less form.
class OtpVerifyScreen extends ConsumerStatefulWidget {
  const OtpVerifyScreen({super.key});

  @override
  ConsumerState<OtpVerifyScreen> createState() => _OtpVerifyScreenState();
}

class _OtpVerifyScreenState extends ConsumerState<OtpVerifyScreen> {
  final _codeController = TextEditingController();
  bool _submitting = false;
  Timer? _cooldownTimer;
  int _cooldownRemaining = 0;

  /// Cached from the last [AuthOtpRequested] state seen. `AuthController`
  /// (Task 6) moves state to [AuthError] on a failed `verifyOtp`/`requestOtp`
  /// call, which — being a *different* union case — no longer carries the
  /// phone/debug-code the form needs to keep functioning (retry, resend,
  /// header text). Caching them locally lets this screen keep showing a
  /// working form with the error surfaced alongside it, instead of bouncing
  /// to `/login` the moment an error arrives.
  String? _phone;
  String? _debugCode;

  @override
  void dispose() {
    _codeController.dispose();
    _cooldownTimer?.cancel();
    super.dispose();
  }

  void _startCooldown() {
    _cooldownTimer?.cancel();
    setState(() => _cooldownRemaining = _resendCooldownSeconds);
    _cooldownTimer = Timer.periodic(const Duration(seconds: 1), (timer) {
      if (!mounted) {
        timer.cancel();
        return;
      }
      setState(() {
        _cooldownRemaining -= 1;
        if (_cooldownRemaining <= 0) {
          timer.cancel();
        }
      });
    });
  }

  Future<void> _submit() async {
    final code = _codeController.text.trim();
    setState(() => _submitting = true);
    try {
      await ref.read(authControllerProvider.notifier).verifyOtp(code);
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  Future<void> _resend(String phone) async {
    setState(() => _submitting = true);
    try {
      await ref.read(authControllerProvider.notifier).requestOtp(phone);
    } finally {
      if (mounted) {
        setState(() => _submitting = false);
        _startCooldown();
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final authState = ref.watch(authControllerProvider);

    if (authState is AuthOtpRequested) {
      _phone = authState.phone;
      _debugCode = authState.debugCode;
    }

    final phone = _phone;
    if (phone == null) {
      // Never had a phone context on this screen (e.g. a stale deep link
      // straight to /login/verify without ever calling requestOtp, or the
      // flow was reset elsewhere). Bounce back to /login rather than
      // rendering a dead-end form. Scheduled for after this frame since
      // navigating during build isn't safe.
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) context.go('/login');
      });
      return const Scaffold(body: SizedBox.shrink());
    }

    final debugCode = _debugCode;
    final serverErrorMessage =
        authState is AuthError ? authState.failure.message : null;

    final theme = Theme.of(context);
    final scheme = theme.colorScheme;

    return Scaffold(
      appBar: AppBar(
        leading: BackButton(onPressed: () => context.go('/login')),
      ),
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
                    Container(
                      width: 64,
                      height: 64,
                      alignment: Alignment.center,
                      decoration: BoxDecoration(
                        color: scheme.primaryContainer,
                        shape: BoxShape.circle,
                      ),
                      child: Icon(
                        Icons.sms_rounded,
                        color: scheme.onPrimaryContainer,
                        size: 30,
                      ),
                    ),
                    const SizedBox(height: AppSpacing.md),
                    Text(
                      l10n.otpHeadline,
                      textAlign: TextAlign.center,
                      style: theme.textTheme.headlineMedium,
                    ),
                    const SizedBox(height: AppSpacing.sm),
                    Text(
                      l10n.phoneConfirmationLabel(phone),
                      textAlign: TextAlign.center,
                      style: theme.textTheme.bodyMedium?.copyWith(
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                    if (kDebugMode && debugCode != null) ...[
                      const SizedBox(height: AppSpacing.sm),
                      Text(
                        l10n.devCodeCaption(debugCode),
                        key: const Key('devCodeCaption'),
                        textAlign: TextAlign.center,
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: scheme.secondary,
                        ),
                      ),
                    ],
                    const SizedBox(height: AppSpacing.xl),
                    TextField(
                      key: const Key('otpField'),
                      controller: _codeController,
                      enabled: !_submitting,
                      keyboardType: TextInputType.number,
                      maxLength: 6,
                      textAlign: TextAlign.center,
                      style: theme.textTheme.headlineSmall?.copyWith(
                        letterSpacing: 8,
                      ),
                      inputFormatters: [FilteringTextInputFormatter.digitsOnly],
                      decoration: InputDecoration(
                        labelText: l10n.otpLabel,
                        counterText: '',
                      ),
                      onSubmitted: (_) => _submit(),
                    ),
                    if (serverErrorMessage != null) ...[
                      const SizedBox(height: AppSpacing.sm),
                      Text(
                        serverErrorMessage,
                        textAlign: TextAlign.center,
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: scheme.error,
                        ),
                      ),
                    ],
                    const SizedBox(height: AppSpacing.lg),
                    PrimaryButton(
                      key: const Key('verifySubmitButton'),
                      label: l10n.verifyButton,
                      loading: _submitting,
                      onPressed: _submitting ? null : () => _submit(),
                    ),
                    const SizedBox(height: AppSpacing.sm),
                    TextButton(
                      key: const Key('resendButton'),
                      onPressed: (_submitting || _cooldownRemaining > 0)
                          ? null
                          : () => _resend(phone),
                      child: Text(
                        _cooldownRemaining > 0
                            ? l10n.resendIn(_cooldownRemaining)
                            : l10n.resendButton,
                      ),
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
