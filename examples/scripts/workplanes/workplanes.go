// Workplanes that follow their geometry, and an axis given as a point
// and a direction: a sketch in the plane through a point parallel to two
// line segments, one square to a normal, and a lathe about the axis
// through a point along a normal.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/workplanes/workplanes.go
//
// In the application, on the sketch that is open: Script > Run File,
// or
//
//	telesapr --script examples/scripts/workplanes/workplanes.go
//
// It writes nothing.
package main

import (
	"fmt"
	"math"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	doc := api.Current()

	// A point and two line segments: the workplane lies through the
	// point, u along the first line and v along the second.
	a := doc.Sketch.Line(api.V(0, 0, 0), api.V(10, 0, 0))
	b := doc.Sketch.Line(api.V(0, 5, 0), api.V(0, 15, 0))
	p := doc.Sketch.Point(api.V(3, 3, 0))
	onLines := doc.Features.SketchOnLines(p, a, b, "on-lines")
	fmt.Println(onLines.Name(), "follows", a, "and", b)

	// A point and a normal: the workplane through the point square to
	// the normal of the YZ plane, so parallel to it.
	doc.Features.SketchIn(doc.Workplane("XY"), "profile")
	origin := doc.Sketch.Point(api.V(0, 0, 0))
	doc.Features.SketchOnNormal(origin, doc.Workplane("YZ").Normal(), "square-to-x")

	// A lathe about the axis through a point along a normal: the normal
	// of the ZX plane runs along y, in the plane of the sketch.
	doc.Features.SketchIn(doc.Workplane("XY"), "ring")
	doc.Sketch.Rect(api.V(10, 0, 0), api.V(20, 10, 0))
	o := doc.Sketch.Point(api.V(0, 0, 0))
	ring := doc.Features.LatheAbout(o, doc.Workplane("ZX").Normal(), "ring")
	want := math.Pi * (20*20 - 10*10) * 10
	got := ring.Analyze().Volume()
	fmt.Printf("ring: %.1f mm³ (%.1f by Pappus)\n", got, want)
	if p := ring.Problem(); p != "" || math.Abs(got-want) > 1e-2*want {
		fmt.Fprintln(os.Stderr, "the ring is wrong:", p)
		os.Exit(1)
	}
}
