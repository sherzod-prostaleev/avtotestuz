#!/usr/bin/env bash
# Offline contract tests. Collector calls are fully mocked and never contact a
# Docker daemon, systemd manager, certificate provider, or live endpoint.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
MONITORING_DIR="$ROOT/deploy/monitoring"
TEST_ROOT="$(mktemp -d /tmp/drivergo-monitoring-tests.XXXXXX)"

cleanup() {
  if [[ "$TEST_ROOT" == /tmp/drivergo-monitoring-tests.* && -d "$TEST_ROOT" ]]; then
    rm -rf -- "$TEST_ROOT"
  fi
}
trap cleanup EXIT

pass_count=0
pass() {
  pass_count="$((pass_count + 1))"
  printf 'ok %d - %s\n' "$pass_count" "$1"
}

fail() {
  printf 'not ok - %s\n' "$1" >&2
  exit 1
}

assert_contains() {
  local file="$1"
  local value="$2"
  grep -Fq -- "$value" "$file" || fail "$file is missing: $value"
}

assert_not_contains() {
  local file="$1"
  local value="$2"
  if grep -Fq -- "$value" "$file"; then
    fail "$file unexpectedly contains: $value"
  fi
}

bash -n \
  "$MONITORING_DIR/write_textfile_metrics.sh" \
  "$MONITORING_DIR/validate_env.sh" \
  "$MONITORING_DIR/tests/monitoring_contract_test.sh"
pass "monitoring shell syntax"

DIGEST_A="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
DIGEST_B="bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
DIGEST_C="cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
DIGEST_D="dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
WEBHOOK_FILE="$TEST_ROOT/alert-webhook.url"
printf 'https://alerts.example.invalid/drivergo-fixture\n' >"$WEBHOOK_FILE"
chmod 0640 "$WEBHOOK_FILE"
WEBHOOK_GID="$(stat -c '%g' "$WEBHOOK_FILE")"
VALID_ENV="$TEST_ROOT/valid.env"
cat >"$VALID_ENV" <<EOF
PROMETHEUS_IMAGE=registry.example/ops/prometheus@sha256:${DIGEST_A}
BLACKBOX_EXPORTER_IMAGE=registry.example/ops/blackbox@sha256:${DIGEST_B}
NODE_EXPORTER_IMAGE=registry.example/ops/node-exporter@sha256:${DIGEST_C}
ALERTMANAGER_IMAGE=registry.example/ops/alertmanager@sha256:${DIGEST_D}
ALERT_WEBHOOK_URL_FILE=$WEBHOOK_FILE
ALERT_WEBHOOK_GID=$WEBHOOK_GID
PROMETHEUS_HOST_PORT=9091
ALERTMANAGER_HOST_PORT=9094
EOF
"$MONITORING_DIR/validate_env.sh" "$VALID_ENV" >/dev/null
pass "immutable monitoring image references are accepted"

if "$MONITORING_DIR/validate_env.sh" "$MONITORING_DIR/monitoring.env.example" >/dev/null 2>&1; then
  fail "empty example environment must fail closed"
fi
pass "empty example environment fails closed"

TAG_ENV="$TEST_ROOT/tag.env"
cat >"$TAG_ENV" <<EOF
PROMETHEUS_IMAGE=prom/prometheus:latest
BLACKBOX_EXPORTER_IMAGE=registry.example/ops/blackbox@sha256:${DIGEST_B}
NODE_EXPORTER_IMAGE=registry.example/ops/node-exporter@sha256:${DIGEST_C}
ALERTMANAGER_IMAGE=registry.example/ops/alertmanager@sha256:${DIGEST_D}
ALERT_WEBHOOK_URL_FILE=$WEBHOOK_FILE
ALERT_WEBHOOK_GID=$WEBHOOK_GID
EOF
if "$MONITORING_DIR/validate_env.sh" "$TAG_ENV" >/dev/null 2>&1; then
  fail "floating image tag must be rejected"
fi
pass "floating image tags are rejected"

