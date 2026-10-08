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
| D3 | Unlinked user: welcome screen → normal login/register forms, each with a «Raqamni Telegram'dan olish» button that pre-fills the phone **and supplies Telegram's signed contact** (the link proof, see D7) | Fewer typos, same validation. No "does this phone exist" endpoint (no new enumeration surface). |
| D4 | Keep the DriverGo design; follow Telegram's light/dark scheme and paint Telegram's header/background with our tokens | One design system; the Mini App looks like the site. |
| D5 | Sessions in Telegram use the same `at`/`rt` cookies issued `SameSite=None; Secure; Partitioned`, plus an `Origin` check on every unsafe BFF request | Only way to work inside web.telegram.org's iframe without weakening the site's `lax` cookies. |
| D6 | Logout inside the Mini App does **not** unlink Telegram; it turns auto-login off for that Telegram user | Bot digests and bot password reset depend on `telegram_account`; logout must not silently cost them. |
| D7 | A Telegram account is linked to a profile **only** when (a) fresh initData (≤ 1 h), (b) a fresh Telegram-signed `requestContact` response, (c) of the same Telegram user, (d) whose phone equals the profile's phone. Then the link may move from another profile and replace the profile's previous link (revised 2026-10-08, final review C1) | initData alone proves only "some Telegram account": anyone can paste their own into a phishing link (`#tgWebAppData=`), and the victim's sign-in would hand the attacker the account and the bot password reset. Telegram's signature over the profile's own phone cannot be phished. |

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
  - found but `phone_verified_at IS NULL` (a legacy `/start <token>` link, which
    proves nothing about who owns the profile) → `200 {need_phone: true,
    first_name}`, no session (revised 2026-10-08, audit-2 C1). One signed phone
    share upgrades the link.
  - found and phone-verified → load profile → `issueSession` (inside a tx, like `Login`) →
    `200 {access_token, refresh_token, must_change_password}`. A banned profile
    surfaces the existing `403 account_blocked`.
  - not found → `200 {need_phone: true, first_name}`.
- Errors: bad/expired init data → `401 invalid_init_data`; no bot token →
  `503 telegram_bot_unconfigured` (same code `TelegramLinkCard` already knows).

`issueSession` adds one refresh-token row, exactly like a new device. No other
session is touched, so this cannot reintroduce the revoke-all logouts.

### 1.3 `/auth/login` and `/auth/register` accept optional `tg_init_data` + `tg_contact`

Absent → byte-for-byte today's behaviour (site, native app, tests unchanged).

`tg_contact` is the raw `response` string of `WebApp.requestContact`
(`contact=<json {user_id, phone_number, ...}>&auth_date=…&hash=…`), signed by
Telegram exactly like initData. `ValidateContact(raw, botToken, now, maxAge)`
shares the initData HMAC code. Telegram contact phones are normalised strictly
(`998` + 9 digits, optional `+`), never as a bare 9-digit national number.

The link (`linkTelegramInTx`, in the sign-in transaction, inside a SAVEPOINT)
is written only when D7 holds: initData valid and ≤ `InitDataLinkMaxAge` (1 h),
contact valid and ≤ 1 h, `contact.user_id == initData user.id`, and the
contact phone == `profile.phone`. Then:

```
DELETE FROM telegram_account WHERE tg_user_id = $1 AND profile_id <> $2;  -- move (D7)
UpsertTelegramAccount(profile_id, tg_user_id, username)                      -- replace
```

A move logs `auth.telegram_link_moved`, a replacement
`auth.telegram_link_replaced` (profile ids only). Anything else never fails
the sign-in: the user is signed in, the link is skipped and
`auth.telegram_link_skipped` is logged with the reason. With
`TELEGRAM_WEBAPP_URL` empty (kill switch) linking is skipped silently. The
response gains `telegram_linked: bool` so the client knows whether to clear
D6's flag.

