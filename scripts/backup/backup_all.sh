#!/usr/bin/env bash
# Full Driver Go backup: PostgreSQL + Redis + MinIO + Humo SQLite.
# Payloads are streamed into an atomic snapshot directory and are encrypted
# with age unless plaintext mode is explicitly opted into for local development.
set -euo pipefail
umask 077

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "$SCRIPT_DIR/lib.sh"

ROOT="$(repo_root)"
cd "$ROOT"

BACKUP_ROOT="${BACKUP_ROOT:-$ROOT/.run/backups/full}"
BACKUP_LOCK_FILE="${BACKUP_LOCK_FILE:-$BACKUP_ROOT/.drivergo-backup.lock}"
POSTGRES_SERVICE="${POSTGRES_SERVICE:-postgres}"
REDIS_SERVICE="${REDIS_SERVICE:-redis}"
MINIO_SERVICE="${MINIO_SERVICE:-minio}"
HUMO_SERVICE="${HUMO_SERVICE:-humo-watcher}"
PGUSER="${PGUSER:-avtotest}"
PGDATABASE="${PGDATABASE:-avtotest}"
MINIO_DATA_PATH="${MINIO_DATA_PATH:-/data}"
AGE_RECIPIENT="${AGE_RECIPIENT:-}"
ALLOW_PLAINTEXT_BACKUP="${ALLOW_PLAINTEXT_BACKUP:-0}"
RCLONE_REMOTE="${RCLONE_REMOTE:-}"
REQUIRE_OFFSITE_BACKUP="${REQUIRE_OFFSITE_BACKUP:-0}"
BACKUP_RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-14}"
OFFSITE_RETENTION_DAYS="${OFFSITE_RETENTION_DAYS:-35}"
BACKUP_MIN_SNAPSHOTS="${BACKUP_MIN_SNAPSHOTS:-3}"
PRUNE_OFFSITE="${PRUNE_OFFSITE:-0}"

case "${1:-}" in
  --production)
    # Command-line production mode cannot be weakened by EnvironmentFile
    # values: encryption, verified off-site copy, and remote retention stay on.
    ALLOW_PLAINTEXT_BACKUP=0
    REQUIRE_OFFSITE_BACKUP=1
    PRUNE_OFFSITE=1
    shift
    ;;
  "") ;;
  *) backup_die "usage: backup_all.sh [--production]" ;;
esac
[[ "$#" -eq 0 ]] || backup_die "usage: backup_all.sh [--production]"

require_command docker
require_command flock
require_command sha256sum
require_command realpath
require_command date
require_bool ALLOW_PLAINTEXT_BACKUP "$ALLOW_PLAINTEXT_BACKUP"
require_bool REQUIRE_OFFSITE_BACKUP "$REQUIRE_OFFSITE_BACKUP"
require_uint BACKUP_RETENTION_DAYS "$BACKUP_RETENTION_DAYS"
require_uint OFFSITE_RETENTION_DAYS "$OFFSITE_RETENTION_DAYS"
require_positive_uint BACKUP_MIN_SNAPSHOTS "$BACKUP_MIN_SNAPSHOTS"
require_bool PRUNE_OFFSITE "$PRUNE_OFFSITE"
validate_backup_root "$BACKUP_ROOT"
BACKUP_ROOT="$(realpath -e "$BACKUP_ROOT")"
for service in "$POSTGRES_SERVICE" "$REDIS_SERVICE" "$MINIO_SERVICE" "$HUMO_SERVICE"; do
  validate_service_name "$service"
