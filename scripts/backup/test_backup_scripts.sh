#!/usr/bin/env bash
# Repository-only backup/DR tests. No Docker daemon, live service, age key, or
# rclone remote is contacted; all writable fixtures stay below a mktemp root.
set -euo pipefail
umask 077

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

for command in bash flock grep python3 sha256sum tar; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "test prerequisite missing: ${command}" >&2
    exit 1
  }
done

TEST_ROOT="$(mktemp -d /tmp/drivergo-backup-tests.XXXXXX)"
cleanup() {
  case "$TEST_ROOT" in
    /tmp/drivergo-backup-tests.*) rm -rf -- "$TEST_ROOT" ;;
    *) echo "refusing unsafe test cleanup: ${TEST_ROOT}" >&2 ;;
  esac
}
trap cleanup EXIT INT TERM

pass_count=0
pass() {
  pass_count="$((pass_count + 1))"
  echo "ok ${pass_count} - $*"
}

expect_failure() {
  local label="$1"
  shift
  if "$@" >"$TEST_ROOT/unexpected.stdout" 2>"$TEST_ROOT/unexpected.stderr"; then
    echo "not ok - expected failure: ${label}" >&2
    return 1
  fi
  pass "$label"
}

for script in "$SCRIPT_DIR"/*.sh; do
  bash -n "$script"
done
pass "all backup scripts pass bash -n"

# Both production Redis paths execute inside the Alpine/BusyBox container.
# BusyBox rejects templates whose X run is followed by a filename suffix.
grep -Fq 'mktemp /tmp/drivergo-redis.XXXXXX)' "$SCRIPT_DIR/backup_all.sh"
grep -Fq 'mktemp /tmp/drivergo-rdb-drill.XXXXXX)' "$SCRIPT_DIR/redis_restore_drill.sh"
if grep -Eq 'mktemp /tmp/drivergo-(redis|rdb-drill)\.X+\.[A-Za-z0-9]' \
  "$SCRIPT_DIR/backup_all.sh" "$SCRIPT_DIR/redis_restore_drill.sh"; then
  echo "not ok - Redis mktemp template has a BusyBox-incompatible suffix" >&2
  exit 1
fi
pass "Redis mktemp templates are BusyBox-compatible"

SERVICE_FILE="$ROOT/deploy/systemd/drivergo-backup.service"
TIMER_FILE="$ROOT/deploy/systemd/drivergo-backup.timer"
grep -Fq 'ExecStart=/opt/drivergo/scripts/backup/backup_all.sh' "$SERVICE_FILE"
grep -Fq 'Environment="ALLOW_PLAINTEXT_BACKUP=0"' "$SERVICE_FILE"
grep -Fq 'Environment="REQUIRE_OFFSITE_BACKUP=1"' "$SERVICE_FILE"
grep -Fq 'Environment="PRUNE_OFFSITE=1"' "$SERVICE_FILE"
grep -Fq 'Environment="BACKUP_QUARANTINE_ROOT=/var/backups/drivergo/quarantine"' "$SERVICE_FILE"
grep -Fq 'ReadWritePaths=/var/backups/drivergo' "$SERVICE_FILE"
grep -Fq 'Persistent=true' "$TIMER_FILE"
pass "systemd schedule is full-stack and fail-closed"

PRODUCTION_COMPOSE="$ROOT/deploy/docker-compose.prod.yml"
BACKUP_RUNBOOK="$SCRIPT_DIR/README.md"
PRODUCTION_REDIS_IMAGE='redis:7-alpine@sha256:6ab0b6e7381779332f97b8ca76193e45b0756f38d4c0dcda72dbb3c32061ab99'
PRODUCTION_MINIO_IMAGE='minio/minio@sha256:14cea493d9a34af32f524e538b8346cf79f3321eff8e708c1e2960462bd8936e'
PRODUCTION_MINIO_MC_IMAGE='minio/mc@sha256:a7fe349ef4bd8521fb8497f55c6042871b2ae640607cf99d9bede5e9bdf11727'
for image in "$PRODUCTION_REDIS_IMAGE" "$PRODUCTION_MINIO_IMAGE" "$PRODUCTION_MINIO_MC_IMAGE"; do
  grep -Fq "image: $image" "$PRODUCTION_COMPOSE"
  grep -Fq "='$image'" "$BACKUP_RUNBOOK"
done
pass "runtime drill rollout digests match committed production images"

SNAPSHOT="$TEST_ROOT/drivergo-20200101T000000Z"
mkdir -m 0700 "$SNAPSHOT"
printf 'postgres-fixture\n' >"$SNAPSHOT/postgres.dump"
printf 'REDIS0009fixture\n' >"$SNAPSHOT/redis.rdb"
printf 'minio-fixture\n' >"$SNAPSHOT/minio-data.tar"
printf 'sqlite-fixture\n' >"$SNAPSHOT/humo-queue.sqlite3"
cat >"$SNAPSHOT/manifest.txt" <<'EOF'
format=drivergo-full-backup-v1
snapshot=drivergo-20200101T000000Z
created_at=2020-01-01T00:00:00Z
backup_started_at=2020-01-01T00:00:00Z
capture_duration_seconds=0
source_host=fixture
git_commit=0000000000000000000000000000000000000000
encryption=none
component_file_postgres=postgres.dump
component_file_redis=redis.rdb
component_file_minio=minio-data.tar
component_file_humo=humo-queue.sqlite3
postgres_database=avtotest
minio_consistency=live-volume-tar-stream
humo_consistency=sqlite-online-backup
technical_backup_interval=24h15m-max-with-committed-timer
size_postgres_dump=17
size_redis_rdb=17
size_minio_data_tar=14
size_humo_queue_sqlite3=15
EOF
(
  cd "$SNAPSHOT"
  sha256sum -- manifest.txt postgres.dump redis.rdb minio-data.tar humo-queue.sqlite3 >SHA256SUMS
)
"$SCRIPT_DIR/verify_snapshot.sh" "$SNAPSHOT" >/dev/null
pass "complete fixture snapshot verifies"

clone_plain_snapshot() {
  local target_root="$1"
  local name="$2"
  local target="$target_root/$name"
  local stamp="${name#drivergo-}"
  local created_at="${stamp:0:4}-${stamp:4:2}-${stamp:6:2}T${stamp:9:2}:${stamp:11:2}:${stamp:13:2}Z"
  cp -a -- "$SNAPSHOT" "$target"
  sed -i \
    -e "s/^snapshot=.*/snapshot=$name/" \
    -e "s/^created_at=.*/created_at=$created_at/" \
    -e "s/^backup_started_at=.*/backup_started_at=$created_at/" \
    "$target/manifest.txt"
  rm -f -- "$target/REMOTE_COMPLETE"
  (
    cd "$target"
    sha256sum -- manifest.txt postgres.dump redis.rdb minio-data.tar humo-queue.sqlite3 >SHA256SUMS
  )
}

mark_remote_complete() {
  local snapshot="$1"
  local name digest
  name="$(basename "$snapshot")"
  digest="$(sha256sum -- "$snapshot/SHA256SUMS" | awk '{ print $1 }')"
  printf 'snapshot=%s\nsha256sums_sha256=%s\n' "$name" "$digest" >"$snapshot/REMOTE_COMPLETE"
}

printf 'tamper\n' >>"$SNAPSHOT/postgres.dump"
expect_failure "snapshot tampering is detected" "$SCRIPT_DIR/verify_snapshot.sh" "$SNAPSHOT"
printf 'postgres-fixture\n' >"$SNAPSHOT/postgres.dump"

cp "$SNAPSHOT/SHA256SUMS" "$TEST_ROOT/SHA256SUMS.good"
awk 'NR == 5 { print first; next } NR == 1 { first=$0 } { print }' \
  "$TEST_ROOT/SHA256SUMS.good" >"$SNAPSHOT/SHA256SUMS"
expect_failure "duplicate checksum entry is rejected" "$SCRIPT_DIR/verify_snapshot.sh" "$SNAPSHOT"
cp "$TEST_ROOT/SHA256SUMS.good" "$SNAPSHOT/SHA256SUMS"

printf 'REDIS0011fixture\n' >"$TEST_ROOT/valid.rdb"
REDIS_HEADER_ONLY_ALLOWED=1 "$SCRIPT_DIR/redis_restore_drill.sh" "$TEST_ROOT/valid.rdb" >/dev/null
pass "Redis header-only fixture check avoids live Redis"
printf 'NOT-A-REDIS-DUMP\n' >"$TEST_ROOT/invalid.rdb"
expect_failure "invalid Redis magic is rejected" env REDIS_HEADER_ONLY_ALLOWED=1 \
  "$SCRIPT_DIR/redis_restore_drill.sh" "$TEST_ROOT/invalid.rdb"

python3 - "$TEST_ROOT/humo.sqlite3" <<'PY'
import sqlite3
import sys

