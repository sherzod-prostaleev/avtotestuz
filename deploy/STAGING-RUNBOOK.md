# Staging deploy runbook (D18 / U-02)

Operator guide for building, pushing, and running the **API + Next.js** stack
on a Docker host. This runbook is **host- and registry-agnostic**: fill in
placeholders — do not invent a provider/DNS until Open Decision **D18** is
answered with a real VPS + registry.

Companion files:

| File | Role |
|------|------|
| `docker-compose.app.yml` | App overlay (api + web) on root infra compose |
| `app.env.example` | Committed env **template** → copy to host-only `app.env` |
| `smoke.sh` | Post-deploy health checks |
| `README.md` | Local overlay quick start |

**Never commit** `deploy/app.env`, registry credentials, SSH keys, or JWT/OTP
secrets.

---

## 0. What you still need (D18 blockers)

Nothing below is provisioned in this repo. An operator must supply them:

1. **Docker host** — a VPS (or laptop for dry-run) with Docker Engine + Compose
   v2, inbound ports for the reverse proxy (typically 80/443), and SSH access.
2. **Container registry** — e.g. GHCR (`ghcr.io/<owner>/…`), Docker Hub, or a
   private registry reachable from the host. Placeholders below use
   `$REGISTRY` (example shape: `ghcr.io/example-org`).
3. **Public origins** — real `PUBLIC_BASE_URL` (frontend users browse) and the
   API URL the proxy exposes. `ENV=staging` **rejects** localhost public URLs
   and sandbox OTP (see `backend` config validation).
4. **Secrets on the host** — materialize `deploy/app.env` from a secret store /
   password manager; never bake into images or git.
5. **OTP channel for real staging** — Telegram Gateway (or future SMS), not
   `sandbox`. Local dry-run may keep `ENV=dev` + `OTP_CHANNEL=sandbox`.
6. **Content** — overlay does **not** seed. Bring your own DB data; do **not**
   run `make seed` / `seed-real` as part of routine deploy unless you
   intentionally want fixture/content reload.

Until (1)–(3) exist, stop after **local validate/build** (§6). Do not fake
hosts in CI workflows.

---

## 1. Image contract

| Image | Dockerfile | Local tag | Registry tag (example) |
|-------|------------|-----------|------------------------|
| Go API | `backend/Dockerfile` | `avtotest-api:local` | `$REGISTRY/avtotest-api:$GIT_SHA` |
| Next.js | `frontend/Dockerfile` | `avtotest-web:local` | `$REGISTRY/avtotest-web:$GIT_SHA` |

- Config is **100% runtime env**. Images contain no `DATABASE_URL`, JWT, or
  payment keys.
- API self-migrates on boot (embedded SQL). Bringing the container up **is**
  the migration path.
- Frontend is Next.js `output: "standalone"`.

```bash
# From repo root — build
export GIT_SHA="$(git rev-parse --short HEAD)"
docker build -t "avtotest-api:${GIT_SHA}" -t avtotest-api:local -f backend/Dockerfile backend/
docker build -t "avtotest-web:${GIT_SHA}" -t avtotest-web:local -f frontend/Dockerfile frontend/
```

---

## 2. Registry push (when `$REGISTRY` exists)

```bash
# Authenticate to your registry first (gh auth token / docker login / cloud CLI).
docker tag "avtotest-api:${GIT_SHA}" "$REGISTRY/avtotest-api:${GIT_SHA}"
docker tag "avtotest-web:${GIT_SHA}" "$REGISTRY/avtotest-web:${GIT_SHA}"
docker push "$REGISTRY/avtotest-api:${GIT_SHA}"
docker push "$REGISTRY/avtotest-web:${GIT_SHA}"

# Optional floating tag for “current staging”
docker tag "$REGISTRY/avtotest-api:${GIT_SHA}" "$REGISTRY/avtotest-api:staging"
docker tag "$REGISTRY/avtotest-web:${GIT_SHA}" "$REGISTRY/avtotest-web:staging"
docker push "$REGISTRY/avtotest-api:staging"
docker push "$REGISTRY/avtotest-web:staging"
```

On the host, either:

- set `API_IMAGE` / `WEB_IMAGE` in `app.env` to the pushed refs and use
  `image:`-only pulls, or
- keep building on the host from a git checkout (slower; fine for first bring-up).

The overlay defaults to `avtotest-api:local` / `avtotest-web:local` for laptop
dry-runs. Override via env (see §3).

---

## 3. Host layout

Recommended layout on the staging box (paths are suggestions, not mandatory):

```text
/opt/avtotest/
  docker-compose.yml          # from repo root (postgres/redis/minio)
  deploy/
    docker-compose.app.yml
    app.env                   # mode 600, host-only — from app.env.example
  # optional: Caddyfile / nginx site in front of web:3000
```

