#!/usr/bin/env bash
# Regenerate C++ golden values for boolean fixture volume tests.
#
# The Go port's TestParityBooleanVolumeVsCppGolden test compares its
# post-boolean-CSG volume against the C++ golden values recorded in
# pkg/persist/slvs/bool_generate_test.go (booleanFixtureCppGoldens).
#
# Those values are extracted from the C++ testsuite's source files,
# where each fixture asserts:
#
#     CHECK_EQ_EPS(m->CalculateVolume() / N, 1.0);
#
# This script extracts N for each boolean fixture and prints them as
# a Go map literal suitable for pasting into bool_generate_test.go.
#
# Usage:
#     ./scripts/regen-cpp-goldens.sh
#
# Prerequisite: the C++ solvespace source is checked out at
# solvespace/ (a sibling of this repository's repo root).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CPP_TEST_DIR="$REPO_ROOT/solvespace/test/group"

if [[ ! -d "$CPP_TEST_DIR" ]]; then
    echo "error: $CPP_TEST_DIR not found" >&2
    echo "  clone the C++ SolveSpace repo at $REPO_ROOT/solvespace" >&2
    exit 1
fi

cat <<'EOF'
// Paster for pkg/persist/slvs/bool_generate_test.go
// booleanFixtureCppGoldens:
var booleanFixtureCppGoldens = map[string]float64{
EOF

for fixture in boolean_coplanar_union boolean_knife_edge \
              boolean_tangent_crossing boolean_tangent_edge \
              boolean_tangent_fillet boolean_tangent_spline; do
    test_file="$CPP_TEST_DIR/$fixture/test.cpp"
    if [[ ! -f "$test_file" ]]; then
        echo "  // $fixture: (no test.cpp)" >&2
        continue
    fi
    golden=$(grep -oE "CalculateVolume\\(\\) / [0-9.]+, 1\\.0" "$test_file" \
        | head -n1 | awk '{print $3}' | tr -d ',')
    if [[ -z "$golden" ]]; then
        echo "  // $fixture: (no volume golden)" >&2
        continue
    fi
    printf '    %-30s %s,\n' "\"$fixture\":" "$golden"
done

echo "}"
echo
echo "// Verify by running the C++ testsuite:"
echo "//   cd solvespace/build && ./bin/solvespace-testsuite --filter 'watertight_volume'"