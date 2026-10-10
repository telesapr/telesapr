// A flanged spindle checked by the ray-cast oracle, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/raycheck/raycheck.go
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/raycheck/raycheck.go
//
// A plate with a bore, a turned spindle joined through it and a slot
// cut across: Group.RayCheck rebuilds the solid from the groups'
// definitions alone and measures it with rays, which a sound B-rep's
// mesh agrees with ray by ray; Group.Probe says where a line passes
// through it. Analyze > Ray-Cast Check does the same from the window.
// The script exits with 1 where the two disagree.
package main

import (
	"fmt"
	"math"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	doc := api.Current()
	doc.Rect(api.V(-30, -30, 0), api.V(30, 30, 0))
	doc.Extrude(8, "plate")

	doc.SketchIn(doc.Workplane("ZX"), "profile")
	axis := doc.Line(api.V(0, 0, -5), api.V(0, 0, 45))
	axis.SetConstruction(true)
	doc.Polyline(true, api.V(0, 0, -2), api.V(10, 0, -2), api.V(10, 0, 12), api.V(6, 0, 12),
		api.V(6, 0, 40), api.V(0, 0, 40))
	doc.Lathe(axis, "spindle")

	doc.SketchIn(doc.Workplane("XY"), "slot")
	doc.Rect(api.V(-40, -3, 0), api.V(40, 3, 0))
	slot := doc.Extrude(3, "cut")
	slot.SetCombine(api.Difference)

	r := slot.RayCheck()
	fmt.Printf("ray-cast volume %.1f mm³ (grids agree within %.3f%%), the mesh's %.1f mm³\n",
		r.Measure.Volume, 100*r.Measure.Spread(), r.MeshVolume)
	fmt.Printf("%d of %d rays disagree with the mesh; mesh closed: %v\n", len(r.Disagree), r.Rays, r.Manifold.Closed())
	for _, in := range slot.Probe(api.V(0, 0, -10), api.V(0, 0, 1)) {
		fmt.Printf("up the axis, inside from z = %.2f to %.2f\n", in.From-10, in.To-10)
	}
	if len(r.Disagree) != 0 || !r.Manifold.Closed() || math.Abs(r.Measure.Volume-r.MeshVolume) > 0.01*r.Measure.Volume {
		os.Exit(1)
	}
}
