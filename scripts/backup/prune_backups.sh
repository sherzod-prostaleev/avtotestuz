#!/usr/bin/env bash
# Retention for complete Driver Go snapshot directories only.
set -euo pipefail
umask 077

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "$SCRIPT_DIR/lib.sh"

ROOT="$(repo_root)"
BACKUP_ROOT="${BACKUP_ROOT:-$ROOT/.run/backups/full}"
BACKUP_QUARANTINE_ROOT="${BACKUP_QUARANTINE_ROOT:-$(dirname "$BACKUP_ROOT")/quarantine}"
BACKUP_LOCK_FILE="${BACKUP_LOCK_FILE:-$BACKUP_ROOT/.drivergo-backup.lock}"
BACKUP_LOCK_HELD="${BACKUP_LOCK_HELD:-0}"
BACKUP_RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-14}"
OFFSITE_RETENTION_DAYS="${OFFSITE_RETENTION_DAYS:-35}"
OFFSITE_INCOMPLETE_RETENTION_DAYS="${OFFSITE_INCOMPLETE_RETENTION_DAYS:-7}"
BACKUP_MIN_SNAPSHOTS="${BACKUP_MIN_SNAPSHOTS:-3}"
PARTIAL_RETENTION_HOURS="${PARTIAL_RETENTION_HOURS:-24}"
RCLONE_REMOTE="${RCLONE_REMOTE:-}"
PRUNE_OFFSITE="${PRUNE_OFFSITE:-0}"
PRUNE_DRY_RUN="${PRUNE_DRY_RUN:-0}"

require_command realpath
require_command date
require_bool BACKUP_LOCK_HELD "$BACKUP_LOCK_HELD"
require_uint BACKUP_RETENTION_DAYS "$BACKUP_RETENTION_DAYS"
require_uint OFFSITE_RETENTION_DAYS "$OFFSITE_RETENTION_DAYS"
require_uint OFFSITE_INCOMPLETE_RETENTION_DAYS "$OFFSITE_INCOMPLETE_RETENTION_DAYS"
require_positive_uint BACKUP_MIN_SNAPSHOTS "$BACKUP_MIN_SNAPSHOTS"
require_uint PARTIAL_RETENTION_HOURS "$PARTIAL_RETENTION_HOURS"
require_bool PRUNE_OFFSITE "$PRUNE_OFFSITE"
require_bool PRUNE_DRY_RUN "$PRUNE_DRY_RUN"
validate_backup_root "$BACKUP_ROOT"
BACKUP_ROOT="$(realpath -e "$BACKUP_ROOT")"
validate_quarantine_root "$BACKUP_ROOT" "$BACKUP_QUARANTINE_ROOT"
BACKUP_QUARANTINE_ROOT="$(realpath -e "$BACKUP_QUARANTINE_ROOT")"

if [[ "$BACKUP_LOCK_HELD" != "1" ]]; then
  require_command flock
  validate_lock_file "$BACKUP_LOCK_FILE"
  exec 9>"$BACKUP_LOCK_FILE"
  flock -n 9 || backup_die "another backup or retention run holds ${BACKUP_LOCK_FILE}"
fi

snapshot_epoch() {
  local name="$1"
  local stamp="${name#drivergo-}"
  date -u -d "${stamp:0:4}-${stamp:4:2}-${stamp:6:2} ${stamp:9:2}:${stamp:11:2}:${stamp:13:2}Z" +%s
}

eligible_for_prune() {
  local name="$1"
  local retention_days="$2"
  local created now cutoff
  created="$(snapshot_epoch "$name")" || return 1
  now="$(date -u +%s)"
  cutoff="$((now - retention_days * 86400))"
  (( created < cutoff ))
}

partial_listing="$(find "$BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d -name '.partial-drivergo-*' -print)" || \
  backup_die "failed to enumerate partial snapshots"
while IFS= read -r entry; do
  [[ -n "$entry" ]] || continue
  name="$(basename "$entry")"
  is_partial_snapshot_name "$name" || continue
  partial_snapshot_name="${name#.partial-}"
  created="$(snapshot_epoch "$partial_snapshot_name")" || continue
  partial_cutoff="$(( $(date -u +%s) - PARTIAL_RETENTION_HOURS * 3600 ))"
  (( created < partial_cutoff )) || continue
  target_real="$(realpath -e "$entry")"
  root_real="$(realpath -e "$BACKUP_ROOT")"
  [[ "$(dirname "$target_real")" == "$root_real" ]] || backup_die "partial prune target escaped root: ${entry}"
  is_partial_snapshot_name "$(basename "$target_real")" || backup_die "unsafe partial prune target: ${target_real}"
  if [[ "$PRUNE_DRY_RUN" == "1" ]]; then
    echo "retention dry-run: partial ${target_real}"
  else
    rm -rf -- "$target_real"
    echo "retention: removed stale partial ${target_real}"
  fi
done < <(printf '%s\n' "$partial_listing" | sort)

