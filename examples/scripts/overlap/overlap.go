// What lies at a point, from a script: where an arc and a line meet, a
// click on the drawing picks one of their two ends, and the property
// browser lists the other under ALSO HERE. Entity.Overlap gives the
// same list, and Edit.Select selects from it.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/overlap/overlap.go
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/overlap/overlap.go
//
// It draws an arc, a line from its end and a line across the first,
// selects the arc's end found from the line's, and prints what lies at
// each point; it writes nothing.
package main

import (
	"fmt"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func main() {
	doc := api.Current()
	doc.Features.SketchIn(doc.Workplane("XY"), "overlap")
	arc := doc.Sketch.Arc(api.V(0, 0, 0), api.V(15, 0, 0), api.V(0, 15, 0))
	line := doc.Sketch.Line(api.V(15, 0, 0), api.V(40, 0, 0))
	doc.Constrain.Coincident(arc.Start(), line.Start())
	across := doc.Sketch.Line(api.V(30, -5, 0), api.V(30, 5, 0))

	// From the line's start, the arc's: the point a click there might
	// not have picked.
	points, curves := line.Start().Overlap()
	if len(points) != 1 || len(curves) != 0 {
		fail("at the line's start: points %v, curves %v", points, curves)
	}
	doc.Edit.Select(points[0])
	if sel := doc.Edit.Selection(); len(sel) != 1 || sel[0].H != arc.Start().H {
		fail("selected %v, want the arc's start", sel)
	}
	fmt.Printf("at %v: %v\n", line.Start().Pos(), points[0])

	// The line across passes through a point of the first line that no
	// end of either is at; a datum point there finds both.
	mid := doc.Sketch.Point(api.V(30, 0, 0))
	_, curves = mid.Overlap()
	if len(curves) != 2 || curves[0].H != line.H || curves[1].H != across.H {
		fail("through the crossing: %v", curves)
	}
	fmt.Printf("through %v: %v and %v\n", mid.Pos(), curves[0], curves[1])
}
