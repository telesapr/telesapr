# Makefile for TeleSAPR

.PHONY: help
help:
	@echo "TeleSAPR Build Targets:"
	@echo ""
	@echo "Development:"
	@echo "  make test-fast    - Run the quick tests (go test -short)"
	@echo "  make test         - Run the tests except the heavy ones"
	@echo "  make test-heavy   - Run only the heavy tests (HEAVY in scripts/go-test.sh)"
	@echo "  make test-sweeps  - Run the robustness sweeps over their whole grids"
	@echo "  make bench        - Run every benchmark once"
	@echo "  make test-gui     - Run the GUI backend tests under Xvfb"
	@echo "  make test-race    - Run the engine packages' tests under the race detector"
	@echo "  make coverage     - Run all tests with coverage, and apply the 85% rule"
	@echo "  make fuzz         - Run each fuzz target for FUZZTIME ($(FUZZTIME))"
	@echo "  make checks       - fmt-check, tidy-check, generate-check and build"
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
	@echo "Release (doc/releasing.md):"
	@echo "  make dist         - The packages for this OS into bin/dist (version $(VERSION))"
	@echo "  make dist-linux   - tar.gz, .deb and .rpm (needs the X11 development packages)"
	@echo "  make dist-windows - zips, cross-compiled; the installers where Inno Setup is"
	@echo "  make dist-macos   - a universal TeleSAPR.app in a .dmg (on macOS only)"
	@echo "  make release-notes - Print what the release of v$(VERSION) would say"
	@echo "  make release      - Push master, wait for CI, tag v$(VERSION), make the GitHub release"
	@echo "  make deploy-site  - Copy site/ to telesapr.sourceforge.io (SF_USER=<user>)"
	@echo ""

.PHONY: test test-heavy test-fast test-sweeps bench test-gui test-race coverage fuzz checks ci fmt fmt-check vet tidy tidy-check vuln lint lint-new generate generate-check build install uninstall clean distclean \
	dist dist-linux dist-windows dist-macos release release-notes

# The version is pkg/version.Version; a release's tag is v$(VERSION).
VERSION := $(shell sed -n 's/^const Version = "\(.*\)"$$/\1/p' pkg/version/version.go)

PREFIX ?= /usr/local
BINDIR  = $(PREFIX)/bin
DATADIR = $(PREFIX)/share
FREEDESKTOP = dist/freedesktop
ICON_SIZES = 16x16 24x24 32x32 48x48

# The tiers: test-fast while working (the -short tests, under a minute
# on a workstation), test for the tests but the heavy ones, test-heavy for
# those, ci for what the GitHub workflow runs;
# the nightly workflow adds test-sweeps, fuzz, bench and the race detector
# over pkg/ui and pkg/script/api. All of them go through scripts/go-test.sh,
# which picks the light or the heavy part, as CI does.
# Every test, the OpenGL and GUI ones under Xvfb where xvfb-run is
# installed; their display numbers start apart from test-gui's, which
# make ci runs beside it.
# make ci runs test and test-heavy at once: each searches from a number of
# its own, since two xvfb-run -a started together can pick the same display.
XVFB = $(if $(shell command -v xvfb-run),xvfb-run -a -n $(XVFB_N) -s "-screen 0 1280x1024x24 -noreset" env TELESAPR_GUI_TESTS=1)
XVFB_N = 140

# test leaves out the HEAVY tests of scripts/go-test.sh, which
# test-heavy runs alone; make ci runs both, on two runners in the workflow.
test:
	@echo "==> go test ./... (without the heavy tests)"
	GOCACHE=$(PWD)/.gocache $(XVFB) env TESTS=light scripts/go-test.sh

test-heavy: XVFB_N = 170
test-heavy:
	@echo "==> go test, the heavy tests only"
	GOCACHE=$(PWD)/.gocache $(XVFB) env TESTS=heavy scripts/go-test.sh

# A test that takes more than a few seconds skips itself under -short
# (testing.Short), so this is what is quick enough before every commit;
# the Windows and macOS jobs run it too.
test-fast:
	@echo "==> go test -short ./..."
	GOCACHE=$(PWD)/.gocache scripts/go-test.sh -short

# The robustness sweeps over their whole grids (pkg/internal/sweep),
# rather than the sample the ordinary run takes; the packages are those
# with sweeps, and their other tests run too.
SWEEP_PACKAGES = ./pkg/script/api ./pkg/sketch ./pkg/srf ./pkg/topo

