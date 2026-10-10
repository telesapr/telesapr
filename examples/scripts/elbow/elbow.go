// A duct with sharp corners in its path, rounded by a corner radius,
// made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/elbow/elbow.go [corner-radius]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/elbow/elbow.go
//
// The path, in the ZX workplane, is three line segments meeting at two
// right-angled corners. The duct, a 6 by 4 mm rectangle round the
// path's start, is swept along it with Group.SetCornerRadius (8 mm
// unless given), as the sweep's screen sets it: each corner becomes an
// arc of that radius on the path, the runs cut back to meet it, so the
// solid is exact; a corner beside an arc, which a mitre can't join, is
// rounded the same way. The script checks the solid sound, and its
// volume against the area times the rounded path's length, by
// Pappus's theorem.
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	r := 8.0
	if len(os.Args) > 1 {
		v, err := strconv.ParseFloat(os.Args[1], 64)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		r = v
	}
	doc := api.Current()
	doc.Edit.SetChordTolerance(0.002)
	doc.Features.SketchIn(doc.Workplane("ZX"), "path")
	a, b, c := api.V(0, 0, 0), api.V(0, 0, 40), api.V(30, 0, 40)
	d := api.V(30, 0, 70)
	path := []api.Entity{doc.Sketch.Line(a, b), doc.Sketch.Line(b, c), doc.Sketch.Line(c, d)}
	doc.Features.SketchIn(doc.Workplane("XY"), "duct")
	doc.Sketch.Rect(api.V(-3, -2, 0), api.V(3, 2, 0))
	duct := doc.Features.Sweep(path, "duct")
	duct.AsSweep().SetCornerRadius(r)
	if p := duct.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	check := duct.Analyze().Check()
	// Two right-angle corners: each cuts r from both runs and adds a
	// quarter circle.
	length := 40 + 30 + 30 - 4*r + 2*r*math.Pi/2
	want := 24 * length
	fmt.Printf("corner radius %g: volume %.3f mm³, %.3f by Pappus; %s\n", r, check.Volume, want, check.Summary())
	if !check.Sound() || math.Abs(check.Volume-want) > 1e-3*want {
		os.Exit(1)
	}
}
