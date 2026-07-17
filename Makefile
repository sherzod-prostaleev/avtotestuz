COMPOSE := docker compose
TEST_DATABASE_URL ?= postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable

.PHONY: up down test lint generate seed run check

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

run:
	cd backend && go run ./cmd/api

check: lint test
