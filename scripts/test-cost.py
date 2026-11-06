#!/usr/bin/env python3
"""Lists the tests that cost the most in a `go test -json` run, read from
standard input or a file, and fails when one costs more than --limit.

A test's cost is the time of its leaf subtests summed (its own when it has
none): a parallel test's own Elapsed leaves out its parallel subtests, and
the summed time is what the test asks of the machine, inflated a little by
the other tests beside it. make test-cost runs it over make test-fast, whose
tests are each to stay within a few seconds of processor (AGENTS.md).

Usage:
    scripts/go-test.sh -short -json | scripts/test-cost.py [--top N] [--limit S]
"""

import argparse
import collections
import json
import sys


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("file", nargs="?", help="go test -json output (default: standard input)")
    ap.add_argument("--top", type=int, default=25, help="how many tests to list")
    ap.add_argument("--limit", type=float, default=0, help="fail when a test costs more seconds than this")
    ap.add_argument("--exempt", default="", help="tests, separated by spaces, the limit leaves out: collections of scenarios, each cheap")
    args = ap.parse_args()

    elapsed = {}  # (package, test) -> seconds, every test and subtest
    failed, pkg_failed = [], []
    with open(args.file) if args.file else sys.stdin as f:
        for line in f:
            try:
                e = json.loads(line)
            except json.JSONDecodeError:
                sys.stdout.write(line)  # a build error, say
                continue
            action, test = e.get("Action"), e.get("Test")
            pkg = e.get("Package", "").removeprefix("github.com/telesapr/telesapr/")
            if action not in ("pass", "fail", "skip"):
                continue
            if test is None:
                if action == "fail":
                    pkg_failed.append(pkg)
                continue
            elapsed[(pkg, test)] = e.get("Elapsed", 0)
            if action == "fail":
                failed.append(f"{pkg} {test}")

    parents = {(p, t.rsplit("/", 1)[0]) for p, t in elapsed if "/" in t}
    cost = collections.Counter()
    for (p, t), s in elapsed.items():
        if (p, t) not in parents:  # a leaf: a subtest without subtests, or a test without any
            cost[(p, t.split("/")[0])] += s

    width = max((len(p) for p, _ in cost), default=0)
    print(f"{'seconds':>8}  {'package':<{width}}  test")
    for (p, t), s in cost.most_common(args.top):
        print(f"{s:8.1f}  {p:<{width}}  {t}")
    print(f"{sum(cost.values()):8.1f}  in all, {len(cost)} tests")

    exempt = set(args.exempt.split())
    over = [(k, s) for k, s in cost.items() if args.limit and s > args.limit and k[1] not in exempt]
    for (p, t), s in sorted(over, key=lambda x: -x[1]):
        print(f"test-cost: {p} {t} costs {s:.1f} s, over the limit of {args.limit:g}", file=sys.stderr)
    for f in failed:
        print(f"test-cost: FAIL {f}", file=sys.stderr)
    for p in pkg_failed:
        print(f"test-cost: FAIL {p}", file=sys.stderr)
    return 1 if over or failed or pkg_failed else 0


if __name__ == "__main__":
    sys.exit(main())
