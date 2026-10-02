#!/usr/bin/env bash
# Regenerate testdata/spcs/golden/*.spcs: the import of one fixture of
# each fixture directory, as TestFixturesRoundTrip in pkg/persist/convert
# records it. Review the diff of every regenerated golden before
# committing it, and commit it on its own with a message that says why
# it changed. testdata/spcs/v1 is never regenerated.
#
# Usage:
#     ./scripts/regen-spcs-golden.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

SPACESOLVE_REGEN_SPCS=1 go test ./pkg/persist/convert -run '^TestFixturesRoundTrip$' -count=1
git status --short testdata/spcs/golden
