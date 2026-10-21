// A plate and an arc of a ring whose thickness and angle come from
// parameters, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/extent/extent.go [thick] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/extent/extent.go
//
// An extrusion's depth and a revolve's angle are free until something
// holds them: a dimension, or the group's own depth or angle, held from
// the group screen ("held") or with SetDepth, SetAngle and SetExpr. Here
// the plate's depth is the parameter "thick", 4 unless given, and the
// ring's angle "arc", 120 degrees. Changing either in the parameter
// table (Edit > Parameters) regenerates the solid. The script checks
// both volumes, prints them, and saves extent.tlspr, unless given "-".
package main

import (
	"fmt"
	"math"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

// check stops the script unless the group's volume is want.
func check(g api.Group, want float64) {
	r := g.Analyze().Check()
	fmt.Printf("%s: %.2f mm³, sound %v, %d dof\n", g, r.Volume, r.Sound(), g.Dof())
	if !r.Sound() || math.Abs(r.Volume-want) > 1e-3*want || g.Dof() != 0 {
		fmt.Fprintf(os.Stderr, "%s: want %.2f mm³ and 0 dof\n", g, want)
		os.Exit(1)
	}
}

func main() {
	thick, write := "4", true
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else {
			thick = a
		}
	}
	doc := api.Current()
	must(doc.Params.Set("thick", thick))
	must(doc.Params.Set("arc", "120"))

	// The plate, 40 x 30, its depth the parameter.
	doc.Sketch.Rect(api.V(0, 0, 0), api.V(40, 30, 0))
	plate := doc.Features.Extrude(10, "plate")
	must(plate.SetExpr(sketch.GroupExtent, "thick"))
	t := plate.AsExtrude().Depth()
	check(plate, 40*30*t)

	// An arc of a ring over the plate, clear of it: a 6 x 5 section 20
	// from the z axis, turned by the angle, from a sketch in the XZ plane.
	doc.Features.SketchIn(doc.Workplane("XZ"), "section")
	doc.Sketch.Rect(api.V(20, 0, t+2), api.V(26, 0, t+7))
	axis := doc.Sketch.Line(api.V(0, 0, 0), api.V(0, 0, 10))
	axis.SetConstruction(true)
	ring := doc.Features.Revolve(axis, 90, "ring")
	must(ring.SetExpr(sketch.GroupExtent, "arc"))
	a := ring.AsRevolve().Angle()
	ringVolume := (26*26 - 20*20) * 5 * math.Pi * a / 360
	if math.Abs(a-120) > 1e-9 {
		fmt.Fprintf(os.Stderr, "the ring turns by %g degrees, not 120\n", a)
		os.Exit(1)
	}
	check(ring, 40*30*t+ringVolume)

	if write {
		must(doc.File.Save("extent.tlspr"))
	}
}
