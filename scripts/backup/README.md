# Driver Go backup and disaster-recovery runbook

This directory implements one full snapshot containing PostgreSQL, Redis,
MinIO, and the Humo watcher's SQLite queue. Production fails closed: the
committed systemd unit invokes immutable `--production` policy, which requires
age encryption, a verified off-site rclone copy, and off-site retention even
if an EnvironmentFile accidentally attempts to disable them.

The scripts never contain credentials. Keep `/opt/drivergo/deploy/app.env`,
`/etc/drivergo/rclone.conf`, the age private identity, and the recovery copy of
`DATA_ENCRYPTION_KEY` (or its legacy `JWT_SECRET` fallback) in an approved
secret manager. The age private identity must not be stored on the VPS that
creates backups.

## What is captured

| Component | Capture method | Restore-drill behavior |
| --- | --- | --- |
| PostgreSQL | `pg_dump -Fc`, logical and internally consistent | Restores only to hard-coded `avtotest_restore_drill`, validates core tables, then drops it |
| Redis | `redis-cli --rdb`, streamed from a container temp file | Offline `redis-check-rdb` by default; opt-in runtime boots the scratch RDB in a digest-pinned, network-isolated ephemeral Redis and records `PING`/`DBSIZE` |
| MinIO | Complete `/data` Docker volume tar stream | Guarded extraction by default; opt-in runtime boots only the extracted scratch copy and records readiness, bucket metadata count, and recursive inventory count |
| Humo queue | SQLite online backup API | Opens the copy read-only/immutable with the real SQLite engine, runs full `integrity_check`, validates `pending_ingest`, and records its row count |

PostgreSQL, Redis, and SQLite each provide a component-consistent capture.
They are taken sequentially, so the four components are **not** one distributed
point-in-time transaction. The MinIO live-volume tar is crash-style media
coverage, not a substitute for MinIO versioning/replication; active writes can
span the copy window. Quiesce writers for the strongest planned recovery, or
add provider-native object replication before claiming a strict media RPO.

## Snapshot guarantees

`backup_all.sh`:

- serializes backup and retention with `flock`;
- writes to an allowlisted `.partial-drivergo-*` directory, then atomically
  renames it only after all four artifacts exist;
- encrypts each payload as it streams with a public age recipient (no plaintext
  staging on the host);
- emits `manifest.txt` plus strict `SHA256SUMS`, and verifies them locally;
- re-verifies every local retention candidate, moves checksum-invalid snapshots
  to the protected sibling `quarantine` directory, and never counts them toward
  the minimum;
- uploads the complete immutable snapshot to a dedicated rclone prefix and
  runs `rclone check --checksum`, then writes a `REMOTE_COMPLETE` marker tied
  to the checksum-file digest;
- retains 14 local days and 35 off-site days by default, while preserving at
  least three snapshots. Only exact `drivergo-YYYYMMDDTHHMMSSZ` directories
  below the configured roots can be removed; remote retention additionally
  downloads and SHA-256 verifies the newest required minimum (falling back to
  older snapshots on bitrot) before any older off-site copy can be pruned;
  requires the manifest, checksums, and post-verification completion marker.
  Failed-upload remote directories without that marker expire after seven
  days; power-loss/SIGKILL local partials expire after 24 hours.

The checksum manifest detects corruption and accidental changes. Age provides
confidentiality and ciphertext integrity, but it is not a sender signature:
an attacker who can write the remote and knows the public recipient can replace
a whole snapshot and its checksums. Provider access controls and object lock
remain required for origin/authenticity assurance. Plaintext development mode
has neither confidentiality nor cryptographic integrity.

## Production installation

Prerequisites on the host: Docker Compose, `age`, `rclone`, `flock`, GNU
coreutils/tar, and enough local scratch capacity for one full snapshot.

Provision these files/directories as root:

```bash
install -d -m 0700 /var/backups/drivergo/full /etc/drivergo
install -m 0600 deploy/backup.env.example /etc/drivergo/backup.env
# Replace the example recipient/remote. Install a separately provisioned
# /etc/drivergo/rclone.conf with mode 0600; do not put it in Git.
install -m 0644 deploy/systemd/drivergo-backup.service /etc/systemd/system/
install -m 0644 deploy/systemd/drivergo-backup.timer /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now drivergo-backup.timer
systemctl start drivergo-backup.service
systemctl status drivergo-backup.service
journalctl -u drivergo-backup.service --since today
```

