import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

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
/// Known contract gap (same shape as `variants_screen.dart`'s bilet-number-
/// vs-UUID note): `POST /sessions`' `category_id`/`sign_id` are UUIDs
/// server-side (`backend/internal/session/handlers.go`), but the content
/// API's [Category]/[Sign] models only expose a `code` (no UUID) — this
/// screen passes `category.code`/`sign.code` as `categoryId`/`signId`,
/// matching the literal interface this plan specifies, but a real backend
/// round-trip will 400 (`invalid_body`, a UUID parse failure) until the
/// backend either accepts a code or content exposes the UUID somewhere the
/// client can read it.
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
    final categoriesAsync = ref.watch(_practiceCategoriesProvider);
    final signsAsync = ref.watch(_practiceSignsProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('Mashq sozlamalari')),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(AppSpacing.lg),
          children: [
            const Text(
              'Mashq qilish uchun kategoriya YOKI belgini tanlang '
              '(ikkalasi emas, faqat bittasi).',
            ),
            const SizedBox(height: AppSpacing.md),
            SegmentedButton<_PracticeTarget>(
              key: const Key('practice-target-selector'),
              segments: const [
                ButtonSegment(
                  value: _PracticeTarget.category,
                  label: Text('Kategoriya'),
                ),
                ButtonSegment(
                  value: _PracticeTarget.sign,
                  label: Text('Belgi'),
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
                    const Text('Kategoriyalarni yuklab bo\'lmadi.'),
                data: (categories) => DropdownButton<String>(
                  key: const Key('practice-category-dropdown'),
                  isExpanded: true,
                  hint: const Text('Kategoriyani tanlang'),
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
                    const Text('Belgilarni yuklab bo\'lmadi.'),
                data: (signs) => DropdownButton<String>(
                  key: const Key('practice-sign-dropdown'),
                  isExpanded: true,
                  hint: const Text('Belgini tanlang'),
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
              decoration: const InputDecoration(labelText: 'Savollar soni'),
              onChanged: (_) => setState(() {}),
            ),
            const SizedBox(height: AppSpacing.lg),
            FilledButton(
              key: const Key('practice-start-button'),
              onPressed: _canStart ? () => _start(context) : null,
              child: const Text('Boshlash'),
            ),
          ],
        ),
      ),
    );
  }
}
