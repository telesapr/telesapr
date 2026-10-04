// A tee of two pipes, the joint filled round by a rolling ball, made by
// a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/tee/tee.go [radius] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/tee/tee.go
//
// A pipe of radius 10 stands on XY, 40 high; a branch of radius 5 leaves
// it square to it, along x, at half its height. Where they meet is a
// closed edge between two cylinders, which no circle or line is: the
// fillet there rolls a ball of the radius given, 2 mm unless given,
// along it, touching both. The script checks the solid sound and the
// volume the fillet adds against OpenCASCADE's for the same tee (3.96,
// 15.77 and 35.37 mm³ for radii 1, 2 and 3), prints it, and saves
// tee.tlspr, unless given "-".
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	radius, write := 2.0, true
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if r, err := strconv.ParseFloat(a, 64); err == nil {
			radius = r
		}
	}
	doc := api.Current()
	doc.SetChordTolerance(0.002)
	doc.SketchIn(doc.Workplane("XY"), "")
	doc.Circle(api.V(0, 0, 0), 10)
	doc.Extrude(40, "pipe")
	doc.SketchIn(doc.Workplane("YZ"), "")
	doc.Circle(api.V(0, 0, 20), 5)
	branch := doc.Extrude(25, "branch")
	if p := branch.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	before := branch.Check().Volume

	// A point of the joint: on the branch (y² + (z-20)² = 25) and on the
	// pipe (x² + y² = 100).
	joint := branch.Solid().EdgeAt(api.V(math.Sqrt(91), 3, 24))
	fill := doc.FilletSolid([]api.SolidEdge{joint}, radius, "joint")
	if p := fill.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	r := fill.Check()
	added := r.Volume - before
	want := map[float64]float64{1: 3.9617, 2: 15.769, 3: 35.370}[radius]
	fmt.Printf("the joint filled round at %g mm: %s, %.2f mm³ added", radius,
		map[bool]string{true: "sound", false: "NOT sound"}[r.Sound()], added)
	if want != 0 {
		fmt.Printf(" (OpenCASCADE %.2f)", want)
	}
	fmt.Println()
	// The mesh's chords along the edge, before and after, count for
	// about a tenth of a mm³ at this tolerance.
	if !r.Sound() || added <= 0 || want != 0 && math.Abs(added-want) > 0.02*want+0.2 {
		os.Exit(1)
	}
	if write {
		if err := doc.Save("tee.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
