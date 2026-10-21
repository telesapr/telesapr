// A picture frame: a moulding swept round a closed rectangle, made by a
// script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/frame/frame.go [width] [height]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/frame/frame.go
//
// The rectangle, width by height (120 by 80 unless given), is drawn in
// the XZ plane with its left side on the z axis. The moulding's section,
// a rebated step 12 wide and 8 deep with a rounded lip, is drawn in XY
// across that side, where the path is straight. Sweep Along Path takes
// the rectangle round from the section and back to it: a closed path
// has no caps, and its corners are mitred, as a framer cuts them. The
// script prints the volume, the section's area times the length of the
// path through its centre.
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	w, h := 120.0, 80.0
	for i, p := range []*float64{&w, &h} {
		if len(os.Args) > i+1 {
			v, err := strconv.ParseFloat(os.Args[i+1], 64)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			*p = v
		}
	}

	doc := api.Current()
	doc.Features.SketchIn(doc.Workplane("ZX"), "path")
	corners := []api.Vec{api.V(0, 0, -h/2), api.V(0, 0, h/2), api.V(w, 0, h/2), api.V(w, 0, -h/2)}
	var path []api.Entity
	for i := 0; i < 4; i++ {
		path = append(path, doc.Sketch.Line(corners[i], corners[(i+1)%4]))
	}

	// The section about the path: the frame's outside at x = -6, its
	// inside at x = 6, a rebate 4 by 3 behind (y below 0) on the inside,
	// and a lip of radius 2 at the front outside edge.
	doc.Features.SketchIn(doc.Workplane("XY"), "moulding")
	p := []api.Vec{
		api.V(-6, -4, 0), api.V(2, -4, 0), api.V(2, -1, 0), api.V(6, -1, 0),
		api.V(6, 4, 0), api.V(-4, 4, 0),
	}
	for i := 0; i+1 < len(p); i++ {
		doc.Sketch.Line(p[i], p[i+1])
	}
	doc.Sketch.Arc(api.V(-4, 2, 0), api.V(-4, 4, 0), api.V(-6, 2, 0))
	doc.Sketch.Line(api.V(-6, 2, 0), p[0])

	frame := doc.Features.Sweep(path, "frame")
	if msg := frame.Problem(); msg != "" {
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
	r := frame.Analyze().Check()
	// The section's area, and its centroid's x: the whole 12 by 8, less
	// the rebate, less what the lip's round leaves of a 2 by 2 square.
	lip := 4 - math.Pi
	area := 12*8 - 4*3 - lip
	lipMoment := 4*(-5) - math.Pi*(-4-8/(3*math.Pi))
	cx := (-4*3*4 - lipMoment) / area
	// Mitred, each run between two corners is shorter by twice how far
	// in a point of the section is: the path through the centroid, the
	// rectangle moved in by its x on every side.
	want := area * (2*w + 2*h - 8*cx)
	fmt.Printf("frame: %.0f mm³ (about %.0f), sound %v\n", r.Volume, want, r.Sound())
	if !r.Sound() || math.Abs(r.Volume-want) > 0.01*want {
		os.Exit(1)
	}
}
