package ui

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// byStyle sorts edges by their style.
func byStyle(el srf.SEdgeList) map[int]srf.SEdgeList {
	out := map[int]srf.SEdgeList{}
	for _, e := range el.Edges {
		l := out[e.AuxA]
		l.Edges = append(l.Edges, e)
		out[e.AuxA] = l
	}
	return out
}

// sectionOf is the edges of the section of the drawing's view of kind
// through the solid's middle, by style, hatched hatch apart (0 for
// none: the hatching is in the thin lines a thread's root is).
func sectionOf(t *testing.T, ss *Application, kind string, d *sketch.Drawing, hatch float64) map[int]srf.SEdgeList {
	t.Helper()
	r, u, _ := sketch.ViewDirection(kind)
	g := ss.SK.Group(ss.GW.activeGroupH())
	return byStyle(ss.sectionEdges(g, vec(r), vec(u), 0, hatch, d.Hidden, d.ModelledThreads))
}

// TestDrawingThreadSection cuts a screw along its axis: by the
// convention the cut face reaches the crest, its outline a straight
// line there with no turn of the ridge, and the root is a thin line on
// it each side; modelled, the cut shows the ridge's teeth.
func TestDrawingThreadSection(t *testing.T) {
	t.Parallel()
	ss, doc := threadedModel(t, false)
	d := doc.Drawing.Get()
	cut := sectionOf(t, ss, "top", &d, 0)
	thin := cut[core.StyleDimSolid]
	if len(thin.Edges) != 2 {
		t.Fatalf("%d thin edges, want the root's two lines", len(thin.Edges))
	}
	for _, e := range thin.Edges {
		if abs(abs(e.A.X)-3.3234) > 1e-3 || abs(abs(e.B.X)-3.3234) > 1e-3 || abs(abs(e.B.Y-e.A.Y)-12) > 1e-6 {
			t.Errorf("a root line from %v to %v", e.A, e.B)
		}
	}
	// Over the thread every solid edge lies at the crest or crosses the
	// cut, as the hatch does; none ends between the root and the crest.
	teeth := func(el srf.SEdgeList) int {
		n := 0
		for _, e := range el.Edges {
			for _, p := range []geom.Vector{e.A, e.B} {
				if x := abs(p.X); x > 3.4 && x < 3.95 && p.Y < -0.7 && p.Y > -11.3 {
					n++
				}
			}
		}
		return n
	}
	if n := teeth(cut[core.StyleSolidEdge]); n != 0 {
		t.Errorf("%d ends of edges between the root and the crest", n)
	}
	// The hatch, thin, reaches the crest.
	reach := 0.0
	for _, e := range sectionOf(t, ss, "top", &d, 0.5)[core.StyleDimSolid].Edges {
		if e.A.Y < -1 && e.A.Y > -11 && e.B.Y < -1 && e.B.Y > -11 {
			reach = math.Max(reach, math.Max(abs(e.A.X), abs(e.B.X)))
		}
	}
	if abs(reach-4) > 1e-3 {
		t.Errorf("the cut reaches %g from the axis, want the crest's 4", reach)
	}

	d.ModelledThreads = true
	modelled := sectionOf(t, ss, "top", &d, 0)
	if len(modelled[core.StyleDimSolid].Edges) != 0 || teeth(modelled[core.StyleSolidEdge]) < 20 {
		t.Errorf("modelled: %d thin edges, %d teeth", len(modelled[core.StyleDimSolid].Edges), teeth(modelled[core.StyleSolidEdge]))
	}

	// Across the axis, through the thread, the root is three quarters
	// of a circle on the cut.
	r, u, _ := sketch.ViewDirection("front")
	g := ss.SK.Group(ss.GW.activeGroupH())
	d.ModelledThreads = false
	across := byStyle(ss.sectionEdges(g, vec(r), vec(u), -9, 0, false, false))
	if n := len(across[core.StyleDimSolid].Edges); n != threadSides*3/4 {
		t.Errorf("across: %d thin edges, want the root's three quarters", n)
	}
}

// TestDrawingThreadCrossHole draws a screw with a hole across its
// thread: the hole's edges stay, on the crest's cylinder, where the
// ridge's are gone.
func TestDrawingThreadCrossHole(t *testing.T) {
	t.Parallel()
	ss, doc := threadedModel(t, false)
	doc.Features.SketchIn(doc.Workplane("ZX"), "")
	doc.Sketch.Circle(api.V(0, 0, 6), 1)
	hole := doc.Features.Extrude(12, "hole")
	hole.SetTwoSided(true)
	hole.SetCombine(sketch.CombineDifference)
	ss.GenerateAll(sketch.GenerateAll)
	d := doc.Drawing.Get()
	view := viewOf(t, ss, "top", &d)
	near := 0
	for _, e := range view[core.StyleSolidEdge].Edges {
		mid := e.A.Plus(e.B).ScaledBy(0.5)
		if math.Hypot(mid.X, mid.Y+6) < 1.1 {
			near++
		}
	}
	if near < 8 {
		t.Errorf("%d edges of the hole, want its outline", near)
	}
}

// TestDrawingThreadIsoRoot draws the root in the isometric view too,
// two thin lines; with a run-out they slant out to the crest at the
// thread's end.
func TestDrawingThreadIsoRoot(t *testing.T) {
	t.Parallel()
	ss, doc := threadedModel(t, false)
	d := doc.Drawing.Get()
	if n := len(viewOf(t, ss, "iso", &d)[core.StyleDimSolid].Edges); n != 2 {
		t.Errorf("iso: %d thin edges, want the root's two lines", n)
	}

	rs, _ := newTestApp(t)
	rd := api.NewDoc(rs.ScriptHost())
	rod := rd.Sketch.Circle(api.V(0, 0, 0), 4)
	rd.Features.Extrude(30, "rod")
	rd.Features.Thread(rod, api.ThreadOptions{Designation: "M8", Length: 12, RunOut: 3})
	rdd := rd.Drawing.Get()
	thin := viewOf(t, rs, "top", &rdd)[core.StyleDimSolid]
	if len(thin.Edges) != 4 {
		t.Fatalf("run-out: %d thin edges, want two lines and two slants", len(thin.Edges))
	}
	slants := 0
	for _, e := range thin.Edges {
		if abs(abs(e.A.X)-abs(e.B.X)) > 0.5 {
			slants++
			if abs(abs(e.B.Y-e.A.Y)-3) > 1e-6 {
				t.Errorf("a slant from %v to %v, want 3 long along the axis", e.A, e.B)
			}
		}
	}
	if slants != 2 {
		t.Errorf("%d slants", slants)
	}
}
