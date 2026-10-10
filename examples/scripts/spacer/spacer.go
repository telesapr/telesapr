// A spacer extruded both ways from its sketch, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/spacer/spacer.go [thickness] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/spacer/spacer.go
//
// A two-sided extrusion keeps its whole depth and puts the sketch in its
// middle, as the text window's "two-sided" does: a ring of 12 and 6 mm
// radii, 8 mm thick unless given, runs from -4 to 4. The script checks
// the solid against the ring's volume, prints it, and saves
// spacer.tlspr, unless given "-".
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	thick, write := 8.0, true
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if v, err := strconv.ParseFloat(a, 64); err == nil {
			thick = v
		}
	}
	outer, inner := 12.0, 6.0
	doc := api.Current()
	doc.Constrain.Diameter(doc.Sketch.Circle(api.V(0, 0, 0), outer), 2*outer)
	doc.Constrain.Diameter(doc.Sketch.Circle(api.V(0, 0, 0), inner), 2*inner)
	spacer := doc.Features.Extrude(thick, "spacer")
	spacer.SetTwoSided(true)

	res := spacer.Analyze().Check()
	want := math.Pi * (outer*outer - inner*inner) * thick
	lo, hi := spacer.Analyze().Bounds()
	fmt.Printf("a spacer %g thick, from z %.3f to %.3f: %s, %.1f mm³ (%.1f by its area)\n",
		thick, lo.Z, hi.Z, res.String(), res.Volume, want)
	if !res.Sound() || math.Abs(res.Volume-want) > want*0.01 || math.Abs(lo.Z+thick/2) > 1e-6 || math.Abs(hi.Z-thick/2) > 1e-6 {
		fmt.Fprintln(os.Stderr, "the spacer is not what was asked for")
		os.Exit(1)
	}
	if write {
		if err := doc.File.Save("spacer.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
