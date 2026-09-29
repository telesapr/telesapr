#!/usr/bin/env bash
# Regenerate testdata/export: every file C++ SolveSpace exports for the
# fixtures TestExportVsCpp lists, and pkg/dxf/header.txt, the DXF
# HEADER section, cut from one of them.
#
# Needs the C++ build in solvespace/build.
#
# Usage:
#     ./scripts/regen-export-golden.sh
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
    "$SCRIPT_DIR/cpp-export-harness.cpp" -o "$TMP/bin/harness" \
    "$B/bin/libsolvespace-core.a" "$B/bin/libsolvespace-headless.a" \
    "$B/bin/libsolvespace-core.a" "$B/bin/libdxfrw.a" \
    -lz -lpng -lfreetype "$B/bin/libmimalloc.a" -lpthread -lrt -latomic -lcairo

cd "$REPO_ROOT/testdata"
sed -n 's/^\t"\([^"]*\.slvs\)",$/\1/p' \
    "$REPO_ROOT/pkg/ui/export_test.go" |
while read -r f; do
    out="$REPO_ROOT/testdata/export/${f%.slvs}"
    rm -rf "$out"
    mkdir -p "$out"
    # Each file in its own process: some fixtures crash C++ at exit.
    (cd "$(dirname "$f")" && "$TMP/bin/harness" "$(basename "$f")" "$out" \
        "$REPO_ROOT/testdata/Gentium-R.ttf") >/dev/null 2>&1 || echo "harness failed: $f" >&2
    echo "$out"
done

# The HEADER section: from its "2 HEADER" group up to its "0 ENDSEC".
first="$(find "$REPO_ROOT/testdata/export" -name view.dxf | sort | head -1)"
python3 - "$first" "$REPO_ROOT/pkg/dxf/header.txt" <<'PY'
import sys
lines = open(sys.argv[1]).read().split("\n")
assert lines[2:6] == ["  0", "SECTION", "  2", "HEADER"], lines[:6]
end = next(i for i in range(4, len(lines) - 1) if lines[i] == "  0" and lines[i + 1] == "ENDSEC")
open(sys.argv[2], "w").write("\n".join(lines[4:end]) + "\n")
PY
