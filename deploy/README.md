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
