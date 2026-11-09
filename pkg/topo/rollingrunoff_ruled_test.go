package topo

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// The field ruledControls fits along a circle's arc, from the unit
// tangents of a cone about the circle's axis at the arc's ends and
// middle, is that cone's tangent all along: the ruling's direction,
// radial out and down at a fixed angle, is a fixed blend of the radius
// to the arc's point and the axis, both of which a rational quadratic
// with the arc's weights holds. So the surface the tangents across a
// torus's circular edge rule is the cone that meets the torus there.
func TestRuledControlsFollowCone(t *testing.T) {
	t.Parallel()
	const r, half = 43.0, 10 * math.Pi / 180
	// The arc of radius r about the z axis from -half to half, rational.
	var e srf.SBezier
	e.Degree = 2
	e.Ctrl[0] = geom.Vector{X: r * math.Cos(-half), Y: r * math.Sin(-half)}
	e.Ctrl[1] = geom.Vector{X: r / math.Cos(half)}
	e.Ctrl[2] = geom.Vector{X: r * math.Cos(half), Y: r * math.Sin(half)}
	e.Weight[0], e.Weight[1], e.Weight[2] = 1, math.Cos(half), 1
	down := math.Atan(2.4)
	cone := func(p geom.Vector) geom.Vector {
		radial := geom.Vector{X: p.X, Y: p.Y}.WithMagnitude(1)
		return radial.ScaledBy(math.Cos(down)).Plus(geom.Vector{Z: -math.Sin(down)})
	}
	tans := make([]geom.Vector, 3)
	for k := range 3 {
		tans[k] = cone(e.PointAt(float64(k) / 2))
	}
	ctrl, ok := ruledControls(&e, tans)
	if !ok {
		t.Fatal("no controls")
	}
	for i := range 21 {
		x := float64(i) / 20
		var num geom.Vector
		den := 0.0
		for j := range 3 {
			b := e.Weight[j] * bernstein(2, j, x)
			num, den = num.Plus(ctrl[j].ScaledBy(b)), den+b
		}
		got, want := num.ScaledBy(1/den), cone(e.PointAt(x))
		if d := got.Minus(want).Magnitude(); d > 1e-9 {
			t.Fatalf("at %g the field is %v, the cone's tangent %v (%g off)", x, got, want, d)
		}
	}
}
