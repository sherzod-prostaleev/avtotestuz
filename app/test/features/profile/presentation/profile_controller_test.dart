import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/profile/data/profile_api.dart';
import 'package:avtotest_app/features/profile/domain/profile.dart';
import 'package:avtotest_app/features/profile/presentation/profile_controller.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class MockProfileApi extends Mock implements ProfileApi {}

final _testProfile = Profile(
  id: 'p1',
  phone: '+998901112233',
  name: 'Aziz Karimov',
  region: 'Toshkent',
  district: 'Chilonzor',
  localePref: 'uz-Latn',
  themePref: 'dark',
  referralCode: '',
  role: 'student',
  createdAt: DateTime(2024, 1, 2),
);

const _testEntitlement = Entitlement(active: true);

void main() {
  late MockProfileApi api;

  setUp(() {
    api = MockProfileApi();
  });

  group('build()', () {
    test(
      'success populates both profile and entitlement from concurrent '
      '/me and /me/entitlement calls',
      () async {
        when(() => api.fetchMe())
            .thenAnswer((_) async => Result<Profile>.ok(_testProfile));
        when(() => api.fetchEntitlement()).thenAnswer(
          (_) async => const Result<Entitlement>.ok(_testEntitlement),
        );
        final container = ProviderContainer(
          overrides: [profileApiProvider.overrideWithValue(api)],
        );
        addTearDown(container.dispose);

        // Reading the provider triggers build(); let the two concurrent
        // fetches settle before asserting (same idiom as
        // `auth_controller_test.dart`'s async-hydration tests).
        container.read(profileControllerProvider);
        await pumpEventQueue();

        final state = container.read(profileControllerProvider);
        expect(state, isA<AsyncData<Object?>>());
        final value =
            (state as AsyncData<({Profile profile, Entitlement entitlement})>)
                .value;
        expect(value.profile, _testProfile);
        expect(value.entitlement, _testEntitlement);
      },
    );

    test(
      'a failed /me call surfaces as AsyncError (not swallowed into a '
      'default/partial value), even though /me/entitlement succeeds',
      () async {
        when(() => api.fetchMe()).thenAnswer(
          (_) async => const Result<Profile>.err(
            Failure(code: 'unauthorized', message: 'missing auth'),
          ),
        );
        when(() => api.fetchEntitlement()).thenAnswer(
          (_) async => const Result<Entitlement>.ok(_testEntitlement),
        );
        final container = ProviderContainer(
          overrides: [profileApiProvider.overrideWithValue(api)],
        );
        addTearDown(container.dispose);

        container.read(profileControllerProvider);
        await pumpEventQueue();

        final state = container.read(profileControllerProvider);
        expect(state, isA<AsyncError<Object?>>());
        final error = (state as AsyncError).error as ProfileFetchFailure;
        expect(error.failure.code, 'unauthorized');
      },
    );

    test(
      'a failed /me/entitlement call surfaces as AsyncError even though '
      '/me succeeds — a partial failure must not be reported as success',
      () async {
        when(() => api.fetchMe())
            .thenAnswer((_) async => Result<Profile>.ok(_testProfile));
        when(() => api.fetchEntitlement()).thenAnswer(
          (_) async => const Result<Entitlement>.err(
            Failure(code: 'internal', message: 'entitlement query failed'),
          ),
        );
        final container = ProviderContainer(
          overrides: [profileApiProvider.overrideWithValue(api)],
        );
        addTearDown(container.dispose);

        container.read(profileControllerProvider);
        await pumpEventQueue();

        final state = container.read(profileControllerProvider);
        expect(state, isA<AsyncError<Object?>>());
        final error = (state as AsyncError).error as ProfileFetchFailure;
        expect(error.failure.code, 'internal');
      },
    );

    test('both /me and /me/entitlement are called exactly once', () async {
      when(() => api.fetchMe())
          .thenAnswer((_) async => Result<Profile>.ok(_testProfile));
      when(() => api.fetchEntitlement()).thenAnswer(
        (_) async => const Result<Entitlement>.ok(_testEntitlement),
      );
      final container = ProviderContainer(
        overrides: [profileApiProvider.overrideWithValue(api)],
      );
      addTearDown(container.dispose);

      container.read(profileControllerProvider);
      await pumpEventQueue();

      verify(() => api.fetchMe()).called(1);
      verify(() => api.fetchEntitlement()).called(1);
    });
  });
}
