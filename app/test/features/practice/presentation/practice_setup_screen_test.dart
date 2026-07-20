import 'package:avtotest_app/app/l10n/app_localizations.dart';
import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/content/data/content_api.dart';
import 'package:avtotest_app/features/content/domain/category.dart';
import 'package:avtotest_app/features/content/domain/question.dart';
import 'package:avtotest_app/features/content/domain/sign.dart';
import 'package:avtotest_app/features/content/domain/variant.dart';
import 'package:avtotest_app/features/practice/presentation/practice_setup_screen.dart';
import 'package:avtotest_app/features/session/data/session_api.dart';
import 'package:avtotest_app/features/session/domain/session_models.dart';
import 'package:avtotest_app/features/session/presentation/session_controller.dart';
import 'package:avtotest_app/features/session/presentation/session_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../session/presentation/session_fakes.dart' show fakeQuestion, fakeSummary;

/// A local [SessionApi] fake capturing every `start()` call's full argument
/// set (`categoryId`/`signId`/`count`), needed to assert the "exactly one of
/// category/sign" rule actually reaches the network boundary correctly —
/// kept self-contained here rather than editing `session_fakes.dart`'s
/// `FakeSessionApi` (owned by Task 4), whose recorded-call tuple doesn't
/// carry those fields.
class _FakeSessionApi implements SessionApi {
  _FakeSessionApi({this.startResult, this.startFailure});

  final SessionSummary? startResult;
  final Failure? startFailure;
  final List<({String mode, String? categoryId, String? signId, int? count})>
  startCalls = [];

  @override
  Future<Result<SessionSummary>> start({
    required String mode,
    String? variantId,
    String? categoryId,
    String? signId,
    required String locale,
    int? count,
  }) async {
    startCalls.add((
      mode: mode,
      categoryId: categoryId,
      signId: signId,
      count: count,
    ));
    final failure = startFailure;
    if (failure != null) return Result.err(failure);
    return Result.ok(startResult!);
  }

  @override
  Future<Result<AnswerResult>> answer({
    required String sessionId,
    required String questionId,
    required String answerId,
  }) => throw UnimplementedError();

  @override
  Future<Result<SessionResult>> finish(String sessionId) =>
      throw UnimplementedError();

  @override
  Future<Result<SessionDetail>> get(String sessionId) =>
      throw UnimplementedError();

  @override
  Future<Result<List<VariantStatus>>> myVariants() =>
      throw UnimplementedError();
}

/// A local [ContentApi] fake supplying fixed categories/signs lists and
/// (for the successful-navigation case) a fake question.
class _FakeContentApi implements ContentApi {
  _FakeContentApi({this.categoriesResult = const [], this.signsResult = const []});

  final List<Category> categoriesResult;
  final List<Sign> signsResult;

  @override
  Future<Result<List<Category>>> categories({required String locale}) async =>
      Result.ok(categoriesResult);

  @override
  Future<Result<List<Sign>>> signs({
    String? group,
    String? query,
    required String locale,
  }) async => Result.ok(signsResult);

  @override
  Future<Result<Question>> question(String id, {required String locale}) async =>
      Result.ok(fakeQuestion(id));

  @override
  Future<Result<Sign>> signByCode(String code, {required String locale}) =>
      throw UnimplementedError();

  @override
  Future<Result<Variant>> variantByNumber(int n, {required String locale}) =>
      throw UnimplementedError();

  @override
  Future<Result<List<Variant>>> variants() => throw UnimplementedError();
}

