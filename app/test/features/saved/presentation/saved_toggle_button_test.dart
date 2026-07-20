import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/saved/data/saved_api.dart';
import 'package:avtotest_app/features/saved/domain/saved_entry.dart';
import 'package:avtotest_app/features/saved/presentation/saved_controller.dart';
import 'package:avtotest_app/features/saved/presentation/saved_toggle_button.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

/// A real-behaviour [SavedApi] fake (not a mock) — `list()` returns whatever
/// [initiallySaved] the test seeded, `save`/`unsave` actually mutate that set
/// so the seam test below exercises the REAL [SavedController] against
/// genuine save/unsave semantics, not a canned single response. This is the
/// seam this project's biggest recurring lesson calls for: a screen+
/// controller pair tested together, not each in isolation.
class _FakeSavedApi implements SavedApi {
  _FakeSavedApi({Set<String> initiallySaved = const {}})
      : _saved = {...initiallySaved};

  final Set<String> _saved;
  final List<String> saveCalls = [];
  final List<String> unsaveCalls = [];

  @override
  Future<Result<List<SavedEntry>>> list() async {
    return Result.ok([
      for (final id in _saved)
        SavedEntry(questionId: id, createdAt: DateTime(2026, 1, 1)),
    ]);
  }

  @override
  Future<Result<void>> save(String questionId) async {
    saveCalls.add(questionId);
    _saved.add(questionId);
    return const Result.ok(null);
  }

  @override
  Future<Result<void>> unsave(String questionId) async {
    unsaveCalls.add(questionId);
    _saved.remove(questionId);
    return const Result.ok(null);
  }
}

Widget _wrap(SavedApi api, String questionId) {
  return ProviderScope(
    overrides: [savedApiProvider.overrideWithValue(api)],
    child: MaterialApp(
      home: Scaffold(body: SavedToggleButton(questionId: questionId)),
    ),
  );
}

void main() {
  testWidgets(
    'tapping an unsaved question calls save() and flips the icon to filled',
    (tester) async {
      final api = _FakeSavedApi();
      await tester.pumpWidget(_wrap(api, 'q1'));
      await tester.pumpAndSettle();

      expect(find.byIcon(Icons.bookmark_border), findsOneWidget);
      expect(find.byIcon(Icons.bookmark), findsNothing);

      await tester.tap(find.byKey(const Key('saved-toggle-q1')));
      await tester.pumpAndSettle();

      expect(api.saveCalls, ['q1']);
      expect(api.unsaveCalls, isEmpty);
      expect(find.byIcon(Icons.bookmark), findsOneWidget);
      expect(find.byIcon(Icons.bookmark_border), findsNothing);
    },
  );

  testWidgets(
    'tapping an already-saved question calls unsave() and flips the icon '
    'back to outline',
    (tester) async {
      final api = _FakeSavedApi(initiallySaved: {'q1'});
      await tester.pumpWidget(_wrap(api, 'q1'));
      await tester.pumpAndSettle();

      expect(find.byIcon(Icons.bookmark), findsOneWidget);

      await tester.tap(find.byKey(const Key('saved-toggle-q1')));
      await tester.pumpAndSettle();

      expect(api.unsaveCalls, ['q1']);
      expect(api.saveCalls, isEmpty);
      expect(find.byIcon(Icons.bookmark_border), findsOneWidget);
      expect(find.byIcon(Icons.bookmark), findsNothing);
    },
  );

  testWidgets(
    'reflects a different question\'s already-saved state independently',
    (tester) async {
      final api = _FakeSavedApi(initiallySaved: {'other-question'});
      await tester.pumpWidget(_wrap(api, 'q1'));
      await tester.pumpAndSettle();

      // q1 itself isn't in the saved set, even though something else is.
      expect(find.byIcon(Icons.bookmark_border), findsOneWidget);
    },
  );

  testWidgets(
    'a failed toggle leaves the icon unchanged and shows a snackbar',
    (tester) async {
      final api = _FailingSavedApi();
      await tester.pumpWidget(_wrap(api, 'q1'));
      await tester.pumpAndSettle();

      expect(find.byIcon(Icons.bookmark_border), findsOneWidget);

      await tester.tap(find.byKey(const Key('saved-toggle-q1')));
      await tester.pumpAndSettle();

      // Still unsaved — the failed call never flipped the icon.
      expect(find.byIcon(Icons.bookmark_border), findsOneWidget);
      expect(find.text('boom'), findsOneWidget);
    },
  );
}

class _FailingSavedApi implements SavedApi {
  @override
  Future<Result<List<SavedEntry>>> list() async => const Result.ok([]);

  @override
  Future<Result<void>> save(String questionId) async =>
      const Result.err(Failure(code: 'network_error', message: 'boom'));

  @override
  Future<Result<void>> unsave(String questionId) async =>
      const Result.err(Failure(code: 'network_error', message: 'boom'));
}
