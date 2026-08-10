#!/usr/bin/env bash
# Operate the isolated API/web candidate stack. No stateful service or Humo
# watcher is duplicated. Default is validation only; --apply is required.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ACTION="${1:-}"
[[ -n "$ACTION" ]] && shift
MODE=dry-run
ACTIVE_INCLUDE="${DRIVERGO_NGINX_UPSTREAM_INCLUDE:-/etc/nginx/snippets/drivergo-upstreams.conf}"
SLOT_LOCK_FILE="${DRIVERGO_SLOT_LOCK_FILE:-/run/lock/drivergo-app-slot.lock}"
ENV_FILE="${DRIVERGO_CANDIDATE_ENV_FILE:-$ROOT/deploy/app.env}"
BACKUP_HOST="${CANDIDATE_BACKUP_HOST:-root@89.117.59.137}"
BACKUP_ALLOWED_HOSTS="${CANDIDATE_BACKUP_ALLOWED_HOSTS:-root@89.117.59.137}"
BACKUP_ROOT="${CANDIDATE_BACKUP_ROOT:-/var/backups/drivergo/full}"
BACKUP_MAX_AGE_SECONDS="${CANDIDATE_BACKUP_MAX_AGE_SECONDS:-93600}"
SSH_OPTS=(-o BatchMode=yes -o ConnectTimeout=10 -o StrictHostKeyChecking=yes)

while (($#)); do
  case "$1" in
    --apply) MODE=apply ;;
    --dry-run) MODE=dry-run ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
  shift
done

case "$ACTION" in
  preflight|up|down|status) ;;
  *) echo "Usage: $0 preflight|up|down|status [--apply]" >&2; exit 2 ;;
esac

is_immutable_ref() {
  local ref="$1"
  [[ "$ref" =~ ^[^[:space:]]+@sha256:[0-9a-f]{64}$ || "$ref" =~ ^sha256:[0-9a-f]{64}$ ]]
}

in_allowlist() {
  local candidate="$1" raw="$2" item
  raw="${raw//,/ }"
  for item in $raw; do
    [[ "$candidate" == "$item" ]] && return 0
  done
  return 1
}

valid_backup_host() {
  [[ "$1" =~ ^[A-Za-z0-9._-]+@[A-Za-z0-9.-]+$ ]] &&
    in_allowlist "$1" "$BACKUP_ALLOWED_HOSTS"
}

valid_backup_root() {
  [[ "$1" =~ ^/var/backups/drivergo(/[A-Za-z0-9._-]+)*$ &&
    "$1" != /var/backups/drivergo &&
    "$1" != */../* && "$1" != */.. && "$1" != *'//' ]]
}

env_has_name() {
  local name="$1"
  awk -F= -v name="$name" '$1 == name { found=1 } END { exit !found }' "$ENV_FILE"
}

print_required_env_names() {
  printf 'required environment names: CANDIDATE_API_IMAGE CANDIDATE_WEB_IMAGE'
  printf ' ENV DATABASE_URL REDIS_URL MINIO_ROOT_USER MINIO_ROOT_PASSWORD CLIENT_IP_ASSERTION_SECRET\n'
}

check_required_env_names() {
  local name
  for name in ENV DATABASE_URL REDIS_URL MINIO_ROOT_USER MINIO_ROOT_PASSWORD CLIENT_IP_ASSERTION_SECRET; do
    env_has_name "$name" || {
      echo "protected environment is missing required name: $name" >&2
      exit 2
    }
  done
  print_required_env_names
}

remote_backup_preflight() {
  valid_backup_host "$BACKUP_HOST" || {
    echo "CANDIDATE_BACKUP_HOST is invalid or not allowlisted" >&2
    exit 2
  }
  valid_backup_root "$BACKUP_ROOT" || {
    echo "CANDIDATE_BACKUP_ROOT must be a dedicated /var/backups/drivergo subdirectory" >&2
    exit 2
  }
  [[ "$BACKUP_MAX_AGE_SECONDS" =~ ^[1-9][0-9]*$ ]] || {
    echo "CANDIDATE_BACKUP_MAX_AGE_SECONDS must be a positive integer" >&2
    exit 2
  }
  ssh "${SSH_OPTS[@]}" "$BACKUP_HOST" bash -s -- "$BACKUP_ROOT" "$BACKUP_MAX_AGE_SECONDS" <<'REMOTE'
set -euo pipefail
backup_root="$1"
max_age_seconds="$2"
[[ -d "$backup_root" && ! -L "$backup_root" ]] || {
  echo "backup snapshot gate: backup root missing or unsafe" >&2; exit 1;
}
latest=""
for snapshot in "$backup_root"/drivergo-*; do
  [[ -d "$snapshot" && ! -L "$snapshot" ]] || continue
  name="$(basename "$snapshot")"
  [[ "$name" =~ ^drivergo-[0-9]{8}T[0-9]{6}Z$ ]] || continue
  [[ -f "$snapshot/manifest.txt" && ! -L "$snapshot/manifest.txt" ]] || continue
  [[ -s "$snapshot/SHA256SUMS" && ! -L "$snapshot/SHA256SUMS" ]] || continue
  if [[ -z "$latest" || "$name" > "$(basename "$latest")" ]]; then
    latest="$snapshot"
  fi
done
[[ -n "$latest" ]] || {
  echo "backup snapshot gate: no complete snapshot metadata found" >&2; exit 1;
}
name="$(basename "$latest")"
stamp="${name#drivergo-}"
created_epoch="$(date -u -d "${stamp:0:8} ${stamp:9:2}:${stamp:11:2}:${stamp:13:2} UTC" +%s)"
now_epoch="$(date -u +%s)"
age_seconds="$((now_epoch - created_epoch))"
(( age_seconds >= 0 && age_seconds <= max_age_seconds )) || {
  echo "backup snapshot gate: newest snapshot is stale or future-dated" >&2; exit 1;
}
for entry in \
  "format=drivergo-full-backup-v1" \
  "snapshot=$name" \
  "encryption=age"; do
  grep -Fxq "$entry" "$latest/manifest.txt" || {
    echo "backup snapshot gate: latest snapshot metadata is invalid" >&2; exit 1;
  }
done
echo "backup snapshot gate: ok (snapshot=$name age_seconds=$age_seconds)"
REMOTE
}

