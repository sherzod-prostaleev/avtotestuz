#!/usr/bin/env bash
# Non-destructive Redis RDB validation. Never issues RESTORE/FLUSH/CONFIG.
set -euo pipefail
umask 077

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "$SCRIPT_DIR/lib.sh"

RDB="${1:-}"
REDIS_SERVICE="${REDIS_SERVICE:-redis}"
REDIS_HEADER_ONLY_ALLOWED="${REDIS_HEADER_ONLY_ALLOWED:-0}"
DRILL_ROOT="${DRILL_ROOT:-/tmp/drivergo-restore-drill}"
REDIS_RUNTIME_RESTORE_DRILL="${REDIS_RUNTIME_RESTORE_DRILL:-${RUNTIME_RESTORE_DRILL:-0}}"
REDIS_DRILL_IMAGE="${REDIS_DRILL_IMAGE:-}"
RUNTIME_DRILL_TIMEOUT_SECONDS="${RUNTIME_DRILL_TIMEOUT_SECONDS:-30}"

[[ -n "$RDB" ]] || backup_die "usage: redis_restore_drill.sh REDIS_RDB"
[[ -f "$RDB" && ! -L "$RDB" && -s "$RDB" ]] || backup_die "RDB missing, empty, or unsafe: ${RDB}"
require_command sha256sum
require_bool REDIS_HEADER_ONLY_ALLOWED "$REDIS_HEADER_ONLY_ALLOWED"
require_bool REDIS_RUNTIME_RESTORE_DRILL "$REDIS_RUNTIME_RESTORE_DRILL"
require_positive_uint RUNTIME_DRILL_TIMEOUT_SECONDS "$RUNTIME_DRILL_TIMEOUT_SECONDS"
validate_service_name "$REDIS_SERVICE"
[[ "$REDIS_HEADER_ONLY_ALLOWED" != "1" || "$REDIS_RUNTIME_RESTORE_DRILL" != "1" ]] || \
  backup_die "header-only validation cannot satisfy a runtime Redis restore drill"

magic="$(LC_ALL=C head -c 5 "$RDB")"
[[ "$magic" == "REDIS" ]] || backup_die "RDB magic mismatch"
source_digest_before="$(sha256sum -- "$RDB" | awk '{ print $1 }')"

if [[ "$REDIS_HEADER_ONLY_ALLOWED" == "1" ]]; then
  source_digest_after="$(sha256sum -- "$RDB" | awk '{ print $1 }')"
  [[ "$source_digest_after" == "$source_digest_before" ]] || backup_die "source Redis RDB changed during drill"
  echo "redis drill: header-only validation explicitly allowed"
  echo "redis_mode=header-only-explicit"
  echo "redis_runtime_image=not-applicable"
  echo "redis_runtime_ping=not-run"
  echo "redis_runtime_dbsize=not-run"
  echo "redis_runtime_rdb_check=not-run"
  echo "redis_runtime_cleanup=not-applicable"
  echo "redis_source_unchanged=1"
  exit 0
fi

