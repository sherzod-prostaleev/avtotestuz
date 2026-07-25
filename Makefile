COMPOSE := docker compose
TEST_DATABASE_URL ?= postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable

.PHONY: up down test test-parallel test-db-reset lint generate seed seed-real validate-real run check

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

# Real, user-licensed avtoimtihon content (61 bilets, 1235 questions). Regenerates
# the canonical dataset from the source tree, then imports it (upsert; truncate the
# DB first for a clean real-only dev DB). Both seed/ dirs are gitignored.
# Checks that the three locale files still describe the same questions before
# a conversion is attempted. The converter joins them by id and stops at the
# first bad one; this lists every offender so the export can be repaired.
validate-real:
	cd backend && go run ./cmd/validateavtoimtihon -src "$(AAA_SRC)"

seed-real:
	cd backend && go run ./cmd/convertavtoimtihon -src "$(AAA_SRC)" -out seed/avtoimtihon -assignments seed/avtoimtihon/assignments.json -strict && go run ./cmd/importer -data seed/avtoimtihon -verified
AAA_SRC ?= /home/sher/Рабочий стол/aaa

run:
	cd backend && go run ./cmd/api

check: lint test