db = sqlite3.connect(sys.argv[1])
db.execute(
    "CREATE TABLE pending_ingest ("
    "msg_id INTEGER PRIMARY KEY, raw_text TEXT NOT NULL, created_at INTEGER NOT NULL)"
)
db.execute(
    "INSERT INTO pending_ingest(msg_id, raw_text, created_at) "
    "VALUES (1, 'fixture', 1)"
)
db.commit()
db.close()
PY
"$SCRIPT_DIR/humo_restore_drill.sh" "$TEST_ROOT/humo.sqlite3" >/dev/null
pass "Humo SQLite fixture passes read-only integrity drill"

python3 - "$TEST_ROOT/humo-wrong-schema.sqlite3" <<'PY'
import sqlite3
import sys

db = sqlite3.connect(sys.argv[1])
db.execute("CREATE TABLE pending_ingest (id INTEGER PRIMARY KEY, payload TEXT)")
db.commit()
db.close()
PY
expect_failure "Humo drill rejects a structurally valid database with the wrong spool schema" \
  "$SCRIPT_DIR/humo_restore_drill.sh" "$TEST_ROOT/humo-wrong-schema.sqlite3"

MINIO_SOURCE="$TEST_ROOT/minio-source"
DRILL_ROOT="$TEST_ROOT/drivergo-restore-drill"
mkdir -m 0700 -p "$MINIO_SOURCE/bucket"
printf 'object\n' >"$MINIO_SOURCE/bucket/object.txt"
tar -cf "$TEST_ROOT/minio.tar" -C "$MINIO_SOURCE" .
DRILL_ROOT="$DRILL_ROOT" "$SCRIPT_DIR/minio_restore_drill.sh" "$TEST_ROOT/minio.tar" >/dev/null
[[ -z "$(find "$DRILL_ROOT" -mindepth 1 -print -quit)" ]]
pass "MinIO fixture extracts only in guarded scratch and cleans up"

python3 - "$TEST_ROOT/traversal.tar" <<'PY'
import io
import tarfile
import sys

with tarfile.open(sys.argv[1], "w") as archive:
    payload = b"escape"
    member = tarfile.TarInfo("../escape")
    member.size = len(payload)
    archive.addfile(member, io.BytesIO(payload))
PY
expect_failure "MinIO path traversal entry is rejected" env DRILL_ROOT="$DRILL_ROOT" \
  "$SCRIPT_DIR/minio_restore_drill.sh" "$TEST_ROOT/traversal.tar"

ln -s object.txt "$MINIO_SOURCE/bucket/link"
tar -cf "$TEST_ROOT/symlink.tar" -C "$MINIO_SOURCE/bucket" link
expect_failure "MinIO non-file/directory entry is rejected" env DRILL_ROOT="$DRILL_ROOT" \
  "$SCRIPT_DIR/minio_restore_drill.sh" "$TEST_ROOT/symlink.tar"

expect_failure "PostgreSQL live database name is rejected before Docker" env \
  DRILL_DB=avtotest RESTORE_DRILL_ACK=avtotest_restore_drill \
  "$SCRIPT_DIR/pg_restore_drill.sh" "$TEST_ROOT/missing.dump"
expect_failure "PostgreSQL arbitrary database name is rejected before Docker" env \
  DRILL_DB=attacker_choice RESTORE_DRILL_ACK=avtotest_restore_drill \
  "$SCRIPT_DIR/pg_restore_drill.sh" "$TEST_ROOT/missing.dump"
expect_failure "PostgreSQL drill requires explicit acknowledgement" env \
  DRILL_DB=avtotest_restore_drill \
  "$SCRIPT_DIR/pg_restore_drill.sh" "$TEST_ROOT/missing.dump"

expect_failure "full drill requires acknowledgement before snapshot access" \
  "$SCRIPT_DIR/full_restore_drill.sh" "$TEST_ROOT/not-a-snapshot"

expect_failure "broad backup root is rejected without chmod/removal" bash -c \
  'source "$1"; validate_backup_root /tmp' _ "$SCRIPT_DIR/lib.sh"
expect_failure "broad drill root is rejected without chmod/removal" bash -c \
  'source "$1"; validate_drill_root /tmp' _ "$SCRIPT_DIR/lib.sh"
expect_failure "rclone traversal prefix is rejected" bash -c \
  'source "$1"; validate_remote_base "remote:../unsafe"' _ "$SCRIPT_DIR/lib.sh"

PRODUCTION_POLICY_ROOT="$TEST_ROOT/production-policy/full"
expect_failure "production CLI policy cannot disable required off-site backup" env \
  PATH="$PATH" \
  BACKUP_ROOT="$PRODUCTION_POLICY_ROOT" \
  BACKUP_LOCK_FILE="$TEST_ROOT/production-policy.lock" \
  ALLOW_PLAINTEXT_BACKUP=1 \
  REQUIRE_OFFSITE_BACKUP=0 \
  PRUNE_OFFSITE=0 \
  "$SCRIPT_DIR/backup_all.sh" --production

# Exercise the full orchestrator with command fixtures. These executables
# shadow docker/age/rclone only inside this one process; no daemon or remote is
# contacted.
MOCK_BIN="$TEST_ROOT/mock-bin"
mkdir -m 0700 "$MOCK_BIN"
cat >"$MOCK_BIN/docker" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
REDIS_ID=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
MINIO_SERVER_ID=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
MINIO_CLIENT_ID=cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
MINIO_NETWORK_ID=dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd
if [[ -n "${MOCK_DOCKER_LOG:-}" ]]; then
  printf '%q ' "$@" >>"$MOCK_DOCKER_LOG"
  printf '\n' >>"$MOCK_DOCKER_LOG"
fi
if [[ "${1:-}" == "compose" ]]; then
  shift
  while [[ "${1:-}" == "-f" || "${1:-}" == "--env-file" ]]; do
    shift 2
  done
  case "${1:-}" in
    ps)
      printf 'mock-minio-container\n'
      ;;
    exec)
      shift
      [[ "${1:-}" == "-T" ]] && shift
      service="${1:-}"
      shift
      if [[ -n "${MOCK_FAIL_SERVICE:-}" && "$service" == "$MOCK_FAIL_SERVICE" ]]; then
        exit 42
      fi
      case "$service" in
        postgres)
          case "${1:-}" in
            pg_dump) printf 'PGDMP fixture postgres\n' ;;
            pg_restore) command cat >/dev/null ;;
            psql)
              sql="$(command cat)"
              if [[ "$sql" == *information_schema.tables* ]]; then
                printf '5 3 1 2\n'
              fi
              ;;
            *) echo "unexpected mock postgres command: ${1:-}" >&2; exit 1 ;;
          esac
          ;;
        redis)
          if [[ "$*" == *"redis-cli --rdb"* ]]; then
            printf 'REDIS0011fixture redis\n'
          elif [[ "$*" == *redis-check-rdb* ]]; then
            command cat >/dev/null
            printf 'mock redis-check-rdb ok\n'
          else
            echo "unexpected mock redis command: $*" >&2
            exit 1
          fi
          ;;
        humo-watcher) command cat "$MOCK_HUMO_DB" ;;
        *) echo "unexpected mock compose service: ${service}" >&2; exit 1 ;;
      esac
      ;;
    *) echo "unexpected mock compose command: ${1:-}" >&2; exit 1 ;;
  esac
elif [[ "${1:-}" == "cp" ]]; then
  command cat "$MOCK_MINIO_TAR"
elif [[ "${1:-}" == "image" && "${2:-}" == "inspect" ]]; then
  [[ "${MOCK_RUNTIME_FAIL:-}" != "image_inspect" ]]
elif [[ "${1:-}" == "network" ]]; then
  case "${2:-}" in
    create)
      [[ -n "${MOCK_DOCKER_STATE_DIR:-}" ]]
      name="${!#}"
      printf '%s|minio\n' "$name" >"$MOCK_DOCKER_STATE_DIR/$MINIO_NETWORK_ID.identity"
      printf '%s\n' "$MINIO_NETWORK_ID"
      ;;
    inspect)
      id="${!#}"
      command cat "$MOCK_DOCKER_STATE_DIR/$id.identity"
      ;;
    rm)
      id="${!#}"
      [[ "${MOCK_RUNTIME_FAIL:-}" != "network_cleanup" ]]
      command rm -f -- "$MOCK_DOCKER_STATE_DIR/$id.identity"
      printf '%s\n' "$id"
      ;;
    *) echo "unexpected mock docker network command: $*" >&2; exit 1 ;;
  esac
elif [[ "${1:-}" == "create" ]]; then
  [[ -n "${MOCK_DOCKER_STATE_DIR:-}" ]]
  name=''
  label=''
  shift
  while [[ "$#" -gt 0 ]]; do
    case "$1" in
      --name) name="$2"; shift 2 ;;
      --label)
        label="${2#com.drivergo.restore-drill=}"
        shift 2
        ;;
      *) shift ;;
    esac
  done
  case "$label" in
    redis) id="$REDIS_ID" ;;
    minio-server) id="$MINIO_SERVER_ID" ;;
    minio-client) id="$MINIO_CLIENT_ID" ;;
    *) echo "unexpected runtime drill label: $label" >&2; exit 1 ;;
  esac
  printf '/%s|%s\n' "$name" "$label" >"$MOCK_DOCKER_STATE_DIR/$id.identity"
  printf '%s\n' "$id"
