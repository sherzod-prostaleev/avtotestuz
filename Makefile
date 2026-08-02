COMPOSE := docker compose
TEST_DATABASE_URL ?= postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable

.PHONY: up down test test-parallel test-db-reset lint generate seed seed-real seed-admin validate-real run check \
	seed-verify extract-legal-refs seed-sync-legal-refs seed-import seed-signs seed-link-signs seed-reset-content seed-dev \
	fe-install fe-lint fe-typecheck fe-test fe-build fe-e2e fe-check dep-scan load-test \
	backup-pg backup-restore-drill tg-digest tg-digest-send

up:
	$(COMPOSE) up -d --wait

down:
	$(COMPOSE) down

# -p 1 is a resource choice, not a correctness one: internal/testdb gives each
# test package its own database, so a parallel run produces the same results —
# it just migrates and pools several databases at once. Use test-parallel when
# wall-clock matters more than load.
test:
	cd backend && TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -p 1 ./... -count=1

test-parallel:
	cd backend && TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test ./... -count=1

# Per-package test databases are reused across runs (re-migrating from scratch
# is the slow part). Drop them after changing a migration in place, or to
# reclaim disk.
test-db-reset:
	$(COMPOSE) exec -T postgres psql -U avtotest -d postgres -tAc \
		"SELECT 'DROP DATABASE IF EXISTS ' || quote_ident(datname) || ';' \
		 FROM pg_database WHERE datname LIKE 'avtotest\_test\_%'" \
	| $(COMPOSE) exec -T postgres psql -U avtotest -d postgres

lint:
	cd backend && golangci-lint run

generate:
	cd backend && sqlc generate

seed:
	cd backend && go run ./cmd/genfixture -out seed/sample && go run ./cmd/importer -data seed/sample -verified

# Local superadmin for /{locale}/admin (M3-0). Reads ADMIN_SEED_* from env /
# backend/.env — never commit a real password. See backend/.env.example.
seed-admin:
	cd backend && go run ./cmd/seedadmin

# Committed corpus parity gate (1260 Q / 63 bilets / 285 signs). Run after any
# convertavtoimtihon / gensigns regeneration before trusting wipe-restore.
seed-verify:
	python3 scripts/seed/verify-committed.py

# Fill explanation.legal_refs from inline YHQ/ПДД citations in blocks.
extract-legal-refs:
	python3 scripts/seed/extract_legal_refs.py

# Refresh explanation.legal_refs in an existing DB (safe with session history).
seed-sync-legal-refs: extract-legal-refs
	cd backend && go run ./cmd/importer -data seed/avtoimtihon -verified -explanations-only

# Import the COMMITTED avtoimtihon JSON (no aaa/ reconvert). Use this after a
# DB wipe so local matches git, not whatever is currently on disk in aaa/.
# Prefer `make seed-dev` after truncate; for legal_refs-only use seed-sync-legal-refs.
seed-import: seed-verify extract-legal-refs
	cd backend && go run ./cmd/importer -data seed/avtoimtihon -verified

# Real road-sign catalogue (7 groups / 285 signs). data.json may be regenerated
# with gensigns; images are vendored under seed/signs/images/.
seed-signs:
	@if [ ! -f backend/seed/signs/data.json ]; then \
		cd backend && go run ./cmd/gensigns -out seed/signs; \
	fi
	cd backend && go run ./cmd/importer -data seed/signs -verified

# Apply committed question↔sign links (requires questions + signs already imported).
seed-link-signs:
	cd backend && go run ./cmd/linkquestionsigns -links seed/avtoimtihon/question_signs.json

# Content-only truncate (keeps profiles/payments/admin/limit_config).
seed-reset-content:
	$(COMPOSE) exec -T postgres psql -U avtotest -d avtotest -v ON_ERROR_STOP=1 \
		-f - < scripts/seed/truncate-content.sql

# Full local content restore after wipe / drift: clean content → committed
# questions+bilets → signs → question_sign links → admin. CMS chrome/legal stay
# empty (FE i18n fallback) until an operator saves them again — intentional.
seed-dev: seed-reset-content seed-import seed-signs seed-link-signs seed-admin
	@echo "seed-dev complete: 1260 questions, 63 bilets, 285 signs, question↔sign links, admin user"

# Real, user-licensed avtoimtihon content. Regenerates canonical JSON from the
# aaa/ source tree (1260 questions / 63 bilets after NEW+MAJOR fold), then imports
# (upsert). Prefer `make seed-dev` for wipe-restore from git; use seed-real only
# when intentionally re-exporting from aaa/. Truncate first for a clean DB.
# NEVER run seed-dev / seed-reset-content on production — it CASCADE-wipes
# learner progress tied to content FKs. Prod = backup + upsert seed-import only.
# Checks that the three locale files still describe the same questions before
# a conversion is attempted. The converter joins them by id and stops at the
# first bad one; this lists every offender so the export can be repaired.
validate-real:
	cd backend && go run ./cmd/validateavtoimtihon -src "$(AAA_SRC)"

seed-real:
	cd backend && go run ./cmd/convertavtoimtihon -src "$(AAA_SRC)" -out seed/avtoimtihon -assignments seed/avtoimtihon/assignments.json -strict && go run ./cmd/importer -data seed/avtoimtihon -verified
	$(MAKE) seed-verify
AAA_SRC ?= /home/sher/Рабочий стол/aaa

run:
	cd backend && go run ./cmd/api

# M4-07: soft due digests to linked Telegram DMs (not groups). Groups use /quiz.
tg-digest:
	cd backend && go run ./cmd/tgdigest

tg-digest-send:
	cd backend && go run ./cmd/tgdigest -send

check: lint test

# Frontend (Next.js) — mirrors CI `frontend` / `e2e` jobs. Prefer these over
# ad-hoc `cd frontend && npm …` so local + docs stay aligned.
fe-install:
	cd frontend && npm ci

fe-lint:
	cd frontend && npm run lint

fe-typecheck:
	cd frontend && npm run typecheck

fe-test:
	cd frontend && npm run test

fe-build:
	cd frontend && npm run build

# Playwright Chromium smoke. Optional E2E_AUTH_TOKEN / E2E_REFRESH_TOKEN for
# session-gate shells (same as CI — never commit real JWTs).
fe-e2e:
	cd frontend && npm run test:e2e

fe-check: fe-lint fe-typecheck fe-test fe-build

# Dependency vulnerability scan (mirrors CI `dependency-scan` job / U-43).
# Go: hard fail on reachable vulns. npm: prints report; exit 0 locally so
# deferred Next/next-intl majors do not block day-to-day checks.
dep-scan:
	cd backend && govulncheck ./...
	cd frontend && (npm audit --audit-level=high || true)

# U-42 — k6 smoke against a running API (not a prod soak). Requires k6 on PATH.
# Defaults match ./run.sh API port; override API_BASE / VUS / DURATION as needed.
API_BASE ?= http://localhost:8090
VUS ?= 5
DURATION ?= 30s

load-test:
	@command -v k6 >/dev/null || { echo "k6 not found — install from https://k6.io/docs/get-started/installation/" >&2; exit 1; }
	API_BASE="$(API_BASE)" VUS="$(VUS)" DURATION="$(DURATION)" k6 run -e API_BASE="$(API_BASE)" -e VUS="$(VUS)" -e DURATION="$(DURATION)" deploy/load-test/smoke.js

# U-44 — Postgres logical dump + restore drill (local compose; no fake host).
backup-pg:
	./scripts/backup/pg_dump.sh

backup-restore-drill:
	./scripts/backup/pg_restore_drill.sh
