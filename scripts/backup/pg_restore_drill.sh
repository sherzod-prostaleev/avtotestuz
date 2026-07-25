#!/usr/bin/env bash
# U-44 — Restore DRILL into a disposable database (never the live app DB).
#
# Default target: avtotest_restore_drill on the local compose Postgres.
# This is intentionally destructive ONLY to that drill DB.
#
# Usage:
#   ./scripts/backup/pg_restore_drill.sh [.run/backups/avtotest-latest.dump]
#   DRILL_DB=avtotest_restore_drill ./scripts/backup/pg_restore_drill.sh path/to.dump
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

COMPOSE="${COMPOSE:-docker compose}"
COMPOSE_SERVICE="${COMPOSE_SERVICE:-postgres}"
PGUSER="${PGUSER:-avtotest}"
PGDATABASE="${PGDATABASE:-avtotest}"
DRILL_DB="${DRILL_DB:-avtotest_restore_drill}"
DUMP="${1:-$ROOT/.run/backups/avtotest-latest.dump}"

if [[ "$DRILL_DB" == "$PGDATABASE" ]]; then
  echo "refusing to restore-drill into live database name '${PGDATABASE}'" >&2
  exit 1
fi
if [[ ! -f "$DUMP" ]]; then
  echo "dump not found: ${DUMP}" >&2
  echo "run ./scripts/backup/pg_dump.sh first" >&2
  exit 1
fi

echo "==> restore drill: ${DUMP} → ${DRILL_DB} (via ${COMPOSE_SERVICE})"
echo "    live DB '${PGDATABASE}' will NOT be modified"

$COMPOSE exec -T "$COMPOSE_SERVICE" psql -U "$PGUSER" -d postgres -v ON_ERROR_STOP=1 <<SQL
SELECT pg_terminate_backend(pid)
  FROM pg_stat_activity
 WHERE datname = '${DRILL_DB}' AND pid <> pg_backend_pid();
DROP DATABASE IF EXISTS ${DRILL_DB};
CREATE DATABASE ${DRILL_DB} OWNER ${PGUSER};
SQL

# Feed dump on stdin to pg_restore inside the container.
$COMPOSE exec -T "$COMPOSE_SERVICE" \
  pg_restore -U "$PGUSER" -d "$DRILL_DB" --no-owner --no-acl --exit-on-error \
  <"$DUMP"

# Sanity: migrations table or a core relation exists.
TABLES="$($COMPOSE exec -T "$COMPOSE_SERVICE" \
  psql -U "$PGUSER" -d "$DRILL_DB" -Atc \
  "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'")"
echo "restore drill: public tables=${TABLES}"
if [[ "${TABLES}" -lt 1 ]]; then
  echo "restore drill produced zero public tables" >&2
  exit 1
fi

echo "restore drill: ok (database ${DRILL_DB})"
echo "drop when done: ${COMPOSE} exec -T ${COMPOSE_SERVICE} psql -U ${PGUSER} -d postgres -c 'DROP DATABASE IF EXISTS ${DRILL_DB};'"
