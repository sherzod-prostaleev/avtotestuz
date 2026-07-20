import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:intl/intl.dart' as intl;

import 'app_localizations_ru.dart';
import 'app_localizations_uz.dart';

// ignore_for_file: type=lint

/// Callers can lookup localized strings with an instance of AppLocalizations
/// returned by `AppLocalizations.of(context)`.
///
/// Applications need to include `AppLocalizations.delegate()` in their app's
/// `localizationDelegates` list, and the locales they support in the app's
/// `supportedLocales` list. For example:
///
/// ```dart
/// import 'l10n/app_localizations.dart';
///
/// return MaterialApp(
///   localizationsDelegates: AppLocalizations.localizationsDelegates,
///   supportedLocales: AppLocalizations.supportedLocales,
///   home: MyApplicationHome(),
/// );
/// ```
///
/// ## Update pubspec.yaml
///
/// Please make sure to update your pubspec.yaml to include the following
/// packages:
///
/// ```yaml
/// dependencies:
///   # Internationalization support.
///   flutter_localizations:
///     sdk: flutter
///   intl: any # Use the pinned version from flutter_localizations
///
///   # Rest of dependencies
/// ```
///
/// ## iOS Applications
///
/// iOS applications define key application metadata, including supported
/// locales, in an Info.plist file that is built into the application bundle.
/// To configure the locales supported by your app, you’ll need to edit this
/// file.
///
/// First, open your project’s ios/Runner.xcworkspace Xcode workspace file.
/// Then, in the Project Navigator, open the Info.plist file under the Runner
/// project’s Runner folder.
///
/// Next, select the Information Property List item, select Add Item from the
/// Editor menu, then select Localizations from the pop-up menu.
///
/// Select and expand the newly-created Localizations item then, for each
/// locale your application supports, add a new item and select the locale
/// you wish to add from the pop-up menu in the Value field. This list should
/// be consistent with the languages listed in the AppLocalizations.supportedLocales
/// property.
abstract class AppLocalizations {
  AppLocalizations(String locale)
    : localeName = intl.Intl.canonicalizedLocale(locale.toString());

  final String localeName;

  static AppLocalizations? of(BuildContext context) {
    return Localizations.of<AppLocalizations>(context, AppLocalizations);
  }

  static const LocalizationsDelegate<AppLocalizations> delegate =
      _AppLocalizationsDelegate();

  /// A list of this localizations delegate along with the default localizations
  /// delegates.
  ///
  /// Returns a list of localizations delegates containing this delegate along with
  /// GlobalMaterialLocalizations.delegate, GlobalCupertinoLocalizations.delegate,
  /// and GlobalWidgetsLocalizations.delegate.
  ///
  /// Additional delegates can be added by appending to this list in
  /// MaterialApp. This list does not have to be used at all if a custom list
  /// of delegates is preferred or required.
  static const List<LocalizationsDelegate<dynamic>> localizationsDelegates =
      <LocalizationsDelegate<dynamic>>[
        delegate,
        GlobalMaterialLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
      ];

  /// A list of this localizations delegate's supported locales.
  static const List<Locale> supportedLocales = <Locale>[
    Locale('ru'),
    Locale('uz'),
    Locale.fromSubtags(languageCode: 'uz', scriptCode: 'Cyrl'),
  ];

  /// The application's title.
  ///
  /// In uz, this message translates to:
  /// **'AvtoTest'**
  String get appTitle;

  /// Label for the phone number input field.
  ///
  /// In uz, this message translates to:
  /// **'Telefon raqami'**
  String get phoneLabel;

  /// Label for the button that submits the phone number and continues to OTP.
  ///
  /// In uz, this message translates to:
  /// **'Davom etish'**
  String get continueButton;

  /// Label for the OTP (one-time password) input field.
  ///
  /// In uz, this message translates to:
  /// **'Tasdiqlash kodi'**
  String get otpLabel;

  /// Label for the button that submits the OTP for verification.
  ///
  /// In uz, this message translates to:
  /// **'Tasdiqlash'**
  String get verifyButton;

  /// Label for the logout action.
  ///
  /// In uz, this message translates to:
  /// **'Chiqish'**
  String get logout;

