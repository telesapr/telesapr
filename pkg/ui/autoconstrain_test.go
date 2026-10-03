package ui

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// What a line being drawn is offered (roadmap 16.11): tangent to an arc
// it leaves along the arc's tangent, perpendicular to a line it leaves
// square, parallel to a line elsewhere, as long as one, and nothing
// when it is none of them; horizontal and vertical first, as before.
func TestSuggestLineConstraint(t *testing.T) {
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	// An arc about the origin from 45 degrees to (-10, 0), and a line.
	r := 10 / math.Sqrt2
	doc.Arc(api.V(0, 0, 0), api.V(r, r, 0), api.V(-10, 0, 0))
	base := doc.Line(api.V(30, 0, 0), api.V(50, 3, 0))
	doc.Regen()
	suggest := func(a, b api.Vec) lineSuggestion {
		l := doc.Line(a, b)
		doc.Regen()
		s, _ := ss.GW.SuggestLineConstraint(l.H.Request())
		l.Delete()
		doc.Regen()
		return s
	}
	baseDir := api.V(20, 3, 0).WithMagnitude(1)
	across := api.V(-baseDir.Y, baseDir.X, 0)
	for name, c := range map[string]struct {
		a, b api.Vec
		want core.ConstraintType
		with core.HEntity
	}{
		"tangent at the arc's start": {api.V(r, r, 0), api.V(r+6, r-6.05, 0), core.ConstraintArcLineTangent, core.HEntity{}},
		"square to the line":         {api.V(50, 3, 0), api.V(50, 3, 0).Plus(across.ScaledBy(7)), core.ConstraintPerpendicular, base.H},
		"parallel to the line":       {api.V(30, 20, 0), api.V(30, 20, 0).Plus(baseDir.ScaledBy(5)), core.ConstraintParallel, base.H},
		"as long as the line":        {api.V(30, 40, 0), api.V(30, 40, 0).Plus(api.V(1, 1, 0).WithMagnitude(math.Hypot(20, 3))), core.ConstraintEqualLengthLines, base.H},
		"horizontal first":           {api.V(30, 60, 0), api.V(50, 60.1, 0), core.ConstraintHorizontal, core.HEntity{}},
		"nothing":                    {api.V(30, 80, 0), api.V(33, 87, 0), 0, core.HEntity{}},
	} {
		s := suggest(c.a, c.b)
		if s.Type != c.want || (c.with.V != 0 && s.B != c.with) {
			t.Errorf("%s: %+v, want type %d with %x", name, s, c.want, c.with.V)
		}
	}
}
