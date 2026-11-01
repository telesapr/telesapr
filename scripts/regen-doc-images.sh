#!/usr/bin/env bash
# Regenerate doc/guide/images: the pictures the scenarios in
# doc/guide/scenarios make, on the headless platform. Look at every
# picture that changed before committing, and commit them on their own.
#
# Usage:
#     ./scripts/regen-doc-images.sh [scenario names...]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

run='^TestDocScenarios$'
if [ $# -gt 0 ]; then
    run="$run/^($(IFS='|'; echo "$*"))\$"
fi
mkdir -p doc/guide/images
TELESAPR_DOC_SHOTS="$REPO_ROOT/doc/guide/images" go test ./pkg/ui -run "$run" -count=1
git status --short doc/guide/images
