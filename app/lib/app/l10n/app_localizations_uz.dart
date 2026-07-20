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

  @override
  String get practiceSetupTitle => 'Mashq sozlamalari';

  @override
  String get practiceSetupDescription =>
      'Mashq qilish uchun kategoriya YOKI belgini tanlang (ikkalasi emas, faqat bittasi).';

  @override
  String get practiceTargetCategory => 'Kategoriya';

  @override
  String get practiceTargetSign => 'Belgi';

  @override
  String get practiceLoadCategoriesError => 'Kategoriyalarni yuklab bo\'lmadi.';

  @override
  String get practiceSelectCategory => 'Kategoriyani tanlang';

  @override
  String get practiceLoadSignsError => 'Belgilarni yuklab bo\'lmadi.';

  @override
  String get practiceSelectSign => 'Belgini tanlang';

  @override
  String get questionCountLabel => 'Savollar soni';

  @override
  String get startButton => 'Boshlash';

  @override
  String get signsScreenTitle => 'Yo\'l belgilari';

  @override
  String get searchLabel => 'Qidiruv';

  @override
  String get groupCodeLabel => 'Guruh kodi (ixtiyoriy)';

  @override
  String get signsLoadError => 'Belgilar ro\'yxatini yuklab bo\'lmadi.';

  @override
  String get signsEmptyState => 'Hech qanday belgi topilmadi.';

  @override
  String get mistakesScreenTitle => 'Xatolar ustida ishlash';

  @override
  String get mistakesScreenDescription =>
      'Avval noto\'g\'ri javob bergan savollaringiz ustida qayta ishlang — savollar tizim tomonidan avtomatik tanlanadi.';

  @override
  String get variantsScreenTitle => 'Biletlar';

  @override
  String get variantsLoadError => 'Biletlar ro\'yxatini yuklab bo\'lmadi.';

  @override
  String get variantsEmptyState => 'Hozircha biletlar mavjud emas.';

  @override
  String get lockedLabel => 'Yopiq';

  @override
  String get sessionQuestionLoadError => 'Savolni yuklab bo\'lmadi.';

  @override
  String get sessionTitleExam => 'Imtihon';

  @override
  String get sessionTitleVariant => 'Bilet';

  @override
  String get sessionTitlePractice => 'Mashq';

  @override
  String get sessionTitleMistakes => 'Xatolar ustida ishlash';

  @override
  String get sessionTitleDefault => 'Test';

  @override
  String sessionProgressLabel(int current, int total) {
    return 'Savol $current / $total';
  }

  @override
  String get sessionFinishButton => 'Yakunlash';

  @override
  String get sessionNextButton => 'Keyingi';

  @override
  String get sessionVipRequiredError =>
      'Bu bo\'lim uchun faol obuna kerak. Obuna hozircha bu versiyada mavjud emas.';

  @override
  String get sessionDailyLimitError =>
      'Bugungi bepul limitga yetdingiz. Ertaga yana davom etishingiz mumkin.';

  @override
  String get vipRequiredTitle => 'Premium bo\'lim';

  @override
  String get vipRequiredHeadline => 'Bu bo\'lim faqat obunachilar uchun';

  @override
  String get vipRequiredBody =>
      'Bu bo\'limdan foydalanish uchun faol obuna (Premium) kerak. Bepul rejada birinchi bilet va ba\'zi bo\'limlar ochiq.';

  @override
  String get vipRequiredPurchaseUnavailable =>
      'Obunani xarid qilish hozircha bu versiyada mavjud emas — to\'lov tizimi keyingi bosqichda qo\'shiladi.';

  @override
  String get homeButton => 'Bosh sahifa';

  @override
  String get sessionStatusPassedLabel => 'O\'tdingiz';

  @override
  String get sessionStatusFailedLabel => 'O\'ta olmadingiz';

  @override
  String get sessionResultViewAbandonedLabel => 'Sessiya to\'xtatildi';

  @override
  String get sessionResultsAbandonedLabel => 'Sessiya tugallanmadi';

  @override
  String get sessionReasonCompletedLabel => 'Barcha savollar yakunlandi';

  @override
  String get sessionReasonTimeUpLabel => 'Vaqt tugadi';

  @override
  String get sessionReasonTooManyErrorsLabel => 'Xatolar soni ko\'payib ketdi';

  @override
  String get sessionResultTitle => 'Natija';

  @override
  String get sessionAnswerRetryMessage =>
      'Javobingizni yuborib bo\'lmadi. Qayta urinib ko\'ring.';

  @override
  String get savedEmptyHint =>
      'Yoqtirgan savollaringizni saqlang — ular shu yerda to\'planadi.';
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

  @override
  String get practiceSetupTitle => 'Машқ созламалари';

  @override
  String get practiceSetupDescription =>
      'Машқ қилиш учун категория ЙОКИ бельгини танланг (иккаласи эмас, фақат биттаси).';

  @override
  String get practiceTargetCategory => 'Категория';

  @override
  String get practiceTargetSign => 'Бельги';

  @override
  String get practiceLoadCategoriesError => 'Категорияларни юклаб бўлмади.';

  @override
  String get practiceSelectCategory => 'Категорияни танланг';

  @override
  String get practiceLoadSignsError => 'Бельгиларни юклаб бўлмади.';

  @override
  String get practiceSelectSign => 'Бельгини танланг';

  @override
  String get questionCountLabel => 'Саволлар сони';

  @override
  String get startButton => 'Бошлаш';

  @override
  String get signsScreenTitle => 'Йўл бельгилари';

  @override
  String get searchLabel => 'Қидирув';

  @override
  String get groupCodeLabel => 'Гуруҳ коди (ихтиёрий)';

  @override
  String get signsLoadError => 'Бельгилар рўйхатини юклаб бўлмади.';

  @override
  String get signsEmptyState => 'Ҳеч қандай бельги топилмади.';

  @override
  String get mistakesScreenTitle => 'Хатолар устида ишлаш';

  @override
  String get mistakesScreenDescription =>
      'Аввал нотўғри жавоб берган саволларингиз устида қайта ишланг — саволлар тизим томонидан автоматик танланади.';

  @override
  String get variantsScreenTitle => 'Билетлар';

  @override
  String get variantsLoadError => 'Билетлар рўйхатини юклаб бўлмади.';

  @override
  String get variantsEmptyState => 'Ҳозирча билетлар мавжуд эмас.';

  @override
  String get lockedLabel => 'Ёпиқ';

  @override
  String get sessionQuestionLoadError => 'Саволни юклаб бўлмади.';

  @override
  String get sessionTitleExam => 'Имтиҳон';

  @override
  String get sessionTitleVariant => 'Билет';

  @override
  String get sessionTitlePractice => 'Машқ';

  @override
  String get sessionTitleMistakes => 'Хатолар устида ишлаш';

  @override
  String get sessionTitleDefault => 'Тест';

  @override
  String sessionProgressLabel(int current, int total) {
    return 'Савол $current / $total';
  }

  @override
  String get sessionFinishButton => 'Якунлаш';

  @override
  String get sessionNextButton => 'Кейинги';

  @override
  String get sessionVipRequiredError =>
      'Бу бўлим учун фаол обуна керак. Обуна ҳозирча бу версияда мавжуд эмас.';

  @override
  String get sessionDailyLimitError =>
      'Бугунги бепул лимитга етдингиз. Эртага яна давом этишингиз мумкин.';

  @override
  String get vipRequiredTitle => 'Премиум бўлим';

  @override
  String get vipRequiredHeadline => 'Бу бўлим фақат обуначилар учун';

  @override
  String get vipRequiredBody =>
      'Бу бўлимдан фойдаланиш учун фаол обуна (Premium) керак. Бепул режада биринчи билет ва баъзи бўлимлар очиқ.';

  @override
  String get vipRequiredPurchaseUnavailable =>
      'Обунани харид қилиш ҳозирча бу версияда мавжуд эмас — тўлов тизими кейинги босқичда қўшилади.';

  @override
  String get homeButton => 'Бош саҳифа';

  @override
  String get sessionStatusPassedLabel => 'Ўтдингиз';

  @override
  String get sessionStatusFailedLabel => 'Ўта олмадингиз';

  @override
  String get sessionResultViewAbandonedLabel => 'Сессия тўхтатилди';

  @override
  String get sessionResultsAbandonedLabel => 'Сессия якунланмади';

  @override
  String get sessionReasonCompletedLabel => 'Барча саволлар якунланди';

  @override
  String get sessionReasonTimeUpLabel => 'Вақт тугади';

  @override
  String get sessionReasonTooManyErrorsLabel => 'Хатолар сони кўпайиб кетди';

  @override
  String get sessionResultTitle => 'Натижа';

  @override
  String get sessionAnswerRetryMessage =>
      'Жавобингизни юбориб бўлмади. Қайта уриниб кўринг.';

  @override
  String get savedEmptyHint =>
      'Ёқтирган саволларингизни сақланг — улар шу ерда тўпланади.';
}
