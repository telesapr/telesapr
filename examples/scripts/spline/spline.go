// A cubic spline's points added and taken away, and a point's
// coincidences deleted, as the context menu's Add Spline Point, Remove
// Spline Point and Delete Point-Coincident Constraint do.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/spline/spline.go
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/spline/spline.go
//
// It draws a cubic from (0, 0) to (60, 0) and a line from its end,
// joined there, bends the cubic through two more points, takes one of
// them back, and frees the line from the cubic. It writes nothing.
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
	doc.SketchIn(doc.Workplane("XY"), "curve")
	c := doc.Cubic(api.V(0, 0, 0), api.V(20, 20, 0), api.V(40, 20, 0), api.V(60, 0, 0))
	tail := doc.Line(api.V(60, 0, 0), api.V(80, 0, 0))
	doc.Coincident(c.End(), tail.Start())

	// Each point goes into the span of the spline's points nearest it.
	c.AddSplinePoint(api.V(20, 25, 0))
	c.AddSplinePoint(api.V(45, 15, 0))
	pts := c.Points()
	if len(pts) != 6 {
		fail("%d points after two were added, want 6", len(pts))
	}
	fmt.Printf("the spline passes through %v and %v\n", pts[2].Pos(), pts[3].Pos())

	pts[3].RemoveSplinePoint()
	if n := len(c.Points()); n != 5 {
		fail("%d points after one was taken away, want 5", n)
	}

	// The line's start is coincident with the spline's end; deleting
	// the coincidence lets the two move apart.
	tail.Start().DeleteCoincidences()
	tail.Start().MoveTo(api.V(65, 5, 0))
	if d := tail.Start().Pos().DistanceTo(c.End().Pos()); d < 1 {
		fail("the line's start is %.3f mm from the spline's end, still joined", d)
	}
	fmt.Printf("the line starts at %v, apart from the spline\n", tail.Start().Pos())
}
