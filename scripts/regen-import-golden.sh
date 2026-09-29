#!/usr/bin/env bash
# Regenerate testdata/importdxf/golden: for each DXF file in
# testdata/importdxf, the sketch C++ SolveSpace makes by importing it
# into a new file, saved, both in the new file's workplane (NAME.cppimport)
# and free in 3d (NAME.3d.cppimport).
#
# Needs the C++ build in solvespace/build.
#
# Usage:
#     ./scripts/regen-import-golden.sh
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
    "$SCRIPT_DIR/cpp-import-harness.cpp" -o "$TMP/bin/harness" \
    "$B/bin/libsolvespace-core.a" "$B/bin/libsolvespace-headless.a" \
    "$B/bin/libsolvespace-core.a" "$B/bin/libdxfrw.a" \
    -lz -lpng -lfreetype "$B/bin/libmimalloc.a" -lpthread -lrt -latomic -lcairo

cd "$REPO_ROOT/testdata/importdxf"
rm -rf golden
mkdir -p golden
find . -name '*.dxf' | sed 's|^\./||' | sort | while read -r f; do
    base="golden/${f%.dxf}"
    mkdir -p "$(dirname "$base")"
    # Each file in its own process: C++ can crash on a bad file.
    "$TMP/bin/harness" "$f" "$base.cppimport" >/dev/null 2>&1 || echo "harness failed: $f" >&2
    "$TMP/bin/harness" "$f" "$base.3d.cppimport" 3d >/dev/null 2>&1 || echo "harness failed (3d): $f" >&2
    echo "$f"
done
