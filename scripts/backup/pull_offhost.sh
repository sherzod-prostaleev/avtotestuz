#!/usr/bin/env bash
# Pull complete encrypted Driver Go snapshots from the VPS onto this PC.
# This is an interim home off-host copy, not provider-backed rclone off-site.
set -euo pipefail
umask 077

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "$SCRIPT_DIR/lib.sh"

require_command ssh
require_command rsync
require_command flock
require_command realpath
require_command sha256sum

OFFHOST_ENV_FILE="${OFFHOST_ENV_FILE:-}"
if [[ -n "$OFFHOST_ENV_FILE" ]]; then
  [[ -f "$OFFHOST_ENV_FILE" && ! -L "$OFFHOST_ENV_FILE" ]] || \
    backup_die "OFFHOST_ENV_FILE missing or unsafe: ${OFFHOST_ENV_FILE}"
  mode="$(stat -c '%a' "$OFFHOST_ENV_FILE")"
  owner="$(stat -c '%u' "$OFFHOST_ENV_FILE")"
  [[ "$owner" == "$EUID" ]] || backup_die "OFFHOST_ENV_FILE must be owned by uid ${EUID}"
  (( 10#$mode % 100 == 0 )) || \
    backup_die "OFFHOST_ENV_FILE must not be group/other-readable (mode ${mode})"
  # shellcheck disable=SC1090
  source "$OFFHOST_ENV_FILE"
fi

# Same default host shape as deploy/sync-to-vps.sh; override via env/file.
OFFHOST_SSH_HOST="${OFFHOST_SSH_HOST:-${DEPLOY_HOST:-root@89.117.59.137}}"
REMOTE_BACKUP_ROOT="${REMOTE_BACKUP_ROOT:-/var/backups/drivergo/full}"
LOCAL_OFFHOST_ROOT="${LOCAL_OFFHOST_ROOT:-${HOME}/drivergo-offhost/full}"
OFFHOST_LOCK_FILE="${OFFHOST_LOCK_FILE:-${HOME}/drivergo-offhost/pull.lock}"
OFFHOST_RETENTION_DAYS="${OFFHOST_RETENTION_DAYS:-35}"
OFFHOST_MIN_SNAPSHOTS="${OFFHOST_MIN_SNAPSHOTS:-3}"
SSH_CONNECT_TIMEOUT="${SSH_CONNECT_TIMEOUT:-15}"

validate_ssh_host() {
  local host="$1"
  # user@host, user@ipv4, or bare Host alias from ~/.ssh/config
  [[ "$host" =~ ^[A-Za-z0-9._-]+(@[A-Za-z0-9.:_-]+)?$ ]] || \
    backup_die "OFFHOST_SSH_HOST looks unsafe: ${host}"
  [[ "$host" != *..* && "$host" != *" "* && "$host" != *'/'* ]] || \
    backup_die "OFFHOST_SSH_HOST looks unsafe: ${host}"
}

validate_remote_backup_root() {
  local root="$1"
  [[ "$root" = /* ]] || backup_die "REMOTE_BACKUP_ROOT must be absolute"
  [[ "$root" != "/" ]] || backup_die "REMOTE_BACKUP_ROOT must not be /"
  [[ "$root" =~ ^/[A-Za-z0-9._/-]+$ ]] || backup_die "REMOTE_BACKUP_ROOT has unsafe characters"
  case "$root" in
    */../*|*/./*|*//*) backup_die "REMOTE_BACKUP_ROOT contains an unsafe path segment" ;;
  esac
}

require_positive_uint OFFHOST_RETENTION_DAYS "$OFFHOST_RETENTION_DAYS"
require_positive_uint OFFHOST_MIN_SNAPSHOTS "$OFFHOST_MIN_SNAPSHOTS"
require_positive_uint SSH_CONNECT_TIMEOUT "$SSH_CONNECT_TIMEOUT"
validate_ssh_host "$OFFHOST_SSH_HOST"
validate_remote_backup_root "$REMOTE_BACKUP_ROOT"
# install -d may create intermediate parents as 0755; tighten the offhost tree.
if [[ "$LOCAL_OFFHOST_ROOT" == */drivergo-offhost/* || "$LOCAL_OFFHOST_ROOT" == */drivergo-offhost ]]; then
  offhost_home="$(dirname "$LOCAL_OFFHOST_ROOT")"
  while [[ "$(basename "$offhost_home")" != "drivergo-offhost" && "$offhost_home" != "/" ]]; do
    offhost_home="$(dirname "$offhost_home")"
  done
  if [[ "$(basename "$offhost_home")" == "drivergo-offhost" && -d "$offhost_home" ]]; then
    chmod 0700 -- "$offhost_home" || true
  fi
fi
validate_backup_root "$LOCAL_OFFHOST_ROOT"
validate_lock_file "$OFFHOST_LOCK_FILE"

SSH_OPTS=(
  -o BatchMode=yes
  -o ConnectTimeout="$SSH_CONNECT_TIMEOUT"
  -o StrictHostKeyChecking=yes
)

ssh_run() {
  ssh "${SSH_OPTS[@]}" "$OFFHOST_SSH_HOST" "$@"
}

remote_list_complete_snapshots() {
  # Remote side only prints basenames of directories that look complete.
  ssh_run env REMOTE_BACKUP_ROOT="$REMOTE_BACKUP_ROOT" bash -s <<'REMOTE'
set -euo pipefail
root="$REMOTE_BACKUP_ROOT"
[[ -d "$root" ]] || exit 0
shopt -s nullglob
for dir in "$root"/drivergo-*; do
  [[ -d "$dir" && ! -L "$dir" ]] || continue
  name="$(basename "$dir")"
  [[ "$name" =~ ^drivergo-[0-9]{8}T[0-9]{6}Z$ ]] || continue
  [[ -f "$dir/manifest.txt" && ! -L "$dir/manifest.txt" ]] || continue
  [[ -f "$dir/SHA256SUMS" && ! -L "$dir/SHA256SUMS" ]] || continue
  printf '%s\n' "$name"
done
REMOTE
}

local_snapshot_complete() {
  local dir="$1"
  [[ -d "$dir" && ! -L "$dir" ]] || return 1
  [[ -f "$dir/manifest.txt" && -f "$dir/SHA256SUMS" ]] || return 1
  "$SCRIPT_DIR/verify_snapshot.sh" "$dir" >/dev/null
}

ensure_pulling_root() {
  local root="${LOCAL_OFFHOST_ROOT}/.pulling"
  if [[ -e "$root" ]]; then
    [[ -d "$root" && ! -L "$root" ]] || backup_die "unsafe pulling root: ${root}"
  else
    mkdir -m 0700 -- "$root"
  fi
  printf '%s\n' "$root"
}

pull_one_snapshot() {
  local name="$1"
  local final stage pulling_root remote_path
  final="${LOCAL_OFFHOST_ROOT}/${name}"
  pulling_root="$(ensure_pulling_root)"
  # Basename must be the real snapshot id so verify_snapshot.sh accepts it.
  stage="${pulling_root}/${name}"
  remote_path="${OFFHOST_SSH_HOST}:${REMOTE_BACKUP_ROOT}/${name}/"

  if [[ -d "$final" && ! -L "$final" ]]; then
    if local_snapshot_complete "$final"; then
      echo "pull_offhost: already verified ${name}"
      return 0
    fi
    backup_die "local snapshot ${name} exists but fails verification; move it aside manually"
  fi

  if [[ -e "$stage" ]]; then
    [[ -d "$stage" && ! -L "$stage" ]] || \
      backup_die "unsafe stage path for ${name}: ${stage}"
  else
    mkdir -m 0700 -- "$stage"
  fi

  echo "pull_offhost: syncing ${name}"
  rsync -a --partial --info=stats1 \
    -e "ssh ${SSH_OPTS[*]}" \
    "$remote_path" \
    "${stage}/"

  "$SCRIPT_DIR/verify_snapshot.sh" "$stage" >/dev/null || \
    backup_die "verification failed for staged ${name}; leaving ${stage} for resume"

  # Refuse to clobber an unexpected final path that appeared during sync.
  if [[ -e "$final" ]]; then
    backup_die "final path appeared during sync: ${final}"
  fi
  mv -T -- "$stage" "$final"
  echo "pull_offhost: verified and installed ${name}"
}

prune_local_snapshots() {
  local -a names=()
  local name cutoff keep_count age_ok
  shopt -s nullglob
  for dir in "${LOCAL_OFFHOST_ROOT}"/drivergo-*; do
    [[ -d "$dir" && ! -L "$dir" ]] || continue
    name="$(basename "$dir")"
    is_snapshot_name "$name" || continue
    names+=("$name")
  done
  shopt -u nullglob

  ((${#names[@]} == 0)) && return 0

  # Newest first by snapshot name (UTC timestamp embedded).
  IFS=$'\n' names=($(printf '%s\n' "${names[@]}" | sort -r))
  unset IFS

  cutoff="$(date -u -d "${OFFHOST_RETENTION_DAYS} days ago" +%Y%m%dT%H%M%SZ 2>/dev/null || true)"
  if [[ -z "$cutoff" ]]; then
    # BusyBox/date without -d: keep only the minimum count.
    cutoff=""
  fi

  keep_count=0
  for name in "${names[@]}"; do
    keep_count=$((keep_count + 1))
    age_ok=1
    if [[ -n "$cutoff" ]]; then
      # Compare drivergo-TIMESTAMPZ payload lexicographically with cutoff.
      stamp="${name#drivergo-}"
      if [[ "$stamp" < "${cutoff}" ]]; then
        age_ok=0
      fi
    fi
    if (( keep_count <= 10#$OFFHOST_MIN_SNAPSHOTS )) || (( age_ok == 1 )); then
      continue
    fi
    echo "pull_offhost: pruning local ${name}"
    rm -rf -- "${LOCAL_OFFHOST_ROOT}/${name}"
  done
}

exec 9>"$OFFHOST_LOCK_FILE"
if ! flock -n 9; then
  backup_die "another pull_offhost run holds ${OFFHOST_LOCK_FILE}"
fi

echo "pull_offhost: listing complete snapshots on ${OFFHOST_SSH_HOST}:${REMOTE_BACKUP_ROOT}"
mapfile -t remote_names < <(remote_list_complete_snapshots | sort)
if ((${#remote_names[@]} == 0)); then
  echo "pull_offhost: no complete remote snapshots found"
  exit 0
fi

pulled=0
for name in "${remote_names[@]}"; do
  is_snapshot_name "$name" || backup_die "remote returned unsafe snapshot name: ${name}"
  pull_one_snapshot "$name"
  pulled=$((pulled + 1))
done

prune_local_snapshots
echo "pull_offhost: done (${pulled} remote snapshot(s) considered)"
