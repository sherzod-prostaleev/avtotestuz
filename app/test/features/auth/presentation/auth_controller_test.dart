import 'dart:async';

import 'package:avtotest_app/core/network/token_storage.dart';
import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/auth/data/auth_api.dart';
import 'package:avtotest_app/features/auth/data/auth_repository.dart';
import 'package:avtotest_app/features/auth/domain/auth_state.dart';
import 'package:avtotest_app/features/auth/presentation/auth_controller.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class MockAuthRepository extends Mock implements AuthRepository {}

class MockAuthApi extends Mock implements AuthApi {}

/// In-memory [TokenStorage] fake — no shared_preferences plugin channel
/// involved. Mirrors the fakes already used in
/// `test/core/network/auth_interceptor_test.dart` and
/// `test/features/auth/data/auth_repository_test.dart`.
class FakeTokenStorage implements TokenStorage {
  String? _access;
  String? _refresh;
  int saveCalls = 0;

  @override
  Future<String?> readAccess() async => _access;

  @override
  Future<String?> readRefresh() async => _refresh;

  @override
  Future<void> save({required String access, required String refresh}) async {
    saveCalls++;
    _access = access;
    _refresh = refresh;
  }

  @override
  Future<void> clear() async {
    _access = null;
    _refresh = null;
  }
}

