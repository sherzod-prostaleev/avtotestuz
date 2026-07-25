# U-44 — Postgres backup + restore drill

Local/operator scripts for **logical** Postgres backup (`pg_dump -Fc`) and a
**non-destructive restore drill** into `avtotest_restore_drill`.

No remote hostnames are invented. Point compose / `DATABASE_URL` at infrastructure
you actually run (`make up`).

## Dump

```bash
make up
./scripts/backup/pg_dump.sh
# → .run/backups/avtotest-YYYYMMDD-HHMMSS.dump (+ avtotest-latest.dump)
```

## Restore drill

```bash
./scripts/backup/pg_restore_drill.sh
# or: ./scripts/backup/pg_restore_drill.sh .run/backups/avtotest-….dump
```

Restores into **`avtotest_restore_drill` only**. Refuses if `DRILL_DB` equals the
live DB name (`avtotest`).

## Makefile

```bash
make backup-pg
make backup-restore-drill
```

## RPO / RTO placeholders

See `deploy/STAGING-RUNBOOK.md` § Backup / DR (U-44). Fill real targets when a
staging/prod host and off-box retention exist (blocked on U-02).

## Out of scope (honest)

- Continuous WAL archiving / PITR
- Off-site object-storage replication
- Redis / MinIO media DR (separate later)
- Automated scheduled backups on a VPS you do not have yet