local_snapshots=()
local_listing="$(find "$BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d -name 'drivergo-*' -print)" || \
  backup_die "failed to enumerate local snapshots"
while IFS= read -r entry; do
  [[ -n "$entry" ]] || continue
  name="$(basename "$entry")"
  is_snapshot_name "$name" || continue
  [[ ! -L "$entry" ]] || backup_die "refusing symlink snapshot: ${entry}"
  if ! "$SCRIPT_DIR/verify_snapshot.sh" "$entry" >/dev/null 2>&1; then
    quarantine_target="$BACKUP_QUARANTINE_ROOT/${name}-invalid"
    [[ ! -e "$quarantine_target" && ! -L "$quarantine_target" ]] || \
      backup_die "quarantine target already exists: ${quarantine_target}"
    if [[ "$PRUNE_DRY_RUN" == "1" ]]; then
      echo "retention dry-run: quarantine invalid local snapshot ${entry}"
    else
      mv -- "$entry" "$quarantine_target"
      echo "retention: quarantined invalid local snapshot ${quarantine_target}"
    fi
    continue
  fi
  local_snapshots+=("$name")
done < <(printf '%s\n' "$local_listing" | sort)

remaining="${#local_snapshots[@]}"
for name in "${local_snapshots[@]}"; do
  (( remaining > BACKUP_MIN_SNAPSHOTS )) || break
  eligible_for_prune "$name" "$BACKUP_RETENTION_DAYS" || continue
  target="$BACKUP_ROOT/$name"
  target_real="$(realpath -e "$target")"
  root_real="$(realpath -e "$BACKUP_ROOT")"
  [[ "$(dirname "$target_real")" == "$root_real" ]] || backup_die "local prune target escaped root: ${target}"
  is_snapshot_name "$(basename "$target_real")" || backup_die "unsafe local prune target: ${target_real}"
  if [[ "$PRUNE_DRY_RUN" == "1" ]]; then
    echo "retention dry-run: local ${target_real}"
  else
    rm -rf -- "$target_real"
    echo "retention: removed local ${target_real}"
  fi
  remaining="$((remaining - 1))"
done