void main() {
  late MockAuthRepository repository;

  setUp(() {
    repository = MockAuthRepository();
  });

  group('build()', () {
    test('returns AuthState.unknown() synchronously, then settles to '
        'authenticated when a stored session exists', () async {
      when(() => repository.hasStoredSession()).thenAnswer((_) async => true);
      final container = ProviderContainer(
        overrides: [authRepositoryProvider.overrideWithValue(repository)],
      );
      addTearDown(container.dispose);

      expect(container.read(authControllerProvider), const AuthState.unknown());

      await pumpEventQueue();

      expect(
        container.read(authControllerProvider),
        const AuthState.authenticated(),
      );
    });

    test('settles to unauthenticated when no stored session exists',
        () async {
      when(() => repository.hasStoredSession())
          .thenAnswer((_) async => false);
      final container = ProviderContainer(
        overrides: [authRepositoryProvider.overrideWithValue(repository)],
      );
      addTearDown(container.dispose);

      expect(container.read(authControllerProvider), const AuthState.unknown());

      await pumpEventQueue();

      expect(
        container.read(authControllerProvider),
        const AuthState.unauthenticated(),
      );
    });

    test(
      'a stale hasStoredSession() result must NOT clobber state that has '
      'already advanced past unknown (e.g. via requestOtp) by the time it '
      'resolves',
      () async {
        // Gate hasStoredSession() on a Completer we control explicitly,
        // so the test pins down the exact interleaving instead of hoping
        // the two futures happen to race a particular way — same
        // discriminating-test discipline as
        // `theme_mode_provider_test.dart`'s `_GatedSharedPreferencesStore`
        // test.
        final hasSessionGate = Completer<bool>();
        when(() => repository.hasStoredSession())
            .thenAnswer((_) => hasSessionGate.future);
        when(() => repository.requestOtp('901112233')).thenAnswer(
          (_) async => const Result<({String channel, String? debugCode})>.ok(
            (channel: 'sandbox', debugCode: '123456'),
          ),
        );

        final container = ProviderContainer(
          overrides: [authRepositoryProvider.overrideWithValue(repository)],
        );
        addTearDown(container.dispose);

        // Reading the provider triggers build(): AuthState.unknown() is
        // returned synchronously, and hasStoredSession() is now suspended
        // on hasSessionGate — still incomplete.
        expect(
          container.read(authControllerProvider),
          const AuthState.unknown(),
        );

        // A user action races ahead of the stale session check and fully
        // resolves (requestOtp's mocked future completes immediately),
        // advancing state past `unknown` before hasSessionGate ever fires.
        await container
            .read(authControllerProvider.notifier)
            .requestOtp('901112233');
        expect(
          container.read(authControllerProvider),
          isA<AuthOtpRequested>(),
        );

        // Only now does the stale session check resolve, simulating a
        // stored session that would otherwise flip state to
        // `authenticated` — a determination that is stale by this point.
        hasSessionGate.complete(true);
        await pumpEventQueue();

        // The guard must have dropped the stale write: state remains
        // whatever the user action produced, not `authenticated`.
        expect(
          container.read(authControllerProvider),
          isA<AuthOtpRequested>(),
        );
        final state = container.read(authControllerProvider) as AuthOtpRequested;
        expect(state.phone, '901112233');
        expect(state.debugCode, '123456');
      },
    );
  });

  group('requestOtp', () {
    test('success transitions to AuthOtpRequested carrying debugCode through',
        () async {
      when(() => repository.hasStoredSession())
          .thenAnswer((_) async => false);
      when(() => repository.requestOtp('901112233')).thenAnswer(
        (_) async => const Result<({String channel, String? debugCode})>.ok(
          (channel: 'sandbox', debugCode: '654321'),
        ),
      );
      final container = ProviderContainer(
        overrides: [authRepositoryProvider.overrideWithValue(repository)],
      );
      addTearDown(container.dispose);
      await pumpEventQueue();

      await container
          .read(authControllerProvider.notifier)
          .requestOtp('901112233');

      final state = container.read(authControllerProvider);
      expect(state, isA<AuthOtpRequested>());
      expect((state as AuthOtpRequested).phone, '901112233');
      expect(state.debugCode, '654321');
    });

    test('success with no debugCode (real SMS channel) carries null through',
        () async {
      when(() => repository.hasStoredSession())
          .thenAnswer((_) async => false);
      when(() => repository.requestOtp('901112233')).thenAnswer(
        (_) async => const Result<({String channel, String? debugCode})>.ok(
          (channel: 'sms', debugCode: null),
        ),
      );
      final container = ProviderContainer(
        overrides: [authRepositoryProvider.overrideWithValue(repository)],
      );
      addTearDown(container.dispose);
      await pumpEventQueue();

      await container
          .read(authControllerProvider.notifier)
          .requestOtp('901112233');

      final state =
          container.read(authControllerProvider) as AuthOtpRequested;
      expect(state.debugCode, isNull);
    });

    test('failure transitions to AuthError with the exact Failure', () async {
      when(() => repository.hasStoredSession())
          .thenAnswer((_) async => false);
      when(() => repository.requestOtp('bad')).thenAnswer(
        (_) async => const Result<({String channel, String? debugCode})>.err(
          Failure(
            code: 'invalid_phone',
            message: 'phone number is not a valid Uzbekistan number',
          ),
        ),
      );
      final container = ProviderContainer(
        overrides: [authRepositoryProvider.overrideWithValue(repository)],
      );
      addTearDown(container.dispose);
      await pumpEventQueue();

      await container.read(authControllerProvider.notifier).requestOtp('bad');

      final state = container.read(authControllerProvider);
      expect(state, isA<AuthError>());
      expect((state as AuthError).failure.code, 'invalid_phone');
    });
  });

  group('verifyOtp', () {
    test('success saves tokens (via a mocked TokenStorage) and transitions '
        'to AuthAuthenticated', () async {
      final api = MockAuthApi();
      final tokenStorage = FakeTokenStorage();
      final realRepository =
          AuthRepository(api: api, tokenStorage: tokenStorage);
      when(() => api.requestOtp('901112233')).thenAnswer(
        (_) async => const Result<({String channel, String? debugCode})>.ok(
          (channel: 'sandbox', debugCode: '111111'),
        ),
      );
      when(() => api.verifyOtp(phone: '901112233', code: '111111'))
          .thenAnswer(
        (_) async => const Result<({String access, String refresh})>.ok(
          (access: 'access-tok', refresh: 'refresh-tok'),
        ),
      );

      final container = ProviderContainer(
        overrides: [authRepositoryProvider.overrideWithValue(realRepository)],
      );
      addTearDown(container.dispose);
      await pumpEventQueue();

      // Enter otpRequested first (verifyOtp uses the phone from state).
      await container
          .read(authControllerProvider.notifier)
          .requestOtp('901112233');
      expect(
        container.read(authControllerProvider),
        isA<AuthOtpRequested>(),
      );

      await container.read(authControllerProvider.notifier).verifyOtp('111111');

      expect(
        container.read(authControllerProvider),
        const AuthState.authenticated(),
      );
      expect(tokenStorage.saveCalls, 1);
      expect(await tokenStorage.readAccess(), 'access-tok');
      expect(await tokenStorage.readRefresh(), 'refresh-tok');
    });

    test('failure with invalid_code surfaces that specific code', () async {
      when(() => repository.hasStoredSession())
          .thenAnswer((_) async => false);
      when(() => repository.requestOtp('901112233')).thenAnswer(
        (_) async => const Result<({String channel, String? debugCode})>.ok(
          (channel: 'sandbox', debugCode: '111111'),
        ),
      );
      when(
        () => repository.verifyOtp(phone: '901112233', code: '000000'),
      ).thenAnswer(
        (_) async => const Result<void>.err(
          Failure(code: 'invalid_code', message: 'code is incorrect'),
        ),
      );
      final container = ProviderContainer(
        overrides: [authRepositoryProvider.overrideWithValue(repository)],
      );
      addTearDown(container.dispose);
      await pumpEventQueue();

      await container
          .read(authControllerProvider.notifier)
          .requestOtp('901112233');
      await container
          .read(authControllerProvider.notifier)
          .verifyOtp('000000');

      final state = container.read(authControllerProvider);
      expect(state, isA<AuthError>());
      expect((state as AuthError).failure.code, 'invalid_code');
    });

    test(
      'retrying after a failed verifyOtp actually calls the repository '
      'again — the guard must not treat a state that moved to AuthError as '
      '"nothing to retry" (regression: a typo on the first attempt used to '
      'silently no-op every subsequent retry)',
      () async {
        when(() => repository.hasStoredSession())
            .thenAnswer((_) async => false);
        when(() => repository.requestOtp('901112233')).thenAnswer(
          (_) async => const Result<({String channel, String? debugCode})>.ok(
            (channel: 'sandbox', debugCode: '111111'),
          ),
        );
        when(
          () => repository.verifyOtp(phone: '901112233', code: '000000'),
        ).thenAnswer(
          (_) async => const Result<void>.err(
            Failure(code: 'invalid_code', message: 'code is incorrect'),
          ),
        );
        when(
          () => repository.verifyOtp(phone: '901112233', code: '111111'),
        ).thenAnswer((_) async => const Result<void>.ok(null));
        final container = ProviderContainer(
          overrides: [authRepositoryProvider.overrideWithValue(repository)],
        );
        addTearDown(container.dispose);
        await pumpEventQueue();

        await container
            .read(authControllerProvider.notifier)
            .requestOtp('901112233');
        expect(
          container.read(authControllerProvider),
          isA<AuthOtpRequested>(),
        );

        // First attempt: a typo. Fails and moves state to AuthError.
        await container
            .read(authControllerProvider.notifier)
            .verifyOtp('000000');
        expect(container.read(authControllerProvider), isA<AuthError>());

        // Second attempt: the user corrects the typo. State is now
        // AuthError, NOT AuthOtpRequested — the guard must still let this
        // call through to the repository rather than silently no-op'ing.
        await container
            .read(authControllerProvider.notifier)
            .verifyOtp('111111');

        verify(
          () => repository.verifyOtp(phone: '901112233', code: '000000'),
        ).called(1);
        verify(
          () => repository.verifyOtp(phone: '901112233', code: '111111'),
        ).called(1);
        expect(
          container.read(authControllerProvider),
          const AuthState.authenticated(),
        );
      },
    );

    test('a distinct code (expired_code) is also surfaced untouched, not '
        'collapsed into a generic message', () async {
      when(() => repository.hasStoredSession())
          .thenAnswer((_) async => false);
      when(() => repository.requestOtp('901112233')).thenAnswer(
        (_) async => const Result<({String channel, String? debugCode})>.ok(
          (channel: 'sandbox', debugCode: '111111'),
        ),
      );
      when(
        () => repository.verifyOtp(phone: '901112233', code: '111111'),
      ).thenAnswer(
        (_) async => const Result<void>.err(
          Failure(code: 'expired_code', message: 'code has expired'),
        ),
      );
      final container = ProviderContainer(
        overrides: [authRepositoryProvider.overrideWithValue(repository)],
      );
      addTearDown(container.dispose);
      await pumpEventQueue();

      await container
          .read(authControllerProvider.notifier)
          .requestOtp('901112233');
      await container
          .read(authControllerProvider.notifier)
          .verifyOtp('111111');

      final state = container.read(authControllerProvider);
      expect(state, isA<AuthError>());
      expect((state as AuthError).failure.code, 'expired_code');
    });
  });

  group('logout', () {
    test('clears the session and transitions to AuthUnauthenticated',
        () async {
      when(() => repository.hasStoredSession())
          .thenAnswer((_) async => true);
      when(() => repository.logout()).thenAnswer((_) async {});
      final container = ProviderContainer(
        overrides: [authRepositoryProvider.overrideWithValue(repository)],
      );
      addTearDown(container.dispose);
      // Reading the provider triggers build(); let its background session
      // check settle before proceeding.
      container.read(authControllerProvider);
      await pumpEventQueue();
      expect(
        container.read(authControllerProvider),
        const AuthState.authenticated(),
      );

      await container.read(authControllerProvider.notifier).logout();

      verify(() => repository.logout()).called(1);
      expect(
        container.read(authControllerProvider),
        const AuthState.unauthenticated(),
      );
    });
  });
}
