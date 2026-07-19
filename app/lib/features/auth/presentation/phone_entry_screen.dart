import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../app/l10n/app_localizations.dart';
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
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final authState = ref.watch(authControllerProvider);
    final serverError = authState is AuthError ? authState.failure.message : null;

    return Scaffold(
      body: SafeArea(
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 420),
            child: Padding(
              padding: const EdgeInsets.all(24),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Text(
                    l10n.appTitle,
                    textAlign: TextAlign.center,
                    style: Theme.of(context).textTheme.headlineMedium,
                  ),
                  const SizedBox(height: 32),
                  TextField(
                    key: const Key('phoneField'),
                    controller: _phoneController,
                    enabled: !_submitting,
                    keyboardType: TextInputType.phone,
                    decoration: InputDecoration(
                      labelText: l10n.phoneLabel,
                      errorText: _formError,
                    ),
                    onSubmitted: (_) => _submit(),
                  ),
                  if (serverError != null) ...[
                    const SizedBox(height: 12),
                    Text(
                      serverError,
                      style: TextStyle(
                        color: Theme.of(context).colorScheme.error,
                      ),
                    ),
                  ],
                  const SizedBox(height: 24),
                  ElevatedButton(
                    key: const Key('phoneSubmitButton'),
                    onPressed: _submitting ? null : _submit,
                    child: _submitting
                        ? const SizedBox(
                            height: 18,
                            width: 18,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : Text(l10n.continueButton),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
