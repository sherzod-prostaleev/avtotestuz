#!/usr/bin/env bash
# Full, non-production restore drill for a Driver Go snapshot.
#
# The only stateful restore is PostgreSQL, and pg_restore_drill.sh enforces the
# hard-coded avtotest_restore_drill allowlist and removes that database on exit.
# Redis and MinIO remain offline-only by default. Explicit runtime opt-in boots
# only digest-pinned ephemeral component containers on scratch data; Humo is
# always opened read-only/immutable by the real SQLite engine.
set -euo pipefail
umask 077

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "$SCRIPT_DIR/lib.sh"

SNAPSHOT_DIR="${1:-}"
DRILL_ROOT="${DRILL_ROOT:-/tmp/drivergo-restore-drill}"
RESTORE_DRILL_ACK="${RESTORE_DRILL_ACK:-}"
AGE_IDENTITY_FILE="${AGE_IDENTITY_FILE:-}"
DRILL_REPORT_FILE="${DRILL_REPORT_FILE:-}"
KEEP_DRILL_DB="${KEEP_DRILL_DB:-0}"
RUNTIME_RESTORE_DRILL="${RUNTIME_RESTORE_DRILL:-0}"
REDIS_RUNTIME_RESTORE_DRILL="${REDIS_RUNTIME_RESTORE_DRILL:-$RUNTIME_RESTORE_DRILL}"
MINIO_RUNTIME_RESTORE_DRILL="${MINIO_RUNTIME_RESTORE_DRILL:-$RUNTIME_RESTORE_DRILL}"
REDIS_DRILL_IMAGE="${REDIS_DRILL_IMAGE:-}"
MINIO_DRILL_IMAGE="${MINIO_DRILL_IMAGE:-}"
MINIO_MC_DRILL_IMAGE="${MINIO_MC_DRILL_IMAGE:-}"
RUNTIME_DRILL_TIMEOUT_SECONDS="${RUNTIME_DRILL_TIMEOUT_SECONDS:-30}"
REPORT_TMP=""
SCRATCH=""
SCRATCH_CLEANED=0

[[ -n "$SNAPSHOT_DIR" ]] || backup_die "usage: full_restore_drill.sh SNAPSHOT_DIR"
[[ "$RESTORE_DRILL_ACK" == "avtotest_restore_drill" ]] || \
  backup_die "set RESTORE_DRILL_ACK=avtotest_restore_drill to acknowledge scratch-only restore"
[[ "$KEEP_DRILL_DB" == "0" ]] || \
  backup_die "full drill requires KEEP_DRILL_DB=0 so the scratch database is removed"
require_bool RUNTIME_RESTORE_DRILL "$RUNTIME_RESTORE_DRILL"
require_bool REDIS_RUNTIME_RESTORE_DRILL "$REDIS_RUNTIME_RESTORE_DRILL"
require_bool MINIO_RUNTIME_RESTORE_DRILL "$MINIO_RUNTIME_RESTORE_DRILL"
require_positive_uint RUNTIME_DRILL_TIMEOUT_SECONDS "$RUNTIME_DRILL_TIMEOUT_SECONDS"
if [[ "$REDIS_RUNTIME_RESTORE_DRILL" == "1" ]]; then
  validate_runtime_drill_image redis "$REDIS_DRILL_IMAGE"
fi
if [[ "$MINIO_RUNTIME_RESTORE_DRILL" == "1" ]]; then
  validate_runtime_drill_image minio "$MINIO_DRILL_IMAGE"
  validate_runtime_drill_image minio-mc "$MINIO_MC_DRILL_IMAGE"
fi
require_command realpath
require_command date
require_command tee
validate_drill_root "$DRILL_ROOT"
TOTAL_STARTED="$(date -u +%s)"
DRILL_STARTED_AT="$(date -u -d "@${TOTAL_STARTED}" +%Y-%m-%dT%H:%M:%SZ)"

"$SCRIPT_DIR/verify_snapshot.sh" "$SNAPSHOT_DIR"
SNAPSHOT_DIR="$(realpath -e "$SNAPSHOT_DIR")"
MANIFEST="$SNAPSHOT_DIR/manifest.txt"
SNAPSHOT_NAME="$(basename "$SNAPSHOT_DIR")"
ENCRYPTION="$(manifest_value "$MANIFEST" encryption)"