elif [[ "${1:-}" == "start" ]]; then
  id="${!#}"
  if [[ "$*" == *' -a '* || "${2:-}" == "-a" ]]; then
    [[ "$id" == "$MINIO_CLIENT_ID" ]]
    [[ "${MOCK_RUNTIME_FAIL:-}" != "minio_inventory" ]]
    printf '%s\n' \
      __DRIVERGO_BUCKETS__ \
      '{"status":"success","key":"media/"}' \
      '{"status":"success","key":"support-attachments/"}' \
      __DRIVERGO_INVENTORY__ \
      '{"status":"success","key":"media/images/object.jpg"}'
  else
    printf '%s\n' "$id"
  fi
elif [[ "${1:-}" == "exec" ]]; then
  id="${2:-}"
  shift 2
  case "$id" in
    "$REDIS_ID")
      if [[ "$*" == *PING* ]]; then
        [[ "${MOCK_RUNTIME_FAIL:-}" != "redis_ping" ]]
        printf 'PONG\n'
      elif [[ "$*" == *DBSIZE* ]]; then
        [[ "${MOCK_RUNTIME_FAIL:-}" != "redis_dbsize" ]]
        printf '7\n'
      elif [[ "${1:-}" == "redis-check-rdb" ]]; then
        [[ "${MOCK_RUNTIME_FAIL:-}" != "redis_rdb_check" ]]
        printf 'mock runtime redis-check-rdb ok\n'
      else
        echo "unexpected mock Redis runtime exec: $*" >&2
        exit 1
      fi
      ;;
    "$MINIO_SERVER_ID")
      [[ "$*" == *'/minio/health/ready'* ]]
      [[ "${MOCK_RUNTIME_FAIL:-}" != "minio_ready" ]]
      ;;
    *) echo "unexpected mock runtime container id: $id" >&2; exit 1 ;;
  esac
elif [[ "${1:-}" == "inspect" ]]; then
  id="${!#}"
  command cat "$MOCK_DOCKER_STATE_DIR/$id.identity"
elif [[ "${1:-}" == "rm" ]]; then
  id="${!#}"
  [[ "${MOCK_RUNTIME_FAIL:-}" != "container_cleanup" ]]
  command rm -f -- "$MOCK_DOCKER_STATE_DIR/$id.identity"
  printf '%s\n' "$id"
else
  echo "unexpected mock docker command: $*" >&2
  exit 1
fi
MOCK
cat >"$MOCK_BIN/age" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
output=''
input=''
while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --recipient|--identity) shift 2 ;;
    --output) output="$2"; shift 2 ;;
    --decrypt) shift ;;
    *) input="$1"; shift ;;
  esac
done
[[ -n "$output" ]]
if [[ -n "$input" ]]; then
  command cat "$input" >"$output"
else
  command cat >"$output"
fi
if [[ "${MOCK_AGE_EMPTY_ENVELOPE:-0}" == "1" && ! -s "$output" ]]; then
  # Real age still writes a syntactically non-empty envelope for empty input.
  # This fixture proves the orchestrator checks the producer's exit status,
  # not merely the encrypted file size.
  printf 'age-envelope-with-empty-payload\n' >"$output"
