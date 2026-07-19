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

  @override
  String get comingSoon => 'Tez orada';

  @override
  String get navVariantsLabel => 'Variantlar';

  @override
  String get navPracticeLabel => 'Mashq qilish';

  @override
  String get navMistakesLabel => 'Xatolar ustida ishlash';

  @override
  String get navStatsLabel => 'Statistika';

  @override
  String get vipActiveLabel => 'VIP: faol';

  @override
  String get vipInactiveLabel => 'VIP: faol emas';

  @override
  String get retryButton => 'Qayta urinish';

  @override
  String get themeToggleTooltip => 'Mavzuni almashtirish';

  @override
  String get profileLoadError => 'Profil ma\'lumotlarini yuklab bo\'lmadi';
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

  @override
  String get comingSoon => 'Тез орада';

  @override
  String get navVariantsLabel => 'Вариантлар';

  @override
  String get navPracticeLabel => 'Машқ қилиш';

  @override
  String get navMistakesLabel => 'Хатолар устида ишлаш';

  @override
  String get navStatsLabel => 'Статистика';

  @override
  String get vipActiveLabel => 'VIP: фаол';

  @override
  String get vipInactiveLabel => 'VIP: фаол эмас';

  @override
  String get retryButton => 'Қайта уриниш';

  @override
  String get themeToggleTooltip => 'Мавзуни алмаштириш';

  @override
  String get profileLoadError => 'Профил маълумотларини юклаб бўлмади';
}
