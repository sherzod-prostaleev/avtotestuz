# Staging / Docker app path (U-02)

**Secrets map:** see [`ENV.md`](./ENV.md) — prod secrets live only in
`deploy/app.env` (gitignored). VPS sync without junk:
`./deploy/sync-to-vps.sh` (+ `rsync-exclude.txt`). It defaults to a remote
preflight and rsync dry-run; only reviewed `--apply` writes. The script validates
an exact allowlisted `/opt/...` target, rejects symlink/realpath drift, requires
a clean worktree, preserves `deploy/app.env`, creates a code-only rollback
snapshot, writes commit provenance, and never restarts containers.

Minimal path to run the **API + Next.js** images against the existing
postgres / redis / minio stack from the repo-root `docker-compose.yml`.

**Full operator guide:** [`STAGING-RUNBOOK.md`](./STAGING-RUNBOOK.md)
(registry push, host layout, health, rollback, D18 blockers).

Remote host provisioning (Open Decision **D18**) is still open — this folder
is host-agnostic: build images locally, or later push to a registry and pull
on the staging box. Do not invent fake hosts or DNS in commits.

## Images

| Image | Dockerfile | Default tag |
|-------|------------|-------------|
| Go API | `backend/Dockerfile` | `avtotest-api:local` (override `API_IMAGE`) |
| Next.js | `frontend/Dockerfile` | `avtotest-web:local` (override `WEB_IMAGE`) |

```bash
# From repo root
docker build -t avtotest-api:local -f backend/Dockerfile backend/
docker build -t avtotest-web:local -f frontend/Dockerfile frontend/
```

- API is a static binary (distroless); migrates on boot via embedded SQL and
  includes `/healthcheck` plus the one-shot, data-preserving `/encryptpan` tool.
- Frontend uses Next.js `output: "standalone"`.
- **No secrets are baked into either image** — inject via env / `deploy/app.env`.
- The protected env file is backend-only. Web receives an explicit four-variable
  allowlist rather than all API/database/payment secrets.

## Key URLs / env

| Variable | Who | Meaning |
|----------|-----|---------|
| `GET /healthz` | API | Liveness: `{"data":{"status":"ok"}}` |
| `BACKEND_URL` | Next (server) | Base URL of the Go API for BFF/`backendFetch` (e.g. `http://api:8080` on compose) |
| `PUBLIC_BASE_URL` | API | **Frontend** origin users browse — referral `invite_url`, payment return URLs. Not the API host. |
| `CLIENT_IP_ASSERTION_SECRET` | API + Next | Shared 32+ byte HMAC secret (required for `ENV=staging\|prod` and for the production Next image) |
| `TRUSTED_PROXY_HOPS` | Next | How many reverse-proxy hops append `X-Forwarded-For` before Next |

Full API env catalogue: `backend/.env.example`.  
Frontend local template: `frontend/.env.local.example`.

## Run against existing infra

```bash
make up                                          # postgres + redis + minio
cp deploy/app.env.example deploy/app.env         # edit if needed
docker compose -f docker-compose.yml -f deploy/docker-compose.app.yml \
  --env-file deploy/app.env up -d --build api web

./deploy/smoke.sh http://localhost:8080 http://localhost:3000
```

Validate compose merge without starting containers:

```bash
docker compose -f docker-compose.yml -f deploy/docker-compose.app.yml \
  --env-file deploy/app.env.example config >/dev/null
```

Content is **not** created by this overlay. Use your existing DB data (do not
`make seed` / `seed-real` / `seed-dev` as part of staging/prod bring-up unless
you intentionally want a content wipe).

### Content + images on the VPS

`backend/seed/avtoimtihon/images/` is **gitignored**. The real deploy path is
`./deploy/sync-to-vps.sh` (rsync) — that exclude list does **not** drop
`images/`, so the blob bundle lands under `/opt/drivergo/.../images/` for
`cmd/importer`. Never rely on `git pull` alone for question images.

Prod content refresh = Postgres backup → rsync → **upsert**
`go run ./cmd/importer -data seed/avtoimtihon -verified` (not `make seed-dev`).

## Hardening notes

- `restart: unless-stopped` on `api` and `web`.
- Log rotation (`json-file`, 10m × 3) on both app services.
- `api`, `web`, MinIO, and Humo watcher have healthchecks; API readiness and
  `smoke.sh` require Postgres, Redis, and private object storage.
