import 'package:avtotest_app/app/l10n/app_localizations.dart';
import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/content/data/content_api.dart';
import 'package:avtotest_app/features/content/domain/category.dart';
import 'package:avtotest_app/features/content/domain/question.dart';
import 'package:avtotest_app/features/content/domain/sign.dart';
import 'package:avtotest_app/features/content/domain/variant.dart';
import 'package:avtotest_app/features/saved/data/saved_api.dart';
import 'package:avtotest_app/features/saved/domain/saved_entry.dart';
import 'package:avtotest_app/features/saved/presentation/saved_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

class _FakeSavedApi implements SavedApi {
  _FakeSavedApi(this.result);

  final Result<List<SavedEntry>> result;

  @override
  Future<Result<List<SavedEntry>>> list() async => result;

  @override
  Future<Result<void>> save(String questionId) async => const Result.ok(null);

  @override
  Future<Result<void>> unsave(String questionId) async =>
      const Result.ok(null);
}

/// A minimal [ContentApi] fake serving `question()` from a fixed map — the
/// only method [SavedScreen]'s per-item `savedQuestionProvider` calls.
class _FakeContentApi implements ContentApi {
  _FakeContentApi(this.questions);

  final Map<String, Question> questions;

  @override
  Future<Result<Question>> question(String id, {required String locale}) async {
    final question = questions[id];
    if (question == null) {
      return const Result.err(Failure(code: 'not_found', message: 'missing'));
    }
    return Result.ok(question);
  }

  @override
  Future<Result<List<Category>>> categories({required String locale}) =>
      throw UnimplementedError();

  @override
  Future<Result<Sign>> signByCode(String code, {required String locale}) =>
      throw UnimplementedError();

  @override
  Future<Result<List<Sign>>> signs({
    String? group,
    String? query,
    required String locale,
  }) => throw UnimplementedError();

  @override
  Future<Result<Variant>> variantByNumber(int n, {required String locale}) =>
      throw UnimplementedError();

  @override
  Future<Result<List<Variant>>> variants() => throw UnimplementedError();
}

Widget _wrap(SavedApi savedApi, ContentApi contentApi) {
  return ProviderScope(
    overrides: [
      savedApiProvider.overrideWithValue(savedApi),
      contentApiProvider.overrideWithValue(contentApi),
    ],
    child: MaterialApp(
      locale: const Locale('uz'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: const SavedScreen(),
    ),
  );
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  const questionA = Question(
    id: 'q1',
    categoryCode: 'cat1',
    text: 'Savol matni 1',
    answers: [],
  );
  const questionB = Question(
    id: 'q2',
    categoryCode: 'cat1',
    text: 'Savol matni 2',
    answers: [],
  );

  testWidgets('renders every saved question\'s content', (tester) async {
    final savedApi = _FakeSavedApi(
      Result.ok([
        SavedEntry(questionId: 'q1', createdAt: DateTime(2026, 1, 1)),
        SavedEntry(questionId: 'q2', createdAt: DateTime(2026, 1, 2)),
      ]),
    );
    final contentApi = _FakeContentApi({'q1': questionA, 'q2': questionB});

    await tester.pumpWidget(_wrap(savedApi, contentApi));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('saved-list')), findsOneWidget);
    expect(find.text('Savol matni 1'), findsOneWidget);
    expect(find.text('Savol matni 2'), findsOneWidget);
  });

  testWidgets('shows an empty state when nothing is saved', (tester) async {
    final savedApi = _FakeSavedApi(const Result.ok([]));
    final contentApi = _FakeContentApi(const {});

    await tester.pumpWidget(_wrap(savedApi, contentApi));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('saved-empty-state')), findsOneWidget);
    expect(find.byKey(const Key('saved-list')), findsNothing);
  });

  testWidgets('a failed fetch shows an error state with retry', (
    tester,
  ) async {
    final savedApi = _FakeSavedApi(
      const Result.err(Failure(code: 'unknown', message: 'boom')),
    );
    final contentApi = _FakeContentApi(const {});

    await tester.pumpWidget(_wrap(savedApi, contentApi));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('saved-error-state')), findsOneWidget);
    expect(find.text('boom'), findsOneWidget);
  });

  testWidgets('each rendered question shows its save-toggle button', (
    tester,
  ) async {
    final savedApi = _FakeSavedApi(
      Result.ok([SavedEntry(questionId: 'q1', createdAt: DateTime(2026, 1, 1))]),
    );
    final contentApi = _FakeContentApi({'q1': questionA});

    await tester.pumpWidget(_wrap(savedApi, contentApi));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('saved-toggle-q1')), findsOneWidget);
  });
}
