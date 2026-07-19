// ignore: unused_import
import 'package:intl/intl.dart' as intl;
import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for Russian (`ru`).
class AppLocalizationsRu extends AppLocalizations {
  AppLocalizationsRu([String locale = 'ru']) : super(locale);

  @override
  String get appTitle => 'АвтоТест';

  @override
  String get phoneLabel => 'Номер телефона';

  @override
  String get continueButton => 'Продолжить';

  @override
  String get otpLabel => 'Код подтверждения';

  @override
  String get verifyButton => 'Подтвердить';

  @override
  String get logout => 'Выйти';

  @override
  String get errorGeneric => 'Произошла ошибка';

  @override
  String get phoneInvalidError => 'Неверный формат номера телефона';

  @override
  String devCodeCaption(String code) {
    return 'Dev-код: $code';
  }

  @override
  String phoneConfirmationLabel(String phone) {
    return 'Телефон: $phone';
  }

  @override
  String get resendButton => 'Отправить повторно';

  @override
  String resendIn(int seconds) {
    return 'Повторная отправка через $secondsс';
  }

  @override
  String get comingSoon => 'Скоро';

  @override
  String get navVariantsLabel => 'Варианты';

  @override
  String get navPracticeLabel => 'Практика';

  @override
  String get navMistakesLabel => 'Работа над ошибками';

  @override
  String get navStatsLabel => 'Статистика';

  @override
  String get vipActiveLabel => 'VIP: активен';

  @override
  String get vipInactiveLabel => 'VIP: не активен';

  @override
  String get retryButton => 'Повторить';

  @override
  String get themeToggleTooltip => 'Сменить тему';

  @override
  String get profileLoadError => 'Не удалось загрузить данные профиля';
}
