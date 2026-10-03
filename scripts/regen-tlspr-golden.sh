#!/usr/bin/env bash
# Regenerate testdata/tlspr/golden/**/*.tlspr: the import of every fixture,
# as TestFixturesRoundTrip in pkg/persist/convert
# records it. Review the diff of every regenerated golden before
# committing it, and commit it on its own with a message that says why
# it changed. testdata/tlspr/v1 is never regenerated.
#
# Usage:
#     ./scripts/regen-tlspr-golden.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

TELESAPR_REGEN_GOLDEN=1 go test ./pkg/persist/convert -run '^TestFixturesRoundTrip$' -count=1
git status --short testdata/tlspr/golden
