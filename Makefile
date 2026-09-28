# Makefile for SolveSpace Go port

.PHONY: help
help:
	@echo "SolveSpace Go Build Targets:"
	@echo ""
	@echo "Development:"
	@echo "  make test         - Run all tests"
	@echo "  make fmt          - Format code"
	@echo "  make tidy         - Tidy go.mod"
	@echo "  make lint         - Run linter"
	@echo ""

.PHONY: test fmt tidy lint

test:
	@echo "==> go test ./..."
	GOCACHE=$(PWD)/.gocache go test ./... -v

fmt:
	@echo "==> gofmt"
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

tidy:
	@echo "==> go mod tidy"
	go mod tidy

lint:
	@echo "==> golangci-lint run"
	GOCACHE=$(PWD)/.gocache GOLANGCI_LINT_CACHE=$(PWD)/.golangci-lint golangci-lint run ./...
