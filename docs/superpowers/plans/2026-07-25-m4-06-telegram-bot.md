# M4-06 — Telegram bot foundation — implementation plan

Design: `docs/superpowers/specs/2026-07-25-m4-06-telegram-bot-design.md`.
Branch: `feat/m4-06-telegram-bot`. TDD for every task that touches the
link/redeem logic (§3 of the design) — write the failing test first, per the
kickoff prompt's "TDD where logic is security-sensitive".

## Task 1 — Config plumbing

Add `TELEGRAM_BOT_TOKEN`, `TELEGRAM_BOT_API_BASE_URL`,
`TELEGRAM_BOT_USERNAME`, `TELEGRAM_BOT_MODE`, `TELEGRAM_WEBHOOK_SECRET` to
`internal/config.Config` + `Load()` + `validate()` (design §5.2).

- Test first (`config_test.go`): extend `configEnvKeys`; add cases to
  `TestLoadValidation` for: invalid mode, `longpoll`+`ENV=prod` rejected,
  `webhook` without token rejected, `webhook` without secret rejected,
  `webhook` with both present accepted, `off` with nothing set accepted
  (default, must not break `TestLoadDefaults`).
- Implement until green.

## Task 2 — Migration: `telegram_link_token`

- `internal/db/migrations/0020_telegram_link_token.up.sql` /
  `.down.sql` per design §5.
- No sqlc queries yet (Task 3 adds them) — just prove the migration applies:
  `go test ./internal/db/...` (runs `db_test.go`'s
  migrate-up-then-down-ish checks) plus manually running `make test` once
  Task 3's package exists will also exercise it via `testdb.New`.

## Task 3 — sqlc queries + generated code

`internal/db/queries/telegram.sql`:
- `CreateLinkToken` (:one, returns id/expires_at)
- `DeleteUnusedLinkTokensForProfile` (:exec)
- `GetLinkTokenByHashForUpdate` (:one, `FOR UPDATE`)
- `MarkLinkTokenUsed` (:exec)
- `GetTelegramAccountByTgUserID` (:one)
- `UpsertTelegramAccount` (:exec, `ON CONFLICT (profile_id) DO UPDATE`)

Run `make generate` (sqlc), commit generated `internal/db/sqlc/*` changes
alongside the `.sql` source — never hand-edit generated files.

## Task 4 — `internal/bot`: token helpers + `LinkService` (TDD core)

This is the security-sensitive core; write tests before implementation.

- `token.go`: `newOpaqueToken() string` (32 random bytes, base64url,
  no padding), `hashToken(raw string) string` (sha256 hex). Small, direct
  unit tests (deterministic hash, distinct random tokens, charset is
  URL-safe).
- `link.go`: `LinkService{Q *sqlc.Queries, Pool *pgxpool.Pool}` with:
  - `GenerateLinkToken(ctx, profileID) (LinkToken, error)` — deletes unused
    tokens for the profile, inserts a new one, returns the **raw** token +
    expiry (never returns the hash).
  - `RedeemLinkToken(ctx, rawToken string, tgUserID int64, username string) (RedeemOutcome, error)`
    — implements design §3.3/§3.4 exactly: `pool.Begin` +
    `GetLinkTokenByHashForUpdate`, status checks, existing-binding checks,
    upsert, mark-used, commit.
  - Sentinel errors: `ErrLinkTokenNotFound`, `ErrLinkTokenExpired`,
    `ErrLinkTokenAlreadyUsed`, `ErrTelegramAccountLinkedElsewhere`.
