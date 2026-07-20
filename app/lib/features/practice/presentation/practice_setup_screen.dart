import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../app/l10n/app_localizations.dart';
import '../../../app/locale/locale_provider.dart';
import '../../../app/theme/app_theme.dart';
import '../../../core/result.dart';
import '../../content/data/content_api.dart';
import '../../content/domain/category.dart';
import '../../content/domain/sign.dart';
import '../../session/presentation/session_controller.dart';

/// Which of category/sign the user is currently configuring `practice` by.
/// `null` means neither has been chosen yet.
enum _PracticeTarget { category, sign }

/// Categories to pick from for the "by category" practice target. A plain
/// one-shot [FutureProvider] (not a dedicated controller — this task's file
/// list only calls for `practice_setup_screen.dart`, no separate practice
/// controller) since it's a read-only list, refetched only if invalidated.
final _practiceCategoriesProvider = FutureProvider.autoDispose<List<Category>>(
  (ref) async {
    final api = ref.read(contentApiProvider);
    final locale = localeToBackendCode(ref.read(localeProvider));
    final result = await api.categories(locale: locale);
    return switch (result) {
      Ok(:final data) => data,
      Err(:final failure) => throw Exception(failure.message),
    };
  },
);

/// Signs to pick from for the "by sign" practice target.
final _practiceSignsProvider = FutureProvider.autoDispose<List<Sign>>((
  ref,
) async {
  final api = ref.read(contentApiProvider);
  final locale = localeToBackendCode(ref.read(localeProvider));
  final result = await api.signs(locale: locale);
  return switch (result) {
    Ok(:final data) => data,
    Err(:final failure) => throw Exception(failure.message),
  };
});

/// Lets the user configure a `mode: 'practice'` session: pick EXACTLY ONE of
/// a category or a sign (the backend's "aynan bittasi" — exactly one —
/// constraint; README: "`category_id` yoki `sign_id`dan biri (aynan
/// bittasi)") plus a question count, then navigates to `/session` with a
/// [SessionStartRequest].
///
/// The "exactly one" rule is enforced client-side by construction, not just
/// checked at submit time: picking a target (category vs. sign) immediately
/// clears whatever was picked for the other one (see [_setTarget]), so it is
/// impossible for both to be non-null when the start button is enabled.
///
/// Formerly-known contract gap, now fixed backend-side (same shape as
/// `variants_screen.dart`'s bilet-number-vs-UUID note, which is still open):
/// `POST /sessions`' `category_id`/`sign_id` used to be UUID-only
/// server-side (`backend/internal/session/handlers.go`), but the content
/// API's [Category]/[Sign] models only ever expose a `code` (no UUID) — a
/// real backend round-trip used to 400 (`invalid_body`, a UUID parse
/// failure). The backend now resolves either a UUID or a `code` in these
/// same two fields (`Service.ResolveCategoryID`/`ResolveSignID`, 404
/// `not_found` for an unrecognized code), so this screen's existing
/// `category.code`/`sign.code` passthrough as `categoryId`/`signId` already
/// works end-to-end with no client-side change needed.
///
/// On `daily_limit_reached` (429) — expected/normal for free-tier users
/// hitting their daily cap, not a bug (D13) — this screen does NOT
/// special-case the error itself: it just navigates to `/session`, and the
/// real `SessionScreen`/`SessionController` reached there already renders
/// the distinct, non-alarming copy for that code (`session_screen.dart`'s
/// `_ErrorView`) — the same "the real screen already handles it" seam
/// `MistakesScreen` relies on for `vip_required`.
class PracticeSetupScreen extends ConsumerStatefulWidget {
  const PracticeSetupScreen({super.key});

  @override
  ConsumerState<PracticeSetupScreen> createState() =>
      _PracticeSetupScreenState();
}

class _PracticeSetupScreenState extends ConsumerState<PracticeSetupScreen> {
  _PracticeTarget? _target;
  String? _categoryCode;
  String? _signCode;
  final _countController = TextEditingController(text: '10');

  int? get _count => int.tryParse(_countController.text);

