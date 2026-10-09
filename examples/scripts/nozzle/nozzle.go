// A nozzle lofted from a square to a circle moved aside, square to both
// ends, made by a script (roadmap 28.3).
//
// Without a window:
//
//	telesapr-cli run examples/scripts/nozzle/nozzle.go [tangency]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/nozzle/nozzle.go
//
// A 20 mm square on the XY plane is lofted to a circle of radius 6, 30
// above and 10 aside, as Features > Loft To Selected Profile does; the
// loft's screen then sets both ends to tangency (1 unless given), so
// that the nozzle leaves the square and reaches the circle square to
// them, as a duct that fits pipes at both ends. The script prints the
// volume with the ends free and set.
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	k := 1.0
	if len(os.Args) > 1 {
		v, err := strconv.ParseFloat(os.Args[1], 64)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		k = v
	}

	doc := api.Current()
	doc.SetChordTolerance(0.01)
	square := doc.SketchIn(doc.Workplane("XY"), "square")
	doc.Rect(api.V(-10, -10, 0), api.V(10, 10, 0))
	doc.Sketch3D("")
	at := doc.Point(api.V(10, 0, 30))
	circle := doc.SketchAt(at, api.V(1, 0, 0), api.V(0, 1, 0), "circle")
	doc.Circle(api.V(10, 0, 30), 6)
	loft := doc.Loft(square, circle, "nozzle")
	free := loft.Check().Volume
	loft.SetTangency(k, k)
	if p := loft.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	r := loft.Check()
	fmt.Printf("free ends: %.0f mm³; square to both by %g: %.0f mm³, sound %v\n", free, k, r.Volume, r.Sound())
	if !r.Sound() {
		os.Exit(1)
	}
}
