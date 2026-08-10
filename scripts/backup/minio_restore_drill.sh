#!/usr/bin/env bash
# Non-destructive MinIO volume-tar drill. Extracts only below DRILL_ROOT.
set -euo pipefail
umask 077

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "$SCRIPT_DIR/lib.sh"

ARCHIVE="${1:-}"
DRILL_ROOT="${DRILL_ROOT:-/tmp/drivergo-restore-drill}"
KEEP_DRILL_FILES="${KEEP_DRILL_FILES:-0}"
MINIO_RUNTIME_RESTORE_DRILL="${MINIO_RUNTIME_RESTORE_DRILL:-${RUNTIME_RESTORE_DRILL:-0}}"
MINIO_DRILL_IMAGE="${MINIO_DRILL_IMAGE:-}"
MINIO_MC_DRILL_IMAGE="${MINIO_MC_DRILL_IMAGE:-}"
RUNTIME_DRILL_TIMEOUT_SECONDS="${RUNTIME_DRILL_TIMEOUT_SECONDS:-30}"

[[ -n "$ARCHIVE" ]] || backup_die "usage: minio_restore_drill.sh MINIO_TAR"
[[ -f "$ARCHIVE" && ! -L "$ARCHIVE" && -s "$ARCHIVE" ]] || backup_die "MinIO archive missing, empty, or unsafe: ${ARCHIVE}"
require_command tar
require_command realpath
require_command python3
require_command sha256sum
require_bool KEEP_DRILL_FILES "$KEEP_DRILL_FILES"
require_bool MINIO_RUNTIME_RESTORE_DRILL "$MINIO_RUNTIME_RESTORE_DRILL"
require_positive_uint RUNTIME_DRILL_TIMEOUT_SECONDS "$RUNTIME_DRILL_TIMEOUT_SECONDS"
validate_drill_root "$DRILL_ROOT"
source_digest_before="$(sha256sum -- "$ARCHIVE" | awk '{ print $1 }')"

if [[ "$MINIO_RUNTIME_RESTORE_DRILL" == "1" ]]; then
  require_command docker
  require_command id
  require_command sleep
  require_command timeout
  validate_runtime_drill_image minio "$MINIO_DRILL_IMAGE"
  validate_runtime_drill_image minio-mc "$MINIO_MC_DRILL_IMAGE"
  docker image inspect "$MINIO_DRILL_IMAGE" >/dev/null 2>&1 || \
    backup_die "digest-pinned MinIO drill image is not present locally (pull it explicitly before the drill)"
  docker image inspect "$MINIO_MC_DRILL_IMAGE" >/dev/null 2>&1 || \
    backup_die "digest-pinned MinIO client drill image is not present locally (pull it explicitly before the drill)"
fi

SCRATCH="$(mktemp -d "$DRILL_ROOT/drivergo-drill.XXXXXX")"
validate_scratch_child "$DRILL_ROOT" "$SCRATCH"
server_id=""
client_id=""
network_id=""
server_name=""
client_name=""
network_name=""
cleanup() {
  local cleanup_status=0
  if [[ -n "$client_id" ]]; then
    safe_remove_runtime_container minio-client "$client_name" "$client_id" || cleanup_status=1
  fi
  if [[ -n "$server_id" ]]; then
    safe_remove_runtime_container minio-server "$server_name" "$server_id" || cleanup_status=1
  fi
  if [[ -n "$network_id" ]]; then
    safe_remove_runtime_network "$network_name" "$network_id" || cleanup_status=1
  fi
  if [[ "$KEEP_DRILL_FILES" != "1" ]]; then
    safe_remove_scratch "$DRILL_ROOT" "$SCRATCH" || cleanup_status=1
  fi
  return "$cleanup_status"
}
cleanup_on_exit() {
  local status="$?"
  trap - EXIT
  cleanup || status=1
  exit "$status"
}
trap cleanup_on_exit EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# Parse names as archive metadata rather than line-oriented `tar -t` output so
# embedded newlines cannot bypass the traversal/type checks.
python3 - "$ARCHIVE" <<'PY'
from pathlib import PurePosixPath
import sys
import tarfile

archive_path = sys.argv[1]
seen = set()
try:
    archive = tarfile.open(archive_path, mode="r:*")
except (OSError, tarfile.TarError) as exc:
    raise SystemExit(f"minio drill: unreadable tar: {exc}")

