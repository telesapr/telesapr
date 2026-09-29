#!/usr/bin/env bash
# Regenerate testdata/res/fonts.golden: C++ resource.cpp's vector and
# bitmap font output that TestFontsVsCpp compares with pkg/res.
#
# Needs the C++ build in solvespace/build.
#
# Usage:
#     ./scripts/regen-res-golden.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
S="$REPO_ROOT/solvespace"
B="$S/build"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# Resources are found at <binary dir>/../res.
mkdir -p "$TMP/bin"
ln -s "$B/res" "$TMP/res"

c++ -O3 -DNDEBUG -std=gnu++11 -w \
    -I"$S/src" -I"$B/src" -I"$S/extlib/libdxfrw" \
    -isystem /usr/include/cairo -isystem /usr/include/freetype2 \
    -isystem "$S/extlib/mimalloc/include" -isystem "$S/extlib/eigen" \
    "$SCRIPT_DIR/cpp-res-harness.cpp" -o "$TMP/bin/harness" \
    "$B/bin/libsolvespace-core.a" "$B/bin/libsolvespace-headless.a" \
    "$B/bin/libsolvespace-core.a" "$B/bin/libdxfrw.a" \
    -lz -lpng -lfreetype "$B/bin/libmimalloc.a" -lpthread -lrt -latomic -lcairo

"$TMP/bin/harness" >"$REPO_ROOT/testdata/res/fonts.golden"
echo testdata/res/fonts.golden
