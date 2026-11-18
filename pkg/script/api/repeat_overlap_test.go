package api

import (
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// windowedTube is a tube about the X axis, R 7.8 to 8.6 and 51.6 long,
// with six lancet windows cut through its upper wall, x 16.6 to 40.6,
// 30° apart; and a triquetra cut from the plane through the axis turned
// 30° about it, as a decorative tube modelled over MCP on 2026-11-14
// has: six arcs of radius 3
// about the corners of a triangle of side 3 centred on the axis at
// x = 42.4, one corner on the axis, reaching the windows' tips. It gives
// the windows, the cut and the axis.
func windowedTube(t *testing.T) (windows, cut Group, axis Entity, d *Doc) {
	t.Helper()
	d = New()
	axis = d.Sketch.Line(V(0, 0, 0), V(10, 0, 0))
	axis.SetConstruction(true)
	d.Sketch.Polyline(true, V(0, 7.8, 0), V(51.6, 7.8, 0), V(51.6, 8.6, 0), V(0, 8.6, 0))
	d.Features.LatheAbout(axis.Start(), axis, "tube")

	const hw, xa = 2.148198074350922, 36.87921179050258
	d.Features.SketchIn(d.Workplane("XY"), "window-outline")
	d.Sketch.Line(V(16.6, -hw, 0), V(16.6, hw, 0))
	d.Sketch.Line(V(16.6, hw, 0), V(xa, hw, 0))
	d.Sketch.Line(V(xa, -hw, 0), V(16.6, -hw, 0))
	d.Sketch.Arc(V(xa, -hw, 0), V(40.6, 0, 0), V(xa, hw, 0))
	d.Sketch.Arc(V(xa, hw, 0), V(xa, -hw, 0), V(40.6, 0, 0))
	w := d.Features.Extrude(10, "window")
	w.SetCombine(sketch.CombineDifference)
	windows = d.Features.Rotate(6, d.Origin(), axis, -30, "windows")

	triquetraOutline(d, 30, 42.4, 3)
	cut = d.Features.Extrude(10, "triquetra")
	cut.SetCombine(sketch.CombineDifference)
	return windows, cut, axis, d
}

// triquetraOutline draws, in a new sketch in the plane through the X
// axis turned plane degrees about it, six arcs of radius side about the
// corners of a triangle of that side centred on the axis at x = c, one
// corner on the axis: the decorative tube's triquetra.
func triquetraOutline(d *Doc, plane, c, side float64) {
	th := plane * math.Pi / 180
	d.Features.SketchAt(d.Origin(), V(1, 0, 0), V(0, math.Cos(th), math.Sin(th)), "triquetra-outline")
	uv := d.Sketch.UV
	h := side * math.Sqrt(3) / 2
	a, b, cc := uv(c-2*h/3, 0), uv(c+h/3, side/2), uv(c+h/3, -side/2)
	dd, f, gg := uv(c+4*h/3, 0), uv(c-2*h/3, -side), uv(c-2*h/3, side)
	d.Sketch.Arc(cc, dd, b)
	d.Sketch.Arc(b, cc, dd)
	d.Sketch.Arc(a, f, cc)
	d.Sketch.Arc(cc, a, f)
	d.Sketch.Arc(b, gg, a)
	d.Sketch.Arc(a, b, gg)
}

// A cut whose copies, turned about an axis its outline touches, overlap
// one another near the axis, where there is no solid: their union
// failed from three copies, and from four, where copies 0 and 3 meet
// face to face on the plane through the axis, its difference with the
// tube failed too, and the copies were cut one at a time
// (combineStepsInTurn). Their union is sound now (roadmap K3) and is cut
// from the tube at once: the tube is sound at every count, and each copy
// removes no more than the one cut does (less where it overlaps a
// window's tip).
func TestRotateOverlappingCopies(t *testing.T) {
	t.Parallel()
	windows, cut, axis, d := windowedTube(t)
	before := windows.Analyze().Volume()
	one := before - cut.Analyze().Volume()
	if r := cut.Analyze().Check(); !r.Sound() || one < 10 {
		t.Fatalf("the one cut removes %.3f, sound %v", one, r.Sound())
	}
	rot := d.Features.Rotate(2, d.Origin(), axis, 60, "triquetras")
	counts := []int{2, 3, 4, 5, 6}
	if testing.Short() {
		// The first count that failed.
		counts = []int{4}
	}
	last, from := cut.Analyze().Volume(), 1
	// In order: each count is checked against the one before it.
	for _, n := range counts {
		rot.AsRepeat().SetCopies(n)
		a := rot.Analyze()
		if r := a.Check(); !r.Sound() || rot.Problem() != "" {
			t.Fatalf("%d copies: not sound (%s), problem %q", n, r.String(), rot.Problem())
		}
		d.read(func(sk *sketch.Sketch) {
			if rot.get(sk).StepsInTurn {
				t.Errorf("%d copies: combined with the tube one at a time", n)
			}
		})
		// A copy's chords on the tube fall where it is turned to: what
		// one removes is the one cut's to a tenth.
		if more := last - a.Volume(); more < 0 || more > 1.1*float64(n-from)*one {
			t.Errorf("copies %d to %d remove %.3f, the one cut %.3f", from+1, n, more, one)
		}
		last, from = a.Volume(), n
	}
}

// TestRotatedTriquetraSweep turns copies of the triquetra about the axis
// its corner lies on, as a solid of their own, and checks every case by
// the ray-cast oracle (roadmap K3, turned copies meeting face to face):
// sound, the union taken at once, and every ray through the mesh
// agreeing with the copies' union.
func TestRotatedTriquetraSweep(t *testing.T) {
	t.Parallel()
	g := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("copies", 3, 4, 6),
			sweep.Floats("step", 60, 45),
			sweep.Floats("plane", 30, 13),
			sweep.Floats("at", 42.4, 20),
		},
		// 2 of the 24 fail, six copies 45° apart, whose union fails and
		// which are then combined one at a time (TestTurnedCopiesUnionSweep's
		// cases left).
		MaxFailRate:       0.084,
		SampleMaxFailRate: 0.125,
		Sample:            8,
		Parallel:          true,
	}
	g.Run(func(p sweep.Point) error {
		d := New()
		axis := d.Sketch.Line(V(0, 0, 0), V(10, 0, 0))
		axis.SetConstruction(true)
		triquetraOutline(d, p.Float("plane"), p.Float("at"), 3)
		d.Features.Extrude(10, "triquetra")
		rot := d.Features.Rotate(int(p.Float("copies")), d.Origin(), axis, p.Float("step"), "triquetras")
		if pr := rot.Problem(); pr != "" {
			return errors.New(pr)
		}
		if r := rot.Analyze().Check(); !r.Sound() {
			return fmt.Errorf("not sound: %s", r.String())
		}
		var inTurn bool
		d.read(func(sk *sketch.Sketch) { inTurn = rot.get(sk).StepsInTurn })
		if inTurn {
			return errors.New("the copies' union failed, and they were combined one at a time")
		}
		r := rot.Analyze().RayCheck()
		if len(r.Disagree) > 0 || !r.Manifold.Closed() || math.Abs(r.MeshVolume-r.Measure.Volume) > 0.01*r.Measure.Volume {
			return fmt.Errorf("oracle %.4f, mesh %.4f, %d of %d rays disagree, closed %v",
				r.Measure.Volume, r.MeshVolume, len(r.Disagree), r.Rays, r.Manifold.Closed())
		}
		return nil
	}).Require(t)
}
