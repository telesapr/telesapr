#!/bin/sh
# Runs `go test "$@"` over PACKAGES, in one go test process: the tests of
# a package run side by side (t.Parallel), and go test runs the packages
# side by side, so the processors are full without splitting a package
# over several processes, which only made them compete.
#
# TESTS picks the part of the suite: "light" leaves out the HEAVY tests,
# "heavy" runs only them, "all" (the default) runs everything. make test
# is the light part and make test-heavy the heavy one; make ci runs both,
# and the workflow gives each its own runner.
#
# With COVERPROFILE set, go test writes the coverage profile there.
#
# Usage:
#     scripts/go-test.sh [go test flags]
#     PACKAGES='./pkg/srf/... ./pkg/sketch/...' scripts/go-test.sh -race
set -eu
cd "$(dirname "$0")/.."

PACKAGES=${PACKAGES:-./...}
# HEAVY are the tests that take the longest, about half of the suite's
# time between them, so that make test and make test-heavy, on a runner
# each, take about as long; a new test of a minute or more goes here.
HEAVY=${HEAVY:-TestFixtures TestClosedLoftSweep TestSmoothLoftSweep TestBSplineSweep TestPathSweepSweep \
TestFilletOnOtherSolids TestChamferOnOtherSolids TestFilletOnOtherSolidsVolumes TestFilletRimMitre TestFilletRimMitreFine \
TestCachesSameAsWithout \
TestParallelSameAsSerial TestThreadSolids TestThreadGoldens TestThreadSweep TestEditThreadShape TestThreadChamfer \
TestFreedomVsShowDof TestMoveGroupSweep TestSuppressAllSweep}
TESTS=${TESTS:-all}
# The heavy names as one regular expression, for -run and -skip.
heavy_re="^($(echo $HEAVY | tr ' ' '|'))\$"
# go test's default of 10 minutes is too short under the race detector
# and with coverage; a -timeout among the arguments comes later and wins.
TEST_TIMEOUT=${TEST_TIMEOUT:-30m}
cover=""
if [ -n "${COVERPROFILE:-}" ]; then
	cover="-coverprofile=$COVERPROFILE"
fi

case $TESTS in
all) exec go test -timeout "$TEST_TIMEOUT" $cover "$@" $PACKAGES ;;
light) exec go test -timeout "$TEST_TIMEOUT" -skip "$heavy_re" $cover "$@" $PACKAGES ;;
heavy) exec go test -timeout "$TEST_TIMEOUT" -run "$heavy_re" $cover "$@" $PACKAGES ;;
*)
	echo "go-test.sh: TESTS must be all, light or heavy, not $TESTS" >&2
	exit 2
	;;
esac
