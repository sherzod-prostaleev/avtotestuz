import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/content/data/content_api.dart';
import 'package:avtotest_app/features/content/domain/category.dart';
import 'package:avtotest_app/features/content/domain/question.dart';
import 'package:avtotest_app/features/content/domain/sign.dart';
import 'package:avtotest_app/features/content/domain/variant.dart';
import 'package:avtotest_app/features/mistakes/presentation/mistakes_screen.dart';
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

/// A local [SessionApi] fake (deliberately not `session_fakes.dart`'s
/// `FakeSessionApi` — this task keeps its own test doubles self-contained
/// rather than editing another task's already-committed test helper)
/// capturing every `start()` call's `mode`/`count` so the "exactly what the
/// screen asked for" assertions below have something to check.
class _FakeSessionApi implements SessionApi {
  _FakeSessionApi({this.startResult, this.startFailure});

  final SessionSummary? startResult;
  final Failure? startFailure;
  final List<({String mode, int? count})> startCalls = [];

  @override
  Future<Result<SessionSummary>> start({
    required String mode,
    String? variantId,
    String? categoryId,
    String? signId,
    required String locale,
    int? count,
  }) async {
    startCalls.add((mode: mode, count: count));
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

/// A minimal [ContentApi] fake — only `question()` is exercised, when a
/// successful `start()` navigates into the real [SessionScreen].
class _FakeContentApi implements ContentApi {
  @override
  Future<Result<Question>> question(String id, {required String locale}) async =>
      Result.ok(fakeQuestion(id));

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

/// Builds a real router with `/` (the [MistakesScreen] under test), `/session`
/// (the real [SessionScreen], same wiring `router.dart` uses) and the
/// results route — only the two network APIs are faked, so a tap genuinely
/// exercises the "screen navigates → real session screen reacts to whatever
/// the server said" seam.
Widget _wrap({required _FakeSessionApi sessionApi, ContentApi? contentApi}) {
  final router = GoRouter(
    initialLocation: '/',
    routes: [
      GoRoute(path: '/', builder: (context, state) => const MistakesScreen()),
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
      contentApiProvider.overrideWithValue(contentApi ?? _FakeContentApi()),
    ],
    child: MaterialApp.router(routerConfig: router),
  );
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  testWidgets('defaults the count to 10 and the start button is enabled', (
    tester,
  ) async {
    await tester.pumpWidget(_wrap(sessionApi: _FakeSessionApi()));
    await tester.pumpAndSettle();

    final field = tester.widget<TextField>(
      find.byKey(const Key('mistakes-count-field')),
    );
    expect(field.controller!.text, '10');
    final button = tester.widget<FilledButton>(
      find.byKey(const Key('mistakes-start-button')),
    );
    expect(button.onPressed, isNotNull);
  });

  testWidgets('an invalid (zero) count disables the start button', (
    tester,
  ) async {
    await tester.pumpWidget(_wrap(sessionApi: _FakeSessionApi()));
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(const Key('mistakes-count-field')),
      '0',
    );
    await tester.pump();

    final button = tester.widget<FilledButton>(
      find.byKey(const Key('mistakes-start-button')),
    );
    expect(button.onPressed, isNull);
  });

  testWidgets(
    'tapping start calls SessionApi.start(mode: mistakes, count: 10) and '
    'navigates to the real session screen',
    (tester) async {
      final api = _FakeSessionApi(
        startResult: fakeSummary(mode: 'mistakes', ids: ['q1']),
      );
      await tester.pumpWidget(_wrap(sessionApi: api));
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('mistakes-start-button')));
      await tester.pumpAndSettle();

      expect(api.startCalls.single.mode, 'mistakes');
      expect(api.startCalls.single.count, 10);
      expect(find.text('Savol matni q1'), findsOneWidget);
    },
  );

  testWidgets(
    'SEAM TEST: a vip_required start() failure surfaces the distinct upsell '
    'copy via the real session screen, not a generic error banner',
    (tester) async {
      final api = _FakeSessionApi(
        startFailure: const Failure(code: 'vip_required', message: 'raw'),
      );
      await tester.pumpWidget(_wrap(sessionApi: api));
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('mistakes-start-button')));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('session-error-view')), findsOneWidget);
      expect(find.textContaining('obuna kerak'), findsOneWidget);
    },
  );
}