`POST /me/telegram/link-webapp {init_data, contact}` (learner auth; 20/h per
profile + the 30/h per-Telegram-user bucket) applies the same rule for a
learner who typed the phone: `200 {linked}` (a failed proof is `linked:false`,
never 401, which the BFF would read as an expired session);
`503 telegram_bot_unconfigured` when switched off. `GET /me/telegram` also
returns the linked `tg_user_id` (the learner's own data, used by §4.2/§4.4).

### 1.4 Bot menu button — `backend/internal/bot`

New config `TELEGRAM_WEBAPP_URL` (e.g. `https://drivergo.uz/uz-Latn/tg`). When
a bot token exists (whatever `TELEGRAM_BOT_MODE` is — the button lives on
Telegram's side and must follow the switch even with the bot off), startup
always calls `setChatMenuButton` once, best-effort (failure is logged, never
fatal): `{type:"web_app", text:"Ochish", web_app:{url}}` when the URL is set,
`{type:"default"}` when it is empty. So clearing the variable and restarting
is the kill switch; it also turns Mini App sign-in off
(`503 telegram_bot_unconfigured`) and all Telegram linking. `/start` replies gain
an inline `web_app` button with the same URL when configured.

`telegram_account` already has `UNIQUE(tg_user_id)` and `PRIMARY KEY(profile_id)`.
Migration 0076 (audit-2 C1, 2026-10-08) adds `telegram_account.phone_verified_at`
— set only by a D7 link or the bot reset's contact + «Ha, men»; the legacy
`/start <token>` redeem leaves it NULL and clears it when it re-points a row to
another Telegram user — and `password_reset_token.verified_tg_user_id` (who
confirmed the reset). `CompletePasswordReset` deletes the profile's link unless
it is phone-verified and belongs to that confirming Telegram user.
`DELETE /me/telegram` (learner auth, idempotent, `200 {unlinked: bool}`) is the
website unlink; `GET /me/telegram` also returns `phone_verified`.

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

- `hasTelegramHost()`: one of the channels telegram-web-app.js itself posts
  through — `window.TelegramWebviewProxy` (Android/iOS/new Desktop),
  `window.external.notify` (legacy Desktop) or a parent frame (web.telegram.org;
  CSP frame-ancestors admits no one else).
- `isTelegramMiniApp()`: a Telegram host **and** (`sessionStorage["tg-webapp"]
  === "1"` or the launch hash carries `tgWebAppData`). The hash alone is never
  enough: a planted `#tgWebAppData=` link in a plain browser is the website
  (C1). The flag is set by `/tg` only once the SDK is ready inside a host;
  `getWebApp()` also returns null without a host.
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

Fast path and identity: a live cookie session (`/api/proxy/me` 200) goes
straight in, unless `GET me/telegram` says the profile is linked to a
**different** `tg_user_id` than the launching user — then step 4 runs (the
launching user's linked profile replaces the session, or the welcome screen
on `need_phone`). Unlinked or unknown keeps the fast path.

Welcome screen: greeting with `first_name`, «Kirish» and «Ro'yxatdan o'tish»
buttons (to the existing pages), and «<first_name> sifatida davom etish» when
`autologin_off` is set and the account is linked.

`(auth)` is already public in the proxy guard; `tg` is not a protected segment.

### 4.3 Existing login / register pages in Mini App mode

- A «Raqamni Telegram'dan olish» button calls `WebApp.requestContact`; the
  shared `phone_number` is normalised and dropped into the phone field, and
  the signed `response` is kept.
- The submitted body adds `tg_init_data: WebApp.initData` and, when the phone
  was shared, `tg_contact: response`.
- On success with `telegram_linked`, CloudStorage `autologin_off` is removed.
- On success without a link and without a shared phone (and no forced
  password change), Telegram's share sheet is offered once, fire-and-forget
  after navigation; a shared number is posted to
  `/api/proxy/me/telegram/link-webapp`, and on `linked` `autologin_off` is
  removed. Declining is silent. (Telegram also drops the shared contact into
  the bot chat; with no password reset pending the bot stays quiet.)

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
| Plain browser with a planted `#tgWebAppData=` (phishing) | No Telegram host → website: no SDK, no `tg_init_data`; `/tg` shows "Botdan oching" (C1, client) |
| Mini App sign-in with someone else's launch data | No signed matching phone → signed in, not linked (C1, server) |
| init data older than 24 h, refresh token alive | No impact: requests use cookies; `/tg` only needed on re-auth |
| init data expired and session dead | `/tg` → `invalid_init_data` → "Botni qayta oching" |
| Cookie refused (Safari + web.telegram.org) | Probe catches it → "open on phone" screen |
| Link conflict race (two profiles at once) | DELETE+UPSERT in one tx; unique violation → link skipped, login succeeds, logged |
| Bot token missing | `/tg` shows "vaqtincha mavjud emas"; site unaffected |
| CloudStorage unavailable (old client) | Treated as auto-login on |
| Several Telegram accounts in one app share the webview cookie jar | The launching identity wins when the session's profile is linked to another `tg_user_id`; otherwise the live session's fast path |
| Kill switch (`TELEGRAM_WEBAPP_URL` empty) | `/tg` "vaqtincha mavjud emas", no linking, menu button reset |
| Shared phone after logout | Advisory: "continue as" signs back in with one tap; real removal = bot `/unlink` or website `DELETE /me/telegram` |
| Legacy `/start <token>` link (no phone proof) | Bot digests keep working; Mini App answers `need_phone` until one signed phone share (C1) |
| Password reset | Link kept only if phone-verified and owned by the Telegram user who confirmed the reset; otherwise deleted |

## 6. Testing

- **Go unit:** `ValidateInitData` — valid vector (signed in-test with a fixed
  token), tampered field, wrong token, missing hash, duplicate key, expired,
  future-dated, bad `user` JSON, `signature` field included in the check string.
- **Go integration (testdb):** webapp login linked / unlinked / banned / rate
  limited; each D7 condition failing alone → no link; matching proof links;
  phishing (attacker initData + attacker contact + victim phone/password) →
  signed in, not linked, attacker gets `need_phone`; move + replace; kill
  switch; `link-webapp`; invalid init data still logs in without linking;
  absent field leaves `telegram_account` untouched. `ValidateContact` has an
  independently computed known-answer vector.
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

Backend + frontend deploy together; migration 0076 (additive, nullable
columns). Existing links start unverified, so their Mini App auto-login asks
for one phone share. Rolling back to an image older than 0076 needs the
migrations rolled down first (see deploy/README.md). Set `TELEGRAM_WEBAPP_URL` in
prod env to turn the menu button on; clear it to turn it off. Optional manual
step in BotFather: "Configure Mini App" for `t.me/<bot>?startapp` links.
