# M1 Plan 06 — Flutter Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A working Flutter web app skeleton — project scaffold, design system, i18n, networking core, routing, and a real phone+OTP auth flow against the live Go backend (Plans 01-05) — that every later Flutter plan (07: test-flow screens, 08: E2E+staging) builds on. Mirrors what Plan 01 was for the backend: not feature-complete, but a solid, tested foundation with no shortcuts.

**Architecture:** Feature-first Clean architecture per the master spec §15 (`lib/app` router/theme/l10n, `lib/core` network/result, `lib/features/*` data/domain/presentation, `lib/shared/widgets`). State: Riverpod. Models: freezed + json_serializable (codegen via build_runner, generated files committed — same "generated code is committed" convention already established for sqlc on the backend). Routing: go_router with an auth guard. Networking: Dio with a JWT-attach + 401-refresh-retry interceptor talking to the Go API's `/api/v1` surface built in Plans 01-05.

**Tech Stack:** Flutter 3.44.6 (stable channel), Dart 3.12, flutter_riverpod, go_router, dio, freezed/freezed_annotation, json_serializable/json_annotation, shared_preferences, intl + flutter_localizations, flutter_lints, mocktail (test doubles), integration_test (bundled with Flutter SDK).

**Plan sequence for M1:** 01 backend foundation → 02 auth+profile+entitlement → 03 sessions/scoring/unlock → 04 FSRS+stats → 05 explanations-AI-draft+saved+streak+limits+events → **06 Flutter foundation (this plan)** → 07 Flutter test flows (exam simulator UI, practice/mistakes, explanation render, saved/streak UI, stats) → 08 E2E+staging deploy.

## Environment note (read first)

Flutter was **not previously installed** in this environment. It has now been installed at `~/.local/flutter` (stable 3.44.6), mirroring the existing `~/.local/go` convention, with web support enabled and Chrome detected via `google-chrome-stable`. **Every shell in this plan must export:**

```bash
export PATH="$HOME/.local/flutter/bin:$PATH"
export CHROME_EXECUTABLE=google-chrome-stable
```

This was verified end-to-end before writing this plan: `flutter create --platforms=web`, `flutter build web`, and `flutter gen-l10n` with a script-qualified `uz_Cyrl` ARB locale (see Task 4) all confirmed working.

**No visual/screenshot verification tool is available to agents in this environment** (no browser-automation/screenshot tool). Verification standard for every task in this plan is therefore: `flutter analyze` (0 issues) + `flutter test` (unit tests + `WidgetTester`-based widget tests, which genuinely build widget trees, pump frames, tap, and assert on rendered output — real behavioral verification, just not pixel-level) + `flutter build web` succeeding. For the one place this isn't enough — the live auth flow against the real backend — Task 9 does a one-time **manual, documented, live verification pass** (same discipline as Plan 05 Task 10's smoke test), not a permanent CI wiring. Full automated E2E-in-CI is explicitly Plan 08's scope, not this plan's. Implementers and reviewers must say "structurally verified via widget test" rather than "visually confirmed" — do not claim visual confirmation that didn't happen.

## Global Constraints