- Optional CPU/memory limits are commented in the overlay — enable after VPS sizing.
- Secrets only via `app.env` / shell — never in YAML or images.
- App containers drop Linux capabilities, use `no-new-privileges`, read-only
  root filesystems plus bounded tmpfs, and separate app/data networks.

## Private support attachment migration

MinIO initialization uses `MINIO_SUPPORT_BUCKET`, the legacy `MINIO_BUCKET`, or
`support-attachments` in that order; it removes anonymous access
from the whole `media` bucket, and grants anonymous download only to
`media/images/*`. Existing `media/support/*` objects therefore become private
immediately but remain readable by the authenticated API fallback.

### Learner avatars and the Cloudflare cache

Telegram profile photos live under `media/images/avatars/`. nginx serves that
prefix with `max-age=3600` (the rest of `/media` keeps 7 days), because a
refreshed or removed photo's old object is deleted. Manual step: the Cloudflare
cache rule for `/media/images/*` must exclude `avatars/` or use an edge TTL of
at most one hour; otherwise Cloudflare keeps serving a deleted photo for up to
its own TTL.

After backing up the MinIO volume and deploying the new policy/API contract:

```bash
./deploy/migrate-support-bucket.sh          # inventory only
./deploy/migrate-support-bucket.sh --apply  # copy only; never deletes legacy
```

Verify old and new messages through learner/admin authenticated download routes.
The copy is idempotent for immutable UUID attachment keys: existing target keys
are skipped, never overwritten, and no remove operation exists. It intentionally
retains the legacy copy for rollback. Delete it only
in a separately approved maintenance window after object counts and downloads
are verified; then set `MINIO_LEGACY_SUPPORT_BUCKET=support-attachments`.

## App-only green/blue validation

`docker-compose.candidate.yml` starts one API and one web candidate on
`127.0.0.1:18081/13010`, reusing the existing `drivergo_default` data network.
It never starts a second Humo watcher or stateful service. Refs must be a digest
or a local content-addressed image ID already present on the host. Both app
containers use `restart: unless-stopped`, because either slot may temporarily
carry production traffic across a process, Docker-daemon, or host restart:

```bash
export CANDIDATE_API_IMAGE='registry.example/drivergo-api@sha256:<64-hex-digest>'
export CANDIDATE_WEB_IMAGE='registry.example/drivergo-web@sha256:<64-hex-digest>'
./deploy/candidate-app.sh up                 # validation only
CANDIDATE_EXPAND_CONTRACT_ACK=1 ./deploy/candidate-app.sh up --apply
./deploy/switch-app-slot.sh --to candidate   # health + diff only
./deploy/switch-app-slot.sh --to candidate --apply
# instant upstream rollback (containers stay running):
./deploy/switch-app-slot.sh --to stable --apply
```

The API self-migrates on startup. Candidate validation is safe only when every
schema change follows expand/contract and both current and candidate binaries
work against the expanded schema. Destructive/contract migrations require a
separate maintenance release. The candidate script pins the API to one replica
and never runs a separate migration command; only that process's normal startup
migration path executes.

## Real staging notes (`ENV=staging`)

`config.Load()` **rejects** `OTP_CHANNEL=sandbox`, the default `JWT_SECRET`,
empty `CLIENT_IP_ASSERTION_SECRET`, and localhost `PUBLIC_BASE_URL` when
`ENV=staging|prod`. Put Telegram Gateway (or a future SMS channel) + real
public origins in the host env — never commit them.

Put a reverse proxy (Caddy/nginx) in front of `web` so `X-Forwarded-For` is
set; otherwise production Next auth routes that build client-IP assertions
will return `network_error`.

## Smoke

`deploy/smoke.sh <api_base> [web_base]` checks:

1. `GET {api}/healthz` → ok envelope  
2. Optional: `GET {web}/uz-Latn` → HTTP 200  

Auth OTP sandbox round-trip is intentionally **not** required here (needs
seeded content + proxy headers); use API-level checks once the host is ready.

## Classroom station agent (B2B)

The API image cross-compiles the Windows agent and serves it from the admin
panel, so **any change under `backend/station/` ships only when `api` is
rebuilt** — rebuilding `web` alone leaves schools downloading the old binary.

The agent is built by its own `station` stage on **Go 1.20** and **GOARCH=386**
— Go 1.21 dropped Windows 7/8/Server 2008/2012, and driving-school classrooms
still run Windows 7, while a 64-bit binary refuses to start on a 32-bit PC.
That one build covers Windows 7 through 11 and both architectures. Do not
"upgrade" that stage to match the server toolchain without a Windows 7 PC to
test on: the failure is silent at build time and total at the school.

