#!/usr/bin/env bash
# Regenerate the goldens: each test below rewrites its golden instead of
# comparing with it when TELESAPR_REGEN_GOLDEN is set. Most were first
# recorded by the C++ SolveSpace harnesses, or checked against them,
# until the C++ comparison was frozen (roadmap package 5.1). Review the diff
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
    [display]=./pkg/persist/slvs [stl]=./pkg/persist/slvs
    [draw]=./pkg/ui [export]=./pkg/ui [textwin]=./pkg/ui [ui]=./pkg/ui
    [render]=./pkg/render [res]=./pkg/res [ttf]=./pkg/ttf
    [tlspr]=./pkg/persist/convert [newfile]=./pkg/persist/slvs [modify]=./pkg/persist/slvs
    [thread]=./pkg/persist/slvs [importdxf]=./pkg/persist/dxf [solid]=./pkg/sketch
)
declare -A run=(
    [display]='^TestDisplayOutlines$'                 # testdata/display
    [stl]='^TestExportStl$'                           # testdata/export/stl
    [draw]='^TestDrawCalls$'                          # testdata/draw
    [export]='^TestExport$'                           # testdata/export
    [textwin]='^TestTextWindow$'                      # testdata/textwin
    [ui]='^(TestGoScripts|TestMenuTable|TestHover|TestInteraction)$' # testdata/ui/golden
    [render]='^TestCamera$'                           # testdata/render/camera.golden
    [res]='^TestFonts$'                               # testdata/res/fonts.golden
    [ttf]='^(TestTTF|TestSynthFonts)$'                # testdata/ttf
    [tlspr]='^TestFixturesRoundTrip$'                  # testdata/tlspr/golden
    [newfile]='^TestNewFile$'                         # testdata/tlspr/golden/newfile.tlspr
    [modify]='^TestModifyTools$'                      # testdata/tlspr/golden/modify
    [thread]='^TestThreadGoldens$'                    # testdata/thread/*.tlspr
    [importdxf]='^TestImport$'                        # testdata/importdxf/golden
    [solid]='^TestSolidGoldens$'                      # testdata/solid/*.tlspr
)
areas=("$@")
if [ ${#areas[@]} -eq 0 ]; then
    areas=(display stl draw export textwin ui render res ttf tlspr newfile modify thread importdxf solid)
fi
for a in "${areas[@]}"; do
    if [ -z "${run[$a]:-}" ]; then
        echo "unknown area $a; known: ${!run[*]}" >&2
        exit 2
    fi
    echo "== $a"
    TELESAPR_REGEN_GOLDEN=1 go test "${pkg[$a]}" -run "${run[$a]}" -count=1
done
git status --short testdata
