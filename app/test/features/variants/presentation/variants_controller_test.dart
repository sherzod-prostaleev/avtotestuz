import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/session/data/session_api.dart';
import 'package:avtotest_app/features/session/domain/session_models.dart';
import 'package:avtotest_app/features/variants/presentation/variants_controller.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class MockSessionApi extends Mock implements SessionApi {}

const _variants = [
  VariantStatus(
    number: 1,
    questionCount: 20,
    unlocked: true,
    bestCorrect: 18,
    attempts: 2,
  ),
  VariantStatus(
    number: 2,
    questionCount: 20,
    unlocked: false,
    bestCorrect: 0,
    attempts: 0,
  ),
];

void main() {
  late MockSessionApi api;

  setUp(() {
    api = MockSessionApi();
  });

  group('build()', () {
    test('success populates the variant list from GET /me/variants', () async {
      when(() => api.myVariants())
          .thenAnswer((_) async => const Result.ok(_variants));
      final container = ProviderContainer(
        overrides: [sessionApiProvider.overrideWithValue(api)],
      );
      addTearDown(container.dispose);

      container.read(variantsControllerProvider);
      await pumpEventQueue();

      final state = container.read(variantsControllerProvider);
      expect(state, isA<AsyncData<List<VariantStatus>>>());
      expect((state as AsyncData<List<VariantStatus>>).value, _variants);
    });

    test(
      'a failed /me/variants call surfaces as AsyncError carrying the '
      'original Failure (not swallowed into a default empty list)',
      () async {
        when(() => api.myVariants()).thenAnswer(
          (_) async => const Result.err(
            Failure(code: 'unauthorized', message: 'missing auth'),
          ),
        );
        final container = ProviderContainer(
          overrides: [sessionApiProvider.overrideWithValue(api)],
        );
        addTearDown(container.dispose);

        container.read(variantsControllerProvider);
        await pumpEventQueue();

        final state = container.read(variantsControllerProvider);
        expect(state, isA<AsyncError<List<VariantStatus>>>());
        final error = (state as AsyncError).error as VariantsFetchFailure;
        expect(error.failure.code, 'unauthorized');
      },
    );

    test('GET /me/variants is called exactly once', () async {
      when(() => api.myVariants())
          .thenAnswer((_) async => const Result.ok(_variants));
      final container = ProviderContainer(
        overrides: [sessionApiProvider.overrideWithValue(api)],
      );
      addTearDown(container.dispose);

      container.read(variantsControllerProvider);
      await pumpEventQueue();

      verify(() => api.myVariants()).called(1);
    });
  });
}
