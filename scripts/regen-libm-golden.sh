#!/usr/bin/env bash
# Regenerate testdata/libm/glibc.txt: what the system's glibc computes
# for sin, cos, tan, asin, acos, atan and atan2 at inputs spread over
# their branches. pkg/libm's TestLibmVsGlibc requires the port to give
# the same bits. On x86-64 glibc uses the FMA variants of these
# functions when the CPU has FMA and AVX2, which is what the port
# follows, so regenerate on such a machine (and with glibc 2.44).
#
# For a much larger comparison, straight against the system's libm, see
# scripts/libmcheck.
#
# Usage:
#     ./scripts/regen-libm-golden.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

cc -O2 "$SCRIPT_DIR/libm-golden.c" -o "$TMP/libm-golden" -lm
mkdir -p "$REPO_ROOT/testdata/libm"
"$TMP/libm-golden" > "$REPO_ROOT/testdata/libm/glibc.txt"
(cd "$REPO_ROOT" && go test ./pkg/libm)
