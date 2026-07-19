import 'package:avtotest_app/core/network/token_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  group('SharedPrefsTokenStorage', () {
    test('readAccess/readRefresh return null when nothing saved', () async {
      final storage = SharedPrefsTokenStorage();

      expect(await storage.readAccess(), isNull);
      expect(await storage.readRefresh(), isNull);
    });

    test('save persists access and refresh tokens', () async {
      final storage = SharedPrefsTokenStorage();

      await storage.save(access: 'access-1', refresh: 'refresh-1');

      expect(await storage.readAccess(), 'access-1');
      expect(await storage.readRefresh(), 'refresh-1');
    });

    test('save overwrites previously stored tokens', () async {
      final storage = SharedPrefsTokenStorage();

      await storage.save(access: 'access-1', refresh: 'refresh-1');
      await storage.save(access: 'access-2', refresh: 'refresh-2');

      expect(await storage.readAccess(), 'access-2');
      expect(await storage.readRefresh(), 'refresh-2');
    });

    test('clear removes stored tokens', () async {
      final storage = SharedPrefsTokenStorage();
      await storage.save(access: 'access-1', refresh: 'refresh-1');

      await storage.clear();

      expect(await storage.readAccess(), isNull);
      expect(await storage.readRefresh(), isNull);
    });
  });
}
