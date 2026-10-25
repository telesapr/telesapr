package ui

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// cosmeticBolt is the bolt of boltModel with a cosmetic thread on its
// shank, picked by its face; with a nut's block beside it, threaded
// cosmetic through its hole when nut is set.
func cosmeticBolt(t *testing.T, nut bool) (*Application, *api.Doc) {
	t.Helper()
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	doc.Sketch.Circle(api.V(0, 0, 0), 5)
	rod := doc.Features.Extrude(30, "shank")
	doc.Features.ThreadOnFace(rod.Solid().FaceAt(api.V(5, 0, 15)), api.ThreadOptions{Designation: "M10", Length: 18, Cosmetic: true})
	if nut {
		doc.Features.SketchIn(doc.Workplane("XY"), "nut-sketch")
		doc.Sketch.Rect(api.V(20, -8, 0), api.V(36, 8, 0))
		doc.Features.Extrude(8, "nut")
		doc.Features.SketchIn(doc.Workplane("XY"), "hole-sketch")
		doc.Sketch.Circle(api.V(28, 0, 0), 4.25)
		hole := doc.Features.Extrude(10, "hole")
		hole.SetCombine(sketch.CombineDifference)
		c := 4.25 / math.Sqrt2
		doc.Features.ThreadOnFace(hole.Solid().FaceAt(api.V(28+c, c, 4)), api.ThreadOptions{Designation: "M10", Cosmetic: true})
	}
	d := doc.Drawing.Get()
	d.Hidden = true
	doc.Drawing.Set(d)
	return ss, doc
}

// TestDrawingThreadCosmetic draws a cosmetic thread by the convention,
// as a modelled one is drawn: across the axis the shank's outline runs
// on to the free end, the root is two thin lines the thread's length,
// and the shank's own surface leaves nothing between root and crest;
// along the axis the root is three quarters of a thin circle. A nut's
// cosmetic thread in a hole at its tap drill has its root hidden.
func TestDrawingThreadCosmetic(t *testing.T) {
	t.Parallel()
	ss, doc := cosmeticBolt(t, false)
	d := doc.Drawing.Get()
	// The top view looks across the axis: the shank from its foot at
	// y = 0 to its free end at y = -30, threaded from there.
	top := viewOf(t, ss, "top", &d)
	for _, x := range []float64{-5, 5} {
		if !covers(top[core.StyleSolidEdge], x, -30, 0) {
			t.Errorf("the outline at x = %g has a gap", x)
		}
	}
	thin := top[core.StyleDimSolid]
	if len(thin.Edges) != 2 || !covers(thin, 4.188, -30, -12) || !covers(thin, -4.188, -30, -12) {
		t.Errorf("the root: %d thin edges %v, want two the thread's length", len(thin.Edges), thin.Edges)
	}
	for _, e := range top[core.StyleSolidEdge].Edges {
		if x := math.Abs(e.A.X); x > 4.1 && x < 4.95 && math.Abs(e.A.Y-e.B.Y) > 1e-9 {
			t.Errorf("an edge between root and crest, from %v to %v", e.A, e.B)
		}
	}
	front := viewOf(t, ss, "front", &d)
	if n := len(front[core.StyleDimSolid].Edges); n != threadSides*3/4 {
		t.Errorf("front: %d thin edges, want the root's three quarters", n)
	}

	ss, doc = cosmeticBolt(t, true)
	d = doc.Drawing.Get()
	top = viewOf(t, ss, "top", &d)
	var hidden srf.SEdgeList
	for _, e := range top[core.StyleHiddenEdge].Edges {
		if e.A.X > 20 {
			hidden.Edges = append(hidden.Edges, e)
		}
	}
	// The nut's root, at the major diameter, hidden in the block.
	if !covers(hidden, 23, -8, 0) || !covers(hidden, 33, -8, 0) {
		t.Errorf("the nut's root isn't hidden lines through the block: %v", hidden.Edges)
	}
}

// TestCosmeticThreadMarks requires the window's helix on a cosmetic
// thread's crest: as many turns as the thread has, and none for a
// modelled thread, whose turns are its solid.
func TestCosmeticThreadMarks(t *testing.T) {
	t.Parallel()
	ss, doc := cosmeticBolt(t, false)
	g := ss.SK.Group(ss.GW.activeGroupH())
	el := cosmeticThreadMarks(ss.SK, g)
	if want := int(math.Ceil(18 / 1.5 * threadMarkSides)); len(el.Edges) != want {
		t.Errorf("%d segments, want %d", len(el.Edges), want)
	}
	for _, e := range el.Edges {
		if r := math.Hypot(e.A.X, e.A.Y); r < 5 || r > 5.1 {
			t.Fatalf("a mark at radius %g, not just outside the crest", r)
		}
	}
	o := doc.Groups()[len(doc.Groups())-1].AsThread().Options()
	o.Cosmetic = false
	doc.Groups()[len(doc.Groups())-1].AsThread().Edit(o)
	g = ss.SK.Group(ss.GW.activeGroupH())
	if el := cosmeticThreadMarks(ss.SK, g); len(el.Edges) != 0 {
		t.Errorf("%d marks on a modelled thread", len(el.Edges))
	}
}
