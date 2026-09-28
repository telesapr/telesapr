#!/usr/bin/env bash
# Regenerate testdata/solver/sparseqr: the least-squares cases, and what
# C++ SolveSpace's Eigen (the Eigen 3.4 vendored in solvespace/extlib,
# built with the flags of the C++ build) computes for them.
# TestSparseQRVsEigen then requires Go's SparseQR port to match bit for
# bit.
#
# Usage:
#     ./scripts/regen-sparseqr-golden.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
OUT="$REPO_ROOT/testdata/solver/sparseqr"
BIN="$(mktemp -d)/eigen-sparseqr-harness"

c++ -O3 -DNDEBUG -std=gnu++11 -isystem "$REPO_ROOT/solvespace/extlib/eigen" \
    "$SCRIPT_DIR/eigen-sparseqr-harness.cpp" -o "$BIN"
(cd "$REPO_ROOT" && SPARSEQR_GEN=1 go test ./pkg/solver -run TestSparseQRVsEigen >/dev/null || true)
"$BIN" < "$OUT/cases.txt" > "$OUT/eigen.txt"
(cd "$REPO_ROOT" && go test ./pkg/solver -run TestSparseQRVsEigen)
