#!/usr/bin/env bash
# perf-fixture-hashes.sh -- Regenerates a copy of every .tlspr fixture
# under testdata/ with telesapr-cli and prints "<sha256>  <fixture>" for
# each output, so that a change meant to make TeleSAPR faster can be
# shown to leave every model it builds byte-for-byte as it was:
#
#     ./scripts/perf-fixture-hashes.sh > before.txt
#     (change, make build)
#     ./scripts/perf-fixture-hashes.sh > after.txt
#     diff before.txt after.txt
#
# A fixture that fails to regenerate is listed with FAIL in place of the
# hash. Fixtures are regenerated in sorted order, so one that links
# another sees the linked copy before or after its own regeneration
# the same way each run. The copies live in a temporary directory that
# is removed after; nothing under testdata/ is written. Links are
# relative, so the whole of testdata/ is copied. The binary is
# bin/telesapr-cli, or $TELESAPR_CLI.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

CLI="${TELESAPR_CLI:-$REPO_ROOT/bin/telesapr-cli}"
if [ ! -x "$CLI" ]; then
  echo "error: '$CLI' is not executable; run make build first" >&2
  exit 2
fi

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
cp -r testdata "$WORK/testdata"

find testdata -name '*.tlspr' | LC_ALL=C sort | while read -r f; do
  # regenerate writes the model back over its input: the copy.
  if "$CLI" regenerate "$WORK/$f" >/dev/null 2>&1; then
    printf '%s  %s\n' "$(sha256sum < "$WORK/$f" | cut -d' ' -f1)" "$f"
  else
    printf 'FAIL  %s\n' "$f"
  fi
done
