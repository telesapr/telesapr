// A moulded D-shaped boss drafted from its parting plane, made by a
// script (roadmap 28.2).
//
// Without a window:
//
//	telesapr-cli run examples/scripts/parting/parting.go [degrees]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/parting/parting.go
//
// A D of radius 20, its flat side along x, is extruded 16 and drafted
// by degrees (3 unless given) from the plane halfway up, the parting
// line, so that it narrows both ways from there. The flat side stays
// upright: the draft leans the arc's side only, as Features > Draft
// Selected Sides does with the arc selected. Where the arc meets the
// flat side at a sharp corner, the cone and the plane meet in a conic.
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	degrees := 3.0
	if len(os.Args) > 1 {
		v, err := strconv.ParseFloat(os.Args[1], 64)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		degrees = v
	}

	doc := api.Current()
	doc.SetChordTolerance(0.005)
	arc := doc.Arc(api.V(0, 0, 0), api.V(20, 0, 0), api.V(-20, 0, 0))
	doc.Line(api.V(-20, 0, 0), api.V(20, 0, 0))
	boss := doc.Extrude(16, "boss")
	boss.SetDraft(degrees)
	boss.SetDraftNeutral(8)
	boss.SetDraftFaces([]api.Entity{arc})
	if p := boss.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	r := boss.Check()
	// Half a cone's frustum each way from the parting plane.
	x := 8 * math.Tan(degrees*math.Pi/180)
	half := math.Pi * 8 / 3 * (20*20 + 20*(20-x) + (20-x)*(20-x)) / 2
	fmt.Printf("drafted %g°: %.0f mm³ (exactly %.0f), sound %v\n", degrees, r.Volume, 2*half, r.Sound())
	if !r.Sound() || math.Abs(r.Volume-2*half) > 0.005*2*half {
		os.Exit(1)
	}
}
