// A spire on a tapered post: a loft from the post's top that carries
// its drafted sides on smoothly, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/spire/spire.go [draft]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/spire/spire.go
//
// A circle of radius 20 is extruded 40 up with a draft of draft degrees
// (8 unless given), narrowing it. The post's top circle is lofted to a
// circle of radius 3 at height 100. Left square to the top, the loft
// would start straight up and the post's taper would break at the top;
// along the post's sides (LoftGroup.SetAlongSides) it leaves at the
// taper's slope, so the post and the spire are one smooth shape. The
// script prints the volume, and whether the solid is sound.
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	draft := 8.0
	if len(os.Args) > 1 {
		v, err := strconv.ParseFloat(os.Args[1], 64)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		draft = v
	}
	slope := math.Tan(draft * math.Pi / 180)

	doc := api.Current()
	doc.Features.SketchIn(doc.Workplane("XY"), "base")
	doc.Sketch.Circle(api.V(0, 0, 0), 20)
	doc.Features.Extrude(40, "post").AsExtrude().SetDraft(draft)
	rTop := 20 - 40*slope

	doc.Features.Sketch3D("")
	at := doc.Sketch.Point(api.V(0, 0, 40))
	top := doc.Features.SketchAt(at, api.V(1, 0, 0), api.V(0, 1, 0), "top")
	doc.Sketch.Circle(api.V(0, 0, 40), rTop)
	doc.Features.Sketch3D("")
	at = doc.Sketch.Point(api.V(0, 0, 100))
	tip := doc.Features.SketchAt(at, api.V(1, 0, 0), api.V(0, 1, 0), "tip")
	doc.Sketch.Circle(api.V(0, 0, 100), 3)

	spire := doc.Features.Loft(top, tip, "spire")
	spire.AsLoft().SetAlongSides(true, false)
	if msg := spire.Problem(); msg != "" {
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
	r := spire.Analyze().Check()
	fmt.Printf("post and spire: %.0f mm³, sound %v\n", r.Volume, r.Sound())
	if !r.Sound() {
		os.Exit(1)
	}
}
