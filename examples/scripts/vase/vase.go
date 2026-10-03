// A vase, lofted smoothly through four profiles, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/vase/vase.go [height] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/vase/vase.go
//
// LoftThrough joins the closed loops of sketches in workplanes, in
// order, by one smooth surface through them all: here a square foot, a
// round belly, a narrow neck and a flared lip, up to the height given,
// 90 mm unless given. The square has four sides and the circles four
// arcs each, so their curves match one to one. The script checks the
// solid sound and its volume between the cylinders of the neck's and
// the belly's radii, prints it, and saves vase.tlspr, unless given "-".
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	h, write := 90.0, true
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if v, err := strconv.ParseFloat(a, 64); err == nil {
			h = v
		}
	}
	doc := api.Current()
	doc.SetChordTolerance(0.05)

	// Each profile's workplane sits on a point up the z axis, drawn in
	// 3d; the foot is in XY.
	at := func(z float64, name string) api.Group {
		if z == 0 {
			return doc.SketchIn(doc.Workplane("XY"), name)
		}
		doc.Sketch3D("")
		o := doc.Point(api.V(0, 0, z))
		return doc.SketchAt(o, api.V(1, 0, 0), api.V(0, 1, 0), name)
	}
	// Variables, not constants: the interpreter mixes untyped float
	// constants with float64s badly.
	foot, belly, neck, lip := 18.0, 30.0, 10.0, 14.0
	profiles := []api.Group{at(0, "foot")}
	doc.Rect(api.V(-foot, -foot, 0), api.V(foot, foot, 0))
	for _, p := range []struct {
		name  string
		r, of float64
	}{{"belly", belly, 0.4}, {"neck", neck, 0.85}, {"lip", lip, 1}} {
		profiles = append(profiles, at(h*p.of, p.name))
		doc.Circle(api.V(0, 0, h*p.of), p.r)
	}

	vase := doc.LoftThrough(profiles, "vase")
	if p := vase.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	r := vase.Check()
	lo, hi := math.Pi*neck*neck*h, math.Pi*belly*belly*h
	fmt.Printf("a vase %g mm high through %d profiles: %s, %.0f mm³ (between %.0f and %.0f)\n",
		h, len(vase.Profiles()), map[bool]string{true: "sound", false: "NOT sound"}[r.Sound()], r.Volume, lo, hi)
	if !r.Sound() || r.Volume < lo || r.Volume > hi {
		os.Exit(1)
	}
	if write {
		if err := doc.Save("vase.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
