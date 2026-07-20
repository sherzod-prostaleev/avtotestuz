import 'dart:async';

import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/events/data/events_api.dart';
import 'package:avtotest_app/features/events/domain/client_event.dart';
import 'package:avtotest_app/features/events/presentation/event_logger.dart';
import 'package:fake_async/fake_async.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class MockEventsApi extends Mock implements EventsApi {}

void main() {
  setUpAll(() {
    registerFallbackValue(<ClientEvent>[]);
  });

  late MockEventsApi api;

  setUp(() {
    api = MockEventsApi();
  });

  group('log()', () {
    test('enqueues the event synchronously without touching the network',
        () {
      final logger = EventLogger(
        api,
        flushInterval: const Duration(minutes: 10),
      );

      logger.log('view_question', props: {'question_id': 'q1'});

      // No `await` anywhere above: if `log()` blocked on or awaited the
      // network, this assertion couldn't even run yet.
      expect(logger.queueForTesting, hasLength(1));
      expect(logger.queueForTesting.single.name, 'view_question');
      expect(
        logger.queueForTesting.single.props,
        {'question_id': 'q1'},
      );
      verifyNever(() => api.logBatch(any()));

      logger.dispose();
    });

    test('flush() genuinely awaits the network call, unlike log()',
        () async {
      final completer = Completer<Result<int>>();
      when(() => api.logBatch(any())).thenAnswer((_) => completer.future);
      final logger = EventLogger(
        api,
        flushInterval: const Duration(minutes: 10),
      );

      logger.log('view_question'); // must return without hanging

      final flushFuture = logger.flush();
      var flushCompleted = false;
      unawaited(flushFuture.then((_) => flushCompleted = true));

      // Let any already-ready microtasks run; the mocked call is still
      // pending on `completer`, so flush() must not have finished yet.
      await Future<void>.delayed(Duration.zero);
      expect(flushCompleted, isFalse);

      completer.complete(const Result.ok(1));
      await flushFuture;

      expect(flushCompleted, isTrue);
      expect(logger.queueForTesting, isEmpty);
      logger.dispose();
    });
  });

  group('flush()', () {
    test('sends queued events in order and clears the queue on success',
        () async {
      when(() => api.logBatch(any())).thenAnswer(
        (_) async => const Result.ok(2),
      );
      final logger = EventLogger(
        api,
        flushInterval: const Duration(minutes: 10),
      );

      logger.log('view_question', props: {'question_id': 'q1'});
      logger.log('answer', props: {'question_id': 'q1', 'correct': true});

      await logger.flush();

      final captured = verify(
        () => api.logBatch(captureAny()),
      ).captured.cast<List<ClientEvent>>();
      expect(captured, hasLength(1));
      expect(
        captured.single.map((e) => e.name).toList(),
        ['view_question', 'answer'],
      );
      expect(logger.queueForTesting, isEmpty);
      logger.dispose();
    });

    test('a queue of 150 events is split into batches of 100 and 50',
        () async {
      when(() => api.logBatch(any())).thenAnswer(
        (_) async => const Result.ok(0),
      );
      final logger = EventLogger(
        api,
        flushInterval: const Duration(minutes: 10),
      );

      for (var i = 0; i < 150; i++) {
        logger.log('event_$i');
      }

      await logger.flush();

      final captured = verify(
        () => api.logBatch(captureAny()),
      ).captured.cast<List<ClientEvent>>();
      expect(captured, hasLength(2));
      expect(captured[0], hasLength(100));
      expect(captured[1], hasLength(50));
      // Order preserved and no event dropped/duplicated across the split.
      expect(captured[0].first.name, 'event_0');
      expect(captured[0].last.name, 'event_99');
      expect(captured[1].first.name, 'event_100');
      expect(captured[1].last.name, 'event_149');
      expect(logger.queueForTesting, isEmpty);
      logger.dispose();
    });

    test('a failed flush does not lose queued events — they are retried '
        'on the next flush', () async {
      var callCount = 0;
      when(() => api.logBatch(any())).thenAnswer((_) async {
        callCount++;
        if (callCount == 1) {
          return const Result.err(
            Failure(code: 'network_error', message: 'connection refused'),
          );
        }
        return const Result.ok(2);
      });
      final logger = EventLogger(
        api,
        flushInterval: const Duration(minutes: 10),
      );

      logger.log('view_question');
      logger.log('answer');

      await logger.flush(); // fails — events must stay queued
      expect(logger.queueForTesting, hasLength(2));

      await logger.flush(); // succeeds this time
      expect(logger.queueForTesting, isEmpty);

      final captured = verify(
        () => api.logBatch(captureAny()),
      ).captured.cast<List<ClientEvent>>();
      expect(captured, hasLength(2));
      // Both attempts sent the same two events — nothing was lost or
      // duplicated across the failed + retried flush.
      expect(captured[0].map((e) => e.name).toList(), ['view_question', 'answer']);
      expect(captured[1].map((e) => e.name).toList(), ['view_question', 'answer']);
      logger.dispose();
    });

    test(
      'an EventsApi.logBatch that throws outright is swallowed and retries '
      'later, same as a Result.err',
      () async {
        var callCount = 0;
        when(() => api.logBatch(any())).thenAnswer((_) async {
          callCount++;
          if (callCount == 1) {
            throw StateError('unexpected');
          }
          return const Result.ok(1);
        });
        final logger = EventLogger(
          api,
          flushInterval: const Duration(minutes: 10),
        );

        logger.log('view_question');

        await logger.flush();
        expect(logger.queueForTesting, hasLength(1));

        await logger.flush();
        expect(logger.queueForTesting, isEmpty);
        logger.dispose();
      },
    );

    test('does nothing (no logBatch call) when the queue is empty',
        () async {
      final logger = EventLogger(
        api,
        flushInterval: const Duration(minutes: 10),
      );

      await logger.flush();

      verifyNever(() => api.logBatch(any()));
      logger.dispose();
    });

    test(
      'stops at the first failing batch and does not attempt further batches; '
      'failed batch events remain queued for retry',
      () async {
        var callCount = 0;
        when(() => api.logBatch(any())).thenAnswer((_) async {
          callCount++;
          // Batch 1 succeeds, batch 2 fails, batch 3 should never be called
          if (callCount == 1) {
            return const Result.ok(1);
          } else if (callCount == 2) {
            return const Result.err(
              Failure(code: 'server_error', message: 'internal error'),
            );
          }
          // If we somehow reach here, that's a test failure — batch 3 should
          // never be attempted when batch 2 fails.
          throw StateError('batch 3 should never be called');
        });

        final logger = EventLogger(
          api,
          flushInterval: const Duration(minutes: 10),
        );

        // Queue 250 events: batches will be [100, 100, 50]
        for (var i = 0; i < 250; i++) {
          logger.log('event_$i');
        }
        expect(logger.queueForTesting, hasLength(250));

        // First flush: batch 1 succeeds, batch 2 fails, batch 3 never called
        await logger.flush();

        // Verify exactly 2 calls were made (batch 1 succeeded, batch 2 failed,
        // batch 3 was never attempted)
        final captured1 = verify(
          () => api.logBatch(captureAny()),
        ).captured.cast<List<ClientEvent>>();
        expect(captured1, hasLength(2));
        expect(captured1[0], hasLength(100)); // batch 1: events 0-99
        expect(captured1[1], hasLength(100)); // batch 2: events 100-199

        // Events from batch 2 (100-199) and batch 3 (200-249) should remain
        // queued, i.e., the first 100 events should have been removed.
        expect(logger.queueForTesting, hasLength(150));
        expect(logger.queueForTesting.first.name, 'event_100');
        expect(logger.queueForTesting.last.name, 'event_249');

        // Reset mock for second flush to avoid the throw check
        reset(api);
        when(() => api.logBatch(any())).thenAnswer(
          (_) async => const Result.ok(1),
        );

        // Second flush: should retry batch 2 and then batch 3
        await logger.flush();

        final captured2 = verify(
          () => api.logBatch(captureAny()),
        ).captured.cast<List<ClientEvent>>();
        expect(captured2, hasLength(2));
        expect(captured2[0], hasLength(100)); // batch 2 retry: events 100-199
        expect(captured2[1], hasLength(50)); // batch 3: events 200-249
        expect(
          captured2[0].first.name,
          'event_100',
          reason: 'batch 2 retry should start from event_100',
        );
        expect(captured2[0].last.name, 'event_199');
        expect(captured2[1].first.name, 'event_200');
        expect(captured2[1].last.name, 'event_249');

        // Queue should now be empty
        expect(logger.queueForTesting, isEmpty);

        logger.dispose();
      },
    );
  });

  group('periodic auto-flush', () {
    test('fires on its own on the configured interval, without an explicit '
        'flush() call', () {
      when(() => api.logBatch(any())).thenAnswer(
        (_) async => const Result.ok(1),
      );

      fakeAsync((async) {
        final logger = EventLogger(
          api,
          flushInterval: const Duration(seconds: 20),
        );
        logger.log('view_question');

        // Not enough time has passed yet for the timer to fire.
        async.elapse(const Duration(seconds: 10));
        verifyNever(() => api.logBatch(any()));
        expect(logger.queueForTesting, hasLength(1));

        // Crossing the interval fires the timer, which flushes on its own.
        async.elapse(const Duration(seconds: 10));
        verify(() => api.logBatch(any())).called(1);
        expect(logger.queueForTesting, isEmpty);

        logger.dispose();
      });
    });

    test('keeps firing every interval, flushing whatever is queued each '
        'time', () {
      when(() => api.logBatch(any())).thenAnswer(
        (_) async => const Result.ok(1),
      );

      fakeAsync((async) {
        final logger = EventLogger(
          api,
          flushInterval: const Duration(seconds: 20),
        );

        logger.log('view_question');
        async.elapse(const Duration(seconds: 20));
        verify(() => api.logBatch(any())).called(1);

        logger.log('answer');
        async.elapse(const Duration(seconds: 20));
        verify(() => api.logBatch(any())).called(1);

        logger.dispose();
      });
    });

    test('dispose() cancels the timer so no further auto-flush happens',
        () {
      when(() => api.logBatch(any())).thenAnswer(
        (_) async => const Result.ok(1),
      );

      fakeAsync((async) {
        final logger = EventLogger(
          api,
          flushInterval: const Duration(seconds: 20),
        );
        logger.log('view_question');
        logger.dispose();

        async.elapse(const Duration(seconds: 40));

        verifyNever(() => api.logBatch(any()));
      });
    });
  });
}
