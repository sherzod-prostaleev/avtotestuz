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

Contains: `DATABASE_URL`, `JWT_SECRET`, `DATA_ENCRYPTION_KEY`,
`CLIENT_IP_ASSERTION_SECRET`, MinIO, `ADMIN_SEED_*`, `PUBLIC_BASE_URL`, ports
`8081` / `3010`, and (when the bot is on) `TELEGRAM_BOT_*` plus
`TELEGRAM_WEBHOOK_SECRET` (self-generated random — not from BotFather).

Edit only on the server (or regenerate locally and `scp` with mode 600).

## The two keys: `JWT_SECRET` vs `DATA_ENCRYPTION_KEY`

They used to be one value, and that made rotating a signing key an act of
data destruction. They are now separate:

| | `JWT_SECRET` | `DATA_ENCRYPTION_KEY` |
|---|---|---|
| Protects | learner + admin JWTs | data at rest (AES-GCM KEK) |
| Rotating it | signs everyone out; nothing else | **destroys data permanently** |
| Safe to rotate on a schedule? | Yes, once the other is set | **No. Never.** |

`DATA_ENCRYPTION_KEY` seals:

* `admin_user.totp_secret_enc` — every admin's authenticator
* `manual_pay_card.pan_full`, `referral_payout.card_number` — card numbers
* `manual_tg_settings.*_enc` — Telethon `api_id` / `api_hash` / session

**Rotating it makes every one of those permanently undecryptable.** There is
no migration, no re-encryption tool and no recovery: admins would have to
re-enroll TOTP, stored PANs could never be revealed again, and the Telegram
userbot session would have to be created from scratch. Treat a change to this
value the same way you would treat `DROP COLUMN`.

## Service-level secret scope

`deploy/app.env` is the operator's single protected source, but Compose does
not inject it into every container. The API receives backend secrets; Humo gets
only its API URL/ingest token; web receives only `BACKEND_URL`,
`CLIENT_IP_ASSERTION_SECRET`, `TRUSTED_PROXY_HOPS`, and its public telemetry
setting. Do not restore `web.env_file: app.env`: that would expose DB, JWT,
payment, Telegram, and data-encryption secrets to the frontend process.

Support attachments use `MINIO_SUPPORT_BUCKET=support-attachments`. If that
new variable is absent, the API honors the previous `MINIO_BUCKET` value before
using `support-attachments`, so an existing private bucket is never orphaned. Public course
images remain under `media/images/*`; the bucket policy does not anonymously
serve `media/support/*`. `MINIO_LEGACY_SUPPORT_BUCKET=media` is an authenticated
read fallback until the copy migration is verified. Afterwards it may be set
equal to `MINIO_SUPPORT_BUCKET` to disable fallback.

**Empty means "use `JWT_SECRET`"** — that is exactly how every row already in
the database was encrypted, which is why this change needs no migration.

### Adopting the split on a running host

```bash
# 1. On the VPS, copy the CURRENT value across — do not invent a new one.
grep '^JWT_SECRET=' /opt/drivergo/deploy/app.env      # -> <current>
# 2. Add (do not replace):
#    DATA_ENCRYPTION_KEY=<current>
# 3. Redeploy. Nothing on disk changes: the app derives the same KEK.
# 4. Only now is JWT_SECRET free to rotate — it costs a sign-out, no data.
```

A brand-new host with no encrypted rows yet can instead generate an
independent random value (`openssl rand -base64 48`).

`cmd/encryptpan` resolves the key the same way (`DATA_ENCRYPTION_KEY`, else
`JWT_SECRET`). Running it with a different key than the API writes ciphertext
the API cannot read back.

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
