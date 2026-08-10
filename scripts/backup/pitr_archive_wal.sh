#!/usr/bin/env bash
# PostgreSQL archive_command helper. It is intentionally inert until an
# operator explicitly enables it in a root-owned host environment file.
set -euo pipefail
umask 077

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "$SCRIPT_DIR/lib.sh"

SOURCE="${1:-}"
WAL_NAME="${2:-}"
PITR_ARCHIVE_ENABLED="${PITR_ARCHIVE_ENABLED:-0}"
PITR_WAL_ALLOW_PLAINTEXT="${PITR_WAL_ALLOW_PLAINTEXT:-0}"
PITR_WAL_ARCHIVE_ROOT="${PITR_WAL_ARCHIVE_ROOT:-}"
AGE_RECIPIENT="${AGE_RECIPIENT:-}"

[[ "$PITR_ARCHIVE_ENABLED" == "1" ]] || \
  backup_die "PITR archiving is disabled; set PITR_ARCHIVE_ENABLED=1 only after operator approval"
require_bool PITR_WAL_ALLOW_PLAINTEXT "$PITR_WAL_ALLOW_PLAINTEXT"
[[ -n "$PITR_WAL_ARCHIVE_ROOT" ]] || backup_die "PITR_WAL_ARCHIVE_ROOT is required"
[[ -n "$SOURCE" && -n "$WAL_NAME" ]] || backup_die "usage: pitr_archive_wal.sh SOURCE WAL_NAME"
[[ -f "$SOURCE" && ! -L "$SOURCE" && -s "$SOURCE" ]] || \
  backup_die "WAL source must be a non-empty regular non-symlink file"
validate_pitr_wal_name "$WAL_NAME"
validate_pitr_wal_archive_root "$PITR_WAL_ARCHIVE_ROOT"
require_command sha256sum

if [[ -z "$AGE_RECIPIENT" ]]; then
  [[ "$PITR_WAL_ALLOW_PLAINTEXT" == "1" ]] || \
    backup_die "AGE_RECIPIENT is required unless PITR_WAL_ALLOW_PLAINTEXT=1 is explicitly approved"
  ENCRYPTION="none"
  ARCHIVE_NAME="$WAL_NAME"
else
  require_command age
  ENCRYPTION="age"
  ARCHIVE_NAME="${WAL_NAME}.age"
fi

ARCHIVE_PATH="$PITR_WAL_ARCHIVE_ROOT/$ARCHIVE_NAME"
COMPLETE_PATH="$PITR_WAL_ARCHIVE_ROOT/${WAL_NAME}.complete"
[[ ! -L "$ARCHIVE_PATH" && ! -L "$COMPLETE_PATH" ]] || \
  backup_die "refusing symlink in WAL archive destination"

if [[ -e "$COMPLETE_PATH" ]]; then
  "$SCRIPT_DIR/pitr_verify_wal.sh" "$PITR_WAL_ARCHIVE_ROOT" "$WAL_NAME" >/dev/null
  echo "PITR WAL archive already verified: ${WAL_NAME}"
  exit 0
fi
[[ ! -e "$ARCHIVE_PATH" ]] || \
  backup_die "archive payload exists without completion marker: ${ARCHIVE_PATH}"

PART="$PITR_WAL_ARCHIVE_ROOT/.partial-${WAL_NAME}.$$"
MARKER_PART="$PITR_WAL_ARCHIVE_ROOT/.partial-${WAL_NAME}.complete.$$"
cleanup() {
  rm -f -- "$PART" "$MARKER_PART"
}
trap cleanup EXIT INT TERM

if [[ "$ENCRYPTION" == "age" ]]; then
  age --recipient "$AGE_RECIPIENT" --output "$PART" "$SOURCE"
else
  cp -- "$SOURCE" "$PART"
fi
[[ -s "$PART" ]] || backup_die "archived WAL payload is empty"

DIGEST="$(sha256sum -- "$PART" | awk '{ print $1 }')"
[[ "$DIGEST" =~ ^[0-9a-f]{64}$ ]] || backup_die "could not calculate WAL archive digest"
cat >"$MARKER_PART" <<EOF
format=drivergo-pitr-wal-v1
wal_name=$WAL_NAME
payload=$ARCHIVE_NAME
sha256=$DIGEST
encryption=$ENCRYPTION
EOF

mv -- "$PART" "$ARCHIVE_PATH"
mv -- "$MARKER_PART" "$COMPLETE_PATH"
trap - EXIT INT TERM
echo "PITR WAL archive complete: ${WAL_NAME}"
