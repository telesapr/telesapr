// A band joined to a cone, the edge where its side meets the cone
// rounded by a rolling ball from the base to the band's end, and the
// other side's edge chamfered, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/bandfillet/bandfillet.go [radius] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/bandfillet/bandfillet.go
//
// A cone narrows from radius 10 to 4 over 30; a band 6 wide runs out of
// it from x = 6 to 20, 25 high. Each side of the band meets the cone
// along a hyperbola that ends at the base and at the band's end, flat
// faces both: the fillet there ends in their planes, and above the
// band's end the cone goes on, so the end face grows by the fillet's
// section. The fillet's radius is 1 mm unless given; the chamfer's
// distance twice that, wide enough that at its end on the band's end
// face it runs off the band's side past the band's top: it goes on to
// the top, which cuts it too and grows by its end. The script checks the
// solid sound after each, prints what each adds, and saves
// bandfillet.tlspr, unless given "-".
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	radius, write := 1.0, true
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if r, err := strconv.ParseFloat(a, 64); err == nil {
			radius = r
		}
	}
	doc := api.Current()
	doc.SetChordTolerance(0.003)
	doc.SketchIn(doc.Workplane("ZX"), "")
	axis := doc.Line(api.V(0, 0, -2), api.V(0, 0, 32))
	axis.SetConstruction(true)
	doc.Polyline(true, api.V(0, 0, 0), api.V(10, 0, 0), api.V(4, 0, 30), api.V(0, 0, 30))
	doc.Lathe(axis, "cone")
	doc.SketchIn(doc.Workplane("XY"), "")
	doc.Rect(api.V(6, -3, 0), api.V(20, 3, 0))
	band := doc.Extrude(25, "band")
	if p := band.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	last := band
	// A point of each side's edge: on the band's side (y = ±3) and on the
	// cone, of radius 8 at height 10.
	for _, side := range []float64{3, -3} {
		before := last.Check().Volume
		edge := last.Solid().EdgeAt(api.V(math.Sqrt(64-9), side, 10))
		var g api.Group
		what, size := "filled round", radius
		if side > 0 {
			g = doc.FilletSolid([]api.SolidEdge{edge}, size, "")
		} else {
			what, size = "chamfered", 2*radius
			g = doc.ChamferSolid([]api.SolidEdge{edge}, size, "")
		}
		if p := g.Problem(); p != "" {
			fmt.Fprintln(os.Stderr, p)
			os.Exit(1)
		}
		r := g.Check()
		added := r.Volume - before
		fmt.Printf("the band's side at y = %g %s at %g mm: %s, %.3f mm³ added\n", side, what, size,
			map[bool]string{true: "sound", false: "NOT sound"}[r.Sound()], added)
		if !r.Sound() || added <= 0 {
			os.Exit(1)
		}
		last = g
	}
	if write {
		if err := doc.Save("bandfillet.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
