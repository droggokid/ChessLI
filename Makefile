.PHONY: help run build test fmt vet tidy generate mockgen

help:
	@echo "Available targets:"
	@echo "  make run    - run the app"
	@echo "  make build  - build all packages"
	@echo "  make test   - run tests"
	@echo "  make fmt    - format Go code"
	@echo "  make vet    - run go vet"
	@echo "  make tidy   - tidy go.mod/go.sum"
	@echo "  make generate - run go generate"
	@echo "  make mockgen  - print mockgen version"

run:
	go run ./cmd

build:
	go build ./...

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

tidy:
	go mod tidy

generate:
	go generate ./...

mockgen:
	go run go.uber.org/mock/mockgen@v0.6.0 -version
