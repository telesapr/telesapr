#!/usr/bin/env bash
# Regenerate testdata/ttf/cpp-dump.txt.gz: what C++ SolveSpace's ttf.cpp
# computes (font names, cap heights, aspect ratios and PlotString's
# Beziers) for testdata/Gentium-R.ttf and the builtin font, over the
# strings in testdata/ttf/strings.txt, and testdata/ttf/cpp-synth-dump.txt.gz,
# the same for the test fonts in testdata/ttf/synth (made by
# scripts/gen-ttf-testfonts.py) over testdata/ttf/synth-strings.txt.
# TestTTFVsCpp and TestSynthFontsVsCpp require pkg/ttf to match them.
#
# With font files as arguments, it instead compares pkg/ttf with C++ on
# them, 60 files at a time (for example every font the system has:
#     ./scripts/regen-ttf-golden.sh $(find /usr/share/fonts -type f)
# ).
#
# Needs the C++ build in solvespace/build (libsolvespace-core.a and
# libsolvespace-headless.a, and res/ for the builtin font).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
S="$REPO_ROOT/solvespace"
B="$S/build"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# The harness finds the builtin font in ../res, next to its directory.
mkdir -p "$TMP/bin"
ln -s "$B/res" "$TMP/res"
c++ -O2 -std=gnu++11 -w \
    -I"$S/src" -I"$B/src" -I"$S/extlib/libdxfrw" \
    -isystem /usr/include/cairo -isystem /usr/include/freetype2 \
    -isystem "$S/extlib/mimalloc/include" -isystem "$S/extlib/eigen" \
    "$SCRIPT_DIR/cpp-ttf-harness.cpp" -o "$TMP/bin/harness" \
    "$B/bin/libsolvespace-core.a" "$B/bin/libsolvespace-headless.a" \
    "$B/bin/libsolvespace-core.a" "$B/bin/libdxfrw.a" \
    -lz -lpng -lfreetype "$B/bin/libmimalloc.a" -lpthread -lrt -latomic -lcairo

STRS="$REPO_ROOT/testdata/ttf/strings.txt"
if [[ $# -eq 0 ]]; then
    "$TMP/bin/harness" "$REPO_ROOT/testdata/Gentium-R.ttf" < "$STRS" 2>/dev/null |
        gzip -9n > "$REPO_ROOT/testdata/ttf/cpp-dump.txt.gz"
    echo testdata/ttf/cpp-dump.txt.gz
    (cd "$REPO_ROOT" && "$TMP/bin/harness" testdata/ttf/synth/* \
        < testdata/ttf/synth-strings.txt 2>/dev/null) |
        gzip -9n > "$REPO_ROOT/testdata/ttf/cpp-synth-dump.txt.gz"
    echo testdata/ttf/cpp-synth-dump.txt.gz
    exit 0
fi

(cd "$REPO_ROOT" && go test -c -o "$TMP/ttf.test" ./pkg/ttf)
printf '%s\n' "$@" | split -l 60 - "$TMP/batch."
for batch in "$TMP"/batch.*; do
    # shellcheck disable=SC2046
    "$TMP/bin/harness" $(cat "$batch") < "$STRS" > "$batch.cpp" 2>/dev/null
    echo "$(head -1 "$batch") ...: $(TTF_CPP_DUMP="$batch.cpp" TTF_FONTS="$batch" \
        TTF_STRS="$STRS" "$TMP/ttf.test" -test.run TestDumpVsCppFile 2>&1 | tail -3 | tr '\n' ' ')"
done