The version stamped into the agent comes from **`backend/station/VERSION`** —
nothing has to be typed at the command line, and there is no `STATION_VERSION`
environment variable to forget:

```bash
cd /opt/drivergo/deploy && \
  docker compose -f docker-compose.prod.yml --env-file app.env build api
```

It used to be a shell variable the operator had to remember on every build.
Nobody remembered it after 2026-08-07, so every rebuild shipped an agent that
reported `1.0.0`, `b2b_station.agent_version` said `1.0.0` for the whole fleet,
and a school on a known-broken build looked exactly like one on the fix. The
build now fails outright rather than stamping a placeholder, and the station
stage prints the version it is stamping:

```
station: building agent 1.0.9
```

Read that line out of the build output — it is the only place the version is
decided. After a classroom PC renews its token, the same string appears in
`b2b_station.agent_version` and in the admin panel's station list, which is how
you confirm the fleet actually moved.

### Where to look when a school says it does not work

Open the school in the admin panel. Every classroom PC shows what it last
reported — running, connecting, or **stopped and needing a human**, with the
reason in Uzbek and its agent version. Below the station list, "Ulanolmagan
kompyuterlar" lists machines that never became stations at all, with the tail
of their `station.log`; that section is empty when everything connected.

That is usually the whole investigation. If it is not, the same log is still at
`C:\ProgramData\AvtoTest\station\station.log` on the machine itself, and the
kiosk page shows the same state at `http://127.0.0.1:17817/station`.

### The fleet updates itself (agent 1.1.0 and later)

Installed classroom PCs poll `GET /api/v1/b2b/stations/agent-manifest` every
six hours, and install anything whose version differs from their own. So
shipping a fix to every school is just:

