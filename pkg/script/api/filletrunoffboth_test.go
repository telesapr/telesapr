package api

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
)

// gabledPlateOnRod is a rod of radius 5 along x from 0 to 40 with a
// plate 2 wide standing on it from x0 to x0+L, its top gabled: hEnd
// above the foot of the plate's side on the rod at its ends, hMid at its
// middle. It gives the edge of the plate's side at y = 1 on the rod and
// the volume before.
func gabledPlateOnRod(d *Doc, x0, l, hEnd, hMid float64) (SolidEdge, float64) {
	d.Features.SketchIn(d.Workplane("YZ"), "")
	d.Sketch.Circle(V(0, 0, 0), 5)
	d.Features.Extrude(40, "rod")
	foot := math.Sqrt(24)
	d.Features.SketchIn(d.Workplane("ZX"), "")
	d.Sketch.Polyline(true, V(x0, 0, 3), V(x0+l, 0, 3), V(x0+l, 0, foot+hEnd), V(x0+l/2, 0, foot+hMid),
		V(x0, 0, foot+hEnd))
	plate := d.Features.Extrude(2, "plate")
	plate.SetTwoSided(true)
	return plate.Solid().EdgeAt(V(x0+l/2, 1, foot)), plate.Analyze().Check().Volume
}

// Chamfers of the edge a short plate with a gabled top stands on a rod
// by, which run off the plate's side past both slopes of its top, one at
// each end: the two cuts lie apart along the edge, but on a chamfer's
// few patches both crossed the middle one, and the chamfer was refused
// as running off "at both its ends". A section put between the cuts
// gives each its own patches (topo's runOffApart). Sound, and adding
// what OpenCASCADE's makeChamfer adds within 0.3%
// (scripts/fillet-runoff-both-fixtures.py), at 200 segments a curve as
// TestChamferRunsOffBand. Plates as short whose chamfers cut a patch
// each were made before, as are the fillets of these sizes.
func TestChamferRunsOffBothEnds(t *testing.T) {
	t.Parallel()
	cases := []struct{ l, r, occ float64 }{
		{3.5, 1.4, 2.811173},
		{4.5, 1.7, 5.045761},
		{5, 1.75, 5.941622},
		{5, 1.8, 6.206340},
		{5.5, 1.9, 7.512083},
	}
	if testing.Short() || fixtures.RaceDetector {
		cases = cases[2:3]
	}
	for _, c := range cases {
		d := New()
		d.Edit.SetChordTolerance(0.003)
		d.Edit.SetMaxSegments(200)
		edge, before := gabledPlateOnRod(d, 10, c.l, 0.58, 0.58+c.l/4)
		g, err := blendOrRefusal(d, edge, c.r, true)
		if err != nil {
			t.Errorf("plate %g long, chamfer of %g: %v", c.l, c.r, err)
			continue
		}
		rep := g.Analyze().Check()
		added := rep.Volume - before
		if !rep.Sound() {
			t.Errorf("plate %g long, chamfer of %g: %s", c.l, c.r, rep.Summary())
		}
		if math.Abs(added-c.occ) > 0.003*c.occ {
			t.Errorf("plate %g long, chamfer of %g added %g, OpenCASCADE's %g", c.l, c.r, added, c.occ)
		}
	}
}