- Repo root: `/home/sher/Рабочий стол/avtotest` (git `main`, contains Cyrillic + a space — always double-quote in shell commands, never backslash-escape). Flutter app lives in `app/`, Dart package name `avtotest_app`.
- M1 target is **web only** (D4: "M1 = Web o'quv yadro"). `flutter create --platforms=web`. Android/iOS/Linux-desktop toolchains are not installed and not needed — that's M6.
- Dev backend runs on `PORT=8090` (established convention, port 8080 is occupied in this environment — see backend README). Flutter dev builds point at `http://localhost:8090/api/v1` by default, overridable via `--dart-define=API_BASE_URL=...`.
- Locales exactly `uz-Latn` (default/fallback), `uz-Cyrl`, `ru` — same three as the backend (`kaa` not yet enabled, per backend convention). Dart locale objects: `Locale('uz')` for uz-Latn, `Locale.fromSubtags(languageCode: 'uz', scriptCode: 'Cyrl')` for uz-Cyrl, `Locale('ru')` for ru. **Never assume a 1:1 string match** between Dart's `Locale.toString()` and the backend's `locale_code` values (`uz-Latn`/`uz-Cyrl`/`ru`) — write an explicit two-way mapping function (Task 4) and use it at every API call site.
- API envelope (from the backend, unchanged since Plan 01): success `{"data":...,"meta":{...}?}`, error `{"error":{"code":"...","message":"..."}}`. The Dio layer's `Result`/`Failure` type (Task 2) must expose both `code` and `message` from error responses, not just a generic exception.
- Auth tokens: JWT access (15 min) + rotating refresh (30 days) returned as **JSON body fields** (not cookies) by `POST /auth/otp/verify` / `POST /auth/refresh` (see backend README's Auth section). Client is responsible for storage and refresh scheduling. Storage: `shared_preferences` (web-compatible via localStorage) — an explicit, documented trade-off for M1 web; revisit with secure storage when M6 adds native mobile targets.
- State management: Riverpod (`flutter_riverpod`), hand-written providers (no `riverpod_generator` codegen layer — keep the codegen surface to freezed/json_serializable only, to limit build_runner complexity in this foundation plan).
- Models: freezed (+ `freezed_annotation`) for domain/state types, json_serializable (+ `json_annotation`) for API DTOs. Any task that adds/changes an annotated class **must** run `dart run build_runner build --delete-conflicting-outputs` from `app/` and commit the generated `*.freezed.dart`/`*.g.dart` files — same "generated code is committed" convention as the backend's sqlc output.
- Design tone (master spec §16): dark-default Material 3, "Apple-clean" minimal-premium. This plan lays foundation-level tokens (color scheme, typography scale, spacing) — not full screen-by-screen visual polish, which accumulates through Plan 07 too.
- Testing: `flutter test` for unit + widget tests; `mocktail` for repository/API mocks (no real network calls in unit/widget tests). `integration_test` package (bundled) for the one live-backend check in Task 9.
- Commits: conventional (`feat:`, `chore:`, `test:` …), Claude co-author trailer, same as backend plans. Work directly on `main`, no feature branches (established project convention).
- CI: extend the existing `.github/workflows/ci.yml` with a `frontend` job (`subosito/flutter-action@v2`, channel stable, working-directory `app`) running `flutter pub get` → (`dart run build_runner build --delete-conflicting-outputs` once annotated models exist) → `flutter analyze` → `flutter test`. Added/updated in the task that first needs it (Task 1 adds the job skeleton; the task that introduces the first codegen'd class adds the build_runner step).

## File Structure (final state of this plan)

```
avtotest/
  .github/workflows/ci.yml            # + frontend job
  app/
    pubspec.yaml
    l10n.yaml
    analysis_options.yaml
    lib/
      main.dart                       # runApp(ProviderScope(child: AvtotestApp()))
      app/
        app.dart                      # AvtotestApp: MaterialApp.router root widget
        router.dart                   # go_router config + auth-guard redirect
        theme/
          app_theme.dart              # ColorScheme/typography/spacing tokens, light+dark ThemeData
          theme_mode_provider.dart    # persisted ThemeMode
        l10n/
          app_uz.arb  app_uz_Cyrl.arb  app_ru.arb
          app_localizations*.dart     # generated, committed
        locale/
          locale_provider.dart        # persisted Locale + Dart<->backend locale_code mapping
      core/
        network/
          api_client.dart             # Dio factory + baseUrl config
          auth_interceptor.dart       # attach access token; 401 -> refresh -> retry-once
          token_storage.dart          # TokenStorage abstraction (SharedPreferences impl)
        result.dart                   # freezed Result<T>/Failure sealed types
      features/
        auth/
          data/auth_api.dart
          data/auth_repository.dart
          domain/auth_state.dart      # freezed AuthState
          presentation/auth_controller.dart     # Riverpod AsyncNotifier
          presentation/phone_entry_screen.dart
          presentation/otp_verify_screen.dart
        profile/
          data/profile_api.dart
          domain/profile.dart         # freezed Profile + Entitlement models
          presentation/profile_controller.dart
        home/
          presentation/home_shell.dart          # nav shell, placeholder destinations
      shared/
        widgets/
          empty_state.dart
          primary_button.dart
          app_card.dart
    test/
      core/network/...
      features/auth/...
      features/profile/...
    integration_test/
      auth_flow_test.dart            # drives the real dev backend, PORT=8090
```

---

### Task 1: Flutter project scaffold + CI

**Files:** create `app/` (via `flutter create`), `app/pubspec.yaml` (deps added), `.github/workflows/ci.yml` (modify — add `frontend` job).

**Steps:**

- [ ] **Step 1: Scaffold.** From repo root:
  ```bash
  export PATH="$HOME/.local/flutter/bin:$PATH"
  export CHROME_EXECUTABLE=google-chrome-stable
  cd "/home/sher/Рабочий стол/avtotest"
  flutter create --platforms=web --org uz.avtotest --project-name avtotest_app app
  ```
  Confirm `app/pubspec.yaml`'s `name:` field reads `avtotest_app`. If `flutter create` didn't honor `--project-name` the way you expect, fix `name:` manually rather than guessing — check the actual generated file before moving on.

- [ ] **Step 2: Dependencies.** From `app/`:
  ```bash
  flutter pub add flutter_riverpod go_router dio shared_preferences freezed_annotation json_annotation
  flutter pub add --dev flutter_lints build_runner freezed json_serializable mocktail
  ```
  Leave `flutter_localizations`/`intl` for Task 4 (i18n), which needs them specifically.

- [ ] **Step 3: CI.** Add a `frontend` job to `.github/workflows/ci.yml`, alongside the existing `backend` job:
  ```yaml
    frontend:
      runs-on: ubuntu-latest
      defaults:
        run:
          working-directory: app
      steps:
        - uses: actions/checkout@v4
        - uses: subosito/flutter-action@v2
          with:
            channel: stable
        - run: flutter pub get
        - run: flutter analyze
        - run: flutter test
  ```
  (A later task in this plan will insert a `dart run build_runner build --delete-conflicting-outputs` step between `pub get` and `analyze` once the first freezed/json_serializable class exists — do that edit in the task that introduces it, not here.)

- [ ] **Step 4: Verify.** `flutter analyze` (0 issues) and `flutter test` (the default counter-app widget test that `flutter create` generates should still pass as-is — this is a fine baseline smoke test for this task only; later tasks replace `main.dart`'s content). `flutter build web` succeeds.

- [ ] **Step 5: Commit.**
  ```bash
  git add app/ .github/workflows/ci.yml
  git commit -m "feat(frontend): Flutter project scaffold + CI"
  ```

---

### Task 2: Core network layer

**Files:** create `app/lib/core/result.dart`, `app/lib/core/network/token_storage.dart`, `app/lib/core/network/api_client.dart`, `app/lib/core/network/auth_interceptor.dart`, and matching `app/test/core/**` test files.

**Interfaces (produced):**
```dart
// core/result.dart — freezed sealed Result
@freezed
sealed class Result<T> with _$Result<T> {
  const factory Result.ok(T data) = Ok<T>;
  const factory Result.err(Failure failure) = Err<T>;
}

@freezed
class Failure with _$Failure {
  const factory Failure({required String code, required String message}) = _Failure;
  // code: backend's error.code when available (e.g. "invalid_code", "vip_required"),
  // or a client-side sentinel ("network_error", "unknown") when the server didn't
  // return a structured envelope (timeout, no connectivity, non-JSON 5xx, etc).
}

// core/network/token_storage.dart
abstract class TokenStorage {
  Future<String?> readAccess();
  Future<String?> readRefresh();
  Future<void> save({required String access, required String refresh});
  Future<void> clear();
}
class SharedPrefsTokenStorage implements TokenStorage { ... }

// core/network/api_client.dart
class ApiClient {
  ApiClient({required Dio dio}) : _dio = dio;
  final Dio _dio;
  Dio get dio => _dio;
}
// factory: Dio buildDio({required String baseUrl, required TokenStorage tokenStorage,
//   required Future<void> Function() onSessionExpired})
//   -> configures BaseOptions(baseUrl: ...), adds AuthInterceptor.

// core/network/auth_interceptor.dart
class AuthInterceptor extends Interceptor {
  AuthInterceptor({required TokenStorage tokenStorage, required Dio refreshDio,
    required Future<void> Function() onSessionExpired});
  // onRequest: attach `Authorization: Bearer <access>` if present.
  // onError: if response?.statusCode == 401 AND request not already retried
  //   (mark via err.requestOptions.extra['retried'] = true before retrying):
  //     call POST /auth/refresh with stored refresh token (via a *separate* plain
  //     Dio instance with no interceptor, to avoid recursive 401 handling),
  //     on success: save new tokens, retry the original request once via `handler.resolve`,
  //     on failure: clear tokens, call onSessionExpired(), reject the error.
}
```

**Logic:**
- `Result`/`Failure` are the only way network errors cross into `features/*` — no raw `DioException` leaks past `core/network`.
- A helper `Future<Result<T>> guard<T>(Future<T> Function() call)` (in `result.dart` or a sibling file) wraps a Dio call, catching `DioException` and mapping: if `e.response?.data` is a JSON map matching `{"error":{"code":...,"message":...}}`, extract those; else map `DioExceptionType.connectionTimeout`/`connectionError`/etc. to `Failure(code: "network_error", message: ...)`; anything else to `Failure(code: "unknown", message: e.toString())`.
- `AuthInterceptor`'s retry-once guard is the important correctness point here — without it, a stale/invalid refresh token would loop the app forever re-attempting `/auth/refresh`. Write a test that specifically exercises "refresh itself returns 401" and asserts `onSessionExpired` fires exactly once, tokens are cleared, and no infinite loop occurs.

**Testing approach:** Use `dio`'s `DioAdapter`-style testing (either `http_mock_adapter` — add as a dev dep if you use it — or a hand-rolled `Dio` with a custom `HttpClientAdapter` returning canned `ResponseBody`s; either is fine, pick whichever is less code, but do not hit a real network in these tests). Cover: successful request attaches bearer header; 401 triggers exactly one refresh+retry cycle on success; 401 with a failing refresh clears tokens and calls `onSessionExpired` exactly once (no loop); a non-401 error passes through unchanged; `guard()` correctly maps a structured error envelope vs. a connection failure.

- [ ] **Step 1-4:** Write tests first (per the interfaces above), confirm they fail, implement, confirm they pass, `dart run build_runner build --delete-conflicting-outputs` (first codegen'd classes in the project — also add the build_runner CI step now, per Task 1 Step 3's note), `flutter analyze` clean.
- [ ] **Step 5: Commit.**
  ```bash
  git add app/lib/core/ app/test/core/ app/pubspec.yaml app/pubspec.lock .github/workflows/ci.yml
  git commit -m "feat(frontend): core network layer — Dio client, token storage, 401-refresh interceptor"
  ```

---

### Task 3: Design system / theme foundation

**Files:** create `app/lib/app/theme/app_theme.dart`, `app/lib/app/theme/theme_mode_provider.dart`, `app/test/app/theme/theme_mode_provider_test.dart`.

**Interfaces (produced):**
```dart
class AppTheme {
  static ThemeData light();
  static ThemeData dark();
}
// A small set of named spacing/radius constants (e.g. AppSpacing.sm/md/lg, AppRadius.card)
// used by shared widgets later — keep this minimal, expand only as later tasks need more.

final themeModeProvider = NotifierProvider<ThemeModeNotifier, ThemeMode>(ThemeModeNotifier.new);
class ThemeModeNotifier extends Notifier<ThemeMode> {
  @override ThemeMode build(); // reads persisted value via SharedPreferences, default ThemeMode.dark
  Future<void> setThemeMode(ThemeMode mode); // persists + updates state
}
```

**Logic:** Dark is the default (master spec §16: "dark (default) + light"). `ThemeData.dark()`/`.light()` built off `ColorScheme.fromSeed` with a seed color of your choosing consistent with a "minimal-premium, Apple-clean" feel (a desaturated, high-contrast palette — avoid loud/saturated defaults); `useMaterial3: true`. Typography: pick one clean sans (system default is fine for foundation — do not add a custom font asset in this task, that's a later polish concern) with a sensible `TextTheme` scale. Persist `ThemeMode` via `shared_preferences` under a fixed key (e.g. `"theme_mode"`, storing `"light"`/`"dark"`/`"system"`).

**Testing approach:** Unit test `ThemeModeNotifier` with `ProviderContainer` + `SharedPreferences.setMockInitialValues({})` (the standard way to fake `shared_preferences` in tests, no real disk/browser storage involved) — verify default is dark, verify `setThemeMode` persists and is re-read on a fresh container. No widget test strictly needed here (no screen exists yet to render) — that's fine, note it explicitly rather than fabricating one.

- [ ] Steps 1-4 (test-first, implement, `flutter analyze` clean).
- [ ] **Step 5: Commit.**
  ```bash
  git add app/lib/app/theme/ app/test/app/theme/
  git commit -m "feat(frontend): dark-default Material 3 theme + persisted theme mode"
  ```

---

### Task 4: i18n foundation (uz-Latn, uz-Cyrl, ru)

**Files:** create `app/l10n.yaml`, `app/lib/app/l10n/app_uz.arb`, `app/lib/app/l10n/app_uz_Cyrl.arb`, `app/lib/app/l10n/app_ru.arb`, `app/lib/app/locale/locale_provider.dart`, `app/test/app/locale/locale_provider_test.dart`; modify `app/pubspec.yaml` (add `flutter_localizations`/`intl`, set `flutter: generate: true`).

**Interfaces (produced):**
```dart
// app/lib/app/locale/locale_provider.dart
final localeProvider = NotifierProvider<LocaleNotifier, Locale>(LocaleNotifier.new);
class LocaleNotifier extends Notifier<Locale> {
  @override Locale build(); // persisted, default Locale('uz') (uz-Latn)
  Future<void> setLocale(Locale locale);
}

// Two-way mapping to the backend's locale_code values ("uz-Latn"/"uz-Cyrl"/"ru")
String localeToBackendCode(Locale locale);
Locale backendCodeToLocale(String code);
```

**Logic:**
- `l10n.yaml` (repo root of `app/`):
  ```yaml
  arb-dir: lib/app/l10n
  template-arb-file: app_uz.arb
  output-localization-file: app_localizations.dart
  ```
- ARB files: `app_uz.arb` (`"@@locale": "uz"`), `app_uz_Cyrl.arb` (`"@@locale": "uz_Cyrl"` — **this exact filename/locale-tag combination was verified working** in this environment before writing this plan; `flutter gen-l10n` correctly generates a `AppLocalizationsUzCyrl extends AppLocalizationsUz` subclass and registers `Locale.fromSubtags(languageCode: 'uz', scriptCode: 'Cyrl')` in `supportedLocales` — do not second-guess this or invent a workaround, just follow the same pattern), `app_ru.arb` (`"@@locale": "ru"`). Populate each with a small starter set of strings actually needed by this plan's screens (not the whole app's eventual vocabulary): e.g. `appTitle`, `phoneLabel`, `continueButton`, `otpLabel`, `verifyButton`, `logout`, `errorGeneric` — add more as later tasks in this plan need them (auth/profile/home), don't pre-invent strings nothing uses yet.
- `pubspec.yaml`: add `flutter_localizations: {sdk: flutter}` and `intl: any` (or whatever `flutter pub add flutter_localizations --sdk=flutter` resolves to) to `dependencies`; set `flutter: generate: true` under the `flutter:` section (this is a **separate** top-level `flutter:` key from `dependencies: flutter: {sdk: flutter}` — don't confuse the two blocks when editing, they look similar).
- `localeToBackendCode`/`backendCodeToLocale`: explicit switch/map, not string manipulation — e.g. `Locale('ru') -> "ru"`, `Locale('uz') -> "uz-Latn"`, `Locale.fromSubtags(languageCode:'uz', scriptCode:'Cyrl') -> "uz-Cyrl"`, and the reverse. This is the single source of truth every API-calling feature must import rather than re-deriving locale strings themselves.
- `LocaleNotifier` persists via `shared_preferences` (key e.g. `"locale"`, storing the backend code string via `localeToBackendCode`/`backendCodeToLocale` so the persisted value and the wire format are the same representation).

**Testing approach:** run `flutter gen-l10n` (or let `flutter test`/`flutter analyze` trigger it via `generate: true` — confirm which actually happens in this Flutter version, some versions require an explicit `flutter gen-l10n` run before `flutter analyze` sees the generated class; if so, document that as a required step here rather than silently depending on implicit generation) and confirm `lib/app/l10n/app_localizations.dart` + per-locale files exist and are committed. Unit test `LocaleNotifier` (same `ProviderContainer` + mocked `shared_preferences` pattern as Task 3) and `localeToBackendCode`/`backendCodeToLocale` round-tripping for all three locales.

- [ ] Steps 1-4 (test-first where practical — the mapping functions are pure and easy to test-first; the ARB/gen-l10n plumbing is more "set up then verify it generates correctly").
- [ ] **Step 5: Commit.**
  ```bash
  git add app/l10n.yaml app/lib/app/l10n/ app/lib/app/locale/ app/test/app/locale/ app/pubspec.yaml app/pubspec.lock
  git commit -m "feat(frontend): i18n foundation — uz-Latn/uz-Cyrl/ru ARB + locale provider"
  ```

---

### Task 5: Router skeleton + app root

**Files:** create `app/lib/app/router.dart`, `app/lib/app/app.dart`; modify `app/lib/main.dart`.

**Interfaces (produced):**
```dart
// app/lib/app/router.dart
final routerProvider = Provider<GoRouter>((ref) => GoRouter(
  initialLocation: '/',
  routes: [ GoRoute(path: '/', builder: (context, state) => const _PlaceholderHome()) ],
  // no auth guard/redirect logic yet — Task 7 adds /login routes + the guard;
  // this task only proves the router/theme/locale wiring works end-to-end.
));

// app/lib/app/app.dart
class AvtotestApp extends ConsumerWidget {
  const AvtotestApp({super.key});
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final router = ref.watch(routerProvider);
    final themeMode = ref.watch(themeModeProvider);
    final locale = ref.watch(localeProvider);
    return MaterialApp.router(
      routerConfig: router,
      theme: AppTheme.light(),
      darkTheme: AppTheme.dark(),
      themeMode: themeMode,
      locale: locale,
      supportedLocales: AppLocalizations.supportedLocales,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
    );
  }
}
```
`main.dart`: `void main() { runApp(const ProviderScope(child: AvtotestApp())); }` — replace the `flutter create` default counter app entirely.

**Logic:** This task's only job is proving the four foundation pieces (theme, locale, routing, DI root) compose correctly with nothing else layered on yet — a single placeholder route rendering something trivial that reads a localized string and the current theme, so a widget test can assert real integration, not just "it compiles."

**Testing approach:** A widget test that pumps `ProviderScope(child: AvtotestApp())`, confirms the placeholder home renders, confirms `find.byType(MaterialApp)` (or the `.router` equivalent) exists, and — this is the meaningful assertion — confirms the correct localized string appears for at least two different `localeProvider` override values (e.g. override with `Locale('ru')` vs `Locale('uz')` and check the rendered text differs accordingly). This is a genuine integration check of Task 3+4+5 composing, done via `WidgetTester`, not a screenshot.

- [ ] Steps 1-4.
- [ ] **Step 5: Commit.**
  ```bash
  git add app/lib/app/app.dart app/lib/app/router.dart app/lib/main.dart app/test/app/
  git commit -m "feat(frontend): router skeleton + app root wiring theme/locale/DI"
  ```

---

### Task 6: Auth data/domain layer

**Files:** create `app/lib/features/auth/data/auth_api.dart`, `app/lib/features/auth/data/auth_repository.dart`, `app/lib/features/auth/domain/auth_state.dart`, `app/lib/features/auth/presentation/auth_controller.dart`, and matching `app/test/features/auth/**`.

**Interfaces (produced):**
```dart
// domain/auth_state.dart
@freezed
sealed class AuthState with _$AuthState {
  const factory AuthState.unknown() = AuthUnknown;             // app just started, token not checked yet
  const factory AuthState.unauthenticated() = AuthUnauthenticated;
  const factory AuthState.otpRequested({required String phone, String? debugCode}) = AuthOtpRequested;
  const factory AuthState.authenticated() = AuthAuthenticated;
  const factory AuthState.error(Failure failure) = AuthError;
}

// data/auth_api.dart — thin Dio wrapper matching the backend README's Auth section exactly:
//   POST /auth/otp/request {phone} -> {channel, debug_code?}
//   POST /auth/otp/verify {phone, code} -> {access_token, refresh_token}
//   POST /auth/refresh {refresh_token} -> {access_token, refresh_token}
//   POST /auth/logout {refresh_token} -> {ok: true}
class AuthApi {
  AuthApi(this._dio);
  final Dio _dio;
  Future<Result<({String channel, String? debugCode})>> requestOtp(String phone);
  Future<Result<({String access, String refresh})>> verifyOtp({required String phone, required String code});
  Future<Result<void>> logout(String refreshToken);
}

// data/auth_repository.dart
class AuthRepository {
  AuthRepository({required AuthApi api, required TokenStorage tokenStorage});
  Future<Result<({String channel, String? debugCode})>> requestOtp(String phone);
  Future<Result<void>> verifyOtp({required String phone, required String code}); // saves tokens on success
  Future<bool> hasStoredSession(); // true if a refresh token is present (not necessarily still valid)
  Future<void> logout(); // calls API best-effort, always clears local tokens
}

// presentation/auth_controller.dart
final authControllerProvider = NotifierProvider<AuthController, AuthState>(AuthController.new);
class AuthController extends Notifier<AuthState> {
  @override AuthState build(); // checks hasStoredSession() -> authenticated/unauthenticated (never leaves .unknown() displayed for long)
  Future<void> requestOtp(String phone);
  Future<void> verifyOtp(String code); // uses phone from the current AuthOtpRequested state
  Future<void> logout();
}
```

**Logic:**
- Phone number format: match whatever the backend actually expects — check `internal/auth` request validation on the backend (`backend/internal/auth/`) for the exact accepted phone format (the README's example uses bare digits like `901112233`/`905550001` without a leading `+998` in some CLI examples but the DB stores `phone text UNIQUE` and OTP examples show `"phone":"905550001"` — read the actual backend validation code, don't guess the format from README prose alone) before writing client-side validation, so the client's validation regex doesn't reject something the server would accept, or vice versa.
- `error.code` mapping the backend README documents for this surface: `invalid_phone`, `rate_limited` (429), `invalid_code`, `expired_code`, `too_many_attempts`, `invalid_refresh`, `refresh_reused` (401). `AuthController` should surface these distinctly enough that Task 7's UI can show a specific message per code (e.g. "invalid_code" vs "expired_code" are different user-facing messages) rather than a single generic error string — don't collapse them.
- `AuthRepository.logout()` must clear local tokens **even if the API call fails** (e.g. network error) — a user pressing "log out" should never get stuck logged-in-locally because of a transient network blip.
- This task does not build any UI — it's the data/domain layer only, unit-tested with a mocked `AuthApi`/`Dio` (mocktail), no widgets, no real network.

**Testing approach:** Unit tests for `AuthController` covering: initial build reflects stored-session state correctly; `requestOtp` success transitions to `AuthOtpRequested` (carrying `debugCode` through when present); `requestOtp` failure transitions to `AuthError` with the right `Failure`; `verifyOtp` success saves tokens (verify via a mocked `TokenStorage`) and transitions to `AuthAuthenticated`; `verifyOtp` failure with e.g. `invalid_code` surfaces that specific code in `AuthError`; `logout` clears storage regardless of the mocked API call's outcome.

- [ ] Steps 1-4.
- [ ] **Step 5: Commit.**
  ```bash
  git add app/lib/features/auth/data/ app/lib/features/auth/domain/ app/lib/features/auth/presentation/auth_controller.dart app/test/features/auth/
  git commit -m "feat(frontend): auth data/domain layer — OTP repository + controller"
  ```

---

### Task 7: Auth UI screens + router guard integration

**Files:** create `app/lib/features/auth/presentation/phone_entry_screen.dart`, `app/lib/features/auth/presentation/otp_verify_screen.dart`, and matching widget tests; modify `app/lib/app/router.dart` (add `/login`, `/login/verify` routes + `redirect:` guard).

**Interfaces / routes (produced):**
- `GET /login` → `PhoneEntryScreen` (phone input + "Kirish"/Continue button, calls `authControllerProvider.requestOtp`).
- `GET /login/verify` → `OtpVerifyScreen` (reachable only after `AuthOtpRequested`; shows the phone number, a 6-digit code input, "Tasdiqlash"/Verify button calling `authControllerProvider.verifyOtp`, a resend-cooldown affordance if you want one — keep it simple, a disabled resend button with a countdown is enough, don't over-build).
- Router `redirect`: reading `authControllerProvider`'s current state (via `ref.read` inside the redirect callback, with the router listening to the notifier so redirects re-evaluate on state changes — `GoRouter`'s `refreshListenable` needs a `Listenable`; wrap the Riverpod state in one, e.g. via a small adapter, or use `ref.listen`-based rebuild of the `routerProvider` itself — pick whichever idiom is cleaner, but make sure redirects actually re-fire on auth state changes, not just once at router construction): `AuthUnauthenticated`/`AuthOtpRequested` → allowed only on `/login*`, redirect elsewhere to `/login`; `AuthAuthenticated` → redirect away from `/login*` to `/`; `AuthUnknown` → show a lightweight loading splash, no redirect yet.

**Logic:**
- **Dev convenience, explicitly scoped to debug builds:** when `AuthOtpRequested.debugCode` is non-null AND `kDebugMode` is true (from `package:flutter/foundation.dart`), show it somewhere visible on `OtpVerifyScreen` (e.g. "Dev kod: 123456" caption) — this exists purely so a developer testing against the sandbox OTP channel doesn't need to check server logs. Never show this in a release build; gating on `kDebugMode` handles that automatically, don't add a separate flag that could be misconfigured.
- Phone input: basic client-side format validation (per Task 6's note about matching the backend's actual accepted format) with an inline error, disabling the submit button while a request is in flight (use `AuthState`'s async-ness — if `AuthController` needs an explicit loading flag beyond the sealed states above, that's fine to add, but keep `AuthState` itself the source of truth rather than introducing parallel loading booleans that can drift out of sync).
- OTP input: 6-digit numeric entry (a simple `TextField` with `TextInputType.number`/length limit is enough — no need for a fancy per-digit-box widget in this foundation plan, that's a polish item for later if ever).

**Testing approach:** Widget tests pumping each screen standalone with `ProviderScope(overrides: [authControllerProvider.overrideWith(...)])` using a fake/mock controller — NOT a real `AuthRepository`/Dio. Cover: `PhoneEntryScreen` shows a validation error for an obviously malformed phone and does not call `requestOtp`; a valid phone submit calls `requestOtp` with the exact string entered; `OtpVerifyScreen` shows the dev debug code caption when `debugCode` is set (you can force `kDebugMode` true in tests — it already is true under `flutter test`) and does not show it when null; entering a code and submitting calls `verifyOtp` with that code; an `AuthError` state renders the failure's message. For the router guard, a focused test using `GoRouter`'s testing utilities (or a simple `ProviderScope` + `MaterialApp.router` pump) confirming: unauthenticated state on a non-`/login` path redirects to `/login`; authenticated state on `/login` redirects to `/`.

- [ ] Steps 1-4.
- [ ] **Step 5: Commit.**
  ```bash
  git add app/lib/features/auth/presentation/phone_entry_screen.dart app/lib/features/auth/presentation/otp_verify_screen.dart app/lib/app/router.dart app/test/features/auth/ app/test/app/
  git commit -m "feat(frontend): auth UI screens + router guard"
  ```

---

### Task 8: Profile fetch + home shell + shared widgets

**Files:** create `app/lib/features/profile/data/profile_api.dart`, `app/lib/features/profile/domain/profile.dart`, `app/lib/features/profile/presentation/profile_controller.dart`, `app/lib/features/home/presentation/home_shell.dart`, `app/lib/shared/widgets/empty_state.dart`, `app/lib/shared/widgets/primary_button.dart`, `app/lib/shared/widgets/app_card.dart`, and matching tests; modify `app/lib/app/router.dart` (add `/` → `HomeShell`, wrapping/replacing the Task 5 placeholder).

**Interfaces (produced):**
```dart
// domain/profile.dart — matches GET /me and GET /me/entitlement response shapes
// (read backend/internal/account and backend README's Auth section for the exact
// fields — e.g. profile has phone/name/region/district/birth_date/locale_pref/etc.,
// vip has active/until — don't invent fields the backend doesn't return)
@freezed
class Profile with _$Profile { ... }
@freezed
class Entitlement with _$Entitlement { const factory Entitlement({required bool active, DateTime? until}) = _Entitlement; }

// presentation/profile_controller.dart
final profileControllerProvider = AsyncNotifierProvider<ProfileController, ({Profile profile, Entitlement entitlement})>(ProfileController.new);
class ProfileController extends AsyncNotifier<({Profile profile, Entitlement entitlement})> {
  @override Future<({Profile profile, Entitlement entitlement})> build(); // fetches /me + /me/entitlement
}
```

**Logic:**
- `HomeShell`: a simple scaffold (app bar + body) shown at `/` once authenticated — displays the profile's name/phone and VIP status (from `profileControllerProvider`), a locale switcher (three options, wired to `localeProvider.setLocale`), a theme toggle (wired to `themeModeProvider.setThemeMode`), a logout button (calls `authControllerProvider.logout()`), and **placeholder navigation entries** for variants/practice/mistakes/stats (each just a disabled-looking `AppCard` or a `Text('Tez orada')`/"coming soon" — the real screens are Plan 07's job; do not build fake versions of them here, an honest placeholder is correct, a half-built fake screen is not).
- `EmptyState`/`PrimaryButton`/`AppCard`: small, reusable, theme-token-driven widgets (using `AppSpacing`/`AppRadius` from Task 3) — establish the pattern Plan 07's richer widgets (`QuestionCard`, `AnswerOption`, etc.) will follow, but keep these three genuinely minimal (a few constructor params each), don't over-engineer a widget library nobody's using yet.
- Router: `/` now requires authentication (covered by Task 7's guard — confirm it still redirects correctly with a real `HomeShell` in place, not just the Task 5 placeholder).

**Testing approach:** Unit test `ProfileController` with a mocked `ProfileApi` (success populates both profile+entitlement; failure surfaces via `AsyncError`). Widget tests for `HomeShell` with `ProviderScope` overrides for both `profileControllerProvider` and `authControllerProvider`: renders profile name/VIP status correctly for a VIP vs. non-VIP entitlement; tapping logout calls `authControllerProvider.logout()`; locale switcher actually changes `localeProvider`'s value (assert via `ProviderContainer.read` after the tap, or via re-rendered localized text). Widget tests for the three shared widgets (trivial — do they render their given child/label, does `PrimaryButton`'s `onPressed` fire).

- [ ] Steps 1-4.
- [ ] **Step 5: Commit.**
  ```bash
  git add app/lib/features/profile/ app/lib/features/home/ app/lib/shared/ app/lib/app/router.dart app/test/features/profile/ app/test/features/home/ app/test/shared/
  git commit -m "feat(frontend): profile fetch + home shell + shared widget primitives"
  ```

---

### Task 9: Full verification + live auth-flow check + docs

- [ ] **Step 1:** `cd "/home/sher/Рабочий стол/avtotest/app" && flutter analyze` — 0 issues. `flutter test` — all green. `flutter build web` — succeeds.

- [ ] **Step 2: Live, one-time, manual verification** (mirrors Plan 05 Task 10's smoke-test discipline — this is not wired into CI, that's Plan 08's job):
  1. Start the backend stack: `make up` (from repo root) + `cd backend && export PATH=$HOME/.local/go/bin:$HOME/go/bin:$PATH && PORT=8090 go run ./cmd/api` (background).
  2. Run the Flutter app against it: `cd app && flutter run -d web-server --web-port=5000 --dart-define=API_BASE_URL=http://localhost:8090/api/v1` (or `-d chrome` if a headed run is more practical in this environment — either way, actually run it, don't simulate).
  3. Either drive this by hand via `curl`-equivalent checks against the running Flutter dev server's compiled output plus direct backend calls to confirm the *data* flowing through is correct, **or** — preferably — write and run a real `integration_test/auth_flow_test.dart` using the `integration_test` package, executed via `flutter test integration_test/auth_flow_test.dart -d chrome`, that: enters a phone number on `PhoneEntryScreen`, requests OTP, reads the dev `debug_code` surfaced in the UI (Task 7), enters it on `OtpVerifyScreen`, submits, and asserts the app lands on `HomeShell` showing the right profile data — a real, automated, headless-Chrome-driven pass against the real backend. This is meaningfully stronger evidence than a manual click-through and should be preferred if it fits in reasonable time; if it proves impractical, fall back to a manual pass and say so explicitly in your report, with the actual commands/responses recorded (not paraphrased).
  4. Also confirm: an invalid/expired code shows the right error; logging out returns to `/login`; switching locale actually changes visible text; switching theme actually changes the rendered colors (to whatever extent you can confirm structurally, e.g. reading `Theme.of(context).brightness` in a debug print or an integration-test assertion, not by eyeballing a screenshot you don't have).

- [ ] **Step 3: README.** Add a "Flutter frontend (M1 Plan 06 holati)" section to the repo root `README.md`: how to install/run Flutter (`~/.local/flutter`, `PATH`/`CHROME_EXECUTABLE` env vars), `flutter pub get`, `flutter run -d chrome --dart-define=API_BASE_URL=...`, `flutter test`, the three supported locales, and an explicit note that this is a foundation (auth + shell only) — variants/exam/practice/mistakes/explanations/stats screens are Plan 07.

- [ ] **Step 4: Commit.**
  ```bash
  git add README.md app/integration_test/
  git commit -m "docs: Flutter foundation dev setup + live auth-flow verification"
  ```

## Self-Review

1. **Spec coverage:** master spec §23 step 1's "Flutter skeleton" (Task 1) and step 8's "Flutter: theme/design-system, auth oqimi, home" (Tasks 3-4, 6-8) are both covered. §15's feature-first/Riverpod/freezed/go_router/Dio/intl-ARB/Material-3 architecture is followed exactly. Exam simulator UI, practice/mistakes/learn/explanation/saved/stats screens (§23 steps 9-11) are explicitly **not** in this plan — that's Plan 07.
2. **Placeholders:** `HomeShell`'s nav entries for variants/practice/mistakes/stats are honest "coming soon" placeholders, not fake screens — called out explicitly in Task 8 rather than silently left vague.
3. **Type/contract consistency:** `Profile`/`Entitlement` field shapes must be read from the actual backend code/README (Task 8), not invented; `AuthApi`'s request/response shapes must match the backend README's Auth section exactly (Task 6); the `locale_code` mapping (Task 4) is the single source every later feature must reuse, not re-derive.
4. **Known scope boundary, not a gap:** no automated CI-wired E2E test against a live backend exists after this plan — that's explicitly Plan 08. This plan's Task 9 does one manual/live verification pass, same as Plan 05's precedent.