if [[ "$PRUNE_OFFSITE" == "1" ]]; then
  [[ -n "$RCLONE_REMOTE" ]] || backup_die "PRUNE_OFFSITE=1 requires RCLONE_REMOTE"
  require_command rclone
  require_command mktemp
  require_command sha256sum
  validate_remote_base "$RCLONE_REMOTE"
  remote_verify_tmp="$(mktemp -d /tmp/drivergo-offsite-verify.XXXXXX)"
  chmod 0700 "$remote_verify_tmp"
  cleanup_remote_verify() {
    local status="$?"
    trap - EXIT
    case "$remote_verify_tmp" in
      /tmp/drivergo-offsite-verify.*) rm -rf -- "$remote_verify_tmp" || status=1 ;;
      *) status=1 ;;
    esac
    exit "$status"
  }
  trap cleanup_remote_verify EXIT

  verify_remote_snapshot() {
    local name="$1"
    local remote_snapshot="${RCLONE_REMOTE%/}/$name"
    local marker_snapshot marker_digest checksums_digest file key count
    local -a expected=(manifest.txt)
    local -A allowed=() seen=()

    rm -f -- "$remote_verify_tmp/manifest.txt" "$remote_verify_tmp/SHA256SUMS" \
      "$remote_verify_tmp/REMOTE_COMPLETE" "$remote_verify_tmp/FILES"
    rclone copyto "$remote_snapshot/manifest.txt" "$remote_verify_tmp/manifest.txt" >/dev/null || return 1
    rclone copyto "$remote_snapshot/SHA256SUMS" "$remote_verify_tmp/SHA256SUMS" >/dev/null || return 1
    rclone copyto "$remote_snapshot/REMOTE_COMPLETE" "$remote_verify_tmp/REMOTE_COMPLETE" >/dev/null || return 1
    rclone lsf "$remote_snapshot" --files-only >"$remote_verify_tmp/FILES" || return 1

    [[ "$(wc -l <"$remote_verify_tmp/REMOTE_COMPLETE" | tr -d ' ')" == "2" ]] || return 1
    for key in snapshot sha256sums_sha256; do
      count="$(awk -F= -v wanted="$key" '$1 == wanted { count++ } END { print count + 0 }' \
        "$remote_verify_tmp/REMOTE_COMPLETE")"
      [[ "$count" == "1" ]] || return 1
    done
    marker_snapshot="$(manifest_value "$remote_verify_tmp/REMOTE_COMPLETE" snapshot)"
    marker_digest="$(manifest_value "$remote_verify_tmp/REMOTE_COMPLETE" sha256sums_sha256)"
    [[ "$marker_snapshot" == "$name" && "$marker_digest" =~ ^[0-9a-fA-F]{64}$ ]] || return 1
    checksums_digest="$(sha256sum -- "$remote_verify_tmp/SHA256SUMS" | awk '{ print $1 }')"
    [[ "${checksums_digest,,}" == "${marker_digest,,}" ]] || return 1

    [[ "$(manifest_value "$remote_verify_tmp/manifest.txt" format)" == "drivergo-full-backup-v1" ]] || return 1
    [[ "$(manifest_value "$remote_verify_tmp/manifest.txt" snapshot)" == "$name" ]] || return 1
    for key in postgres redis minio humo; do
      count="$(awk -F= -v wanted="component_file_${key}" '$1 == wanted { count++ } END { print count + 0 }' \
        "$remote_verify_tmp/manifest.txt")"
      [[ "$count" == "1" ]] || return 1
      file="$(manifest_value "$remote_verify_tmp/manifest.txt" "component_file_${key}")"
      [[ -n "$file" && "$file" != */* && "$file" != .* ]] || return 1
      expected+=("$file")
    done
    for file in "${expected[@]}" SHA256SUMS REMOTE_COMPLETE; do
      [[ -z "${allowed[$file]:-}" ]] || return 1
      allowed["$file"]=1
    done
    count=0
    while IFS= read -r file; do
      [[ -n "$file" && "$file" != */* && -n "${allowed[$file]:-}" && -z "${seen[$file]:-}" ]] || return 1
      seen["$file"]=1
      count="$((count + 1))"
    done <"$remote_verify_tmp/FILES"
    [[ "$count" -eq "${#allowed[@]}" ]] || return 1

    # At most BACKUP_MIN_SNAPSHOTS candidates are downloaded and hashed. This
    # detects remote payload bitrot without re-reading the entire retention
    # history every day.
    rclone hashsum SHA-256 "$remote_snapshot" --download \
      --checkfile "$remote_verify_tmp/SHA256SUMS" >/dev/null
  }

  remote_snapshots=()
  remote_listing="$(rclone lsf "$RCLONE_REMOTE" --dirs-only)" || \
    backup_die "failed to enumerate off-site snapshots"
  while IFS= read -r entry; do
    [[ -n "$entry" ]] || continue
    name="${entry%/}"
    is_snapshot_name "$name" || continue
    remote_files="$(rclone lsf "${RCLONE_REMOTE%/}/$name" --files-only)" || \
      backup_die "failed to inspect off-site snapshot ${name}"
    if ! grep -Fxq 'manifest.txt' <<<"$remote_files" || \
       ! grep -Fxq 'SHA256SUMS' <<<"$remote_files" || \
       ! grep -Fxq 'REMOTE_COMPLETE' <<<"$remote_files"; then
      if eligible_for_prune "$name" "$OFFSITE_INCOMPLETE_RETENTION_DAYS"; then
        incomplete_target="${RCLONE_REMOTE%/}/$name"
        if [[ "$PRUNE_DRY_RUN" == "1" ]]; then
          echo "retention dry-run: incomplete off-site ${incomplete_target}"
        else
          rclone purge "$incomplete_target"
          echo "retention: removed incomplete off-site ${incomplete_target}"
        fi
      fi
      continue
    fi
    remote_snapshots+=("$name")
  done < <(printf '%s\n' "$remote_listing" | sort)

  required_protected="${#remote_snapshots[@]}"
  (( required_protected > BACKUP_MIN_SNAPSHOTS )) && required_protected="$BACKUP_MIN_SNAPSHOTS"
  verified_count=0
  declare -A protected_remote=()
  # A failed verification can mean bitrot, provider throttling, timeout, or a
  # transient transport failure. Never auto-delete that evidence: preserve it
  # for manual diagnosis and fall back to another fully verified snapshot.
  declare -A uncertain_remote=()
  while IFS= read -r name; do
    [[ -n "$name" ]] || continue
    if verify_remote_snapshot "$name"; then
      protected_remote["$name"]=1
      verified_count="$((verified_count + 1))"
      (( verified_count >= required_protected )) && break
    else
      uncertain_remote["$name"]=1
      echo "retention: preserving unverified off-site snapshot for manual review: ${name}" >&2
    fi
  done < <(printf '%s\n' "${remote_snapshots[@]}" | sort -r)
  (( verified_count == required_protected )) || \
    backup_die "fewer than ${required_protected} off-site snapshots passed full hash verification"

  remaining="$verified_count"
  for name in "${remote_snapshots[@]}"; do
    [[ -z "${protected_remote[$name]:-}" ]] || continue
    [[ -z "${uncertain_remote[$name]:-}" ]] || continue
    eligible_for_prune "$name" "$OFFSITE_RETENTION_DAYS" || continue
    remote_target="${RCLONE_REMOTE%/}/$name"
    if [[ "$PRUNE_DRY_RUN" == "1" ]]; then
      echo "retention dry-run: off-site ${remote_target}"
    else
      rclone purge "$remote_target"
      echo "retention: removed off-site ${remote_target}"
    fi
  done
fi
