#!/usr/bin/env bash
# Isolated PostgreSQL point-in-time recovery drill. This never addresses a
# Compose service, production volume, host port, or configurable database name.
set -euo pipefail
umask 077

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "$SCRIPT_DIR/lib.sh"

BASE_BACKUP="${1:-}"
PITR_TARGET_TIME="${2:-}"
PITR_RESTORE_DRILL_ACK="${PITR_RESTORE_DRILL_ACK:-}"
PITR_WAL_ARCHIVE_ROOT="${PITR_WAL_ARCHIVE_ROOT:-}"
PITR_DRILL_ROOT="${PITR_DRILL_ROOT:-/tmp/drivergo-pitr-restore-drill}"
PITR_POSTGRES_IMAGE="${PITR_POSTGRES_IMAGE:-postgres:16-alpine@sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777}"
PITR_DRILL_TIMEOUT_SECONDS="${PITR_DRILL_TIMEOUT_SECONDS:-120}"
AGE_IDENTITY_FILE="${AGE_IDENTITY_FILE:-}"
SCRATCH=""
CONTAINER_ID=""
CONTAINER_NAME=""
START_EPOCH=""

[[ "$PITR_RESTORE_DRILL_ACK" == "drivergo_pitr_restore_drill" ]] || \
  backup_die "set PITR_RESTORE_DRILL_ACK=drivergo_pitr_restore_drill to acknowledge an isolated scratch-only PITR drill"
[[ "$PITR_TARGET_TIME" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]] || \
  backup_die "PITR target time must be UTC RFC3339 seconds precision"
require_positive_uint PITR_DRILL_TIMEOUT_SECONDS "$PITR_DRILL_TIMEOUT_SECONDS"
[[ "$PITR_POSTGRES_IMAGE" == "postgres:16-alpine@sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777" ]] || \
  backup_die "PITR_POSTGRES_IMAGE must be the committed immutable PostgreSQL recovery image"
[[ -n "$BASE_BACKUP" && -d "$BASE_BACKUP" && ! -L "$BASE_BACKUP" ]] || \
  backup_die "base backup must be an existing non-symlink directory"
[[ "$(basename "${BASE_BACKUP%/}")" == "drivergo-pitr-basebackup" ]] || \
  backup_die "base backup directory basename must be drivergo-pitr-basebackup"
[[ -n "$PITR_WAL_ARCHIVE_ROOT" ]] || backup_die "PITR_WAL_ARCHIVE_ROOT is required"
validate_pitr_wal_archive_root "$PITR_WAL_ARCHIVE_ROOT"
require_command docker
require_command mktemp
require_command timeout
require_command cp
require_command find
require_command date

