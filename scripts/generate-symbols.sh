#!/bin/sh
# Runs the //go:generate lines of pkg/script/symbols/symbols.go, as
# `go generate ./pkg/script/...` does, but side by side: each
# `yaegi extract` of several packages is split into one per package,
# which yaegi writes to a file of its own. The output is the same; the
# time is that of the slowest package, about 4 s instead of 17 s.
set -eu
cd "$(dirname "$0")/../pkg/script/symbols"

jobs=$(mktemp)
trap 'rm -f "$jobs"' EXIT
sed -n 's|^//go:generate ||p' symbols.go | while read -r line; do
	case "$line" in
	"go tool yaegi extract "*)
		# The flags come first; then the packages, one job each.
		set -- ${line#go tool yaegi extract }
		flags=""
		while [ $# -gt 0 ] && [ "${1#-}" != "$1" ]; do
			flags="$flags $1 $2"
			shift 2
		done
		for pkg in "$@"; do
			echo "go tool yaegi extract$flags $pkg" >>"$jobs"
		done
		;;
	*)
		echo "$line" >>"$jobs"
		;;
	esac
done
# Each line is one job; a failure fails the whole.
tr '\n' '\0' <"$jobs" | xargs -0 -P "$(nproc 2>/dev/null || echo 4)" -I{} sh -c '{}'
