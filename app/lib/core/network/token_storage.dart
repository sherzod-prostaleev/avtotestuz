import 'package:shared_preferences/shared_preferences.dart';

/// Persists the access/refresh JWT pair. The backend returns these as JSON
/// body fields (not cookies) from `/auth/otp/verify` and `/auth/refresh` —
/// storage and refresh scheduling are the client's responsibility.
abstract class TokenStorage {
  Future<String?> readAccess();
  Future<String?> readRefresh();
  Future<void> save({required String access, required String refresh});
  Future<void> clear();
}

/// [TokenStorage] backed by `shared_preferences` (web-compatible via
/// localStorage). An explicit, documented trade-off for M1 web — revisit
/// with secure storage when M6 adds native mobile targets.
class SharedPrefsTokenStorage implements TokenStorage {
  static const _accessKey = 'auth.access_token';
  static const _refreshKey = 'auth.refresh_token';

  @override
  Future<String?> readAccess() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString(_accessKey);
  }

  @override
  Future<String?> readRefresh() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString(_refreshKey);
  }

  @override
  Future<void> save({required String access, required String refresh}) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_accessKey, access);
    await prefs.setString(_refreshKey, refresh);
  }

  @override
  Future<void> clear() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_accessKey);
    await prefs.remove(_refreshKey);
  }
}
