// A bracket with a rib, the rib's foot filled all round, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/rib/rib.go [radius] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/rib/rib.go
//
// The bracket is a plate 10 thick bent up by 110 degrees round an arc of
// radius 20, 42 wide. The rib is a triangle 10 thick across the bend,
// from the floor to the wall. The eight edges where the rib meets the
// bracket are filled at 2 mm unless given, up to 3 (the rib's edge on
// the floor is 5 long): along each side a cylinder
// on the floor, a torus round the bend and a cylinder up the wall, and
// a cylinder across each end of the rib's sloping top.
//
// At the rib's four corners a side's fillet meets an end's below an
// edge of the top, which stays sharp and sticks out, while the fillets
// go in. Their cylinders meet along an ellipse that lies outside the
// rib, beyond that edge: the top grows out to it, and the side's
// fillet, which turns further, ends on the top where it has grown. The
// script checks the solid, compares what the corners add with the
// volume worked out, for a radius of 2 or more, prints it, and saves
// rib.tlspr, unless given "-".
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
	doc.Edit.SetChordTolerance(0.003)

	// The bracket's outline: the floor along x, the wall up at 110
	// degrees, the bend's arc tangent to both.
	const thick, bend, reach = 10.0, 20.0, 26.0
	up := api.V(math.Cos(110*math.Pi/180), math.Sin(110*math.Pi/180), 0)
	along := func(p api.Vec, k float64) api.Vec { return api.V(p.X+k*up.X, p.Y+k*up.Y, 0) }
	outer := along(api.V(0, 0, 0), 45)
	// The inner corner the floor's top and the wall's inside would meet
	// at, were it not for the bend.
	corner := api.V(thick*up.Y+(thick+thick*up.X)/up.Y*up.X, thick, 0)
	tangent := bend / math.Tan(55*math.Pi/180)
	onFloor, onWall := api.V(corner.X+tangent, thick, 0), along(corner, tangent)
	inner := api.V(outer.X+thick*up.Y, outer.Y-thick*up.X, 0)
	// The middle of the bend, 235 degrees round its centre.
	midBend := api.V(onFloor.X+bend*math.Cos(235*math.Pi/180), thick+bend+bend*math.Sin(235*math.Pi/180), 0)
	lines := []api.Entity{
		doc.Sketch.Line(outer, api.V(0, 0, 0)),
		doc.Sketch.Line(api.V(0, 0, 0), api.V(50, 0, 0)),
		doc.Sketch.Line(api.V(50, 0, 0), api.V(50, thick, 0)),
		doc.Sketch.Line(api.V(50, thick, 0), onFloor),
	}
	arc := doc.Sketch.Arc(api.V(onFloor.X, thick+bend, 0), onWall, onFloor)
	lines = append(lines, doc.Sketch.Line(onWall, inner), doc.Sketch.Line(inner, outer))
	for i := 0; i+1 < 4; i++ {
		doc.Constrain.Coincident(lines[i].End(), lines[i+1].Start())
	}
	doc.Constrain.Coincident(lines[3].End(), arc.End())
	doc.Constrain.Coincident(arc.Start(), lines[4].Start())
	doc.Constrain.Coincident(lines[4].End(), lines[5].Start())
	doc.Constrain.Coincident(lines[5].End(), lines[0].Start())
	bracket := doc.Features.Extrude(42, "bracket")
	bracket.SetTwoSided(true)

	// The rib: from the floor, reach along it from the corner, to the
	// wall as far up.
	foot, top := api.V(reach, thick, 0), along(corner, reach-corner.X)
	doc.Features.SketchIn(doc.Workplane("XY"), "rib-outline")
	doc.Sketch.Polyline(true, top, corner, foot)
	rib := doc.Features.Extrude(thick, "rib")
	rib.SetTwoSided(true)
	doc.Edit.Regen()
	if p := rib.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}

	// The rib's foot: each side on the floor, round the bend and up the
	// wall, and each end of its top.
	edges := func(s api.Solid, sides, ends bool) []api.SolidEdge {
		var out []api.SolidEdge
		for _, z := range []float64{thick / 2, -thick / 2} {
			if !sides {
				break
			}
			a, b := along(onWall, 0.5), api.V(onFloor.X+0.5, thick, z)
			out = append(out, s.EdgeAt(b), s.EdgeAt(api.V(midBend.X, midBend.Y, z)), s.EdgeAt(api.V(a.X, a.Y, z)))
		}
		if ends {
			out = append(out, s.EdgeAt(foot), s.EdgeAt(top))
		}
		return out
	}
	volume := func(sides, ends bool) float64 {
		g := doc.Features.FilletSolid(edges(rib.Solid(), sides, ends), radius, "")
		defer g.Delete()
		return g.Analyze().Volume()
	}
	bare := rib.Analyze().Volume()
	onlySides, onlyEnds := volume(true, false), volume(false, true)

	filled := doc.Features.FilletSolid(edges(rib.Solid(), true, true), radius, "filled")
	if p := filled.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	r := filled.Analyze().Check()
	corners := r.Volume - onlySides - onlyEnds + bare
	fmt.Printf("rib's foot filled at %g mm: %s, %.1f mm³, of which the corners %.3f mm³", radius,
		map[bool]string{true: "sound", false: "NOT sound"}[r.Sound()], r.Volume, corners)
	// What the four corners hold beyond the sides' fillets and the
	// ends', each alone: the least of the two fillets' heights over the
	// floor, less the side's fillet cut off by the top's plane, summed
	// over the floor beside the rib. That is 0.0600 mm³ a corner at 2 mm,
	// the top sloping at 35 degrees, and goes as the radius cubed; under
	// 2 mm the mesh's chords hide it.
	if radius >= 2 {
		k := radius / 2
		want := 4 * 0.0600158 * k * k * k
		fmt.Printf(" (%.3f worked out)", want)
		if math.Abs(corners-want) > 0.1*want {
			fmt.Println()
			os.Exit(1)
		}
	}
	fmt.Println()
	if !r.Sound() {
		os.Exit(1)
	}
	if write {
		if err := doc.File.Save("rib.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
