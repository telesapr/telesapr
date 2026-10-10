// A pedestal: a base, a column on its top and a cap on the column's
// top, each drawn on the face below it, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/pedestal/pedestal.go [thickness] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/pedestal/pedestal.go
//
// Sketch On Face starts a sketch in a flat face of the solid, found here
// by a point on it: the column is drawn on the top of a 60 x 40 base 8
// mm thick, the cap on the top of the column. The sketches follow their
// faces, so when the base is made thicker (12 mm unless given) the
// column and the cap go up with it. The script checks the volume and
// the height before and after, prints them, and saves pedestal.tlspr,
// unless given "-".
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	thick, write := 12.0, true
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if v, err := strconv.ParseFloat(a, 64); err == nil {
			thick = v
		}
	}
	doc := api.Current()
	doc.Rect(api.V(0, 0, 0), api.V(60, 40, 0))
	base := doc.Extrude(8, "base")

	doc.SketchOnFace(base.Solid().FaceAt(api.V(30, 20, 8)), "column-sketch")
	doc.Rect(api.V(20, 13, 8), api.V(40, 27, 8))
	column := doc.Extrude(12, "column")

	doc.SketchOnFace(column.Solid().FaceAt(api.V(30, 20, 20)), "cap-sketch")
	doc.Rect(api.V(25, 16, 20), api.V(35, 24, 20))
	pedestal := doc.Extrude(4, "cap")

	check := func(t float64) {
		r := pedestal.Check()
		want, high := 60*40*t+20*14*12+10*8*4.0, t+12+4
		fmt.Printf("base %g mm thick: %s, %.1f mm³ (%.1f by the formula), %g mm high\n",
			t, map[bool]string{true: "sound", false: "NOT sound"}[r.Sound()], r.Volume, want, r.Max.Z)
		if !r.Sound() || math.Abs(r.Volume-want) > 1e-6*want || math.Abs(r.Max.Z-high) > 1e-9 {
			os.Exit(1)
		}
	}
	check(8)
	// The base's parameters are the vector to its middle.
	base.Param(2).Set(thick / 2)
	doc.Regen()
	check(thick)

	if write {
		if err := doc.Save("pedestal.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
