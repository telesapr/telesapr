// A block with its four upright edges rounded, made by a script.
//
// Without a window:
//
//	spacesolve-cli run examples/scripts/fillet/fillet.go [radius] [-]
//
// In the application: Script > Run File, or
//
//	spacesolve --script examples/scripts/fillet/fillet.go
//
// Fillet Edges rounds straight edges between flat faces with an exact
// cylinder, by cutting the faces back along the lines where it touches
// them and putting its face in: here the four upright edges of a 60 x
// 40 x 10 block, radius 8 unless given. The script checks the solid
// against the volume worked out from the radius, prints it, and saves
// fillet.spcs, unless given "-".
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/spacesolve/spacesolve/pkg/script/api"
)

func main() {
	radius, write := 8.0, true
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if r, err := strconv.ParseFloat(a, 64); err == nil {
			radius = r
		}
	}
	doc := api.Current()
	doc.SetChordTolerance(0.01)
	doc.Rect(api.V(0, 0, 0), api.V(60, 40, 0))
	block := doc.Extrude(10, "block")

	// The upright edges are the extrusion's lines along z, one at each
	// corner.
	var upright []api.Entity
	seen := map[api.Vec]bool{}
	for _, e := range block.Entities() {
		if e.Kind() != "line" || math.Abs(e.Dir().Z) < 0.99 {
			continue
		}
		if at := e.Start().Pos(); !seen[at] {
			seen[at] = true
			upright = append(upright, e)
		}
	}
	round := doc.Fillet(upright, radius, "rounded")
	if p := round.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	r := round.Check()
	// Each corner loses a square of the radius less a quarter circle.
	want := 60*40*10 - 4*(radius*radius-math.Pi*radius*radius/4)*10
	fmt.Printf("%d upright edges rounded at %g mm: %s, %.1f mm³ (%.1f by the formula)\n",
		len(upright), radius, map[bool]string{true: "sound", false: "NOT sound"}[r.Sound()], r.Volume, want)
	if !r.Sound() || math.Abs(r.Volume-want) > 0.005*want {
		os.Exit(1)
	}
	if write {
		if err := doc.Save("fillet.spcs"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
