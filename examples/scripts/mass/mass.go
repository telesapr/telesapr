// A steel shaft in an aluminium housing: a density for each part, and
// the mass, centre of mass and inertia of each and of the whole.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/mass/mass.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/mass/mass.go
//
// A density belongs to a part: a group assembled into the model is one,
// at its own density, and the solid made before the first of them is
// another, at the density last set among its groups. Analyze > Measure
// Volume gives the masses too, and the group screen sets the density on
// the appearance row. The script prints the masses and checks them
// against the closed forms, and saves mass.tlspr, unless given "-".
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
	doc.Sketch.Rect(api.V(-20, -20, 0), api.V(20, 20, 0))
	housing := doc.Features.Extrude(15, "housing")
	housing.SetDensity(2.7) // aluminium, g/cm³
	doc.Features.SketchIn(doc.Workplane("XY"), "")
	doc.Sketch.Circle(api.V(0, 0, 0), 6)
	shaft := doc.Features.Extrude(40, "shaft")
	shaft.SetCombine(api.Assemble)
	shaft.SetDensity(7.85) // steel

	r := shaft.Analyze().MassProperties()
	for _, p := range r.Parts {
		fmt.Printf("part g%03x: %.1f mm³ at %g g/cm³, %.2f g\n", p.Group.V, p.Volume, p.Density, p.Mass)
	}
	c := r.CenterOfMass
	fmt.Printf("the whole: %.2f g, centre (%.3f, %.3f, %.3f), moment about z %.0f g·mm²\n",
		r.Mass, c.X, c.Y, c.Z, r.Inertia[2][2])

	// The housing is a box, exact; the shaft a cylinder, short by its
	// mesh's chord tolerance.
	box := 40 * 40 * 15 * 2.7 / 1000
	rod := math.Pi * 36 * 40 * 7.85 / 1000
	if len(r.Parts) != 2 || math.Abs(r.Parts[0].Mass-box) > 1e-6 || math.Abs(r.Mass-box-rod) > 0.01*rod {
		fmt.Fprintf(os.Stderr, "masses %v, want %g and %g\n", r.Parts, box, rod)
		os.Exit(1)
	}
	if write {
		if err := doc.File.Save("mass.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
