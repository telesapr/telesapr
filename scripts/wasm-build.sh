#!/bin/sh
# Builds the browser version (cmd/telesapr-web, doc/dev-notes/browser.md)
# into a folder that any static file server can serve.
#
#   scripts/wasm-build.sh [out-dir]
#
# The program is about 40 MB. If wasm-opt (binaryen) is on PATH, or at
# $WASM_OPT, it is shrunk with it; if brotli is on PATH, a .br copy is
# written beside it (about a fifth of the size) for a server to send with
# Content-Encoding: br. scripts/wasm-serve.py is such a server for trying
# it locally. A host that compresses by itself needs neither copy.
set -eu
out=${1:-bin/web}
mkdir -p "$out"
GOOS=js GOARCH=wasm go build -trimpath -ldflags=-s -o "$out/telesapr-web.wasm" ./cmd/telesapr-web
opt=${WASM_OPT:-$(command -v wasm-opt || true)}
if [ -n "$opt" ]; then
	# The flags are the features Go's wasm output uses.
	"$opt" -Oz --enable-bulk-memory --enable-sign-ext --enable-nontrapping-float-to-int \
		--enable-mutable-globals "$out/telesapr-web.wasm" -o "$out/telesapr-web.opt.wasm"
	mv "$out/telesapr-web.opt.wasm" "$out/telesapr-web.wasm"
fi
if command -v brotli >/dev/null 2>&1; then
	brotli -q 11 -f -k "$out/telesapr-web.wasm"
fi
# wasm_exec.js is read-only in GOROOT, so install it rather than copy it over the last one.
install -m 644 "$(go env GOROOT)/lib/wasm/wasm_exec.js" cmd/telesapr-web/web/index.html cmd/telesapr-web/web/memfs.js "$out/"
ls -l "$out"/telesapr-web.wasm*
