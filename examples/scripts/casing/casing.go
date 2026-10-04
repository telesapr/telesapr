// A casing: a block whose upright edges are rounded at one radius and
// whose top edges at a smaller one, in one fillet, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/casing/casing.go [upright [top]] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/casing/casing.go
//
// Where edges of different radii meet at a corner the fillet rounds
// them one radius at a time, the larger first (roadmap 22.4): the
// upright edges, 8 mm unless given, leave the top's outline four lines
// joined by arcs, and the top's edges, 3 mm unless given, go on round
// those arcs, a torus at each. The top's radius must be the smaller.
// The script checks the solid against the volume by formula, prints it,
// and saves casing.tlspr, unless given "-".
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	radii, write := []float64{8, 3}, true
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
	casing := doc.FilletSolid(edges, upright, "rounded")
	for i := uprights; i < len(edges); i++ {
		casing.SetEdgeRadius(i, top)
	}
	if p := casing.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	r := casing.Check()
	// Each upright edge takes a square of its radius less a quarter
	// circle along the height; the top takes the same section of its own
	// radius along the lines left between the arcs, and round each arc a
	// quarter turn of it, by Pappus, about the arc's centre at the
	// section's centroid.
	spandrel := func(r float64) float64 { return r * r * (1 - math.Pi/4) }
	centroid := top * (10 - 3*math.Pi) / (12 - 3*math.Pi)
	want := w*l*h - 4*spandrel(upright)*h -
		spandrel(top)*2*(w+l-4*upright) - spandrel(top)*2*math.Pi*(upright-centroid)
	fmt.Printf("upright edges rounded at %g mm, top at %g: %s, %.1f mm³ (%.1f by formula)\n",
		upright, top, map[bool]string{true: "sound", false: "NOT sound"}[r.Sound()], r.Volume, want)
	if !r.Sound() || math.Abs(r.Volume-want) > 1e-3*want {
		os.Exit(1)
	}
	if write {
		if err := doc.Save("casing.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
