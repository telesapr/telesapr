// A STEP file linked as a reference mesh, made by a script.
//
// Without a window:
//
//	spacesolve-cli run examples/scripts/step/step.go [file.step]
//
// In the application: Script > Run File, or
//
//	spacesolve --script examples/scripts/step/step.go
//
// Given a STEP file, the script links it and prints what was read.
// Given none, it makes one first: a plate with a hole, exported as
// plate.step, then linked back, and the two volumes compared; the
// linked copy is a mesh, so they agree to within its chord tolerance.
// The plate's own solid is suppressed once exported, so the check of
// the link is of the link alone.
package main

import (
	"fmt"
	"math"
	"os"

	"github.com/spacesolve/spacesolve/pkg/script/api"
)

func main() {
	doc := api.Current()
	path := "plate.step"
	var plate api.Group
	plateVolume := 0.0
	if len(os.Args) > 1 {
		path = os.Args[1]
	} else {
		doc.Rect(api.V(0, 0, 0), api.V(60, 40, 0))
		doc.Circle(api.V(30, 20, 0), 8)
		plate = doc.Extrude(6, "original")
		if err := doc.Export(path); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		plateVolume = plate.Check().Volume
		plate.SetSuppress(api.SuppressSolid)
	}

	part, err := doc.Link(path, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	for _, line := range part.LinkInfo() {
		fmt.Println(line)
	}
	if p := part.LinkProblem(); p != "" {
		fmt.Println("not all of it was read:", p)
	}
	points := 0
	for _, e := range part.Entities() {
		if e.Kind() == "point" {
			points++
		}
	}
	r := part.Check()
	fmt.Printf("%s: sound %v, %d triangles, volume %.1f mm³, %d points to constrain to\n",
		part.Name(), r.Sound(), r.Triangles, r.Volume, points)

	if plate.Valid() {
		if math.Abs(r.Volume-plateVolume) > 0.01*plateVolume || !r.Sound() {
			fmt.Fprintf(os.Stderr, "the linked plate's volume is %.1f, the plate's %.1f\n", r.Volume, plateVolume)
			os.Exit(1)
		}
	}
}
