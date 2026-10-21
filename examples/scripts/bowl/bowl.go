// A turned bowl, hollowed by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/bowl/bowl.go [thickness] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/bowl/bowl.go
//
// Shell Solid hollows a solid that one lathe or revolve made by its
// profile: the profile offset into its material, each line moved square
// to itself and each arc given a smaller radius about its centre,
// turned again and taken away. Here the profile is an arc of radius 50
// about a point 40 above the foot, from the foot's edge, 30 out, up to
// the rim at the height of the centre: the bowl's wall is a zone of a
// sphere and its foot flat. Walls 3 mm thick unless given. The script checks the solid
// against the volume worked out by the formula for a spherical zone,
// prints it, and saves bowl.tlspr, unless given "-".
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// zone is the volume of the part of a ball of radius r about the
// origin between the heights z0 and z1.
func zone(r, z0, z1 float64) float64 {
	below := func(z float64) float64 { return math.Pi * (r*r*z - z*z*z/3) }
	return below(z1) - below(z0)
}

func main() {
	thick, write := 3.0, true
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if v, err := strconv.ParseFloat(a, 64); err == nil {
			thick = v
		}
	}
	r, h := 50.0, 40.0
	doc := api.Current()
	doc.Edit.SetChordTolerance(0.01)
	// In the ZX plane, turned about the Z axis: the ball of radius r
	// about (0, 0, h), from z = 0 up to its centre.
	doc.Features.SketchIn(doc.Workplane("ZX"), "profile")
	axis := doc.Sketch.Line(api.V(0, 0, -5), api.V(0, 0, h+5))
	axis.SetConstruction(true)
	foot := math.Sqrt(r*r - h*h)
	c := api.V(0, 0, h)
	doc.Sketch.Line(api.V(0, 0, 0), api.V(foot, 0, 0))
	// Arcs run counter-clockwise: from the rim down to the foot.
	doc.Sketch.Arc(c, api.V(r, 0, h), api.V(foot, 0, 0))
	doc.Sketch.Line(api.V(r, 0, h), api.V(0, 0, h))
	doc.Sketch.Line(api.V(0, 0, h), api.V(0, 0, 0))
	body := doc.Features.Lathe(axis, "body")
	if p := body.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}

	top := body.Solid().FaceAt(api.V(1, 1, h))
	bowl := doc.Features.ShellSolid([]api.SolidFace{top}, thick, "bowl")
	if p := bowl.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	res := bowl.Analyze().Check()
	// The solid is the ball about c between heights 0 and h; inside, the
	// ball of radius r - thick between thick and h. Heights from c.
	want := zone(r, -h, 0) - zone(r-thick, thick-h, 0)
	fmt.Printf("a bowl of radius %g, %g high, hollowed to walls %g mm thick: %s, %.1f mm³ (%.1f by the formula)\n",
		r, h, thick, map[bool]string{true: "sound", false: "NOT sound"}[res.Sound()], res.Volume, want)
	// The round walls are drawn as chords, which cut inside them.
	if !res.Sound() || math.Abs(res.Volume-want) > 0.01*want {
		os.Exit(1)
	}
	if write {
		if err := doc.File.Save("bowl.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
