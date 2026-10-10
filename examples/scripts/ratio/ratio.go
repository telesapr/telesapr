// A tray whose sizes are held to each other by a length ratio and a
// length difference, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/ratio/ratio.go [width] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/ratio/ratio.go
//
// The outline is half as deep as it is wide (doc.Constrain.Ratio, as
// Constrain > Length / Arc Ratio), and the pocket cut into it is 8 mm
// narrower and shallower than the outline (doc.Constrain.Difference,
// Constrain > Length / Arc Difference), so the one width dimension
// sizes the whole tray. The script checks the volume, prints it, and
// saves ratio.tlspr, unless given "-".
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func main() {
	width, write := 80.0, true
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if v, err := strconv.ParseFloat(a, 64); err == nil {
			width = v
		}
	}
	doc := api.Current()
	sketch, constrain := doc.Sketch, doc.Constrain

	outer := sketch.Rect(api.V(0, 0, 0), api.V(70, 30, 0))
	constrain.Coincident(outer[0].Start(), doc.Origin())
	constrain.Length(outer[0], width)
	constrain.Ratio(outer[1], outer[0], 0.5)
	tray := doc.Features.Extrude(10, "tray")

	doc.Features.SketchIn(doc.Workplane("XY"), "pocket-outline")
	// A distance from a line keeps the side the point is on when it is
	// added, so the pocket is drawn near where it goes, and placed
	// before it is sized.
	inner := sketch.Rect(api.V(5, 5, 0), api.V(width-5, width/2-5, 0))
	constrain.Distance(inner[0].Start(), outer[0], 4)
	constrain.Distance(inner[0].Start(), outer[3], 4)
	constrain.Difference(outer[0], inner[0], 8)
	constrain.Difference(outer[1], inner[1], 8)
	pocket := doc.Features.Extrude(6, "pocket")
	pocket.SetCombine(api.Difference)

	r := pocket.Analyze().Check()
	depth := width / 2
	want := width*depth*10 - (width-8)*(depth-8)*6
	fmt.Printf("width %g: %s, %.1f mm³ (%.1f by the sizes)\n",
		width, map[bool]string{true: "sound", false: "NOT sound"}[r.Sound()], r.Volume, want)
	if !r.Sound() || math.Abs(r.Volume-want) > 1e-6*want || tray.Dof() != 1 {
		os.Exit(1)
	}
	if write {
		must(doc.File.Save("ratio.tlspr"))
	}
}
