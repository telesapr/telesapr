// Pegs along a curved rail: a post repeated along a path of a straight
// run, a bend and another run, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/pattern/pattern.go [copies] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/pattern/pattern.go
//
// The path is drawn in the XY workplane: 30 mm along x, a quarter bend
// of radius 20, and 30 mm along y, joined and made tangent by
// constraints. A post 3 mm across stands at the path's start, and New
// Group > Pattern Along Path repeats it along the path, 7 times unless
// given, the last at the far end, each turned with the path. The script
// checks that the posts stand apart, prints the volume, and saves
// pattern.tlspr, unless given "-".
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	copies, write := 7, true
	if len(os.Args) > 1 {
		n, err := strconv.Atoi(os.Args[1])
		if err != nil || n < 2 {
			fmt.Fprintln(os.Stderr, "copies: a whole number, 2 or more")
			os.Exit(2)
		}
		copies = n
	}
	if len(os.Args) > 2 && os.Args[2] == "-" {
		write = false
	}

	doc := api.Current()
	doc.SketchIn(doc.Workplane("XY"), "rail")
	run := doc.Line(api.V(0, 0, 0), api.V(30, 0, 0))
	bend := doc.Arc(api.V(30, 20, 0), api.V(30, 0, 0), api.V(50, 20, 0))
	tail := doc.Line(api.V(50, 20, 0), api.V(50, 50, 0))
	doc.Coincident(run.End(), bend.Start())
	doc.Coincident(bend.End(), tail.Start())
	doc.Tangent(run, bend)
	doc.Tangent(bend, tail)

	doc.SketchIn(doc.Workplane("XY"), "post-sketch")
	doc.Circle(api.V(0, 0, 0), 1.5)
	post := doc.Extrude(10, "post")
	one := post.Check().Volume

	pegs := doc.PatternAlongPath([]api.Entity{run, bend, tail}, copies, "pegs")
	r := pegs.Check()
	fmt.Printf("%d pegs: sound %v, %d faces, volume %.1f mm³ (one %.1f)\n",
		pegs.Copies(), r.Sound(), r.Faces, r.Volume, one)
	if !r.Sound() || math.Abs(r.Volume-float64(copies)*one) > 1e-6*r.Volume {
		fmt.Fprintln(os.Stderr, "the pegs overlap or the solid is not sound")
		os.Exit(1)
	}
	if write {
		if err := doc.Save("pattern.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
