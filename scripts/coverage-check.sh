#!/bin/sh
# Runs every test with a coverage profile over ./pkg/..., prints the
# total, and fails when a package listed in scripts/coverage-packages.txt
# has less than MIN percent of its statements covered (docs/done-audit.md:
# new packages hold at least 85%).
#
# The profile counts a package's statements whichever package's tests
# reach them, since pkg/sketch and pkg/core are mostly exercised from
# other packages.
set -eu
cd "$(dirname "$0")/.."

MIN=${MIN:-85}
PROFILE=${PROFILE:-coverage.out}
# Each test process: the instrumented fillet tests alone outlast go test's
# default ten minutes on a runner.
TIMEOUT=${TIMEOUT:-90m}
LIST=scripts/coverage-packages.txt
MODULE=$(go list -m)

# Sharded as scripts/go-test.sh shards it; it merges the profiles.
COVERPROFILE="$PROFILE" scripts/go-test.sh -coverpkg=./pkg/... -timeout "$TIMEOUT"

# A block appears once per test binary that was built with it: it is
# covered when any of them ran it.
awk -v min="$MIN" -v module="$MODULE" -v list="$LIST" '
BEGIN {
	while ((getline line < list) > 0) {
		sub(/#.*/, "", line); gsub(/[ \t]/, "", line)
		if (line != "") want[line] = 1
	}
}
FNR == 1 && /^mode:/ { next }
{
	block = $1; stmts[block] = $2
	if ($3 > 0) hit[block] = 1
}
END {
	for (block in stmts) {
		pkg = block; sub(/:.*/, "", pkg); sub(/\/[^\/]*$/, "", pkg)
		sub("^" module "/", "", pkg)
		total[pkg] += stmts[block]; all += stmts[block]
		if (block in hit) { covered[pkg] += stmts[block]; allCovered += stmts[block] }
	}
	printf "total: %.1f%% of %d statements in ./pkg/...\n", 100 * allCovered / all, all
	bad = 0
	for (pkg in want) {
		if (!(pkg in total)) {
			printf "FAIL %s: listed in %s, but it has no statements in the profile\n", pkg, list
			bad = 1
			continue
		}
		pct = 100 * covered[pkg] / total[pkg]
		verdict = "ok  "
		if (pct < min) { verdict = "FAIL"; bad = 1 }
		printf "%s %s: %.1f%% (at least %d%%)\n", verdict, pkg, pct, min
	}
	exit bad
}' "$PROFILE"
