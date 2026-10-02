# Makefile for SolveSpace Go port

.PHONY: help
help:
	@echo "SolveSpace Go Build Targets:"
	@echo ""
	@echo "Development:"
	@echo "  make test         - Run all tests"
	@echo "  make test-gui     - Run the GUI backend tests under Xvfb"
	@echo "  make test-race    - Run the engine packages' tests under the race detector"
	@echo "  make coverage     - Run all tests with coverage, and apply the 85% rule"
	@echo "  make fuzz         - Run each fuzz target for FUZZTIME ($(FUZZTIME))"
	@echo "  make ci           - Everything the GitHub workflow runs"
	@echo "  make fmt          - Format code"
	@echo "  make fmt-check    - Fail if gofmt would change a tracked file"
	@echo "  make vet          - Run go vet"
	@echo "  make tidy         - Tidy go.mod"
	@echo "  make lint         - Run linter"
	@echo "  make lint-new     - Run linter on what changed since LINT_BASE ($(LINT_BASE))"
	@echo "  make generate     - Regenerate what scripts can import (pkg/script/symbols)"
	@echo "  make generate-check - Fail if that is out of date"
	@echo ""
	@echo "Application:"
	@echo "  make build        - Build bin/spacesolve and bin/spacesolve-cli"
	@echo "  make install      - Install it with its desktop entry, icons and MIME type"
	@echo "                      (PREFIX=$(PREFIX), DESTDIR for staging)"
	@echo "  make uninstall    - Remove what install put in place"
	@echo ""

.PHONY: test test-gui test-race coverage fuzz ci fmt fmt-check vet tidy lint lint-new generate generate-check build install uninstall

PREFIX ?= /usr/local
BINDIR  = $(PREFIX)/bin
DATADIR = $(PREFIX)/share
FREEDESKTOP = solvespace/res/freedesktop
ICON_SIZES = 16x16 24x24 32x32 48x48

test:
	@echo "==> go test ./..."
	GOCACHE=$(PWD)/.gocache go test ./... -v

# The GUI backend's tests open windows, so they run under Xvfb.
test-gui:
	@echo "==> GUI tests under Xvfb"
	GOCACHE=$(PWD)/.gocache SPACESOLVE_GUI_TESTS=1 xvfb-run -a -s "-screen 0 1280x1024x24 -noreset" \
		go test ./pkg/platform/tkgui/... -v

# The packages whose loops run on several goroutines, and what loads
# and regenerates through them.
RACE_PACKAGES = ./pkg/srf/... ./pkg/sketch/... ./pkg/solver/... ./pkg/persist/...

test-race:
	@echo "==> go test -race $(RACE_PACKAGES)"
	GOCACHE=$(PWD)/.gocache go test -race -timeout 30m $(RACE_PACKAGES)

# Every test, with a coverage profile over ./pkg/...; fails when a
# package listed in scripts/coverage-packages.txt is under 85%.
coverage:
	@echo "==> tests with coverage"
	GOCACHE=$(PWD)/.gocache scripts/coverage-check.sh

# The fuzz targets, as package:target; go test fuzzes one at a time.
FUZZTIME ?= 30s
FUZZ_TARGETS = ./pkg/core:FuzzExpr ./pkg/persist/slvs:FuzzLoad

