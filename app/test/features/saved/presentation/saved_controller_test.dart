import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/saved/data/saved_api.dart';
import 'package:avtotest_app/features/saved/domain/saved_entry.dart';
import 'package:avtotest_app/features/saved/presentation/saved_controller.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class MockSavedApi extends Mock implements SavedApi {}

final _entryA = SavedEntry(questionId: 'q1', createdAt: DateTime(2026, 1, 1));
final _entryB = SavedEntry(questionId: 'q2', createdAt: DateTime(2026, 1, 2));

void main() {
  late MockSavedApi api;

  setUp(() {
    api = MockSavedApi();
    registerFallbackValue('');
  });

  ProviderContainer container() {
    final c = ProviderContainer(
      overrides: [savedApiProvider.overrideWithValue(api)],
    );
    return c;
  }

  test('build() fetches the saved list', () async {
    when(() => api.list()).thenAnswer((_) async => Result.ok([_entryA, _entryB]));
    final c = container();
    addTearDown(c.dispose);

    c.read(savedControllerProvider);
    await pumpEventQueue();

    final state = c.read(savedControllerProvider);
    expect(state, isA<AsyncData<List<SavedEntry>>>());
    expect((state as AsyncData<List<SavedEntry>>).value, [_entryA, _entryB]);
  });

  test('build() succeeds with an empty list', () async {
    when(() => api.list()).thenAnswer((_) async => const Result.ok([]));
    final c = container();
    addTearDown(c.dispose);

    c.read(savedControllerProvider);
    await pumpEventQueue();

    final state = c.read(savedControllerProvider);
    expect((state as AsyncData<List<SavedEntry>>).value, isEmpty);
  });

  test(
    'a failed fetch surfaces as AsyncError carrying the original Failure',
    () async {
      when(() => api.list()).thenAnswer(
        (_) async =>
            const Result.err(Failure(code: 'unknown', message: 'boom')),
      );
      final c = container();
      addTearDown(c.dispose);

      c.read(savedControllerProvider);
      await pumpEventQueue();

      final state = c.read(savedControllerProvider);
      expect(state, isA<AsyncError<List<SavedEntry>>>());
      final error = (state as AsyncError).error as SavedFetchFailure;
      expect(error.failure.code, 'unknown');
    },
  );

  test('toggle() calls save() when the question is not yet saved, then '
      'appends it to the local list', () async {
    when(() => api.list()).thenAnswer((_) async => Result.ok([_entryA]));
    when(() => api.save('q2')).thenAnswer((_) async => const Result.ok(null));
    final c = container();
    addTearDown(c.dispose);

    c.read(savedControllerProvider);
    await pumpEventQueue();

    final result = await c.read(savedControllerProvider.notifier).toggle('q2');

    expect(result, isA<Ok<void>>());
    verify(() => api.save('q2')).called(1);
    verifyNever(() => api.unsave(any()));
    final state = c.read(savedControllerProvider);
    final questionIds = (state as AsyncData<List<SavedEntry>>).value
        .map((e) => e.questionId);
    expect(questionIds, containsAll(['q1', 'q2']));
  });

  test('toggle() calls unsave() when the question is already saved, then '
      'removes it from the local list', () async {
    when(() => api.list()).thenAnswer((_) async => Result.ok([_entryA, _entryB]));
    when(() => api.unsave('q1')).thenAnswer((_) async => const Result.ok(null));
    final c = container();
    addTearDown(c.dispose);

    c.read(savedControllerProvider);
    await pumpEventQueue();

    final result = await c.read(savedControllerProvider.notifier).toggle('q1');

    expect(result, isA<Ok<void>>());
    verify(() => api.unsave('q1')).called(1);
    verifyNever(() => api.save(any()));
    final state = c.read(savedControllerProvider);
    final questionIds = (state as AsyncData<List<SavedEntry>>).value
        .map((e) => e.questionId);
    expect(questionIds, ['q2']);
  });

  test('a failed toggle leaves the local list untouched and returns Err', () async {
    when(() => api.list()).thenAnswer((_) async => Result.ok([_entryA]));
    when(() => api.save('q2')).thenAnswer(
      (_) async =>
          const Result.err(Failure(code: 'network_error', message: 'boom')),
    );
    final c = container();
    addTearDown(c.dispose);

    c.read(savedControllerProvider);
    await pumpEventQueue();

    final result = await c.read(savedControllerProvider.notifier).toggle('q2');

    expect(result, isA<Err<void>>());
    final state = c.read(savedControllerProvider);
    final questionIds = (state as AsyncData<List<SavedEntry>>).value
        .map((e) => e.questionId);
    expect(questionIds, ['q1']); // unchanged — q2 never got added.
  });
}
