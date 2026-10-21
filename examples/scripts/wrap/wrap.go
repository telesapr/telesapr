// A knurl-less handle: a cylinder with a word raised on it and a band
// of squares sunk into a cone below, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/wrap/wrap.go [height] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/wrap/wrap.go
//
// Wrap Sketch Onto Face rolls a sketch drawn in a workplane parallel to
// a cylinder's or a cone's axis onto that face, as paper is, and raises
// it from the face (Emboss) or sinks it in (Engrave). Here the handle is
// a profile lathed about the z axis: a cylinder of radius 12 on a cone.
// The word is sketched in the plane through the axis, running round it;
// the squares on the cone are engraved 0.4 mm. The script checks the
// solid, prints its volume, and saves wrap.tlspr, unless given "-".
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	height, write := 0.8, true
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if v, err := strconv.ParseFloat(a, 64); err == nil {
			height = v
		}
	}
	doc := api.Current()

	// The handle: a cone from radius 16 at z = 0 to 12 at z = 10, then a
	// cylinder of radius 12 to z = 40.
	doc.Features.SketchIn(doc.Workplane("ZX"), "profile")
	axis := doc.Sketch.Line(api.V(0, 0, -1), api.V(0, 0, 41))
	axis.SetConstruction(true)
	doc.Sketch.Polyline(true, api.V(0, 0, 0), api.V(16, 0, 0), api.V(12, 0, 10),
		api.V(12, 0, 40), api.V(0, 0, 40))
	handle := doc.Features.Lathe(axis, "handle")
	cylinder := handle.Solid().FaceAt(api.V(12, 0, 25))
	cone := handle.Solid().FaceAt(api.V(14, 0, 5))

	// The word, in the plane through the axis, its x running round it.
	doc.Features.SketchAt(doc.Origin(), api.V(1, 0, 0), api.V(0, 0, 1), "word")
	doc.Sketch.Text("GRIP", "", api.V(-9, 0, 22), 6)
	word := doc.Features.Emboss(cylinder, height, "raised")

	// Squares round the cone, each its own loop.
	doc.Features.SketchAt(doc.Origin(), api.V(1, 0, 0), api.V(0, 0, 1), "squares")
	for i := 0; i < 5; i++ {
		x := -11.0 + 4.6*float64(i)
		doc.Sketch.Rect(api.V(x, 0, 3), api.V(x+2.6, 0, 6))
	}
	squares := doc.Features.Engrave(cone, 0.4, "sunk")

	for _, g := range []api.Group{word, squares} {
		if p := g.Problem(); p != "" {
			fmt.Fprintf(os.Stderr, "%s: %s\n", g.Name(), p)
			os.Exit(1)
		}
	}
	r := squares.Analyze().Check()
	fmt.Printf("a handle with a word raised %g mm and squares sunk 0.4 mm: %s, %.1f mm³\n",
		height, map[bool]string{true: "sound", false: "NOT sound"}[r.Sound()], r.Volume)
	if !r.Sound() {
		os.Exit(1)
	}
	if write {
		if err := doc.File.Save("wrap.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
