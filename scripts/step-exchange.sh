#!/usr/bin/env bash
# Exchanges STEP files with other programs and compares what each finds
# (doc/dev-notes/step.md). TeleSAPR exports a set of models; OpenCASCADE
# (DRAWEXE) and FreeCAD read them back; OpenCASCADE writes each again as
# AP242 and TeleSAPR links that as a solid; NIST's STEP File Analyzer
# checks the files' syntax where SFA_CL names its sfa-cl.exe (run under
# wine). Then scripts/step-exchange/compare.py prints a line per model
# against the CAx-IF's thresholds for validation properties.
#
#     scripts/step-exchange.sh [out-dir]
#
# Needs DRAWEXE and FreeCADCmd on PATH; SFA and wine are optional.
# Nothing in the tree is changed: the files go to out-dir, a new
# temporary directory by default. The fixtures that OpenCASCADE writes
# for TeleSAPR's reader, testdata/step/exchange, are made by
# scripts/step-exchange/occ-fixtures.tcl instead.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
here=$root/scripts/step-exchange
out=${1:-$(mktemp -d -t step-exchange.XXXXXX)}
mkdir -p "$out/export" "$out/back" "$out/models"
out=$(cd "$out" && pwd)
results=$out/results.tsv
: >"$results"

for tool in DRAWEXE FreeCADCmd; do
	command -v "$tool" >/dev/null || { echo "step-exchange: $tool not found" >&2; exit 1; }
done

cli=$out/telesapr-cli
(cd "$root" && go build -o "$cli" ./cmd/telesapr-cli)

# The models: fixtures of the tree, and scripts that save their model.
models=(
	testdata/solid/chamfer-corner.tlspr
	testdata/solid/fillet-corner-sphere.tlspr
	testdata/solid/fillet-cone.tlspr
	testdata/solid/fillet-mitre.tlspr
	testdata/solid/fillet-tee.tlspr
	testdata/solid/loft-ring.tlspr
	testdata/solid/loft-circle-to-square.tlspr
	testdata/solid/sweep-pipe.tlspr
	testdata/solid/sweep-twisted-arcs.tlspr
	testdata/solid/shell-revolve.tlspr
	testdata/solid/shell-holed-block.tlspr
	testdata/solid/mirror-apart.tlspr
	testdata/solid/pattern-ring.tlspr
	testdata/solid/wrap-cylinder-text-sunk.tlspr
	testdata/solid/draft-sides.tlspr
	testdata/thread/bolt_m8x30.tlspr
	testdata/thread/nut_m8.tlspr
	examples/scripts/mass/mass.go
	examples/scripts/knob/knob.go
	examples/scripts/vase/vase.go
	scripts/step-exchange/colours.go
)

for m in "${models[@]}"; do
	name=$(basename "${m%.go}" .tlspr)
	model=$root/$m
	if [[ $m == *.go ]]; then
		# A script saves its model in the directory it runs in.
		dir=$out/models/$name
		mkdir -p "$dir"
		(cd "$dir" && "$cli" run "$model" >/dev/null 2>&1) || true
		model=$(ls -t "$dir"/*.tlspr 2>/dev/null | head -1 || true)
		if [[ -z $model ]]; then
			printf 'ours\t%s\tno model saved\n' "$name" >>"$results"
			continue
		fi
	fi
	if line=$("$cli" run "$model" "$here/export.go" "$out/export/$name.step" 2>&1 | grep -P '^ours\t'); then
		printf '%s\n' "$line" | sed "s/^ours\t/ours\t$name\t/" >>"$results"
	else
		printf 'ours\t%s\texport failed\n' "$name" >>"$results"
	fi
done

STEP_DIR=$out/export DRAWEXE -b -f "$here/occ-read.tcl" 2>&1 | grep -P '^occ\t' >>"$results" || true
STEP_DIR=$out/export FreeCADCmd "$here/freecad-read.py" 2>&1 | grep -P '^fc\t' >>"$results" || true

# Round trip: OpenCASCADE writes each file again, TeleSAPR links it.
IN_DIR=$out/export OUT_DIR=$out/back DRAWEXE -b -f "$here/occ-rewrite.tcl" >/dev/null 2>&1 || true
for f in "$out"/back/*.step; do
	[[ -e $f ]] || continue
	name=$(basename "$f" .step)
	if line=$("$cli" run "$here/link.go" "$f" 2>&1 | grep -P '^back\t'); then
		printf '%s\n' "$line" | sed "s/^back\t/back\t$name\t/" >>"$results"
	else
		printf 'back\t%s\tlink failed\n' "$name" >>"$results"
	fi
done

if [[ -n ${SFA_CL:-} ]]; then
	# sfa-cl.exe wants .stp; it writes its report beside the file.
	mkdir -p "$out/sfa"
	for f in "$out"/export/*.step; do
		name=$(basename "$f" .step)
		cp "$f" "$out/sfa/$name.stp"
		if wine "$SFA_CL" "$out/sfa/$name.stp" syntax noopen nolog 2>&1 | grep -q 'No syntax errors or warnings'; then
			printf 'sfa\t%s\tclean\n' "$name" >>"$results"
		else
			printf 'sfa\t%s\tsyntax errors or warnings\n' "$name" >>"$results"
		fi
	done
fi

echo "files in $out"
python3 "$here/compare.py" "$results"
