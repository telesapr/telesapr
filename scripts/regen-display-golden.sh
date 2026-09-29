#!/usr/bin/env bash
# Regenerate testdata/display (*.outlines): each group's display outlines,
# as C++ Group::GenerateDisplayItems joins them with PolylineBuilder,
# for the fixtures TestDisplayOutlinesVsCpp lists.
#
# Needs the C++ build in solvespace/build (libsolvespace-core.a and
# libsolvespace-headless.a).
#
# Usage:
#     ./scripts/regen-display-golden.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
S="$REPO_ROOT/solvespace"
B="$S/build"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# The builtin font is found at <binary dir>/../res.
mkdir -p "$TMP/bin"
ln -s "$B/res" "$TMP/res"

c++ -O3 -DNDEBUG -std=gnu++11 -w \
    -I"$S/src" -I"$B/src" -I"$S/extlib/libdxfrw" \
    -isystem /usr/include/cairo -isystem /usr/include/freetype2 \
    -isystem "$S/extlib/mimalloc/include" -isystem "$S/extlib/eigen" \
    "$SCRIPT_DIR/cpp-display-harness.cpp" -o "$TMP/bin/harness" \
    "$B/bin/libsolvespace-core.a" "$B/bin/libsolvespace-headless.a" \
    "$B/bin/libsolvespace-core.a" "$B/bin/libdxfrw.a" \
    -lz -lpng -lfreetype "$B/bin/libmimalloc.a" -lpthread -lrt -latomic -lcairo

cd "$REPO_ROOT/testdata"
sed -n 's/^\t"\([^"]*\.slvs\)",$/\1/p' \
    "$REPO_ROOT/pkg/persist/slvs/display_test.go" |
while read -r f; do
    out="display/${f%.slvs}.outlines"
    mkdir -p "$(dirname "$out")"
    # Each file in its own process: some fixtures crash C++ at exit.
    (cd "$(dirname "$f")" && "$TMP/bin/harness" "$(basename "$f")" \
        "$REPO_ROOT/testdata/Gentium-R.ttf") >"$out"
    echo "$out"
done
