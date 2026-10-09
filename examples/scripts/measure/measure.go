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
	doc.SetChordTolerance(0.01)

	// A copy of an outline, pasted turned a quarter about the origin,
	// in a sketch of its own.
	doc.SketchIn(doc.Workplane("XY"), "outline")
	outline := doc.Rect(api.V(0, 0, 0), api.V(40, 20, 0))
	doc.Copy(outline...)
	turned := doc.Paste(api.V(-30, 0, 0), 90, 1)
	if len(turned) != 4 {
		fail("pasted %d lines, want 4", len(turned))
	}
	fmt.Printf("the outline is %.3f mm round, and so is its copy: %.3f mm\n",
		doc.CurveLength(outline...), doc.CurveLength(turned...))

	plate := doc.SketchIn(doc.Workplane("XY"), "plate")
	doc.Rect(api.V(0, 0, 0), api.V(40, 20, 0))
	doc.Circle(api.V(20, 10, 0), 5)
	solid := doc.Extrude(10, "")

	area := plate.Area()
	if !near(area, 800-25*math.Pi, 0.01) {
		fail("the plate's area is %.3f mm²", area)
	}
	fmt.Printf("the plate's sketch is %.1f mm²\n", area)
	fmt.Printf("its flat faces are %.1f mm²\n", doc.FacesArea(solid.Faces()...))
	c := solid.CenterOfMass()
	if c.DistanceTo(api.V(20, 10, 5)) > 0.05 {
		fail("the centre of mass is at %v", c)
	}
	fmt.Printf("its centre of mass is at (%.2f, %.2f, %.2f)\n", c.X, c.Y, c.Z)
}
