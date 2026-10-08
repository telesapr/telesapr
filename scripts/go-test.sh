#!/bin/sh
# Runs `go test "$@"` over PACKAGES, with each package of SHARDED split
# into several go test processes run side by side: its heavy tests
# (HEAVY) one process each, and the rest dealt round robin over
# REST_SHARDS processes. Each process has its own globals, so tests that
# set the chord tolerance or the Boolean mode, as every regeneration
# does, need not run in one process, and t.Parallel is not needed.
#
# With COVERPROFILE set, every process writes a profile, and they are
# merged into that file; a block then appears once per process, which
# scripts/coverage-check.sh already allows for.
#
# Usage:
#     scripts/go-test.sh [go test flags]
#     PACKAGES='./pkg/srf/... ./pkg/sketch/...' scripts/go-test.sh -race
set -eu
cd "$(dirname "$0")/.."

PACKAGES=${PACKAGES:-./...}
SHARDED=${SHARDED:-./pkg/persist/slvs}
HEAVY=${HEAVY:-TestParallelSameAsSerial TestParallelFastSameOnAnyGoroutines TestThreadSolids \
TestThreadGoldens TestThreadSweep TestFreedomVsShowDof TestParallelFastSameShapeAsSerial TestMoveGroupSweep TestSuppressAllSweep}
REST_SHARDS=${REST_SHARDS:-3}
COVERPROFILE=${COVERPROFILE:-}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# The packages to shard are those of SHARDED that PACKAGES names.
all=$(go list $PACKAGES)
shardable=$(go list $SHARDED)
sharded=""
others=""
for p in $all; do
	case " $(echo $shardable) " in
	*" $p "*) sharded="$sharded $p" ;;
	*) others="$others $p" ;;
	esac
done

# Build every test binary once, so the processes below share it.
go test "$@" -run '^$' $all >/dev/null

n=0
job() { # job <go test arguments...>
	n=$((n + 1))
	cover=""
	if [ -n "$COVERPROFILE" ]; then
		cover="-coverprofile=$tmp/cover.$n"
	fi
	( go test "$@" $cover >"$tmp/out.$n" 2>&1 && echo ok >"$tmp/status.$n" ||
		echo fail >"$tmp/status.$n" ) &
}

if [ -n "$others" ]; then
	job "$@" $others
fi
for p in $sharded; do
	tests=$(go test "$@" -list '.*' "$p" | grep -E '^(Test|Fuzz|Example)' || true)
	rest=""
	for name in $tests; do
		case " $(echo $HEAVY) " in
		*" $name "*) job "$@" -run "^$name\$" "$p" ;;
		*) rest="$rest $name" ;;
		esac
	done
	i=0
	while [ $i -lt "$REST_SHARDS" ]; do
		k=0
		names=""
		for name in $rest; do
			if [ $((k % REST_SHARDS)) -eq $i ]; then
				names="$names|$name"
			fi
			k=$((k + 1))
		done
		if [ -n "$names" ]; then
			job "$@" -run "^(${names#|})\$" "$p"
		fi
		i=$((i + 1))
	done
done
wait

failed=0
i=1
while [ $i -le $n ]; do
	cat "$tmp/out.$i"
	if [ "$(cat "$tmp/status.$i" 2>/dev/null)" != ok ]; then
		failed=1
	fi
	i=$((i + 1))
done

if [ -n "$COVERPROFILE" ]; then
	first=$(ls "$tmp"/cover.* | head -n 1)
	head -n 1 "$first" >"$COVERPROFILE"
	for f in "$tmp"/cover.*; do
		tail -n +2 "$f" >>"$COVERPROFILE"
	done
fi
exit $failed
