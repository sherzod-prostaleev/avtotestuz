import 'package:avtotest_app/app/l10n/app_localizations.dart';
import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/content/data/content_api.dart';
import 'package:avtotest_app/features/content/domain/category.dart';
import 'package:avtotest_app/features/content/domain/question.dart';
import 'package:avtotest_app/features/content/domain/sign.dart';
import 'package:avtotest_app/features/content/domain/variant.dart';
import 'package:avtotest_app/features/signs/presentation/signs_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// A [ContentApi] fake whose `signs()` actually filters [allSigns] by
/// `group`/`query` the way the real backend does — this lets the seam test
/// (real [SignsScreen] + real `SignsController`, fake only `ContentApi`)
/// exercise genuine filtering behaviour, not a canned single response.
class _FakeContentApi implements ContentApi {
  _FakeContentApi({required this.allSigns});

  final List<Sign> allSigns;

  @override
  Future<Result<List<Sign>>> signs({
    String? group,
    String? query,
    required String locale,
  }) async {
    var result = allSigns;
    if (group != null) {
      result = result.where((s) => s.groupCode == group).toList();
    }
    if (query != null) {
      final lower = query.toLowerCase();
      result = result
          .where((s) => s.name.toLowerCase().contains(lower))
          .toList();
    }
    return Result.ok(result);
  }

  @override
  Future<Result<List<Category>>> categories({required String locale}) =>
      throw UnimplementedError();

  @override
  Future<Result<Question>> question(String id, {required String locale}) =>
      throw UnimplementedError();

  @override
  Future<Result<Sign>> signByCode(String code, {required String locale}) =>
      throw UnimplementedError();

  @override
  Future<Result<Variant>> variantByNumber(int n, {required String locale}) =>
      throw UnimplementedError();

  @override
  Future<Result<List<Variant>>> variants() => throw UnimplementedError();
}

Widget _wrap(ContentApi contentApi) {
  return ProviderScope(
    overrides: [contentApiProvider.overrideWithValue(contentApi)],
    child: MaterialApp(
      home: const SignsScreen(),
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

  const signA = Sign(code: 'a', name: 'Berish yo\'li', groupCode: 'priority');
  const signB = Sign(
    code: 'b',
    name: 'Taqiqlash belgisi',
    groupCode: 'prohibition',
  );

  testWidgets('renders the fetched sign list', (tester) async {
    await tester.pumpWidget(_wrap(_FakeContentApi(allSigns: [signA, signB])));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('signs-list')), findsOneWidget);
    expect(find.text('Berish yo\'li'), findsOneWidget);
    expect(find.text('Taqiqlash belgisi'), findsOneWidget);
  });

  testWidgets('filters the catalog by group', (tester) async {
    await tester.pumpWidget(_wrap(_FakeContentApi(allSigns: [signA, signB])));
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(const Key('signs-group-field')),
      'priority',
    );
    await tester.pumpAndSettle();

    expect(find.text('Berish yo\'li'), findsOneWidget);
    expect(find.text('Taqiqlash belgisi'), findsNothing);
  });

  testWidgets('filters the catalog by free-text query', (tester) async {
    await tester.pumpWidget(_wrap(_FakeContentApi(allSigns: [signA, signB])));
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(const Key('signs-query-field')),
      'Taqiqlash',
    );
    await tester.pumpAndSettle();

    expect(find.text('Taqiqlash belgisi'), findsOneWidget);
    expect(find.text('Berish yo\'li'), findsNothing);
  });

  testWidgets('shows an empty state when no sign matches the filter', (
    tester,
  ) async {
    await tester.pumpWidget(_wrap(_FakeContentApi(allSigns: [signA, signB])));
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(const Key('signs-query-field')),
      'yo\'q narsa',
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('signs-empty-state')), findsOneWidget);
  });

  testWidgets('a failed fetch shows an error state with retry', (
    tester,
  ) async {
    final api = _FailingContentApi();
    await tester.pumpWidget(_wrap(api));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('signs-error-state')), findsOneWidget);
    expect(find.text('boom'), findsOneWidget);
  });
}

class _FailingContentApi implements ContentApi {
  @override
  Future<Result<List<Sign>>> signs({
    String? group,
    String? query,
    required String locale,
  }) async => const Result.err(Failure(code: 'unknown', message: 'boom'));

  @override
  Future<Result<List<Category>>> categories({required String locale}) =>
      throw UnimplementedError();

  @override
  Future<Result<Question>> question(String id, {required String locale}) =>
      throw UnimplementedError();

  @override
  Future<Result<Sign>> signByCode(String code, {required String locale}) =>
      throw UnimplementedError();

  @override
  Future<Result<Variant>> variantByNumber(int n, {required String locale}) =>
      throw UnimplementedError();

  @override
  Future<Result<List<Variant>>> variants() => throw UnimplementedError();
}
