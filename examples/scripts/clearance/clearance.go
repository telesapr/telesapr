// A pin in a bored plate, and a box hollowed to thin walls: the gap
// between the parts and the thinnest wall, as Analyze > Clearance and
// Walls finds them.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/clearance/clearance.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/clearance/clearance.go
//
// The plate's bore is 10 across and the pin 9: the gap is 0.5 all round.
// It is found on the parts' meshes and followed onto their cylinders,
// so it comes out at 0.5 though the meshes are a little nearer. The box
// beside them is shelled to walls 1.2 thick. The script prints both and
// checks them, and saves clearance.tlspr, unless given "-".
package main

import (
	"fmt"
	"math"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	write := len(os.Args) < 2 || os.Args[1] != "-"
	doc := api.Current()

	// A box beside the plate, open at its top, walls 1.2 thick.
	doc.Sketch.Rect(api.V(30, -10, 0), api.V(50, 10, 0))
	box := doc.Features.Extrude(10, "box")
	top := box.Solid().FaceAt(api.V(40, 0, 10))
	doc.Features.Shell([]api.Entity{top.Entity()}, 1.2, "walls")

	doc.Features.SketchIn(doc.Workplane("XY"), "")
	doc.Sketch.Rect(api.V(-15, -15, 0), api.V(15, 15, 0))
	doc.Features.Extrude(10, "plate").SetCombine(api.Assemble)
	doc.Features.SketchIn(doc.Workplane("XY"), "")
	doc.Sketch.Circle(api.V(0, 0, 0), 5)
	doc.Features.Extrude(10, "bore").SetCombine(api.Difference)
	doc.Features.SketchIn(doc.Workplane("XY"), "")
	doc.Sketch.Circle(api.V(0, 0, 0), 4.5)
	pin := doc.Features.Extrude(10, "pin")
	pin.SetCombine(api.Assemble)

	// Every pair's gap, the nearest first.
	gaps := doc.Assembly.Clearances(0)
	for _, c := range gaps {
		fmt.Printf("%s and %s: %.4f mm (within %.4f)\n", c.A.Name(), c.B.Name(), c.Gap, c.Tolerance)
	}
	// With each part taken by its solids, the gaps between the solids of
	// one part too: here every part is one solid, so they are the same.
	if within := doc.Assembly.ClearancesWithin(0); len(within) != len(gaps) {
		fmt.Fprintf(os.Stderr, "%d gaps by solids, %d by parts\n", len(within), len(gaps))
		os.Exit(1)
	}
	walls := pin.Analyze().WallThickness(1.5)
	fmt.Printf("thinnest wall %.4f mm, %d of %d points under 1.5 mm\n",
		walls.Thinnest.Thickness, len(walls.Thin), walls.Samples)

	if len(gaps) == 0 || gaps[0].B.Name() != "pin" || math.Abs(gaps[0].Gap-0.5) > 1e-6 {
		fmt.Fprintf(os.Stderr, "gaps %v, want the pin 0.5 from the plate\n", gaps)
		os.Exit(1)
	}
	if math.Abs(walls.Thinnest.Thickness-1.2) > 1e-6 || len(walls.Thin) == 0 {
		fmt.Fprintf(os.Stderr, "thinnest wall %g, want 1.2\n", walls.Thinnest.Thickness)
		os.Exit(1)
	}
	if write {
		if err := doc.File.Save("clearance.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
