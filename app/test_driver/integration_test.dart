// Driver entrypoint for `flutter drive` against
// `integration_test/auth_flow_test.dart`. `flutter test -d chrome` does not
// support web devices for `integration_test` in this Flutter version
// ("Web devices are not supported for integration tests yet."), so the
// standard `flutter drive` web pattern is used instead:
//
//   flutter drive \
//     --driver=test_driver/integration_test.dart \
//     --target=integration_test/auth_flow_test.dart \
//     -d chrome \
//     --dart-define=API_BASE_URL=http://localhost:8090/api/v1
import 'package:integration_test/integration_test_driver.dart';

Future<void> main() => integrationDriver();
