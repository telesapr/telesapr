package api

import (
	"math"
	"strings"
	"testing"
)

// A band whose side cuts a lathed cone along a hyperbola: the edge
// between them runs from the base to the band's end, a flat face at
// each end, and the rolling ball rounds it (roadmap 22.2): the fillet
// fills the corner, the chamfer too, each sound and adding about what
// its section's area along the edge gives.
func TestFilletFlatOnConeOffAxis(t *testing.T) {
	for _, what := range []string{"a fillet", "a chamfer"} {
		d := New()
		d.SetChordTolerance(0.002)
		d.SketchIn(d.Workplane("ZX"), "")
		axis := d.Line(V(0, 0, -2), V(0, 0, 32))
		axis.SetConstruction(true)
		d.Polyline(true, V(0, 0, 0), V(10, 0, 0), V(4, 0, 30), V(0, 0, 30))
		d.Lathe(axis, "")
		d.SketchIn(d.Workplane("XY"), "")
		d.Rect(V(6, -3, 0), V(20, 3, 0))
		band := d.Extrude(25, "band")
		if p := band.Problem(); p != "" {
			t.Fatal(p)
		}
		before := band.Check().Volume
		edge := band.Solid().EdgeAt(V(math.Sqrt(64-9), 3, 10))
		var g Group
		if what == "a fillet" {
			g = d.FilletSolid([]SolidEdge{edge}, 1, "")
		} else {
			g = d.ChamferSolid([]SolidEdge{edge}, 1, "")
		}
		if p := g.Problem(); p != "" {
			t.Fatalf("%s: %s", what, p)
		}
		r := g.Check()
		added := r.Volume - before
		// The corner between the band's side and the cone is wide: a
		// fillet of 1 fills about 1.27 over the edge, a chamfer at 1 about
		// 7.88, the same within a few thousandths at chord tolerances from
		// 0.002% to 0.005%.
		lo, hi := 1.2, 1.35
		if what == "a chamfer" {
			lo, hi = 7.6, 8.1
		}
		if !r.Sound() || added < lo || added > hi {
			t.Errorf("%s of the band's edge on the cone: %s, added %g, want %g to %g", what, r.Summary(), added, lo, hi)
		}
	}
}

// A chamfer of 2 on the band's edge on the cone would run off the band's
// side past its top, at its end on the band's end face: refused there
// (roadmap 22.3), and nothing changed.
func TestChamferRunsOffBand(t *testing.T) {
	d := New()
	d.SetChordTolerance(0.003)
	d.SketchIn(d.Workplane("ZX"), "")
	axis := d.Line(V(0, 0, -2), V(0, 0, 32))
	axis.SetConstruction(true)
	d.Polyline(true, V(0, 0, 0), V(10, 0, 0), V(4, 0, 30), V(0, 0, 30))
	d.Lathe(axis, "")
	d.SketchIn(d.Workplane("XY"), "")
	d.Rect(V(6, -3, 0), V(20, 3, 0))
	band := d.Extrude(25, "band")
	n := len(d.Groups())
	edge := band.Solid().EdgeAt(V(math.Sqrt(64-9), 3, 10))
	func() {
		defer func() {
			err, _ := recover().(error)
			if err == nil || !strings.Contains(err.Error(), "runs off the face on side 2 here, past the end of its edge") {
				t.Errorf("a chamfer of 2: %v", err)
			}
		}()
		d.ChamferSolid([]SolidEdge{edge}, 2, "")
	}()
	if len(d.Groups()) != n {
		t.Errorf("%d groups after the refusal, %d before", len(d.Groups()), n)
	}
}