fi
MOCK
cat >"$MOCK_BIN/rclone" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"$MOCK_RCLONE_LOG"
if [[ "${MOCK_RCLONE_LIST_MODE:-}" == "retention" ]]; then
  prefix="${MOCK_RCLONE_REMOTE_PREFIX:-fixture:drivergo/full}"
  root="${MOCK_RCLONE_REMOTE_ROOT:?}"
  case "${1:-}" in
    lsf)
      remote="${2:-}"
      if [[ "$remote" == "$prefix" ]]; then
        find "$root" -mindepth 1 -maxdepth 1 -type d -printf '%f/\n' | sort
      else
        relative="${remote#"$prefix"/}"
        [[ "$relative" != "$remote" && "$relative" != */* ]]
        find "$root/$relative" -mindepth 1 -maxdepth 1 -type f -printf '%f\n' | sort
      fi
      ;;
    copyto)
      source="${2:-}"
      destination="${3:-}"
      relative="${source#"$prefix"/}"
      [[ "$relative" != "$source" && "$relative" == */* ]]
      command cp -- "$root/$relative" "$destination"
      ;;
    hashsum)
      remote="${3:-}"
      relative="${remote#"$prefix"/}"
      [[ "$relative" != "$remote" && "$relative" != */* ]]
      if [[ "${MOCK_RCLONE_FAIL_HASH_NAME:-}" == "$relative" ]]; then
        exit 75
      fi
      checkfile=''
      shift 3
      while (($#)); do
        case "$1" in
          --checkfile) checkfile="$2"; shift 2 ;;
          *) shift ;;
        esac
      done
      [[ -n "$checkfile" ]]
      (cd "$root/$relative" && sha256sum --check --strict "$checkfile")
      ;;
    purge) ;;
    *) echo "unexpected retention rclone command: $*" >&2; exit 1 ;;
  esac
  exit 0
fi
if [[ "${1:-}" == "rcat" ]]; then
  command cat >/dev/null
fi
MOCK
chmod 0700 "$MOCK_BIN/docker" "$MOCK_BIN/age" "$MOCK_BIN/rclone"

OFFSITE_PREFLIGHT="$SCRIPT_DIR/offsite_preflight.sh"
OFFSITE_RCLONE_CONFIG="$TEST_ROOT/offsite-rclone.conf"
printf '[fixture]\ntype = s3\n' >"$OFFSITE_RCLONE_CONFIG"
chmod 0600 "$OFFSITE_RCLONE_CONFIG"
OFFSITE_PREFLIGHT_LOG="$TEST_ROOT/offsite-preflight-rclone.log"
: >"$OFFSITE_PREFLIGHT_LOG"
PATH="$MOCK_BIN:$PATH" \
MOCK_RCLONE_LOG="$OFFSITE_PREFLIGHT_LOG" \
AGE_RECIPIENT=age1fixture \
RCLONE_REMOTE=fixture:drivergo/full \
RCLONE_CONFIG="$OFFSITE_RCLONE_CONFIG" \
  "$OFFSITE_PREFLIGHT" >"$TEST_ROOT/offsite-preflight.out"
grep -Fq 'off-site preflight passed' "$TEST_ROOT/offsite-preflight.out"
grep -Fq 'lsd fixture:drivergo/full' "$OFFSITE_PREFLIGHT_LOG"
if grep -Fq 'age1fixture' "$TEST_ROOT/offsite-preflight.out" || \
   grep -Fq 'fixture:drivergo/full' "$TEST_ROOT/offsite-preflight.out"; then
  echo "not ok - off-site preflight printed configuration values" >&2
  exit 1
fi
pass "off-site preflight validates a protected config without printing values"

expect_failure "off-site preflight rejects a missing age recipient" env \
  OFFSITE_PREFLIGHT_SKIP_REMOTE=1 \
  RCLONE_REMOTE=fixture:drivergo/full \
  RCLONE_CONFIG="$OFFSITE_RCLONE_CONFIG" \
  "$OFFSITE_PREFLIGHT"
expect_failure "off-site preflight rejects a blank age recipient" env \
  OFFSITE_PREFLIGHT_SKIP_REMOTE=1 \
  AGE_RECIPIENT='   ' \
  RCLONE_REMOTE=fixture:drivergo/full \
  RCLONE_CONFIG="$OFFSITE_RCLONE_CONFIG" \
  "$OFFSITE_PREFLIGHT"
expect_failure "off-site preflight rejects an unsafe remote prefix" env \
  OFFSITE_PREFLIGHT_SKIP_REMOTE=1 \
  AGE_RECIPIENT=age1fixture \
  RCLONE_REMOTE=fixture:../unsafe \
  RCLONE_CONFIG="$OFFSITE_RCLONE_CONFIG" \
  "$OFFSITE_PREFLIGHT"
chmod 0640 "$OFFSITE_RCLONE_CONFIG"
expect_failure "off-site preflight requires mode 0600 rclone config" env \
  OFFSITE_PREFLIGHT_SKIP_REMOTE=1 \
  AGE_RECIPIENT=age1fixture \
  RCLONE_REMOTE=fixture:drivergo/full \
  RCLONE_CONFIG="$OFFSITE_RCLONE_CONFIG" \
  "$OFFSITE_PREFLIGHT"
chmod 0600 "$OFFSITE_RCLONE_CONFIG"
: >"$OFFSITE_PREFLIGHT_LOG"
PATH="$MOCK_BIN:$PATH" \
MOCK_RCLONE_LOG="$OFFSITE_PREFLIGHT_LOG" \
OFFSITE_PREFLIGHT_SKIP_REMOTE=1 \
AGE_RECIPIENT=age1fixture \
RCLONE_REMOTE=fixture:drivergo/full \
RCLONE_CONFIG="$OFFSITE_RCLONE_CONFIG" \
  "$OFFSITE_PREFLIGHT" >/dev/null
[[ ! -s "$OFFSITE_PREFLIGHT_LOG" ]]
pass "off-site preflight dry mode does not contact rclone"

REDIS_RUNTIME_IMAGE="redis:7-alpine@sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
MINIO_RUNTIME_IMAGE="minio/minio@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
MINIO_MC_RUNTIME_IMAGE="minio/mc@sha256:abababababababababababababababababababababababababababababababab"

INVALID_IMAGE_LOG="$TEST_ROOT/invalid-image-docker.log"
: >"$INVALID_IMAGE_LOG"
expect_failure "Redis runtime drill rejects a floating image before Docker" env \
  PATH="$MOCK_BIN:$PATH" \
  MOCK_DOCKER_LOG="$INVALID_IMAGE_LOG" \
  REDIS_RUNTIME_RESTORE_DRILL=1 \
  REDIS_DRILL_IMAGE=redis:latest \
  DRILL_ROOT="$TEST_ROOT/invalid-redis/drivergo-restore-drill" \
  "$SCRIPT_DIR/redis_restore_drill.sh" "$TEST_ROOT/valid.rdb"
[[ ! -s "$INVALID_IMAGE_LOG" ]]

expect_failure "MinIO runtime drill rejects a non-allowlisted image before Docker" env \
  PATH="$MOCK_BIN:$PATH" \
  MOCK_DOCKER_LOG="$INVALID_IMAGE_LOG" \
  MINIO_RUNTIME_RESTORE_DRILL=1 \
  MINIO_DRILL_IMAGE="attacker.example/minio@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff" \
  MINIO_MC_DRILL_IMAGE="$MINIO_MC_RUNTIME_IMAGE" \
  DRILL_ROOT="$TEST_ROOT/invalid-minio/drivergo-restore-drill" \
  "$SCRIPT_DIR/minio_restore_drill.sh" "$TEST_ROOT/minio.tar"
[[ ! -s "$INVALID_IMAGE_LOG" ]]

expect_failure "runtime cleanup rejects a non-drill container name before Docker" bash -c \
  'source "$1"; safe_remove_runtime_container redis production-redis aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' \
  _ "$SCRIPT_DIR/lib.sh"

RUNTIME_PATH_ROOT="$TEST_ROOT/runtime-path/drivergo-restore-drill"
mkdir -p -m 0700 "$RUNTIME_PATH_ROOT"
printf 'outside\n' >"$TEST_ROOT/runtime-path/outside.rdb"
expect_failure "runtime bind validation rejects a source outside guarded scratch" bash -c \
  'source "$1"; validate_runtime_bind_source "$2" "$3" file' \
  _ "$SCRIPT_DIR/lib.sh" "$RUNTIME_PATH_ROOT" "$TEST_ROOT/runtime-path/outside.rdb"

CLEANUP_ID=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
CLEANUP_STATE="$TEST_ROOT/runtime-cleanup/docker-state"
CLEANUP_LOG="$TEST_ROOT/runtime-cleanup/docker.log"
mkdir -p -m 0700 "$CLEANUP_STATE"
printf '/drivergo-redis-drill-abcdef|wrong-label\n' >"$CLEANUP_STATE/$CLEANUP_ID.identity"
expect_failure "runtime cleanup refuses a container whose verified label changed" env \
  PATH="$MOCK_BIN:$PATH" \
  MOCK_DOCKER_STATE_DIR="$CLEANUP_STATE" \
  MOCK_DOCKER_LOG="$CLEANUP_LOG" \
  bash -c 'source "$1"; safe_remove_runtime_container redis drivergo-redis-drill-abcdef "$2"' \
  _ "$SCRIPT_DIR/lib.sh" "$CLEANUP_ID"
if grep -Fq 'rm -f' "$CLEANUP_LOG"; then
  echo "not ok - identity-mismatched runtime container reached docker rm" >&2
  exit 1
fi

REDIS_RUNTIME_ROOT="$TEST_ROOT/runtime-redis-success/drivergo-restore-drill"
REDIS_RUNTIME_STATE="$TEST_ROOT/runtime-redis-success/docker-state"
REDIS_RUNTIME_LOG="$TEST_ROOT/runtime-redis-success/docker.log"
mkdir -p -m 0700 "$REDIS_RUNTIME_STATE"
redis_source_digest="$(sha256sum -- "$TEST_ROOT/valid.rdb" | awk '{ print $1 }')"
PATH="$MOCK_BIN:$PATH" \
MOCK_DOCKER_STATE_DIR="$REDIS_RUNTIME_STATE" \
MOCK_DOCKER_LOG="$REDIS_RUNTIME_LOG" \
REDIS_RUNTIME_RESTORE_DRILL=1 \
REDIS_DRILL_IMAGE="$REDIS_RUNTIME_IMAGE" \
RUNTIME_DRILL_TIMEOUT_SECONDS=2 \
DRILL_ROOT="$REDIS_RUNTIME_ROOT" \
  "$SCRIPT_DIR/redis_restore_drill.sh" "$TEST_ROOT/valid.rdb" >"$TEST_ROOT/runtime-redis.out"
grep -Fq 'redis_mode=ephemeral-runtime-restore' "$TEST_ROOT/runtime-redis.out"
grep -Fq "redis_runtime_image=$REDIS_RUNTIME_IMAGE" "$TEST_ROOT/runtime-redis.out"
grep -Fq 'redis_runtime_ping=PONG' "$TEST_ROOT/runtime-redis.out"
grep -Fq 'redis_runtime_dbsize=7' "$TEST_ROOT/runtime-redis.out"
grep -Fq 'redis_runtime_cleanup=completed' "$TEST_ROOT/runtime-redis.out"
[[ "$(sha256sum -- "$TEST_ROOT/valid.rdb" | awk '{ print $1 }')" == "$redis_source_digest" ]]
[[ -z "$(find "$REDIS_RUNTIME_STATE" -mindepth 1 -print -quit)" ]]
[[ -z "$(find "$REDIS_RUNTIME_ROOT" -mindepth 1 -print -quit)" ]]
grep -Fq -- '--pull never' "$REDIS_RUNTIME_LOG"
grep -Fq -- '--network none' "$REDIS_RUNTIME_LOG"
grep -Eq -- '--name drivergo-redis-drill-[a-z0-9]{6}' "$REDIS_RUNTIME_LOG"
grep -Fq -- 'com.drivergo.restore-drill=redis' "$REDIS_RUNTIME_LOG"
if grep -Eq -- '^create .* (-p|--publish|--volume|--volumes-from)( |$)' "$REDIS_RUNTIME_LOG" || \
  grep -Fq 'compose ' "$REDIS_RUNTIME_LOG"; then
  echo "not ok - Redis runtime drill touched a host port, Docker volume, or Compose" >&2
  exit 1
fi
pass "ephemeral Redis runtime drill uses scratch/network-none and cleans up"

REDIS_FAILURE_ROOT="$TEST_ROOT/runtime-redis-failure/drivergo-restore-drill"
REDIS_FAILURE_STATE="$TEST_ROOT/runtime-redis-failure/docker-state"
REDIS_FAILURE_LOG="$TEST_ROOT/runtime-redis-failure/docker.log"
mkdir -p -m 0700 "$REDIS_FAILURE_STATE"
expect_failure "Redis runtime failure still removes its verified container and scratch" env \
  PATH="$MOCK_BIN:$PATH" \
  MOCK_DOCKER_STATE_DIR="$REDIS_FAILURE_STATE" \
  MOCK_DOCKER_LOG="$REDIS_FAILURE_LOG" \
  MOCK_RUNTIME_FAIL=redis_dbsize \
  REDIS_RUNTIME_RESTORE_DRILL=1 \
  REDIS_DRILL_IMAGE="$REDIS_RUNTIME_IMAGE" \
  RUNTIME_DRILL_TIMEOUT_SECONDS=2 \
  DRILL_ROOT="$REDIS_FAILURE_ROOT" \
  "$SCRIPT_DIR/redis_restore_drill.sh" "$TEST_ROOT/valid.rdb"
[[ -z "$(find "$REDIS_FAILURE_STATE" -mindepth 1 -print -quit)" ]]
[[ -z "$(find "$REDIS_FAILURE_ROOT" -mindepth 1 -print -quit)" ]]
grep -Fq 'rm -f -- aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' "$REDIS_FAILURE_LOG"

MINIO_RUNTIME_ROOT="$TEST_ROOT/runtime-minio-success/drivergo-restore-drill"
MINIO_RUNTIME_STATE="$TEST_ROOT/runtime-minio-success/docker-state"
MINIO_RUNTIME_LOG="$TEST_ROOT/runtime-minio-success/docker.log"
mkdir -p -m 0700 "$MINIO_RUNTIME_STATE"
minio_source_digest="$(sha256sum -- "$TEST_ROOT/minio.tar" | awk '{ print $1 }')"
PATH="$MOCK_BIN:$PATH" \
MOCK_DOCKER_STATE_DIR="$MINIO_RUNTIME_STATE" \
MOCK_DOCKER_LOG="$MINIO_RUNTIME_LOG" \
MINIO_RUNTIME_RESTORE_DRILL=1 \
MINIO_DRILL_IMAGE="$MINIO_RUNTIME_IMAGE" \
MINIO_MC_DRILL_IMAGE="$MINIO_MC_RUNTIME_IMAGE" \
RUNTIME_DRILL_TIMEOUT_SECONDS=2 \
DRILL_ROOT="$MINIO_RUNTIME_ROOT" \
  "$SCRIPT_DIR/minio_restore_drill.sh" "$TEST_ROOT/minio.tar" >"$TEST_ROOT/runtime-minio.out"
grep -Fq 'minio_mode=ephemeral-runtime-restore' "$TEST_ROOT/runtime-minio.out"
grep -Fq "minio_runtime_server_image=$MINIO_RUNTIME_IMAGE" "$TEST_ROOT/runtime-minio.out"
grep -Fq "minio_runtime_client_image=$MINIO_MC_RUNTIME_IMAGE" "$TEST_ROOT/runtime-minio.out"
grep -Fq 'minio_runtime_ready=ok' "$TEST_ROOT/runtime-minio.out"
grep -Fq 'minio_runtime_bucket_count=2' "$TEST_ROOT/runtime-minio.out"
grep -Fq 'minio_runtime_inventory_entries=1' "$TEST_ROOT/runtime-minio.out"
grep -Fq 'minio_runtime_cleanup=completed' "$TEST_ROOT/runtime-minio.out"
[[ "$(sha256sum -- "$TEST_ROOT/minio.tar" | awk '{ print $1 }')" == "$minio_source_digest" ]]
[[ -z "$(find "$MINIO_RUNTIME_STATE" -mindepth 1 -print -quit)" ]]
[[ -z "$(find "$MINIO_RUNTIME_ROOT" -mindepth 1 -print -quit)" ]]
grep -Fq 'network create --driver bridge --internal' "$MINIO_RUNTIME_LOG"
grep -Eq -- '--name drivergo-minio-server-drill-[a-z0-9]{6}' "$MINIO_RUNTIME_LOG"
grep -Eq -- '--name drivergo-minio-client-drill-[a-z0-9]{6}' "$MINIO_RUNTIME_LOG"
grep -Fq -- '--pull never' "$MINIO_RUNTIME_LOG"
if grep -Eq -- '^create .* (-p|--publish|--volume|--volumes-from)( |$)' "$MINIO_RUNTIME_LOG" || \
  grep -Fq 'compose ' "$MINIO_RUNTIME_LOG"; then
  echo "not ok - MinIO runtime drill touched a host port, Docker volume, or Compose" >&2
  exit 1
fi
pass "ephemeral MinIO runtime drill uses an internal network and cleans up"

MINIO_FAILURE_ROOT="$TEST_ROOT/runtime-minio-failure/drivergo-restore-drill"
MINIO_FAILURE_STATE="$TEST_ROOT/runtime-minio-failure/docker-state"
MINIO_FAILURE_LOG="$TEST_ROOT/runtime-minio-failure/docker.log"
mkdir -p -m 0700 "$MINIO_FAILURE_STATE"
expect_failure "MinIO inventory failure still removes client, server, network, and scratch" env \
  PATH="$MOCK_BIN:$PATH" \
  MOCK_DOCKER_STATE_DIR="$MINIO_FAILURE_STATE" \
  MOCK_DOCKER_LOG="$MINIO_FAILURE_LOG" \
  MOCK_RUNTIME_FAIL=minio_inventory \
  MINIO_RUNTIME_RESTORE_DRILL=1 \
  MINIO_DRILL_IMAGE="$MINIO_RUNTIME_IMAGE" \
  MINIO_MC_DRILL_IMAGE="$MINIO_MC_RUNTIME_IMAGE" \
  RUNTIME_DRILL_TIMEOUT_SECONDS=2 \
  DRILL_ROOT="$MINIO_FAILURE_ROOT" \
  "$SCRIPT_DIR/minio_restore_drill.sh" "$TEST_ROOT/minio.tar"
[[ -z "$(find "$MINIO_FAILURE_STATE" -mindepth 1 -print -quit)" ]]
[[ -z "$(find "$MINIO_FAILURE_ROOT" -mindepth 1 -print -quit)" ]]
grep -Fq 'network rm dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd' "$MINIO_FAILURE_LOG"

MOCK_MINIO_SOURCE="$TEST_ROOT/mock-minio-source"
mkdir -m 0700 "$MOCK_MINIO_SOURCE"
printf 'mock object\n' >"$MOCK_MINIO_SOURCE/object"
tar -cf "$TEST_ROOT/mock-minio.tar" -C "$MOCK_MINIO_SOURCE" .
cp "$TEST_ROOT/humo.sqlite3" "$TEST_ROOT/mock-humo.sqlite3"
ORCHESTRATOR_ROOT="$TEST_ROOT/orchestrator/full"
MOCK_RCLONE_LOG="$TEST_ROOT/rclone.log"
PATH="$MOCK_BIN:$PATH" \
MOCK_MINIO_TAR="$TEST_ROOT/mock-minio.tar" \
MOCK_HUMO_DB="$TEST_ROOT/mock-humo.sqlite3" \
MOCK_RCLONE_LOG="$MOCK_RCLONE_LOG" \
BACKUP_ROOT="$ORCHESTRATOR_ROOT" \
BACKUP_LOCK_FILE="$TEST_ROOT/orchestrator.lock" \
AGE_RECIPIENT=age1fixture \
RCLONE_REMOTE=fixture:drivergo/full \
REQUIRE_OFFSITE_BACKUP=1 \
PRUNE_OFFSITE=0 \
BACKUP_MIN_SNAPSHOTS=1 \
  "$SCRIPT_DIR/backup_all.sh" >/dev/null
mapfile -t orchestrator_snapshots < <(
  find "$ORCHESTRATOR_ROOT" -mindepth 1 -maxdepth 1 -type d -name 'drivergo-*' -print
)
[[ "${#orchestrator_snapshots[@]}" -eq 1 ]]
"$SCRIPT_DIR/verify_snapshot.sh" "${orchestrator_snapshots[0]}" >/dev/null
grep -Fq 'copy ' "$MOCK_RCLONE_LOG"
grep -Fq 'check ' "$MOCK_RCLONE_LOG"
grep -Fq 'rcat ' "$MOCK_RCLONE_LOG"
grep -Fq 'encryption=age' "${orchestrator_snapshots[0]}/manifest.txt"
pass "full encrypted/off-site orchestrator path completes with isolated command fixtures"

FAILURE_ROOT="$TEST_ROOT/failure/full"
expect_failure "component failure removes partial snapshot atomically" env \
  PATH="$MOCK_BIN:$PATH" \
  MOCK_MINIO_TAR="$TEST_ROOT/mock-minio.tar" \
  MOCK_HUMO_DB="$TEST_ROOT/mock-humo.sqlite3" \
  MOCK_FAIL_SERVICE=redis \
  MOCK_AGE_EMPTY_ENVELOPE=1 \
  BACKUP_ROOT="$FAILURE_ROOT" \
  BACKUP_LOCK_FILE="$TEST_ROOT/failure.lock" \
  AGE_RECIPIENT=age1fixture \
  REQUIRE_OFFSITE_BACKUP=0 \
  "$SCRIPT_DIR/backup_all.sh"
[[ -z "$(find "$FAILURE_ROOT" -mindepth 1 -maxdepth 1 -type d -print -quit)" ]]

HELD_ROOT="$TEST_ROOT/held/full"
HELD_LOCK="$TEST_ROOT/held.lock"
exec 8>"$HELD_LOCK"
flock -n 8
expect_failure "concurrent backup lock fails without touching services" env \
  PATH="$MOCK_BIN:$PATH" \
  BACKUP_ROOT="$HELD_ROOT" \
  BACKUP_LOCK_FILE="$HELD_LOCK" \
  AGE_RECIPIENT=age1fixture \
  REQUIRE_OFFSITE_BACKUP=0 \
  "$SCRIPT_DIR/backup_all.sh"
flock -u 8
exec 8>&-

remote_marker_digest="$(sha256sum -- "${orchestrator_snapshots[0]}/SHA256SUMS" | awk '{ print $1 }')"
printf 'snapshot=%s\nsha256sums_sha256=%s\n' \
  "$(basename "${orchestrator_snapshots[0]}")" "$remote_marker_digest" \
  >"${orchestrator_snapshots[0]}/REMOTE_COMPLETE"
"$SCRIPT_DIR/verify_snapshot.sh" "${orchestrator_snapshots[0]}" >/dev/null
pass "downloaded off-site completion marker is tied to checksum manifest"
cp "${orchestrator_snapshots[0]}/REMOTE_COMPLETE" "$TEST_ROOT/REMOTE_COMPLETE.good"
printf 'snapshot=%s\nsha256sums_sha256=%064d\n' \
  "$(basename "${orchestrator_snapshots[0]}")" 0 \
  >"${orchestrator_snapshots[0]}/REMOTE_COMPLETE"
expect_failure "tampered off-site completion marker is rejected" \
  "$SCRIPT_DIR/verify_snapshot.sh" "${orchestrator_snapshots[0]}"
cp "$TEST_ROOT/REMOTE_COMPLETE.good" "${orchestrator_snapshots[0]}/REMOTE_COMPLETE"

printf 'mock age identity\n' >"$TEST_ROOT/age-identity.txt"
mkdir -m 0700 "$TEST_ROOT/reports"
FULL_DRILL_ROOT="$TEST_ROOT/full-restore/drivergo-restore-drill"
FULL_DRILL_REPORT="$TEST_ROOT/reports/full-drill.txt"
PATH="$MOCK_BIN:$PATH" \
MOCK_MINIO_TAR="$TEST_ROOT/mock-minio.tar" \
MOCK_HUMO_DB="$TEST_ROOT/mock-humo.sqlite3" \
RESTORE_DRILL_ACK=avtotest_restore_drill \
AGE_IDENTITY_FILE="$TEST_ROOT/age-identity.txt" \
DRILL_ROOT="$FULL_DRILL_ROOT" \
DRILL_REPORT_FILE="$FULL_DRILL_REPORT" \
  "$SCRIPT_DIR/full_restore_drill.sh" "${orchestrator_snapshots[0]}" >/dev/null
grep -Fq 'status=success' "$FULL_DRILL_REPORT"
grep -Fq 'postgres_target=avtotest_restore_drill' "$FULL_DRILL_REPORT"
grep -Fq 'runtime_restore_drill_requested=0' "$FULL_DRILL_REPORT"
grep -Fq 'redis_mode=offline-rdb-check' "$FULL_DRILL_REPORT"
grep -Fq 'redis_runtime_image=not-applicable' "$FULL_DRILL_REPORT"
grep -Fq 'redis_runtime_ping=not-run' "$FULL_DRILL_REPORT"
grep -Fq 'minio_mode=guarded-scratch-extraction' "$FULL_DRILL_REPORT"
grep -Fq 'minio_runtime_server_image=not-applicable' "$FULL_DRILL_REPORT"
grep -Fq 'minio_runtime_client_image=not-applicable' "$FULL_DRILL_REPORT"
grep -Fq 'minio_runtime_ready=not-run' "$FULL_DRILL_REPORT"
grep -Fq 'humo_integrity_check=ok' "$FULL_DRILL_REPORT"
grep -Fq 'humo_schema_check=ok' "$FULL_DRILL_REPORT"
[[ -z "$(find "$FULL_DRILL_ROOT" -mindepth 1 -print -quit)" ]]
pass "full restore drill validates every component with isolated command fixtures"

CLEANUP_FAIL_BIN="$TEST_ROOT/cleanup-fail-bin"
mkdir -m 0700 "$CLEANUP_FAIL_BIN"
cat >"$CLEANUP_FAIL_BIN/rm" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
for argument in "$@"; do
  if [[ "${MOCK_FAIL_TOP_LEVEL_SCRATCH_REMOVE:-0}" == "1" && \
        "$argument" == */drivergo-restore-drill/drivergo-drill.* ]]; then
    exit 91
  fi
