MIGRATIONS_DIR := internal/database/migrations

ifneq (,$(wildcard ./.env))
include .env
export
endif

.PHONY: help run build test test-stack fmt fmt-check vet check test-race cover tidy clean generate mockgen run-deps stop-deps clean-deps seed-local migration migrate-up migrate-down migrate-status sqlc

help:
	@echo "Available targets:"
	@echo "  make run    - run the app"
	@echo "  make build  - build all packages"
	@echo "  make test   - run tests"
	@echo "  make test-stack - run WebSocket stack tests with the race detector"
	@echo "  make fmt    - format Go code"
	@echo "  make fmt-check - verify Go formatting"
	@echo "  make vet    - run go vet"
	@echo "  make check  - run formatting, vet, and tests"
	@echo "  make test-race - run tests with the race detector"
	@echo "  make cover  - report test coverage"
	@echo "  make tidy   - tidy go.mod/go.sum"
	@echo "  make clean  - clean Go build and test caches"
	@echo "  make generate - run go generate"
	@echo "  make mockgen  - print mockgen version"
	@echo "  make run-deps  - start PostgreSQL"
	@echo "  make stop-deps - stop PostgreSQL"
	@echo "  make clean-deps - stop PostgreSQL and remove its data"
	@echo "  make seed-local - add local placeholder database records"
	@echo "  make migration name=... - create a SQL migration"
	@echo "  make migrate-up - apply migrations"
	@echo "  make migrate-down - revert the latest migration"
	@echo "  make migrate-status - show migration status"

run:
	go run ./cmd

build:
	@mkdir -p bin
	go build -o bin/chessli ./cmd

test:
	go test ./...

test-stack:
	go test -count=1 -race ./internal/websocket -run '^TestWebSocketStack'

fmt:
	go fmt ./...

fmt-check:
	@test -z "$$(gofmt -l $$(find . -type f -name '*.go' -not -path './vendor/*'))" || (echo "Run 'make fmt' to format Go files."; exit 1)

vet:
	go vet ./...

check: fmt-check vet test

test-race:
	go test -race ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

tidy:
	go mod tidy

clean:
	go clean -cache -testcache
	rm -rf bin

generate:
	go generate ./...

mockgen:
	go run go.uber.org/mock/mockgen@v0.6.0 -version

run-deps:
	docker compose up -d --wait postgres

stop-deps:
	docker compose down

clean-deps:
	docker compose down -v

seed-local:
	sh scripts/seed-local.sh

migration:
	@test -n "$(name)" || (echo "usage: make migration name=create_users"; exit 1)
	goose -dir $(MIGRATIONS_DIR) create $(name) sql

migrate-up:
	goose -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" up

migrate-down:
	goose -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" down

migrate-status:
	goose -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" status

sqlc:
	mkdir -p internal/database/db
	sqlc generate
