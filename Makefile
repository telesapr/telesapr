# Makefile for SolveSpace Go port

.PHONY: help
help:
	@echo "SolveSpace Go Build Targets:"
	@echo ""
	@echo "Development:"
	@echo "  make test         - Run all tests"
	@echo "  make test-gui     - Run the GUI backend tests under Xvfb"
	@echo "  make fmt          - Format code"
	@echo "  make tidy         - Tidy go.mod"
	@echo "  make lint         - Run linter"
	@echo ""
	@echo "Application:"
	@echo "  make build        - Build bin/spacesolve and bin/spacesolve-cli"
	@echo "  make install      - Install it with its desktop entry, icons and MIME type"
	@echo "                      (PREFIX=$(PREFIX), DESTDIR for staging)"
	@echo "  make uninstall    - Remove what install put in place"
	@echo ""

.PHONY: test test-gui fmt tidy lint build install uninstall

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

fmt:
	@echo "==> gofmt"
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

tidy:
	@echo "==> go mod tidy"
	go mod tidy

lint:
	@echo "==> golangci-lint run"
	GOCACHE=$(PWD)/.gocache GOLANGCI_LINT_CACHE=$(PWD)/.golangci-lint golangci-lint run ./...

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
