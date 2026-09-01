#!/usr/bin/env bash
# Fail-safe code sync. This script never builds, migrates, restarts, or reloads
# application services. Default mode is a read-only remote preflight + rsync
# dry-run; --apply is required for any remote write.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEFAULT_HOST="root@89.117.59.137"
DEFAULT_DEST="/opt/drivergo"
HOST="${DEPLOY_HOST:-$DEFAULT_HOST}"
DEST="${DEPLOY_PATH:-$DEFAULT_DEST}"
ALLOWED_HOSTS="${DEPLOY_ALLOWED_HOSTS:-$DEFAULT_HOST}"
ALLOWED_PATHS="${DEPLOY_ALLOWED_PATHS:-$DEFAULT_DEST}"
EXCLUDE="${ROOT}/deploy/rsync-exclude.txt"
MODE=dry-run
ROLLBACK_ID=""
# Every --apply leaves a ~75 MB snapshot behind. Without a cap they only ever
# accumulate: 58 of them (4.4 GB) had piled up by 2026-09-01. Ten covers far
# more history than a rollback is ever useful for, since older trees predate
# database migrations that a code-only rollback cannot undo.
ROLLBACK_KEEP="${DEPLOY_ROLLBACK_KEEP:-10}"
SSH_OPTS=(-p 2222 -o BatchMode=yes -o ConnectTimeout=10 -o StrictHostKeyChecking=yes)

usage() {
  cat <<'EOF'
Usage:
  ./deploy/sync-to-vps.sh [--host user@host] [--path /opt/drivergo] [--apply]
  ./deploy/sync-to-vps.sh --rollback SNAPSHOT_ID [--host user@host] [--apply]

Default: validate local/remote state, show rsync changes, and write nothing.
--apply: create a code-only rollback snapshot, sync, write provenance, and
         verify the already-running containers without restarting them.

After a successful --apply, rollback snapshots older than the newest
DEPLOY_ROLLBACK_KEEP (default 10) are removed. A dry-run only lists them.

Allow additional exact targets only through explicit operator configuration:
  DEPLOY_ALLOWED_HOSTS='user@host user@staging'  # space/comma separated
  DEPLOY_ALLOWED_PATHS='/opt/drivergo /opt/drivergo-staging'
  DEPLOY_ROLLBACK_KEEP=10                        # snapshots to retain, 1-999
EOF
}

die() {
  printf 'sync-to-vps: %s\n' "$*" >&2
  exit 2
}

in_allowlist() {
  local candidate="$1" raw="$2" item
  raw="${raw//,/ }"
  for item in $raw; do
    [[ "$candidate" == "$item" ]] && return 0
  done
  return 1
}

validate_host() {
  local host="$1" allowed="$2"
  [[ "$host" =~ ^[A-Za-z0-9._-]+@[A-Za-z0-9.-]+$ ]] || return 1
  in_allowlist "$host" "$allowed"
}

