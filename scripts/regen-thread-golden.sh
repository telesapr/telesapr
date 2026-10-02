#!/usr/bin/env bash
# Regenerate testdata/thread (*.slvs): what C++ SolveSpace saves after
# opening the files that the thread generator makes (pkg/sketch
# AddThread, which C++ has no counterpart of). TestThreadVsCpp requires
# Go to save the same bytes after opening them, and the fixture sweeps
# (round trip, parallel) take them in with the rest of testdata.
#
# Needs the C++ build in solvespace/build (libsolvespace-core.a and
# libsolvespace-headless.a).
#
# Usage:
#     ./scripts/regen-thread-golden.sh
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

mkdir "$TMP/made"
(cd "$REPO_ROOT" && THREAD_FIXTURE_DIR="$TMP/made" \
    go test ./pkg/persist/slvs -count=1 -run 'TestThreadVsCpp$' >/dev/null)

mkdir -p "$REPO_ROOT/testdata/thread"
rm -f "$REPO_ROOT/testdata/thread"/*.slvs
for f in "$TMP/made"/*.slvs; do
    name="$(basename "$f")"
    # Each file in its own process, as C++ opens one.
    (cd "$TMP/made" && "$TMP/harness" "$name" "saved-$name") >/dev/null 2>&1
    cp "$TMP/made/saved-$name" "$REPO_ROOT/testdata/thread/$name"
    # Where C++'s own load and save of its file changes the last digits
    # of solved values, the fixture must be in roundTripExclusions.
    (cd "$TMP/made" && "$TMP/harness" "saved-$name" "again-$name") >/dev/null 2>&1
    if cmp -s "$TMP/made/saved-$name" "$TMP/made/again-$name"; then
        echo "thread/$name"
    else
        echo "thread/$name (not a fixed point of C++'s save: exclude it from the round trip)"
    fi
done