test-sweeps:
	@echo "==> TELESAPR_SWEEP=1 go test $(SWEEP_PACKAGES)"
	GOCACHE=$(PWD)/.gocache TELESAPR_SWEEP=1 PACKAGES='$(SWEEP_PACKAGES)' scripts/go-test.sh -timeout 180m

# Every benchmark once: they are run so that they keep working, not
# timed; the nightly workflow keeps the output.
bench:
	@echo "==> go test -bench . -benchtime 1x ./..."
	GOCACHE=$(PWD)/.gocache go test -run '^$$' -bench . -benchtime 1x ./...

# The GUI backend's tests open windows, so they run under Xvfb.
test-gui:
	@echo "==> GUI tests under Xvfb"
	GOCACHE=$(PWD)/.gocache TELESAPR_GUI_TESTS=1 xvfb-run -a -s "-screen 0 1280x1024x24 -noreset" \
		go test ./pkg/platform/tkgui/... ./pkg/render/gl/... -v

# The packages whose loops run on several goroutines, what loads and
# regenerates through them, and the servers and platforms that hand
# work between goroutines: every package with tests. The GitHub
# workflow runs the set in shards, each a RACE_PACKAGES of its own, on
# a push to master without pkg/ui and pkg/script/api, which the nightly
# workflow runs.
RACE_PACKAGES ?= ./pkg/srf/... ./pkg/sketch/... ./pkg/solver/... ./pkg/persist/... \
	./pkg/script ./pkg/script/agent ./pkg/mcpserver ./pkg/platform/... ./pkg/render ./pkg/render/soft ./cmd/... \
	./pkg/core ./pkg/geom ./pkg/libm ./pkg/internal/... ./pkg/check ./pkg/thread ./pkg/res ./pkg/ttf ./pkg/topo \
	./pkg/ui ./pkg/script/api

# The fixture loops take a sample under the race detector
# (fixtures.Sample in pkg/internal/fixtures).
# RACE_TIMEOUT bounds each go test process: pkg/script/api's
# TestFilletOnOtherSolidsVolumes alone takes 14 minutes here under the
# detector, and twice that on a runner, so the nightly workflow gives it
# more.
RACE_TIMEOUT ?= 30m

test-race:
	@echo "==> go test -race $(RACE_PACKAGES)"
	GOCACHE=$(PWD)/.gocache PACKAGES='$(RACE_PACKAGES)' scripts/go-test.sh -race -timeout $(RACE_TIMEOUT)

# Every test, with a coverage profile over ./pkg/...; fails when a
# package listed in scripts/coverage-packages.txt is under 85%. The
# instrumentation makes it several times slower than test, so the
# nightly workflow runs it, not make ci. The OpenGL packages are only
# reached with an X server, so it runs under Xvfb as test does.
coverage:
	@echo "==> tests with coverage"
	GOCACHE=$(PWD)/.gocache $(XVFB) scripts/coverage-check.sh

# The fuzz targets, as package:target; go test fuzzes one at a time.
FUZZTIME ?= 30s
FUZZ_TARGETS = ./pkg/core:FuzzExpr ./pkg/persist/slvs:FuzzLoad ./pkg/persist/tlspr:FuzzRead ./pkg/persist/step:FuzzStep

