import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/content/data/content_api.dart';
import 'package:avtotest_app/features/content/domain/sign.dart';
import 'package:avtotest_app/features/signs/presentation/signs_controller.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:shared_preferences/shared_preferences.dart';

class MockContentApi extends Mock implements ContentApi {}

const _signA = Sign(code: 'a', name: 'A belgisi', groupCode: 'g1');
const _signB = Sign(code: 'b', name: 'B belgisi', groupCode: 'g2');

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late MockContentApi api;

  setUp(() {
    // signsControllerProvider reads localeProvider, whose Notifier hydrates
    // from SharedPreferences — mock it so that hydration resolves cleanly.
    SharedPreferences.setMockInitialValues({});
    api = MockContentApi();
  });

  ProviderContainer container() {
    final c = ProviderContainer(
      overrides: [contentApiProvider.overrideWithValue(api)],
    );
    return c;
  }

  test('build() fetches the unfiltered list (group/query both null)', () async {
    when(
      () => api.signs(
        group: any(named: 'group'),
        query: any(named: 'query'),
        locale: any(named: 'locale'),
      ),
    ).thenAnswer((_) async => const Result.ok([_signA, _signB]));
    final c = container();
    addTearDown(c.dispose);

    c.read(signsControllerProvider);
    await pumpEventQueue();

    final state = c.read(signsControllerProvider);
    expect(state, isA<AsyncData<List<Sign>>>());
    expect((state as AsyncData<List<Sign>>).value, [_signA, _signB]);
    verify(
      () => api.signs(group: null, query: null, locale: any(named: 'locale')),
    ).called(1);
  });

  test('setGroup refetches with the new group filter applied', () async {
    when(
      () => api.signs(group: null, query: null, locale: any(named: 'locale')),
    ).thenAnswer((_) async => const Result.ok([_signA, _signB]));
    when(
      () => api.signs(group: 'g1', query: null, locale: any(named: 'locale')),
    ).thenAnswer((_) async => const Result.ok([_signA]));
    final c = container();
    addTearDown(c.dispose);

    c.read(signsControllerProvider);
    await pumpEventQueue();

    await c.read(signsControllerProvider.notifier).setGroup('g1');

    final state = c.read(signsControllerProvider);
    expect((state as AsyncData<List<Sign>>).value, [_signA]);
  });

  test('setQuery refetches with the new query filter applied', () async {
    when(
      () => api.signs(group: null, query: null, locale: any(named: 'locale')),
    ).thenAnswer((_) async => const Result.ok([_signA, _signB]));
    when(
      () => api.signs(group: null, query: 'A', locale: any(named: 'locale')),
    ).thenAnswer((_) async => const Result.ok([_signA]));
    final c = container();
    addTearDown(c.dispose);

    c.read(signsControllerProvider);
    await pumpEventQueue();

    await c.read(signsControllerProvider.notifier).setQuery('A');

    final state = c.read(signsControllerProvider);
    expect((state as AsyncData<List<Sign>>).value, [_signA]);
  });

  test('an empty setQuery clears the filter back to null', () async {
    when(
      () => api.signs(group: null, query: null, locale: any(named: 'locale')),
    ).thenAnswer((_) async => const Result.ok([_signA, _signB]));
    final c = container();
    addTearDown(c.dispose);

    c.read(signsControllerProvider);
    await pumpEventQueue();

    await c.read(signsControllerProvider.notifier).setQuery('');

    verify(
      () => api.signs(group: null, query: null, locale: any(named: 'locale')),
    ).called(2); // once for build(), once for the empty-query refetch.
  });

  test(
    'a failed fetch surfaces as AsyncError carrying the original Failure '
    '(not swallowed into an empty list)',
    () async {
      when(
        () => api.signs(
          group: any(named: 'group'),
          query: any(named: 'query'),
          locale: any(named: 'locale'),
        ),
      ).thenAnswer(
        (_) async =>
            const Result.err(Failure(code: 'unknown', message: 'boom')),
      );
      final c = container();
      addTearDown(c.dispose);

      c.read(signsControllerProvider);
      await pumpEventQueue();

      final state = c.read(signsControllerProvider);
      expect(state, isA<AsyncError<List<Sign>>>());
      final error = (state as AsyncError).error as SignsFetchFailure;
      expect(error.failure.code, 'unknown');
    },
  );
}
