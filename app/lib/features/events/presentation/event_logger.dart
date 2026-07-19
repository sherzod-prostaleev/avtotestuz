import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/result.dart';
import '../data/events_api.dart';
import '../domain/client_event.dart';

/// How often [EventLogger] auto-flushes its local queue. Chosen as a
/// reasonable middle ground for an analytics-style, non-latency-sensitive
/// stream: frequent enough that events don't sit unsent for long if the app
/// stays open, infrequent enough not to spam `POST /events` on every single
/// `log()` call.
const eventLoggerFlushInterval = Duration(seconds: 20);

/// The backend's per-request cap (`backend/internal/events/service.go`'s
/// `maxBatchSize`) — an empty or >100-event batch is rejected with `400
/// invalid_request`. [EventLogger.flush] splits its queue into chunks of at
/// most this size rather than ever sending (or dropping) more.
const _maxBatchSize = 100;

/// Supplies the app's single [EventLogger] instance. Built with a real
/// [EventsApi] (via [eventsApiProvider], itself wired at the app root — see
/// that provider's doc comment), so no override is needed in production; the
/// periodic auto-flush [Timer] started in [EventLogger]'s constructor is
/// cancelled via [Ref.onDispose] when this provider is disposed/overridden,
/// e.g. in tests.
final eventLoggerProvider = Provider<EventLogger>((ref) {
  final logger = EventLogger(ref.watch(eventsApiProvider));
  ref.onDispose(logger.dispose);
  return logger;
});

/// Client-side batching logger for analytics-style events (`view_question`,
/// `answer`, `session_finish`, ...) sent to `POST /events`
/// (`features/events/data/events_api.dart`).
///
/// [log] is fire-and-forget: it only appends to an in-memory queue and
/// returns immediately — it never awaits the network and never throws, so a
/// slow/broken connection can never affect the learning experience. The
/// queue is drained by [flush], which this class also calls on a periodic
/// [Timer] (every [flushInterval]) so a caller doesn't have to remember to
/// flush manually, though callers are free to call [flush] explicitly too
/// (e.g. on a session finishing) for lower latency in the common case.
///
/// A failed [flush] (network error, non-2xx, or the [EventsApi] call
/// throwing outright) is swallowed — never rethrown — and every event that
/// wasn't confirmedly sent stays queued for the next flush cycle (periodic
/// or manual). Events are never dropped just because a flush attempt
/// failed; the only way the queue shrinks is a successful `logBatch` for
/// the events at its front.
///
/// Call [dispose] when the logger is no longer needed to cancel its
/// periodic timer (handled automatically for [eventLoggerProvider] via
/// `ref.onDispose`).
class EventLogger {
  EventLogger(this._api, {this.flushInterval = eventLoggerFlushInterval}) {
    _timer = Timer.periodic(flushInterval, (_) => flush());
  }

  final EventsApi _api;

  /// How often this instance auto-flushes; exposed read-only mainly for
  /// tests to reason about (defaults to [eventLoggerFlushInterval]).
  final Duration flushInterval;
  late final Timer _timer;

  final List<ClientEvent> _queue = [];

  /// Read-only view of what's currently queued — exposed for tests only
  /// (production code has no legitimate reason to inspect the queue).
  @visibleForTesting
  List<ClientEvent> get queueForTesting => List.unmodifiable(_queue);

  /// Enqueues an event for later sending. Never blocks the caller and never
  /// throws — the actual network call happens later, inside [flush].
  void log(String name, {Map<String, dynamic>? props}) {
    _queue.add(ClientEvent(name: name, props: props, ts: DateTime.now()));
  }

  /// Sends everything currently queued, splitting into chunks of at most
  /// [_maxBatchSize] events (the backend's per-request cap). Chunks are sent
  /// in order, front of the queue first; the first chunk that fails to send
  /// stops the flush (remaining chunks, including the failed one, stay
  /// queued for the next attempt) so ordering is preserved and nothing already
  /// confirmed-sent is ever resent.
  ///
  /// Never throws: a failed `logBatch` call (network error, non-2xx, or an
  /// unexpected exception from [_api]) is swallowed here, optionally
  /// `debugPrint`ed in debug builds, and simply leaves the un-sent events in
  /// the queue for the next flush cycle.
  Future<void> flush() async {
    if (_queue.isEmpty) return;

    // Snapshot what's queued right now — anything `log()`ed while this
    // flush's `await`s are in flight is appended to `_queue` after this
    // snapshot and is left untouched below.
    final pending = List<ClientEvent>.of(_queue);
    var sentCount = 0;

    for (var start = 0; start < pending.length; start += _maxBatchSize) {
      final end = (start + _maxBatchSize < pending.length)
          ? start + _maxBatchSize
          : pending.length;
      final batch = pending.sublist(start, end);

      final ok = await _sendBatch(batch);
      if (!ok) break;
      sentCount = end;
    }

    if (sentCount > 0) {
      _queue.removeRange(0, sentCount);
    }
  }

  /// Sends a single batch (already within the 1-100 cap), returning whether
  /// it was sent successfully. Never throws.
  Future<bool> _sendBatch(List<ClientEvent> batch) async {
    try {
      final result = await _api.logBatch(batch);
      switch (result) {
        case Ok():
          return true;
        case Err(:final failure):
          if (kDebugMode) {
            debugPrint(
              'EventLogger: flush failed (${failure.code}: '
              '${failure.message}) — ${batch.length} event(s) retained for '
              'the next flush.',
            );
          }
          return false;
      }
    } catch (e) {
      if (kDebugMode) {
        debugPrint(
          'EventLogger: flush threw ($e) — ${batch.length} event(s) '
          'retained for the next flush.',
        );
      }
      return false;
    }
  }

  /// Cancels the periodic auto-flush timer. Queued-but-unsent events are
  /// simply dropped along with this instance — there is no "flush on
  /// dispose" here since disposal isn't necessarily a natural boundary (e.g.
  /// hot-reload/test teardown), matching the brief's "periodic + on-finish
  /// is enough for this foundation pass" scope.
  void dispose() {
    _timer.cancel();
  }
}