active_slot() {
  [[ -f "$ACTIVE_INCLUDE" && ! -L "$ACTIVE_INCLUDE" ]] || return 1
  if cmp -s -- "$ACTIVE_INCLUDE" "$ROOT/deploy/nginx/upstreams-stable.conf"; then
    printf 'stable\n'
  elif cmp -s -- "$ACTIVE_INCLUDE" "$ROOT/deploy/nginx/upstreams-candidate.conf"; then
    printf 'candidate\n'
  else
    return 1
  fi
}

lock_and_require_stable_slot() {
  local slot
  [[ "$EUID" -eq 0 ]] || { echo "--apply must run as root on the nginx host" >&2; exit 2; }
  [[ "$SLOT_LOCK_FILE" = /* && "$SLOT_LOCK_FILE" == *.lock ]] || {
    echo "DRIVERGO_SLOT_LOCK_FILE must be an absolute .lock path" >&2; exit 2;
  }
  mkdir -p -m 0755 -- "$(dirname "$SLOT_LOCK_FILE")"
  exec 9>"$SLOT_LOCK_FILE"
  flock -n 9 || { echo "another app-slot operation is in progress" >&2; exit 1; }
  slot="$(active_slot)" || {
    echo "active nginx upstream is missing, unsafe, or not a known slot; refusing" >&2
    exit 1
  }
  [[ "$slot" == stable ]] || {
    echo "candidate is carrying production traffic; switch to a healthy stable slot before ${ACTION}" >&2
    exit 1
  }
}

[[ -f "$ENV_FILE" ]] || { echo "missing protected deploy/app.env" >&2; exit 2; }
[[ "$(stat -c '%a' "$ENV_FILE")" == "600" ]] || {
  echo "deploy/app.env must have mode 600" >&2; exit 2;
}
: "${CANDIDATE_API_IMAGE:?set CANDIDATE_API_IMAGE to a repository digest or local sha256 image ID}"
: "${CANDIDATE_WEB_IMAGE:?set CANDIDATE_WEB_IMAGE to a repository digest or local sha256 image ID}"
is_immutable_ref "$CANDIDATE_API_IMAGE" || { echo "API image ref is not immutable" >&2; exit 2; }
is_immutable_ref "$CANDIDATE_WEB_IMAGE" || { echo "web image ref is not immutable" >&2; exit 2; }
check_required_env_names

compose=(docker compose -f "$ROOT/deploy/docker-compose.candidate.yml" --env-file "$ENV_FILE")
"${compose[@]}" config --quiet

candidate_runtime_preflight() {
  docker network inspect drivergo_default >/dev/null
  docker image inspect "$CANDIDATE_API_IMAGE" "$CANDIDATE_WEB_IMAGE" >/dev/null
  remote_backup_preflight
}

if [[ "$ACTION" == status ]]; then
  "${compose[@]}" ps
  exit 0
fi

if [[ "$ACTION" == preflight ]]; then
  candidate_runtime_preflight
  echo "candidate preflight ok; no container started and nginx was not changed"
  exit 0
fi

if [[ "$ACTION" == down ]]; then
  if [[ "$MODE" != apply ]]; then
    "${compose[@]}" ps
    echo "dry-run only: candidate stack was not removed"
    exit 0
  fi
  lock_and_require_stable_slot
  curl -fsS "http://127.0.0.1:8081/readyz" | grep -q '"status"[[:space:]]*:[[:space:]]*"ok"'
  curl -fsS -o /dev/null "http://127.0.0.1:3010/uz-Latn"
  "${compose[@]}" down
  exit 0
fi

candidate_runtime_preflight
if [[ "$MODE" != apply ]]; then
  echo "candidate preflight ok; no container started"
  echo "set CANDIDATE_EXPAND_CONTRACT_ACK=1 and re-run with --apply"
  exit 0
fi

lock_and_require_stable_slot

[[ "${CANDIDATE_EXPAND_CONTRACT_ACK:-0}" == 1 ]] || {
  echo "refusing candidate API start without CANDIDATE_EXPAND_CONTRACT_ACK=1" >&2
  echo "API startup self-migrates; current and candidate must both support the schema" >&2
  exit 2
}
# One candidate API process means one normal startup migration attempt. This
# script never invokes a migration command separately and never scales API out.
"${compose[@]}" up -d --no-build --wait \
  --scale api-candidate=1 --scale web-candidate=1
curl -fsS "http://127.0.0.1:${CANDIDATE_API_PORT:-18081}/readyz" >/dev/null
curl -fsS -o /dev/null "http://127.0.0.1:${CANDIDATE_WEB_PORT:-13010}/uz-Latn"
echo "candidate healthy; nginx remains unchanged"
echo "review: ./deploy/switch-app-slot.sh --to candidate"
