# U-44 — Postgres backup + restore drill

Operator scripts for **logical** Postgres backup (`pg_dump -Fc`), optional
streaming age encryption/off-site rclone upload, and a **non-destructive
restore drill** into `avtotest_restore_drill`.

No remote hostnames are invented. Point compose / `DATABASE_URL` at infrastructure
you actually run (`make up`).

## Dump

```bash
make up
./scripts/backup/pg_dump.sh
# → .run/backups/avtotest-YYYYMMDD-HHMMSS.dump (+ avtotest-latest.dump)
```

Production (plaintext is never written):

```bash
AGE_RECIPIENT='age1…' REQUIRE_ENCRYPTED_BACKUP=1 \
RCLONE_REMOTE='drivergo-offsite:drivergo/postgres' REQUIRE_OFFSITE_BACKUP=1 \
COMPOSE='docker compose -f deploy/docker-compose.prod.yml --env-file deploy/app.env' \
BACKUP_DIR=/var/backups/drivergo/postgres ./scripts/backup/pg_dump.sh
```

Install `deploy/systemd/drivergo-backup.{service,timer}` only after creating
`/etc/drivergo/backup.env` (0600) with a real age recipient and tested rclone
remote. The service fails closed when encryption or off-site config is absent.

## Restore drill

```bash
AGE_IDENTITY_FILE=/secure/off-host/drivergo-age-key.txt \
  ./scripts/backup/pg_restore_drill.sh /path/to/avtotest-….dump.age
# or: ./scripts/backup/pg_restore_drill.sh .run/backups/avtotest-….dump
```

Restores into **`avtotest_restore_drill` only**. Refuses if `DRILL_DB` equals the
live DB name (`avtotest`).

## Makefile

```bash
make backup-pg
make backup-restore-drill
```

## RPO / RTO

The committed timer provides a maximum scheduled logical-backup interval of
24 hours. A business-approved RPO/RTO and retention period still require an
off-site provider and measured restore duration; until then those values are
`insufficient evidence`, not promises.

## Out of scope (honest)

- Continuous WAL archiving / PITR
- Off-site provider credentials/provisioning (the rclone hook is implemented)
- Redis / MinIO media DR (separate later)
- Automated scheduled backups on a VPS you do not have yet