Run `rclone lsd drivergo-offsite:drivergo/full` and a restore drill before
treating the schedule as operational. Route a failed
`drivergo-backup.service` unit to the real paging/notification system; this
repository cannot invent that external destination.

For an intentional local-only exercise, plaintext must be opted into explicitly:

```bash
docker compose -f docker-compose.yml -f deploy/docker-compose.app.yml \
  --env-file deploy/app.env up -d --build api humo-watcher
COMPOSE_FILE="$PWD/docker-compose.yml" \
COMPOSE_EXTRA_FILE="$PWD/deploy/docker-compose.app.yml" \
COMPOSE_ENV_FILE="$PWD/deploy/app.env" \
ALLOW_PLAINTEXT_BACKUP=1 REQUIRE_OFFSITE_BACKUP=0 \
  BACKUP_ROOT="$PWD/.run/backups/full" \
  ./scripts/backup/backup_all.sh
```

Never use that plaintext escape hatch for staging or production.

## Interim home PC off-host pull

Until a real S3/R2/B2/SFTP rclone remote exists, the operator PC may pull
encrypted snapshots over SSH. This is a **home off-host copy**, not
provider-backed off-site. Do not enable `drivergo-backup.service` (cloud
fail-closed) until rclone restore evidence exists; use the interim VPS unit
instead.

### VPS (create encrypted snapshots locally)

```bash
# Requires /etc/drivergo/backup.env with AGE_RECIPIENT=... (mode 0600).
# Do not set REQUIRE_OFFSITE_BACKUP=1 yet.
install -d -m 0700 /var/backups/drivergo/full /var/backups/drivergo/quarantine
install -m 0644 deploy/systemd/drivergo-backup-homepc.service /etc/systemd/system/
install -m 0644 deploy/systemd/drivergo-backup-homepc.timer /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now drivergo-backup-homepc.timer
systemctl start drivergo-backup-homepc.service
journalctl -u drivergo-backup-homepc.service --since today
```

### Operator PC (CachyOS) — pull when online

```bash
install -d -m 0700 "$HOME/.config/drivergo" "$HOME/drivergo-offhost/full"
sed "s|/home/REPLACE_ME|$HOME|g" scripts/backup/offhost-pull.env.example \
  >"$HOME/.config/drivergo/offhost-pull.env"
chmod 600 "$HOME/.config/drivergo/offhost-pull.env"
chmod 0700 "$HOME/drivergo-offhost" "$HOME/drivergo-offhost/full"
# Keep quotes around paths that contain spaces (e.g. ~/Рабочий стол/...).
# Edit OFFHOST_SSH_HOST if needed. BatchMode SSH key access to the VPS is required.

# One-shot pull (fails closed when the PC/network is down; retry later):
OFFHOST_ENV_FILE="$HOME/.config/drivergo/offhost-pull.env" \
  ./scripts/backup/pull_offhost.sh

# User timer (every ~30 minutes, Persistent=true):
mkdir -p "$HOME/.config/systemd/user"
ln -sf "$PWD/scripts/backup/systemd/drivergo-offhost-pull.service" \
  "$HOME/.config/systemd/user/"
ln -sf "$PWD/scripts/backup/systemd/drivergo-offhost-pull.timer" \
  "$HOME/.config/systemd/user/"
systemctl --user daemon-reload
systemctl --user enable --now drivergo-offhost-pull.timer
loginctl enable-linger "$USER"   # optional: pull even when no GUI session
```

`pull_offhost.sh` only copies complete `drivergo-*` directories (manifest +
checksums), resumes under `.pulling/<snapshot-id>/`, verifies with
`verify_snapshot.sh`, then atomically renames into the local root. Incomplete
remote directories are skipped. Keep the age private identity on this PC (or
other escrow), never on the VPS.

## Verification and restore drill

Verification reads files only:

```bash
./scripts/backup/verify_snapshot.sh \
  /var/backups/drivergo/full/drivergo-YYYYMMDDTHHMMSSZ
```