with archive:
    for member in archive:
        name = member.name
        path = PurePosixPath(name)
        if path.is_absolute() or ".." in path.parts:
            raise SystemExit(f"minio drill: unsafe archive path: {name!r}")
        if not (member.isfile() or member.isdir()):
            raise SystemExit(
                "minio drill: unsupported archive entry type "
                f"for {name!r} (only files/directories are allowed)"
            )
        normalized = str(path)
        if normalized not in ("", "."):
            if normalized in seen:
                raise SystemExit(f"minio drill: duplicate archive path: {name!r}")
            seen.add(normalized)
PY

tar --extract --file "$ARCHIVE" --directory "$SCRATCH" \
  --no-same-owner --no-same-permissions --delay-directory-restore

file_count="$(find "$SCRATCH" -type f -print | wc -l | tr -d ' ')"
[[ "$file_count" -gt 0 ]] || backup_die "MinIO drill extracted zero files"

parse_mc_inventory() {
  python3 -c '
import json
import sys

mode = None
seen_buckets_marker = False
seen_inventory_marker = False
buckets = 0
inventory = 0
for raw in sys.stdin:
    line = raw.strip()
    if line == "__DRIVERGO_BUCKETS__":
        mode = "buckets"
        seen_buckets_marker = True
        continue
    if line == "__DRIVERGO_INVENTORY__":
        mode = "inventory"
        seen_inventory_marker = True
        continue
    if not line:
        continue
    if mode is None:
        raise SystemExit("minio drill: unexpected client output before inventory marker")
    try:
        row = json.loads(line)
    except json.JSONDecodeError as exc:
        raise SystemExit(f"minio drill: invalid mc JSON output: {exc}")
    if row.get("status") not in (None, "success"):
        raise SystemExit(f"minio drill: mc reported failure: {row!r}")
    if mode == "buckets":
        buckets += 1
    else:
        inventory += 1
if not seen_buckets_marker or not seen_inventory_marker:
    raise SystemExit("minio drill: incomplete mc inventory output")
print(f"{buckets} {inventory}")
'
}

