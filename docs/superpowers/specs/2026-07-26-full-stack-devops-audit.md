# Full-stack devops / quality audit

**Date:** 2026-07-26  
**Repo:** `avtotest` @ post–B2B M5 (`551c51d` then audit-fix SHA below)  
**Scope:** Whole frontend + backend (not B2B-only). Excludes `.worktrees/`, `node_modules/`.  
**Constraints honored:** no seed wipe, no exam redesign, no force push / `--no-verify`.

## Verdict

**Gates green** after small audit fixes. Honest leftovers listed at the end (product/ops, not failing CI gates).

## Gate matrix

| # | Gate | Result | Notes |
|---|------|--------|-------|
| 1 | `go test ./internal/...` (`-p 1`) | **PASS** | All internal packages OK (~4.5 min) |
| 1b | `go test ./cmd/... ./internal/server/` | **PASS** | |
| 2 | `golangci-lint run` (via `$GOPATH/bin`) | **PASS** | Fixed 1× staticcheck in `metrics.go`. Note: bare `make lint` fails if `golangci-lint` not on PATH (binary lives in `~/go/bin`) |
| 3a | `tsc --noEmit` | **PASS** | Fixed Button `variant="default"` (invalid) + null banner dismiss |
| 3b | `npm run lint` (next lint) | **PASS** | |
| 3c | Vitest full suite | **PASS** | 82 files / 348 tests |
| 4 | Migration up/down integrity | **PASS** | 36/36 pairs; versions 1–36 contiguous; no orphans |
| 5a | `go build ./cmd/api` | **PASS** | |
| 5b | `npm run build` (frontend) | **PASS** | |
| 6 | Secrets hygiene | **PASS** | Only `*.env.example` tracked; `backend/.env` / `frontend/.env` gitignored |
| 7 | Broken imports / recent-wave bugs | **PASS** | CSV proxy passthrough narrowed (no `text/plain`); profile invite/teacher cards tolerate non-array `apiGet` mocks |

## Fixes landed in this audit (SHA)

| SHA | What |
|-----|------|
| *(this commit)* | `metrics.go` staticcheck; Button variants on admin providers/flags; support-banner null-safe dismiss; proxy CSV-only passthrough (restore 502 on malformed JSON); harden teacher/invites cards for non-array API payloads |

**Prior B2B M5 completion (Phase 1):** `551c51d` — invite/enroll, teacher write, stats+CSV, admin completeness, U-40 inventory.

Related B2B history (for handoff):

| SHA | Summary |
|-----|---------|
| `ddfe4aa` | U-40 org/seats/admin grant foundation |
| `cc230a0` | Teacher read portal stub |
| `551c51d` | M5 invite + write + stats/CSV + admin completeness |

## Commands used

```bash
# BE
TEST_DATABASE_URL=postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable \
  go test -p 1 ./internal/... -count=1
golangci-lint run   # PATH includes $(go env GOPATH)/bin
go build -o /tmp/avtotest-api ./cmd/api

# FE
npx tsc --noEmit -p tsconfig.json
npm run lint
npx vitest run
npm run build

# Migrations
# 36 up ↔ 36 down, ids 0001…0036, no gaps/orphans

# Secrets
git ls-files | rg '\.env'   # only *.example
```

## Remaining known issues (honest; not gate failures)

1. **`make lint` PATH:** `golangci-lint` installed under `~/go/bin` but not always on shell PATH → document/export or pin Makefile to `$(shell go env GOPATH)/bin/golangci-lint`.
2. **B2B seat sales / self-serve checkout** still open (U-40 done-enough; needs a school customer).
3. **U-02 / U-03:** real staging host + production Payme/Click merchant keys not inventable here.
4. **U-41:** metrics + optional Sentry present; no tracing / Grafana / pager.
5. **U-43:** Dependabot + `govulncheck` gate; FE Next 14 / next-intl major bumps deferred.
6. **U-39:** full offline exam/session/image cache still large/open.
7. **Arena Redis multi-instance / practice bot** deferred (U-48/U-49).

## STOP

Per orders: audit report pushed and gates green (or leftovers listed). **Do not auto-start next product feature.** Parent decides next.
