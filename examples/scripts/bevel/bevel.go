// A plate with one long top edge bevelled by a chamfer of two
// distances, made by a script (roadmap 28.1).
//
// Without a window:
//
//	telesapr-cli run examples/scripts/bevel/bevel.go [across] [down]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/bevel/bevel.go
//
// The front top edge of a 60 x 20 x 6 plate is cut back across mm on
// the top and down mm on the front (6 and 2 unless given), as New Group
// > Chamfer Edges does with a second distance set on its screen. Which
// face takes the first distance is the one of the lower identity, so
// the script reads the cut back from the solid and swaps the distances
// when they came out the other way round. It prints the volume taken
// off: half the two distances' product along the edge.
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func arg(i int, def float64) float64 {
	if len(os.Args) <= i {
		return def
	}
	v, err := strconv.ParseFloat(os.Args[i], 64)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	return v
}

func main() {
	across, down := arg(1, 6), arg(2, 2)

	doc := api.Current()
	doc.Rect(api.V(0, 0, 0), api.V(60, 20, 0))
	plate := doc.Extrude(6, "plate")
	before := plate.Check().Volume

	// The front top edge: the copy of the sketch's front line at the
	// far end of the extrusion.
	var edge api.Entity
	for _, e := range plate.Entities() {
		if e.Kind() != "line" {
			continue
		}
		s, f := e.Start().Pos(), e.End().Pos()
		if s.Z == 6 && f.Z == 6 && s.Y == 0 && f.Y == 0 {
			edge = e
		}
	}
	// A chamfer starts with one distance, on both faces: the smaller,
	// which fits both.
	c := doc.Chamfer([]api.Entity{edge}, math.Min(across, down), "bevel")
	c.SetDistance(across)
	c.SetSecondDistance(down)

	// The top face's corner at x = 0 shows how far the top is cut back.
	cutTop := 0.0
	for _, v := range c.Solid().Vertices() {
		if p := v.Pos(); p.X == 0 && math.Abs(p.Z-6) < 1e-9 && p.Y > 1e-9 && p.Y < 20-1e-9 {
			cutTop = p.Y
		}
	}
	if math.Abs(cutTop-across) > 1e-9 {
		c.SwapDistances()
	}
	if p := c.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	r := c.Check()
	fmt.Printf("bevel %g across and %g down: %.2f mm³ taken off, sound %v\n",
		across, down, before-r.Volume, r.Sound())
	if !r.Sound() || math.Abs(before-r.Volume-0.5*across*down*60) > 1e-6 {
		os.Exit(1)
	}
}