fuzz:
	@for t in $(FUZZ_TARGETS); do \
		pkg=$${t%%:*}; name=$${t##*:}; \
		echo "==> $$name in $$pkg for $(FUZZTIME)"; \
		GOCACHE=$(PWD)/.gocache go test $$pkg -run '^$$' -fuzz "^$$name\$$" -fuzztime $(FUZZTIME) || exit 1; \
	done

# The quick checks: those that rewrite or read the sources, then the
# build. The GitHub workflow runs them as one job beside the tests and
# the lint. govulncheck is not among them: a new advisory against the
# standard library or a dependency fails it with no commit at all, so
# the workflow runs vuln as a job of its own that only warns, and the
# nightly and Release workflows as one that fails.
checks: fmt-check tidy-check generate-check build

# What .github/workflows/ci.yml runs on a push to master, in one go:
# the checks, then the rest side by side, each target's output printed
# when it is done. govet is in the lint; the GUI packages run under
# Xvfb inside test. On a branch the workflow leaves out test-race,
# which it runs on master and on request.
ci: checks
	@$(MAKE) --no-print-directory -j --output-sync=target test test-heavy test-race lint vuln

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

# golangci-lint is pinned in tools/lint/go.mod, a module of its own so
# that its dependencies stay out of ours, and built from there into bin/;
# the GitHub workflow runs the same binary. It is built with go.mod's
# toolchain: one older than the code's cannot read the export data of
# the standard library it type-checks against.
GOLANGCI_LINT = bin/golangci-lint
LINT_ENV = GOCACHE=$(PWD)/.gocache GOLANGCI_LINT_CACHE=$(PWD)/.golangci-lint

$(GOLANGCI_LINT): go.mod tools/lint/go.mod tools/lint/go.sum
	GOCACHE=$(PWD)/.gocache GOTOOLCHAIN=$$(go env GOVERSION) go build -C tools/lint -o $(PWD)/$(GOLANGCI_LINT) \
		github.com/golangci/golangci-lint/v2/cmd/golangci-lint

lint: $(GOLANGCI_LINT)
	@echo "==> golangci-lint run"
	$(LINT_ENV) $(GOLANGCI_LINT) run ./...

# Where the branch left master, so that a branch's commits are linted
# and not only what is uncommitted; on master that is HEAD. A quicker
# check while working; make ci lints everything.
LINT_BASE ?= $(shell git merge-base HEAD master 2>/dev/null || echo HEAD)

lint-new: $(GOLANGCI_LINT)
	@echo "==> golangci-lint run --new-from-rev=$(LINT_BASE)"
	$(LINT_ENV) $(GOLANGCI_LINT) run --new-from-rev=$(LINT_BASE) ./...

# The tables of what a script can import are generated from the
# packages; they change when what those export does.
# scripts/generate-symbols.sh runs the //go:generate lines side by
# side; `go generate ./pkg/script/...` gives the same files, slower.
generate:
	@echo "==> go generate ./pkg/script/..."
	GOCACHE=$(PWD)/.gocache scripts/generate-symbols.sh

generate-check: generate
	git diff --exit-code -- pkg/script/symbols

# LDFLAGS is empty for a development build; the release scripts set it
# to -s -w, and -trimpath through GOFLAGS.
LDFLAGS ?=

build:
	@echo "==> go build ./cmd/telesapr ./cmd/telesapr-cli"
	GOCACHE=$(PWD)/.gocache go build -ldflags '$(LDFLAGS)' -o bin/telesapr ./cmd/telesapr
	GOCACHE=$(PWD)/.gocache go build -ldflags '$(LDFLAGS)' -o bin/telesapr-cli ./cmd/telesapr-cli

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

# The packages a release carries, into bin/dist: each script builds the
# binaries for its OS with -trimpath and without symbols, then wraps
# them as doc/releasing.md lists. dist picks the script for this host.
DIST_OUT = $(PWD)/bin/dist
DIST_ENV = VERSION=$(VERSION) OUT=$(DIST_OUT) GOCACHE=$(PWD)/.gocache

dist:
	@case "$$(uname -s)" in \
		Linux) $(MAKE) --no-print-directory dist-linux dist-windows ;; \
		Darwin) $(MAKE) --no-print-directory dist-macos ;; \
		MINGW*|MSYS*) $(MAKE) --no-print-directory dist-windows ;; \
		*) echo "make dist: no packaging for $$(uname -s)"; exit 1 ;; \
	esac

dist-linux:
	@echo "==> Linux packages for $(VERSION)"
	$(DIST_ENV) scripts/dist-linux.sh

dist-windows:
	@echo "==> Windows packages for $(VERSION)"
	$(DIST_ENV) scripts/dist-windows.sh

dist-macos:
	@echo "==> macOS package for $(VERSION)"
	$(DIST_ENV) scripts/dist-macos.sh

# The release of v$(VERSION): scripts/release.sh pushes master, waits
# for CI on it, tags, pushes the tag and creates the GitHub release;
# the Release workflow then attaches the packages. release-notes only
# prints what the release would say.
release:
	scripts/release.sh

release-notes:
	scripts/release.sh --notes

# The web site in site/, to the project's web space on SourceForge.
deploy-site:
	scripts/deploy-site.sh

# Build output only; dist/ holds tracked sources and .gocache is kept.
clean:
	rm -rf bin coverage.out

# clean, and the caches: the project's .gocache and .golangci-lint, and
# Go's default build, test and fuzz caches. The module cache is kept.
distclean: clean
	rm -rf .gocache .golangci-lint
	go clean -cache -testcache -fuzzcache
