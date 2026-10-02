#!/usr/bin/env bash
# Regenerate the goldens that were first recorded by the C++ SolveSpace
# harnesses and that Go writes since the C++ comparison was frozen
# (roadmap package 5.1): each test below rewrites its golden instead of
# comparing with it when SPACESOLVE_REGEN_GOLDEN is set. Review the diff
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
)
areas=("$@")
if [ ${#areas[@]} -eq 0 ]; then
    areas=(display stl draw export textwin ui render res ttf)
fi
for a in "${areas[@]}"; do
    if [ -z "${run[$a]:-}" ]; then
        echo "unknown area $a; known: ${!run[*]}" >&2
        exit 2
    fi
    echo "== $a"
    SPACESOLVE_REGEN_GOLDEN=1 go test "${pkg[$a]}" -run "${run[$a]}" -count=1
done
git status --short testdata
