#!/usr/bin/env bash
# Verify completed PostgreSQL WAL archive artifacts without contacting Docker.
set -euo pipefail
umask 077

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "$SCRIPT_DIR/lib.sh"

PITR_WAL_ARCHIVE_ROOT="${1:-}"
WAL_NAME="${2:-}"
[[ -n "$PITR_WAL_ARCHIVE_ROOT" ]] || \
  backup_die "usage: pitr_verify_wal.sh PITR_WAL_ARCHIVE_ROOT [WAL_NAME]"
validate_pitr_wal_archive_root "$PITR_WAL_ARCHIVE_ROOT"
require_command sha256sum
require_command find

verify_one() {
  local wal_name="$1"
  local complete payload digest encryption actual count
  validate_pitr_wal_name "$wal_name"
  complete="$PITR_WAL_ARCHIVE_ROOT/${wal_name}.complete"
  [[ -f "$complete" && ! -L "$complete" ]] || \
    backup_die "missing or unsafe WAL completion marker: ${wal_name}"

  count="$(awk -F= '$1 == "format" || $1 == "wal_name" || $1 == "payload" || $1 == "sha256" || $1 == "encryption" { seen[$1]++ } END { print (seen["format"] == 1 && seen["wal_name"] == 1 && seen["payload"] == 1 && seen["sha256"] == 1 && seen["encryption"] == 1) ? 1 : 0 }' "$complete")"
  [[ "$count" == "1" ]] || backup_die "invalid WAL completion marker: ${wal_name}"
  [[ "$(wc -l <"$complete" | tr -d ' ')" == "5" ]] || \
    backup_die "WAL completion marker has unexpected fields: ${wal_name}"
  [[ "$(manifest_value "$complete" format)" == "drivergo-pitr-wal-v1" ]] || \
    backup_die "unknown WAL completion marker format: ${wal_name}"
  [[ "$(manifest_value "$complete" wal_name)" == "$wal_name" ]] || \
    backup_die "WAL completion marker name mismatch: ${wal_name}"
  payload="$(manifest_value "$complete" payload)"
  digest="$(manifest_value "$complete" sha256)"
  encryption="$(manifest_value "$complete" encryption)"
  case "$encryption" in
    none) [[ "$payload" == "$wal_name" ]] ;;
    age) [[ "$payload" == "${wal_name}.age" ]] ;;
    *) backup_die "unsupported WAL archive encryption value: ${wal_name}" ;;
  esac
  [[ "$digest" =~ ^[0-9a-f]{64}$ ]] || backup_die "invalid WAL archive digest: ${wal_name}"
  [[ -f "$PITR_WAL_ARCHIVE_ROOT/$payload" && ! -L "$PITR_WAL_ARCHIVE_ROOT/$payload" ]] || \
    backup_die "missing or unsafe WAL archive payload: ${wal_name}"
  actual="$(sha256sum -- "$PITR_WAL_ARCHIVE_ROOT/$payload" | awk '{ print $1 }')"
  [[ "$actual" == "$digest" ]] || backup_die "WAL archive checksum mismatch: ${wal_name}"
}

if [[ -n "$WAL_NAME" ]]; then
  verify_one "$WAL_NAME"
  echo "PITR WAL verification: ok (${WAL_NAME})"
  exit 0
fi

mapfile -t markers < <(find "$PITR_WAL_ARCHIVE_ROOT" -mindepth 1 -maxdepth 1 -type f -name '*.complete' -printf '%f\n' | sort)
(( ${#markers[@]} > 0 )) || backup_die "no completed WAL archive artifacts found"
for marker in "${markers[@]}"; do
  wal_name="${marker%.complete}"
  verify_one "$wal_name"
done
echo "PITR WAL verification: ok (${#markers[@]} artifacts)"
