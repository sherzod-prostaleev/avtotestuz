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

  @override
  String get phoneInvalidError => 'Telefon raqami noto\'g\'ri formatda';

  @override
  String devCodeCaption(String code) {
    return 'Dev kod: $code';
  }

  @override
  String phoneConfirmationLabel(String phone) {
    return 'Telefon: $phone';
  }

  @override
  String get resendButton => 'Qayta yuborish';

  @override
  String resendIn(int seconds) {
    return 'Qayta yuborish (${seconds}s)';
  }
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

  @override
  String get phoneInvalidError => 'Телефон рақами нотўғри форматда';

  @override
  String devCodeCaption(String code) {
    return 'Dev код: $code';
  }

  @override
  String phoneConfirmationLabel(String phone) {
    return 'Телефон: $phone';
  }

  @override
  String get resendButton => 'Қайта юбориш';

  @override
  String resendIn(int seconds) {
    return 'Қайта юбориш ($secondsс)';
  }
}
