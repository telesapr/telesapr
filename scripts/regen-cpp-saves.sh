#!/usr/bin/env bash
# Regenerate testdata/cppsave (*.cppsave): what C++ SolveSpace writes when it opens
# and saves the fixtures that TestPhase7AllFixturesRoundTrip can't
# compare with themselves (roundTripExclusions): files older versions
# wrote, and edits whose solved values C++'s own load changes.
# TestExcludedFixturesVsCppSave requires Go's save to match these.
#
# Needs the C++ build in solvespace/build (libsolvespace-core.a and
# libsolvespace-headless.a).
#
# Usage:
#     ./scripts/regen-cpp-saves.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
S="$REPO_ROOT/solvespace"
B="$S/build"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

c++ -O3 -DNDEBUG -std=gnu++11 -w \
    -I"$S/src" -I"$B/src" -I"$S/extlib/libdxfrw" \
    -isystem /usr/include/cairo -isystem /usr/include/freetype2 \
    -isystem "$S/extlib/mimalloc/include" -isystem "$S/extlib/eigen" \
    "$SCRIPT_DIR/cpp-save-harness.cpp" -o "$TMP/harness" \
    "$B/bin/libsolvespace-core.a" "$B/bin/libsolvespace-headless.a" \
    "$B/bin/libsolvespace-core.a" "$B/bin/libdxfrw.a" \
    -lz -lpng -lfreetype "$B/bin/libmimalloc.a" -lpthread -lrt -latomic -lcairo

cd "$REPO_ROOT/testdata"
sed -n 's/^\t"\([^"]*\.slvs\)": *"\([^"]*\)",$/\1 \2/p' \
    "$REPO_ROOT/pkg/persist/slvs/phase7_test.go" |
while read -r f reason; do
    [[ "$reason" == not* ]] && continue
    out="cppsave/${f%.slvs}.cppsave"
    mkdir -p "$(dirname "$out")"
    # Save next to the original, so that linked files keep their
    # relative paths.
    tmp="$(dirname "$f")/.cppsave-$(basename "$f")"
    (cd "$(dirname "$f")" && "$TMP/harness" "$(basename "$f")" "$(basename "$tmp")") >/dev/null 2>&1
    mv "$tmp" "$out"
    echo "$out"
done
