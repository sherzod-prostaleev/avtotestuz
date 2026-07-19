// Driver entrypoint for `flutter drive` against
// `integration_test/auth_flow_test.dart`. `flutter test -d chrome` does not
// support web devices for `integration_test` in this Flutter version
// ("Web devices are not supported for integration tests yet."), and
// `flutter drive -d chrome` also fails (it launches Chrome via DWDS
// directly, which can't carry the integration_test result channel) — so an
// external WebDriver-controlled browser is required instead:
//
//   chromedriver --port=4444   # in a separate terminal, version matching
//                              # `google-chrome-stable --version` exactly
//   flutter drive \
//     --driver=test_driver/integration_test.dart \
//     --target=integration_test/auth_flow_test.dart \
//     -d web-server --web-port=7357 --browser-name=chrome \
//     --dart-define=API_BASE_URL=http://localhost:8090/api/v1
//
// See integration_test/auth_flow_test.dart's header for the full rationale.
import 'package:integration_test/integration_test_driver.dart';

Future<void> main() => integrationDriver();
