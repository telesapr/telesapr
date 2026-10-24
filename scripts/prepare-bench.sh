#!/usr/bin/env bash
# prepare-bench.sh -- Stage the models for the telesapr-cli vs
# solvespace-cli benchmark (bench-vs-solvespace.md): copies each .slvs
# fixture and converts it to .tlspr, so both binaries can regenerate the
# same geometry. Solvespace-cli reads the .slvs, telesapr-cli the .tlspr
# (it cannot open a .slvs directly).
#
# Usage:
#     ./scripts/prepare-bench.sh [output-dir]
#
# The output directory defaults to /tmp/opencode/bench. For each model it
# is <output-dir>/<stem>/<stem>.slvs and <stem>.tlspr. Nothing under
# testdata/ is touched. The binaries are taken from bin/telesapr-cli and
# solvespace/build/bin/solvespace-cli, or from $TELESAPR_CLI and
# $SOLVESPACE_CLI.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

OUT="${1:-/tmp/opencode/bench}"
TELESAPR_CLI="${TELESAPR_CLI:-$REPO_ROOT/bin/telesapr-cli}"
SOLVESPACE_CLI="${SOLVESPACE_CLI:-$REPO_ROOT/solvespace/build/bin/solvespace-cli}"

# The benchmark corpus: a .slvs source under testdata/, and the stem its
# staged copies are named after. Features only SolveSpace has, so both
# tools regenerate the same geometry.
MODELS=(
  "group/force_to_mesh/translate.slvs translate"
  "group/translate_nd/normal.slvs translate_nd"
  "group/boolean_coplanar_union/normal.slvs boolean_coplanar_union"
  "group/force_to_mesh/union.slvs union"
  "group/boolean_tangent_crossing/normal.slvs boolean_tangent_crossing"
  "group/boolean_knife_edge/normal.slvs boolean_knife_edge"
  "group/boolean_tangent_spline/normal.slvs boolean_tangent_spline"
  "group/boolean_tangent_edge/normal.slvs boolean_tangent_edge"
  "group/boolean_tangent_fillet/normal.slvs boolean_tangent_fillet"
  "group/force_to_mesh/difference.slvs difference"
  "group/translate_asy/normal.slvs translate_asy"
  "request/ttf_text/extrude.slvs ttf_extrude"
  "request/ttf_text/extrude_kerning.slvs ttf_extrude_kerning"
)

for tool in "$TELESAPR_CLI" "$SOLVESPACE_CLI"; do
  # solvespace-cli's version exits 1, so only check the file is there.
  if [ ! -x "$tool" ]; then
    echo "error: '$tool' is not executable; build it first" >&2
    exit 2
  fi
done

rm -rf "$OUT"
mkdir -p "$OUT"
printf 'source\tstem\n' > "$OUT/models.tsv"

for entry in "${MODELS[@]}"; do
  read -r src stem <<< "$entry"
  d="$OUT/$stem"
  mkdir -p "$d"
  cp "$REPO_ROOT/testdata/$src" "$d/$stem.slvs"
  printf '%s\t%s\n' "$src" "$stem" >> "$OUT/models.tsv"

  if ! "$SOLVESPACE_CLI" regenerate "$d/$stem.slvs" >"$d/solvespace.log" 2>&1; then
    echo "warning: '$stem' failed to regenerate in solvespace-cli" >&2
    continue
  fi
  if ! "$TELESAPR_CLI" convert "$d/$stem.slvs" >"$d/convert.log" 2>&1; then
    echo "warning: '$stem' failed to convert to .tlspr" >&2
    continue
  fi
  if ! TELESAPR_PARALLEL=on "$TELESAPR_CLI" regenerate "$d/$stem.tlspr" >"$d/telesapr.log" 2>&1; then
    echo "warning: '$stem' failed to regenerate in telesapr-cli" >&2
    continue
  fi
  echo "staged $stem"
done

echo "models staged in $OUT"