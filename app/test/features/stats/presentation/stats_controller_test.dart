import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/stats/data/progress_api.dart';
import 'package:avtotest_app/features/stats/domain/stats.dart';
import 'package:avtotest_app/features/stats/domain/streak.dart';
import 'package:avtotest_app/features/stats/presentation/stats_controller.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class MockProgressApi extends Mock implements ProgressApi {}

const _testStreak = Streak(
  current: 5,
  best: 12,
  todayDone: 3,
  dailyGoal: 10,
  lastActiveDate: '2026-07-20',
);

const _testStats = Stats(
  categories: [
    CategoryStat(categoryCode: 'signs', mastery: 0.75, seen: 40, correct: 30),
  ],
  readinessPct: 62,
  dueCount: 4,
);

void main() {
  late MockProgressApi api;

  setUp(() {
    api = MockProgressApi();
  });

  ProviderContainer containerWith() {
    final container = ProviderContainer(
      overrides: [progressApiProvider.overrideWithValue(api)],
    );
    addTearDown(container.dispose);
    return container;
  }

  test('success populates both streak and stats', () async {
    when(() => api.streak())
        .thenAnswer((_) async => const Result<Streak>.ok(_testStreak));
    when(() => api.stats())
        .thenAnswer((_) async => const Result<Stats>.ok(_testStats));
    final container = containerWith();

    container.read(statsControllerProvider);
    await container.read(statsControllerProvider.future);

    final state = container.read(statsControllerProvider);
    expect(state, isA<AsyncData<StatsData>>());
    final value = state.requireValue;
    expect(value.streak, _testStreak);
    expect(value.stats, _testStats);
  });

  test('empty stats (fresh account) still succeeds', () async {
    const emptyStats = Stats(categories: [], readinessPct: 0, dueCount: 0);
    const freshStreak = Streak(current: 0, best: 0, todayDone: 0, dailyGoal: 10);
    when(() => api.streak())
        .thenAnswer((_) async => const Result<Streak>.ok(freshStreak));
    when(() => api.stats())
        .thenAnswer((_) async => const Result<Stats>.ok(emptyStats));
    final container = containerWith();

    await container.read(statsControllerProvider.future);

    final value = container.read(statsControllerProvider).requireValue;
    expect(value.stats.categories, isEmpty);
    expect(value.streak.lastActiveDate, isNull);
  });

  test('a failed streak fetch surfaces as AsyncError (StatsFetchFailure)',
      () async {
    when(() => api.streak()).thenAnswer(
      (_) async => const Result<Streak>.err(
        Failure(code: 'unauthorized', message: 'missing auth'),
      ),
    );
    when(() => api.stats())
        .thenAnswer((_) async => const Result<Stats>.ok(_testStats));
    final container = containerWith();

    container.read(statsControllerProvider);
    await pumpEventQueue();

    final state = container.read(statsControllerProvider);
    expect(state, isA<AsyncError<StatsData>>());
    final error = (state as AsyncError).error as StatsFetchFailure;
    expect(error.failure.code, 'unauthorized');
  });

  test('a failed stats fetch surfaces as AsyncError (StatsFetchFailure)',
      () async {
    when(() => api.streak())
        .thenAnswer((_) async => const Result<Streak>.ok(_testStreak));
    when(() => api.stats()).thenAnswer(
      (_) async => const Result<Stats>.err(
        Failure(code: 'internal', message: 'unexpected error'),
      ),
    );
    final container = containerWith();

    container.read(statsControllerProvider);
    await pumpEventQueue();

    final state = container.read(statsControllerProvider);
    expect(state, isA<AsyncError<StatsData>>());
    final error = (state as AsyncError).error as StatsFetchFailure;
    expect(error.failure.code, 'internal');
  });
}