1. bump `backend/station/VERSION` in the same commit as the agent change
   (CI's `station-version-gate` fails the PR otherwise);
2. deploy with `build api` — **`build web` alone never updates the fleet**;
3. watch `b2b_station.agent_version` in the admin panel converge.

The swap is written to disk immediately but the running process is not killed
mid-lesson: the new binary takes over at the PC's next start, or sooner if the
kiosk has made no API call for 30 minutes. Expect a school to be fully migrated
by the morning after a deploy.

**Rolling back is the kill switch.** Any version difference triggers an update,
in both directions, so restoring the previous `drivergo-api` image walks every
classroom back to the agent inside it.

**PCs installed before 1.1.0 do not have this.** They must download the `.exe`
from the admin panel once more and run it; it reuses the existing
`station.key`, so no seat is consumed and no re-enrolment happens. After that
one manual step they keep themselves current.

## Telegram Mini App

The learner app opens inside Telegram from the bot's menu button and `/start`
launcher. Configuration lives next to the other `TELEGRAM_BOT_*` variables in
`deploy/app.prod.env.example`:

- `TELEGRAM_BOT_TOKEN`, `TELEGRAM_BOT_USERNAME`, `TELEGRAM_BOT_MODE` - the bot
  itself (the Mini App's sign-in validates Telegram's signed launch data with
  the bot token; without it `/tg` shows "vaqtincha mavjud emas").
  `TELEGRAM_BOT_USERNAME` is also passed to the **web** service: the payment
  return page `/checkout/done/<bot>` shows its "back to the bot" button only
  when `<bot>` is this username; unset, the page is text-only.
- `TELEGRAM_WEBAPP_URL` - the Mini App entry, `https://drivergo.uz/uz-Latn/tg`.
  It must be an absolute URL; the API refuses to start with anything but
  `https://` when `ENV=staging|prod` (Telegram opens only https Mini Apps).
  **Kill switch:** set it empty and restart the API. That stops Mini App
  sign-in (`/auth/telegram/webapp` answers `telegram_bot_unconfigured`, `/tg`
  shows "vaqtincha mavjud emas") and all Telegram linking, and resets the
  menu button to the default - also with `TELEGRAM_BOT_MODE=off`, as long as
  `TELEGRAM_BOT_TOKEN` is set (the menu button lives on Telegram's side and
  would otherwise outlive the switch). An empty value also resets any menu
  button set by hand in BotFather on that restart.
- Never share the production bot token with staging or dev: any process that
  boots with the token resets the bot's global menu button to its own
  `TELEGRAM_WEBAPP_URL`. Give each environment its own bot.
- Optional: BotFather "Configure Mini App" (`/newapp` or `/mybots` -> Bot
  Settings -> Configure Mini App) with the same URL, for the `t.me/<bot>/<app>`
  direct link.

Linking: a Telegram account is linked to a profile only when Telegram itself
vouches for the profile's phone - the learner shares their number through
Telegram's own sheet (`requestContact`), whose signed response must belong to
the same Telegram user and equal the profile phone. Launch data alone never
links (it can be copied into a phishing link). A learner who typed their
number is asked once, after sign-in, to share it; declining just leaves the
account unlinked (`auth.telegram_link_skipped` logs the reason).

Auto-login needs a **phone-verified** link (`telegram_account.phone_verified_at`,
migration 0076). Links made the old way (`/start <token>` from the website
link card) prove nothing about who owns the profile, so the Mini App answers
`need_phone` for them until the learner shares the phone once; bot digests and
`/status` keep working off them. All links existing before 0076 start
unverified, so every already-linked learner sees the phone sheet once after
this deploy - expected. A password reset deletes the link unless it is
phone-verified and belongs to the Telegram account that confirmed the reset
(`auth.telegram_link_dropped_on_reset`).

Shared phones: logout inside the Mini App is **advisory**. It turns
auto-login off for that Telegram account (CloudStorage `autologin_off`) and
clears the cookies, but the account stays linked (bot digests and the bot
password reset rely on it) and "continue as" signs back in with one tap.
Anyone holding the same unlocked Telegram account can do that. To really
take a Telegram account off a profile, unlink it in the bot (`/unlink`) or on
the website (`DELETE /me/telegram`, the profile's own link).

Framing: learner pages send CSP `frame-ancestors 'self' https://web.telegram.org`
and no `X-Frame-Options`; `/admin` keeps `frame-ancestors 'none'` + `DENY`
and does not allow the Telegram SDK origin in `script-src`.

Rollback: this feature adds migrations 0075 (`password_reset_token.confirm_nonce_hash`)
and 0076 (`telegram_account.phone_verified_at`, `password_reset_token.verified_tg_user_id`),
both additive. The API migrates up on every start, and golang-migrate refuses
to start when the database is at a version its embedded files do not know, so
an image older than 0076 **cannot (re)start** against a migrated database. Two
safe ways back:

- **Slot switch only** (`switch-app-slot.sh --to stable --apply`): the stable
  containers are already running and do not migrate again, and the old code
  works on the expanded schema. Do not restart them until the database is
  rolled down or the new image is back.
  **Before rolling forward again** (switching back to the new slot), run in
  the postgres container:

  ```sql
  UPDATE telegram_account SET phone_verified_at = NULL WHERE linked_at > phone_verified_at;
  ```

  Why: the old code's link upsert (`/start <token>`) knows nothing about
  `phone_verified_at`. Re-pointing a profile's link to a different Telegram
  account during the rollback window bumps `linked_at` but keeps the previous
  account's proof, so after the roll-forward that new Telegram account would
  be signed in to the Mini App without ever proving the phone (the takeover
  0076 exists to stop). New code always writes `linked_at` and
  `phone_verified_at` in the same statement, so a link newer than its proof
  can only come from the old code or from a same-account legacy re-link;
  clearing the latter too costs that learner one phone share, nothing more.
- **Roll the schema down first**, then start the older image: apply the down
  files newest first (`0076_*.down.sql`, then `0075_*.down.sql` for an image
  older than 0075) with `psql` in the postgres container and set
  `UPDATE schema_migrations SET version = <target>, dirty = false` (75 or 74).
  Down drops the columns: every link's phone proof and any open bot reset
  question are lost - learners re-share the phone / restart the reset.

### Manual device checklist (before announcing)

Run on Android, iOS, Telegram Desktop and web.telegram.org:

1. Open from the bot (menu button and `/start`).
2. Linked account: auto-login lands on the dashboard.
3. Unlinked: phone login with "Raqamni Telegram'dan olish" links the account;
   a typed phone gets Telegram's share-number sheet once after sign-in.
   Sharing it links; declining stays unlinked without an error.
4. Logout, then reopen: "continue as" is offered, not forced.
5. Start an exam, press the system/back close: the closing confirmation appears.
6. Payment hand-off: the checkout opens Payme/Click in the external browser and the return page (`/checkout/done/<bot>`) leads back to the bot.
7. Phishing check: open `https://drivergo.uz/uz-Latn/login#tgWebAppData=x` in
   a normal browser: no "Raqamni Telegram'dan olish" button, website as usual.

A real **staging Payme/Click payment from inside the Mini App must be done
once** before announcing; the e2e suite stubs the backend and cannot prove it.

## Telegram daily reminder («Kun savoli»)

Every day from **19:00 Asia/Tashkent** the api process DMs every bot user
(table `telegram_bot_user`: anyone who wrote to the bot, tapped its buttons or
started/unblocked it; seeded by migration 0078 from linked accounts and solo
`/quiz` chats) one quiz poll plus a personal line (streak / due reviews /
comeback / signup trial) and two buttons («📝 Bugungi mashq», «🔕 Eslatmalarni
o'chirish»). Users toggle it with `/eslatma`.

- **Enable:** admin → settings → flags → `telegram_daily_reminder` (seeded
  **off**). It also needs `TELEGRAM_BOT_TOKEN` and `TELEGRAM_BOT_MODE` =
  `webhook` (prod) or `longpoll`; with mode `off` nothing is sent, because
  nobody would answer the opt-out button. Turning the flag off mid-run stops
  the run within 25 recipients.
- **Window:** a pass starts at the first minute tick at/after 19:00 and stops
  at 21:00 — it never sends between 21:00 and 09:00, even when catching up
  after downtime. Whoever is not reached by 21:00 is skipped for that day.
- **Safety:** one pass at a time (Postgres advisory lock); each recipient is
  claimed by setting `last_reminder_on = today` in the same UPDATE, so a crash
  or redeploy mid-run resumes the rest and never sends twice (the one user
  being sent to at the crash instant may miss that day).
- **Rate limits:** 25 messages/s overall; a 429 waits Telegram's
  `retry_after`; 403 (blocked) and 400 "chat not found" set `blocked_at` and
  the user is skipped until they write to the bot again; other 4xx are logged
  and skipped; 5xx/transport errors retry twice, then skip that user for today.
- **Log:** one `telegram daily reminder: run` line per pass with counts only
  (eligible, pending, sent, blocked, opted_out, errors, interrupted, duration).
- **Dry run on prod** (sends and writes nothing):
  `docker exec <api container> /tgdigest --dry-run` — prints the audience by
  segment (streak/due/inactive/unlinked/generic), opted-out and blocked
  counts, and today's question id with whether it fits poll limits.
  Locally: `make tg-digest`.
- **Superseded:** the old linked-only due digest (`tgdigest -send`, flag
  `telegram_dm_digest`) is removed; migration 0078 deletes that flag.
- **Rollback:** `0078_telegram_daily_brief.down.sql` drops `telegram_bot_user`
  and `telegram_daily_question` (opt-outs are lost) and restores the old flag.

## CI implications

- Image builds are not yet a required CI job (keep PRs light). Operators build
  before deploy; adding a `workflow_dispatch` build/push once D18 secrets exist
  is documented in the runbook.
- Do not commit `deploy/app.env` (gitignored).

## Load-test smoke (U-42)

```bash
make load-test                          # needs k6 + running API
# docs: deploy/load-test/README.md
```

## Docker build cache (disk)

BuildKit keeps every layer it has ever built and reclaims none of it. On
2026-08-27 that was 503 entries holding **30 GB** on a 72 GB disk that had
reached 77% full; pruning them took it to 35%.

Read the `Build Cache` row, not the `Images` row. `docker system df` reported
"Images 34 GB, 28.89 GB reclaimable (84%)" and pointed at 64 accumulated image
tags — but with the cache gone those images occupied 5.25 GB in total, 80 MB of
it reclaimable, because their layers are shared with the running ones. Deleting
old `rollback-*` tags would have freed almost nothing and cost the ability to
roll back.

A weekly timer keeps it bounded (Sunday 03:40 UTC, clear of the 02:15 backup),
keeping the last week so incremental builds stay fast:

```bash
install -m 0755 deploy/prune-build-cache.sh /opt/drivergo/deploy/
install -m 0644 deploy/systemd/drivergo-prune-build-cache.service /etc/systemd/system/
install -m 0644 deploy/systemd/drivergo-prune-build-cache.timer   /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now drivergo-prune-build-cache.timer
systemctl start drivergo-prune-build-cache.service   # run once now
journalctl -u drivergo-prune-build-cache.service -n 20
```

Weekly rather than after each deploy: deploys here are manual and irregular, so
a schedule catches the growth whoever ran the build and whether or not they
remembered. Run it by hand any time with `deploy/prune-build-cache.sh`;
`BUILD_CACHE_KEEP` overrides the one-week window.