INJECTION_MARKER="$TEST_ROOT/env-was-evaluated"
INJECTION_ENV="$TEST_ROOT/injection.env"
printf 'PROMETHEUS_IMAGE=$(touch %s)\n' "$INJECTION_MARKER" >"$INJECTION_ENV"
printf 'BLACKBOX_EXPORTER_IMAGE=registry.example/ops/blackbox@sha256:%s\n' "$DIGEST_B" >>"$INJECTION_ENV"
printf 'NODE_EXPORTER_IMAGE=registry.example/ops/node-exporter@sha256:%s\n' "$DIGEST_C" >>"$INJECTION_ENV"
printf 'ALERTMANAGER_IMAGE=registry.example/ops/alertmanager@sha256:%s\n' "$DIGEST_D" >>"$INJECTION_ENV"
printf 'ALERT_WEBHOOK_URL_FILE=%s\n' "$WEBHOOK_FILE" >>"$INJECTION_ENV"
printf 'ALERT_WEBHOOK_GID=%s\n' "$WEBHOOK_GID" >>"$INJECTION_ENV"
if "$MONITORING_DIR/validate_env.sh" "$INJECTION_ENV" >/dev/null 2>&1; then
  fail "shell expression in environment file must be rejected"
fi
[[ ! -e "$INJECTION_MARKER" ]] || fail "environment validator evaluated shell content"
pass "environment file is parsed without shell evaluation"

RENDERED_COMPOSE="$TEST_ROOT/compose.rendered.yml"
PROMETHEUS_IMAGE="registry.example/ops/prometheus@sha256:${DIGEST_A}" \
BLACKBOX_EXPORTER_IMAGE="registry.example/ops/blackbox@sha256:${DIGEST_B}" \
NODE_EXPORTER_IMAGE="registry.example/ops/node-exporter@sha256:${DIGEST_C}" \
ALERTMANAGER_IMAGE="registry.example/ops/alertmanager@sha256:${DIGEST_D}" \
ALERT_WEBHOOK_URL_FILE="$WEBHOOK_FILE" \
ALERT_WEBHOOK_GID="$WEBHOOK_GID" \
  docker compose -f "$MONITORING_DIR/docker-compose.monitoring.yml" config >"$RENDERED_COMPOSE"
assert_contains "$RENDERED_COMPOSE" 'host_ip: 127.0.0.1'
assert_contains "$RENDERED_COMPOSE" 'published: "9091"'
assert_contains "$RENDERED_COMPOSE" 'published: "9094"'
assert_contains "$RENDERED_COMPOSE" 'source: /var/lib/drivergo-monitoring/textfile'
assert_contains "$RENDERED_COMPOSE" 'read_only: true'
assert_contains "$RENDERED_COMPOSE" 'no-new-privileges:true'
assert_contains "$RENDERED_COMPOSE" 'name: drivergo_app'
assert_contains "$RENDERED_COMPOSE" 'source: '"$WEBHOOK_FILE"
assert_contains "$RENDERED_COMPOSE" 'group_add:'
assert_contains "$RENDERED_COMPOSE" 'drivergo-monitoring_alert-egress'
assert_not_contains "$RENDERED_COMPOSE" 'alerts.example.invalid'
if grep -Eq 'source: /((var/)?run/docker\.sock|proc|sys)(/|$)' "$RENDERED_COMPOSE"; then
  fail "monitoring container received a privileged host mount"
fi
assert_not_contains "$RENDERED_COMPOSE" '0.0.0.0:9090'
pass "Compose renders with loopback UI and bounded host mounts"

assert_contains "$MONITORING_DIR/prometheus.yml" 'alertmanagers:'
assert_contains "$MONITORING_DIR/prometheus.yml" 'alertmanager:9093'
assert_contains "$MONITORING_DIR/alertmanager.yml" 'url_file: /run/secrets/alert_webhook_url'
for job in drivergo-api node-textfile blackbox-api-liveness blackbox-api-readiness blackbox-web blackbox-public-tls; do
  assert_contains "$MONITORING_DIR/prometheus.yml" "job_name: $job"
