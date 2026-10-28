// A block whose top edges are chamfered in one chamfer, the front one
// wider than the rest, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/chamferedges/chamferedges.go [d [front]] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/chamferedges/chamferedges.go
//
// The four top edges of a 40 x 30 x 20 block are cut in one chamfer of
// 2 mm, unless given, and the front edge gets a distance of its own,
// 4 mm unless given (ChamferGroup.SetEdgeDistance): a lead-in. Where
// the front chamfer meets a side's, above an upright edge left sharp,
// the two meet in a mitre. The script checks the solid is sound and
// loses what the four chamfers' prisms take less what they share at
// the corners, prints its volume, and saves chamferedges.tlspr, unless
// given "-".
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	ds, write := []float64{2, 4}, true
	n := 0
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if d, err := strconv.ParseFloat(a, 64); err == nil && n < 2 {
			ds[n] = d
			n++
		}
	}
	d, front := ds[0], ds[1]
	w, l, h := 40.0, 30.0, 20.0
	doc := api.Current()
	doc.Sketch.Rect(api.V(0, 0, 0), api.V(w, l, 0))
	block := doc.Features.Extrude(h, "block")

	// The top's edges, the front one (along x at y = 0) first.
	var top []api.SolidEdge
	for _, e := range block.Solid().Edges() {
		a, b := e.Ends()
		if math.Abs(a.Z-h) < 1e-9 && math.Abs(b.Z-h) < 1e-9 {
			if math.Abs(a.Y) < 1e-9 && math.Abs(b.Y) < 1e-9 {
				top = append([]api.SolidEdge{e}, top...)
			} else {
				top = append(top, e)
			}
		}
	}
	if len(top) != 4 {
		fmt.Fprintf(os.Stderr, "%d top edges, want 4\n", len(top))
		os.Exit(1)
	}
	c := doc.Features.ChamferSolid(top, d, "top")
	c.AsChamfer().SetEdgeDistance(0, front)
	if p := c.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	r := c.Analyze().Check()
	// Each edge's prism, less what two prisms share at a corner: the
	// integral over the depth w of the two triangles' widths there.
	shared := func(a, b float64) float64 {
		m := math.Min(a, b)
		return a*b*m - (a+b)*m*m/2 + m*m*m/3
	}
	took := front*front/2*w + d*d/2*(w+2*l) - 2*shared(front, d) - 2*shared(d, d)
	fmt.Printf("the top chamfered at %g mm, the front at %g: %s, %.3f mm³ (%.3f expected)\n",
		d, front, map[bool]string{true: "sound", false: "NOT sound"}[r.Sound()], r.Volume, w*l*h-took)
	if !r.Sound() || math.Abs(r.Volume-(w*l*h-took)) > 1e-6*w*l*h {
		os.Exit(1)
	}
	if write {
		if err := doc.File.Save("chamferedges.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
