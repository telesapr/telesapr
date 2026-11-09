package sketch

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// The freedom a span of twists gives, for the kinds of motion a part
// can have, its origin at o: what turns and slides, and what stays.
func TestAnalyseTwists(t *testing.T) {
	t.Parallel()
	o := geom.Vector{X: 10, Y: 20, Z: 30}
	x, y, z := geom.Vector{X: 1}, geom.Vector{Y: 1}, geom.Vector{Z: 1}
	// A turn w about the axis through p, as the speed of o.
	about := func(w, p geom.Vector) twist { return twist{w: w, v: w.Cross(o.Minus(p))} }
	slide := func(v geom.Vector) twist { return twist{v: v} }
	p := geom.Vector{X: 1, Y: 2, Z: 3}
	for _, tc := range []struct {
		name          string
		ts            []twist
		turns, slides int
		inv           Invariant
		point, dir    geom.Vector
		lead          float64
		text          string
	}{
		{"none", nil, 0, 0, InvariantNone, geom.Vector{}, geom.Vector{}, 0, "fixed"},
		{"slide", []twist{slide(y.ScaledBy(-3))}, 0, 1, InvariantDirection, geom.Vector{}, y, 0, "slides along (0, 1, 0)"},
		{"slides", []twist{slide(x), slide(x.Plus(y))}, 0, 2, InvariantPlane, o, z, 0,
			"slides in the plane through (10, 20, 30) normal to (0, 0, 1)"},
		{"hinge", []twist{about(z.ScaledBy(2), p)}, 1, 0, InvariantAxis, geom.Vector{X: 1, Y: 2, Z: 30}, z, 0,
			"turns about the axis through (1, 2, 30) along (0, 0, 1)"},
		{"screw", []twist{{w: z, v: z.Cross(o.Minus(p)).Plus(z.ScaledBy(2 / (2 * math.Pi)))}}, 1, 0, InvariantAxis,
			geom.Vector{X: 1, Y: 2, Z: 30}, z, 2, "screws along the axis through (1, 2, 30) along (0, 0, 1), 2 a turn"},
		{"cylinder", []twist{about(x, p), slide(x.ScaledBy(5))}, 1, 1, InvariantAxis, geom.Vector{X: 10, Y: 2, Z: 3}, x, 0,
			"turns about and slides along the axis through (10, 2, 3) along (1, 0, 0)"},
		{"planar", []twist{about(z, p), slide(x), slide(y)}, 1, 2, InvariantPlane, o, z, 0,
			"moves in the plane through (10, 20, 30) normal to (0, 0, 1)"},
		{"ball", []twist{about(x, p), about(y, p), about(z, p)}, 3, 0, InvariantPoint, p, geom.Vector{}, 0,
			"turns about the point (1, 2, 3)"},
		{"turn and slide across", []twist{about(z, p), slide(x)}, 1, 1, InvariantNone, geom.Vector{}, geom.Vector{}, 0,
			"turns 1 way and slides 1 way"},
		{"free", []twist{about(x, p), about(y, p), about(z, p), slide(x), slide(y), slide(z)}, 3, 3, InvariantNone,
			geom.Vector{}, geom.Vector{}, 0, "free"},
	} {
		f := analyseTwists(core.HGroup{V: 4}, o, tc.ts)
		if f.Turns != tc.turns || f.Slides != tc.slides || f.Invariant != tc.inv ||
			f.Point.Minus(tc.point).Magnitude() > 1e-9 || f.Dir.Minus(tc.dir).Magnitude() > 1e-9 ||
			math.Abs(f.Lead-tc.lead) > 1e-9 || f.String() != tc.text {
			t.Errorf("%s: %+v %q", tc.name, f, f.String())
		}
	}
}

// A turn read off a change of a quaternion is the turn the change
// makes: a quaternion turned by a small angle about an axis, less the
// one it was, over the angle, is that axis.
func TestTurnOf(t *testing.T) {
	t.Parallel()
	q := geom.QuaternionFromAxisAngle(geom.Vector{X: 1, Y: 2, Z: 2}.WithMagnitude(1), 0.7)
	axis := geom.Vector{X: -2, Y: 1, Z: 0.5}.WithMagnitude(1)
	const h = 1e-6
	turned := geom.QuaternionFromAxisAngle(axis, h).Times(q)
	dq := geom.Quaternion{W: (turned.W - q.W) / h, X: (turned.X - q.X) / h, Y: (turned.Y - q.Y) / h, Z: (turned.Z - q.Z) / h}
	if w := turnOf(q, dq); w.Minus(axis).Magnitude() > 1e-5 {
		t.Errorf("turn %v, want %v", w, axis)
	}
}