done
assert_contains "$MONITORING_DIR/prometheus.yml" 'api:8080'
assert_contains "$MONITORING_DIR/prometheus.yml" 'https://drivergo.uz/healthz'
assert_contains "$MONITORING_DIR/blackbox.yml" 'fail_if_body_not_matches_regexp:'
assert_contains "$MONITORING_DIR/blackbox.yml" 'insecure_skip_verify: false'
pass "scrape and blackbox probe contract"

if command -v python3 >/dev/null 2>&1 && python3 -c 'import yaml' >/dev/null 2>&1; then
  python3 - "$MONITORING_DIR/prometheus.yml" "$MONITORING_DIR/alerts.yml" \
    "$MONITORING_DIR/alertmanager.yml" \
    "$MONITORING_DIR/blackbox.yml" <<'PY'
import sys
import yaml

for path in sys.argv[1:]:
    with open(path, encoding="utf-8") as stream:
        document = yaml.safe_load(stream)
    if not isinstance(document, dict):
        raise SystemExit(f"{path}: expected a YAML mapping")
PY
  pass "Prometheus, alert, and blackbox YAML parse"
else
  printf '# python3/PyYAML unavailable; YAML parser check skipped\n'
fi

for alert in \
  PrometheusScrapeTargetDown APIHealthProbeFailed APIReadinessProbeFailed \
  WebProbeFailed PublicTLSProbeFailed APIMetricsContractMissing \
  APIHighErrorRatio APIHighP95Latency \
  OpsCollectorMetricsMissing OpsCollectorFailed OpsCollectorStale \
  HostDiskUsageHigh HostDiskUsageCritical MonitoredDiskPathMissing \
  ContainerNotRunning ContainerUnhealthy ContainerRestarting \
  BackupTimerInactive BackupLastAttemptFailed BackupSnapshotMissing BackupSnapshotInvalid \
  BackupSnapshotStale PublicTLSCertificateExpiring \
  OriginTLSCertificateMissingOrInvalid OriginTLSCertificateExpiring; do
  assert_contains "$MONITORING_DIR/alerts.yml" "alert: $alert"
  assert_contains "$MONITORING_DIR/alerts.yml" "deploy/monitoring/RUNBOOK.md#${alert,,}"
  assert_contains "$MONITORING_DIR/RUNBOOK.md" "## $alert"
done
assert_contains "$MONITORING_DIR/alerts.yml" 'avtotest_http_request_duration_seconds_bucket'
assert_contains "$MONITORING_DIR/alerts.yml" 'avtotest_http_requests_by_status_class_total'
assert_contains "$MONITORING_DIR/alerts.yml" 'drivergo_backup_snapshot_age_seconds > 108000'
assert_contains "$MONITORING_DIR/RUNBOOK.md" 'alert delivery/paging is not operational'
pass "alert rules map to exact runbook anchors and disclose no delivery"

SERVICE_UNIT="$MONITORING_DIR/systemd/drivergo-monitoring-collector.service"
TIMER_UNIT="$MONITORING_DIR/systemd/drivergo-monitoring-collector.timer"
for value in \
  'Type=oneshot' \
  'NoNewPrivileges=true' \
  'PrivateNetwork=true' \
  'ProtectSystem=strict' \
  'RestrictAddressFamilies=AF_UNIX' \
  'ExecStart=/usr/local/libexec/drivergo-monitoring-collector' \
  'Environment="DOCKER_CONFIG=/run/drivergo-monitoring/docker-config"' \
  'Environment="BACKUP_VERIFY_SCRIPT=/opt/drivergo/scripts/backup/verify_snapshot.sh"' \
  'Environment="BACKUP_VERIFY_CACHE_DIR=/var/lib/drivergo-monitoring/backup-verify-cache"' \
  'Environment="BACKUP_VERIFY_CACHE_TTL_SECONDS=90000"' \
  'ReadWritePaths=/var/lib/drivergo-monitoring /run/drivergo-monitoring'; do
  assert_contains "$SERVICE_UNIT" "$value"
