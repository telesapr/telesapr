// A spinning top's body: a frustum turned by a lathe, its top's rim
// rounded and its foot chamfered, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/spinner/spinner.go [radius] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/spinner/spinner.go
//
// Fillet Edges and Chamfer Edges take circles where a flat face meets a
// cone about the circle's axis, as a lathe's flat top meets its slanted
// side: the fillet is a torus all round, the chamfer a cone. Here a
// frustum 20 across its foot, 12 across its top and 8 high, its top's
// rim rounded at 2 mm unless given and its foot chamfered at 1. The
// script checks the solid against the volume Pappus gives, prints it,
// and saves spinner.tlspr, unless given "-".
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// cut is the volume a fillet of radius r, or with flat a chamfer at
// distance r along both faces, takes from a circular edge of radius rad, by Pappus: f and
// g are the directions into the two faces in the half plane through the
// axis, as (along the radius, along the axis).
func cut(rad, r float64, f, g [2]float64, flat bool) float64 {
	beta := math.Acos(f[0]*g[0] + f[1]*g[1])
	s := r / math.Tan(beta/2)
	if flat {
		s = r
	}
	a, b := rad+s*f[0], rad+s*g[0]
	if flat {
		return s * s * math.Sin(beta) / 2 * 2 * math.Pi * (rad + a + b) / 3
	}
	// The kite between the edge, the tangent points and the arc's centre,
	// less the arc's sector.
	bx, by := f[0]+g[0], f[1]+g[1]
	l := math.Hypot(bx, by)
	c := rad + bx/l*r/math.Sin(beta/2)
	kiteX := (rad + a + c + rad + c + b) / 6
	alpha := (math.Pi - beta) / 2
	sectorX := c - bx/l*2*r*math.Sin(alpha)/(3*alpha)
	return 2 * math.Pi * (r*s*kiteX - alpha*r*r*sectorX)
}

func main() {
	radius, write := 2.0, true
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if r, err := strconv.ParseFloat(a, 64); err == nil {
			radius = r
		}
	}
	foot, top, h, chamfer := 10.0, 6.0, 8.0, 1.0
	doc := api.Current()
	doc.SetChordTolerance(0.01)
	doc.SketchIn(doc.Workplane("ZX"), "profile")
	axis := doc.Line(api.V(0, 0, -2), api.V(0, 0, h+2))
	axis.SetConstruction(true)
	doc.Polyline(true, api.V(0, 0, 0), api.V(foot, 0, 0), api.V(top, 0, h), api.V(0, 0, h))
	body := doc.Lathe(axis, "body")

	s := body.Solid()
	doc.FilletSolid([]api.SolidEdge{s.EdgeAt(api.V(top*math.Cos(1), top*math.Sin(1), h))}, radius, "rounded")
	spinner := doc.ChamferSolid([]api.SolidEdge{s.EdgeAt(api.V(foot*math.Cos(1), foot*math.Sin(1), 0))}, chamfer, "foot")
	if p := spinner.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	r := spinner.Check()
	// The side runs out and down from the top, in and up from the foot.
	l := math.Hypot(foot-top, h)
	want := math.Pi*h/3*(foot*foot+foot*top+top*top) -
		cut(top, radius, [2]float64{-1, 0}, [2]float64{(foot - top) / l, -h / l}, false) -
		cut(foot, chamfer, [2]float64{-1, 0}, [2]float64{(top - foot) / l, -h / l}, true)
	fmt.Printf("top rounded at %g mm, foot chamfered at %g: %s, %.1f mm³ (%.1f by Pappus)\n",
		radius, chamfer, map[bool]string{true: "sound", false: "NOT sound"}[r.Sound()], r.Volume, want)
	if !r.Sound() || math.Abs(r.Volume-want) > 1e-3*want {
		os.Exit(1)
	}
	if write {
		if err := doc.Save("spinner.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
