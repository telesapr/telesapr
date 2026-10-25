#!/usr/bin/env bash
# Regenerate the goldens: each test below rewrites its golden instead of
# comparing with it when TELESAPR_REGEN_GOLDEN is set. Review the diff
# before committing, and commit regenerated goldens on their own.
#
# Usage:
#     ./scripts/regen-golden.sh            # every golden
#     ./scripts/regen-golden.sh display ui # some areas
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

# area: package and tests, and the goldens they write.
declare -A pkg=(
    [display]=./pkg/sketch [stl]=./pkg/sketch
    [draw]=./pkg/ui [export]=./pkg/ui [textwin]=./pkg/ui [ui]=./pkg/ui
    [render]=./pkg/render [soft]=./pkg/render/soft [res]=./pkg/res [ttf]=./pkg/ttf
    [tlspr]=./pkg/persist/convert [newfile]=./pkg/sketch [modify]=./pkg/sketch
    [thread]=./pkg/sketch [importdxf]=./pkg/persist/dxf [solid]=./pkg/sketch
    [sparseqr]=./pkg/solver [groups]=./pkg/sketch [sprintf]=./pkg/internal/cfmt
)
declare -A run=(
    [display]='^TestDisplayOutlines$'                 # testdata/display
    [stl]='^TestExportStl$'                           # testdata/export/stl
    [draw]='^TestDrawCalls$'                          # testdata/draw
    [export]='^TestExport$'                           # testdata/export
    [textwin]='^TestTextWindow$'                      # testdata/textwin
    [ui]='^(TestGoScripts|TestMenuTable|TestHover|TestInteraction)$' # testdata/ui/golden
    [render]='^TestCamera$'                           # testdata/render/camera.golden
    [soft]='^(TestSoftLayers|TestSoftLines|TestSoftText|TestSoftPolygon|TestSoftMesh)$' # testdata/render/soft
    [res]='^TestFonts$'                               # testdata/res/fonts.golden
    [ttf]='^(TestTTF|TestSynthFonts)$'                # testdata/ttf
    [tlspr]='^TestFixturesRoundTrip$'                  # testdata/tlspr/golden
    [newfile]='^TestNewFile$'                         # testdata/newfile.tlspr
    [modify]='^TestModifyTools$'                      # testdata/modify
    [thread]='^(TestThreadGoldens|TestThreadOnFaceGolden|TestCosmeticThreadGoldens)$' # testdata/thread/*.tlspr
    [importdxf]='^TestImport$'                        # testdata/importdxf/golden
    [solid]='^TestSolidGoldens$'                      # testdata/solid/*.tlspr
    [sparseqr]='^TestSparseQRGolden$'                 # testdata/solver/sparseqr
    [groups]='^TestFixtureGroupsSolve$'               # testdata/fixture-groups.txt
    [sprintf]='^TestSprintfGolden$'                   # pkg/internal/cfmt/testdata/sprintf.golden
)
areas=("$@")
if [ ${#areas[@]} -eq 0 ]; then
    areas=(display stl draw export textwin ui render soft res ttf tlspr newfile modify thread importdxf solid sparseqr groups sprintf)
fi
for a in "${areas[@]}"; do
    if [ -z "${run[$a]:-}" ]; then
        echo "unknown area $a; known: ${!run[*]}" >&2
        exit 2
    fi
    echo "== $a"
    TELESAPR_REGEN_GOLDEN=1 go test "${pkg[$a]}" -run "${run[$a]}" -count=1
done
git status --short testdata pkg/internal/cfmt/testdata
