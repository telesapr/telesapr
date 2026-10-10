// What the Analyze and Edit menus do, from a script: a plate's outline
// copied and pasted turned, the length of its curves, the area of its
// sketch and of its faces, and the centre of mass of its solid.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/measure/measure.go
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/measure/measure.go
//
// It draws a 40 by 20 plate with a hole of radius 5, extrudes it 10,
// and prints what it measures; it writes nothing.
package main

import (
	"fmt"
	"math"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func near(got, want, frac float64) bool { return math.Abs(got-want) <= frac*math.Abs(want) }

func main() {
	doc := api.Current()
	doc.Edit.SetChordTolerance(0.01)

	// A copy of an outline, pasted turned a quarter about the origin,
	// in a sketch of its own.
	doc.Features.SketchIn(doc.Workplane("XY"), "outline")
	outline := doc.Sketch.Rect(api.V(0, 0, 0), api.V(40, 20, 0))
	doc.Edit.Copy(outline...)
	turned := doc.Edit.Paste(api.V(-30, 0, 0), 90, 1)
	if len(turned) != 4 {
		fail("pasted %d lines, want 4", len(turned))
	}
	fmt.Printf("the outline is %.3f mm round, and so is its copy: %.3f mm\n",
		doc.Analyze.CurveLength(outline...), doc.Analyze.CurveLength(turned...))

	plate := doc.Features.SketchIn(doc.Workplane("XY"), "plate")
	doc.Sketch.Rect(api.V(0, 0, 0), api.V(40, 20, 0))
	doc.Sketch.Circle(api.V(20, 10, 0), 5)
	solid := doc.Features.Extrude(10, "")

	area := plate.Analyze().Area()
	if !near(area, 800-25*math.Pi, 0.01) {
		fail("the plate's area is %.3f mm²", area)
	}
	fmt.Printf("the plate's sketch is %.1f mm²\n", area)
	fmt.Printf("its flat faces are %.1f mm²\n", doc.Analyze.FacesArea(solid.Faces()...))
	c := solid.Analyze().CenterOfMass()
	if c.DistanceTo(api.V(20, 10, 5)) > 0.05 {
		fail("the centre of mass is at %v", c)
	}
	fmt.Printf("its centre of mass is at (%.2f, %.2f, %.2f)\n", c.X, c.Y, c.Z)
}