- `link_test.go` (testdb-backed, mirrors `leaderboard/service_test.go`'s
  `newTestService`/`createProfile` helpers), one test per row of design
  §3.3 plus §3.4/§3.2:
  1. `TestGenerateLinkToken_ReturnsUnexpiredToken`
  2. `TestGenerateLinkToken_InvalidatesPriorUnusedToken` (old token stops
     redeeming after a new one is generated for the same profile)
  3. `TestRedeemLinkToken_Success_NewBinding`
  4. `TestRedeemLinkToken_ExpiredRejected`
  5. `TestRedeemLinkToken_ReuseRejected` (redeem twice, second fails with
     `ErrLinkTokenAlreadyUsed`, binding from the first redemption is
     untouched)
  6. `TestRedeemLinkToken_UnknownTokenRejected`
  7. `TestRedeemLinkToken_SelfRelinkIsIdempotent` (same profile, same
     tg_user_id, already linked — redeeming a *new* token for that pair
     succeeds and does not error)
  8. `TestRedeemLinkToken_ProfileRelinksToNewTelegramAccount` (same profile,
     different tg_user_id than its current binding — old binding replaced)
  9. `TestRedeemLinkToken_RejectsHijackOfAnotherProfilesTelegramAccount`
     (profile B tries to redeem its own valid token, but the tg_user_id is
     already bound to profile A — must reject, must NOT modify A's binding,
     token is left unused)
  10. `TestRedeemLinkToken_ConcurrentRedeemOnlyOneWins` (`-race`, two
      goroutines redeem the *same* token concurrently for two different
      `tg_user_id`s — exactly one must succeed, the other must get
      `ErrLinkTokenAlreadyUsed`; this is the row-lock regression test for
      design §3.4, same spirit as `entitlement_race_test.go`)

## Task 5 — `internal/bot`: Telegram client

- `types.go`: minimal `Update`/`Message`/`User`/`Chat` structs (only fields
  read: `update_id`, `message.text`, `message.from.id`,
  `message.from.username`, `message.chat.id`).
- `client.go`: `Client{BaseURL, Token string; HC *http.Client}` with
  `SendMessage(ctx, chatID int64, text string) error`,
  `GetUpdates(ctx, offset int64, timeoutSec int) ([]Update, error)`,
  `SetWebhook`/`DeleteWebhook` (used by an operator's curl per design §7,
  but implemented here too since it's a two-line wrapper and useful for a
  future ops script).
- `client_test.go`: spin up an `httptest.Server` standing in for
  `api.telegram.org`, assert request shape (path includes token, body
  encoding) and response parsing, including a non-200/`ok:false` response
  mapping to an error (mirrors `auth.TelegramSender`'s test style — check
  `internal/auth/telegram_test.go` first for the existing convention).

## Task 6 — `internal/bot`: dispatcher (commands)

- `dispatcher.go`: `Bot{Link *LinkService, Billing billing.Service,
  Progress *progress.Service, TG *Client, BotUsername string}` +
  `HandleUpdate(ctx, Update) error`. Command parsing: split
  `update.Message.Text` on whitespace, switch on the first token
  (`/start`, `/start@BotUsername` — Telegram appends the bot username in
  group chats, strip it), `/link`, `/status`, default → help text.
  `/start`/`/link` extract the token from the remaining text.
- `dispatcher_test.go`: fake `TG` (records `SendMessage` calls instead of
  hitting the network) + testdb-backed `LinkService`/`billing.Service`/
  `progress.Service`. One test per command:
  - `/start` with no token, not yet linked → greeting text.
  - `/start <validtoken>` → success reply, binding created (assert via
    `GetTelegramAccountByTgUserID` or re-redeeming fails as already-used).
  - `/start <expiredtoken>` / `/start <bogus>` → user-facing failure reply,
    no panic, no partial state change.
  - `/link <token>` behaves identically to `/start <token>`.
  - `/status` unlinked → "not linked" reply.
  - `/status` linked, VIP active → reply mentions VIP + streak numbers
    (assert on substring, not exact copy, to keep the test resilient to
    wording tweaks).
  - unknown command → fallback reply, `HandleUpdate` returns nil (never
    errors out on user input, only on infra failure like a DB error).

## Task 7 — `internal/bot`: webhook + long-poll runners

