#!/bin/sh
# Runs `go test "$@"` over PACKAGES, in one go test process: the tests of
# a package run side by side (t.Parallel), and go test runs the packages
# side by side, so the processors are full without splitting a package
# over several processes, which only made them compete.
#
# TESTS picks the part of the suite: "light" leaves out the HEAVY tests,
# "heavy" runs only them, "all" (the default) runs everything. make test
# is the light part and make test-heavy the heavy one; make ci runs both,
# and the workflow gives each its own runner. make test-fast is the light
# part under -short: a HEAVY test never runs there, whether or not it
# skips itself under -short.
#
# SKIP is a regular expression of more tests to leave out, as -skip
# takes it (scripts/test-os.sh's lists): go test keeps only the last
# -skip it is given, so the two are joined here.
#
# With COVERPROFILE set, go test writes the coverage profile there.
#
# Usage:
#     scripts/go-test.sh [go test flags]
#     PACKAGES='./pkg/srf/... ./pkg/sketch/...' scripts/go-test.sh -race
set -eu
cd "$(dirname "$0")/.."

PACKAGES=${PACKAGES:-./...}
# HEAVY are the tests of a minute or more of processor (measured alone,
# 2026-11-06), about half of the suite's processor time between them, so
# that make test and make test-heavy, on a runner each, take about as
# long; a new test of a minute or more goes here. pkg/ui's TestGoScripts
# and TestPathEquivalence and pkg/script's TestScriptRoundTrip cost more
# but stay out, since make test-fast runs no HEAVY test and they are the
# interface's and the scripts' checks there.
HEAVY=${HEAVY:-TestFilletRibAllOverOracleCDT TestFilletRibAllOverOracleEarClip TestFilletRibFootFirstOracle \
TestFilletRibFootAfterTop TestClosedLoftSweep TestSmoothLoftSweep \
TestFilletOnOtherSolids TestChamferOnOtherSolids TestFilletOnOtherSolidsVolumes TestFilletRimMitreFine \
TestFilletPastSideSolidBeyond TestFilletRodThroughPlate TestFilletRodJustPastSides TestFilletPastSlantedSides \
TestChamferRunsOffPastRoundTop TestFilletRunsOffOntoCylinder TestFilletRunsOffOntoCylinderBetween \
TestShellDraftedInsideCornerBesideHollow TestSweepCrossesItselfFittedOrClosed \
TestThreadSolids TestEditThreadShape TestThreadCrests TestThreadRoundRootSweep \
TestCachesSameAsWithout TestExamplesRun}
TESTS=${TESTS:-all}
# The heavy names as one regular expression, for -run and -skip. A
# name matches in every package, so a HEAVY name is defined in one
# package only (TestHeavyNamesUnique, pkg/internal/fixtures).
heavy_re="^($(echo $HEAVY | tr ' ' '|'))\$"
skip_light=$heavy_re
if [ -n "${SKIP:-}" ]; then
	skip_light="$heavy_re|$SKIP"
fi
# go test's default of 10 minutes is too short under the race detector
# and with coverage, and a package's tests on a CI runner of four cores
# took 23 to 30 minutes at 0.6.0; a -timeout among the arguments comes
# later and wins.
TEST_TIMEOUT=${TEST_TIMEOUT:-60m}
cover=""
if [ -n "${COVERPROFILE:-}" ]; then
	cover="-coverprofile=$COVERPROFILE"
fi

case $TESTS in
all) exec go test -timeout "$TEST_TIMEOUT" ${SKIP:+-skip "$SKIP"} $cover "$@" $PACKAGES ;;
light) exec go test -timeout "$TEST_TIMEOUT" -skip "$skip_light" $cover "$@" $PACKAGES ;;
heavy) exec go test -timeout "$TEST_TIMEOUT" -run "$heavy_re" ${SKIP:+-skip "$SKIP"} $cover "$@" $PACKAGES ;;
*)
	echo "go-test.sh: TESTS must be all, light or heavy, not $TESTS" >&2
	exit 2
	;;
esac