done
[[ "$PGUSER" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || backup_die "unsafe PGUSER: ${PGUSER}"
[[ "$PGDATABASE" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || backup_die "unsafe PGDATABASE: ${PGDATABASE}"
[[ "$MINIO_DATA_PATH" =~ ^/[A-Za-z0-9._/-]+$ && "$MINIO_DATA_PATH" != "/" ]] || \
  backup_die "MINIO_DATA_PATH must be a non-root absolute container path"
case "/$MINIO_DATA_PATH/" in
  */../*|*/./*) backup_die "MINIO_DATA_PATH contains an unsafe path segment" ;;
esac

if [[ -z "$AGE_RECIPIENT" ]]; then
  [[ "$ALLOW_PLAINTEXT_BACKUP" == "1" ]] || \
    backup_die "AGE_RECIPIENT is required (local-only escape hatch: ALLOW_PLAINTEXT_BACKUP=1)"
  ENCRYPTION="none"
  PAYLOAD_SUFFIX=""
  echo "WARNING: creating an explicitly requested plaintext local backup" >&2
else
  require_command age
  ENCRYPTION="age"
  PAYLOAD_SUFFIX=".age"
fi

if [[ -z "$RCLONE_REMOTE" ]]; then
  [[ "$REQUIRE_OFFSITE_BACKUP" == "0" ]] || \
    backup_die "RCLONE_REMOTE is required when REQUIRE_OFFSITE_BACKUP=1"
else
  require_command rclone
  validate_remote_base "$RCLONE_REMOTE"
fi

validate_lock_file "$BACKUP_LOCK_FILE"
exec 9>"$BACKUP_LOCK_FILE"
flock -n 9 || backup_die "another backup or retention run holds ${BACKUP_LOCK_FILE}"

BACKUP_STARTED_EPOCH="$(date -u +%s)"
STAMP="$(date -u -d "@${BACKUP_STARTED_EPOCH}" +%Y%m%dT%H%M%SZ)"
BACKUP_STARTED_AT="$(date -u -d "@${BACKUP_STARTED_EPOCH}" +%Y-%m-%dT%H:%M:%SZ)"
SNAPSHOT_ID="drivergo-${STAMP}"
FINAL_DIR="$BACKUP_ROOT/$SNAPSHOT_ID"
STAGING_DIR="$BACKUP_ROOT/.partial-${SNAPSHOT_ID}"
is_snapshot_name "$SNAPSHOT_ID" || backup_die "generated unsafe snapshot id: ${SNAPSHOT_ID}"
[[ ! -e "$FINAL_DIR" && ! -e "$STAGING_DIR" ]] || backup_die "snapshot already exists: ${SNAPSHOT_ID}"
mkdir -m 0700 "$STAGING_DIR"

cleanup_partial() {
  if [[ -d "$STAGING_DIR" ]]; then
    case "$STAGING_DIR" in
      "$BACKUP_ROOT"/.partial-drivergo-*) rm -rf -- "$STAGING_DIR" ;;
      *) echo "refusing unsafe partial cleanup: ${STAGING_DIR}" >&2 ;;
    esac
  fi
}
trap cleanup_partial EXIT INT TERM

postgres_stream() {
  compose_cmd exec -T "$POSTGRES_SERVICE" \
    pg_dump -U "$PGUSER" -d "$PGDATABASE" \
      --format=custom --no-owner --no-acl
}

redis_stream() {
  # redis-cli --rdb obtains an RDB through the replication protocol without
  # replacing the live server's configured persistence files.
  compose_cmd exec -T "$REDIS_SERVICE" sh -euc '
    # BusyBox mktemp (used by the Alpine Redis image) requires the X run at
    # the end of the template; a suffix makes it fail with "Invalid argument".
    tmp="$(mktemp /tmp/drivergo-redis.XXXXXX)"
    trap '\''rm -f -- "$tmp"'\'' EXIT
    redis-cli --rdb "$tmp" >/dev/null
    test -s "$tmp"
    redis-check-rdb "$tmp" >/dev/null
    cat "$tmp"
  '
}

minio_stream() {
  local container_id
  container_id="$(compose_container_id "$MINIO_SERVICE")"
  # docker cp with local destination '-' emits a tar stream. This captures the
  # complete MinIO data volume without staging plaintext on the host.
  docker cp "${container_id}:${MINIO_DATA_PATH%/}/." -
}

humo_stream() {
  # SQLite's online backup API produces a transactionally consistent database
  # even while the watcher keeps appending to its WAL-backed queue.
  compose_cmd exec -T "$HUMO_SERVICE" python -c '
import os
import shutil
import sqlite3
import sys
import tempfile

source = os.environ.get("HUMO_QUEUE_DB", "/data/humo-queue.sqlite3")
fd, target = tempfile.mkstemp(prefix="drivergo-humo-", suffix=".sqlite3")
os.close(fd)
try:
    src = sqlite3.connect(f"file:{source}?mode=ro", uri=True, timeout=30)
    dst = sqlite3.connect(target, timeout=30)
    try:
        src.backup(dst)
    finally:
        dst.close()
        src.close()
    with open(target, "rb") as handle:
        shutil.copyfileobj(handle, sys.stdout.buffer)
finally:
    try:
        os.unlink(target)
    except FileNotFoundError:
        pass
'
}

capture_component() {
  local logical_name="$1"
  local base_name="$2"
  shift 2
  local final_name="${base_name}${PAYLOAD_SUFFIX}"
  local partial="$STAGING_DIR/.${final_name}.part"
  local output="$STAGING_DIR/$final_name"

  echo "==> ${logical_name}" >&2
  if [[ "$ENCRYPTION" == "age" ]]; then
    # age emits a non-empty envelope even for empty input. Check the whole
    # pipe explicitly so an upstream capture failure can never masquerade as
    # a valid encrypted artifact (including inside command substitution).
    if ! "$@" | age --recipient "$AGE_RECIPIENT" --output "$partial"; then
      rm -f -- "$partial"
      backup_die "${logical_name} capture or encryption failed"
    fi
  else
    if ! "$@" >"$partial"; then
      rm -f -- "$partial"
      backup_die "${logical_name} capture failed"
    fi
  fi
  [[ -s "$partial" ]] || backup_die "${logical_name} produced an empty artifact"
  mv -- "$partial" "$output"
  printf '%s\n' "$final_name"
}

POSTGRES_FILE="$(capture_component postgres postgres.dump postgres_stream)"
REDIS_FILE="$(capture_component redis redis.rdb redis_stream)"
MINIO_FILE="$(capture_component minio minio-data.tar minio_stream)"
HUMO_FILE="$(capture_component humo_sqlite humo-queue.sqlite3 humo_stream)"

GIT_COMMIT="$(git rev-parse --verify HEAD 2>/dev/null || printf 'unknown')"
SOURCE_HOST="$(hostname | tr -cd 'A-Za-z0-9._-')"
CAPTURE_FINISHED_EPOCH="$(date -u +%s)"
CREATED_AT="$(date -u -d "@${CAPTURE_FINISHED_EPOCH}" +%Y-%m-%dT%H:%M:%SZ)"
CAPTURE_DURATION_SECONDS="$((CAPTURE_FINISHED_EPOCH - BACKUP_STARTED_EPOCH))"
cat >"$STAGING_DIR/manifest.txt" <<EOF
format=drivergo-full-backup-v1
snapshot=$SNAPSHOT_ID
created_at=$CREATED_AT
backup_started_at=$BACKUP_STARTED_AT
capture_duration_seconds=$CAPTURE_DURATION_SECONDS
source_host=${SOURCE_HOST:-unknown}
git_commit=$GIT_COMMIT
encryption=$ENCRYPTION
component_file_postgres=$POSTGRES_FILE
component_file_redis=$REDIS_FILE
component_file_minio=$MINIO_FILE
component_file_humo=$HUMO_FILE
postgres_database=$PGDATABASE
minio_consistency=live-volume-tar-stream
humo_consistency=sqlite-online-backup
technical_backup_interval=24h15m-max-with-committed-timer
EOF

for component in "$POSTGRES_FILE" "$REDIS_FILE" "$MINIO_FILE" "$HUMO_FILE"; do
  bytes="$(wc -c <"$STAGING_DIR/$component" | tr -d ' ')"
  printf 'size_%s=%s\n' "${component//[^A-Za-z0-9]/_}" "$bytes" >>"$STAGING_DIR/manifest.txt"
done

(
  cd "$STAGING_DIR"
  sha256sum -- manifest.txt "$POSTGRES_FILE" "$REDIS_FILE" "$MINIO_FILE" "$HUMO_FILE" >SHA256SUMS
  sha256sum --check --strict SHA256SUMS
)

# Directory rename on one filesystem is atomic: operators never see a complete
# snapshot name containing only a subset of components.
mv -- "$STAGING_DIR" "$FINAL_DIR"
trap - EXIT INT TERM

if [[ -n "$RCLONE_REMOTE" ]]; then
  REMOTE_SNAPSHOT="${RCLONE_REMOTE%/}/$SNAPSHOT_ID"
  echo "==> off-site upload: ${REMOTE_SNAPSHOT}" >&2
  rclone copy "$FINAL_DIR" "$REMOTE_SNAPSHOT" --checksum --immutable
  rclone check "$FINAL_DIR" "$REMOTE_SNAPSHOT" --one-way --checksum
  checksums_digest="$(sha256sum -- "$FINAL_DIR/SHA256SUMS" | awk '{ print $1 }')"
  printf 'snapshot=%s\nsha256sums_sha256=%s\n' "$SNAPSHOT_ID" "$checksums_digest" | \
    rclone rcat "${REMOTE_SNAPSHOT}/REMOTE_COMPLETE" --immutable
  echo "off-site verification: ok" >&2
fi

BACKUP_LOCK_HELD=1 \
BACKUP_ROOT="$BACKUP_ROOT" \
BACKUP_RETENTION_DAYS="$BACKUP_RETENTION_DAYS" \
OFFSITE_RETENTION_DAYS="$OFFSITE_RETENTION_DAYS" \
BACKUP_MIN_SNAPSHOTS="$BACKUP_MIN_SNAPSHOTS" \
RCLONE_REMOTE="$RCLONE_REMOTE" \
PRUNE_OFFSITE="$PRUNE_OFFSITE" \
  "$SCRIPT_DIR/prune_backups.sh"

echo "backup: complete -> ${FINAL_DIR}"
