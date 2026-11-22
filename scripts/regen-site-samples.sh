#!/usr/bin/env bash
# regen-site-samples.sh -- Makes the samples the landing page shows
# (site/samples) from the example scripts, with the program itself:
#
#     ./scripts/regen-site-samples.sh [name...]
#
# Each sample is an example under examples/scripts, run by telesapr-cli
# with a title block filled in, and written as its drawing sheet
# (<name>.svg, first angle, as File > Export > Drawing gives it but
# with its lines twice as wide, for the screen) and a
# picture of the model (<name>.png). The table below names them; with
# names given, only those are made again. bin/telesapr-cli must be built
# (make build).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CLI="$REPO_ROOT/bin/telesapr-cli"
OUT="$REPO_ROOT/site/samples"

# example | title | drawing number | material | hidden edges | sheet | scale
# (hidden edges "keep" leaves the sheet as the example set it up)
SAMPLES=(
  "bolt|BOLT M8x30|TS.01.001|STEEL 45|no|A4|2"
  "drawing|ANGLE BRACKET|TS.01.002|STEEL 3|keep|A3|0"
  "housing|TURNED HOUSING|TS.01.003|CAST IRON|yes|A4|1"
  "knob|KNOB|TS.01.004|ALUMINIUM|no|A3|2"
  "tee|PIPE TEE|TS.01.005|BRASS|yes|A3|2"
  "pot|POT WITH DRAIN|TS.01.006|ABS|yes|A4|1"
  "rib|RIBBED SUPPORT|TS.01.007|STEEL 3|no|A3|1"
  "mirror|MIRRORED BRACKET|TS.01.008|STEEL 3|yes|A3|2"
  "spring|COIL SPRING|TS.01.009|STEEL 65G|no|A3|2"
  "vase|VASE|TS.01.010|GLASS|no|A3|1"
  "threadsizes|THREAD SAMPLES|TS.01.011|STEEL 45|no|A4|1"
  "bandfillet|CONE WITH BAND|TS.01.012|BRONZE|no|A4|2"
  "plate|MOUNTING PLATE|TS.01.013|STEEL 3|no|A4|1"
)

if [ ! -x "$CLI" ]; then
  echo "error: $CLI not found: run make build" >&2
  exit 2
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir -p "$OUT"

for row in "${SAMPLES[@]}"; do
  IFS='|' read -r name title number material hidden sheet scale <<<"$row"
  if [ $# -gt 0 ] && [[ " $* " != *" $name "* ]]; then
    continue
  fi
  src="$REPO_ROOT/examples/scripts/$name/$name.go"
  dir="$work/$name"
  mkdir -p "$dir"

  # The example's main becomes a function, and a main after it fills in
  # the sheet and saves the model where the exports below read it.
  settings="d.Sheet, d.Portrait, d.Scale, d.Hidden = \"$sheet\", false, $scale, $([ "$hidden" = yes ] && echo true || echo false)"
  if [ "$hidden" = keep ]; then
    settings=""
  fi
  {
    sed 's/^func main() {$/func exampleMain() {/' "$src"
    cat <<EOF

func main() {
	exampleMain()
	doc := api.Current()
	d := doc.Drawing.Get()
	$settings
	d.FirstAngle = true
	d.Title, d.Number, d.Material = "$title", "$number", "$material"
	d.Author, d.Company = "TeleSAPR", "telesapr-cli"
	doc.Drawing.Set(d)
	if err := doc.File.Save("sample.tlspr"); err != nil {
		panic(err)
	}
}
EOF
  } >"$dir/sample.go"

  echo "==> $name"
  (
    cd "$dir"
    "$CLI" run sample.go >run.log 2>&1 || { cat run.log >&2; exit 1; }
    "$CLI" export-drawing -o sheet.svg sample.tlspr >/dev/null
    # A sheet is shown a few hundred pixels wide, where the pen widths
    # of a print (0.15 and 0.35 mm) are too fine to see: they are doubled.
    awk '/^stroke-width:/ { split($0, a, /[:;]/); printf "stroke-width:%.2f;\n", a[2] * 2; next } { print }' \
      sheet.svg >"$OUT/$name.svg"
    "$CLI" thumbnail -o "$OUT/$name.png" --size 640x480 -v isometric sample.tlspr >/dev/null
  )
done
