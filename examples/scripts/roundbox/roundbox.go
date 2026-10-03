// A box with every edge rounded, made by a script.
//
// Without a window:
//
//	spacesolve-cli run examples/scripts/roundbox/roundbox.go [radius] [-]
//
// In the application: Script > Run File, or
//
//	spacesolve --script examples/scripts/roundbox/roundbox.go
//
// Fillet Edges rounds edges that meet at corners: where three meet, a
// sphere rounds the corner; where two meet, they are mitred. Here all
// twelve edges of a 60 x 40 x 20 block, radius 4 unless given. The
// script checks the solid against the volume worked out from the
// radius, prints it, and saves roundbox.spcs, unless given "-".
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/spacesolve/spacesolve/pkg/script/api"
)

func main() {
	radius, write := 4.0, true
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if r, err := strconv.ParseFloat(a, 64); err == nil {
			radius = r
		}
	}
	const w, d, h = 60.0, 40.0, 20.0
	doc := api.Current()
	doc.SetChordTolerance(0.01)
	doc.Rect(api.V(0, 0, 0), api.V(w, d, 0))
	block := doc.Extrude(h, "block")

	// The block's edges are its line segments: the sketch's four at
	// each end and the four along its sides, some drawn twice.
	var edges []api.Entity
	seen := map[[2]api.Vec]bool{}
	for _, e := range block.Entities() {
		if e.Kind() != "line" {
			continue
		}
		a, b := e.Start().Pos(), e.End().Pos()
		if seen[[2]api.Vec{a, b}] || seen[[2]api.Vec{b, a}] {
			continue
		}
		seen[[2]api.Vec{a, b}] = true
		edges = append(edges, e)
	}
	round := doc.Fillet(edges, radius, "rounded")
	if p := round.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	r := round.Check()
	// Each edge loses its square of the radius less a quarter circle,
	// along its length; each corner the cube the three edges count
	// three times over less the sphere's eighth.
	a := radius * radius * (1 - math.Pi/4)
	want := w*d*h - a*4*(w+d+h) - 8*radius*radius*radius*(7*math.Pi/12-2)
	fmt.Printf("%d edges rounded at %g mm: %s, %.1f mm³ (%.1f by the formula)\n",
		len(edges), radius, map[bool]string{true: "sound", false: "NOT sound"}[r.Sound()], r.Volume, want)
	// The mesh cuts each arc into at most ten chords, losing the
	// segments between them: allow twice that along the edges, for the
	// corners too.
	phi := math.Pi / 2 / 10
	chords := 10 * radius * radius / 2 * (phi - math.Sin(phi)) * 4 * (w + d + h)
	if !r.Sound() || math.Abs(r.Volume-want) > 2*chords {
		os.Exit(1)
	}
	if write {
		if err := doc.Save("roundbox.spcs"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
