// A pot with rounded corners and a drain hole, hollowed by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/pot/pot.go [thickness] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/pot/pot.go
//
// Shell Solid hollows prisms whose sides are round as well as flat,
// open at their ends: here a 60 x 40 rectangle with corners of radius
// 8, extruded 30 high, with a hole of radius 4 through its foot. The
// inside's corners keep their centres, radius less the thickness; the
// hole gets a wall round it, radius more. Walls 2 mm thick unless given.
// The script checks the solid against the volume worked out from the
// outlines, prints it, and saves pot.tlspr, unless given "-".
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// roundedArea is the area of a w x h rectangle with corners of radius r.
func roundedArea(w, h, r float64) float64 { return w*h - (4-math.Pi)*r*r }

func main() {
	thick, write := 2.0, true
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if v, err := strconv.ParseFloat(a, 64); err == nil {
			thick = v
		}
	}
	w, d, r, h, hole := 60.0, 40.0, 8.0, 30.0, 4.0
	doc := api.Current()
	// The outline, counter-clockwise: each side, then the corner after it.
	x, y := w/2-r, d/2-r
	corners := []api.Vec{api.V(x, y, 0), api.V(-x, y, 0), api.V(-x, -y, 0), api.V(x, -y, 0)}
	for k, c := range corners {
		a0 := float64(k) * math.Pi / 2
		at := func(a float64) api.Vec { return c.Plus(api.V(r*math.Cos(a), r*math.Sin(a), 0)) }
		next := corners[(k+3)%4]
		doc.Line(next.Plus(api.V(r*math.Cos(a0), r*math.Sin(a0), 0)), at(a0))
		doc.Arc(c, at(a0), at(a0+math.Pi/2))
	}
	doc.Circle(api.V(0, 0, 0), hole)
	body := doc.Extrude(h, "body")

	top := body.Solid().FaceAt(api.V(0, y, h))
	pot := doc.ShellSolid([]api.SolidFace{top}, thick, "pot")
	if p := pot.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	res := pot.Check()
	outside := (roundedArea(w, d, r) - math.Pi*hole*hole) * h
	inside := (roundedArea(w-2*thick, d-2*thick, r-thick) - math.Pi*(hole+thick)*(hole+thick)) * (h - thick)
	want := outside - inside
	fmt.Printf("a %g x %g pot, %g high, hollowed to walls %g mm thick: %s, %.1f mm³ (%.1f by the formula)\n",
		w, d, h, thick, map[bool]string{true: "sound", false: "NOT sound"}[res.Sound()], res.Volume, want)
	// The round walls are drawn as chords, which cut inside them.
	if !res.Sound() || math.Abs(res.Volume-want) > 0.01*want {
		os.Exit(1)
	}
	if write {
		if err := doc.Save("pot.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
