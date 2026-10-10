// A horn lofted from a wide circle to a narrow one along two guide
// curves that flare it, made by a script (roadmap 28.3).
//
// Without a window:
//
//	telesapr-cli run examples/scripts/horn/horn.go [flare]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/horn/horn.go
//
// A circle of radius 20 on the XY plane is lofted to one of radius 6,
// 60 above. Free, the loft is a cone's frustum; two cubics drawn in the
// XZ plane from one circle to the other, one each side, curve inwards
// and hold it, as Features > Guide Loft by Selected Curves does, so
// that it flares out at the mouth (by flare mm at the cubics' inner
// handles, 12 unless given). The script prints the volume before and
// after.
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	flare := 12.0
	if len(os.Args) > 1 {
		v, err := strconv.ParseFloat(os.Args[1], 64)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		flare = v
	}

	doc := api.Current()
	doc.Edit.SetChordTolerance(0.01)
	mouth := doc.Features.SketchIn(doc.Workplane("XY"), "mouth")
	doc.Sketch.Circle(api.V(0, 0, 0), 20)
	doc.Features.Sketch3D("guides")
	right := doc.Sketch.Cubic(api.V(20, 0, 0), api.V(20-flare, 0, 5), api.V(6, 0, 30), api.V(6, 0, 60))
	left := doc.Sketch.Cubic(api.V(-20, 0, 0), api.V(-20+flare, 0, 5), api.V(-6, 0, 30), api.V(-6, 0, 60))
	doc.Features.Sketch3D("")
	at := doc.Sketch.Point(api.V(0, 0, 60))
	throat := doc.Features.SketchAt(at, api.V(1, 0, 0), api.V(0, 1, 0), "throat")
	doc.Sketch.Circle(api.V(0, 0, 60), 6)
	horn := doc.Features.Loft(throat, mouth, "horn")
	cone := horn.Analyze().Check().Volume
	horn.AsLoft().SetGuides([]api.Entity{right, left})
	if p := horn.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	r := horn.Analyze().Check()
	fmt.Printf("a cone's frustum: %.0f mm³; flared by %g along its guides: %.0f mm³, sound %v\n", cone, flare, r.Volume, r.Sound())
	if !r.Sound() {
		os.Exit(1)
	}
}
