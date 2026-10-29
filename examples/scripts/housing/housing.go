// A turned housing with a square boss out of its side, hollowed by a
// script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/housing/housing.go [thickness] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/housing/housing.go
//
// Shell Solid hollows a solid joined by unions from lathes, revolves and
// extrusions as the union of the parts' insides. Here the body is a
// cylinder of radius 20 and height 40 turned about the Z axis, and the
// boss a square 12 across extruded 30 along X from the axis. The boss's
// foot lies inside the body, so it is moved out by the thickness rather
// than in, and the boss's inside runs into the body's with no wall
// between them. Open at the top, walls 2 mm thick unless given. The
// script checks the solid against the volume worked out by hand, prints
// it, and saves housing.tlspr, unless given "-".
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// chordArea is the integral of √(r² - y²) for y from -h to h: the area
// of the circle of radius r between the lines y = ±h, halved.
func chordArea(r, h float64) float64 {
	prim := func(y float64) float64 { return (y*math.Sqrt(r*r-y*y) + r*r*math.Asin(y/r)) / 2 }
	return prim(h) - prim(-h)
}

func main() {
	thick, write := 2.0, true
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if v, err := strconv.ParseFloat(a, 64); err == nil {
			thick = v
		}
	}
	r, h, side, reach := 20.0, 40.0, 12.0, 30.0
	doc := api.Current()
	doc.Edit.SetChordTolerance(0.01)
	doc.Features.SketchIn(doc.Workplane("ZX"), "profile")
	axis := doc.Sketch.Line(api.V(0, 0, -5), api.V(0, 0, h+5))
	axis.SetConstruction(true)
	doc.Sketch.Polyline(true, api.V(0, 0, 0), api.V(r, 0, 0), api.V(r, 0, h), api.V(0, 0, h))
	body := doc.Features.Lathe(axis, "body")

	// The boss, a square about the body's middle height, drawn in the
	// plane through the axis and extruded out along X.
	doc.Features.SketchIn(doc.Workplane("YZ"), "boss profile")
	doc.Sketch.Rect(api.V(0, -side/2, h/2-side/2), api.V(0, side/2, h/2+side/2))
	boss := doc.Features.Extrude(reach, "boss")
	if p := boss.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}

	top := body.Solid().FaceAt(api.V(1, 1, h))
	housing := doc.Features.ShellSolid([]api.SolidFace{top}, thick, "housing")
	if p := housing.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	res := housing.Analyze().Check()
	// The body and the boss past it; inside, the body moved in, open at
	// the top, and the boss moved in but at its foot, past the body's
	// inside.
	ri, si := r-thick, side-2*thick
	outer := math.Pi*r*r*h + side*(reach*side-chordArea(r, side/2))
	inner := math.Pi*ri*ri*(h-thick) + si*((reach-thick)*si-chordArea(ri, si/2))
	want := outer - inner
	fmt.Printf("a housing %g across with a boss %g out, hollowed to walls %g mm thick: %s, %.1f mm³ (%.1f by the formula)\n",
		2*r, reach-r, thick, map[bool]string{true: "sound", false: "NOT sound"}[res.Sound()], res.Volume, want)
	if !res.Sound() || math.Abs(res.Volume-want) > 0.01*want {
		os.Exit(1)
	}
	if write {
		if err := doc.File.Save("housing.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
