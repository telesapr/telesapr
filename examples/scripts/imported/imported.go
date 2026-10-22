// What a DXF import draws and the window doesn't: an arc in 3d, a
// workplane drawn in a workplane, and the same comment twice. File >
// Export > Script writes an imported drawing with these calls.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/imported/imported.go
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/imported/imported.go
//
// It draws, in 3d, an arc standing up in the XZ plane; then, in a
// sketch on XY, a workplane in it with an arc in that; then a comment,
// and the same comment again. It writes nothing.
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

func main() {
	doc := api.Current()

	// An arc in 3d, in the XZ plane.
	doc.Features.Sketch3D("free")
	arc := doc.Sketch.ArcIn3D(api.V(0, 0, 0), api.V(10, 0, 0), api.V(-10, 0, 0), api.V(1, 0, 0), api.V(0, 0, 1))
	_, v := arc.Orientation()
	if math.Abs(v.Z-1) > 1e-9 {
		fail("the arc's plane goes up %v, want z", v)
	}
	fmt.Printf("the arc in 3d starts at %v\n", arc.Start().Pos())

	// A workplane in the sketch's own, parallel to it with its origin at
	// (20, 0), and an arc drawn about that origin.
	doc.Features.SketchIn(doc.Workplane("XY"), "flat")
	wp := doc.Sketch.NewWorkplaneIn(api.V(20, 0, 0))
	doc.Sketch.InWorkplane(wp)
	doc.Sketch.Arc(api.V(20, 0, 0), api.V(25, 0, 0), api.V(15, 0, 0))

	// The same words at the same place: Comment keeps one, Again adds
	// the second, as an import writes it.
	note := doc.Constrain.Comment(api.V(0, 30, 0), "from the DXF")
	if again := doc.Constrain.Comment(api.V(0, 30, 0), "from the DXF"); again != note {
		fail("a second equal comment was added")
	}
	doc.Constrain.Again(note)
	if n := len(doc.Active().Constraints()); n != 2 {
		fail("the sketch has %d constraints, want the comment twice", n)
	}
	fmt.Println("the comment is in the sketch twice")
}
