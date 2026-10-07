# Telegram Mini App

2026-10-07. The learner app opens inside Telegram as a Mini App from the
existing bot's menu button. A learner whose Telegram account is linked lands on
the dashboard with no form at all; everyone else signs in or registers with
their phone number exactly as on the website, and the Telegram account is
linked as a side effect so the next open is instant.

The phone number stays the identity. Telegram is an extra way *into* an
existing account, never a second kind of account.

## Decisions (owner delegated the remaining choices, 2026-10-07)

| # | Decision | Why |
|---|---|---|
| D1 | Payments inside the Mini App keep today's card / Payme / Click flow | Owner's call. Telegram policy wants Stars for digital goods; the risk is to the bot, not the website. Revisit later. |
| D2 | Host the Mini App on the existing bot (`TELEGRAM_BOT_USERNAME`) | Its token already validates the link flow; users already know it. |
| D3 | Unlinked user: welcome screen → normal login/register forms, each with a «Raqamni Telegram'dan olish» button that pre-fills the phone | Fewer typos, same validation. No "does this phone exist" endpoint (no new enumeration surface). |
| D4 | Keep the DriverGo design; follow Telegram's light/dark scheme and paint Telegram's header/background with our tokens | One design system; the Mini App looks like the site. |
| D5 | Sessions in Telegram use the same `at`/`rt` cookies issued `SameSite=None; Secure; Partitioned`, plus an `Origin` check on every unsafe BFF request | Only way to work inside web.telegram.org's iframe without weakening the site's `lax` cookies. |
| D6 | Logout inside the Mini App does **not** unlink Telegram; it turns auto-login off for that Telegram user | Bot digests and bot password reset depend on `telegram_account`; logout must not silently cost them. |
| D7 | Phone login/register from inside the Mini App **moves** the Telegram link to that profile | The person proved both the Telegram account and the profile's password; "this Telegram = this account" is their intent. |

## Non-goals

- Telegram Stars payments (D1).
- Changing any behaviour of the website, the native mobile app, the kiosk or
  the admin panel. Every new backend field is optional; every frontend change
  is gated on "running inside Telegram".
- A Telegram-only account type, or skipping phone + password for new users.

## 1. Backend (`backend/internal/auth`)

### 1.1 `initData` validation — `telegram_webapp.go`

`ValidateInitData(raw, botToken string, now time.Time, maxAge time.Duration) (WebAppUser, error)`

Telegram's documented algorithm:

1. Parse `raw` as a URL query. Reject a duplicated key (`ErrInitDataInvalid`).
2. Remove `hash`; build `data_check_string` from **every** remaining pair
   (including `signature`), sorted by key, `key=value` joined by `\n`.
3. `secret = HMAC_SHA256(key="WebAppData", msg=bot_token)`;
   `expected = hex(HMAC_SHA256(key=secret, msg=data_check_string))`.
4. `hmac.Equal` against `hash` (constant time).
5. `auth_date` must parse, must not be more than 1 minute in the future, must
   not be older than `maxAge` (24 h) → `ErrInitDataExpired`.
6. `user` must be JSON with a positive `id` → `WebAppUser{ID, FirstName, Username, LanguageCode}`.

Nothing outside the signed string is trusted. Empty bot token →
`ErrTelegramUnconfigured`.

### 1.2 `POST /auth/telegram/webapp` `{init_data}`

`Service.TelegramWebAppLogin(ctx, initData, ip)`:

- Rate limit: `tgwebapp:tg:<id>` 30/h and `tgwebapp:ip:<ip>` 60/h (same Limiter).
- `GetTelegramAccountByTgUserID`:
  - found → load profile → `issueSession` (inside a tx, like `Login`) →
    `200 {access_token, refresh_token, must_change_password}`. A banned profile
    surfaces the existing `403 account_blocked`.
  - not found → `200 {need_phone: true, first_name}`.
- Errors: bad/expired init data → `401 invalid_init_data`; no bot token →
  `503 telegram_bot_unconfigured` (same code `TelegramLinkCard` already knows).

`issueSession` adds one refresh-token row, exactly like a new device. No other
session is touched, so this cannot reintroduce the revoke-all logouts.

### 1.3 `/auth/login` and `/auth/register` accept optional `tg_init_data`

Absent → byte-for-byte today's behaviour (site, native app, tests unchanged).

Present → validated with 1.1. On success the link is written **in the same
transaction** as the session (`linkTelegramInTx`):

```
DELETE FROM telegram_account WHERE tg_user_id = $1 AND profile_id <> $2;  -- D7
UpsertTelegramAccount(profile_id, tg_user_id, username)
```

A moved link is logged (`auth.telegram_link_moved`, profile ids only). Invalid
init data never fails the login: the user is signed in, the link is skipped,
`auth.telegram_link_skipped` is logged with the reason. The response gains
`telegram_linked: bool` so the client knows whether to clear D6's flag.

