#!/usr/bin/env bash
# Read-only SQLite integrity drill for the Humo durable ingest spool.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "$SCRIPT_DIR/lib.sh"

SQLITE_DB="${1:-}"
[[ -n "$SQLITE_DB" ]] || backup_die "usage: humo_restore_drill.sh HUMO_SQLITE_DB"
[[ -f "$SQLITE_DB" && ! -L "$SQLITE_DB" && -s "$SQLITE_DB" ]] || backup_die "SQLite backup missing, empty, or unsafe: ${SQLITE_DB}"
require_command python3
require_command sha256sum

source_digest_before="$(sha256sum -- "$SQLITE_DB" | awk '{ print $1 }')"

python3 - "$SQLITE_DB" <<'PY'
from pathlib import Path
import sqlite3
import sys

path = Path(sys.argv[1]).resolve(strict=True)
uri = path.as_uri() + "?mode=ro&immutable=1"
db = sqlite3.connect(uri, uri=True, timeout=10)
try:
    result = db.execute("PRAGMA integrity_check").fetchone()
    if not result or result[0] != "ok":
        raise SystemExit(f"humo drill: integrity_check failed: {result!r}")
    table = db.execute(
        "SELECT 1 FROM sqlite_master WHERE type='table' AND name='pending_ingest'"
    ).fetchone()
    if table is None:
        raise SystemExit("humo drill: pending_ingest table is missing")
    columns = [row[1] for row in db.execute("PRAGMA table_info(pending_ingest)")]
    expected_columns = ["msg_id", "raw_text", "created_at"]
    if columns != expected_columns:
        raise SystemExit(
            "humo drill: pending_ingest schema mismatch: "
            f"expected {expected_columns!r}, got {columns!r}"
        )
    pending = db.execute("SELECT count(*) FROM pending_ingest").fetchone()[0]
    db.execute(
        "SELECT msg_id, raw_text, created_at "
        "FROM pending_ingest ORDER BY created_at, msg_id LIMIT 1"
    ).fetchone()
finally:
    db.close()

print(f"humo drill: sqlite integrity_check ok (pending={pending})")
print("humo_mode=sqlite-read-only-integrity-check")
print("humo_integrity_check=ok")
print("humo_schema_check=ok")
print(f"humo_pending_count={pending}")
PY

source_digest_after="$(sha256sum -- "$SQLITE_DB" | awk '{ print $1 }')"
[[ "$source_digest_after" == "$source_digest_before" ]] || \
  backup_die "source Humo SQLite database changed during read-only drill"
echo "humo_source_unchanged=1"