The full drill requires an explicit acknowledgement. Its only database target
is `avtotest_restore_drill`; arbitrary names, the application DB, `postgres`,
and template DBs are rejected before any Docker/PostgreSQL command. It removes
the scratch DB on successful completion. Decrypted data exists only in the
guarded scratch directory and is deleted by the exit trap.

```bash
install -d -m 0700 /var/lib/drivergo-dr-evidence
RESTORE_DRILL_ACK=avtotest_restore_drill \
AGE_IDENTITY_FILE=/secure/off-host/drivergo-age-identity.txt \
DRILL_REPORT_FILE=/var/lib/drivergo-dr-evidence/$(date -u +%Y%m%dT%H%M%SZ).txt \
  ./scripts/backup/full_restore_drill.sh \
  /var/backups/drivergo/full/drivergo-YYYYMMDDTHHMMSSZ
```

Component drills are also available:

```bash
RESTORE_DRILL_ACK=avtotest_restore_drill \
  ./scripts/backup/pg_restore_drill.sh postgres.dump
./scripts/backup/redis_restore_drill.sh redis.rdb
./scripts/backup/minio_restore_drill.sh minio-data.tar
./scripts/backup/humo_restore_drill.sh humo-queue.sqlite3
```

The default remains offline: neither Redis nor MinIO is started unless runtime
restore is explicitly enabled. For an isolated runtime drill, first make the
exact committed image digests available locally. The drill itself uses
`--pull never`; a missing image, floating tag, or non-allowlisted repository
fails before a runtime container is created.
`REDIS_RUNTIME_RESTORE_DRILL` and `MINIO_RUNTIME_RESTORE_DRILL` can override
the common flag independently when a component-only exercise is required.

```bash
REDIS_DRILL_IMAGE='redis:7-alpine@sha256:6ab0b6e7381779332f97b8ca76193e45b0756f38d4c0dcda72dbb3c32061ab99'
MINIO_DRILL_IMAGE='minio/minio@sha256:14cea493d9a34af32f524e538b8346cf79f3321eff8e708c1e2960462bd8936e'
MINIO_MC_DRILL_IMAGE='minio/mc@sha256:a7fe349ef4bd8521fb8497f55c6042871b2ae640607cf99d9bede5e9bdf11727'
docker pull "$REDIS_DRILL_IMAGE"
docker pull "$MINIO_DRILL_IMAGE"
docker pull "$MINIO_MC_DRILL_IMAGE"
docker image inspect "$REDIS_DRILL_IMAGE" "$MINIO_DRILL_IMAGE" "$MINIO_MC_DRILL_IMAGE" >/dev/null

install -d -m 0700 /tmp/drivergo-restore-drill /var/lib/drivergo-dr-evidence
# On an isolated recovery host, start only a separately named PostgreSQL
# recovery project. /secure/recovery/drivergo.env must contain recovery-only
# values satisfying the production Compose contract.
COMPOSE_PROJECT_NAME=drivergo-restore-drill \
  docker compose -f deploy/docker-compose.prod.yml \
  --env-file /secure/recovery/drivergo.env up -d postgres

RUNTIME_RESTORE_DRILL=1 \
REDIS_DRILL_IMAGE="$REDIS_DRILL_IMAGE" \
MINIO_DRILL_IMAGE="$MINIO_DRILL_IMAGE" \
MINIO_MC_DRILL_IMAGE="$MINIO_MC_DRILL_IMAGE" \
RUNTIME_DRILL_TIMEOUT_SECONDS=60 \
RESTORE_DRILL_ACK=avtotest_restore_drill \
AGE_IDENTITY_FILE=/secure/off-host/drivergo-age-identity.txt \
DRILL_ROOT=/tmp/drivergo-restore-drill \
DRILL_REPORT_FILE=/var/lib/drivergo-dr-evidence/$(date -u +%Y%m%dT%H%M%SZ).txt \
COMPOSE_PROJECT_NAME=drivergo-restore-drill \
COMPOSE_FILE="$PWD/deploy/docker-compose.prod.yml" \
COMPOSE_ENV_FILE=/secure/recovery/drivergo.env \
  ./scripts/backup/full_restore_drill.sh \
  /var/backups/drivergo/full/drivergo-YYYYMMDDTHHMMSSZ
```

