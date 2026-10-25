package sketch

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// Each kind of contour error says what and where.
func TestPolyProblem(t *testing.T) {
	t.Parallel()
	var g Group
	if _, _, ok := g.PolyProblem(); ok {
		t.Errorf("good contours have a problem")
	}
	g.PolyError.ErrorPoint = geom.Vector{X: 1.23456, Y: -0.0001, Z: 2}
	g.PolyError.NotClosed.A = geom.Vector{X: 3}
	for how, want := range map[core.PolyError]string{
		core.PolyNotClosed:        "is not closed: a contour has an open end at (3, 0, 0) (or its curves are not all of one style)",
		core.PolyNotCoplanar:      "is not all in one plane: (1.235, 0, 2) is off it",
		core.PolySelfIntersecting: "crosses itself at (1.235, 0, 2)",
		core.PolyZeroLenEdge:      "has a curve of zero length at (1.235, 0, 2)",
	} {
		g.PolyError.How = int(how)
		msg, at, ok := g.PolyProblem()
		if !ok || msg != want {
			t.Errorf("%d: %q, want %q", how, msg, want)
		}
		if how == core.PolyNotClosed && at != g.PolyError.NotClosed.A ||
			how != core.PolyNotClosed && at != g.PolyError.ErrorPoint {
			t.Errorf("%d: at %v", how, at)
		}
	}
}

// Which groups make a solid from a sketch's contours.
func TestTakesProfile(t *testing.T) {
	t.Parallel()
	h := core.HGroup{V: 7}
	for typ, want := range map[GroupType]bool{
		GroupExtrude: true, GroupLathe: true, GroupRevolve: true, GroupHelix: true, GroupSweep: true,
		GroupTranslate: false, GroupDrawingWorkplane: false,
	} {
		g := Group{Type: typ, OpA: h}
		if g.takesProfile(h) != want {
			t.Errorf("%v takes the profile: %t", typ, !want)
		}
	}
	loft := Group{Type: GroupLoft, OpA: core.HGroup{V: 1}, OpB: core.HGroup{V: 2}, LoftThrough: []core.HGroup{h}}
	if !loft.takesProfile(h) || loft.takesProfile(core.HGroup{V: 9}) {
		t.Errorf("a loft through the sketch")
	}
}
