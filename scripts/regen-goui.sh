#!/usr/bin/env bash
# Regenerate testdata/ui/golden/<name>.goui: what the interface shows
# for each script in testdata/ui/scripts. The goldens of the scripts
# that are not ext_ were first recorded from C++ SolveSpace; Go writes
# them all since the C++ comparison was frozen. Review the diff before
# committing.
#
# Usage:
#     ./scripts/regen-goui.sh [script names...]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

run='^(TestGoScripts|TestMenuTable|TestHover|TestInteraction)$'
if [ $# -gt 0 ]; then
    run="$run/^($(IFS='|'; echo "$*"))\$"
fi
TELESAPR_REGEN_GOLDEN=1 go test ./pkg/ui -run "$run" -count=1
git status --short testdata/ui/golden