if [[ "$REDIS_RUNTIME_RESTORE_DRILL" == "1" ]]; then
  require_command docker
  require_command chmod
  require_command install
  require_command realpath
  require_command sleep
  require_command timeout
  validate_runtime_drill_image redis "$REDIS_DRILL_IMAGE"
  validate_drill_root "$DRILL_ROOT"
  docker image inspect "$REDIS_DRILL_IMAGE" >/dev/null 2>&1 || \
    backup_die "digest-pinned Redis drill image is not present locally (pull it explicitly before the drill)"

  SCRATCH="$(mktemp -d "$DRILL_ROOT/drivergo-drill.XXXXXX")"
  validate_scratch_child "$DRILL_ROOT" "$SCRATCH"
  token="$(runtime_drill_token "$SCRATCH")"
  container_name="drivergo-redis-drill-${token}"
  validate_runtime_drill_container_name redis "$container_name"
  scratch_rdb="$SCRATCH/dump.rdb"
  install -m 0444 -- "$RDB" "$scratch_rdb"
  validate_runtime_bind_source "$DRILL_ROOT" "$scratch_rdb" file
  chmod 0555 "$SCRATCH"
  validate_runtime_bind_source "$DRILL_ROOT" "$SCRATCH" directory

  container_id=""
  cleanup_runtime_redis() {
    local cleanup_status=0
    if [[ -n "$container_id" ]]; then
      safe_remove_runtime_container redis "$container_name" "$container_id" || cleanup_status=1
    fi
    if [[ -n "$SCRATCH" && -d "$SCRATCH" ]]; then
      # The directory is deliberately read-only while Redis has it mounted.
      # Restore only the guarded scratch root's owner mode before unlinking it.
      chmod 0700 -- "$SCRATCH" || cleanup_status=1
      safe_remove_scratch "$DRILL_ROOT" "$SCRATCH" || cleanup_status=1
    fi
    return "$cleanup_status"
  }
  trap cleanup_runtime_redis EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM

  container_id="$(docker create \
    --name "$container_name" \
    --label com.drivergo.restore-drill=redis \
    --pull never \
    --network none \
    --read-only \
    --user redis \
    --cap-drop ALL \
    --security-opt no-new-privileges \
    --pids-limit 64 \
    --memory 256m \
    --cpus 0.50 \
    --stop-timeout 5 \
    --tmpfs /tmp:rw,noexec,nosuid,size=16m,mode=1777 \
    --mount "type=bind,src=${SCRATCH},dst=/data,readonly" \
    "$REDIS_DRILL_IMAGE" \
    redis-server \
    --dir /data \
    --dbfilename dump.rdb \
    --appendonly no \
    --save '' \
    --protected-mode yes \
    --bind 127.0.0.1 \
    --port 6379 \
    --loglevel warning)"
  validate_runtime_object_id container "$container_id"
  timeout "${RUNTIME_DRILL_TIMEOUT_SECONDS}s" docker start "$container_id" >/dev/null

  pong=""
  deadline="$((SECONDS + 10#$RUNTIME_DRILL_TIMEOUT_SECONDS))"
  while (( SECONDS < deadline )); do
    if pong="$(timeout "${RUNTIME_DRILL_TIMEOUT_SECONDS}s" docker exec "$container_id" \
      redis-cli -h 127.0.0.1 -p 6379 --raw PING 2>/dev/null)"; then
      pong="${pong//$'\r'/}"
      [[ "$pong" == "PONG" ]] && break
    fi
    sleep 1
  done
  [[ "$pong" == "PONG" ]] || backup_die "ephemeral Redis did not become ready before the drill timeout"
  dbsize="$(timeout "${RUNTIME_DRILL_TIMEOUT_SECONDS}s" docker exec "$container_id" \
    redis-cli -h 127.0.0.1 -p 6379 --raw DBSIZE)"
  dbsize="${dbsize//$'\r'/}"
  require_uint redis_runtime_dbsize "$dbsize"
  timeout "${RUNTIME_DRILL_TIMEOUT_SECONDS}s" docker exec "$container_id" \
    redis-check-rdb /data/dump.rdb >/dev/null

  safe_remove_runtime_container redis "$container_name" "$container_id" || \
    backup_die "failed to remove verified ephemeral Redis container"
  container_id=""
  chmod 0700 -- "$SCRATCH"
  safe_remove_scratch "$DRILL_ROOT" "$SCRATCH"
  SCRATCH=""
  trap - EXIT INT TERM

  source_digest_after="$(sha256sum -- "$RDB" | awk '{ print $1 }')"
  [[ "$source_digest_after" == "$source_digest_before" ]] || \
    backup_die "source Redis RDB changed during runtime drill"
  echo "redis drill: ephemeral restore boot, PING, DBSIZE, and RDB check ok (keys=${dbsize})"
  echo "redis_mode=ephemeral-runtime-restore"
  echo "redis_runtime_image=${REDIS_DRILL_IMAGE}"
  echo "redis_runtime_ping=PONG"
  echo "redis_runtime_dbsize=${dbsize}"
  echo "redis_runtime_rdb_check=ok"
  echo "redis_runtime_cleanup=completed"
  echo "redis_source_unchanged=1"
  exit 0
fi

# redis-check-rdb runs against a temporary file in the existing Redis
# container. It never connects to or mutates the live Redis server/data volume.
compose_cmd exec -T "$REDIS_SERVICE" sh -euc '
  # BusyBox mktemp requires the X run at the end of the template.
  tmp="$(mktemp /tmp/drivergo-rdb-drill.XXXXXX)"
  trap '\''rm -f -- "$tmp"'\'' EXIT
  cat >"$tmp"
  redis-check-rdb "$tmp"
' <"$RDB"

source_digest_after="$(sha256sum -- "$RDB" | awk '{ print $1 }')"
[[ "$source_digest_after" == "$source_digest_before" ]] || \
  backup_die "source Redis RDB changed during offline drill"
echo "redis drill: rdb checksum/structure ok"
echo "redis_mode=offline-rdb-check"
echo "redis_runtime_image=not-applicable"
echo "redis_runtime_ping=not-run"
echo "redis_runtime_dbsize=not-run"
echo "redis_runtime_rdb_check=ok"
echo "redis_runtime_cleanup=not-applicable"
echo "redis_source_unchanged=1"
