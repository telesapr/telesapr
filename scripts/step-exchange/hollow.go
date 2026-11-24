//go:build ignore

// A model for scripts/step-exchange.sh with cavities: a block with a
// box and a ball taken away inside it, neither reaching its sides,
// saved as hollow.tlspr in the directory it runs in. Its STEP file is a
// BREP_WITH_VOIDS whose voids are the two cavities.
package main

import (
	"fmt"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	doc := api.Current()
	doc.Sketch.Rect(api.V(0, 0, 0), api.V(60, 30, 0))
	doc.Features.Extrude(30, "block")

	// The planes of the cavities' profiles.
	doc.Features.Sketch3D("planes")
	low := doc.Sketch.NewWorkplane(api.V(0, 0, 10), api.V(1, 0, 0), api.V(0, 1, 0))
	ball := doc.Sketch.NewWorkplane(api.V(42, 15, 15), api.V(1, 0, 0), api.V(0, 0, 1))

	// The box: a rectangle in a plane 10 up, cut 10 deep.
	doc.Features.SketchIn(low, "box profile")
	doc.Sketch.Rect(api.V(5, 10, 10), api.V(25, 20, 10))
	box := doc.Features.Extrude(10, "box")
	box.SetCombine(api.Difference)

	// The ball: a half disc of radius 8 about (42, 15, 15), turned
	// about the line through its centre parallel to Z.
	doc.Features.SketchIn(ball, "ball profile")
	axis := doc.Sketch.Line(api.V(42, 15, 5), api.V(42, 15, 25))
	axis.SetConstruction(true)
	doc.Sketch.Arc(api.V(42, 15, 15), api.V(42, 15, 7), api.V(42, 15, 23))
	doc.Sketch.Line(api.V(42, 15, 23), api.V(42, 15, 7))
	cut := doc.Features.Lathe(axis, "ball")
	cut.SetCombine(api.Difference)
	for _, g := range []api.Group{box, cut} {
		if p := g.Problem(); p != "" {
			fmt.Fprintln(os.Stderr, g.Name()+": "+p)
			os.Exit(1)
		}
	}
	if err := doc.File.Save("hollow.tlspr"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
