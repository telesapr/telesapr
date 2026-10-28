// A plate with a stud under it and a tapped hole through it, both
// threaded by pointing at their faces, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/threadface/threadface.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/threadface/threadface.go
//
// Features.ThreadOnFace takes a cylindrical face of the solid, as
// Solid.FaceAt finds it, and works out the rest: the stud's face is a
// shaft's, so it carries a screw's thread, started at the stud's tip
// and run up towards the plate; the hole's face is a hole's, 8.5 mm
// across, a tap drill's size, so it is offered M10 and, FullLength set,
// is threaded through the plate. Each thread is drawn on a circle in a
// datum at its face's end, which follows the face. The script checks
// the solid, prints the threads, and saves threadface.tlspr unless
// given "-".
package main

import (
	"fmt"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// check stops the script unless the group's solid is sound.
func check(g api.Group) {
	r := g.Analyze().Check()
	fmt.Printf("%s: %.2f mm³, sound %v\n", g, r.Volume, r.Sound())
	if !r.Sound() {
		fmt.Fprintf(os.Stderr, "%s: want a sound solid\n", g)
		os.Exit(1)
	}
}

func main() {
	write := len(os.Args) < 2 || os.Args[1] != "-"
	doc := api.Current()

	doc.Sketch.Rect(api.V(-15, -15, 0), api.V(15, 15, 0))
	doc.Features.Extrude(10, "plate")

	// The stud hangs 20 mm under the plate.
	doc.Features.SketchIn(doc.Workplane("XY"), "stud")
	doc.Sketch.Circle(api.V(-8, 0, 0), 4)
	doc.Features.Extrude(-20, "stud")

	// The hole goes through the plate.
	doc.Features.SketchIn(doc.Workplane("XY"), "hole")
	doc.Sketch.Circle(api.V(8, 0, 0), 4.25)
	hole := doc.Features.Extrude(12, "drill")
	hole.SetCombine(1) // difference

	// Each face is picked on the seam between two of its quarter patches.
	s := doc.Active().Solid()
	stud := doc.Features.ThreadOnFace(s.FaceAt(api.V(-4, 0, -10)), api.ThreadOptions{Length: 12})
	check(stud)
	s = doc.Active().Solid()
	tapped := doc.Features.ThreadOnFace(s.FaceAt(api.V(12.25, 0, 5)), api.ThreadOptions{FullLength: true})
	check(tapped)

	for _, g := range []api.Group{stud, tapped} {
		o := g.AsThread().Options()
		fmt.Printf("%s: internal %v, %.2f mm long, through %v\n", o.Designation, o.Internal, o.Length, o.Through)
	}
	if o := tapped.AsThread().Options(); o.Designation != "M10" || !o.Internal || !o.Through {
		fmt.Fprintln(os.Stderr, "the hole was not tapped M10 through")
		os.Exit(1)
	}

	if write {
		if err := doc.File.Save("threadface.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
