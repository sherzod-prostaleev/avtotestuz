import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/stats/data/progress_api.dart';
import 'package:avtotest_app/features/stats/domain/stats.dart';
import 'package:avtotest_app/features/stats/domain/streak.dart';
import 'package:avtotest_app/features/stats/presentation/stats_screen.dart';
import 'package:avtotest_app/shared/widgets/mastery_bar.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class MockProgressApi extends Mock implements ProgressApi {}

const _streak = Streak(
  current: 5,
  best: 12,
  todayDone: 3,
  dailyGoal: 10,
  lastActiveDate: '2026-07-20',
);

const _stats = Stats(
  categories: [
    CategoryStat(categoryCode: 'signs', mastery: 0.75, seen: 40, correct: 30),
    CategoryStat(categoryCode: 'traffic', mastery: 0.5, seen: 20, correct: 10),
  ],
  readinessPct: 62,
  dueCount: 4,
);

/// Wraps the REAL [StatsScreen] over the REAL `statsControllerProvider`,
/// faking only the network boundary ([ProgressApi]) — the seam test this
/// plan asks for (screen + controller driven together, not each in
/// isolation).
Widget _wrap(MockProgressApi api) {
  return ProviderScope(
    overrides: [progressApiProvider.overrideWithValue(api)],
    child: const MaterialApp(home: StatsScreen()),
  );
}

void main() {
  late MockProgressApi api;

  setUp(() {
    api = MockProgressApi();
  });

  testWidgets('renders streak, readiness, due count and one bar per category',
      (tester) async {
    when(() => api.streak())
        .thenAnswer((_) async => const Result<Streak>.ok(_streak));
    when(() => api.stats())
        .thenAnswer((_) async => const Result<Stats>.ok(_stats));

    await tester.pumpWidget(_wrap(api));
    await tester.pumpAndSettle();

    // Streak card.
    expect(find.byKey(const Key('streakCard')), findsOneWidget);
    expect(find.text('5 kun'), findsOneWidget);
    expect(find.text('Rekord: 12'), findsOneWidget);
    expect(find.text('Bugun: 3/10'), findsOneWidget);

    // Readiness + due count.
    expect(find.byKey(const Key('readinessPctText')), findsOneWidget);
    expect(find.text('62%'), findsOneWidget);
    expect(find.byKey(const Key('dueCountText')), findsOneWidget);
    expect(find.text('4'), findsOneWidget);

    // One MasteryBar per category.
    expect(find.byType(MasteryBar), findsNWidgets(2));
    expect(find.byKey(const Key('masteryBar-signs')), findsOneWidget);
    expect(find.byKey(const Key('masteryBar-traffic')), findsOneWidget);
  });

  testWidgets('empty categories shows the informational placeholder',
      (tester) async {
    when(() => api.streak()).thenAnswer(
      (_) async => const Result<Streak>.ok(
        Streak(current: 0, best: 0, todayDone: 0, dailyGoal: 10),
      ),
    );
    when(() => api.stats()).thenAnswer(
      (_) async => const Result<Stats>.ok(
        Stats(categories: [], readinessPct: 0, dueCount: 0),
      ),
    );

    await tester.pumpWidget(_wrap(api));
    await tester.pumpAndSettle();

    expect(find.byType(MasteryBar), findsNothing);
    expect(find.byKey(const Key('stats-empty-categories')), findsOneWidget);
    // Never-active streak label.
    expect(find.text('Oxirgi faollik: hali faol emas'), findsOneWidget);
  });

  testWidgets('a fetch failure shows the error state with the backend message',
      (tester) async {
    when(() => api.streak()).thenAnswer(
      (_) async => const Result<Streak>.err(
        Failure(code: 'unauthorized', message: 'missing auth'),
      ),
    );
    when(() => api.stats())
        .thenAnswer((_) async => const Result<Stats>.ok(_stats));

    await tester.pumpWidget(_wrap(api));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('stats-error-state')), findsOneWidget);
    expect(find.text('missing auth'), findsOneWidget);
  });
}
