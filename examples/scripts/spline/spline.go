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
// them back, and frees the line from the cubic. Then it draws a spline
// and a closed one straight from their points, as File > Export >
// Script writes them. It writes nothing.
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
	doc.Features.SketchIn(doc.Workplane("XY"), "curve")
	c := doc.Sketch.Cubic(api.V(0, 0, 0), api.V(20, 20, 0), api.V(40, 20, 0), api.V(60, 0, 0))
	tail := doc.Sketch.Line(api.V(60, 0, 0), api.V(80, 0, 0))
	doc.Constrain.Coincident(c.End(), tail.Start())

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

	// Closing a spline: it goes round, and its two ends are one, as a
	// click on the first point closes the one the mouse is drawing.
	ring := doc.Sketch.Cubic(api.V(0, 0, 0), api.V(20, 0, 0), api.V(20, 20, 0), api.V(0, 20, 0))
	for _, p := range []api.Vec{api.V(0, 0, 0), api.V(20, 0, 0), api.V(20, 20, 0)} {
		ring.AddSplinePoint(p)
	}
	ring.MakePeriodic()
	if ring.Kind() != "periodic-cubic" {
		fail("the spline is a %s, not a closed one", ring.Kind())
	}
	fmt.Printf("the spline goes round through %d points\n", len(ring.Points()))

	// Drawn from all their points at once: an open spline's are its
	// start, the control point beside it, the points it passes through,
	// the end's control point and its end; a closed one's the points it
	// passes through.
	wave := doc.Sketch.Spline(api.V(0, 40, 0), api.V(10, 50, 0), api.V(20, 40, 0), api.V(30, 30, 0), api.V(40, 40, 0), api.V(50, 50, 0))
	loop := doc.Sketch.PeriodicSpline(api.V(70, 30, 0), api.V(80, 40, 0), api.V(70, 50, 0), api.V(60, 40, 0))
	if len(wave.Points()) != 6 || loop.Kind() != "periodic-cubic" {
		fail("the wave has %d points, the loop is a %s", len(wave.Points()), loop.Kind())
	}
	fmt.Printf("a wave of %d points and a loop through %d\n", len(wave.Points()), len(loop.Points()))
}