SCRATCH="$(mktemp -d "$DRILL_ROOT/drivergo-drill.XXXXXX")"
validate_scratch_child "$DRILL_ROOT" "$SCRATCH"
cleanup() {
  local original_status="$?"
  local cleanup_status=0
  trap - EXIT
  if [[ -n "$REPORT_TMP" && -f "$REPORT_TMP" && ! -L "$REPORT_TMP" ]]; then
    case "$(basename "$REPORT_TMP")" in
      .drivergo-drill-report.*) rm -f -- "$REPORT_TMP" || cleanup_status="$?" ;;
    esac
  fi
  if [[ "$SCRATCH_CLEANED" != "1" && -n "$SCRATCH" ]]; then
    safe_remove_scratch "$DRILL_ROOT" "$SCRATCH" || cleanup_status="$?"
  fi
  if (( original_status != 0 )); then
    exit "$original_status"
  fi
  if (( cleanup_status != 0 )); then
    echo "backup: restore-drill scratch cleanup failed" >&2
    exit "$cleanup_status"
  fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

if [[ "$ENCRYPTION" == "age" ]]; then
  require_command age
  [[ -n "$AGE_IDENTITY_FILE" && -f "$AGE_IDENTITY_FILE" && ! -L "$AGE_IDENTITY_FILE" ]] || \
    backup_die "AGE_IDENTITY_FILE must be a regular, non-symlink file for an encrypted snapshot"
fi

component_source() {
  local component="$1"
  local output_name="$2"
  local file source target
  file="$(manifest_value "$MANIFEST" "component_file_${component}")"
  source="$SNAPSHOT_DIR/$file"
  if [[ "$ENCRYPTION" == "age" ]]; then
    target="$SCRATCH/$output_name"
    age --decrypt --identity "$AGE_IDENTITY_FILE" --output "$target" "$source"
    [[ -s "$target" ]] || backup_die "decrypted ${component} artifact is empty"
    printf '%s\n' "$target"
  else
    printf '%s\n' "$source"
  fi
}

POSTGRES_DUMP="$(component_source postgres postgres.dump)"
REDIS_RDB="$(component_source redis redis.rdb)"
MINIO_TAR="$(component_source minio minio-data.tar)"
HUMO_SQLITE="$(component_source humo humo-queue.sqlite3)"

run_timed() {
  local label="$1"
  shift
  local started finished
  started="$(date -u +%s)"
  echo "==> ${label} restore drill"
  "$@"
  finished="$(date -u +%s)"
  LAST_DURATION="$((finished - started))"
}

