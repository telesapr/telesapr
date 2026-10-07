#!/usr/bin/env bash
# Regenerate testdata/solver/sparseqr: the least-squares cases, and what
# Eigen 3.4's SparseQR, built with the flags of a SolveSpace build,
# computes for them. TestSparseQRVsEigen then requires the SparseQR
# port in pkg/solver to match bit for bit.
#
# Eigen is taken from a SolveSpace source tree, which vendors it in
# extlib/eigen; TELESAPR_SOLVESPACE_SRC names the tree, by default
# ../solvespace beside this repository.
#
# Usage:
#     TELESAPR_SOLVESPACE_SRC=/path/to/solvespace ./scripts/regen-sparseqr-golden.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
OUT="$REPO_ROOT/testdata/solver/sparseqr"
SOLVESPACE_SRC="${TELESAPR_SOLVESPACE_SRC:-$REPO_ROOT/../solvespace}"
BIN="$(mktemp -d)/eigen-sparseqr-harness"

c++ -O3 -DNDEBUG -std=gnu++11 -isystem "$SOLVESPACE_SRC/extlib/eigen" \
    "$SCRIPT_DIR/eigen-sparseqr-harness.cpp" -o "$BIN"
(cd "$REPO_ROOT" && SPARSEQR_GEN=1 go test ./pkg/solver -run TestSparseQRVsEigen >/dev/null || true)
"$BIN" < "$OUT/cases.txt" > "$OUT/eigen.txt"
(cd "$REPO_ROOT" && go test ./pkg/solver -run TestSparseQRVsEigen)