if [[ "$MINIO_RUNTIME_RESTORE_DRILL" == "1" ]]; then
  validate_runtime_bind_source "$DRILL_ROOT" "$SCRATCH" directory
  token="$(runtime_drill_token "$SCRATCH")"
  server_name="drivergo-minio-server-drill-${token}"
  client_name="drivergo-minio-client-drill-${token}"
  network_name="drivergo-minio-drill-net-${token}"
  validate_runtime_drill_container_name minio-server "$server_name"
  validate_runtime_drill_container_name minio-client "$client_name"
  validate_runtime_drill_network_name "$network_name"
  runtime_user="drilladmin"
  runtime_password="$(python3 -c 'import secrets; print(secrets.token_hex(24))')"
  runtime_uid="$(id -u)"
  runtime_gid="$(id -g)"
  require_uint runtime_uid "$runtime_uid"
  require_uint runtime_gid "$runtime_gid"

  network_id="$(docker network create \
    --driver bridge \
    --internal \
    --label com.drivergo.restore-drill=minio \
    "$network_name")"
  validate_runtime_object_id network "$network_id"
  server_id="$(docker create \
    --name "$server_name" \
    --label com.drivergo.restore-drill=minio-server \
    --pull never \
    --network "$network_name" \
    --network-alias minio-drill \
    --read-only \
    --user "${runtime_uid}:${runtime_gid}" \
    --cap-drop ALL \
    --security-opt no-new-privileges \
    --pids-limit 128 \
    --memory 512m \
    --cpus 0.75 \
    --stop-timeout 10 \
    --tmpfs /tmp:rw,noexec,nosuid,size=32m,mode=1777 \
    --mount "type=bind,src=${SCRATCH},dst=/data" \
    --env HOME=/tmp \
    --env MINIO_BROWSER=off \
    --env "MINIO_ROOT_USER=${runtime_user}" \
    --env "MINIO_ROOT_PASSWORD=${runtime_password}" \
    "$MINIO_DRILL_IMAGE" \
    server /data --address :9000 --console-address 127.0.0.1:9001)"
  validate_runtime_object_id container "$server_id"
  timeout "${RUNTIME_DRILL_TIMEOUT_SECONDS}s" docker start "$server_id" >/dev/null

  ready=0
  deadline="$((SECONDS + 10#$RUNTIME_DRILL_TIMEOUT_SECONDS))"
  while (( SECONDS < deadline )); do
    if timeout "${RUNTIME_DRILL_TIMEOUT_SECONDS}s" docker exec "$server_id" \
      curl -fsS --connect-timeout 2 --max-time 3 \
      http://127.0.0.1:9000/minio/health/ready >/dev/null 2>&1; then
      ready=1
      break
    fi
    sleep 1
  done
  [[ "$ready" == "1" ]] || backup_die "ephemeral MinIO did not become ready before the drill timeout"

  client_id="$(docker create \
    --name "$client_name" \
    --label com.drivergo.restore-drill=minio-client \
    --pull never \
    --network "$network_name" \
    --read-only \
    --user 65532:65532 \
    --cap-drop ALL \
    --security-opt no-new-privileges \
    --pids-limit 64 \
    --memory 128m \
    --cpus 0.25 \
    --tmpfs /tmp:rw,noexec,nosuid,size=16m,mode=1777 \
    --env HOME=/tmp \
    --env MC_CONFIG_DIR=/tmp/mc \
    --env "MC_HOST_drill=http://${runtime_user}:${runtime_password}@minio-drill:9000" \
    --entrypoint /bin/sh \
    "$MINIO_MC_DRILL_IMAGE" \
    -euc 'printf "__DRIVERGO_BUCKETS__\\n"; mc --json ls drill; printf "__DRIVERGO_INVENTORY__\\n"; mc --json ls --recursive drill')"
  validate_runtime_object_id container "$client_id"
  if ! inventory_summary="$(timeout "${RUNTIME_DRILL_TIMEOUT_SECONDS}s" \
    docker start -a "$client_id" | parse_mc_inventory)"; then
    backup_die "ephemeral MinIO bucket/inventory query failed"
  fi
  read -r bucket_count inventory_entries extra <<<"$inventory_summary"
  [[ -z "${extra:-}" ]] || backup_die "unexpected MinIO inventory summary"
  require_positive_uint minio_runtime_bucket_count "$bucket_count"
  require_uint minio_runtime_inventory_entries "$inventory_entries"

  safe_remove_runtime_container minio-client "$client_name" "$client_id" || \
    backup_die "failed to remove verified ephemeral MinIO client"
  client_id=""
  safe_remove_runtime_container minio-server "$server_name" "$server_id" || \
    backup_die "failed to remove verified ephemeral MinIO server"
  server_id=""
  safe_remove_runtime_network "$network_name" "$network_id" || \
    backup_die "failed to remove verified ephemeral MinIO network"
  network_id=""

  source_digest_after="$(sha256sum -- "$ARCHIVE" | awk '{ print $1 }')"
  [[ "$source_digest_after" == "$source_digest_before" ]] || \
    backup_die "source MinIO archive changed during runtime drill"
  echo "minio drill: ephemeral restore readiness and S3 bucket/inventory checks ok (buckets=${bucket_count}, entries=${inventory_entries})"
  echo "minio_mode=ephemeral-runtime-restore"
  echo "minio_extracted_files=${file_count}"
  echo "minio_runtime_server_image=${MINIO_DRILL_IMAGE}"
  echo "minio_runtime_client_image=${MINIO_MC_DRILL_IMAGE}"
  echo "minio_runtime_ready=ok"
  echo "minio_runtime_bucket_count=${bucket_count}"
  echo "minio_runtime_inventory_entries=${inventory_entries}"
  echo "minio_runtime_cleanup=completed"
  echo "minio_source_unchanged=1"
else
  echo "minio drill: tar safe and extractable (files=${file_count})"
  echo "minio_mode=guarded-scratch-extraction"
  echo "minio_extracted_files=${file_count}"
  echo "minio_runtime_server_image=not-applicable"
  echo "minio_runtime_client_image=not-applicable"
  echo "minio_runtime_ready=not-run"
  echo "minio_runtime_bucket_count=not-run"
  echo "minio_runtime_inventory_entries=not-run"
  echo "minio_runtime_cleanup=not-applicable"
fi
source_digest_after="$(sha256sum -- "$ARCHIVE" | awk '{ print $1 }')"
[[ "$source_digest_after" == "$source_digest_before" ]] || \
  backup_die "source MinIO archive changed during drill"
if [[ "$MINIO_RUNTIME_RESTORE_DRILL" != "1" ]]; then
  echo "minio_source_unchanged=1"
fi
if [[ "$KEEP_DRILL_FILES" == "1" ]]; then
  echo "minio drill scratch retained: ${SCRATCH}"
fi