  bool get _canStart {
    final hasCategory =
        _target == _PracticeTarget.category && _categoryCode != null;
    final hasSign = _target == _PracticeTarget.sign && _signCode != null;
    final exactlyOne = hasCategory != hasSign; // XOR: exactly one is true.
    final count = _count;
    return exactlyOne && count != null && count > 0;
  }

  void _setTarget(Set<_PracticeTarget> selected) {
    setState(() {
      _target = selected.isEmpty ? null : selected.first;
      // Selecting one target clears the other's pick — makes "exactly one"
      // impossible to violate by construction, rather than only checked at
      // submit time.
      if (_target != _PracticeTarget.category) _categoryCode = null;
      if (_target != _PracticeTarget.sign) _signCode = null;
    });
  }

  void _start(BuildContext context) {
    if (!_canStart) return;
    context.push(
      '/session',
      extra: SessionStartRequest(
        mode: 'practice',
        categoryId: _target == _PracticeTarget.category
            ? _categoryCode
            : null,
        signId: _target == _PracticeTarget.sign ? _signCode : null,
        count: _count,
      ),
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
    final categoriesAsync = ref.watch(_practiceCategoriesProvider);
    final signsAsync = ref.watch(_practiceSignsProvider);

    return Scaffold(
      appBar: AppBar(
        title: Text(l10n.practiceSetupTitle),
        actions: [
          IconButton(
            key: const Key('open-signs-catalog'),
            icon: const Icon(Icons.info_outline),
            tooltip: l10n.signsScreenTitle,
            onPressed: () => context.push('/signs'),
          ),
        ],
      ),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(AppSpacing.lg),
          children: [
            Text(l10n.practiceSetupDescription),
            const SizedBox(height: AppSpacing.md),
            SegmentedButton<_PracticeTarget>(
              key: const Key('practice-target-selector'),
              segments: [
                ButtonSegment(
                  value: _PracticeTarget.category,
                  label: Text(l10n.practiceTargetCategory),
                ),
                ButtonSegment(
                  value: _PracticeTarget.sign,
                  label: Text(l10n.practiceTargetSign),
                ),
              ],
              selected: _target == null ? const {} : {_target!},
              emptySelectionAllowed: true,
              onSelectionChanged: _setTarget,
            ),
            const SizedBox(height: AppSpacing.md),
            if (_target == _PracticeTarget.category)
              categoriesAsync.when(
                loading: () => const LinearProgressIndicator(),
                error: (error, stackTrace) =>
                    Text(l10n.practiceLoadCategoriesError),
                data: (categories) => DropdownButton<String>(
                  key: const Key('practice-category-dropdown'),
                  isExpanded: true,
                  hint: Text(l10n.practiceSelectCategory),
                  value: _categoryCode,
                  items: [
                    for (final category in categories)
                      DropdownMenuItem(
                        value: category.code,
                        child: Text(category.name),
                      ),
                  ],
                  onChanged: (value) => setState(() => _categoryCode = value),
                ),
              ),
            if (_target == _PracticeTarget.sign)
              signsAsync.when(
                loading: () => const LinearProgressIndicator(),
                error: (error, stackTrace) =>
                    Text(l10n.practiceLoadSignsError),
                data: (signs) => DropdownButton<String>(
                  key: const Key('practice-sign-dropdown'),
                  isExpanded: true,
                  hint: Text(l10n.practiceSelectSign),
                  value: _signCode,
                  items: [
                    for (final sign in signs)
                      DropdownMenuItem(
                        value: sign.code,
                        child: Text(sign.name),
                      ),
                  ],
                  onChanged: (value) => setState(() => _signCode = value),
                ),
              ),
            const SizedBox(height: AppSpacing.lg),
            TextField(
              key: const Key('practice-count-field'),
              controller: _countController,
              keyboardType: TextInputType.number,
              decoration: InputDecoration(labelText: l10n.questionCountLabel),
              onChanged: (_) => setState(() {}),
            ),
            const SizedBox(height: AppSpacing.lg),
            FilledButton(
              key: const Key('practice-start-button'),
              onPressed: _canStart ? () => _start(context) : null,
              child: Text(l10n.startButton),
            ),
          ],
        ),
      ),
    );
  }
}
