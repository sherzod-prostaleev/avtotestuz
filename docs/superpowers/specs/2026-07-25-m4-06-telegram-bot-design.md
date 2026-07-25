# M4-06 — Telegram bot foundation — design

Roadmap: `docs/superpowers/2026-07-24-roadmap-m2-to-admin.md` §3, row M4-06
(`Telegram bot — poydevor (auth-link, komandalar)`, no dependency, BE only).
Follow-up: M4-07 (daily quiz + notifications), depends on this plan.

## 1. Scope

### 1.1 Goals for M4-06 (this wave)

- A running bot process (webhook in staging/prod, long-poll behind a flag in
  dev) that answers `/start`, `/link`, `/status`.
- A secure way to bind a Telegram account to an existing AvtoTest profile —
  the **auth-link flow** — without ever trusting a `tg_user_id` supplied by
  an untrusted client.
- Config plumbing (`internal/config`) and a new package (`internal/bot`) that
  M4-07 can extend with quiz/notification sending, without redesigning the
  link/auth layer.
- `telegram_account` (already exists, migration `0002`) becomes populated
  through a real, tested flow instead of sitting unused.

### 1.2 Explicitly deferred to M4-07

- Daily quiz question delivery, scheduling, streak reminders.
- Any outbound push initiated by the *server* (cron/worker sending
  unsolicited messages). This wave only replies to inbound updates.
- Inline keyboards / callback queries, rich formatting, localization of bot
  copy beyond a single language (uz-Latn, matching the rest of the product's
  default).
- Rate-limiting bot commands beyond what's needed to keep `/link` abuse-safe
  (see §3.4). General command flood protection is a quiz-era concern once
  the bot does more than answer three commands.
- A `GET /me/telegram` web endpoint to show link status in the frontend
  (nice-to-have, no FE work in this wave per the roadmap; `/status` in the
  bot already covers "am I linked").
- Un-linking (`/unlink`). Documented as a TODO; low risk to leave out because
  re-linking (see §3.3) already lets a user move the binding to a new
  Telegram account.

## 2. Why a new package, not `internal/auth`

`internal/auth` owns phone+OTP+JWT session issuance — a different trust
boundary (a phone number the user types in, verified by a code sent over a
channel we chose). The bot's trust boundary is Telegram's own servers
(webhook secret / outbound long-poll connection we initiate), and its
identity source is a `tg_user_id` Telegram asserts to us. Bundling the two
would blur that boundary. `internal/bot` depends on `internal/auth` (for
`auth.Required` on the one authenticated web endpoint) and on
`internal/billing` / `internal/progress` (read-only, for `/status`), but
`auth` has zero dependency back on `bot`.

## 3. Auth-link flow

### 3.1 Threat model

The one thing this design must prevent: **a Telegram user binding
themselves to somebody else's AvtoTest profile**, and the mirror case, **an
attacker forging a `tg_user_id` over HTTP** to claim a profile they don't
control on Telegram. Concretely:

- A public HTTP endpoint that accepts `{profile_id, tg_user_id}` and writes
  `telegram_account` would let anyone link any profile to any Telegram
  account by guessing/enumerating IDs — a `tg_user_id` typed into a web form
  is unauthenticated data, full stop.
- Telegram never asks us to *prove* a bot update is real beyond: (a) for
  webhooks, the `X-Telegram-Bot-Api-Secret-Token` header we set at
  `setWebhook` time round-trips on every call, and (b) for long-poll, the
  update came back on an HTTPS connection *we* opened to
  `api.telegram.org` with our bot token — nobody else can inject updates
  into that stream. Both are enough to trust `update.message.from.id` as
  "the Telegram account that sent this", but neither says anything about
  *which AvtoTest profile* that person is.

### 3.2 Design: link token as the single source of truth for "which profile"

