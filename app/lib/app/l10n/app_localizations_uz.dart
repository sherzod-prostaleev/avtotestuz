// ignore: unused_import
import 'package:intl/intl.dart' as intl;
import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for Uzbek (`uz`).
class AppLocalizationsUz extends AppLocalizations {
  AppLocalizationsUz([String locale = 'uz']) : super(locale);

  @override
  String get appTitle => 'AvtoTest';

  @override
  String get phoneLabel => 'Telefon raqami';

  @override
  String get continueButton => 'Davom etish';

  @override
  String get otpLabel => 'Tasdiqlash kodi';

  @override
  String get verifyButton => 'Tasdiqlash';

  @override
  String get logout => 'Chiqish';

  @override
  String get errorGeneric => 'Xatolik yuz berdi';
}

/// The translations for Uzbek, using the Cyrillic script (`uz_Cyrl`).
class AppLocalizationsUzCyrl extends AppLocalizationsUz {
  AppLocalizationsUzCyrl() : super('uz_Cyrl');

  @override
  String get appTitle => 'АвтоТест';

  @override
  String get phoneLabel => 'Телефон рақами';

  @override
  String get continueButton => 'Давом этиш';

  @override
  String get otpLabel => 'Тасдиқлаш коди';

  @override
  String get verifyButton => 'Тасдиқлаш';

  @override
  String get logout => 'Чиқиш';

  @override
  String get errorGeneric => 'Хатолик юз берди';
}
