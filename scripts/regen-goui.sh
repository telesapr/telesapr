#!/usr/bin/env bash
# Regenerate testdata/ui/golden/*.goui: what the Go interface shows for
# each script testdata/ui/scripts/ext_*.script, recorded by Go itself.
# These scripts cover what only spacesolve has, so there is no C++
# output to compare with; the golden is a frozen baseline. Review the
# diff of every regenerated golden before committing it, and commit it
# on its own with a message that says why it changed.
#
# Usage:
#     ./scripts/regen-goui.sh [script names...]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

run='^TestGoScripts$'
if [ $# -gt 0 ]; then
    run="$run/^($(IFS='|'; echo "$*"))\$"
fi
SPACESOLVE_REGEN_GOUI=1 go test ./pkg/ui -run "$run" -count=1
git status --short testdata/ui/golden