The opt-in Redis container uses the copied RDB from a guarded, read-only bind,
Docker `network none`, no host port, and no named/anonymous volume. The MinIO
server and one-shot `mc` client use a uniquely named internal Docker network,
no host port, and only the extracted scratch directory. Runtime object names,
IDs, and restore-drill labels are all checked before cleanup; a changed or
unverifiable identity is never passed to `docker rm`. Each component input is
hash-checked before and after its drill. The verified snapshot itself is never
mounted writable or accepted as an evidence-report target. Exit traps remove
verified runtime objects and scratch data on both success and failure.

The full drill also performs its existing PostgreSQL restore through the
configured Compose project, always into the hard-coded
`avtotest_restore_drill` database and then drops it. That is why the rollout
above uses a separately named project on an isolated recovery host. The
Redis/MinIO runtime path never addresses a Compose service, production volume,
or host port.

Humo is already exercised as a real restored SQLite copy: full integrity and a
required-table query run through SQLite in immutable read-only mode. The drill
intentionally does not launch `humo-watcher`, because doing so could dequeue or
send payment events and would no longer be non-destructive.

`full_restore_drill.sh` produces machine-readable duration evidence. Store the
report outside the immutable snapshot and retain it with the incident/DR
records. Runtime reports include the exact image digests, Redis
`PING`/`DBSIZE`, MinIO readiness and inventory counts, Humo
integrity/pending-row evidence, source immutability, verified cleanup status,
and per-component durations. A repository mock test is not runtime evidence:
do not record the runtime path as operational until the command above succeeds
on an isolated recovery host. The drill validates recoverability of data
artifacts; it does not measure DNS change, host provisioning, secret recovery,
image pulls, or full application smoke testing.

## Recovery sequence for an incident

1. Declare the incident, stop application writers, record the last known-good
   time, and preserve affected disks/logs.
2. Retrieve one immutable snapshot from off-site storage into an isolated host.
3. Run `verify_snapshot.sh`; do not decrypt or restore a failed snapshot.
4. Recover the exact application secret set from escrow, especially
   `DATA_ENCRYPTION_KEY`/legacy `JWT_SECRET`, MinIO credentials, and the age
   identity. Never rotate data-encryption keys during recovery.
5. Run `full_restore_drill.sh` in isolation and retain its evidence report.
6. Restore components into newly provisioned, empty production targets using a
   separately reviewed change plan. The committed drill intentionally has no
   live-target restore mode.
7. Start readers before writers, run `/healthz`, `/readyz`, API/media/Humo smoke
   checks, reconcile the component time skew, then approve traffic cutover.
8. Record actual data-loss window and time-to-service; rotate credentials only
   after recovery is confirmed.

## RPO/RTO evidence and limits

The committed timer fires daily at 02:15 UTC with up to 15 minutes randomized
delay. On a continuously running host with an active timer and successful jobs,
the technical scheduled backup interval is therefore at most **24h15m**. Host
downtime, a failed job, insufficient disk, or a broken off-site provider makes
the real recovery point older. There is no WAL archiving/PITR, so this is not
evidence for a one-hour database RPO.

No measured production RTO is committed. A successful drill report measures
artifact validation/restoration only; the full service RTO must also include
off-site download, clean-host provisioning, secret recovery, deployment,
reconciliation, smoke tests, and traffic cutover. Product/operations owners
must approve the business RPO/RTO after at least one timed isolated-host drill.
Repeat a full drill quarterly and after schema, storage, encryption, or backup
tooling changes.

Open controls before a high-assurance claim:

- PostgreSQL WAL archiving/PITR for sub-daily recovery points;
- MinIO versioning or independent object-level replication;
- an externally monitored backup-age/failure alert and capacity alert;
- off-site immutability/object lock enforced by the storage provider;
- a tested secret-escrow recovery path and a second operator;
- a complete isolated-host restore plus application-level smoke test.

## Repository-only checks

`./scripts/backup/test_backup_scripts.sh` performs Bash syntax, fixture
integrity/tamper, path/image/name guards, local retention, offline component
checks, runtime evidence contracts, and success/failure cleanup tests. It uses
a temporary directory only; command fixtures ensure it never contacts a Docker
daemon, age key, rclone remote, or live database.

`pg_dump.sh` remains a local PostgreSQL-only helper for compatibility. It is
not the production schedule and does not replace `backup_all.sh`.
