// A column that runs straight up through its narrow waist, made by a
// script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/waist/waist.go [tangency]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/waist/waist.go
//
// Three circles, of radius 15 at the foot, 8 at height 40 and 12 at
// height 80, are lofted smoothly. Left free, the loft bends through the
// waist as the three circles shape it; held square to the waist's plane
// (LoftGroup.SetProfileTangency, by tangency, 0.3 unless given) it runs
// straight up through it for a stretch, the slope across there tangency
// times the loft's length. The script prints the volume free and held,
// and whether the solid is sound.
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	tangency := 0.3
	if len(os.Args) > 1 {
		v, err := strconv.ParseFloat(os.Args[1], 64)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		tangency = v
	}

	doc := api.Current()
	var profiles []api.Group
	for i, c := range []struct{ z, r float64 }{{0, 15}, {40, 8}, {80, 12}} {
		doc.Features.Sketch3D("")
		at := doc.Sketch.Point(api.V(0, 0, c.z))
		profiles = append(profiles, doc.Features.SketchAt(at, api.V(1, 0, 0), api.V(0, 1, 0), fmt.Sprintf("circle%d", i+1)))
		doc.Sketch.Circle(api.V(0, 0, c.z), c.r)
	}

	column := doc.Features.LoftThrough(profiles, "column")
	free := column.Analyze().Check().Volume
	column.AsLoft().SetProfileTangency(profiles[1], tangency)
	if msg := column.Problem(); msg != "" {
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
	r := column.Analyze().Check()
	fmt.Printf("free %.0f mm³, held square to the waist %.0f mm³, sound %v\n", free, r.Volume, r.Sound())
	if !r.Sound() {
		os.Exit(1)
	}
}
