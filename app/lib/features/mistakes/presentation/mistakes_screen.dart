import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../../app/l10n/app_localizations.dart';
import '../../../app/theme/app_theme.dart';
import '../../../shared/widgets/app_card.dart';
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
/// real `SessionScreen`/`SessionController` reached at `/session` routes a
/// `vip_required` start to the shared paywall-style `VipRequiredScreen`
/// (Task 9, via `session_screen.dart`'s error handling), so routing through
/// the real screen rather than re-implementing error handling here means the
/// same seam Task 5's `VariantsScreen` relies on for bilet #2+ "just works"
/// here too.
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

  /// Nudges the count field by [delta], clamped to a sane `1..999` range —
  /// a convenience ADDITION alongside the existing `TextField`, writing
  /// through the same [_countController] the field already owns so its
  /// `Key`/behavior is untouched.
  void _adjustCount(int delta) {
    final next = ((_count ?? 0) + delta).clamp(1, 999);
    setState(() {
      _countController.text = '$next';
      _countController.selection = TextSelection.collapsed(
        offset: _countController.text.length,
      );
    });
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
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: Text(l10n.mistakesScreenTitle)),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(AppSpacing.lg),
          children: [
            // Hero moment: a big icon + headline explaining what "mistakes
            // mode" actually does before the plain count-picker below.
            Container(
              width: 88,
              height: 88,
              decoration: BoxDecoration(
                color: theme.colorScheme.primaryContainer,
                shape: BoxShape.circle,
              ),
              child: Icon(
                Icons.replay_rounded,
                size: 44,
                color: theme.colorScheme.onPrimaryContainer,
              ),
            ),
            const SizedBox(height: AppSpacing.lg),
            Text(
              l10n.mistakesScreenTitle,
              style: theme.textTheme.headlineSmall,
            ),
            const SizedBox(height: AppSpacing.sm),
            Text(
              l10n.mistakesScreenDescription,
              style: theme.textTheme.bodyLarge,
            ),
            const SizedBox(height: AppSpacing.xl),
            AppCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    l10n.questionCountLabel,
                    style: theme.textTheme.titleSmall,
                  ),
                  const SizedBox(height: AppSpacing.sm),
                  Row(
                    children: [
                      IconButton(
                        onPressed: () => _adjustCount(-1),
                        icon: const Icon(Icons.remove_rounded),
                        tooltip: '-1',
                      ),
                      const SizedBox(width: AppSpacing.sm),
                      Expanded(
                        child: TextField(
                          key: const Key('mistakes-count-field'),
                          controller: _countController,
                          keyboardType: TextInputType.number,
                          textAlign: TextAlign.center,
                          style: theme.textTheme.titleLarge,
                          decoration: const InputDecoration(isDense: true),
                          onChanged: (_) => setState(() {}),
                        ),
                      ),
                      const SizedBox(width: AppSpacing.sm),
                      IconButton(
                        onPressed: () => _adjustCount(1),
                        icon: const Icon(Icons.add_rounded),
                        tooltip: '+1',
                      ),
                    ],
                  ),
                ],
              ),
            ),
            const SizedBox(height: AppSpacing.lg),
            SizedBox(
              width: double.infinity,
              child: FilledButton.icon(
                key: const Key('mistakes-start-button'),
                onPressed: _canStart ? () => _start(context) : null,
                icon: const Icon(Icons.play_arrow_rounded),
                label: Text(l10n.startButton),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