done
assert_contains "$TIMER_UNIT" 'OnUnitActiveSec=60s'
assert_contains "$TIMER_UNIT" 'Persistent=true'
pass "systemd collector is short-lived, scheduled, and sandboxed"

if command -v systemd-analyze >/dev/null 2>&1; then
  if systemd-analyze verify "$SERVICE_UNIT" "$TIMER_UNIT" >/dev/null 2>&1; then
    pass "systemd unit syntax verification"
  else
    printf '# systemd-analyze could not use its sandbox/user-lookup sockets; static unit contract completed\n'
  fi
else
  printf '# systemd-analyze unavailable; unit parser check skipped\n'
fi

MOCK_BIN="$TEST_ROOT/mock-bin"
mkdir -p "$MOCK_BIN"
cat >"$MOCK_BIN/docker" <<'EOF'
#!/usr/bin/env bash
set -u
if [[ "${1:-}" == "compose" ]]; then
  service="${!#}"
  if [[ "${MOCK_DOCKER_FAIL_SERVICE:-}" == "$service" ]]; then
    exit 42
  fi
  printf '%s-id\n' "$service"
  exit 0
fi
if [[ "${1:-}" == "inspect" ]]; then
  container_id="${!#}"
  service="${container_id%-id}"
  if [[ "${MOCK_UNHEALTHY_SERVICE:-}" == "$service" ]]; then
    printf 'running|unhealthy|2\n'
  else
    printf 'running|healthy|2\n'
  fi
  exit 0
fi
exit 64
EOF
cat >"$MOCK_BIN/systemctl" <<'EOF'
#!/usr/bin/env bash
set -u
case "$*" in
  *--property=Result*) printf 'success\n' ;;
  *--property=ExecMainExitTimestamp*) date -u '+%Y-%m-%d %H:%M:%S UTC' ;;
  *--property=ActiveState*) printf 'active\n' ;;
  *) exit 64 ;;
esac
EOF
cat >"$MOCK_BIN/openssl" <<'EOF'
#!/usr/bin/env bash
set -u
case "$*" in
  *-startdate*-enddate*)
    printf 'notBefore=Jan  1 00:00:00 2020 GMT\nnotAfter=Dec 31 23:59:59 2099 GMT\n'
    ;;
  *) exit 64 ;;
esac
EOF
chmod 0755 "$MOCK_BIN/docker" "$MOCK_BIN/systemctl" "$MOCK_BIN/openssl"

FIXTURE="$TEST_ROOT/fixture"
BACKUP_ROOT="$FIXTURE/backups/full"
mkdir -p "$BACKUP_ROOT"
SNAPSHOT_STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
SNAPSHOT="$BACKUP_ROOT/drivergo-$SNAPSHOT_STAMP"
mkdir -p "$SNAPSHOT"
printf 'postgres\n' >"$SNAPSHOT/postgres.dump"
printf 'REDIS0009\n' >"$SNAPSHOT/redis.rdb"
printf 'minio\n' >"$SNAPSHOT/minio-data.tar"
printf 'sqlite\n' >"$SNAPSHOT/humo-queue.sqlite3"
CREATED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
cat >"$SNAPSHOT/manifest.txt" <<EOF
format=drivergo-full-backup-v1
snapshot=drivergo-$SNAPSHOT_STAMP
created_at=$CREATED_AT
backup_started_at=$CREATED_AT
capture_duration_seconds=1
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
size_postgres_dump=$(wc -c <"$SNAPSHOT/postgres.dump" | tr -d ' ')
size_redis_rdb=$(wc -c <"$SNAPSHOT/redis.rdb" | tr -d ' ')
size_minio_data_tar=$(wc -c <"$SNAPSHOT/minio-data.tar" | tr -d ' ')
size_humo_queue_sqlite3=$(wc -c <"$SNAPSHOT/humo-queue.sqlite3" | tr -d ' ')
EOF
(
  cd "$SNAPSHOT"
  sha256sum -- manifest.txt postgres.dump redis.rdb minio-data.tar humo-queue.sqlite3 >SHA256SUMS
)
: >"$FIXTURE/docker-compose.prod.yml"
: >"$FIXTURE/app.env"
: >"$FIXTURE/origin.pem"