run_timed_capture() {
  local label="$1"
  local capture_file="$2"
  shift 2
  local started finished
  [[ "$capture_file" == "$SCRATCH"/* && ! -e "$capture_file" && ! -L "$capture_file" ]] || \
    backup_die "unsafe component evidence capture path"
  started="$(date -u +%s)"
  echo "==> ${label} restore drill"
  "$@" | tee "$capture_file"
  finished="$(date -u +%s)"
  LAST_DURATION="$((finished - started))"
}

evidence_value() {
  local evidence_file="$1"
  local key="$2"
  local count value
  count="$(awk -F= -v wanted="$key" '$1 == wanted { count++ } END { print count + 0 }' "$evidence_file")"
  [[ "$count" == "1" ]] || backup_die "component evidence key ${key} must occur exactly once"
  value="$(awk -F= -v wanted="$key" '$1 == wanted { sub(/^[^=]*=/, ""); print; exit }' "$evidence_file")"
  [[ "$value" =~ ^[A-Za-z0-9._:@/-]+$ ]] || backup_die "component evidence key ${key} has an unsafe value"
  printf '%s\n' "$value"
}

# Humo always uses immutable/read-only SQLite. MinIO uses scratch extraction and
# optional ephemeral runtime. Redis uses either the existing container only as
# an offline redis-check-rdb tool or the isolated ephemeral runtime path.
HUMO_EVIDENCE="$SCRATCH/humo.evidence"
MINIO_EVIDENCE="$SCRATCH/minio.evidence"
REDIS_EVIDENCE="$SCRATCH/redis.evidence"
run_timed_capture humo "$HUMO_EVIDENCE" "$SCRIPT_DIR/humo_restore_drill.sh" "$HUMO_SQLITE"
HUMO_DURATION="$LAST_DURATION"
run_timed_capture minio "$MINIO_EVIDENCE" env \
  DRILL_ROOT="$DRILL_ROOT" \
  KEEP_DRILL_FILES=0 \
  MINIO_RUNTIME_RESTORE_DRILL="$MINIO_RUNTIME_RESTORE_DRILL" \
  MINIO_DRILL_IMAGE="$MINIO_DRILL_IMAGE" \
  MINIO_MC_DRILL_IMAGE="$MINIO_MC_DRILL_IMAGE" \
  RUNTIME_DRILL_TIMEOUT_SECONDS="$RUNTIME_DRILL_TIMEOUT_SECONDS" \
  "$SCRIPT_DIR/minio_restore_drill.sh" "$MINIO_TAR"
MINIO_DURATION="$LAST_DURATION"
run_timed_capture redis "$REDIS_EVIDENCE" env \
  DRILL_ROOT="$DRILL_ROOT" \
  REDIS_HEADER_ONLY_ALLOWED=0 \
  REDIS_RUNTIME_RESTORE_DRILL="$REDIS_RUNTIME_RESTORE_DRILL" \
  REDIS_DRILL_IMAGE="$REDIS_DRILL_IMAGE" \
  RUNTIME_DRILL_TIMEOUT_SECONDS="$RUNTIME_DRILL_TIMEOUT_SECONDS" \
  "$SCRIPT_DIR/redis_restore_drill.sh" "$REDIS_RDB"
REDIS_DURATION="$LAST_DURATION"

HUMO_MODE="$(evidence_value "$HUMO_EVIDENCE" humo_mode)"
HUMO_INTEGRITY_CHECK="$(evidence_value "$HUMO_EVIDENCE" humo_integrity_check)"
HUMO_SCHEMA_CHECK="$(evidence_value "$HUMO_EVIDENCE" humo_schema_check)"
HUMO_PENDING_COUNT="$(evidence_value "$HUMO_EVIDENCE" humo_pending_count)"
HUMO_SOURCE_UNCHANGED="$(evidence_value "$HUMO_EVIDENCE" humo_source_unchanged)"
require_uint humo_pending_count "$HUMO_PENDING_COUNT"
[[ "$HUMO_MODE" == "sqlite-read-only-integrity-check" && "$HUMO_INTEGRITY_CHECK" == "ok" && \
  "$HUMO_SCHEMA_CHECK" == "ok" && "$HUMO_SOURCE_UNCHANGED" == "1" ]] || \
  backup_die "Humo restore evidence contract mismatch"

MINIO_MODE="$(evidence_value "$MINIO_EVIDENCE" minio_mode)"
MINIO_EXTRACTED_FILES="$(evidence_value "$MINIO_EVIDENCE" minio_extracted_files)"
MINIO_RUNTIME_SERVER_IMAGE="$(evidence_value "$MINIO_EVIDENCE" minio_runtime_server_image)"
MINIO_RUNTIME_CLIENT_IMAGE="$(evidence_value "$MINIO_EVIDENCE" minio_runtime_client_image)"
MINIO_RUNTIME_READY="$(evidence_value "$MINIO_EVIDENCE" minio_runtime_ready)"
MINIO_RUNTIME_BUCKET_COUNT="$(evidence_value "$MINIO_EVIDENCE" minio_runtime_bucket_count)"
MINIO_RUNTIME_INVENTORY_ENTRIES="$(evidence_value "$MINIO_EVIDENCE" minio_runtime_inventory_entries)"
MINIO_RUNTIME_CLEANUP="$(evidence_value "$MINIO_EVIDENCE" minio_runtime_cleanup)"
MINIO_SOURCE_UNCHANGED="$(evidence_value "$MINIO_EVIDENCE" minio_source_unchanged)"
require_positive_uint minio_extracted_files "$MINIO_EXTRACTED_FILES"
[[ "$MINIO_SOURCE_UNCHANGED" == "1" ]] || backup_die "MinIO source immutability evidence mismatch"
if [[ "$MINIO_RUNTIME_RESTORE_DRILL" == "1" ]]; then
  validate_runtime_drill_image minio "$MINIO_RUNTIME_SERVER_IMAGE"
  validate_runtime_drill_image minio-mc "$MINIO_RUNTIME_CLIENT_IMAGE"
  require_positive_uint minio_runtime_bucket_count "$MINIO_RUNTIME_BUCKET_COUNT"
  require_uint minio_runtime_inventory_entries "$MINIO_RUNTIME_INVENTORY_ENTRIES"
  [[ "$MINIO_RUNTIME_SERVER_IMAGE" == "$MINIO_DRILL_IMAGE" && "$MINIO_RUNTIME_CLIENT_IMAGE" == "$MINIO_MC_DRILL_IMAGE" && \
    "$MINIO_MODE" == "ephemeral-runtime-restore" && "$MINIO_RUNTIME_READY" == "ok" && "$MINIO_RUNTIME_CLEANUP" == "completed" ]] || \
    backup_die "MinIO runtime restore evidence contract mismatch"
else
  [[ "$MINIO_MODE" == "guarded-scratch-extraction" && "$MINIO_RUNTIME_READY" == "not-run" && \
    "$MINIO_RUNTIME_SERVER_IMAGE" == "not-applicable" && "$MINIO_RUNTIME_CLIENT_IMAGE" == "not-applicable" && \
    "$MINIO_RUNTIME_BUCKET_COUNT" == "not-run" && "$MINIO_RUNTIME_INVENTORY_ENTRIES" == "not-run" && \
    "$MINIO_RUNTIME_CLEANUP" == "not-applicable" ]] || backup_die "MinIO offline evidence contract mismatch"
fi

REDIS_MODE="$(evidence_value "$REDIS_EVIDENCE" redis_mode)"
REDIS_RUNTIME_IMAGE="$(evidence_value "$REDIS_EVIDENCE" redis_runtime_image)"
REDIS_RUNTIME_PING="$(evidence_value "$REDIS_EVIDENCE" redis_runtime_ping)"
REDIS_RUNTIME_DBSIZE="$(evidence_value "$REDIS_EVIDENCE" redis_runtime_dbsize)"
REDIS_RUNTIME_RDB_CHECK="$(evidence_value "$REDIS_EVIDENCE" redis_runtime_rdb_check)"
REDIS_RUNTIME_CLEANUP="$(evidence_value "$REDIS_EVIDENCE" redis_runtime_cleanup)"
REDIS_SOURCE_UNCHANGED="$(evidence_value "$REDIS_EVIDENCE" redis_source_unchanged)"
[[ "$REDIS_RUNTIME_RDB_CHECK" == "ok" && "$REDIS_SOURCE_UNCHANGED" == "1" ]] || \
  backup_die "Redis source/check evidence contract mismatch"
if [[ "$REDIS_RUNTIME_RESTORE_DRILL" == "1" ]]; then
  validate_runtime_drill_image redis "$REDIS_RUNTIME_IMAGE"
  require_uint redis_runtime_dbsize "$REDIS_RUNTIME_DBSIZE"
  [[ "$REDIS_RUNTIME_IMAGE" == "$REDIS_DRILL_IMAGE" && "$REDIS_MODE" == "ephemeral-runtime-restore" && "$REDIS_RUNTIME_PING" == "PONG" && \
    "$REDIS_RUNTIME_CLEANUP" == "completed" ]] || backup_die "Redis runtime restore evidence contract mismatch"
else
  [[ "$REDIS_RUNTIME_IMAGE" == "not-applicable" && "$REDIS_MODE" == "offline-rdb-check" && "$REDIS_RUNTIME_PING" == "not-run" && \
    "$REDIS_RUNTIME_DBSIZE" == "not-run" && "$REDIS_RUNTIME_CLEANUP" == "not-applicable" ]] || \
    backup_die "Redis offline evidence contract mismatch"
fi

# Only after all local/ephemeral evidence contracts pass, pg_restore_drill.sh
# independently repeats the hard-coded DB allowlist and acknowledgement checks
# before issuing any PostgreSQL command.
run_timed postgres env \
  RESTORE_DRILL_ACK=avtotest_restore_drill \
  DRILL_DB=avtotest_restore_drill \
  KEEP_DRILL_DB=0 \
  "$SCRIPT_DIR/pg_restore_drill.sh" "$POSTGRES_DUMP"
POSTGRES_DURATION="$LAST_DURATION"

TOTAL_FINISHED="$(date -u +%s)"
TOTAL_DURATION="$((TOTAL_FINISHED - TOTAL_STARTED))"
DRILL_FINISHED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
SNAPSHOT_CREATED_AT="$(manifest_value "$MANIFEST" created_at)"
SNAPSHOT_CREATED_EPOCH="$(date -u -d "$SNAPSHOT_CREATED_AT" +%s)"
SNAPSHOT_AGE_SECONDS="$((TOTAL_STARTED - SNAPSHOT_CREATED_EPOCH))"
(( SNAPSHOT_AGE_SECONDS >= 0 )) || backup_die "snapshot creation time is in the future"

# Success is not reportable while decrypted artifacts remain on disk. Do the
# guarded removal explicitly (rather than relying only on EXIT semantics), and
# make any cleanup failure a hard drill failure before status=success exists.
if ! safe_remove_scratch "$DRILL_ROOT" "$SCRATCH"; then
  backup_die "restore-drill scratch cleanup failed"
fi
[[ ! -e "$SCRATCH" && ! -L "$SCRATCH" ]] || \
  backup_die "restore-drill scratch still exists after cleanup"
SCRATCH_CLEANED=1

emit_report() {
  cat <<EOF
format=drivergo-restore-drill-v1
status=success
snapshot=$SNAPSHOT_NAME
snapshot_created_at=$SNAPSHOT_CREATED_AT
snapshot_capture_duration_seconds=$(manifest_value "$MANIFEST" capture_duration_seconds)
snapshot_age_seconds_at_drill=$SNAPSHOT_AGE_SECONDS
drill_started_at=$DRILL_STARTED_AT
drill_finished_at=$DRILL_FINISHED_AT
duration_seconds_total=$TOTAL_DURATION
duration_seconds_postgres=$POSTGRES_DURATION
duration_seconds_redis=$REDIS_DURATION
duration_seconds_minio=$MINIO_DURATION
duration_seconds_humo=$HUMO_DURATION
runtime_restore_drill_requested=$RUNTIME_RESTORE_DRILL
redis_runtime_restore_drill=$REDIS_RUNTIME_RESTORE_DRILL
minio_runtime_restore_drill=$MINIO_RUNTIME_RESTORE_DRILL
runtime_drill_timeout_seconds=$RUNTIME_DRILL_TIMEOUT_SECONDS
postgres_target=avtotest_restore_drill
postgres_cleanup=required-and-completed
decrypted_scratch_cleanup=completed
redis_mode=$REDIS_MODE
redis_runtime_image=$REDIS_RUNTIME_IMAGE
redis_runtime_ping=$REDIS_RUNTIME_PING
redis_runtime_dbsize=$REDIS_RUNTIME_DBSIZE
redis_runtime_rdb_check=$REDIS_RUNTIME_RDB_CHECK
redis_runtime_cleanup=$REDIS_RUNTIME_CLEANUP
redis_source_unchanged=$REDIS_SOURCE_UNCHANGED
minio_mode=$MINIO_MODE
minio_extracted_files=$MINIO_EXTRACTED_FILES
minio_runtime_server_image=$MINIO_RUNTIME_SERVER_IMAGE
minio_runtime_client_image=$MINIO_RUNTIME_CLIENT_IMAGE
minio_runtime_ready=$MINIO_RUNTIME_READY
minio_runtime_bucket_count=$MINIO_RUNTIME_BUCKET_COUNT
minio_runtime_inventory_entries=$MINIO_RUNTIME_INVENTORY_ENTRIES
minio_runtime_cleanup=$MINIO_RUNTIME_CLEANUP
minio_source_unchanged=$MINIO_SOURCE_UNCHANGED
humo_mode=$HUMO_MODE
humo_integrity_check=$HUMO_INTEGRITY_CHECK
humo_schema_check=$HUMO_SCHEMA_CHECK
humo_pending_count=$HUMO_PENDING_COUNT
humo_source_unchanged=$HUMO_SOURCE_UNCHANGED
EOF
}

if [[ -n "$DRILL_REPORT_FILE" ]]; then
  [[ "$DRILL_REPORT_FILE" = /* ]] || backup_die "DRILL_REPORT_FILE must be absolute"
  report_parent="$(dirname "$DRILL_REPORT_FILE")"
  [[ -d "$report_parent" && ! -L "$report_parent" ]] || \
    backup_die "DRILL_REPORT_FILE parent must be an existing non-symlink directory"
  if [[ -e "$DRILL_REPORT_FILE" || -L "$DRILL_REPORT_FILE" ]]; then
    [[ -f "$DRILL_REPORT_FILE" && ! -L "$DRILL_REPORT_FILE" ]] || \
      backup_die "DRILL_REPORT_FILE must be a regular non-symlink file when it exists"
  fi
  report_real="$(realpath -m "$DRILL_REPORT_FILE")"
  case "$report_real" in
    "$SNAPSHOT_DIR"|"$SNAPSHOT_DIR"/*)
      backup_die "DRILL_REPORT_FILE must not modify the immutable snapshot"
      ;;
  esac
  REPORT_TMP="$(mktemp "$report_parent/.drivergo-drill-report.XXXXXX")"
  emit_report >"$REPORT_TMP"
  mv -- "$REPORT_TMP" "$DRILL_REPORT_FILE"
  REPORT_TMP=""
  echo "restore drill evidence: ${DRILL_REPORT_FILE}"
fi

emit_report
