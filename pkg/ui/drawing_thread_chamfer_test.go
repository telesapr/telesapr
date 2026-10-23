package ui

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/srf"
)

// TestDrawingThreadChamfer draws a bolt whose free end is chamfered,
// by the convention. Across the axis the outline runs from the end, at
// the chamfer's radius, at 45° out to the crest and on along it, the
// chamfer's edges across are thick, and the root's thin lines start
// where the root leaves the cone; along the axis no circle is drawn at
// the chamfer's radius, as drafting leaves it out.
func TestDrawingThreadChamfer(t *testing.T) {
	ss, doc := boltModelWith(t, api.ThreadOptions{Chamfer: true})
	bands := ss.SK.ThreadBands(ss.SK.ActiveGroup())
	if len(bands) != 1 || bands[0].Chamfer == 0 {
		t.Fatalf("bands %+v", bands)
	}
	b := bands[0]
	reach := b.ChamferReach()
	from, _ := b.RootSpan()
	d := doc.Drawing.Get()
	top := viewOf(t, ss, "top", &d)
	solid := top[core.StyleSolidEdge]
	// The free end is at y = -30, the head at 0.
	for _, x := range []float64{-b.Crest, b.Crest} {
		if !covers(solid, x, -30+reach+0.01, 0) {
			t.Errorf("the outline at x = %g has a gap", x)
		}
	}
	// onCone tells whether a point lies on the chamfer's outline.
	onCone := func(x, y float64) bool {
		h := y + 30
		return h > -1e-3 && h < reach+1e-3 && math.Abs(math.Abs(x)-(b.Chamfer+h)) < 0.02
	}
	slanted := map[bool]float64{}
	for _, e := range solid.Edges {
		if onCone(e.A.X, e.A.Y) && onCone(e.B.X, e.B.Y) && math.Abs(e.A.Y-e.B.Y) > 1e-6 {
			slanted[e.A.X > 0] += math.Abs(e.A.Y - e.B.Y)
		}
	}
	for _, side := range []bool{false, true} {
		if math.Abs(slanted[side]-reach) > 0.05 {
			t.Errorf("the chamfer's outline on the side x > 0 %v is %.3f along the axis, want %.3f", side, slanted[side], reach)
		}
	}
	// The end's line, across from the chamfer's radius, and the cone's
	// edge with the crest.
	across := func(y, r float64) bool {
		var el srf.SEdgeList
		for _, e := range solid.Edges {
			if math.Abs(e.A.Y-y) < 0.02 && math.Abs(e.B.Y-y) < 0.02 {
				el.Edges = append(el.Edges, srf.SEdge{A: e.A, B: e.B})
			}
		}
		w := 0.0
		for _, e := range el.Edges {
			w = max(w, math.Abs(e.A.X), math.Abs(e.B.X))
		}
		return w > r-0.05 && w < r+0.05
	}
	if !across(-30, b.Chamfer) {
		t.Error("no line across the chamfered end, as wide as the chamfer")
	}
	if !across(-30+reach, b.Crest) {
		t.Error("no line across where the chamfer meets the crest")
	}
	thin := top[core.StyleDimSolid]
	for _, x := range []float64{-b.Root, b.Root} {
		if !covers(thin, x, -30+from, -12) {
			t.Errorf("the root at x = %g doesn't run from the cone at %.3f to the thread's end", x, -30+from)
		}
	}
	for _, e := range thin.Edges {
		if min(e.A.Y, e.B.Y) < -30+from-1e-3 {
			t.Errorf("the root runs into the chamfer, from %v to %v", e.A, e.B)
		}
	}

	front := viewOf(t, ss, "front", &d)
	for st, el := range front {
		for _, e := range el.Edges {
			if math.Abs(math.Hypot(e.A.X, e.A.Y)-b.Chamfer) < 0.03 && math.Abs(math.Hypot(e.B.X, e.B.Y)-b.Chamfer) < 0.03 {
				t.Errorf("front: an edge in style %d at the chamfer's radius, from %v to %v", st, e.A, e.B)
				break
			}
		}
	}
}