done
exec /usr/bin/rm "$@"
MOCK
chmod 0700 "$CLEANUP_FAIL_BIN/rm"
CLEANUP_FAILURE_ROOT="$TEST_ROOT/cleanup-failure/drivergo-restore-drill"
CLEANUP_FAILURE_REPORT="$TEST_ROOT/reports/cleanup-failure.txt"
expect_failure "decrypted scratch cleanup failure cannot report restore success" env \
  PATH="$CLEANUP_FAIL_BIN:$MOCK_BIN:$PATH" \
  MOCK_FAIL_TOP_LEVEL_SCRATCH_REMOVE=1 \
  MOCK_MINIO_TAR="$TEST_ROOT/mock-minio.tar" \
  MOCK_HUMO_DB="$TEST_ROOT/mock-humo.sqlite3" \
  RESTORE_DRILL_ACK=avtotest_restore_drill \
  AGE_IDENTITY_FILE="$TEST_ROOT/age-identity.txt" \
  DRILL_ROOT="$CLEANUP_FAILURE_ROOT" \
  DRILL_REPORT_FILE="$CLEANUP_FAILURE_REPORT" \
  "$SCRIPT_DIR/full_restore_drill.sh" "${orchestrator_snapshots[0]}"
[[ ! -e "$CLEANUP_FAILURE_REPORT" ]]
find "$CLEANUP_FAILURE_ROOT" -mindepth 1 -maxdepth 1 -type d -name 'drivergo-drill.*' -print -quit | grep -q .
/usr/bin/rm -rf -- "$CLEANUP_FAILURE_ROOT"

