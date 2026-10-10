// A flange with a ring of bolt holes, made by a script: one hole cut
// and repeated round a circle (roadmap 28.5).
//
// Without a window:
//
//	telesapr-cli run examples/scripts/flange/flange.go [holes]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/flange/flange.go
//
// A disc of radius 50 and 8 thick has one hole of radius 4 cut on a
// pitch circle of radius 35, which Features > Pattern Along Path
// repeats round the circle, holes times in all (6 unless given), with
// the circle selected: a path that closes shares the copies out round
// it. The script prints the volume, the disc's less the holes'.
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	holes := 6
	if len(os.Args) > 1 {
		n, err := strconv.Atoi(os.Args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		holes = n
	}

	doc := api.Current()
	doc.Sketch.Circle(api.V(0, 0, 0), 50)
	doc.Features.Extrude(8, "disc")

	doc.Features.SketchIn(doc.Workplane("XY"), "hole sketch")
	doc.Sketch.Circle(api.V(35, 0, 0), 4)
	pitch := doc.Sketch.Circle(api.V(0, 0, 0), 35)
	pitch.SetConstruction(true)
	hole := doc.Features.Extrude(8, "hole")
	hole.SetCombine(api.Difference)

	ring := doc.Features.PatternAlongPath([]api.Entity{pitch}, holes, "holes")
	if p := ring.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	r := ring.Analyze().Check()
	want := math.Pi * 8 * (50*50 - float64(holes)*4*4)
	fmt.Printf("%d holes: %.0f mm³ (exactly %.0f), sound %v\n", holes, r.Volume, want, r.Sound())
	if !r.Sound() || math.Abs(r.Volume-want) > 0.02*want {
		os.Exit(1)
	}
}
