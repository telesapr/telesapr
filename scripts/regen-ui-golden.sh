#!/usr/bin/env bash
# Regenerate testdata/ui/golden (*.cppui): what C++ SolveSpace's
# interface shows for each script in testdata/ui/scripts, run by
# scripts/cpp-ui-harness.cpp. pkg/ui/uiscript_test.go runs the same
# scripts on the Go port.
#
# Needs the C++ build in solvespace/build.
#
# Usage:
#     ./scripts/regen-ui-golden.sh [script names...]
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

# The harness replaces guinone.cpp's windows, menus, timers, dialogs and
# settings, so its definitions must win over the library's.
c++ -O3 -DNDEBUG -std=gnu++11 -w \
    -I"$S/src" -I"$B/src" -I"$S/extlib/libdxfrw" \
    -isystem /usr/include/cairo -isystem /usr/include/freetype2 \
    -isystem "$S/extlib/mimalloc/include" -isystem "$S/extlib/eigen" \
    "$SCRIPT_DIR/cpp-ui-harness.cpp" -o "$TMP/bin/harness" \
    -Wl,--allow-multiple-definition \
    "$B/bin/libsolvespace-core.a" "$B/bin/libsolvespace-headless.a" \
    "$B/bin/libsolvespace-core.a" "$B/bin/libdxfrw.a" \
    -lz -lpng -lfreetype "$B/bin/libmimalloc.a" -lpthread -lrt -latomic -lcairo

cd "$REPO_ROOT/testdata"
mkdir -p ui/golden
if [ $# -gt 0 ]; then
    names=("$@")
else
    names=()
    for f in ui/scripts/*.script; do names+=("$(basename "$f" .script)"); done
fi
for n in "${names[@]}"; do
    out="ui/golden/$n.cppui"
    # Each script in its own process: some states crash C++ at exit.
    "$TMP/bin/harness" "ui/scripts/$n.script" "$TMP/save.slvs" \
        "$REPO_ROOT/testdata/Gentium-R.ttf" >"$out" || echo "harness failed: $n" >&2
    echo "$out"
done
