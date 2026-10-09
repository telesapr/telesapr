// A bent tube: a ring swept along a straight, a bend and a straight,
// made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/sweep/sweep.go [bend-radius]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/sweep/sweep.go
//
// The path is drawn in the ZX workplane: up 30 mm from the origin, a
// quarter bend of bend-radius mm (25 unless given), and 40 mm along x,
// joined and made tangent by constraints. The ring, 12 mm across with
// a 2 mm wall, is drawn in XY round the start of the path, and swept
// along it as Features > Sweep Along Path does. The script prints the
// volume against what Pappus's theorem gives.
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	r := 25.0
	if len(os.Args) > 1 {
		v, err := strconv.ParseFloat(os.Args[1], 64)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		r = v
	}

	doc := api.Current()
	doc.SketchIn(doc.Workplane("ZX"), "path")
	up := doc.Line(api.V(0, 0, 0), api.V(0, 0, 30))
	bend := doc.Arc(api.V(r, 0, 30), api.V(0, 0, 30), api.V(r, 0, 30+r))
	along := doc.Line(api.V(r, 0, 30+r), api.V(r+40, 0, 30+r))
	doc.Coincident(up.End(), bend.Start())
	doc.Coincident(bend.End(), along.Start())
	doc.Tangent(up, bend)
	doc.Tangent(bend, along)

	doc.SketchIn(doc.Workplane("XY"), "ring")
	doc.Circle(api.V(0, 0, 0), 6)
	doc.Circle(api.V(0, 0, 0), 4)
	tube := doc.Sweep([]api.Entity{up, bend, along}, "tube")
	if p := tube.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	res := tube.Check()
	area := math.Pi * (6*6 - 4*4)
	want := area * (30 + r*math.Pi/2 + 40)
	fmt.Printf("tube: sound %v, volume %.0f mm³, Pappus gives %.0f\n", res.Sound(), res.Volume, want)
	if !res.Sound() {
		os.Exit(1)
	}
}
