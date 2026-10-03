// A knob: a cylinder with its top edge rounded and its foot chamfered,
// made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/knob/knob.go [radius] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/knob/knob.go
//
// Fillet Edges and Chamfer Edges take circles where a flat face meets a
// cylinder: the fillet is a torus all round, the chamfer a cone. Here a
// cylinder of radius 15 and height 20, its top rounded at 4 mm unless
// given and its foot chamfered at 1. The script checks the solid against
// the volume Pappus gives, prints it, and saves knob.tlspr, unless given
// "-".
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
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
	rad, h, chamfer := 15.0, 20.0, 1.0
	doc := api.Current()
	doc.SetChordTolerance(0.01)
	doc.Circle(api.V(0, 0, 0), rad)
	body := doc.Extrude(h, "body")

	// The extrusion's circles: the sketch's copy at its foot and at its
	// top.
	var top, foot api.Entity
	for _, e := range body.Entities() {
		if e.Kind() != "circle" {
			continue
		}
		if e.Center().Pos().Z > h/2 {
			top = e
		} else {
			foot = e
		}
	}
	doc.Fillet([]api.Entity{top}, radius, "rounded")
	knob := doc.Chamfer([]api.Entity{foot}, chamfer, "foot")
	if p := knob.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	r := knob.Check()
	// Pappus: each cut's cross section turned about the axis by its
	// centroid. The fillet takes a square of the radius less a quarter
	// circle, its centroid (10 - 3 pi) / (12 - 3 pi) of the radius in;
	// the chamfer a right triangle, its centroid a third of its side in.
	a := radius * radius * (1 - math.Pi/4)
	c := radius * (10 - 3*math.Pi) / (12 - 3*math.Pi)
	want := math.Pi*rad*rad*h - a*2*math.Pi*(rad-c) - chamfer*chamfer/2*2*math.Pi*(rad-chamfer/3)
	fmt.Printf("top rounded at %g mm, foot chamfered at %g: %s, %.1f mm³ (%.1f by Pappus)\n",
		radius, chamfer, map[bool]string{true: "sound", false: "NOT sound"}[r.Sound()], r.Volume, want)
	// The mesh's chords cut inside the curved faces, by much less than
	// a thousandth of the volume at this tolerance.
	if !r.Sound() || math.Abs(r.Volume-want) > 1e-3*want {
		os.Exit(1)
	}
	if write {
		if err := doc.Save("knob.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
