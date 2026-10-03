// A plinth: a block whose top edges are rounded at one radius and whose
// foot's edges at another, in one fillet, made by a script.
//
// Without a window:
//
//	spacesolve-cli run examples/scripts/plinth/plinth.go [top [foot]] [-]
//
// In the application: Script > Run File, or
//
//	spacesolve --script examples/scripts/plinth/plinth.go
//
// A fillet's edges may each have a radius of their own
// (Group.SetEdgeRadius); edges that meet at a corner need one radius,
// so here the top's four edges, mitred at the corners, take the
// fillet's radius, 5 mm unless given, and the foot's four 1.5 mm unless
// given. The script checks the solid against the volume by formula,
// prints it, and saves plinth.spcs, unless given "-".
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/spacesolve/spacesolve/pkg/script/api"
)

func main() {
	radii, write := []float64{5, 1.5}, true
	n := 0
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if r, err := strconv.ParseFloat(a, 64); err == nil && n < 2 {
			radii[n] = r
			n++
		}
	}
	top, foot := radii[0], radii[1]
	w, l, h := 60.0, 40.0, 25.0
	doc := api.Current()
	doc.SetChordTolerance(0.01)
	doc.Rect(api.V(0, 0, 0), api.V(w, l, 0))
	block := doc.Extrude(h, "block")

	// The block's horizontal edges, the top's first.
	var edges []api.Entity
	for _, z := range []float64{h, 0} {
		for _, e := range block.Entities() {
			if e.Kind() != "line" {
				continue
			}
			a, b := e.Start().Pos(), e.End().Pos()
			if math.Abs(a.Z-z) < 1e-6 && math.Abs(b.Z-z) < 1e-6 {
				edges = append(edges, e)
			}
		}
	}
	plinth := doc.Fillet(edges, top, "rounded")
	for i := 4; i < len(edges); i++ {
		plinth.SetEdgeRadius(i, foot)
	}
	if p := plinth.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	r := plinth.Check()
	// Each ring takes a square of its radius less a quarter circle along
	// its length, less at each of its four mitres the part two edges both
	// count, r³(5/3 - π/2).
	mitre := float64(5)/3 - math.Pi/2
	ring := func(r float64) float64 {
		return r*r*(1-math.Pi/4)*2*(w+l) - 4*r*r*r*mitre
	}
	want := w*l*h - ring(top) - ring(foot)
	fmt.Printf("top rounded at %g mm, foot at %g: %s, %.1f mm³ (%.1f by formula)\n",
		top, foot, map[bool]string{true: "sound", false: "NOT sound"}[r.Sound()], r.Volume, want)
	if !r.Sound() || math.Abs(r.Volume-want) > 1e-3*want {
		os.Exit(1)
	}
	if write {
		if err := doc.Save("plinth.spcs"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
