# Where env / secrets live

**One rule:** real secrets never go in git. Templates end in `.example`.

| File | Tracked? | Used by | Notes |
|------|----------|---------|--------|
| `deploy/app.env` | **No** (gitignored) | Docker Compose on VPS + local overlay | **Canonical secrets file.** On server: `/opt/drivergo/deploy/app.env` (`chmod 600`). |
| `deploy/app.env.example` | Yes | Copy → `app.env` for local docker | Placeholders only. |
| `deploy/app.prod.env.example` | Yes | Prod port/URL shape | No real secrets. |
| `deploy/app.env.prod.generated` | **No** | Operator backup of generated prod secrets | Local machine only. |
| `backend/.env.example` | Yes | Documents every API `ENV` var | Do not put prod values here. |
| `frontend/.env.local` | **No** | Next.js local dev | `BACKEND_URL`, etc. |
| `frontend/.env.local.example` | Yes | Template for `.env.local` | |

## Production (VPS)

```text
/opt/drivergo/deploy/app.env
```

Contains: `DATABASE_URL`, `JWT_SECRET`, `CLIENT_IP_ASSERTION_SECRET`, MinIO,
`ADMIN_SEED_*`, `PUBLIC_BASE_URL`, ports `8081` / `3010`, and (when the bot is on)
`TELEGRAM_BOT_*` plus `TELEGRAM_WEBHOOK_SECRET` (self-generated random — not from BotFather).

Edit only on the server (or regenerate locally and `scp` with mode 600).

## Local docker overlay

```bash
cp deploy/app.env.example deploy/app.env
# edit, then:
docker compose -f docker-compose.yml -f deploy/docker-compose.app.yml \
  --env-file deploy/app.env up -d --build
```

## Find admin seed after prod bring-up

```bash
grep ADMIN_SEED_ /opt/drivergo/deploy/app.env   # on VPS
# or locally:
grep ADMIN_SEED_ deploy/app.env.prod.generated
```
