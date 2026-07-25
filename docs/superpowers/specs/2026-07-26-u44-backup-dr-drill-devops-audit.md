# U-44 Backup + DR drill — devops audit

**Date:** 2026-07-26  
**Scope:** `pg_dump` + restore-drill scripts, runbook RPO/RTO placeholders. No fake host.

## Secrets / hygiene
- Dumps land under `.run/backups/` (gitignored).
- Scripts use local compose Postgres by default; `DATABASE_URL` optional for host-side `pg_dump`.
- Restore drill **refuses** target DB name equal to live `avtotest`.

## Safety
| Check | Result |
|-------|--------|
| Live DB wipe | blocked (`DRILL_DB != PGDATABASE`) |
| Seed wipe | not used |
| Invented staging hostname | none |

## Artifacts
| Path | Role |
|------|------|
| `scripts/backup/pg_dump.sh` | logical dump (`-Fc`) |
| `scripts/backup/pg_restore_drill.sh` | restore into `avtotest_restore_drill` |
| `scripts/backup/README.md` | operator notes |
| `make backup-pg` / `backup-restore-drill` | entrypoints |
| `deploy/STAGING-RUNBOOK.md` § Backup/DR | RPO/RTO placeholders |

## Remains
Off-site retention, scheduled jobs, WAL/PITR, Redis/MinIO DR, encrypted backup
transport — need real host (U-02) before claiming production DR.
