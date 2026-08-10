#!/usr/bin/env bash
# PostgreSQL-only compatibility helper for local/operator use.
# Production scheduling must use backup_all.sh so Redis, MinIO, and Humo are
# captured in the same managed snapshot.
set -euo pipefail
umask 077

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "$SCRIPT_DIR/lib.sh"

ROOT="$(repo_root)"
cd "$ROOT"

BACKUP_DIR="${BACKUP_DIR:-$ROOT/.run/backups}"
BACKUP_LOCK_FILE="${BACKUP_LOCK_FILE:-$BACKUP_DIR/.drivergo-pg-backup.lock}"
COMPOSE_SERVICE="${COMPOSE_SERVICE:-postgres}"
PGUSER="${PGUSER:-avtotest}"
PGDATABASE="${PGDATABASE:-avtotest}"
AGE_RECIPIENT="${AGE_RECIPIENT:-}"
REQUIRE_ENCRYPTED_BACKUP="${REQUIRE_ENCRYPTED_BACKUP:-0}"
RCLONE_REMOTE="${RCLONE_REMOTE:-}"
REQUIRE_OFFSITE_BACKUP="${REQUIRE_OFFSITE_BACKUP:-0}"

require_command flock
require_command sha256sum
require_bool REQUIRE_ENCRYPTED_BACKUP "$REQUIRE_ENCRYPTED_BACKUP"
require_bool REQUIRE_OFFSITE_BACKUP "$REQUIRE_OFFSITE_BACKUP"
validate_service_name "$COMPOSE_SERVICE"
[[ "$PGUSER" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || backup_die "unsafe PGUSER: ${PGUSER}"
[[ "$PGDATABASE" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || backup_die "unsafe PGDATABASE: ${PGDATABASE}"

if [[ -z "$AGE_RECIPIENT" ]]; then
  [[ "$REQUIRE_ENCRYPTED_BACKUP" == "0" ]] || \
    backup_die "AGE_RECIPIENT is required when REQUIRE_ENCRYPTED_BACKUP=1"
  echo "WARNING: PostgreSQL-only helper is writing a plaintext local dump" >&2
else
  require_command age
fi

if [[ -z "$RCLONE_REMOTE" ]]; then
  [[ "$REQUIRE_OFFSITE_BACKUP" == "0" ]] || \
    backup_die "RCLONE_REMOTE is required when REQUIRE_OFFSITE_BACKUP=1"
else
  require_command rclone
  validate_remote_base "$RCLONE_REMOTE"
fi

mkdir -p -m 0700 "$BACKUP_DIR"
BACKUP_DIR="$(realpath -e "$BACKUP_DIR")"
if [[ "$BACKUP_LOCK_FILE" != /* ]]; then
  BACKUP_LOCK_FILE="$ROOT/$BACKUP_LOCK_FILE"
fi
validate_lock_file "$BACKUP_LOCK_FILE"
exec 9>"$BACKUP_LOCK_FILE"
flock -n 9 || backup_die "another PostgreSQL backup holds ${BACKUP_LOCK_FILE}"

STAMP="$(date -u +%Y%m%d-%H%M%S)"
OUT_BASE="${BACKUP_DIR}/avtotest-${STAMP}.dump"
OUT="$OUT_BASE"
[[ -z "$AGE_RECIPIENT" ]] || OUT="${OUT_BASE}.age"
PART="${OUT}.partial.$$"
LATEST="${BACKUP_DIR}/avtotest-latest.dump"
[[ -z "$AGE_RECIPIENT" ]] || LATEST="${LATEST}.age"

[[ ! -e "$OUT" && ! -L "$OUT" ]] || backup_die "output already exists: ${OUT}"
[[ ! -e "$PART" && ! -L "$PART" ]] || backup_die "partial output already exists: ${PART}"
[[ ! -e "${OUT}.sha256" && ! -L "${OUT}.sha256" ]] || backup_die "checksum output already exists: ${OUT}.sha256"
if [[ -e "$LATEST" && ! -L "$LATEST" ]]; then
  backup_die "refusing to replace non-symlink latest path: ${LATEST}"
fi

cleanup_partial() {
  case "$PART" in
    "$BACKUP_DIR"/avtotest-*.dump.partial.*|"$BACKUP_DIR"/avtotest-*.dump.age.partial.*)
      rm -f -- "$PART"
      ;;
    *) echo "refusing unsafe PostgreSQL partial cleanup: ${PART}" >&2 ;;
  esac
}
trap cleanup_partial EXIT INT TERM

dump_stream() {
  if [[ -n "${DATABASE_URL:-}" ]]; then
    require_command pg_dump
    pg_dump --format=custom --no-owner --no-acl --dbname="$DATABASE_URL" --file=-
  else
    compose_cmd exec -T "$COMPOSE_SERVICE" \
      pg_dump -U "$PGUSER" -d "$PGDATABASE" \
        --format=custom --no-owner --no-acl
  fi
}

echo "==> pg_dump -> ${OUT}"
if [[ -n "$AGE_RECIPIENT" ]]; then
  dump_stream | age --recipient "$AGE_RECIPIENT" --output "$PART"
else
  dump_stream >"$PART"
fi

BYTES="$(wc -c <"$PART" | tr -d ' ')"
[[ "$BYTES" -ge 100 ]] || backup_die "backup looks empty (${BYTES} bytes); is PostgreSQL available?"
mv -- "$PART" "$OUT"
trap - EXIT INT TERM

sha256sum -- "$OUT" >"${OUT}.sha256"
ln -sfn -- "$(basename "$OUT")" "$LATEST"

if [[ -n "$RCLONE_REMOTE" ]]; then
  remote_dump="${RCLONE_REMOTE%/}/$(basename "$OUT")"
  remote_checksum="${RCLONE_REMOTE%/}/$(basename "${OUT}.sha256")"
  rclone copyto "$OUT" "$remote_dump" --checksum --immutable
  rclone copyto "${OUT}.sha256" "$remote_checksum" --checksum --immutable
  echo "off-site copy: ok -> ${remote_dump}"
fi

echo "backup: ok (${BYTES} bytes) -> ${OUT}"
echo "$OUT"
