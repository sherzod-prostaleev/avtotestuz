#!/usr/bin/env bash
# Validate and atomically switch nginx between stable and app-only candidate.
# Default is dry-run. This script never starts containers or runs migrations.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SLOT=""
MODE=dry-run
ACTIVE_INCLUDE="${DRIVERGO_NGINX_UPSTREAM_INCLUDE:-/etc/nginx/snippets/drivergo-upstreams.conf}"
SLOT_LOCK_FILE="${DRIVERGO_SLOT_LOCK_FILE:-/run/lock/drivergo-app-slot.lock}"

usage() {
  printf 'Usage: %s --to stable|candidate [--apply]\n' "$0"
}

while (($#)); do
  case "$1" in
    --to) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; SLOT="$2"; shift 2 ;;
    --apply) MODE=apply; shift ;;
    --dry-run) MODE=dry-run; shift ;;
    -h|--help) usage; exit 0 ;;
    *) printf 'unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
done

[[ "$SLOT" == stable || "$SLOT" == candidate ]] || { usage >&2; exit 2; }
source_file="$ROOT/deploy/nginx/upstreams-${SLOT}.conf"
[[ -f "$source_file" && ! -L "$source_file" ]] || { echo "slot config missing" >&2; exit 2; }

if [[ "$SLOT" == candidate ]]; then
  api_port=18081
  web_port=13010
else
  api_port=8081
  web_port=3010
fi

if [[ "$MODE" == apply ]]; then
  [[ "$EUID" -eq 0 ]] || { echo "--apply must run as root on the nginx host" >&2; exit 2; }
  [[ "$SLOT_LOCK_FILE" = /* && "$SLOT_LOCK_FILE" == *.lock ]] || {
    echo "DRIVERGO_SLOT_LOCK_FILE must be an absolute .lock path" >&2; exit 2;
  }
  mkdir -p -m 0755 -- "$(dirname "$SLOT_LOCK_FILE")"
  exec 9>"$SLOT_LOCK_FILE"
  flock -n 9 || { echo "another app-slot operation is in progress" >&2; exit 1; }
fi

# For --apply the shared slot lock is already held, so a concurrent candidate
# cleanup cannot invalidate this successful probe before the nginx switch.
curl -fsS "http://127.0.0.1:${api_port}/readyz" | grep -q '"status"[[:space:]]*:[[:space:]]*"ok"'
curl -fsS -o /dev/null "http://127.0.0.1:${web_port}/uz-Latn"

if [[ -e "$ACTIVE_INCLUDE" ]]; then
  diff -u "$ACTIVE_INCLUDE" "$source_file" || true
else
  printf 'active include does not exist yet: %s\n' "$ACTIVE_INCLUDE"
fi

if [[ "$MODE" != apply ]]; then
  printf 'dry-run only: %s is healthy; nginx was not changed\n' "$SLOT"
  exit 0
fi

include_dir="$(dirname "$ACTIVE_INCLUDE")"
[[ -d "$include_dir" && ! -L "$include_dir" ]] || { echo "unsafe nginx include directory" >&2; exit 2; }
backup="$(mktemp "$include_dir/.drivergo-upstreams.backup.XXXXXX")"
candidate="$(mktemp "$include_dir/.drivergo-upstreams.candidate.XXXXXX")"
had_active=0
candidate_installed=0
committed=0
cleanup() {
  rm -f -- "$candidate"
  # Keep the rollback snapshot on any interrupted/failed transaction.
  [[ "$committed" == 1 ]] && rm -f -- "$backup"
}
trap cleanup EXIT

if [[ -e "$ACTIVE_INCLUDE" ]]; then
  [[ -f "$ACTIVE_INCLUDE" && ! -L "$ACTIVE_INCLUDE" ]] || {
    echo "unsafe active nginx include" >&2
    exit 2
  }
  had_active=1
  cp -a -- "$ACTIVE_INCLUDE" "$backup"
fi
install -m 0644 -o root -g root "$source_file" "$candidate"

restore_previous() {
  if [[ "$had_active" == 1 ]]; then
    cp -a -- "$backup" "$ACTIVE_INCLUDE"
  else
    rm -f -- "$ACTIVE_INCLUDE"
  fi
}

rollback_and_reload_previous() {
  restore_previous &&
    nginx -t &&
    systemctl reload nginx
}

rollback_on_signal() {
  local signal="$1"
  trap - HUP INT TERM
  if [[ "$candidate_installed" == 1 ]] && ! rollback_and_reload_previous; then
    echo "interrupted by $signal; previous upstream include restored but nginx rollback reload failed" >&2
  fi
  exit 128
}
trap 'rollback_on_signal HUP' HUP
trap 'rollback_on_signal INT' INT
trap 'rollback_on_signal TERM' TERM

# Mark before the atomic rename so a signal cannot slip between installation
# and enabling rollback; restoring before a completed rename is harmless.
candidate_installed=1
mv -f -- "$candidate" "$ACTIVE_INCLUDE"

if ! nginx -t; then
  if rollback_and_reload_previous; then
    echo "nginx validation failed; previous upstream include restored and reloaded" >&2
  else
    echo "nginx validation failed; previous upstream include restored but rollback reload failed" >&2
  fi
  exit 1
fi
if ! systemctl reload nginx; then
  if rollback_and_reload_previous; then
    echo "nginx reload failed; previous upstream include restored and reloaded" >&2
  else
    echo "nginx reload failed; previous upstream include restored but rollback reload failed" >&2
  fi
  exit 1
fi
committed=1
printf 'nginx switched gracefully to %s; rollback: %s --to %s --apply\n' \
  "$SLOT" "$0" "$([[ "$SLOT" == candidate ]] && echo stable || echo candidate)"
