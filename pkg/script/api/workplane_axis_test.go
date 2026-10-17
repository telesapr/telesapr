package api

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// A workplane made from a point and two line segments, or a point and a
// normal, follows its geometry: moving a line moves the plane.
func TestSketchOnLinesAndNormal(t *testing.T) {
	t.Parallel()
	d := New()
	a := d.Sketch.Line(V(0, 0, 0), V(10, 0, 0))
	b := d.Sketch.Line(V(0, 5, 0), V(0, 15, 0))
	p := d.Sketch.Point(V(3, 3, 0))
	g := d.Features.SketchOnLines(p, a, b, "")
	sk := d.Host().Sketch()
	got := sk.Group(g.H)
	if got.Subtype != sketch.GroupWorkplaneByLineSegments || got.Name != "sketch-in-plane" {
		t.Fatalf("subtype %d, name %q", got.Subtype, got.Name)
	}
	if got.Predef.EntityB != a.H || got.Predef.EntityC != b.H || got.Predef.Origin != p.H {
		t.Errorf("predef %+v", got.Predef)
	}
	if got.Predef.SwapUV || got.Predef.NegateU != true || got.Predef.NegateV != true {
		// u and v run the way the lines were drawn; a line's vector runs
		// from its end to its start, which the flags undo.
		t.Errorf("flags swap %v negate %v %v", got.Predef.SwapUV, got.Predef.NegateU, got.Predef.NegateV)
	}
	if msg := g.Problem(); msg != "" {
		t.Errorf("problem %q", msg)
	}

	// The same point and the normal of a workplane.
	d2 := New()
	p2 := d2.Sketch.Point(V(1, 2, 3))
	n := d2.Workplane("YZ").Normal()
	g2 := d2.Features.SketchOnNormal(p2, n, "tilted")
	got2 := d2.Host().Sketch().Group(g2.H)
	if got2.Subtype != sketch.GroupWorkplaneByPointNormal || got2.Name != "tilted" || got2.Predef.EntityB != n.H {
		t.Errorf("subtype %d, name %q, normal %v", got2.Subtype, got2.Name, got2.Predef.EntityB)
	}
	// A workplane stands for its normal.
	g3 := d2.Features.SketchOnNormal(p2, d2.Workplane("YZ"), "")
	if got3 := d2.Host().Sketch().Group(g3.H); got3.Predef.EntityB != n.H {
		t.Errorf("the workplane's normal is %v, not %v", got3.Predef.EntityB, n.H)
	}

	// Undo is one step and redo brings the group back.
	d2.Edit.Undo()
	if got := d2.Host().Sketch().Group(g3.H); got != nil {
		t.Errorf("undo left %v", got)
	}
	d2.Edit.Redo()
	if got := d2.Host().Sketch().Group(g3.H); got == nil || got.Subtype != sketch.GroupWorkplaneByPointNormal {
		t.Errorf("redo brought back %v", got)
	}

	d3 := New()
	l1 := d3.Sketch.Line(V(0, 0, 0), V(10, 0, 0))
	l2 := d3.Sketch.Line(V(0, 5, 0), V(20, 5, 0))
	l3 := d3.Sketch.Line(V(0, 0, 0), V(0, 5, 0))
	pt := d3.Sketch.Point(V(1, 1, 0))
	mustFail(t, "parallel lines", "parallel", func() { d3.Features.SketchOnLines(pt, l1, l2, "") })
	mustFail(t, "the same line twice", "parallel", func() { d3.Features.SketchOnLines(pt, l1, l1, "") })
	mustFail(t, "a point for a line", "line segment", func() { d3.Features.SketchOnLines(pt, pt, l3, "") })
	mustFail(t, "a line for a point", "point", func() { d3.Features.SketchOnLines(l1, l1, l3, "") })
	mustFail(t, "a line for a normal", "normal", func() { d3.Features.SketchOnNormal(pt, l1, "") })
	mustFail(t, "a line for a point and normal", "point", func() { d3.Features.SketchOnNormal(l1, d3.Workplane("XY").Normal(), "") })
}

// A lathe, a revolve and a helix take an axis as a point and a vector:
// a line segment or a normal; a line segment alone as before.
func TestTurnAboutPointAndVector(t *testing.T) {
	t.Parallel()
	// The normal of the ZX workplane lies along y, in the sketch's plane.
	d := New()
	d.Sketch.Rect(V(10, 0, 0), V(20, 10, 0))
	origin := d.Sketch.Point(V(0, 0, 0))
	la := d.Features.LatheAbout(origin, d.Workplane("ZX").Normal(), "")
	if msg := la.Problem(); msg != "" {
		t.Fatalf("lathe: %q", msg)
	}
	want := math.Pi * (20*20 - 10*10) * 10
	if v := la.Analyze().Volume(); math.Abs(v-want) > 0.01*want {
		t.Errorf("volume %g, want about %g", v, want)
	}
	sk := d.Host().Sketch()
	if g := sk.Group(la.H); g.Predef.Origin != origin.H || g.Predef.EntityB != d.Workplane("ZX").Normal().H {
		t.Errorf("axis %+v", g.Predef)
	}

	// A line segment with a point beside it: the axis through the point.
	d = New()
	d.Sketch.Rect(V(10, 0, 0), V(20, 10, 0))
	axis := d.Sketch.Line(V(0, -5, 0), V(0, 20, 0))
	axis.SetConstruction(true)
	pt := d.Sketch.Point(V(0, 0, 0))
	re := d.Features.RevolveAbout(pt, axis, 90, "")
	if msg := re.Problem(); msg != "" {
		t.Fatalf("revolve: %q", msg)
	}
	if g := d.Host().Sketch().Group(re.H); g.Predef.Origin != pt.H || g.Predef.EntityB != axis.H {
		t.Errorf("axis %+v", g.Predef)
	}

	d = New()
	d.Sketch.Rect(V(10, 0, 0), V(12, 2, 0))
	axis = d.Sketch.Line(V(0, 0, 0), V(0, 20, 0))
	axis.SetConstruction(true)
	hx := d.Features.HelixAbout(d.Sketch.Point(V(0, 0, 0)), axis, 360, 8, "")
	if msg := hx.Problem(); msg != "" {
		t.Fatalf("helix: %q", msg)
	}

	// The line segment alone still takes its own first end.
	d = New()
	d.Sketch.Rect(V(10, 0, 0), V(20, 10, 0))
	axis = d.Sketch.Line(V(0, 0, 0), V(0, 20, 0))
	axis.SetConstruction(true)
	l := d.Features.Lathe(axis, "")
	if g := d.Host().Sketch().Group(l.H); g.Predef.Origin != d.Host().Sketch().MustEntity(axis.H).Point[0] {
		t.Errorf("origin %v", g.Predef.Origin)
	}

	// What is no axis is refused, with the sketch still in its workplane.
	d = New()
	d.Sketch.Rect(V(10, 0, 0), V(20, 10, 0))
	axis = d.Sketch.Line(V(0, 0, 0), V(0, 20, 0))
	pt = d.Sketch.Point(V(0, 0, 0))
	mustFail(t, "a point alone", "line segment or a normal", func() { d.Features.LatheAbout(pt, pt, "") })
	mustFail(t, "a normal alone", "no point", func() { d.Features.Lathe(d.Workplane("ZX").Normal(), "") })
	mustFail(t, "a line for a point", "point", func() { d.Features.LatheAbout(axis, axis, "") })
}
