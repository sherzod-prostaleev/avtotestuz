import 'package:dio/dio.dart';

import '../../../core/result.dart';
import '../domain/profile.dart';

/// Thin [Dio] wrapper around `backend/internal/account/handlers.go`'s `/me`
/// and `/me/entitlement` endpoints. Every call is wrapped in [guard] so raw
/// [DioException]s never leak past this layer (same convention as
/// `features/auth/data/auth_api.dart`).
///
/// Response shapes (confirmed by reading `handlers.go` directly):
/// ```
/// GET /me
///   -> {"profile": <profileDTO>, "vip": <vipDTO>}
/// GET /me/entitlement
///   -> <vipDTO>
/// ```
/// both wrapped in the standard `{"data": ...}` envelope
/// (`backend/internal/httpx`). [fetchMe] only extracts the `profile` half of
/// `GET /me`'s response — [fetchEntitlement] is the dedicated call for VIP
/// status, matching `ProfileController.build()`'s two concurrent requests.
class ProfileApi {
  ProfileApi(this._dio);

  final Dio _dio;

  Future<Result<Profile>> fetchMe() {
    return guard(() async {
      final response = await _dio.get<Map<String, dynamic>>('/me');
      final data = response.data!['data'] as Map<String, dynamic>;
      final profile = data['profile'] as Map<String, dynamic>;
      return _profileFromJson(profile);
    });
  }

  Future<Result<Entitlement>> fetchEntitlement() {
    return guard(() async {
      final response = await _dio.get<Map<String, dynamic>>(
        '/me/entitlement',
      );
      final data = response.data!['data'] as Map<String, dynamic>;
      return _entitlementFromJson(data);
    });
  }
}

Profile _profileFromJson(Map<String, dynamic> json) {
  final birthDate = json['birth_date'] as String?;
  return Profile(
    id: json['id'] as String,
    phone: json['phone'] as String,
    name: json['name'] as String,
    region: json['region'] as String,
    district: json['district'] as String,
    birthDate: birthDate == null ? null : DateTime.parse(birthDate),
    localePref: json['locale_pref'] as String,
    themePref: json['theme_pref'] as String,
    referralCode: json['referral_code'] as String,
    role: json['role'] as String,
    createdAt: DateTime.parse(json['created_at'] as String),
  );
}

Entitlement _entitlementFromJson(Map<String, dynamic> json) {
  final until = json['until'] as String?;
  return Entitlement(
    active: json['active'] as bool,
    until: until == null ? null : DateTime.parse(until),
  );
}
