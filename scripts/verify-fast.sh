#!/usr/bin/env bash
# Every local gate, in parallel, with a timing table at the end.
#
# Why this exists: the gates used to run one after another against the durable
# dev database (backend alone took ~27 minutes). Nothing here depends on
# anything else except Playwright, which waits for tsc so the dev server is not
# rewriting .next while the type-check reads it.
#
#   scripts/verify-fast.sh            # everything
#   scripts/verify-fast.sh backend    # only backend gates
#   scripts/verify-fast.sh frontend   # only frontend gates (incl. e2e)
#   SKIP_E2E=1 scripts/verify-fast.sh # skip Playwright
#
# Tunables: GO_P (packages in parallel, default = cores), PW_WORKERS (default 8).
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCOPE="${1:-all}"
CORES="$(nproc)"
GO_P="${GO_P:-$CORES}"
PW_WORKERS="${PW_WORKERS:-8}"
LOGS="$(mktemp -d "${TMPDIR:-/tmp}/verify-fast.XXXXXX")"
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"

PGTEST_NAME="avtotest-pgtest"
PGTEST_PORT=5433
export TEST_DATABASE_URL="${TEST_DATABASE_URL:-postgres://avtotest:avtotest@localhost:${PGTEST_PORT}/avtotest_test?sslmode=disable}"

# The test database is throwaway on purpose: tmpfs + fsync off. Every test
# TRUNCATEs ~60 tables, and waiting for the disk on each one was the whole cost.
# Never point this at the dev database on 5432.
ensure_pgtest() {
  if docker ps --format '{{.Names}}' | grep -qx "$PGTEST_NAME"; then return 0; fi
  if docker ps -a --format '{{.Names}}' | grep -qx "$PGTEST_NAME"; then
    docker start "$PGTEST_NAME" >/dev/null
  else
    docker run -d --name "$PGTEST_NAME" --restart unless-stopped \
      --tmpfs /var/lib/postgresql/data:rw,size=12g --shm-size=1g \
      -e POSTGRES_USER=avtotest -e POSTGRES_PASSWORD=avtotest -e POSTGRES_DB=avtotest_test \
      -p "127.0.0.1:${PGTEST_PORT}:5432" \
      postgres:16-alpine \
      -c fsync=off -c synchronous_commit=off -c full_page_writes=off \
      -c max_connections=400 -c shared_buffers=1GB -c max_wal_size=4GB >/dev/null
  fi
  for _ in $(seq 1 30); do
    docker exec "$PGTEST_NAME" pg_isready -U avtotest -q 2>/dev/null && return 0
    sleep 1
  done
  echo "verify-fast: $PGTEST_NAME did not become ready" >&2
  return 1
}

NAMES=()
run() { # run <name> <dir> <command...>
  local name="$1" dir="$2"; shift 2
  NAMES+=("$name")
  (
    cd "$dir" || exit 97
    local start; start=$(date +%s)
    "$@" >"$LOGS/$name.log" 2>&1
    echo "$? $(( $(date +%s) - start ))" >"$LOGS/$name.rc"
  ) &
}

T0=$(date +%s)

if [[ "$SCOPE" == all || "$SCOPE" == backend ]]; then
  ensure_pgtest || exit 1
  run go-test   "$ROOT/backend" go test -p "$GO_P" ./... -count=1
  run go-lint   "$ROOT/backend" golangci-lint run ./...
fi

if [[ "$SCOPE" == all || "$SCOPE" == frontend ]]; then
  rm -rf "$ROOT/frontend/.next"   # a stale .next gives false type errors
  run fe-tsc    "$ROOT/frontend" npx tsc --noEmit
  run fe-lint   "$ROOT/frontend" npx eslint .
  run fe-vitest "$ROOT/frontend" npx vitest run
  if [[ -z "${SKIP_E2E:-}" ]]; then
    NAMES+=("fe-e2e")
    (
      cd "$ROOT/frontend" || exit 97
      while [[ ! -f "$LOGS/fe-tsc.rc" ]]; do sleep 1; done
      start=$(date +%s)
      CI=true PORT="${PORT:-3112}" PW_WORKERS="$PW_WORKERS" npx playwright test >"$LOGS/fe-e2e.log" 2>&1
      echo "$? $(( $(date +%s) - start ))" >"$LOGS/fe-e2e.rc"
    ) &
  fi
fi

wait

# next dev rewrites this tracked file on every run; it is not a real change.
git -C "$ROOT" restore frontend/next-env.d.ts 2>/dev/null || true

FAILED=0
printf '\n%-10s %-6s %6s\n' "gate" "result" "time"
for name in "${NAMES[@]}"; do
  if [[ -f "$LOGS/$name.rc" ]]; then read -r rc secs <"$LOGS/$name.rc"; else rc=98; secs=0; fi
  if [[ "$rc" == 0 ]]; then res="ok"; else res="FAIL"; FAILED=1; fi
  printf '%-10s %-6s %5ss\n' "$name" "$res" "$secs"
done
printf '%-10s %-6s %5ss\n' "TOTAL" "" "$(( $(date +%s) - T0 ))"

if [[ "$FAILED" != 0 ]]; then
  for name in "${NAMES[@]}"; do
    [[ -f "$LOGS/$name.rc" ]] && read -r rc _ <"$LOGS/$name.rc" || rc=98
    if [[ "$rc" != 0 ]]; then
      printf '\n===== %s (exit %s) — last 40 lines of %s\n' "$name" "$rc" "$LOGS/$name.log"
      tail -40 "$LOGS/$name.log"
    fi
  done
  exit 1
fi
echo "logs: $LOGS"
