// A round-to-square duct adapter, lofted, made by a script.
//
// Without a window:
//
//	spacesolve-cli run examples/scripts/loft/loft.go [height] [-]
//
// In the application: Script > Run File, or
//
//	spacesolve --script examples/scripts/loft/loft.go
//
// Loft joins the closed loop of one sketch in a workplane to that of
// another by ruled faces, curve by curve: here a circle of radius 20 in
// XY, which counts as four arcs, to a square of side 30 in a workplane
// 40 mm above unless given. Its solid lies between the lofts of the
// circle's inscribed and circumscribed squares to the square, which the
// script checks; it prints the volume and saves loft.spcs, unless given
// "-".
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/spacesolve/spacesolve/pkg/script/api"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

func main() {
	h, write := 40.0, true
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if v, err := strconv.ParseFloat(a, 64); err == nil {
			h = v
		}
	}
	const radius, side = 20.0, 30.0
	doc := api.Current()
	doc.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0.01 })

	round := doc.SketchIn(doc.Workplane("XY"), "round")
	doc.Circle(api.V(0, 0, 0), radius)

	// The square's workplane sits on a point h up the z axis, drawn in 3d.
	doc.Sketch3D("")
	origin := doc.Point(api.V(0, 0, h))
	square := doc.SketchAt(origin, api.V(1, 0, 0), api.V(0, 1, 0), "square")
	doc.Rect(api.V(-side/2, -side/2, h), api.V(side/2, side/2, h))

	duct := doc.Loft(round, square, "duct")
	if p := duct.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	r := duct.Check()
	// Frustums of squares: the circle's inscribed square (area 2r²) and
	// its circumscribed one (4r²), each to the top square.
	frustum := func(a0, a1 float64) float64 { return h / 3 * (a0 + a1 + math.Sqrt(a0*a1)) }
	inner, outer := frustum(2*radius*radius, side*side), frustum(4*radius*radius, side*side)
	fmt.Printf("circle r %g to square %g, %g mm apart: %s, %.1f mm³ (between %.1f and %.1f)\n",
		radius, side, h, map[bool]string{true: "sound", false: "NOT sound"}[r.Sound()], r.Volume, inner, outer)
	if !r.Sound() || r.Volume < inner || r.Volume > outer {
		os.Exit(1)
	}
	if write {
		if err := doc.Save("loft.spcs"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