validate_path() {
  local path="$1" allowed="$2"
  [[ "$path" =~ ^/opt/[A-Za-z0-9._/-]+$ ]] || return 1
  [[ "$path" != */../* && "$path" != *'/..' && "$path" != *'//'* ]] || return 1
  [[ "$path" != "/opt" && "$path" != "/opt/" && "$path" != "/" ]] || return 1
  in_allowlist "$path" "$allowed"
}

worktree_clean() {
  git -C "$ROOT" diff --quiet --ignore-submodules -- &&
    git -C "$ROOT" diff --cached --quiet --ignore-submodules -- &&
    [[ -z "$(git -C "$ROOT" ls-files --others --exclude-standard)" ]]
}

remote_preflight() {
  ssh "${SSH_OPTS[@]}" "$HOST" bash -s -- "$DEST" <<'REMOTE'
set -euo pipefail
dest="$1"
[[ "$dest" == /opt/* && "$dest" != /opt && "$dest" != /opt/ ]] || {
  echo "remote preflight: unsafe destination" >&2; exit 20;
}
[[ -d "$dest" ]] || { echo "remote preflight: destination missing: $dest" >&2; exit 21; }
resolved="$(readlink -f -- "$dest")"
[[ "$resolved" == "$dest" ]] || {
  echo "remote preflight: destination is a symlink or non-canonical ($resolved)" >&2; exit 22;
}
[[ -f "$dest/deploy/docker-compose.prod.yml" ]] || {
  echo "remote preflight: production compose sentinel missing" >&2; exit 23;
}
[[ -f "$dest/deploy/app.env" ]] || {
  echo "remote preflight: protected deploy/app.env missing" >&2; exit 24;
}
mode="$(stat -c '%a' "$dest/deploy/app.env")"
[[ "$mode" == "600" ]] || {
  echo "remote preflight: deploy/app.env mode is $mode, require 600" >&2; exit 25;
}
if [[ -e "$dest/.drivergo-deploy-root" ]]; then
  [[ -f "$dest/.drivergo-deploy-root" && ! -L "$dest/.drivergo-deploy-root" ]] || {
    echo "remote preflight: invalid deploy-root sentinel" >&2; exit 26;
  }
  grep -Fxq 'drivergo deploy root sentinel v1' \
    "$dest/.drivergo-deploy-root" || {
    echo "remote preflight: deploy-root sentinel content mismatch" >&2; exit 27;
  }
else
  echo "remote preflight: legacy target; sentinel will arrive on first approved sync"
fi
command -v rsync >/dev/null
command -v docker >/dev/null
df -Pk "$dest" | tail -n 1
REMOTE
}

remote_health() {
  ssh "${SSH_OPTS[@]}" "$HOST" bash -s -- "$DEST" <<'REMOTE'
set -euo pipefail
dest="$1"
cd "$dest"
compose=(docker compose -f deploy/docker-compose.prod.yml --env-file deploy/app.env)
"${compose[@]}" config --quiet
"${compose[@]}" ps --status running api web >/dev/null
"${compose[@]}" exec -T api /healthcheck http://127.0.0.1:8080/readyz
# Prefer the cheap probe; fall back to a locale document on images that
# predate /api/healthz so a code sync can land before the web recreate.
if ! "${compose[@]}" exec -T web wget -qO- http://127.0.0.1:3000/api/healthz >/dev/null; then
  "${compose[@]}" exec -T web wget -qO- http://127.0.0.1:3000/uz-Latn >/dev/null
fi
REMOTE
}

create_snapshot() {
  local snapshot_id="$1"
  ssh "${SSH_OPTS[@]}" "$HOST" bash -s -- "$DEST" "$snapshot_id" <<'REMOTE'
set -euo pipefail
dest="$1"
snapshot_id="$2"
rollback_root="$dest/.deploy-rollbacks"
snapshot_root="$rollback_root/$snapshot_id"
partial_root="$rollback_root/.partial-$snapshot_id"
umask 077
[[ ! -e "$snapshot_root" && ! -L "$snapshot_root" && ! -e "$partial_root" && ! -L "$partial_root" ]] || {
  echo "snapshot already exists: $snapshot_id" >&2; exit 30;
}
mkdir -p -m 0700 "$rollback_root"
[[ -d "$rollback_root" && ! -L "$rollback_root" ]] || { echo "unsafe rollback root" >&2; exit 30; }
mkdir -p "$partial_root/tree"
cleanup_partial() {
  status="$?"
  trap - EXIT
  if [[ -d "$partial_root" && ! -L "$partial_root" && "$(dirname "$partial_root")" == "$rollback_root" && \
        "$(basename "$partial_root")" == ".partial-$snapshot_id" ]]; then
    rm -rf -- "$partial_root"
  fi
  exit "$status"
}
trap cleanup_partial EXIT
[[ -f "$dest/deploy/rsync-exclude.txt" && ! -L "$dest/deploy/rsync-exclude.txt" ]] || {
  echo "snapshot exclude policy missing or unsafe" >&2; exit 31;
}
rsync -a --delete \
  --exclude-from="$dest/deploy/rsync-exclude.txt" \
  --exclude='.deploy-rollbacks/' \
  --exclude='deploy/app.env' \
  --exclude='.deployed-commit' \
  "$dest/" "$partial_root/tree/"
if [[ -f "$dest/.deployed-commit" ]]; then
  cp -a "$dest/.deployed-commit" "$partial_root/previous-deployed-commit"
fi
{
  printf 'format=drivergo-deploy-rollback-v1\n'
  printf 'snapshot_id=%s\n' "$snapshot_id"
  printf 'created_at=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
} >"$partial_root/metadata"
metadata_sha256="$(sha256sum "$partial_root/metadata" | awk '{print $1}')"
{
  printf 'snapshot_id=%s\n' "$snapshot_id"
  printf 'metadata_sha256=%s\n' "$metadata_sha256"
} >"$partial_root/COMPLETE"
mv -- "$partial_root" "$snapshot_root"
trap - EXIT
REMOTE
}

write_provenance() {
  local commit="$1" snapshot_id="$2"
  ssh "${SSH_OPTS[@]}" "$HOST" bash -s -- "$DEST" "$commit" "$snapshot_id" <<'REMOTE'
set -euo pipefail
dest="$1"
commit="$2"
snapshot_id="$3"
tmp="$dest/.deployed-commit.tmp.$$"
umask 022
{
  printf 'commit=%s\n' "$commit"
  printf 'synced_at=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'rollback_snapshot=%s\n' "$snapshot_id"
} >"$tmp"
mv -f "$tmp" "$dest/.deployed-commit"
REMOTE
}

rollback_snapshot() {
  local snapshot_id="$1"
  ssh "${SSH_OPTS[@]}" "$HOST" bash -s -- "$DEST" "$snapshot_id" "$MODE" <<'REMOTE'
set -euo pipefail
dest="$1"
snapshot_id="$2"
mode="$3"
snapshot_root="$dest/.deploy-rollbacks/$snapshot_id"
[[ -d "$snapshot_root" && ! -L "$snapshot_root" && -d "$snapshot_root/tree" && ! -L "$snapshot_root/tree" ]] || {
  echo "rollback snapshot missing or unsafe: $snapshot_id" >&2; exit 40;
}
metadata="$snapshot_root/metadata"
complete="$snapshot_root/COMPLETE"
[[ -f "$metadata" && ! -L "$metadata" && -f "$complete" && ! -L "$complete" ]] || {
  echo "rollback snapshot is incomplete: $snapshot_id" >&2; exit 40;
}
grep -Fxq 'format=drivergo-deploy-rollback-v1' "$metadata" || { echo "rollback metadata format mismatch" >&2; exit 40; }
grep -Fxq "snapshot_id=$snapshot_id" "$metadata" || { echo "rollback metadata id mismatch" >&2; exit 40; }
grep -Fxq "snapshot_id=$snapshot_id" "$complete" || { echo "rollback completion id mismatch" >&2; exit 40; }
expected_metadata_sha="$(awk -F= '$1 == "metadata_sha256" {print $2}' "$complete")"
[[ "$expected_metadata_sha" =~ ^[0-9a-f]{64}$ ]] || { echo "rollback completion digest invalid" >&2; exit 40; }
actual_metadata_sha="$(sha256sum "$metadata" | awk '{print $1}')"
[[ "$actual_metadata_sha" == "$expected_metadata_sha" ]] || { echo "rollback metadata digest mismatch" >&2; exit 40; }
exclude_file="$snapshot_root/tree/deploy/rsync-exclude.txt"
[[ -f "$exclude_file" && ! -L "$exclude_file" ]] || {
  echo "rollback exclude policy missing or unsafe" >&2; exit 41;
}
current_exclude="$dest/deploy/rsync-exclude.txt"
[[ -f "$current_exclude" && ! -L "$current_exclude" ]] || {
  echo "current exclude policy missing or unsafe" >&2; exit 42;
}
args=(-a --delete --delay-updates --itemize-changes
  --exclude-from="$exclude_file"
  --exclude-from="$current_exclude"
  --filter='P deploy/app.env'
  --filter='P .deploy-rollbacks/'
  --filter='P .deployed-commit'
  --filter='P .drivergo-deploy-root')
if [[ "$mode" != apply ]]; then
  args+=(-n)
fi
rsync "${args[@]}" "$snapshot_root/tree/" "$dest/"
if [[ "$mode" == apply && -f "$snapshot_root/previous-deployed-commit" ]]; then
  cp -a "$snapshot_root/previous-deployed-commit" "$dest/.deployed-commit"
fi
REMOTE
}

# Emitted as a standalone payload rather than an inline heredoc so the test
# suite can run this exact code against a temporary tree. This is the only
# path in the script that deletes anything, so its selection logic is proven
# by execution instead of asserted with grep.
prune_payload() {
  cat <<'REMOTE'
set -euo pipefail
dest="$1"
keep="$2"
mode="$3"
rollback_root="$dest/.deploy-rollbacks"
[[ "$keep" =~ ^[1-9][0-9]{0,2}$ ]] || { echo "prune: keep must be 1-999, got: $keep" >&2; exit 50; }
[[ "$mode" == apply || "$mode" == dry-run ]] || { echo "prune: invalid mode: $mode" >&2; exit 50; }
# A missing or symlinked rollback root is not an error worth failing a
# finished deploy over; there is simply nothing safe to prune.
[[ -d "$rollback_root" && ! -L "$rollback_root" ]] || exit 0
# Snapshot ids start with a UTC timestamp, so a lexicographic sort is a
# chronological one. mtime is deliberately not used: inspecting or restoring
# a snapshot can touch it, and reading must never change what gets deleted.
# -type d also drops symlinks, so a symlink named like a snapshot is ignored
# rather than followed and deleted through.
mapfile -t snapshots < <(
  find "$rollback_root" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' |
    grep -xE '[0-9]{8}T[0-9]{6}Z-[0-9a-f]{7,40}' | sort
)
total=${#snapshots[@]}
(( total > keep )) || {
  printf 'prune: %d snapshot(s), keeping %d, nothing to remove\n' "$total" "$keep"
  exit 0
}
if [[ "$mode" == apply ]]; then verb=removed; else verb=would-remove; fi
removed=0
for name in "${snapshots[@]:0:total-keep}"; do
  target="$rollback_root/$name"
  # Re-validate immediately before deleting, not only when listing.
  [[ "$name" =~ ^[0-9]{8}T[0-9]{6}Z-[0-9a-f]{7,40}$ ]] || continue
  [[ -d "$target" && ! -L "$target" && "$(dirname "$target")" == "$rollback_root" ]] || continue
  if [[ "$mode" == apply ]]; then
    rm -rf -- "$target"
  fi
  printf 'prune: %s %s\n' "$verb" "$name"
  removed=$((removed + 1))
done
printf 'prune: %d %s, %d kept (limit %d)\n' "$removed" "$verb" "$((total - removed))" "$keep"
REMOTE
}

prune_snapshots() {
  local keep="$1"
  prune_payload | ssh "${SSH_OPTS[@]}" "$HOST" bash -s -- "$DEST" "$keep" "$MODE"
}

main() {
  while (($#)); do
    case "$1" in
      --apply) MODE=apply; shift ;;
      --dry-run) MODE=dry-run; shift ;;
      --host) [[ $# -ge 2 ]] || die "--host needs a value"; HOST="$2"; shift 2 ;;
      --path) [[ $# -ge 2 ]] || die "--path needs a value"; DEST="$2"; shift 2 ;;
      --rollback) [[ $# -ge 2 ]] || die "--rollback needs a snapshot id"; ROLLBACK_ID="$2"; shift 2 ;;
      -h|--help) usage; return 0 ;;
      *) die "unknown argument: $1" ;;
    esac
  done

  validate_host "$HOST" "$ALLOWED_HOSTS" || die "host is invalid or not allowlisted: $HOST"
  validate_path "$DEST" "$ALLOWED_PATHS" || die "path is invalid or not allowlisted: $DEST"
  [[ "$ROLLBACK_KEEP" =~ ^[1-9][0-9]{0,2}$ ]] ||
    die "DEPLOY_ROLLBACK_KEEP must be 1-999: $ROLLBACK_KEEP"
  [[ -f "$EXCLUDE" ]] || die "exclude file missing: $EXCLUDE"
  for cmd in git rsync ssh docker; do command -v "$cmd" >/dev/null || die "$cmd is required"; done

  if [[ -n "$ROLLBACK_ID" ]]; then
    [[ "$ROLLBACK_ID" =~ ^[0-9]{8}T[0-9]{6}Z-[0-9a-f]{7,40}$ ]] || die "invalid rollback snapshot id"
    remote_preflight
    rollback_snapshot "$ROLLBACK_ID"
    if [[ "$MODE" == apply ]]; then
      remote_health
      printf 'rollback files restored without service restart: %s\n' "$ROLLBACK_ID"
      printf 'running-container health is unchanged and does not prove the restored files are loaded\n'
    else
      printf 'rollback dry-run only; re-run with --apply after review\n'
    fi
    return 0
  fi

  if ! worktree_clean; then
    [[ "$MODE" == dry-run ]] || die "refusing --apply from a dirty/untracked worktree"
    printf 'warning: worktree is dirty; dry-run only\n' >&2
  fi

  commit="$(git -C "$ROOT" rev-parse --verify HEAD)"
  [[ "$commit" =~ ^[0-9a-f]{40}$ ]] || die "cannot resolve commit provenance"
  docker compose -f "$ROOT/deploy/docker-compose.prod.yml" \
    --env-file "$ROOT/deploy/app.prod.env.example" config --no-env-resolution --quiet
  remote_preflight
  remote_health

  rsync_args=(-az --delete-delay --delay-updates --itemize-changes
    --exclude-from="$EXCLUDE"
    --filter='P deploy/app.env'
    --filter='P .deploy-rollbacks/'
    --filter='P .deployed-commit'
    --filter='P .drivergo-deploy-root'
    -e "ssh ${SSH_OPTS[*]}")

  if [[ "$MODE" == dry-run ]]; then
    rsync "${rsync_args[@]}" -n "$ROOT/" "$HOST:$DEST/"
    prune_snapshots "$ROLLBACK_KEEP"
    printf 'dry-run only: no remote files changed; re-run with --apply after review\n'
    return 0
  fi

  snapshot_id="$(date -u +%Y%m%dT%H%M%SZ)-${commit:0:12}"
  create_snapshot "$snapshot_id"
  if ! rsync "${rsync_args[@]}" "$ROOT/" "$HOST:$DEST/"; then
    printf 'sync failed; snapshot retained: %s\n' "$snapshot_id" >&2
    printf 'review rollback: ./deploy/sync-to-vps.sh --rollback %s\n' "$snapshot_id" >&2
    return 1
  fi
  write_provenance "$commit" "$snapshot_id"
  if ! remote_health; then
    printf 'post-sync health failed; running containers were NOT restarted\n' >&2
    printf 'review rollback: ./deploy/sync-to-vps.sh --rollback %s\n' "$snapshot_id" >&2
    return 1
  fi
  # Only after health passes: a failed deploy is exactly when the older
  # snapshots still matter. Housekeeping must never fail a good deploy either,
  # so a prune error is reported and swallowed.
  if ! prune_snapshots "$ROLLBACK_KEEP"; then
    printf 'warning: snapshot pruning failed; snapshots left untouched\n' >&2
  fi
  printf 'sync complete: %s:%s (commit %s)\n' "$HOST" "$DEST" "$commit"
  printf 'rollback snapshot: %s\n' "$snapshot_id"
}

if [[ "${SYNC_TO_VPS_SOURCE_ONLY:-0}" != "1" ]]; then
  main "$@"
fi
