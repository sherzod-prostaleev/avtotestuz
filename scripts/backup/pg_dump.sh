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

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

BACKUP_DIR="${BACKUP_DIR:-$ROOT/.run/backups}"
COMPOSE="${COMPOSE:-docker compose}"
COMPOSE_SERVICE="${COMPOSE_SERVICE:-postgres}"
PGUSER="${PGUSER:-avtotest}"
PGDATABASE="${PGDATABASE:-avtotest}"
STAMP="$(date -u +%Y%m%d-%H%M%S)"
OUT="${BACKUP_DIR}/avtotest-${STAMP}.dump"

mkdir -p "$BACKUP_DIR"

if [[ -n "${DATABASE_URL:-}" ]]; then
  echo "==> pg_dump via DATABASE_URL → ${OUT}"
  # Prefer local pg_dump if present; else use compose postgres image client.
  if command -v pg_dump >/dev/null 2>&1; then
    pg_dump --format=custom --no-owner --no-acl --dbname="$DATABASE_URL" --file="$OUT"
  else
    echo "pg_dump not on PATH; using ${COMPOSE_SERVICE} container with localhost URL unsupported." >&2
    echo "Install postgresql-client or unset DATABASE_URL to dump via compose exec." >&2
    exit 1
  fi
else
  echo "==> pg_dump via ${COMPOSE} exec ${COMPOSE_SERVICE} → ${OUT}"
  # Stream custom-format dump from the running compose Postgres (no fake host).
  $COMPOSE exec -T "$COMPOSE_SERVICE" \
    pg_dump -U "$PGUSER" -d "$PGDATABASE" --format=custom --no-owner --no-acl \
    >"$OUT"
fi

BYTES="$(wc -c <"$OUT" | tr -d ' ')"
if [[ "$BYTES" -lt 100 ]]; then
  echo "backup looks empty (${BYTES} bytes) — is Postgres up?" >&2
  rm -f "$OUT"
  exit 1
fi

# Keep a moving "latest" pointer (same directory; overwrite symlink/copy).
ln -sfn "$(basename "$OUT")" "${BACKUP_DIR}/avtotest-latest.dump" 2>/dev/null \
  || cp -f "$OUT" "${BACKUP_DIR}/avtotest-latest.dump"

echo "backup: ok (${BYTES} bytes) → ${OUT}"
echo "$OUT"
