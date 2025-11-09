# Makefile for SolveSpace Go port

.PHONY: help
help:
	@echo "SolveSpace Go Build Targets:"
	@echo ""
	@echo "UI Applications:"
	@echo "  make all          - Build all applications (spacesolve + tests)"
	@echo "  make spacesolve   - Build main application"
	@echo "  make run          - Build and run spacesolve"
	@echo ""
	@echo "Development:"
	@echo "  make test         - Run all tests"
	@echo "  make fmt          - Format code"
	@echo "  make tidy         - Tidy go.mod"
	@echo "  make lint         - Run linter"
	@echo ""
	@echo "Utilities:"
	@echo "  make clean-ui     - Remove UI binaries"
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

# UI build targets (GTK3 + OpenGL)
BUILD_FLAGS = -tags gtk_3_18
BIN_DIR = bin

.PHONY: all
all: spacesolve

.PHONY: spacesolve
spacesolve:
	@mkdir -p $(BIN_DIR)
	@echo "==> Building spacesolve"
	@go build $(BUILD_FLAGS) -o $(BIN_DIR)/spacesolve ./cmd/spacesolve


.PHONY: run
run: spacesolve
	@echo "==> Running spacesolve"
	@./$(BIN_DIR)/spacesolve

.PHONY: clean-ui
clean-ui:
	@echo "==> Cleaning UI binaries"
	@rm -f $(BIN_DIR)/spacesolve