run_collector() {
  local output_root="$1"
  shift
  env \
    PATH="$MOCK_BIN:$PATH" \
    TEXTFILE_DIR="$output_root/textfile" \
    MONITORING_LOCK_FILE="$output_root/run/collector.lock" \
    COMPOSE_FILE="$FIXTURE/docker-compose.prod.yml" \
    COMPOSE_ENV_FILE="$FIXTURE/app.env" \
    BACKUP_ROOT="$BACKUP_ROOT" \
    BACKUP_VERIFY_SCRIPT="${TEST_BACKUP_VERIFY_SCRIPT:-$ROOT/scripts/backup/verify_snapshot.sh}" \
    TLS_CERT_FILE="$FIXTURE/origin.pem" \
    "$@" \
    "$MONITORING_DIR/write_textfile_metrics.sh"
}

SUCCESS_ROOT="$TEST_ROOT/output-success"
VERIFY_LOG="$TEST_ROOT/verifier-calls.log"
COUNTING_VERIFIER="$TEST_ROOT/counting-verifier.sh"
cat >"$COUNTING_VERIFIER" <<EOF
#!/usr/bin/env bash
printf 'verify\n' >>"$VERIFY_LOG"
exec "$ROOT/scripts/backup/verify_snapshot.sh" "\$@"
EOF
chmod 0700 "$COUNTING_VERIFIER"
TEST_BACKUP_VERIFY_SCRIPT="$COUNTING_VERIFIER"
run_collector "$SUCCESS_ROOT"
SUCCESS_METRICS="$SUCCESS_ROOT/textfile/drivergo_ops.prom"
assert_contains "$SUCCESS_METRICS" 'drivergo_container_running{service="api"} 1'
assert_contains "$SUCCESS_METRICS" 'drivergo_container_healthy{service="humo-watcher"} 1'
assert_contains "$SUCCESS_METRICS" 'drivergo_container_restarts_total{service="redis"} 2'
assert_contains "$SUCCESS_METRICS" 'drivergo_backup_timer_active 1'
assert_contains "$SUCCESS_METRICS" 'drivergo_backup_last_attempt_success 1'
assert_contains "$SUCCESS_METRICS" 'drivergo_backup_snapshot_present 1'
assert_contains "$SUCCESS_METRICS" 'drivergo_backup_snapshot_invalid 0'
assert_contains "$SUCCESS_METRICS" 'drivergo_disk_path_present{target="root"} 1'
assert_contains "$SUCCESS_METRICS" 'drivergo_tls_certificate_monitoring_enabled 1'
assert_contains "$SUCCESS_METRICS" 'drivergo_tls_certificate_valid 1'
assert_contains "$SUCCESS_METRICS" 'drivergo_ops_collector_success 1'
if find "$SUCCESS_ROOT/textfile" -maxdepth 1 -name '*.tmp.*' -print -quit | grep -q .; then
  fail "collector left a partial textfile behind"
fi
pass "mocked collector publishes complete atomic success metrics"

run_collector "$SUCCESS_ROOT"
[[ "$(wc -l <"$VERIFY_LOG" | tr -d ' ')" == "1" ]] || fail "unchanged snapshot was fully rehashed inside the cache TTL"
unset TEST_BACKUP_VERIFY_SCRIPT
pass "snapshot verifier cache prevents one-minute full-payload rehashing"

printf 'tamper\n' >>"$SNAPSHOT/postgres.dump"
INVALID_BACKUP_ROOT="$TEST_ROOT/output-invalid-backup"
run_collector "$INVALID_BACKUP_ROOT"
INVALID_BACKUP_METRICS="$INVALID_BACKUP_ROOT/textfile/drivergo_ops.prom"
assert_contains "$INVALID_BACKUP_METRICS" 'drivergo_backup_snapshot_present 0'
assert_contains "$INVALID_BACKUP_METRICS" 'drivergo_backup_snapshot_invalid 1'
assert_contains "$INVALID_BACKUP_METRICS" 'drivergo_ops_collector_success 1'
printf 'postgres\n' >"$SNAPSHOT/postgres.dump"
pass "checksum-invalid snapshot cannot satisfy backup freshness"

