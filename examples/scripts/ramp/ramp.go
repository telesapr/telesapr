// A ramp with its top's edges rounded, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/ramp/ramp.go [radius] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/ramp/ramp.go
//
// A trapezoid drawn upright, 20 along its foot, 15 along its top and 10
// high, extruded 10: its top meets its front and back square and its
// slope at 116.6 degrees. The top's front, slope and back edges are
// rounded at 2 mm unless given. Where two rounded edges meet, their
// cylinders meet along an ellipse; here the faces don't lie alike about
// the edges, so the cylinder along the slope, which turns further, goes
// on past the ellipse to where it meets the front. The script checks
// the solid, compares its volume with OpenCASCADE's for a radius of 1,
// 2 or 3, prints it, and saves ramp.tlspr, unless given "-".
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
	doc.SetChordTolerance(0.01)
	doc.SketchIn(doc.Workplane("ZX"), "profile")
	doc.Polyline(true, api.V(0, 0, 0), api.V(20, 0, 0), api.V(15, 0, 10), api.V(0, 0, 10))
	ramp := doc.Extrude(10, "ramp")

	// The extrusion runs along y, one way or the other.
	s := ramp.Solid()
	back := 10.0
	if a, b := s.EdgeAt(api.V(20, 0, 0)).Ends(); a.Y < -1 || b.Y < -1 {
		back = -10
	}
	edges := []api.SolidEdge{
		s.EdgeAt(api.V(7, 0, 10)),
		s.EdgeAt(api.V(15, back/2, 10)),
		s.EdgeAt(api.V(7, back, 10)),
	}
	rounded := doc.FilletSolid(edges, radius, "rounded")
	if p := rounded.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	r := rounded.Check()
	fmt.Printf("top rounded at %g mm: %s, %.1f mm³", radius,
		map[bool]string{true: "sound", false: "NOT sound"}[r.Sound()], r.Volume)
	// OpenCASCADE's volumes (FreeCAD's makeFillet), whose curves are
	// exact for this ramp.
	occ := map[float64]float64{1: 1742.944765, 2: 1721.888725, 3: 1686.996378}
	if want, ok := occ[radius]; ok {
		fmt.Printf(" (%.1f by OpenCASCADE)", want)
		if math.Abs(r.Volume-want) > 1e-3*want {
			fmt.Println()
			os.Exit(1)
		}
	}
	fmt.Println()
	if !r.Sound() {
		os.Exit(1)
	}
	if write {
		if err := doc.Save("ramp.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
