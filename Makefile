# Makefile for TeleSAPR

.PHONY: help
help:
	@echo "TeleSAPR Build Targets:"
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
	@echo "  make tidy-check   - Fail if go.mod or go.sum is not tidy"
	@echo "  make vuln         - Run govulncheck"
	@echo "  make lint         - Run linter"
	@echo "  make lint-new     - Run linter on what changed since LINT_BASE ($(LINT_BASE))"
	@echo "  make generate     - Regenerate what scripts can import (pkg/script/symbols)"
	@echo "  make generate-check - Fail if that is out of date"
	@echo ""
	@echo "Application:"
	@echo "  make build        - Build bin/telesapr and bin/telesapr-cli"
	@echo "  make install      - Install it with its desktop entry, icons and MIME type"
	@echo "                      (PREFIX=$(PREFIX), DESTDIR for staging)"
	@echo "  make uninstall    - Remove what install put in place"
	@echo "  make clean        - Remove bin/ and coverage.out"
	@echo "  make distclean    - clean, plus the Go build, test and module caches"
	@echo ""

.PHONY: test test-gui test-race coverage fuzz ci fmt fmt-check vet tidy tidy-check vuln lint lint-new generate generate-check build install uninstall clean distclean

PREFIX ?= /usr/local
BINDIR  = $(PREFIX)/bin
DATADIR = $(PREFIX)/share
FREEDESKTOP = dist/freedesktop
ICON_SIZES = 16x16 24x24 32x32 48x48

test:
	@echo "==> go test ./..."
	GOCACHE=$(PWD)/.gocache go test ./...

# The GUI backend's tests open windows, so they run under Xvfb.
test-gui:
	@echo "==> GUI tests under Xvfb"
	GOCACHE=$(PWD)/.gocache TELESAPR_GUI_TESTS=1 xvfb-run -a -s "-screen 0 1280x1024x24 -noreset" \
		go test ./pkg/platform/tkgui/... ./pkg/render/gl/... -v

# The packages whose loops run on several goroutines, what loads and
# regenerates through them, and the servers and platforms that hand
# work between goroutines. pkg/ui and pkg/script/api take over ten
# minutes each under the detector and wait for WP2 of the review plan
# (docs/adr, 2026-10-07), which lets their tests run in parallel.
RACE_PACKAGES = ./pkg/srf/... ./pkg/sketch/... ./pkg/solver/... ./pkg/persist/... \
	./pkg/script ./pkg/script/agent ./pkg/mcpserver ./pkg/platform/... ./pkg/render ./pkg/render/soft ./cmd/... \
	./pkg/core ./pkg/geom ./pkg/libm ./pkg/internal/... ./pkg/check ./pkg/thread ./pkg/res ./pkg/ttf ./pkg/topo

# The slow packages' tests run as several processes side by side
# (scripts/go-test.sh), and the fixture loops take a sample under the
# race detector (raceSample in pkg/persist/slvs).
test-race:
	@echo "==> go test -race $(RACE_PACKAGES)"
	GOCACHE=$(PWD)/.gocache PACKAGES='$(RACE_PACKAGES)' scripts/go-test.sh -race -timeout 30m

# Every test, with a coverage profile over ./pkg/...; fails when a
# package listed in scripts/coverage-packages.txt is under 85%. The
# OpenGL packages are only reached with an X server, so it runs under
# Xvfb, with the GUI tests, where xvfb-run is installed; its display
# numbers start apart from test-gui's, which make ci runs beside it.
XVFB_COVERAGE := $(if $(shell command -v xvfb-run),xvfb-run -a -n 140 -s "-screen 0 1280x1024x24 -noreset" env TELESAPR_GUI_TESTS=1)

coverage:
	@echo "==> tests with coverage"
	GOCACHE=$(PWD)/.gocache $(XVFB_COVERAGE) scripts/coverage-check.sh

# The fuzz targets, as package:target; go test fuzzes one at a time.
FUZZTIME ?= 30s
FUZZ_TARGETS = ./pkg/core:FuzzExpr ./pkg/persist/slvs:FuzzLoad ./pkg/persist/tlspr:FuzzRead ./pkg/persist/step:FuzzStep

