# Staging / Docker app path (U-02)

**Secrets map:** see [`ENV.md`](./ENV.md) — prod secrets live only in
`deploy/app.env` (gitignored). VPS sync without junk:
`./deploy/sync-to-vps.sh` (+ `rsync-exclude.txt`).

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
`make seed` / `seed-real` as part of staging bring-up unless you intentionally
want fixture data).

## Hardening notes

- `restart: unless-stopped` on `api` and `web`.
- Log rotation (`json-file`, 10m × 3) on both app services.
- `api` and `web` both have HTTP healthchecks; `smoke.sh` remains the external
  end-to-end check.
- Optional CPU/memory limits are commented in the overlay — enable after VPS sizing.
- Secrets only via `app.env` / shell — never in YAML or images.

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
