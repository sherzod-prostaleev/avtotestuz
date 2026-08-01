#!/usr/bin/env bash
# U-44 — Postgres logical backup via pg_dump (local compose / operator laptop).
# Does NOT invent remote hosts. Point DATABASE_URL / compose at what you run.
#
# Usage:
#   ./scripts/backup/pg_dump.sh
#   BACKUP_DIR=./.run/backups COMPOSE_SERVICE=postgres ./scripts/backup/pg_dump.sh
#   DATABASE_URL=postgres://avtotest:avtotest@localhost:5432/avtotest ./scripts/backup/pg_dump.sh
#
# Output: $BACKUP_DIR/avtotest-YYYYMMDD-HHMMSS.dump (custom format -Fc)
set -euo pipefail
umask 077

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

BACKUP_DIR="${BACKUP_DIR:-$ROOT/.run/backups}"
COMPOSE="${COMPOSE:-docker compose}"
COMPOSE_SERVICE="${COMPOSE_SERVICE:-postgres}"
PGUSER="${PGUSER:-avtotest}"
PGDATABASE="${PGDATABASE:-avtotest}"
STAMP="$(date -u +%Y%m%d-%H%M%S)"
OUT_BASE="${BACKUP_DIR}/avtotest-${STAMP}.dump"
AGE_RECIPIENT="${AGE_RECIPIENT:-}"
REQUIRE_ENCRYPTED_BACKUP="${REQUIRE_ENCRYPTED_BACKUP:-0}"
RCLONE_REMOTE="${RCLONE_REMOTE:-}"
REQUIRE_OFFSITE_BACKUP="${REQUIRE_OFFSITE_BACKUP:-0}"

if [[ -z "$AGE_RECIPIENT" && "$REQUIRE_ENCRYPTED_BACKUP" == "1" ]]; then
  echo "AGE_RECIPIENT is required when REQUIRE_ENCRYPTED_BACKUP=1" >&2
  exit 1
fi
if [[ -n "$AGE_RECIPIENT" ]] && ! command -v age >/dev/null 2>&1; then
  echo "age is required for encrypted backups" >&2
  exit 1
fi
if [[ -z "$RCLONE_REMOTE" && "$REQUIRE_OFFSITE_BACKUP" == "1" ]]; then
  echo "RCLONE_REMOTE is required when REQUIRE_OFFSITE_BACKUP=1" >&2
  exit 1
fi
if [[ -n "$RCLONE_REMOTE" ]] && ! command -v rclone >/dev/null 2>&1; then
  echo "rclone is required for off-site backups" >&2
  exit 1
fi

OUT="$OUT_BASE"
if [[ -n "$AGE_RECIPIENT" ]]; then
  OUT="${OUT_BASE}.age"
fi

mkdir -p "$BACKUP_DIR"

dump_stream() {
  if [[ -n "${DATABASE_URL:-}" ]]; then
    if ! command -v pg_dump >/dev/null 2>&1; then
      echo "pg_dump is required when DATABASE_URL is used" >&2
      return 1
    fi
    pg_dump --format=custom --no-owner --no-acl --dbname="$DATABASE_URL" --file=-
  else
    $COMPOSE exec -T "$COMPOSE_SERVICE" \
      pg_dump -U "$PGUSER" -d "$PGDATABASE" --format=custom --no-owner --no-acl
  fi
}

echo "==> pg_dump → ${OUT}"
if [[ -n "$AGE_RECIPIENT" ]]; then
  dump_stream | age --recipient "$AGE_RECIPIENT" --output "$OUT"
else
  dump_stream >"$OUT"
fi

BYTES="$(wc -c <"$OUT" | tr -d ' ')"
if [[ "$BYTES" -lt 100 ]]; then
  echo "backup looks empty (${BYTES} bytes) — is Postgres up?" >&2
  rm -f "$OUT"
  exit 1
fi

# Keep a moving "latest" pointer (same directory; overwrite symlink/copy).
LATEST="${BACKUP_DIR}/avtotest-latest.dump"
if [[ -n "$AGE_RECIPIENT" ]]; then
  LATEST="${LATEST}.age"
fi
ln -sfn "$(basename "$OUT")" "$LATEST" 2>/dev/null || cp -f "$OUT" "$LATEST"
sha256sum "$OUT" >"${OUT}.sha256"

if [[ -n "$RCLONE_REMOTE" ]]; then
  rclone copyto "$OUT" "${RCLONE_REMOTE%/}/$(basename "$OUT")"
  rclone copyto "${OUT}.sha256" "${RCLONE_REMOTE%/}/$(basename "${OUT}.sha256")"
  echo "off-site copy: ok → ${RCLONE_REMOTE%/}/$(basename "$OUT")"
fi

echo "backup: ok (${BYTES} bytes) → ${OUT}"
echo "$OUT"