fuzz:
	@for t in $(FUZZ_TARGETS); do \
		pkg=$${t%%:*}; name=$${t##*:}; \
		echo "==> $$name in $$pkg for $(FUZZTIME)"; \
		GOCACHE=$(PWD)/.gocache go test $$pkg -run '^$$' -fuzz "^$$name\$$" -fuzztime $(FUZZTIME) || exit 1; \
	done

# What .github/workflows/ci.yml runs, the whole tree linted.
# The checks that rewrite or read the sources first; then the rest side
# by side, each target's output printed when it is done.
ci: fmt-check vet tidy-check generate-check
	@$(MAKE) --no-print-directory -j --output-sync=target build coverage test-race test-gui lint vuln

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

tidy-check:
	@echo "==> go mod tidy -diff"
	go mod tidy -diff

# govulncheck is pinned in go.mod's tool block; it reports only the
# vulnerabilities the binaries can reach.
vuln:
	@echo "==> govulncheck ./..."
	GOCACHE=$(PWD)/.gocache go tool govulncheck ./...

lint:
	@echo "==> golangci-lint run"
	GOCACHE=$(PWD)/.gocache GOLANGCI_LINT_CACHE=$(PWD)/.golangci-lint golangci-lint run ./...

# Where the branch left master, so that a branch's commits are linted
# and not only what is uncommitted; on master that is HEAD. A quicker
# check while working; make ci lints everything.
LINT_BASE ?= $(shell git merge-base HEAD master 2>/dev/null || echo HEAD)

lint-new:
	@echo "==> golangci-lint run --new-from-rev=$(LINT_BASE)"
	GOCACHE=$(PWD)/.gocache GOLANGCI_LINT_CACHE=$(PWD)/.golangci-lint golangci-lint run --new-from-rev=$(LINT_BASE) ./...

# The tables of what a script can import are generated from the
# packages; they change when what those export does.
# scripts/generate-symbols.sh runs the //go:generate lines side by
# side; `go generate ./pkg/script/...` gives the same files, slower.
generate:
	@echo "==> go generate ./pkg/script/..."
	GOCACHE=$(PWD)/.gocache scripts/generate-symbols.sh

generate-check: generate
	git diff --exit-code -- pkg/script/symbols

build:
	@echo "==> go build ./cmd/telesapr ./cmd/telesapr-cli"
	GOCACHE=$(PWD)/.gocache go build -o bin/telesapr ./cmd/telesapr
	GOCACHE=$(PWD)/.gocache go build -o bin/telesapr-cli ./cmd/telesapr-cli

# The desktop entry, icons and MIME type, from dist/freedesktop; the
# layout follows SolveSpace's res/CMakeLists.txt.
install: build
	install -Dm755 bin/telesapr $(DESTDIR)$(BINDIR)/telesapr
	install -Dm755 bin/telesapr-cli $(DESTDIR)$(BINDIR)/telesapr-cli
	sed 's|@BINDIR@|$(BINDIR)|' dist/freedesktop/telesapr.desktop.in > bin/telesapr.desktop
	install -Dm644 bin/telesapr.desktop $(DESTDIR)$(DATADIR)/applications/telesapr.desktop
	install -Dm644 dist/freedesktop/telesapr-mime.xml $(DESTDIR)$(DATADIR)/mime/packages/telesapr-mime.xml
	install -Dm644 $(FREEDESKTOP)/telesapr.svg \
		$(DESTDIR)$(DATADIR)/icons/hicolor/scalable/apps/telesapr.svg
	install -Dm644 $(FREEDESKTOP)/telesapr.svg \
		$(DESTDIR)$(DATADIR)/icons/hicolor/scalable/mimetypes/application-x-telesapr.svg
	for s in $(ICON_SIZES); do \
		install -Dm644 $(FREEDESKTOP)/telesapr-$$s.png \
			$(DESTDIR)$(DATADIR)/icons/hicolor/$$s/apps/telesapr.png; \
		install -Dm644 $(FREEDESKTOP)/telesapr-$$s.png \
			$(DESTDIR)$(DATADIR)/icons/hicolor/$$s/mimetypes/application-x-telesapr.png; \
	done

uninstall:
	rm -f $(DESTDIR)$(BINDIR)/telesapr $(DESTDIR)$(BINDIR)/telesapr-cli
	rm -f $(DESTDIR)$(DATADIR)/applications/telesapr.desktop
	rm -f $(DESTDIR)$(DATADIR)/mime/packages/telesapr-mime.xml
	rm -f $(DESTDIR)$(DATADIR)/icons/hicolor/scalable/apps/telesapr.svg
	rm -f $(DESTDIR)$(DATADIR)/icons/hicolor/scalable/mimetypes/application-x-telesapr.svg
	for s in $(ICON_SIZES); do \
		rm -f $(DESTDIR)$(DATADIR)/icons/hicolor/$$s/apps/telesapr.png \
			$(DESTDIR)$(DATADIR)/icons/hicolor/$$s/mimetypes/application-x-telesapr.png; \
	done

# Build output only; dist/ holds tracked sources and .gocache is kept.
clean:
	rm -rf bin coverage.out

# clean, and the caches: the project's .gocache and .golangci-lint, and
# Go's default build, test and fuzz caches. The module cache is kept.
distclean: clean
	rm -rf .gocache .golangci-lint
	go clean -cache -testcache -fuzzcache