### 1.4 Bot menu button — `backend/internal/bot`

New config `TELEGRAM_WEBAPP_URL` (e.g. `https://drivergo.uz/uz-Latn/tg`). When
the bot is enabled, startup always calls `setChatMenuButton` once,
best-effort (failure is logged, never fatal): `{type:"web_app", text:"Ochish",
web_app:{url}}` when the URL is set, `{type:"default"}` when it is empty. So
clearing the variable and restarting is the kill switch. `/start` replies gain
an inline `web_app` button with the same URL when configured.

No migration. `telegram_account` already has `UNIQUE(tg_user_id)` and
`PRIMARY KEY(profile_id)`.

## 2. BFF (`frontend/src/app/api`, `frontend/src/lib`)

### 2.1 Cookie mode — `auth-cookies.ts`

```ts
type CookieMode = "site" | "telegram";
setAuthCookies(res, tokens, mode = "site")
clearAuthCookies(res, mode = "site")
cookieModeFor(request): CookieMode   // "telegram" iff TG_MODE_COOKIE ("tgp") present
```

`telegram` mode writes `at`, `rt` and the marker `tgp=1` with
`sameSite: "none", secure: true, partitioned: true`. Clearing must repeat
`partitioned` or the browser keeps the partitioned cookie — so **every**
set/clear call site (proxy, refresh, logout, login, register, telegram) passes
`cookieModeFor(request)`. The site never sees `tgp`, so its cookies are
unchanged.

Browsers that ignore `Partitioned` (iOS WKWebView in the Telegram app) store a
plain `SameSite=None` cookie in the Telegram app's own cookie jar — still
first-party there, still covered by 2.2.

### 2.2 Origin guard — `lib/same-origin.ts`

`rejectCrossSite(request): NextResponse | null` for every non-GET/HEAD request
in `api/proxy` and `api/auth/*`:

- `Origin` present → its host must equal the request `Host` (nginx forwards
  `$host`), else `403 cross_site`.