  /// Generic fallback error message shown when a more specific one is unavailable.
  ///
  /// In uz, this message translates to:
  /// **'Xatolik yuz berdi'**
  String get errorGeneric;

  /// Inline error shown under the phone field when the entered value doesn't match the accepted phone format.
  ///
  /// In uz, this message translates to:
  /// **'Telefon raqami noto\'g\'ri formatda'**
  String get phoneInvalidError;

  /// Dev-only caption on the OTP screen surfacing the sandbox debug OTP code (kDebugMode only).
  ///
  /// In uz, this message translates to:
  /// **'Dev kod: {code}'**
  String devCodeCaption(String code);

  /// Shows the phone number the OTP was requested for, on the OTP verification screen.
  ///
  /// In uz, this message translates to:
  /// **'Telefon: {phone}'**
  String phoneConfirmationLabel(String phone);

  /// Label for the OTP resend button.
  ///
  /// In uz, this message translates to:
  /// **'Qayta yuborish'**
  String get resendButton;

  /// Label for the OTP resend button while its cooldown is active.
  ///
  /// In uz, this message translates to:
  /// **'Qayta yuborish ({seconds}s)'**
  String resendIn(int seconds);

  /// Honest placeholder label for not-yet-built nav sections (variants/practice/mistakes/stats).
  ///
  /// In uz, this message translates to:
  /// **'Tez orada'**
  String get comingSoon;

  /// Home shell nav entry label for the (not yet built) exam variants section.
  ///
  /// In uz, this message translates to:
  /// **'Variantlar'**
  String get navVariantsLabel;

  /// Home shell nav entry label for the (not yet built) practice section.
  ///
  /// In uz, this message translates to:
  /// **'Mashq qilish'**
  String get navPracticeLabel;

  /// Home shell nav entry label for the (not yet built) mistakes-review section.
  ///
  /// In uz, this message translates to:
  /// **'Xatolar ustida ishlash'**
  String get navMistakesLabel;

  /// Home shell nav entry label for the (not yet built) stats section.
  ///
  /// In uz, this message translates to:
  /// **'Statistika'**
  String get navStatsLabel;

  /// Shown on the home shell when the user's VIP entitlement is active.
  ///
  /// In uz, this message translates to:
  /// **'VIP: faol'**
  String get vipActiveLabel;

  /// Shown on the home shell when the user's VIP entitlement is not active.
  ///
  /// In uz, this message translates to:
  /// **'VIP: faol emas'**
  String get vipInactiveLabel;

  /// Label for a generic retry action shown after a failed data load.
  ///
  /// In uz, this message translates to:
  /// **'Qayta urinish'**
  String get retryButton;

  /// Tooltip for the home shell's light/dark theme toggle button.
  ///
  /// In uz, this message translates to:
  /// **'Mavzuni almashtirish'**
  String get themeToggleTooltip;

  /// Fallback message shown on the home shell when profile/entitlement fetch fails without a specific backend message.
  ///
  /// In uz, this message translates to:
  /// **'Profil ma\'lumotlarini yuklab bo\'lmadi'**
  String get profileLoadError;

  /// No description provided for @practiceSetupTitle.
  ///
  /// In uz, this message translates to:
  /// **'Mashq sozlamalari'**
  String get practiceSetupTitle;

  /// No description provided for @practiceSetupDescription.
  ///
  /// In uz, this message translates to:
  /// **'Mashq qilish uchun kategoriya YOKI belgini tanlang (ikkalasi emas, faqat bittasi).'**
  String get practiceSetupDescription;

  /// No description provided for @practiceTargetCategory.
  ///
  /// In uz, this message translates to:
  /// **'Kategoriya'**
  String get practiceTargetCategory;

  /// No description provided for @practiceTargetSign.
  ///
  /// In uz, this message translates to:
  /// **'Belgi'**
  String get practiceTargetSign;

  /// No description provided for @practiceLoadCategoriesError.
  ///
  /// In uz, this message translates to:
  /// **'Kategoriyalarni yuklab bo\'lmadi.'**
  String get practiceLoadCategoriesError;