RUNTIME_FULL_DRILL_ROOT="$TEST_ROOT/full-runtime-restore/drivergo-restore-drill"
RUNTIME_FULL_DRILL_REPORT="$TEST_ROOT/reports/full-runtime-drill.txt"
RUNTIME_FULL_STATE="$TEST_ROOT/full-runtime-restore/docker-state"
RUNTIME_FULL_LOG="$TEST_ROOT/full-runtime-restore/docker.log"
mkdir -p -m 0700 "$RUNTIME_FULL_STATE"
PATH="$MOCK_BIN:$PATH" \
MOCK_DOCKER_STATE_DIR="$RUNTIME_FULL_STATE" \
MOCK_DOCKER_LOG="$RUNTIME_FULL_LOG" \
MOCK_MINIO_TAR="$TEST_ROOT/mock-minio.tar" \
MOCK_HUMO_DB="$TEST_ROOT/mock-humo.sqlite3" \
RESTORE_DRILL_ACK=avtotest_restore_drill \
AGE_IDENTITY_FILE="$TEST_ROOT/age-identity.txt" \
DRILL_ROOT="$RUNTIME_FULL_DRILL_ROOT" \
DRILL_REPORT_FILE="$RUNTIME_FULL_DRILL_REPORT" \
RUNTIME_RESTORE_DRILL=1 \
REDIS_DRILL_IMAGE="$REDIS_RUNTIME_IMAGE" \
MINIO_DRILL_IMAGE="$MINIO_RUNTIME_IMAGE" \
MINIO_MC_DRILL_IMAGE="$MINIO_MC_RUNTIME_IMAGE" \
RUNTIME_DRILL_TIMEOUT_SECONDS=2 \
  "$SCRIPT_DIR/full_restore_drill.sh" "${orchestrator_snapshots[0]}" >/dev/null
grep -Fq 'status=success' "$RUNTIME_FULL_DRILL_REPORT"
grep -Fq 'runtime_restore_drill_requested=1' "$RUNTIME_FULL_DRILL_REPORT"
grep -Fq 'redis_runtime_restore_drill=1' "$RUNTIME_FULL_DRILL_REPORT"
grep -Fq 'redis_mode=ephemeral-runtime-restore' "$RUNTIME_FULL_DRILL_REPORT"
grep -Fq "redis_runtime_image=$REDIS_RUNTIME_IMAGE" "$RUNTIME_FULL_DRILL_REPORT"
grep -Fq 'redis_runtime_ping=PONG' "$RUNTIME_FULL_DRILL_REPORT"
grep -Fq 'redis_runtime_dbsize=7' "$RUNTIME_FULL_DRILL_REPORT"
grep -Fq 'redis_runtime_cleanup=completed' "$RUNTIME_FULL_DRILL_REPORT"
grep -Fq 'minio_runtime_restore_drill=1' "$RUNTIME_FULL_DRILL_REPORT"
grep -Fq 'minio_mode=ephemeral-runtime-restore' "$RUNTIME_FULL_DRILL_REPORT"
grep -Fq "minio_runtime_server_image=$MINIO_RUNTIME_IMAGE" "$RUNTIME_FULL_DRILL_REPORT"
grep -Fq "minio_runtime_client_image=$MINIO_MC_RUNTIME_IMAGE" "$RUNTIME_FULL_DRILL_REPORT"
grep -Fq 'minio_runtime_ready=ok' "$RUNTIME_FULL_DRILL_REPORT"
grep -Fq 'minio_runtime_bucket_count=2' "$RUNTIME_FULL_DRILL_REPORT"
grep -Fq 'minio_runtime_inventory_entries=1' "$RUNTIME_FULL_DRILL_REPORT"
grep -Fq 'minio_runtime_cleanup=completed' "$RUNTIME_FULL_DRILL_REPORT"
grep -Fq 'humo_integrity_check=ok' "$RUNTIME_FULL_DRILL_REPORT"
grep -Fq 'humo_schema_check=ok' "$RUNTIME_FULL_DRILL_REPORT"
grep -Fq 'humo_pending_count=1' "$RUNTIME_FULL_DRILL_REPORT"
[[ -z "$(find "$RUNTIME_FULL_STATE" -mindepth 1 -print -quit)" ]]
[[ -z "$(find "$RUNTIME_FULL_DRILL_ROOT" -mindepth 1 -print -quit)" ]]
if grep -Fq 'compose exec -T redis' "$RUNTIME_FULL_LOG"; then
  echo "not ok - full runtime drill used the live Redis Compose service" >&2
  exit 1
fi
pass "full runtime restore drill records Redis, MinIO, and Humo evidence with mock isolation"

RETENTION_ROOT="$TEST_ROOT/backup-root/full"
mkdir -p -m 0700 "$RETENTION_ROOT"
for name in drivergo-20000101T000000Z drivergo-20010101T000000Z drivergo-29990101T000000Z; do
  clone_plain_snapshot "$RETENTION_ROOT" "$name"
