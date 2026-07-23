COMPOSE := docker compose
TEST_DATABASE_URL ?= postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable

.PHONY: up down test lint generate seed seed-real validate-real run check

up:
	$(COMPOSE) up -d --wait

down:
	$(COMPOSE) down

# -p 1: DB test packages share one database and must not run in parallel
test:
	cd backend && TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -p 1 ./... -count=1

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
