package api

import (
	"math"
	"testing"

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

	th := 30 * math.Pi / 180
	d.Features.SketchAt(d.Origin(), V(1, 0, 0), V(0, math.Cos(th), math.Sin(th)), "triquetra-outline")
	uv := d.Sketch.UV
	const c, side, r = 42.4, 3.0, 3.0
	h := side * math.Sqrt(3) / 2
	a, b, cc := uv(c-2*h/3, 0), uv(c+h/3, side/2), uv(c+h/3, -side/2)
	dd, f, gg := uv(c+4*h/3, 0), uv(c-2*h/3, -r), uv(c-2*h/3, r)
	d.Sketch.Arc(cc, dd, b)
	d.Sketch.Arc(b, cc, dd)
	d.Sketch.Arc(a, f, cc)
	d.Sketch.Arc(cc, a, f)
	d.Sketch.Arc(b, gg, a)
	d.Sketch.Arc(a, b, gg)
	cut = d.Features.Extrude(10, "triquetra")
	cut.SetCombine(sketch.CombineDifference)
	return windows, cut, axis, d
}

// A cut whose copies, turned about an axis its outline touches, overlap
// one another near the axis, where there is no solid: their union
// fails from three copies, and from four, where copies 0 and 3 meet
// face to face on the plane through the axis, its difference with the
// tube failed too. The copies are then cut one at a time, as separate
// cuts are: the tube is sound at every count, and each copy removes no
// more than the one cut does (less where it overlaps a window's tip).
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
		// A copy's chords on the tube fall where it is turned to: what
		// one removes is the one cut's to a tenth.
		if more := last - a.Volume(); more < 0 || more > 1.1*float64(n-from)*one {
			t.Errorf("copies %d to %d remove %.3f, the one cut %.3f", from+1, n, more, one)
		}
		last, from = a.Volume(), n
	}
}
