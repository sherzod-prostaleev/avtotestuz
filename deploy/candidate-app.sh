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

while (($#)); do
  case "$1" in
    --apply) MODE=apply ;;
    --dry-run) MODE=dry-run ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
  shift
done

case "$ACTION" in
  up|down|status) ;;
  *) echo "Usage: $0 up|down|status [--apply]" >&2; exit 2 ;;
esac

is_immutable_ref() {
  local ref="$1"
  [[ "$ref" =~ ^[^[:space:]]+@sha256:[0-9a-f]{64}$ || "$ref" =~ ^sha256:[0-9a-f]{64}$ ]]
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

ENV_FILE="$ROOT/deploy/app.env"
[[ -f "$ENV_FILE" ]] || { echo "missing protected deploy/app.env" >&2; exit 2; }
[[ "$(stat -c '%a' "$ENV_FILE")" == "600" ]] || {
  echo "deploy/app.env must have mode 600" >&2; exit 2;
}
: "${CANDIDATE_API_IMAGE:?set CANDIDATE_API_IMAGE to a repository digest or local sha256 image ID}"
: "${CANDIDATE_WEB_IMAGE:?set CANDIDATE_WEB_IMAGE to a repository digest or local sha256 image ID}"
is_immutable_ref "$CANDIDATE_API_IMAGE" || { echo "API image ref is not immutable" >&2; exit 2; }
is_immutable_ref "$CANDIDATE_WEB_IMAGE" || { echo "web image ref is not immutable" >&2; exit 2; }

compose=(docker compose -f "$ROOT/deploy/docker-compose.candidate.yml" --env-file "$ENV_FILE")
"${compose[@]}" config --quiet

if [[ "$ACTION" == status ]]; then
  "${compose[@]}" ps
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

docker network inspect drivergo_default >/dev/null
docker image inspect "$CANDIDATE_API_IMAGE" "$CANDIDATE_WEB_IMAGE" >/dev/null
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