1. An **already-authenticated** web/app user (valid JWT) calls
   `POST /api/v1/me/telegram/link-token`. The server:
   - deletes any unused link tokens already issued to that profile (keeps
     the table small and avoids "which of my 5 links is live" confusion —
     only the newest is valid),
   - generates 24 random bytes, base64url-encodes them (`NewOpaqueToken`,
     mirrors `auth.NewRefreshToken`'s shape so it fits Telegram's `/start`
     payload charset: `[A-Za-z0-9_-]`, ≤64 bytes — ours is 32),
   - stores **only the sha256 hash** of the token (`telegram_link_token`
     table; same "never store the secret itself" rule as `refresh_token`),
     with `profile_id`, `expires_at = now()+10m`,
   - returns `{token, deep_link, expires_at}` where
     `deep_link = https://t.me/<BOT_USERNAME>?start=<token>` — **only to
     that authenticated caller**, over the same TLS connection as the rest
     of the API.
2. The profile_id is baked into the token server-side at generation time.
   **There is no HTTP "redeem" endpoint that accepts a profile_id or
   tg_user_id from a client.** Redeeming happens entirely inside the bot's
   own update dispatcher (§4), which:
   - extracts `tg_user_id`/`username` from `update.message.from` — trusted
     per §3.1,
   - extracts the token from `/start <token>` or `/link <token>` text the
     same trusted update carries,
   - looks the token up by hash, locks the row (`FOR UPDATE`, mirroring the
     `LockProfileForGrant` pattern in billing — see §3.4), checks
     `used_at IS NULL AND expires_at > now()`,
   - binds `telegram_account` for **the profile the token was minted for**,
     never a profile the Telegram side names.

   This is why there is no separate "redeem endpoint" as an HTTP route: the
   bot process *is* the redeemer, in-process, and the only inputs it trusts
   (the token, and Telegram's own assertion of `tg_user_id`) are exactly the
   two things that must both be true for a legitimate link. Exposing the
   same logic over unauthenticated HTTP would reopen the hole in §3.1.
3. This is the "CSRF-ish binding" the kickoff prompt asked for: the token is
   only ever revealed to the profile that generated it (authenticated
   response body, not a query param that ends up in logs/referrers), it is
   single-use, and it expires quickly. Anyone who obtains the raw token
   (screenshot, shared clipboard) *could* link their own Telegram to the
   victim's profile before the victim does — same risk class as a password
   reset link, and mitigated the same way: short TTL, single generation
   endpoint requires an already-valid session, and the bot's success/failure
   reply always names which profile just got linked so a victim notices a
   surprise link immediately. Full CSRF-token-in-double-submit-cookie
   defenses do not apply here since the "form" is a Telegram deep link, not
   a browser form.

### 3.3 Binding semantics (what `RedeemLinkToken` actually does)

`telegram_account.profile_id` is the primary key and `tg_user_id` is
`UNIQUE` (migration `0002`, unchanged). Given a valid, unexpired, unused
token minted for profile `P`, and an inbound `tg_user_id = T`:

| Existing state | Action |
|---|---|
| No row for `T`, no row for `P` | Insert `(P, T)`. |
| Row `(P, T)` already exists (same profile re-links the same Telegram account) | Update `username`/`linked_at` — idempotent no-op success. This is the "self-binding" case: binding a profile to a Telegram account it is *already* bound to must succeed, not error, since a user re-opening their own stale deep link is normal, not an attack. |
| Row `(P, T2)` exists, `T2 != T` (profile re-links to a *different* Telegram account) | Update to `(P, T)`, replacing the old binding. Foundation-wave choice: one Telegram account per profile, last link wins, no explicit `/unlink` needed (§1.2). |
| Row `(P2, T)` exists, `P2 != P` (this Telegram account is already linked to **somebody else's** profile) | **Reject.** Do not overwrite. This is the hijack case from §3.1 — an attacker who obtains a stranger's link token must not be able to detach that stranger's Telegram from the stranger's account. The token is left unused so the legitimate token owner can still complete their own link with a fresh attempt if they want to (rare/edge; does not weaken anything since the token was never going to expire faster by trying). |

The last row is checked twice: once as a normal `SELECT` before the
`UPDATE` (fast, friendly error), and once as the actual database
`UNIQUE(tg_user_id)` constraint the `UPSERT` would violate if two
concurrent redemptions raced for the same `tg_user_id` under two different
tokens — the same "app-level check plus DB-constraint backstop" pattern
already used for `promo_redemption` (see `SESSION-HANDOFF.md` §1.1.1).
Catching `23505` and mapping it to the same "already linked elsewhere"
error is enough; no separate row lock on `telegram_account` is needed
because Postgres's unique index enforces the invariant atomically.

### 3.4 Concurrency: locking the link-token row

Redeeming is check-then-act (read token status, then write). Two Telegram
updates for the same token arriving within the same second (double-tap on
the deep link, Telegram's own retry of a slow webhook response) must not
both pass the `used_at IS NULL` check before either writes it. `RedeemLinkToken`
therefore runs inside `pool.Begin` + `SELECT ... FOR UPDATE` on the
`telegram_link_token` row before checking status — the same
"transaction + row lock" shape `billing.LockProfileForGrant` and the promo
redemption path already use. This is the one place in this feature where
money-critical-style rigor applies even though no money moves: a lost
update here means a **wrong profile silently gets no linked Telegram, or
the wrong two accounts end up cross-linked** — a support/trust problem, not
a financial one, but "TDD where logic is security-sensitive" applies
regardless.

### 3.5 Rate limiting

`POST /me/telegram/link-token` is behind `auth.Required`, so abuse is
bounded by "how many JWTs can one profile mint tokens with" rather than
being open to the internet. No additional limiter in this wave — if this
becomes a vector (e.g. token-table bloat from spamming the endpoint), the
existing `auth.Limiter`/Redis cooldown pattern from OTP requests is the
obvious extension point, deferred since deleting old unused tokens on every
new generation (§3.2 step 1) already caps the table at one row per profile.

## 4. Bot commands (M4-06 scope)

Plain text only, uz-Latn, no inline keyboards.

- **`/start`** (no payload) — greeting + short explanation + instructions to
  open the AvtoTest app/site and use the "Telegram bilan bog'lash" action to
  get a link. If already linked, greets by name instead.
- **`/start <token>`** (Telegram's deep-link payload convention) —
  equivalent to `/link <token>`, handled identically. This is the path a
  real user takes: tapping the `deep_link` from §3.2 opens the bot with the
  payload pre-filled.
- **`/link <token>`** — manual fallback (paste the token as text) for anyone
  whose Telegram client mangles deep links, and the one exercised directly
  in bot-level tests without simulating a deep link tap.
- **`/status`** — if not linked: says so, points at `/start`. If linked:
  reports VIP active/until (`billing.Service.Status`) and streak
  current/best (`progress.Service.GetStreak`) — both already-computed, cheap
  reads, no new queries beyond the existing services.
- Anything else — a one-line "unknown command, try /start" fallback. Never
  silently drops an update (Telegram will retry webhook non-2xx responses,
  which is a separate reason to always return 200 once the update is
  durably queued/handled).

## 5. Architecture

```
internal/bot/
  token.go        newOpaqueToken() / hashToken() — mirrors auth/jwt.go's
                   NewRefreshToken/HashToken shape, kept local to avoid a
                   cross-domain naming mismatch (these aren't refresh tokens).
  link.go         LinkService{Q, Pool}: GenerateLinkToken, RedeemLinkToken,
                   sentinel errors (ErrTokenNotFound/Expired/AlreadyUsed/
                   ErrTelegramLinkedElsewhere).
  link_test.go    TDD: expiry, reuse, hijack-rejection, self-relink,
                   re-link-to-new-account. testdb-backed.
  client.go       Client{BaseURL, Token, HC}: SendMessage, GetUpdates,
                   SetWebhook, DeleteWebhook — minimal REST calls, same shape
                   as auth.TelegramSender (no external bot SDK dependency).
  types.go        Update/Message/User/Chat — only the fields we read.
  dispatcher.go   Bot{Link, Billing, Progress, TG}.HandleUpdate(ctx, Update):
                   parses command text, calls the right handler, sends the
                   reply via TG.SendMessage. This is where §3.2's "redeem
                   happens in-process" lives.
  dispatcher_test.go  Command routing against a fake Client + testdb.
  webhook.go      HTTP handler: checks X-Telegram-Bot-Api-Secret-Token,
                   decodes body, calls HandleUpdate, always replies 200
                   (Telegram-facing errors are logged, not surfaced as 5xx,
                   to avoid infinite webhook retries on a bad update).
  longpoll.go     RunLongPoll(ctx, ...): dev-only loop calling GetUpdates
                   with an increasing offset; cancels cleanly on ctx.Done().
  handlers.go     Handler{Link, BotUsername}: the one authenticated web
                   route, POST /me/telegram/link-token.
  handlers_test.go
```

New DB objects (migration `0020`, prefixing off the existing `19`):

```sql
CREATE TABLE telegram_link_token (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  profile_id uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  token_hash text NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  used_at    timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX telegram_link_token_profile_idx ON telegram_link_token(profile_id);
```

`telegram_account` (migration `0002`) is reused as-is — no schema change
needed there, confirming the kickoff prompt's hunch.

### 5.1 Webhook vs long-poll

Config-driven, one flag (`TELEGRAM_BOT_MODE=off|webhook|longpoll`):

- `off` (default) — no bot wiring at all; `internal/bot` types compile but
  nothing calls Telegram. Safe default for environments without a bot
  token.
- `webhook` — `server.New` registers `POST /api/v1/telegram/webhook`
  guarded by the secret-token header. Intended for staging/prod behind a
  public HTTPS URL. **This wave does not auto-call `setWebhook`** — that's
  a one-time `curl` (documented in §7) run by whoever deploys, same spirit
  as Payme/Click credentials being env-supplied rather than
  programmatically registered.
- `longpoll` — `cmd/api` starts a background goroutine calling
  `bot.RunLongPoll` instead of registering the webhook route. For local dev
  behind NAT/no public URL. Rejected at config-validation time when
  `ENV=prod` (long-poll is a single-consumer model; running it from more
  than one instance means only one gets updates, and Telegram's own docs
  warn against mixing webhook+long-poll for the same bot).

### 5.2 Config additions (`internal/config`)

| Var | Default | Notes |
|---|---|---|
| `TELEGRAM_BOT_TOKEN` | `""` | Bot API token from @BotFather. **Different product from `TELEGRAM_GATEWAY_TOKEN`** (Gateway API is OTP-delivery-only and cannot send arbitrary messages or receive updates) — do not conflate the two even though both come from Telegram. |
| `TELEGRAM_BOT_API_BASE_URL` | `https://api.telegram.org` | Overridable so tests/dispatcher logic can point at a fake server instead of the real Bot API. |
| `TELEGRAM_BOT_USERNAME` | `""` | No leading `@`. Used only to build the `deep_link` in the link-token response. |
| `TELEGRAM_BOT_MODE` | `off` | `off\|webhook\|longpoll`. |
| `TELEGRAM_WEBHOOK_SECRET` | `""` | Compared against `X-Telegram-Bot-Api-Secret-Token`. Required when mode is `webhook`. |

Validation additions to `Config.validate()`:
- `mode` must be one of the three values.
- `mode != off` requires `TELEGRAM_BOT_TOKEN`.
- `mode == webhook` requires `TELEGRAM_WEBHOOK_SECRET`.
- `mode == longpoll` is rejected when `ENV == prod` (see §5.1).

## 6. What NOT to build in this wave (recap)

- Quiz content, scheduling, or any cron/worker that initiates outbound
  messages (M4-07).
- Push notification infra (M4-08, separate roadmap row, web push — unrelated
  transport anyway).
- `/unlink` command or endpoint.
- Frontend "connect Telegram" UI/button (BE only per roadmap; the API
  contract — `POST /me/telegram/link-token` → `{token, deep_link,
  expires_at}` — is enough for a future FE task to wire a button to).
- Multi-language bot copy.
- Automatic `setWebhook` registration from application code.

## 7. Operational notes

Setting the webhook once a staging/prod instance is deployed with
`TELEGRAM_BOT_MODE=webhook`:

```bash
curl -s "https://api.telegram.org/bot$TELEGRAM_BOT_TOKEN/setWebhook" \
  -d "url=https://<api-host>/api/v1/telegram/webhook" \
  -d "secret_token=$TELEGRAM_WEBHOOK_SECRET"
```

Local dev: set `TELEGRAM_BOT_MODE=longpoll` and `TELEGRAM_BOT_TOKEN` in
`.env`; no public URL or webhook call needed. `TELEGRAM_WEBHOOK_SECRET` is
irrelevant in this mode.
