#!/bin/sh
# Builds the browser spike (cmd/telesapr-wasm, doc/notes/browser-wasm-spike.md)
# into a folder that any static file server can serve, with a model to show.
#
#   scripts/wasm-spike.sh [out-dir [model.tlspr]]
#
# Then, for example: python3 -m http.server -d <out-dir> 8080 and open
# http://localhost:8080/?frames=60. Under node, without a browser:
#   env -i PATH="$PATH" "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" <out-dir>/telesapr.wasm <model.tlspr>
# (env -i: wasm_exec refuses an environment over 12 KB.)
set -eu
out=${1:-bin/wasm}
model=${2:-testdata/solid/fillet-tee.tlspr}
mkdir -p "$out"
GOOS=js GOARCH=wasm go build -trimpath -ldflags=-s -o "$out/telesapr.wasm" ./cmd/telesapr-wasm
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" cmd/telesapr-wasm/web/index.html cmd/telesapr-wasm/web/memfs.js "$out/"
cp "$model" "$out/model.tlspr"
ls -l "$out/telesapr.wasm"