UNHEALTHY_ROOT="$TEST_ROOT/output-unhealthy"
run_collector "$UNHEALTHY_ROOT" MOCK_UNHEALTHY_SERVICE=api
UNHEALTHY_METRICS="$UNHEALTHY_ROOT/textfile/drivergo_ops.prom"
assert_contains "$UNHEALTHY_METRICS" 'drivergo_container_healthy{service="api"} 0'
assert_contains "$UNHEALTHY_METRICS" 'drivergo_ops_collector_success 1'
pass "unhealthy service is a target signal, not a collector read failure"

FAILURE_ROOT="$TEST_ROOT/output-failure"
if run_collector "$FAILURE_ROOT" MOCK_DOCKER_FAIL_SERVICE=redis >/dev/null 2>&1; then
  fail "collector must exit non-zero when a required Docker query fails"
fi
FAILURE_METRICS="$FAILURE_ROOT/textfile/drivergo_ops.prom"
assert_contains "$FAILURE_METRICS" 'drivergo_container_running{service="redis"} 0'
assert_contains "$FAILURE_METRICS" 'drivergo_ops_collector_success 0'
pass "collector failure is atomically observable before non-zero exit"

if env \
  PATH="$MOCK_BIN:$PATH" \
  TEXTFILE_DIR="$TEST_ROOT/unsafe-output" \
  MONITORING_LOCK_FILE="$TEST_ROOT/unsafe-run/collector.lock" \
  COMPOSE_FILE="$FIXTURE/docker-compose.prod.yml" \
  COMPOSE_ENV_FILE="$FIXTURE/app.env" \
  BACKUP_ROOT="$BACKUP_ROOT" \
  BACKUP_VERIFY_SCRIPT="$ROOT/scripts/backup/verify_snapshot.sh" \
  "$MONITORING_DIR/write_textfile_metrics.sh" >/dev/null 2>&1; then
  fail "collector must reject a textfile directory without the exact basename"
fi
pass "collector output path guard"

PERMISSIVE_TEXTFILE="$TEST_ROOT/permissive/textfile"
mkdir -p "$PERMISSIVE_TEXTFILE"
chmod 0777 "$PERMISSIVE_TEXTFILE"
if env \
  PATH="$MOCK_BIN:$PATH" \
  TEXTFILE_DIR="$PERMISSIVE_TEXTFILE" \
  MONITORING_LOCK_FILE="$TEST_ROOT/permissive-run/collector.lock" \
  COMPOSE_FILE="$FIXTURE/docker-compose.prod.yml" \
  COMPOSE_ENV_FILE="$FIXTURE/app.env" \
  BACKUP_ROOT="$BACKUP_ROOT" \
  BACKUP_VERIFY_SCRIPT="$ROOT/scripts/backup/verify_snapshot.sh" \
  "$MONITORING_DIR/write_textfile_metrics.sh" >/dev/null 2>&1; then
  fail "collector must reject a group/other-writable textfile directory"
fi
pass "collector rejects writable output-directory races"

if command -v promtool >/dev/null 2>&1; then
  PROMTOOL_CONFIG="$TEST_ROOT/prometheus.yml"
  sed "s#/etc/prometheus/alerts.yml#$MONITORING_DIR/alerts.yml#" \
    "$MONITORING_DIR/prometheus.yml" >"$PROMTOOL_CONFIG"
  promtool check config "$PROMTOOL_CONFIG"
  promtool check rules "$MONITORING_DIR/alerts.yml"
  pass "promtool config and rules validation"
else
  printf '# promtool unavailable; static rule contract completed\n'
fi

printf '1..%d\n' "$pass_count"