  /// No description provided for @practiceSelectCategory.
  ///
  /// In uz, this message translates to:
  /// **'Kategoriyani tanlang'**
  String get practiceSelectCategory;

  /// No description provided for @practiceLoadSignsError.
  ///
  /// In uz, this message translates to:
  /// **'Belgilarni yuklab bo\'lmadi.'**
  String get practiceLoadSignsError;

  /// No description provided for @practiceSelectSign.
  ///
  /// In uz, this message translates to:
  /// **'Belgini tanlang'**
  String get practiceSelectSign;

  /// No description provided for @questionCountLabel.
  ///
  /// In uz, this message translates to:
  /// **'Savollar soni'**
  String get questionCountLabel;

  /// No description provided for @startButton.
  ///
  /// In uz, this message translates to:
  /// **'Boshlash'**
  String get startButton;

  /// No description provided for @signsScreenTitle.
  ///
  /// In uz, this message translates to:
  /// **'Yo\'l belgilari'**
  String get signsScreenTitle;

  /// No description provided for @searchLabel.
  ///
  /// In uz, this message translates to:
  /// **'Qidiruv'**
  String get searchLabel;

  /// No description provided for @groupCodeLabel.
  ///
  /// In uz, this message translates to:
  /// **'Guruh kodi (ixtiyoriy)'**
  String get groupCodeLabel;

  /// No description provided for @signsLoadError.
  ///
  /// In uz, this message translates to:
  /// **'Belgilar ro\'yxatini yuklab bo\'lmadi.'**
  String get signsLoadError;

  /// No description provided for @signsEmptyState.
  ///
  /// In uz, this message translates to:
  /// **'Hech qanday belgi topilmadi.'**
  String get signsEmptyState;

  /// No description provided for @mistakesScreenTitle.
  ///
  /// In uz, this message translates to:
  /// **'Xatolar ustida ishlash'**
  String get mistakesScreenTitle;

  /// No description provided for @mistakesScreenDescription.
  ///
  /// In uz, this message translates to:
  /// **'Avval noto\'g\'ri javob bergan savollaringiz ustida qayta ishlang — savollar tizim tomonidan avtomatik tanlanadi.'**
  String get mistakesScreenDescription;

  /// No description provided for @variantsScreenTitle.
  ///
  /// In uz, this message translates to:
  /// **'Biletlar'**
  String get variantsScreenTitle;

  /// No description provided for @variantsLoadError.
  ///
  /// In uz, this message translates to:
  /// **'Biletlar ro\'yxatini yuklab bo\'lmadi.'**
  String get variantsLoadError;

  /// No description provided for @variantsEmptyState.
  ///
  /// In uz, this message translates to:
  /// **'Hozircha biletlar mavjud emas.'**
  String get variantsEmptyState;

  /// No description provided for @lockedLabel.
  ///
  /// In uz, this message translates to:
  /// **'Yopiq'**
  String get lockedLabel;

  /// No description provided for @sessionQuestionLoadError.
  ///
  /// In uz, this message translates to:
  /// **'Savolni yuklab bo\'lmadi.'**
  String get sessionQuestionLoadError;

  /// No description provided for @sessionTitleExam.
  ///
  /// In uz, this message translates to:
  /// **'Imtihon'**
  String get sessionTitleExam;

  /// No description provided for @sessionTitleVariant.
  ///
  /// In uz, this message translates to:
  /// **'Bilet'**
  String get sessionTitleVariant;

  /// No description provided for @sessionTitlePractice.
  ///
  /// In uz, this message translates to:
  /// **'Mashq'**
  String get sessionTitlePractice;

  /// No description provided for @sessionTitleMistakes.
  ///
  /// In uz, this message translates to:
  /// **'Xatolar ustida ishlash'**
  String get sessionTitleMistakes;

  /// No description provided for @sessionTitleDefault.
  ///
  /// In uz, this message translates to:
  /// **'Test'**
  String get sessionTitleDefault;

  /// Progress label on the session screen, e.g. 'Savol 3 / 20'.
  ///
  /// In uz, this message translates to:
  /// **'Savol {current} / {total}'**
  String sessionProgressLabel(int current, int total);