```bash
cp deploy/app.env.example deploy/app.env
chmod 600 deploy/app.env
# Edit: JWT_SECRET, CLIENT_IP_ASSERTION_SECRET, PUBLIC_BASE_URL, OTP, …

# Optional image overrides after registry push:
# API_IMAGE=$REGISTRY/avtotest-api:staging
# WEB_IMAGE=$REGISTRY/avtotest-web:staging
```

**Secrets rule:** compose only interpolates `${…}` from the env file / shell.
No secret literals in YAML. Payment / Telegram / ops tokens stay empty until
needed; empty Payme/Click keys keep webhooks rejecting (safe default).

---

## 4. Bring-up / update

```bash
cd /opt/avtotest   # or your checkout

# Infra once (persistent volumes)
docker compose -f docker-compose.yml up -d

# Apps (build local tags, or pull if API_IMAGE/WEB_IMAGE point at registry)
docker compose -f docker-compose.yml -f deploy/docker-compose.app.yml \
  --env-file deploy/app.env up -d --pull missing api web

# Health
./deploy/smoke.sh "https://api.example.invalid" "https://app.example.invalid"
# Local dry-run:
./deploy/smoke.sh http://localhost:8080 http://localhost:3000
```

Put **Caddy/nginx** (or similar) in front of `web` so `X-Forwarded-For` is set.
Production Next auth routes that build client-IP assertions return
`network_error` without a trusted proxy — set `TRUSTED_PROXY_HOPS` to match.

`restart: unless-stopped` is set on both app services. Resource ceilings are
documented as comments in the overlay (uncomment when the host size is known).

---

## 5. Health checks & rollback

### Health

| Check | Expect |
|-------|--------|
| `GET {API}/healthz` | JSON envelope with `"status":"ok"` |
| `GET {WEB}/{locale}` e.g. `/uz-Latn` | HTTP 200 |
| Compose | `api` / `web` `running` (`restart: unless-stopped`) |

`deploy/smoke.sh` covers the two HTTP checks. In-container Docker
`HEALTHCHECK` for the API image is omitted (distroless has no curl/shell);
rely on smoke + process restart policy.

### Rollback

1. Note the previously working image digest/tag (`$GIT_SHA` or `:staging-prev`).
2. Point `API_IMAGE` / `WEB_IMAGE` (or retag `:staging`) at the last-good refs.
3. Re-run compose `up -d` for `api` `web` only — infra volumes stay untouched.
4. Re-run `smoke.sh`.
5. If a bad migration shipped: **do not** wipe volumes. Restore Postgres from
   backup (U-44) or fix-forward with a new migration. Staging DB wipe is an
   explicit operator choice, never part of this runbook’s happy path.

```bash
# Example pin to a previous SHA
# API_IMAGE=$REGISTRY/avtotest-api:abc1234
# WEB_IMAGE=$REGISTRY/avtotest-web:abc1234
docker compose -f docker-compose.yml -f deploy/docker-compose.app.yml \
  --env-file deploy/app.env up -d api web
./deploy/smoke.sh "$API_PUBLIC" "$WEB_PUBLIC"
```

---

## 6. Local smoke (no remote host)

Validates compose merge + optional image builds on a developer machine:

```bash
# Config only (no daemon pull of app images required for interpolate check)
docker compose -f docker-compose.yml -f deploy/docker-compose.app.yml \
  --env-file deploy/app.env.example config >/dev/null

# Optional: build images (needs Docker build resources; skip if offline)
docker build -t avtotest-api:local -f backend/Dockerfile backend/
docker build -t avtotest-web:local -f frontend/Dockerfile frontend/

# Optional full local stack (uses existing DB data — no seed)
make up
cp -n deploy/app.env.example deploy/app.env
docker compose -f docker-compose.yml -f deploy/docker-compose.app.yml \
  --env-file deploy/app.env up -d api web
./deploy/smoke.sh http://localhost:8080 http://localhost:3000
```

---

## 7. CI implications

- Image build/push and SSH deploy are **not** required CI jobs yet (keeps PRs
  light). Prefer `workflow_dispatch` once `STAGING_HOST` / registry secrets
  exist.
- Do not store staging secrets in the workflow YAML — use GitHub Environments /
  Actions secrets only after D18.
- Playwright CI (`E2E_AUTH_TOKEN`) is independent of this host; see
  `frontend/e2e` + inventory U-14.

---

## 8. Checklist before calling staging “up”

- [ ] Host reachable over SSH; Docker + Compose installed
- [ ] Registry login works from CI or operator laptop
- [ ] `app.env` on host (`chmod 600`), not in git
- [ ] `ENV=staging` → non-sandbox OTP, strong `JWT_SECRET`, shared
      `CLIENT_IP_ASSERTION_SECRET` (32+ bytes), non-localhost `PUBLIC_BASE_URL`
- [ ] Reverse proxy + `TRUSTED_PROXY_HOPS` correct
- [ ] `smoke.sh` green against public URLs
- [ ] Rollback tag/digest recorded
- [ ] Content strategy decided (existing DB vs one-time import) — no casual seed
