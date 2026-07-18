.PHONY: help run build test fmt fmt-check vet check test-race cover tidy clean generate mockgen

help:
	@echo "Available targets:"
	@echo "  make run    - run the app"
	@echo "  make build  - build all packages"
	@echo "  make test   - run tests"
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

run:
	go run ./cmd

build:
	@mkdir -p bin
	go build -o bin/chessli ./cmd

test:
	go test ./...

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