done
clone_plain_snapshot "$RETENTION_ROOT" drivergo-29980101T000000Z
printf 'bitrot\n' >>"$RETENTION_ROOT/drivergo-29980101T000000Z/postgres.dump"
mkdir -m 0700 "$RETENTION_ROOT/do-not-delete"
mkdir -m 0700 "$RETENTION_ROOT/.partial-drivergo-20000101T000000Z"
mkdir -m 0700 "$RETENTION_ROOT/.partial-drivergo-29990101T000000Z"
BACKUP_ROOT="$RETENTION_ROOT" \
BACKUP_LOCK_FILE="$TEST_ROOT/retention.lock" \
BACKUP_RETENTION_DAYS=1 \
BACKUP_MIN_SNAPSHOTS=1 \
PRUNE_OFFSITE=0 \
  "$SCRIPT_DIR/prune_backups.sh" >/dev/null
[[ ! -e "$RETENTION_ROOT/drivergo-20000101T000000Z" ]]
[[ ! -e "$RETENTION_ROOT/drivergo-20010101T000000Z" ]]
[[ -d "$RETENTION_ROOT/drivergo-29990101T000000Z" ]]
[[ ! -e "$RETENTION_ROOT/drivergo-29980101T000000Z" ]]
[[ -d "$TEST_ROOT/backup-root/quarantine/drivergo-29980101T000000Z-invalid" ]]
[[ -d "$RETENTION_ROOT/do-not-delete" ]]
[[ ! -e "$RETENTION_ROOT/.partial-drivergo-20000101T000000Z" ]]
[[ -d "$RETENTION_ROOT/.partial-drivergo-29990101T000000Z" ]]
pass "retention quarantines checksum-invalid snapshots and preserves a verified minimum"

REMOTE_RETENTION_ROOT="$TEST_ROOT/remote-retention/full"
mkdir -p -m 0700 "$REMOTE_RETENTION_ROOT"
REMOTE_FIXTURE_ROOT="$TEST_ROOT/remote-retention/provider"
mkdir -p -m 0700 "$REMOTE_FIXTURE_ROOT"
for name in \
  drivergo-20000101T000000Z drivergo-20010101T000000Z \
  drivergo-29980101T000000Z drivergo-29990101T000000Z; do
  clone_plain_snapshot "$REMOTE_FIXTURE_ROOT" "$name"
  mark_remote_complete "$REMOTE_FIXTURE_ROOT/$name"
done
# Newest is structurally complete but corrupt; retention must fall back to the
# next newest fully hashed snapshot for its protected minimum.
printf 'remote-bitrot\n' >>"$REMOTE_FIXTURE_ROOT/drivergo-29990101T000000Z/minio-data.tar"
for name in drivergo-19990101T000000Z drivergo-29970101T000000Z; do
  clone_plain_snapshot "$REMOTE_FIXTURE_ROOT" "$name"
  # No REMOTE_COMPLETE: these are interrupted/incomplete uploads.
done
mkdir -m 0700 "$REMOTE_FIXTURE_ROOT/unsafe-directory"
MOCK_RCLONE_LOG="$TEST_ROOT/rclone-retention.log"
PATH="$MOCK_BIN:$PATH" \
MOCK_RCLONE_LOG="$MOCK_RCLONE_LOG" \
MOCK_RCLONE_LIST_MODE=retention \
MOCK_RCLONE_REMOTE_ROOT="$REMOTE_FIXTURE_ROOT" \
BACKUP_ROOT="$REMOTE_RETENTION_ROOT" \
BACKUP_LOCK_FILE="$TEST_ROOT/remote-retention.lock" \
BACKUP_MIN_SNAPSHOTS=1 \
OFFSITE_RETENTION_DAYS=1 \
RCLONE_REMOTE=fixture:drivergo/full \
PRUNE_OFFSITE=1 \
  "$SCRIPT_DIR/prune_backups.sh" >/dev/null
grep -Fq 'purge fixture:drivergo/full/drivergo-20000101T000000Z' "$MOCK_RCLONE_LOG"
grep -Fq 'purge fixture:drivergo/full/drivergo-20010101T000000Z' "$MOCK_RCLONE_LOG"
grep -Fq 'purge fixture:drivergo/full/drivergo-19990101T000000Z' "$MOCK_RCLONE_LOG"
! grep -Fq 'purge fixture:drivergo/full/drivergo-29980101T000000Z' "$MOCK_RCLONE_LOG"
! grep -Fq 'purge fixture:drivergo/full/drivergo-29990101T000000Z' "$MOCK_RCLONE_LOG"
grep -Fq 'hashsum SHA-256 fixture:drivergo/full/drivergo-29990101T000000Z --download' "$MOCK_RCLONE_LOG"
grep -Fq 'hashsum SHA-256 fixture:drivergo/full/drivergo-29980101T000000Z --download' "$MOCK_RCLONE_LOG"
pass "off-site retention fully hashes a valid minimum and rejects remote bitrot"

TRANSIENT_REMOTE_ROOT="$TEST_ROOT/remote-transient/provider"
TRANSIENT_BACKUP_ROOT="$TEST_ROOT/remote-transient/local"
mkdir -p -m 0700 "$TRANSIENT_REMOTE_ROOT" "$TRANSIENT_BACKUP_ROOT"
for name in drivergo-20020101T000000Z drivergo-20030101T000000Z; do
  clone_plain_snapshot "$TRANSIENT_REMOTE_ROOT" "$name"
  mark_remote_complete "$TRANSIENT_REMOTE_ROOT/$name"
done
TRANSIENT_LOG="$TEST_ROOT/rclone-transient.log"
PATH="$MOCK_BIN:$PATH" \
MOCK_RCLONE_LOG="$TRANSIENT_LOG" \
MOCK_RCLONE_LIST_MODE=retention \
MOCK_RCLONE_REMOTE_ROOT="$TRANSIENT_REMOTE_ROOT" \
MOCK_RCLONE_FAIL_HASH_NAME=drivergo-20030101T000000Z \
BACKUP_ROOT="$TRANSIENT_BACKUP_ROOT" \
BACKUP_LOCK_FILE="$TEST_ROOT/remote-transient.lock" \
BACKUP_MIN_SNAPSHOTS=1 \
OFFSITE_RETENTION_DAYS=1 \
RCLONE_REMOTE=fixture:drivergo/full \
PRUNE_OFFSITE=1 \
  "$SCRIPT_DIR/prune_backups.sh" >/dev/null 2>&1
! grep -Fq 'purge fixture:drivergo/full/drivergo-20030101T000000Z' "$TRANSIENT_LOG"
grep -Fq 'hashsum SHA-256 fixture:drivergo/full/drivergo-20020101T000000Z --download' "$TRANSIENT_LOG"
pass "off-site retention never purges a snapshot after an uncertain verification failure"

HOMEPC_SERVICE="$ROOT/deploy/systemd/drivergo-backup-homepc.service"
HOMEPC_TIMER="$ROOT/deploy/systemd/drivergo-backup-homepc.timer"
grep -Fq 'Environment="REQUIRE_OFFSITE_BACKUP=0"' "$HOMEPC_SERVICE"
grep -Fq 'Environment="PRUNE_OFFSITE=0"' "$HOMEPC_SERVICE"
grep -Fq 'Conflicts=drivergo-backup.service' "$HOMEPC_SERVICE"
grep -Fq 'Unit=drivergo-backup-homepc.service' "$HOMEPC_TIMER"
grep -Fq 'Persistent=true' "$HOMEPC_TIMER"
pass "interim home-PC VPS backup unit is local-only and conflicts with cloud unit"

OFFHOST_USER_SERVICE="$SCRIPT_DIR/systemd/drivergo-offhost-pull.service"
OFFHOST_USER_TIMER="$SCRIPT_DIR/systemd/drivergo-offhost-pull.timer"
grep -Fq 'EnvironmentFile=%h/.config/drivergo/offhost-pull.env' "$OFFHOST_USER_SERVICE"
grep -Fq 'OFFHOST_PULL_SCRIPT' "$OFFHOST_USER_SERVICE"
grep -Fq 'OnUnitActiveSec=30min' "$OFFHOST_USER_TIMER"
grep -Fq 'Persistent=true' "$OFFHOST_USER_TIMER"
pass "home PC pull user unit reads env and retries when online"

PULL_REMOTE="$TEST_ROOT/pull-remote"
PULL_LOCAL="$TEST_ROOT/pull-local"
PULL_BIN="$TEST_ROOT/pull-bin"
mkdir -p -m 0700 "$PULL_REMOTE" "$PULL_LOCAL" "$PULL_BIN"
clone_plain_snapshot "$PULL_REMOTE" "drivergo-20200103T120000Z"
clone_plain_snapshot "$PULL_REMOTE" "drivergo-20200104T120000Z"
# Incomplete remote must be ignored by the listing contract.
mkdir -m 0700 "$PULL_REMOTE/drivergo-20200105T120000Z"
printf 'incomplete\n' >"$PULL_REMOTE/drivergo-20200105T120000Z/manifest.txt"

