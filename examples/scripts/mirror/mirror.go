// A symmetric bracket: one half drawn and extruded, then mirrored, made
// by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/mirror/mirror.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/mirror/mirror.go
//
// The half is an L from the middle plane x = 0 out to x = 30, with a
// hole in its foot, and a bore down its upright cut by a later group;
// Features > Mirror in the YZ workplane, set to the whole solid so
// far, makes the other half, holes and all, and joins it on, a T. The
// script prints what Check Solid finds and saves bracket.tlspr, unless
// given "-".
package main

import (
	"fmt"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	doc := api.Current()
	doc.Sketch.Polyline(true, api.V(0, 0, 0), api.V(30, 0, 0), api.V(30, 6, 0),
		api.V(6, 6, 0), api.V(6, 24, 0), api.V(0, 24, 0))
	// A hole through the foot.
	doc.Sketch.Circle(api.V(18, 3, 0), 1.5)
	half := doc.Features.Extrude(20, "half")
	// A hole down through the upright, cut by a group of its own.
	doc.Features.SketchIn(doc.Workplane("XY"), "bore-sketch")
	doc.Sketch.Circle(api.V(3, 15, 0), 1.5)
	bore := doc.Features.Extrude(20, "bore")
	bore.SetCombine(api.Difference)

	// Mirror reflects the active group in the YZ workplane, and joins
	// the reflection on; set to the whole solid so far, it reflects the
	// half and the bore together, as a body of several groups needs.
	both := doc.Features.Mirror(doc.Workplane("YZ"), "bracket")
	both.AsMirror().SetMirrorSolid(true)
	r := both.Analyze().Check()
	fmt.Printf("bracket: sound %v, %d faces, volume %.1f mm³ (half %.1f)\n",
		r.Sound(), r.Faces, r.Volume, half.Analyze().Check().Volume)
	if !r.Sound() {
		os.Exit(1)
	}
	if len(os.Args) > 1 && os.Args[1] == "-" {
		return
	}
	if err := doc.File.Save("bracket.tlspr"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
