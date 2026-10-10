// A moulded boss: a rounded block with a bore, its sides drafted so
// that it comes out of the mould, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/draft/draft.go [degrees] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/draft/draft.go
//
// An extrusion's draft leans its sides into the solid from the sketch's
// plane: here a 40 x 30 rectangle with corners of radius 6 and a bore of
// radius 5, extruded 20 high, drafted 3 degrees unless given. The
// outside narrows to the top, its corners cones; the bore widens, a cone
// too. Each section is the sketch offset by the height times the
// angle's tangent, so the volume is the integral of a quadratic, which
// Simpson's rule gives exactly. The script checks the solid against it,
// prints it, and saves draft.tlspr, unless given "-".
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
	deg, write := 3.0, true
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if v, err := strconv.ParseFloat(a, 64); err == nil {
			deg = v
		}
	}
	w, d, r, h, bore := 40.0, 30.0, 6.0, 20.0, 5.0
	doc := api.Current()
	// The outline, counter-clockwise: each side, then the corner after it.
	x, y := w/2-r, d/2-r
	corners := []api.Vec{api.V(x, y, 0), api.V(-x, y, 0), api.V(-x, -y, 0), api.V(x, -y, 0)}
	for k, c := range corners {
		a0 := float64(k) * math.Pi / 2
		at := func(a float64) api.Vec { return c.Plus(api.V(r*math.Cos(a), r*math.Sin(a), 0)) }
		prev := corners[(k+3)%4]
		doc.Sketch.Line(prev.Plus(api.V(r*math.Cos(a0), r*math.Sin(a0), 0)), at(a0))
		doc.Sketch.Arc(c, at(a0), at(a0+math.Pi/2))
	}
	doc.Sketch.Circle(api.V(0, 0, 0), bore)
	boss := doc.Features.Extrude(h, "boss")
	boss.AsExtrude().SetDraft(deg)
	if p := boss.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}

	res := boss.Analyze().Check()
	t := math.Tan(deg * math.Pi / 180)
	section := func(z float64) float64 {
		s := z * t
		return roundedArea(w-2*s, d-2*s, r-s) - math.Pi*(bore+s)*(bore+s)
	}
	want := h / 6 * (section(0) + 4*section(h/2) + section(h))
	fmt.Printf("a %g x %g boss, %g high, drafted %g°: %s, %.1f mm³ (%.1f by the sections)\n",
		w, d, h, deg, map[bool]string{true: "sound", false: "NOT sound"}[res.Sound()], res.Volume, want)
	// The round sides are drawn as chords, which cut inside them.
	if !res.Sound() || math.Abs(res.Volume-want) > 0.01*want {
		os.Exit(1)
	}
	if write {
		if err := doc.File.Save("draft.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
