# PostgreSQL PITR / RPO reduction — design (P1-E)

Date: 2026-08-10  
Status: design only — **not implemented in this change window**

## Why separate

Daily encrypted snapshots give a technical RPO up to ~24h15m. True RPO
reduction needs WAL archiving, restore-time testing, and business approval of
RPO/RTO. Mixing this into an app rollout or backup-script change risks
production data paths.

## Proposed approach (next reviewed change)

1. Enable `archive_mode` + `archive_command` (or pgBackRest/barman) writing
   WAL segments to an encrypted off-host store (same age/rclone policy family).
2. Base backups remain the four-component snapshot or a dedicated PG basebackup.
3. Restore drill: new isolated Postgres → restore base → replay WAL to a
   target time → validate core tables → destroy scratch.
4. Document measured RPO/RTO after a timed isolated-host recovery including
   secrets, DNS, cutover, and reconciliation.
5. Object-store: enable provider object lock/versioning when cloud off-site
   exists; keep MinIO live-volume tar as crash-consistent media until
   storage-native replication is available.

## Non-goals for now

- No production `postgresql.conf` change in this remediation landing.
- No claim of minute-level RPO until drills pass.

## Exit condition

P1-E closes only after business RPO/RTO sign-off and a passing timed
isolated-host PITR drill with evidence retained off the immutable snapshot.
