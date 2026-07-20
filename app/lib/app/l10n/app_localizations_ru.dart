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

  @override
  String get homeGreetingLabel => 'Добро пожаловать';

  @override
  String get navVariantsSubtitle => 'Официальные экзаменационные билеты';

  @override
  String get navPracticeSubtitle => 'Тренируйтесь по темам';

  @override
  String get navMistakesSubtitle => 'Работайте над своими ошибками';

  @override
  String get navStatsSubtitle => 'Отслеживайте свои результаты';

  @override
  String get authTagline => 'Подготовка к экзамену';

  @override
  String get authHeadline => 'Получите права легко!';

  @override
  String get phoneEntrySubtitle => 'Введите номер телефона, чтобы начать';

  @override
  String get otpHeadline => 'Введите код';

  @override
  String get practiceSetupTitle => 'Настройки практики';

  @override
  String get practiceSetupDescription =>
      'Для практики выберите категорию или знак дорожного движения (не оба, только один).';

  @override
  String get practiceTargetCategory => 'Категория';

  @override
  String get practiceTargetSign => 'Знак';

  @override
  String get practiceLoadCategoriesError => 'Не удалось загрузить категории.';

  @override
  String get practiceSelectCategory => 'Выберите категорию';

  @override
  String get practiceLoadSignsError => 'Не удалось загрузить знаки.';

  @override
  String get practiceSelectSign => 'Выберите знак';

  @override
  String get questionCountLabel => 'Количество вопросов';

  @override
  String get startButton => 'Начать';

  @override
  String get signsScreenTitle => 'Дорожные знаки';

  @override
  String get searchLabel => 'Поиск';

  @override
  String get groupCodeLabel => 'Код группы (необязательно)';

  @override
  String get signsLoadError => 'Не удалось загрузить список знаков.';

  @override
  String get signsEmptyState => 'Знаки не найдены.';

  @override
  String get mistakesScreenTitle => 'Работа над ошибками';

  @override
  String get mistakesScreenDescription =>
      'Повторно отработайте вопросы, на которые вы ответили неправильно — вопросы автоматически выбираются системой.';

  @override
  String get variantsScreenTitle => 'Билеты';

  @override
  String get variantsLoadError => 'Не удалось загрузить список билетов.';

  @override
  String get variantsEmptyState => 'Билеты недоступны.';

  @override
  String get lockedLabel => 'Заблокировано';

  @override
  String get sessionQuestionLoadError => 'Не удалось загрузить вопрос.';

  @override
  String get sessionTitleExam => 'Экзамен';

  @override
  String get sessionTitleVariant => 'Билет';

  @override
  String get sessionTitlePractice => 'Практика';

  @override
  String get sessionTitleMistakes => 'Работа над ошибками';

  @override
  String get sessionTitleDefault => 'Тест';

  @override
  String sessionProgressLabel(int current, int total) {
    return 'Вопрос $current / $total';
  }

  @override
  String get sessionFinishButton => 'Завершить';

  @override
  String get sessionNextButton => 'Далее';

  @override
  String get sessionVipRequiredError =>
      'Для этого раздела нужна активная подписка. Подписка пока недоступна в этой версии.';

  @override
  String get sessionDailyLimitError =>
      'Вы достигли сегодняшнего бесплатного лимита. Продолжить можно завтра.';

  @override
  String get vipRequiredTitle => 'Премиум-раздел';

  @override
  String get vipRequiredHeadline => 'Этот раздел доступен только по подписке';

  @override
  String get vipRequiredBody =>
      'Для доступа к этому разделу нужна активная подписка (Premium). В бесплатном режиме открыты первый билет и некоторые разделы.';

  @override
  String get vipRequiredPurchaseUnavailable =>
      'Покупка подписки пока недоступна в этой версии — оплата появится на следующем этапе.';

  @override
  String get homeButton => 'Главная';

  @override
  String get sessionStatusPassedLabel => 'Вы сдали';

  @override
  String get sessionStatusFailedLabel => 'Вы не сдали';

  @override
  String get sessionResultViewAbandonedLabel => 'Сессия остановлена';

  @override
  String get sessionResultsAbandonedLabel => 'Сессия не завершена';

  @override
  String get sessionReasonCompletedLabel => 'Все вопросы пройдены';

  @override
  String get sessionReasonTimeUpLabel => 'Время вышло';

  @override
  String get sessionReasonTooManyErrorsLabel => 'Количество ошибок превышено';

  @override
  String get sessionResultTitle => 'Результат';

  @override
  String get sessionAnswerRetryMessage =>
      'Не удалось отправить ваш ответ. Попробуйте ещё раз.';
}