- `Origin` absent → `Sec-Fetch-Site: cross-site` → 403; otherwise allow
  (non-browser callers cannot attach a victim's cookies anyway).

This is what keeps another Mini App inside web.telegram.org from riding our
`SameSite=None` cookies, and it is free hardening for the site.

### 2.3 `POST /api/auth/telegram`

Forwards `{init_data}` to 1.2 with the client-IP assertion headers. Tokens →
`setAuthCookies(res, tokens, "telegram")`, body
`{data:{ok:true, must_change_password}}`. `need_phone` passes through. Login and
register routes use `"telegram"` mode when the JSON body carries
`tg_init_data`.

## 3. Security headers (`next.config.mjs`)

- Global CSP: `frame-ancestors 'self' https://web.telegram.org`;
  `script-src` adds `https://telegram.org`. `X-Frame-Options` is dropped
  globally (it cannot express an allow-list).
- `/:locale/admin/:path*` and `/api/admin/:path*` additionally send
  `Content-Security-Policy: frame-ancestors 'none'` and `X-Frame-Options: DENY`.
  Multiple CSP headers are intersected by browsers, so admin stays unframeable.

## 4. Frontend

### 4.1 Detection and SDK loading — `lib/telegram/`

- `isTelegramMiniApp()`: `sessionStorage["tg-webapp"] === "1"` or the launch
  URL hash carries `tgWebAppData`. Set by `/tg` on first load; a webview reload
  keeps sessionStorage; the plain website never sets it.
- `TelegramProvider` (mounted once in `app/providers.tsx`): when detected,
  injects `https://telegram.org/js/telegram-web-app.js` and exposes a typed
  `useTelegram()` (`webApp | null`). Outside Telegram it renders children and
  nothing else — no script, no work on the website.

### 4.2 Entry route `/[locale]/tg` — in the existing public `(auth)` group

Splash (logo + spinner) while it:

1. Calls `WebApp.ready()`, `expand()`, `disableVerticalSwipes()`.
2. Locale: if no `NEXT_LOCALE` cookie yet, `language_code` `ru` → `/ru/tg`,
   else stay on `uz-Latn` (no prefetch of other locales — NEXT_LOCALE trap).
3. Reads CloudStorage `autologin_off`. If set → welcome screen.
4. `POST /api/auth/telegram`: tokens → `router.replace(next || "/dashboard")`
   (`next` must be a same-origin path starting with `/` and not `//`);
   `need_phone` → welcome screen; `invalid_init_data` → "Botdan oching" screen
   with a `t.me/<bot>` link; network error → retry button.
5. After success, probes `GET /api/proxy/me`. A 401 means the browser refused
   the cookie (Safari on web.telegram.org) → "Telefoningizdagi Telegram'da
   oching" screen instead of a broken app.

Welcome screen: greeting with `first_name`, «Kirish» and «Ro'yxatdan o'tish»
buttons (to the existing pages), and «<first_name> sifatida davom etish» when
`autologin_off` is set and the account is linked.

`(auth)` is already public in the proxy guard; `tg` is not a protected segment.

### 4.3 Existing login / register pages in Mini App mode

- A «Raqamni Telegram'dan olish» button calls `WebApp.requestContact`; the
  shared `phone_number` is normalised and dropped into the phone field.
- The submitted body adds `tg_init_data: WebApp.initData`.
- On success with `telegram_linked`, CloudStorage `autologin_off` is removed.

### 4.4 Chrome inside Telegram (`TelegramChrome`, client only)

- **Theme:** `colorScheme` drives next-themes (`setTheme`); `themeChanged`
  follows live. Header, background and bottom bar colours are set from our
  `--background` token so Telegram's frame matches the page.
- **Safe area:** `html.tg-webapp` adds
  `var(--tg-content-safe-area-inset-*)` to the existing
  `env(safe-area-inset-*)` paddings (top bar, tab bar, full-screen runner).
- **BackButton:** shown on any route that is not a tab root
  (dashboard/tickets/practice/arena/profile); click → `router.back()`, falling
  back to `/dashboard` when history is empty.
- **Closing confirmation:** enabled only on `/session/*` and exam runners;
  vertical swipes stay disabled app-wide.
- **Haptics:** `useHaptics()` → `selectionChanged` on answer pick,
  `notificationOccurred("success"|"error")` on graded answer; no-op outside
  Telegram.
- **Links:** external URLs open via `openLink`, `t.me` via
  `openTelegramLink`; Payme/Click hand-off uses `openLink` (their pages refuse
  to be framed).
- **Hidden in Mini App:** PWA install prompts, service-worker registration,
  the «Telegram'ga ulash» card (replaced by "Telegram ulangan" status).

### 4.5 Session expiry and logout

- `SessionExpiredGate` in Mini App mode → `/tg?next=<current path>` (silent
  re-auth via init data) instead of `/login`.
- Logout in Mini App mode → `CloudStorage.setItem("autologin_off","1")`, then
  the normal logout (partitioned cookies cleared via 2.1), then `/tg`.

## 5. Failure modes

| Situation | Behaviour |
|---|---|
| Opened as a plain URL in a browser | `/tg` shows "Botdan oching"; nothing else changes |
| init data older than 24 h, refresh token alive | No impact: requests use cookies; `/tg` only needed on re-auth |
| init data expired and session dead | `/tg` → `invalid_init_data` → "Botni qayta oching" |
| Cookie refused (Safari + web.telegram.org) | Probe catches it → "open on phone" screen |
| Link conflict race (two profiles at once) | DELETE+UPSERT in one tx; unique violation → link skipped, login succeeds, logged |
| Bot token missing | `/tg` shows "vaqtincha mavjud emas"; site unaffected |
| CloudStorage unavailable (old client) | Treated as auto-login on |

## 6. Testing

- **Go unit:** `ValidateInitData` — valid vector (signed in-test with a fixed
  token), tampered field, wrong token, missing hash, duplicate key, expired,
  future-dated, bad `user` JSON, `signature` field included in the check string.
- **Go integration (testdb):** webapp login linked / unlinked / banned / rate
  limited; login + register with `tg_init_data` link; link moves from another
  profile; invalid init data still logs in without linking; absent field leaves
  `telegram_account` untouched.
- **Vitest:** cookie modes (attributes on set and clear); `cookieModeFor`;
  refresh keeps telegram mode; origin guard matrix; telegram route; login route
  picks mode from body; headers config (admin DENY, others Telegram-only);
  proxy-guard public groups; `TelegramProvider` renders nothing outside
  Telegram; BackButton root/non-root; contact pre-fill.
- **Playwright:** `/tg` with a stubbed `window.Telegram.WebApp` and a signed
  init data from a test bot token at 390×844 — linked → dashboard; unlinked →
  welcome → login → linked; no duplicated tab bar, nothing clipped.
- **Gates:** `go test ./...`, `golangci-lint run`, `npm run lint`,
  `npx tsc --noEmit`, `npx vitest run`, e2e; `rm -rf frontend/.next` first.
- **Live:** after deploy, open from the bot on Android, iOS, Desktop and
  web.telegram.org.

## 7. Rollout

Backend + frontend deploy together; no migration. Set `TELEGRAM_WEBAPP_URL` in
prod env to turn the menu button on; clear it to turn it off. Optional manual
step in BotFather: "Configure Mini App" for `t.me/<bot>?startapp` links.
