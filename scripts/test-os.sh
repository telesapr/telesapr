#!/bin/sh
# The quick tests (go test -short, through scripts/go-test.sh) on the
# system it runs on, without the tests scripts/os-skip/<GOOS>.txt lists:
# those fail there for a reason of the platform and are to be fixed.
# The CI and Release workflows run it on Windows and macOS; on Linux
# nothing is skipped and it is make test-fast.
#
# Usage:
#     scripts/test-os.sh [go test flags]
set -eu
cd "$(dirname "$0")/.."

list=scripts/os-skip/$(go env GOOS).txt
skip=""
if [ -f "$list" ]; then
	skip=$(sed -e 's/#.*//' -e 's/[[:space:]]//g' "$list" | grep -v '^$' | paste -sd '|' -)
fi
if [ -n "$skip" ]; then
	echo "skipping, as $list lists: $skip"
	exec scripts/go-test.sh -short -skip "^($skip)\$" "$@"
fi
exec scripts/go-test.sh -short "$@"