- `webhook.go`: `Handler.Webhook(w, r)` — validates
  `X-Telegram-Bot-Api-Secret-Token` (constant-time compare via
  `crypto/subtle.ConstantTimeCompare`) against configured secret, 401s on
  mismatch/missing without touching the body; decodes JSON, calls
  `Bot.HandleUpdate` with a bounded-timeout context, always responds `200`
  once the header check passes (logs dispatch errors, does not surface them
  as 5xx — design §4's "never trigger a retry storm" note).
- `webhook_test.go`: missing/wrong secret → 401, dispatcher not invoked;
  correct secret + valid update → 200, dispatcher invoked with the decoded
  update; malformed JSON body → 200 (swallowed, logged) not 500 — a bad
  body must not make Telegram hammer retries either.
- `longpoll.go`: `RunLongPoll(ctx, tg *Client, bot *Bot, log *zap.Logger)` —
  loop: `GetUpdates(ctx, offset, 30)`, dispatch each, advance
  `offset = last update_id + 1`, back off (short sleep) on transport
  errors, return cleanly on `ctx.Done()`. No dedicated test beyond a short
  loop test with a fake client returning a couple of updates then an
  error then `ctx` cancellation, asserting offsets advance and the loop
  exits — long-poll against the real network is exercised manually per the
  "how to run" section of the final report, not in `go test`.

## Task 8 — Web endpoint: generate link token

- `handlers.go`: `Handler{Link *LinkService, BotUsername string}` +
  `AuthedRoutes(r chi.Router)` registering
  `POST /me/telegram/link-token`. Pulls `profileID` from
  `auth.FromContext` (401 if missing, matching every other authenticated
  handler in the codebase), calls `GenerateLinkToken`, responds
  `{token, deep_link, expires_at}` via `httpx.Data`.
- `handlers_test.go`: authenticated request → 200 with a token whose hash
  matches what's in the DB and a `deep_link` containing
  `https://t.me/<BotUsername>?start=<token>`; missing/invalid auth → 401
  (middleware-level, but assert the route is actually mounted behind
  `auth.Required` in `server.go`, not just in isolation).

## Task 9 — Wiring: `server.go` + `cmd/api/main.go`

- `server.go`: when `cfg.TelegramBotMode != "off"`, build `bot.LinkService`,
  `bot.Client`, `bot.Bot`; register `POST /api/v1/telegram/webhook` only
  when mode is `webhook`; always register the authenticated
  `POST /me/telegram/link-token` route whenever a bot token is configured
  (link tokens are useful to hand out even before the bot itself answers,
  though in practice mode will always be set together).
- `cmd/api/main.go`: when mode is `longpoll`, start
  `go bot.RunLongPoll(ctx, ...)` alongside the HTTP server, using the same
  `signal.NotifyContext` for shutdown.
- Manual smoke test (documented in the final report): `OTP_CHANNEL=sandbox`
  dev run, `TELEGRAM_BOT_MODE=off` — confirm nothing regresses when the bot
  is disabled (`go build ./...`, `make test`, hit `/healthz`).

## Task 10 — `.env.example` + whole-slice review

- Add the five new vars to `backend/.env.example` with the same
  comment style as the existing Telegram Gateway block, explicitly noting
  they're a *different* Telegram product than `TELEGRAM_GATEWAY_*`.
- Run `make test` (or `test-parallel`) full suite, `go vet ./...`,
  `gofmt -l .` clean.
- Re-read `link.go`/`webhook.go` once as a whole (not per-task) against
  design §3.1's threat model before calling this "done" — the standing
  instruction after every past M4 plan in this repo is that task-level
  review alone has repeatedly missed cross-cutting bugs (see
  `SESSION-HANDOFF.md` §6).

## Explicitly out of scope for this plan (M4-07 or later)

See design §1.2 / §6 — quiz, notifications, push, `/unlink`, FE button,
multi-language copy, automatic `setWebhook` call.
