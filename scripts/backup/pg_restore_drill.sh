#!/usr/bin/env bash
# PostgreSQL restore DRILL. The only permitted target is the hard-coded scratch
# database avtotest_restore_drill; live/application database names are rejected.
set -euo pipefail
umask 077

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "$SCRIPT_DIR/lib.sh"

ROOT="$(repo_root)"
cd "$ROOT"

POSTGRES_SERVICE="${POSTGRES_SERVICE:-postgres}"
PGUSER="${PGUSER:-avtotest}"
PGDATABASE="${PGDATABASE:-avtotest}"
DRILL_DB="${DRILL_DB:-avtotest_restore_drill}"
DUMP="${1:-$ROOT/.run/backups/avtotest-latest.dump}"
RESTORE_DRILL_ACK="${RESTORE_DRILL_ACK:-}"
KEEP_DRILL_DB="${KEEP_DRILL_DB:-0}"
AGE_IDENTITY_FILE="${AGE_IDENTITY_FILE:-}"

require_bool KEEP_DRILL_DB "$KEEP_DRILL_DB"
validate_service_name "$POSTGRES_SERVICE"
[[ "$PGUSER" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || backup_die "unsafe PGUSER: ${PGUSER}"
[[ "$PGDATABASE" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || backup_die "unsafe PGDATABASE: ${PGDATABASE}"

# This allowlist is intentionally not configurable through environment. A typo
# or hostile env file therefore cannot turn the drill into a live restore.
[[ "$DRILL_DB" == "avtotest_restore_drill" ]] || \
  backup_die "DRILL_DB is not the hard-coded scratch database: ${DRILL_DB}"
case "$DRILL_DB" in
  avtotest|postgres|template0|template1|"$PGDATABASE")
    backup_die "refusing protected PostgreSQL database name: ${DRILL_DB}"
    ;;
esac
[[ "$RESTORE_DRILL_ACK" == "avtotest_restore_drill" ]] || \
  backup_die "set RESTORE_DRILL_ACK=avtotest_restore_drill to acknowledge scratch-only restore"
[[ -f "$DUMP" && ! -L "$DUMP" && -s "$DUMP" ]] || \
  backup_die "dump missing, empty, or unsafe: ${DUMP}"

if [[ -f "${DUMP}.sha256" ]]; then
  require_command sha256sum
  expected_hash="$(awk 'NR == 1 { print $1 } NR > 1 { extra=1 } END { if (extra) exit 1 }' "${DUMP}.sha256")" || \
    backup_die "checksum sidecar must contain exactly one line"
  [[ "$expected_hash" =~ ^[0-9a-fA-F]{64}$ ]] || backup_die "checksum sidecar contains an invalid digest"
  actual_hash="$(sha256sum -- "$DUMP" | awk '{ print $1 }')"
  [[ "${actual_hash,,}" == "${expected_hash,,}" ]] || backup_die "dump checksum mismatch"
fi

if [[ "$DUMP" == *.age ]]; then
  require_command age
  [[ -n "$AGE_IDENTITY_FILE" && -f "$AGE_IDENTITY_FILE" && ! -L "$AGE_IDENTITY_FILE" ]] || \
    backup_die "AGE_IDENTITY_FILE must be a regular, non-symlink file for an encrypted dump"
fi

drop_drill_database() {
  compose_cmd exec -T "$POSTGRES_SERVICE" \
    psql -U "$PGUSER" -d postgres -v ON_ERROR_STOP=1 \
      -v drill_db="$DRILL_DB" <<'SQL'
SELECT pg_terminate_backend(pid)
  FROM pg_stat_activity
 WHERE datname = :'drill_db' AND pid <> pg_backend_pid();
SELECT format('DROP DATABASE IF EXISTS %I', :'drill_db') \gexec
SQL
}

create_drill_database() {
  compose_cmd exec -T "$POSTGRES_SERVICE" \
    psql -U "$PGUSER" -d postgres -v ON_ERROR_STOP=1 \
      -v drill_db="$DRILL_DB" -v drill_owner="$PGUSER" <<'SQL'
SELECT format('CREATE DATABASE %I OWNER %I', :'drill_db', :'drill_owner') \gexec
SQL
}

cleanup() {
  if [[ "$KEEP_DRILL_DB" != "1" && "$DRILL_DB_CREATED" == "1" ]]; then
    if ! drop_drill_database; then
      echo "ERROR: failed to remove scratch database ${DRILL_DB}" >&2
      return 1
    fi
    DRILL_DB_CREATED=0
  fi
}

echo "==> PostgreSQL restore drill: ${DUMP} -> ${DRILL_DB}"
echo "    protected live DB: ${PGDATABASE}"
DRILL_DB_CREATED=1
trap cleanup EXIT INT TERM
drop_drill_database
create_drill_database

if [[ "$DUMP" == *.age ]]; then
  age --decrypt --identity "$AGE_IDENTITY_FILE" "$DUMP" | \
    compose_cmd exec -T "$POSTGRES_SERVICE" \
      pg_restore -U "$PGUSER" -d "$DRILL_DB" \
        --no-owner --no-acl --exit-on-error
else
  compose_cmd exec -T "$POSTGRES_SERVICE" \
    pg_restore -U "$PGUSER" -d "$DRILL_DB" \
      --no-owner --no-acl --exit-on-error <"$DUMP"
fi

read -r TABLES CORE_TABLES PROFILE_ROWS QUESTION_ROWS < <(
  compose_cmd exec -T "$POSTGRES_SERVICE" \
    psql -U "$PGUSER" -d "$DRILL_DB" -At -F ' ' -v ON_ERROR_STOP=1 <<'SQL'
SELECT
  (SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public'),
  (SELECT count(*) FROM (VALUES
     (to_regclass('public.schema_migrations')),
     (to_regclass('public.profile')),
     (to_regclass('public.question'))
   ) AS required(rel) WHERE rel IS NOT NULL),
  (SELECT count(*) FROM profile),
  (SELECT count(*) FROM question);
SQL
)

[[ "$TABLES" =~ ^[0-9]+$ && "$TABLES" -gt 0 ]] || backup_die "restore produced no public tables"
[[ "$CORE_TABLES" == "3" ]] || backup_die "restore is missing schema_migrations/profile/question"

if [[ "$KEEP_DRILL_DB" != "1" ]]; then
  drop_drill_database
  DRILL_DB_CREATED=0
  trap - EXIT INT TERM
fi

echo "postgres drill: ok (tables=${TABLES}, profiles=${PROFILE_ROWS}, questions=${QUESTION_ROWS})"
if [[ "$KEEP_DRILL_DB" == "1" ]]; then
  echo "postgres drill scratch retained: ${DRILL_DB}"
fi
