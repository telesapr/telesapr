// A lid: a block whose top edges are rounded at a larger radius than
// its upright edges, in one fillet, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/setback/setback.go [upright [top]] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/setback/setback.go
//
// Where a top edge's fillet is larger than the upright edge's it meets,
// the top can't go on round the upright's arc as in the casing example;
// each fillet is cut short of the corner instead, set back, and the hole
// left filled by patches tangent to the fillets and the faces round it
// (roadmap 22.4). The upright edges are 2 mm unless given, the top's
// 6 mm. The script checks the solid is sound and takes more than the
// fillets alone would, prints its volume, and saves setback.tlspr,
// unless given "-".
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	radii, write := []float64{2, 6}, true
	n := 0
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if r, err := strconv.ParseFloat(a, 64); err == nil && n < 2 {
			radii[n] = r
			n++
		}
	}
	upright, top := radii[0], radii[1]
	w, l, h := 60.0, 40.0, 25.0
	doc := api.Current()
	doc.SetChordTolerance(0.01)
	doc.Rect(api.V(0, 0, 0), api.V(w, l, 0))
	block := doc.Extrude(h, "block")

	// The block's upright edges, then its top's.
	solid := block.Solid()
	edges := solid.EdgesAlong(api.V(0, 0, 1))
	uprights := len(edges)
	for _, e := range solid.Edges() {
		a, b := e.Ends()
		if math.Abs(a.Z-h) < 1e-6 && math.Abs(b.Z-h) < 1e-6 {
			edges = append(edges, e)
		}
	}
	lid := doc.FilletSolid(edges, upright, "rounded")
	for i := uprights; i < len(edges); i++ {
		lid.SetEdgeRadius(i, top)
	}
	if p := lid.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	r := lid.Check()
	// The fillets alone, each run to the corners: a square of its radius
	// less a quarter circle along each edge.
	spandrel := func(r float64) float64 { return r * r * (1 - math.Pi/4) }
	alone := w*l*h - 4*spandrel(upright)*h - spandrel(top)*2*(w+l)
	fmt.Printf("upright edges rounded at %g mm, top at %g, set back at the corners: %s, %.1f mm³ (%.1f with no corners)\n",
		upright, top, map[bool]string{true: "sound", false: "NOT sound"}[r.Sound()], r.Volume, alone)
	if !r.Sound() || r.Volume >= w*l*h || r.Volume < alone-4*top*top*top {
		os.Exit(1)
	}
	if write {
		if err := doc.Save("setback.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
