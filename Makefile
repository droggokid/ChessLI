.PHONY: help run build test fmt vet tidy

help:
	@echo "Available targets:"
	@echo "  make run    - run the app"
	@echo "  make build  - build all packages"
	@echo "  make test   - run tests"
	@echo "  make fmt    - format Go code"
	@echo "  make vet    - run go vet"
	@echo "  make tidy   - tidy go.mod/go.sum"

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
