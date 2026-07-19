import 'package:dio/dio.dart';
import 'package:freezed_annotation/freezed_annotation.dart';

part 'result.freezed.dart';

/// The only shape network errors are allowed to take once they cross out of
/// `core/network` — no raw [DioException] should ever leak into `features/*`.
@freezed
sealed class Result<T> with _$Result<T> {
  const factory Result.ok(T data) = Ok<T>;
  const factory Result.err(Failure failure) = Err<T>;
}

/// - `code`: the backend's `error.code` when the server returned a
///   structured `{"error":{"code":...,"message":...}}` envelope (e.g.
///   `"invalid_code"`, `"vip_required"`), or a client-side sentinel
///   (`"network_error"`, `"unknown"`) otherwise.
/// - `message`: a human-readable message, from the backend when available.
@freezed
abstract class Failure with _$Failure {
  const factory Failure({required String code, required String message}) =
      _Failure;
}

/// Wraps a Dio call, turning any [DioException] (or other error) into a
/// [Result.err] instead of letting it propagate as an exception.
Future<Result<T>> guard<T>(Future<T> Function() call) async {
  try {
    final data = await call();
    return Result.ok(data);
  } on DioException catch (e) {
    return Result.err(_failureFromDioException(e));
  } catch (e) {
    return Result.err(Failure(code: 'unknown', message: e.toString()));
  }
}

Failure _failureFromDioException(DioException e) {
  final envelope = _errorEnvelope(e.response?.data);
  if (envelope != null) {
    return envelope;
  }

  switch (e.type) {
    case DioExceptionType.connectionTimeout:
    case DioExceptionType.sendTimeout:
    case DioExceptionType.receiveTimeout:
    case DioExceptionType.transformTimeout:
    case DioExceptionType.connectionError:
      return Failure(
        code: 'network_error',
        message: e.message ?? 'A network error occurred.',
      );
    case DioExceptionType.badCertificate:
    case DioExceptionType.badResponse:
    case DioExceptionType.cancel:
    case DioExceptionType.unknown:
      return Failure(code: 'unknown', message: e.toString());
  }
}

/// Extracts `{code, message}` from a `{"error":{"code":...,"message":...}}`
/// response body, or returns null if `data` doesn't match that shape.
Failure? _errorEnvelope(Object? data) {
  if (data is! Map) return null;
  final error = data['error'];
  if (error is! Map) return null;
  final code = error['code'];
  final message = error['message'];
  if (code is! String || message is! String) return null;
  return Failure(code: code, message: message);
}