  /// No description provided for @sessionFinishButton.
  ///
  /// In uz, this message translates to:
  /// **'Yakunlash'**
  String get sessionFinishButton;

  /// No description provided for @sessionNextButton.
  ///
  /// In uz, this message translates to:
  /// **'Keyingi'**
  String get sessionNextButton;

  /// No description provided for @sessionVipRequiredError.
  ///
  /// In uz, this message translates to:
  /// **'Bu bo\'lim uchun faol obuna kerak. Obuna hozircha bu versiyada mavjud emas.'**
  String get sessionVipRequiredError;

  /// No description provided for @sessionDailyLimitError.
  ///
  /// In uz, this message translates to:
  /// **'Bugungi bepul limitga yetdingiz. Ertaga yana davom etishingiz mumkin.'**
  String get sessionDailyLimitError;

  /// No description provided for @homeButton.
  ///
  /// In uz, this message translates to:
  /// **'Bosh sahifa'**
  String get homeButton;

  /// No description provided for @sessionStatusPassedLabel.
  ///
  /// In uz, this message translates to:
  /// **'O\'tdingiz'**
  String get sessionStatusPassedLabel;

  /// No description provided for @sessionStatusFailedLabel.
  ///
  /// In uz, this message translates to:
  /// **'O\'ta olmadingiz'**
  String get sessionStatusFailedLabel;

  /// No description provided for @sessionResultViewAbandonedLabel.
  ///
  /// In uz, this message translates to:
  /// **'Sessiya to\'xtatildi'**
  String get sessionResultViewAbandonedLabel;

  /// No description provided for @sessionResultsAbandonedLabel.
  ///
  /// In uz, this message translates to:
  /// **'Sessiya tugallanmadi'**
  String get sessionResultsAbandonedLabel;

  /// No description provided for @sessionReasonCompletedLabel.
  ///
  /// In uz, this message translates to:
  /// **'Barcha savollar yakunlandi'**
  String get sessionReasonCompletedLabel;

  /// No description provided for @sessionReasonTimeUpLabel.
  ///
  /// In uz, this message translates to:
  /// **'Vaqt tugadi'**
  String get sessionReasonTimeUpLabel;

  /// No description provided for @sessionReasonTooManyErrorsLabel.
  ///
  /// In uz, this message translates to:
  /// **'Xatolar soni ko\'payib ketdi'**
  String get sessionReasonTooManyErrorsLabel;

  /// No description provided for @sessionResultTitle.
  ///
  /// In uz, this message translates to:
  /// **'Natija'**
  String get sessionResultTitle;

  /// No description provided for @sessionAnswerRetryMessage.
  ///
  /// In uz, this message translates to:
  /// **'Javobingizni yuborib bo\'lmadi. Qayta urinib ko\'ring.'**
  String get sessionAnswerRetryMessage;
}

class _AppLocalizationsDelegate
    extends LocalizationsDelegate<AppLocalizations> {
  const _AppLocalizationsDelegate();

  @override
  Future<AppLocalizations> load(Locale locale) {
    return SynchronousFuture<AppLocalizations>(lookupAppLocalizations(locale));
  }

  @override
  bool isSupported(Locale locale) =>
      <String>['ru', 'uz'].contains(locale.languageCode);

  @override
  bool shouldReload(_AppLocalizationsDelegate old) => false;
}

AppLocalizations lookupAppLocalizations(Locale locale) {
  // Lookup logic when language+script codes are specified.
  switch (locale.languageCode) {
    case 'uz':
      {
        switch (locale.scriptCode) {
          case 'Cyrl':
            return AppLocalizationsUzCyrl();
        }
        break;
      }
  }

  // Lookup logic when only language code is specified.
  switch (locale.languageCode) {
    case 'ru':
      return AppLocalizationsRu();
    case 'uz':
      return AppLocalizationsUz();
  }

  throw FlutterError(
    'AppLocalizations.delegate failed to load unsupported locale "$locale". This is likely '
    'an issue with the localizations generation tool. Please file an issue '
    'on GitHub with a reproducible sample app and the gen-l10n configuration '
    'that was used.',
  );
}
