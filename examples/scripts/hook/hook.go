// A hook: a round bar swept up a straight shank and round a bend that
// leaves it at a sharp corner, with no corner radius.
//
//	telesapr --script examples/scripts/hook/hook.go
//
// The path, in the ZX workplane, is a line 30 mm up and an arc of radius
// 12 that sets off from the line's top at a right angle and turns three
// quarters of a turn. At the corner beside the arc no plane can mitre
// the pieces; the sweep joins them where each point of the profile's
// paths along the line and round the arc cross, the outside of the turn
// filled by both pieces carried on past it. The script checks the solid
// sound, and its volume against the bar's area times the path's length,
// which the joint changes by little (what it takes from one piece it
// about gives the other).
package main

import (
	"fmt"
	"math"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	doc := api.Current()
	doc.Edit.SetChordTolerance(0.005)
	doc.Features.SketchIn(doc.Workplane("ZX"), "path")
	top := api.V(0, 0, 30)
	path := []api.Entity{
		doc.Sketch.Line(api.V(0, 0, 0), top),
		doc.Sketch.Arc(api.V(0, 0, 42), top, api.V(12, 0, 42)),
	}
	doc.Features.SketchIn(doc.Workplane("XY"), "bar")
	doc.Sketch.Circle(api.V(0, 0, 0), 2)
	hook := doc.Features.Sweep(path, "hook")
	if p := hook.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	check := hook.Analyze().Check()
	want := math.Pi * 4 * (30 + 12*3*math.Pi/2)
	fmt.Printf("volume %.3f mm³, %.3f the area times the path; %s\n", check.Volume, want, check.Summary())
	if !check.Sound() || math.Abs(check.Volume-want) > 0.02*want {
		os.Exit(1)
	}
}
