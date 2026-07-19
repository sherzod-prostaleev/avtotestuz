import 'package:freezed_annotation/freezed_annotation.dart';

part 'profile.freezed.dart';

/// The authenticated user's own profile, matching `GET /me`'s `data.profile`
/// shape exactly as returned by `backend/internal/account/handlers.go`'s
/// `profileDTO`/`toProfileDTO` (confirmed by reading that file directly, not
/// just README prose):
/// ```
/// {
///   "id": string,                  // profile UUID
///   "phone": string,               // normalized "+998XXXXXXXXX"
///   "name": string,
///   "region": string,
///   "district": string,
///   "birth_date": string | null,   // "YYYY-MM-DD", null if never set
///   "locale_pref": string,         // "uz-Latn" | "uz-Cyrl" | "ru"
///   "theme_pref": string,
///   "referral_code": string,      // always present, may be "" (pgtype.Text
///                                  // .String — not a JSON-nullable field)
///   "role": string,
///   "created_at": string           // RFC3339
/// }
/// ```
@freezed
abstract class Profile with _$Profile {
  const factory Profile({
    required String id,
    required String phone,
    required String name,
    required String region,
    required String district,
    DateTime? birthDate,
    required String localePref,
    required String themePref,
    required String referralCode,
    required String role,
    required DateTime createdAt,
  }) = _Profile;
}

/// VIP entitlement status, matching both `GET /me`'s `data.vip` and
/// `GET /me/entitlement`'s `data` shape — both produced by the same
/// `toVIPDTO` helper in `backend/internal/account/handlers.go`:
/// ```
/// { "active": bool, "until": string | null }   // "until": RFC3339
/// ```
@freezed
abstract class Entitlement with _$Entitlement {
  const factory Entitlement({required bool active, DateTime? until}) =
      _Entitlement;
}