cat >"$PULL_BIN/ssh" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"${MOCK_SSH_LOG:?}"
export REMOTE_BACKUP_ROOT="${MOCK_REMOTE_ROOT:?}"
exec bash -s
MOCK
cat >"$PULL_BIN/rsync" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"${MOCK_RSYNC_LOG:?}"
dest="${!#}"
src=""
for arg in "$@"; do
  case "$arg" in
    *:*) src="${arg#*:}" ;;
  esac
done
src="${src%/}"
name="$(basename "$src")"
[[ "$name" =~ ^drivergo-[0-9]{8}T[0-9]{6}Z$ ]] || {
  echo "mock rsync: bad snapshot name: ${name}" >&2
  exit 1
}
from="${MOCK_REMOTE_ROOT:?}/${name}"
[[ -d "$from" ]] || {
  echo "mock rsync: missing ${from}" >&2
  exit 1
}
to="${dest%/}"
mkdir -p -m 0700 "$to"
cp -a -- "$from"/. "$to"/
MOCK
chmod 0700 "$PULL_BIN/ssh" "$PULL_BIN/rsync"

MOCK_SSH_LOG="$TEST_ROOT/pull-ssh.log"
MOCK_RSYNC_LOG="$TEST_ROOT/pull-rsync.log"
: >"$MOCK_SSH_LOG"
: >"$MOCK_RSYNC_LOG"
PATH="$PULL_BIN:$PATH" \
MOCK_SSH_LOG="$MOCK_SSH_LOG" \
MOCK_RSYNC_LOG="$MOCK_RSYNC_LOG" \
MOCK_REMOTE_ROOT="$PULL_REMOTE" \
OFFHOST_SSH_HOST=root@192.0.2.10 \
REMOTE_BACKUP_ROOT=/var/backups/drivergo/full \
LOCAL_OFFHOST_ROOT="$PULL_LOCAL" \
OFFHOST_LOCK_FILE="$TEST_ROOT/pull.lock" \
OFFHOST_RETENTION_DAYS=36500 \
OFFHOST_MIN_SNAPSHOTS=1 \
  "$SCRIPT_DIR/pull_offhost.sh" >/dev/null
[[ -d "$PULL_LOCAL/drivergo-20200103T120000Z" ]]
[[ -d "$PULL_LOCAL/drivergo-20200104T120000Z" ]]
[[ ! -e "$PULL_LOCAL/drivergo-20200105T120000Z" ]]
"$SCRIPT_DIR/verify_snapshot.sh" "$PULL_LOCAL/drivergo-20200103T120000Z" >/dev/null
"$SCRIPT_DIR/verify_snapshot.sh" "$PULL_LOCAL/drivergo-20200104T120000Z" >/dev/null
pass "pull_offhost copies only complete remote snapshots and verifies them"

PATH="$PULL_BIN:$PATH" \
MOCK_SSH_LOG="$MOCK_SSH_LOG" \
MOCK_RSYNC_LOG="$MOCK_RSYNC_LOG" \
MOCK_REMOTE_ROOT="$PULL_REMOTE" \
OFFHOST_SSH_HOST=root@192.0.2.10 \
REMOTE_BACKUP_ROOT=/var/backups/drivergo/full \
LOCAL_OFFHOST_ROOT="$PULL_LOCAL" \
OFFHOST_LOCK_FILE="$TEST_ROOT/pull.lock" \
OFFHOST_RETENTION_DAYS=36500 \
OFFHOST_MIN_SNAPSHOTS=1 \
  "$SCRIPT_DIR/pull_offhost.sh" >/dev/null
pass "pull_offhost is idempotent for already-verified local snapshots"

expect_failure "pull_offhost rejects unsafe SSH host" \
  env OFFHOST_SSH_HOST='root@host;evil' \
  REMOTE_BACKUP_ROOT=/var/backups/drivergo/full \
  LOCAL_OFFHOST_ROOT="$PULL_LOCAL" \
  OFFHOST_LOCK_FILE="$TEST_ROOT/pull-bad.lock" \
  "$SCRIPT_DIR/pull_offhost.sh"

expect_failure "pull_offhost rejects relative remote root" \
  env OFFHOST_SSH_HOST=root@192.0.2.10 \
  REMOTE_BACKUP_ROOT=var/backups/drivergo/full \
  LOCAL_OFFHOST_ROOT="$PULL_LOCAL" \
  OFFHOST_LOCK_FILE="$TEST_ROOT/pull-bad2.lock" \
  "$SCRIPT_DIR/pull_offhost.sh"

PITR_ARCHIVE_SCRIPT="$SCRIPT_DIR/pitr_archive_wal.sh"
PITR_VERIFY_SCRIPT="$SCRIPT_DIR/pitr_verify_wal.sh"
PITR_DRILL_SCRIPT="$SCRIPT_DIR/pitr_restore_to_time_drill.sh"
for script in "$PITR_ARCHIVE_SCRIPT" "$PITR_VERIFY_SCRIPT" "$PITR_DRILL_SCRIPT"; do
  [[ -x "$script" ]]
  bash -n "$script"
done
grep -Fq 'PITR_ARCHIVE_ENABLED="${PITR_ARCHIVE_ENABLED:-0}"' "$PITR_ARCHIVE_SCRIPT"
grep -Fq 'PITR_ARCHIVE_ENABLED=1' "$PITR_ARCHIVE_SCRIPT"
grep -Fq 'PITR_WAL_ALLOW_PLAINTEXT="${PITR_WAL_ALLOW_PLAINTEXT:-0}"' "$PITR_ARCHIVE_SCRIPT"
grep -Fq 'PITR_RESTORE_DRILL_ACK=drivergo_pitr_restore_drill' "$PITR_DRILL_SCRIPT"
grep -Fq 'drivergo-pitr-restore-drill' "$PITR_DRILL_SCRIPT"
if grep -Eq '(docker compose|archive_mode[[:space:]]*=|postgresql\.conf)' \
  "$PITR_ARCHIVE_SCRIPT" "$PITR_VERIFY_SCRIPT" "$PITR_DRILL_SCRIPT"; then
  echo "not ok - PITR scripts must not edit PostgreSQL config or invoke Compose" >&2
  exit 1
fi
grep -Fq 'PITR scaffolding exists, but it is **OFF by default**.' "$BACKUP_RUNBOOK"
grep -Fq 'No live PostgreSQL configuration, VPS service, systemd unit, or app' \
  "$ROOT/DEVOPS-REMEDIATION-HANDOFF.md"
pass "PITR scaffolding is opt-in, scratch-only, and leaves live configuration untouched"

WAL_SOURCE="$TEST_ROOT/000000010000000000000001"
WAL_ARCHIVE="$TEST_ROOT/drivergo-pitr-wal"
printf 'WAL-fixture\n' >"$WAL_SOURCE"
expect_failure "WAL archive helper is disabled by default" \
  env PITR_WAL_ARCHIVE_ROOT="$WAL_ARCHIVE" \
  "$PITR_ARCHIVE_SCRIPT" "$WAL_SOURCE" 000000010000000000000001
expect_failure "WAL archive requires encryption unless plaintext is explicitly approved" env \
  PITR_ARCHIVE_ENABLED=1 \
  PITR_WAL_ARCHIVE_ROOT="$WAL_ARCHIVE" \
  "$PITR_ARCHIVE_SCRIPT" "$WAL_SOURCE" 000000010000000000000001
PITR_ARCHIVE_ENABLED=1 \
PITR_WAL_ALLOW_PLAINTEXT=1 \
PITR_WAL_ARCHIVE_ROOT="$WAL_ARCHIVE" \
  "$PITR_ARCHIVE_SCRIPT" "$WAL_SOURCE" 000000010000000000000001 >/dev/null
"$PITR_VERIFY_SCRIPT" "$WAL_ARCHIVE" >/dev/null
printf 'WAL-encrypted-fixture\n' >"$TEST_ROOT/000000010000000000000002"
PATH="$MOCK_BIN:$PATH" \
PITR_ARCHIVE_ENABLED=1 \
PITR_WAL_ARCHIVE_ROOT="$WAL_ARCHIVE" \
AGE_RECIPIENT=age1fixture \
  "$PITR_ARCHIVE_SCRIPT" "$TEST_ROOT/000000010000000000000002" \
  000000010000000000000002 >/dev/null
"$PITR_VERIFY_SCRIPT" "$WAL_ARCHIVE" 000000010000000000000002 >/dev/null
pass "WAL archive fixture is sealed, optionally encrypted, and verified without Docker"
printf 'tamper\n' >>"$WAL_ARCHIVE/000000010000000000000001"
expect_failure "WAL verifier rejects tampered archive data" \
  "$PITR_VERIFY_SCRIPT" "$WAL_ARCHIVE"

expect_failure "PITR drill requires its scratch-only acknowledgement" \
  "$PITR_DRILL_SCRIPT" "$TEST_ROOT/missing-base" 2026-08-10T00:00:00Z
expect_failure "PITR drill rejects an arbitrary target timestamp before Docker" env \
  PITR_RESTORE_DRILL_ACK=drivergo_pitr_restore_drill \
  "$PITR_DRILL_SCRIPT" "$TEST_ROOT/missing-base" '2026-08-10 00:00:00'

echo "1..${pass_count}"
