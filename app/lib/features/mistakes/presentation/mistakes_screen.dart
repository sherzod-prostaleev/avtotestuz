import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../../app/l10n/app_localizations.dart';
import '../../../app/theme/app_theme.dart';
import '../../session/presentation/session_controller.dart';

/// Entry point into `mode: 'mistakes'` — the personal bank of previously
/// missed questions the FSRS scheduler currently has due for review
/// (README's "Xatolar banki" section). Just a count picker (default 10,
/// matching the backend's own default) plus a start button.
///
/// This screen never calls `SessionApi.start` itself — same "the controller
/// owns `start()`" design every other session-entry screen follows (see
/// `SessionController`'s doc comment): it only builds a
/// [SessionStartRequest] and navigates to `/session`.
///
/// Non-VIP gating: `mistakes` is one of the three modes `POST /sessions` can
/// reject with `vip_required` (402) (README: bilet #2+, `exam`, `mistakes`).
/// This screen deliberately does NOT special-case that error itself — the
/// real `SessionScreen`/`SessionController` reached at `/session` already
/// renders the distinct upsell copy for `vip_required`
/// (`session_screen.dart`'s `_ErrorView`, reviewed clean in Task 4), so
/// routing through the real screen rather than re-implementing error
/// handling here means the same seam Task 5's `VariantsScreen` relies on for
/// bilet #2+ "just works" here too.
class MistakesScreen extends StatefulWidget {
  const MistakesScreen({super.key});

  @override
  State<MistakesScreen> createState() => _MistakesScreenState();
}

class _MistakesScreenState extends State<MistakesScreen> {
  static const _defaultCount = 10;
  final _countController = TextEditingController(text: '$_defaultCount');

  int? get _count => int.tryParse(_countController.text);

  bool get _canStart {
    final count = _count;
    return count != null && count > 0;
  }

  void _start(BuildContext context) {
    final count = _count;
    if (count == null || count <= 0) return;
    context.push(
      '/session',
      extra: SessionStartRequest(mode: 'mistakes', count: count),
    );
  }

  @override
  void dispose() {
    _countController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    return Scaffold(
      appBar: AppBar(title: Text(l10n.mistakesScreenTitle)),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(AppSpacing.lg),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(l10n.mistakesScreenDescription),
              const SizedBox(height: AppSpacing.lg),
              TextField(
                key: const Key('mistakes-count-field'),
                controller: _countController,
                keyboardType: TextInputType.number,
                decoration: InputDecoration(labelText: l10n.questionCountLabel),
                onChanged: (_) => setState(() {}),
              ),
              const SizedBox(height: AppSpacing.lg),
              FilledButton(
                key: const Key('mistakes-start-button'),
                onPressed: _canStart ? () => _start(context) : null,
                child: Text(l10n.startButton),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
