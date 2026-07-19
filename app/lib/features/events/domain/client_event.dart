import 'package:freezed_annotation/freezed_annotation.dart';

part 'client_event.freezed.dart';

/// A single client-reported analytics event, matching
/// `backend/internal/events/service.go`'s `Event`/`eventDTO` shape exactly:
/// ```
/// { "name": string, "props": object?, "ts": string? }   // "ts": RFC3339
/// ```
/// `props` is an arbitrary free-form JSON object (the backend defaults a
/// missing/empty `props` to `{}` at the DB layer — this client sends it
/// omitted entirely when null, same "don't send absent filters" convention
/// as `features/content/data/content_api.dart`'s `group`/`q`). `ts` is
/// likewise omitted when null — the backend defaults to `now()` server-side.
@freezed
abstract class ClientEvent with _$ClientEvent {
  const factory ClientEvent({
    required String name,
    Map<String, dynamic>? props,
    DateTime? ts,
  }) = _ClientEvent;
}
