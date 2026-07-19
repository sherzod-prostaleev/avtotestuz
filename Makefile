COMPOSE := docker compose
TEST_DATABASE_URL ?= postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable

.PHONY: up down test lint generate seed seed-real run check

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
seed-real:
	cd backend && go run ./cmd/convertavtoimtihon -src "$(AAA_SRC)" -out seed/avtoimtihon && go run ./cmd/importer -data seed/avtoimtihon -verified
AAA_SRC ?= /home/sher/Рабочий стол/aaa

run:
	cd backend && go run ./cmd/api

check: lint test
