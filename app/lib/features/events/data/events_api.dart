import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/result.dart';
import '../domain/client_event.dart';

/// Supplies the [EventsApi] [EventLogger] (see
/// `features/events/presentation/event_logger.dart`) sends batched client
/// events through. Has no usable default — a real instance (backed by the
/// same configured [Dio] `main.dart` already built for
/// `AuthApi`/`ProfileApi`/`ContentApi`) is wired at the app root, same
/// "no default, overridden at the app root" seam as `contentApiProvider`.
/// Tests override this directly with a mock.
final eventsApiProvider = Provider<EventsApi>((ref) {
  throw UnimplementedError(
    'eventsApiProvider has no default — override it at the app root with '
    'a real EventsApi.',
  );
});

/// Thin [Dio] wrapper around `backend/internal/events/handlers.go`'s
/// `POST /events` batch-ingestion endpoint. Every call is wrapped in [guard]
/// so raw [DioException]s never leak past this layer (same convention as
/// `features/content/data/content_api.dart`).
///
/// Request/response shape confirmed by reading
/// `backend/internal/events/handlers.go`/`service.go` directly (not just
/// README prose):
/// ```
/// POST /events {"events": [{"name": string, "props": object?, "ts": string?}, ...]}
///   -> {"data": {"ok": true, "count": <n>}}
/// ```
/// wrapped in the standard envelope (`backend/internal/httpx`). A batch must
/// have 1-100 events — an empty or >100 batch is rejected with `400
/// invalid_request` (`ErrInvalidRequest` in `service.go`); it is
/// [EventLogger]'s job to never send a batch outside that range.
class EventsApi {
  EventsApi(this._dio);

  final Dio _dio;

  Future<Result<int>> logBatch(List<ClientEvent> events) {
    return guard(() async {
      final response = await _dio.post<Map<String, dynamic>>(
        '/events',
        data: {'events': events.map(_eventToJson).toList()},
      );
      final data = response.data!['data'] as Map<String, dynamic>;
      return data['count'] as int;
    });
  }
}

Map<String, dynamic> _eventToJson(ClientEvent event) {
  return {
    'name': event.name,
    if (event.props != null) 'props': event.props,
    if (event.ts != null) 'ts': event.ts!.toUtc().toIso8601String(),
  };
}