Widget _wrap({required _FakeSessionApi sessionApi, required _FakeContentApi contentApi}) {
  final router = GoRouter(
    initialLocation: '/',
    routes: [
      GoRoute(
        path: '/',
        builder: (context, state) => const PracticeSetupScreen(),
      ),
      GoRoute(
        path: '/session',
        builder: (context, state) =>
            SessionScreen(request: state.extra! as SessionStartRequest),
      ),
      GoRoute(
        path: sessionResultRoute,
        builder: (context, state) =>
            SessionResultView(result: state.extra! as SessionResult),
      ),
    ],
  );
  return ProviderScope(
    overrides: [
      sessionApiProvider.overrideWithValue(sessionApi),
      contentApiProvider.overrideWithValue(contentApi),
    ],
    child: MaterialApp.router(
      routerConfig: router,
      locale: const Locale('uz'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
    ),
  );
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  const categories = [
    Category(code: 'signs', name: 'Yo\'l belgilari', sortOrder: 1),
  ];
  const signs = [Sign(code: 'a1', name: 'Bosh yo\'l', groupCode: 'priority')];

  testWidgets(
    'the start button stays disabled until exactly one of category/sign is '
    'chosen',
    (tester) async {
      final contentApi = _FakeContentApi(
        categoriesResult: categories,
        signsResult: signs,
      );
      await tester.pumpWidget(
        _wrap(sessionApi: _FakeSessionApi(), contentApi: contentApi),
      );
      await tester.pumpAndSettle();

      // Nothing picked yet.
      var button = tester.widget<FilledButton>(
        find.byKey(const Key('practice-start-button')),
      );
      expect(button.onPressed, isNull);

      // Target chosen, but no specific category value picked yet.
      await tester.tap(find.text('Kategoriya'));
      await tester.pumpAndSettle();
      button = tester.widget<FilledButton>(
        find.byKey(const Key('practice-start-button')),
      );
      expect(button.onPressed, isNull);

      // Now actually pick a category from the dropdown.
      await tester.tap(find.byKey(const Key('practice-category-dropdown')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Yo\'l belgilari').last);
      await tester.pumpAndSettle();

      button = tester.widget<FilledButton>(
        find.byKey(const Key('practice-start-button')),
      );
      expect(button.onPressed, isNotNull);
    },
  );

  testWidgets(
    'switching target clears the other pick — exactly one of '
    'category/sign is ever sent to start()',
    (tester) async {
      final api = _FakeSessionApi(
        startResult: fakeSummary(mode: 'practice', ids: ['q1']),
      );
      final contentApi = _FakeContentApi(
        categoriesResult: categories,
        signsResult: signs,
      );
      await tester.pumpWidget(_wrap(sessionApi: api, contentApi: contentApi));
      await tester.pumpAndSettle();

      // Pick a category first.
      await tester.tap(find.text('Kategoriya'));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('practice-category-dropdown')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Yo\'l belgilari').last);
      await tester.pumpAndSettle();

      // Now switch to "Belgi" — the category pick must be cleared, so the
      // button goes back to disabled until a sign is also picked.
      await tester.tap(find.text('Belgi'));
      await tester.pumpAndSettle();
      var button = tester.widget<FilledButton>(
        find.byKey(const Key('practice-start-button')),
      );
      expect(button.onPressed, isNull);

      await tester.tap(find.byKey(const Key('practice-sign-dropdown')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Bosh yo\'l').last);
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('practice-start-button')));
      await tester.pumpAndSettle();

      expect(api.startCalls.single.mode, 'practice');
      expect(api.startCalls.single.categoryId, isNull);
      expect(api.startCalls.single.signId, 'a1');
      expect(api.startCalls.single.count, 10);
    },
  );

  testWidgets(
    'SEAM TEST: a daily_limit_reached start() failure shows the distinct, '
    'non-alarming copy via the real session screen, not a generic error '
    'banner',
    (tester) async {
      final api = _FakeSessionApi(
        startFailure: const Failure(
          code: 'daily_limit_reached',
          message: 'raw',
        ),
      );
      final contentApi = _FakeContentApi(
        categoriesResult: categories,
        signsResult: signs,
      );
      await tester.pumpWidget(_wrap(sessionApi: api, contentApi: contentApi));
      await tester.pumpAndSettle();

      await tester.tap(find.text('Kategoriya'));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('practice-category-dropdown')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Yo\'l belgilari').last);
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('practice-start-button')));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('session-error-view')), findsOneWidget);
      expect(find.textContaining('limitga yetdingiz'), findsOneWidget);
    },
  );

  testWidgets('an invalid (zero) count disables the start button', (
    tester,
  ) async {
    final contentApi = _FakeContentApi(
      categoriesResult: categories,
      signsResult: signs,
    );
    await tester.pumpWidget(
      _wrap(sessionApi: _FakeSessionApi(), contentApi: contentApi),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Kategoriya'));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('practice-category-dropdown')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Yo\'l belgilari').last);
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(const Key('practice-count-field')),
      '0',
    );
    await tester.pump();

    final button = tester.widget<FilledButton>(
      find.byKey(const Key('practice-start-button')),
    );
    expect(button.onPressed, isNull);
  });
}