fuzz:
	@for t in $(FUZZ_TARGETS); do \
		pkg=$${t%%:*}; name=$${t##*:}; \
		echo "==> $$name in $$pkg for $(FUZZTIME)"; \
		GOCACHE=$(PWD)/.gocache go test $$pkg -run '^$$' -fuzz "^$$name\$$" -fuzztime $(FUZZTIME) || exit 1; \
	done

# What .github/workflows/ci.yml runs. The existing code is not
# lint-clean, so only what changed since LINT_BASE is linted.
ci: fmt-check vet build generate-check coverage test-race test-gui lint-new

fmt-check:
	@echo "==> gofmt -l"
	@unformatted=$$(gofmt -l $$(git ls-files '*.go')); \
	if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi

vet:
	@echo "==> go vet ./..."
	GOCACHE=$(PWD)/.gocache go vet ./...

fmt:
	@echo "==> gofmt"
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

tidy:
	@echo "==> go mod tidy"
	go mod tidy

lint:
	@echo "==> golangci-lint run"
	GOCACHE=$(PWD)/.gocache GOLANGCI_LINT_CACHE=$(PWD)/.golangci-lint golangci-lint run ./...

LINT_BASE ?= HEAD

lint-new:
	@echo "==> golangci-lint run --new-from-rev=$(LINT_BASE)"
	GOCACHE=$(PWD)/.gocache GOLANGCI_LINT_CACHE=$(PWD)/.golangci-lint golangci-lint run --new-from-rev=$(LINT_BASE) ./...

# The tables of what a script can import are generated from the
# packages; they change when what those export does.
generate:
	@echo "==> go generate ./pkg/script/..."
	GOCACHE=$(PWD)/.gocache go generate ./pkg/script/...

generate-check: generate
	git diff --exit-code -- pkg/script/symbols

build:
	@echo "==> go build ./cmd/spacesolve ./cmd/spacesolve-cli"
	GOCACHE=$(PWD)/.gocache go build -o bin/spacesolve ./cmd/spacesolve
	GOCACHE=$(PWD)/.gocache go build -o bin/spacesolve-cli ./cmd/spacesolve-cli

# The desktop entry, icons and MIME type follow solvespace/res/CMakeLists.txt.
install: build
	install -Dm755 bin/spacesolve $(DESTDIR)$(BINDIR)/spacesolve
	install -Dm755 bin/spacesolve-cli $(DESTDIR)$(BINDIR)/spacesolve-cli
	sed 's|@BINDIR@|$(BINDIR)|' dist/freedesktop/spacesolve.desktop.in > bin/spacesolve.desktop
	install -Dm644 bin/spacesolve.desktop $(DESTDIR)$(DATADIR)/applications/spacesolve.desktop
	install -Dm644 dist/freedesktop/spacesolve-mime.xml $(DESTDIR)$(DATADIR)/mime/packages/spacesolve-mime.xml
	install -Dm644 $(FREEDESKTOP)/solvespace-scalable.svg \
		$(DESTDIR)$(DATADIR)/icons/hicolor/scalable/apps/spacesolve.svg
	install -Dm644 $(FREEDESKTOP)/solvespace-scalable.svg \
		$(DESTDIR)$(DATADIR)/icons/hicolor/scalable/mimetypes/application-x-solvespace.svg
	for s in $(ICON_SIZES); do \
		install -Dm644 $(FREEDESKTOP)/solvespace-$$s.png \
			$(DESTDIR)$(DATADIR)/icons/hicolor/$$s/apps/spacesolve.png; \
		install -Dm644 $(FREEDESKTOP)/solvespace-$$s.png \
			$(DESTDIR)$(DATADIR)/icons/hicolor/$$s/mimetypes/application-x-solvespace.png; \
	done

uninstall:
	rm -f $(DESTDIR)$(BINDIR)/spacesolve $(DESTDIR)$(BINDIR)/spacesolve-cli
	rm -f $(DESTDIR)$(DATADIR)/applications/spacesolve.desktop
	rm -f $(DESTDIR)$(DATADIR)/mime/packages/spacesolve-mime.xml
	rm -f $(DESTDIR)$(DATADIR)/icons/hicolor/scalable/apps/spacesolve.svg
	rm -f $(DESTDIR)$(DATADIR)/icons/hicolor/scalable/mimetypes/application-x-solvespace.svg
	for s in $(ICON_SIZES); do \
		rm -f $(DESTDIR)$(DATADIR)/icons/hicolor/$$s/apps/spacesolve.png \
			$(DESTDIR)$(DATADIR)/icons/hicolor/$$s/mimetypes/application-x-solvespace.png; \
	done