[[ "$PITR_DRILL_ROOT" = /* && "${PITR_DRILL_ROOT%/}" != "/" && \
  "$(basename "${PITR_DRILL_ROOT%/}")" == "drivergo-pitr-restore-drill" ]] || \
  backup_die "PITR_DRILL_ROOT must be an absolute path ending in drivergo-pitr-restore-drill"
if [[ -e "$PITR_DRILL_ROOT" ]]; then
  [[ -d "$PITR_DRILL_ROOT" && ! -L "$PITR_DRILL_ROOT" ]] || \
    backup_die "PITR_DRILL_ROOT must be a non-symlink directory"
else
  mkdir -p -m 0700 "$PITR_DRILL_ROOT"
fi
[[ "$(stat -c '%u' "$PITR_DRILL_ROOT")" == "$EUID" ]] || \
  backup_die "PITR_DRILL_ROOT must be owned by uid ${EUID}"
(( 10#$(stat -c '%a' "$PITR_DRILL_ROOT") % 100 == 0 )) || \
  backup_die "PITR_DRILL_ROOT must not be accessible by group/other"

START_EPOCH="$(date -u +%s)"
"$SCRIPT_DIR/pitr_verify_wal.sh" "$PITR_WAL_ARCHIVE_ROOT" >/dev/null
BASE_BACKUP="$(realpath -e "$BASE_BACKUP")"
PITR_WAL_ARCHIVE_ROOT="$(realpath -e "$PITR_WAL_ARCHIVE_ROOT")"
if find "$BASE_BACKUP" -type l -print -quit | grep -q .; then
  backup_die "base backup must not contain symlinks/tablespaces for this drill"
fi

cleanup() {
  local original_status="$?"
  local cleanup_status=0
  trap - EXIT
  if [[ -n "$CONTAINER_ID" ]]; then
    safe_remove_runtime_container pitr "$CONTAINER_NAME" "$CONTAINER_ID" || cleanup_status="$?"
  fi
  if [[ -n "$SCRATCH" ]]; then
    safe_remove_scratch "$PITR_DRILL_ROOT" "$SCRATCH" || cleanup_status="$?"
  fi
  if (( original_status != 0 )); then
    exit "$original_status"
  fi
  (( cleanup_status == 0 )) || exit "$cleanup_status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

SCRATCH="$(mktemp -d "$PITR_DRILL_ROOT/drivergo-drill.XXXXXX")"
validate_scratch_child "$PITR_DRILL_ROOT" "$SCRATCH"
DATA_DIR="$SCRATCH/data"
WAL_DIR="$SCRATCH/wal"
mkdir -m 0700 "$DATA_DIR" "$WAL_DIR"
cp -a -- "$BASE_BACKUP/." "$DATA_DIR/"

mapfile -t markers < <(find "$PITR_WAL_ARCHIVE_ROOT" -mindepth 1 -maxdepth 1 -type f -name '*.complete' -printf '%f\n' | sort)
for marker in "${markers[@]}"; do
  wal_name="${marker%.complete}"
  marker_path="$PITR_WAL_ARCHIVE_ROOT/$marker"
  payload="$(manifest_value "$marker_path" payload)"
  encryption="$(manifest_value "$marker_path" encryption)"
  if [[ "$encryption" == "age" ]]; then
    require_command age
    [[ -n "$AGE_IDENTITY_FILE" && -f "$AGE_IDENTITY_FILE" && ! -L "$AGE_IDENTITY_FILE" ]] || \
      backup_die "AGE_IDENTITY_FILE must be a regular non-symlink file for encrypted WAL"
    age --decrypt --identity "$AGE_IDENTITY_FILE" --output "$WAL_DIR/$wal_name" \
      "$PITR_WAL_ARCHIVE_ROOT/$payload"
  else
    cp -- "$PITR_WAL_ARCHIVE_ROOT/$payload" "$WAL_DIR/$wal_name"
  fi
  [[ -s "$WAL_DIR/$wal_name" ]] || backup_die "staged WAL is empty: ${wal_name}"
done

touch "$DATA_DIR/recovery.signal"
cat >>"$DATA_DIR/postgresql.auto.conf" <<EOF
restore_command = 'cp /pitr-wal/%f %p'
recovery_target_time = '$PITR_TARGET_TIME'
recovery_target_action = 'promote'
EOF

token="$(runtime_drill_token "$SCRATCH")"
CONTAINER_NAME="drivergo-postgres-pitr-drill-$token"
validate_runtime_drill_container_name pitr "$CONTAINER_NAME"
CONTAINER_ID="$(docker create \
  --name "$CONTAINER_NAME" \
  --label com.drivergo.restore-drill=pitr \
  --network none \
  --pull never \
  --mount "type=bind,src=$DATA_DIR,dst=/var/lib/postgresql/data" \
  --mount "type=bind,src=$WAL_DIR,dst=/pitr-wal,readonly" \
  --tmpfs /tmp:rw,noexec,nosuid,size=64m \
  "$PITR_POSTGRES_IMAGE")"
validate_runtime_object_id container "$CONTAINER_ID"

docker start "$CONTAINER_ID" >/dev/null
ready=0
for _ in $(seq 1 "$PITR_DRILL_TIMEOUT_SECONDS"); do
  if docker exec "$CONTAINER_ID" pg_isready -U avtotest -d postgres >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 1
done
[[ "$ready" == "1" ]] || backup_die "PITR scratch PostgreSQL did not become ready before timeout"

read -r TABLES CORE_TABLES < <(
  docker exec "$CONTAINER_ID" psql -U avtotest -d avtotest -At -F ' ' -v ON_ERROR_STOP=1 <<'SQL'
SELECT
  (SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public'),
  (SELECT count(*) FROM (VALUES
     (to_regclass('public.schema_migrations')),
     (to_regclass('public.profile')),
     (to_regclass('public.question'))
   ) AS required(rel) WHERE rel IS NOT NULL);
SQL
)
[[ "$TABLES" =~ ^[0-9]+$ && "$TABLES" -gt 0 && "$CORE_TABLES" == "3" ]] || \
  backup_die "PITR scratch validation failed"

safe_remove_runtime_container pitr "$CONTAINER_NAME" "$CONTAINER_ID"
CONTAINER_ID=""
safe_remove_scratch "$PITR_DRILL_ROOT" "$SCRATCH"
SCRATCH=""
trap - EXIT INT TERM
FINISHED_EPOCH="$(date -u +%s)"
echo "PITR drill: ok (target=${PITR_TARGET_TIME}, tables=${TABLES}, core_tables=${CORE_TABLES}, duration_seconds=$((FINISHED_EPOCH - START_EPOCH)))"
