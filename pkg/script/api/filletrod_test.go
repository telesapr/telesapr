package api

import (
	"math"
	"testing"
)

// plateOnRod is a rod of radius 8 along x, 30 long, with a plate 6 thick
// and l long standing on it, its sides at y = ±3, and the edge where the
// side at y = 3 meets the rod.
func plateOnRod(d *Doc, l float64) SolidEdge {
	d.Features.SketchIn(d.Workplane("YZ"), "")
	d.Sketch.Circle(V(0, 0, 0), 8)
	rod := d.Features.Extrude(30, "rod")
	rod.SetTwoSided(true)
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Rect(V(-l/2, -3, 0), V(l/2, 3, 0))
	plate := d.Features.Extrude(12, "plate")
	return plate.Solid().EdgeAt(V(0, 3, math.Sqrt(55)))
}

// A fillet of 2 between the plate's side and the rod has its foot on the
// rod at y = 4, along the rod's length within 0.03 mm of a row of the
// rod's grid. The ear clipper fanned the strip between them from the
// grid's point at the rod's end to the foot's points, slivers 12 mm long
// whose chords cut under the cylinder and crossed the fillet's mesh by
// its foot, at chord tolerances of 0.002% to 0.005% (radii of 1.9 and
// 2.1 were sound). A curved face's sliver makes it a candidate for the
// constrained Delaunay mesher's mending (srf's earMeshSlivers), and each
// is sound. Under -short, two of the cases that crossed.
func TestFilletFootAlongRodRow(t *testing.T) {
	t.Parallel()
	type tc struct {
		l, tol float64
		segs   int
	}
	cases := []tc{{6, 0.003, 20}, {2, 0.005, 20}}
	if !testing.Short() {
		cases = cases[:0]
		for _, l := range []float64{2, 6, 10} {
			for _, tol := range []float64{0.002, 0.003, 0.005} {
				for _, segs := range []int{20, 200} {
					cases = append(cases, tc{l, tol, segs})
				}
			}
		}
	}
	for _, c := range cases {
		d := New()
		d.Edit.SetChordTolerance(c.tol)
		d.Edit.SetMaxSegments(c.segs)
		g := d.Features.FilletSolid([]SolidEdge{plateOnRod(d, c.l)}, 2, "")
		if p := g.Problem(); p != "" {
			t.Errorf("plate %g long, chord %g%%, %d segments: %s", c.l, c.tol, c.segs, p)
			continue
		}
		if r := g.Analyze().Check(); !r.Sound() {
			t.Errorf("plate %g long, chord %g%%, %d segments: %s", c.l, c.tol, c.segs, r.Summary())
		}
	}
}
