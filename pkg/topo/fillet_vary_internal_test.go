package topo

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// quarterTopEdge is the convex edge where a cylinder of radius 5 about
// z meets its flat top at z 10, a quarter turn of it, rounded at r: the
// arc from (5, 0, 10) to (0, 5, 10).
func quarterTopEdge(r float64) filletEdge {
	h := math.Sqrt(0.5)
	curve := arc(geom.Vector{X: 5, Z: 10}, geom.Vector{X: 5, Y: 5, Z: 10}, geom.Vector{Y: 5, Z: 10}, h)
	beta := math.Pi / 2
	fe := filletEdge{
		circ: true, curve: curve, axis: geom.Vector{Z: 1}, centre: geom.Vector{Z: 10}, radius: 5, flatK: 0, r: r,
		gen: [2][2]float64{{-1, 0}, {0, -1}},
	}
	fe.ss, fe.w = [2]float64{r / math.Tan(beta/2), r / math.Tan(beta/2)}, math.Sin(beta/2)
	return fe
}

// The rounded face of a circular edge whose radius varies is, where it
// doesn't, the torus the edge's own surface is; its sides are its
// tangent curves; and its cross section at each end is the one of that
// end's radius.
func TestVaryingSurface(t *testing.T) {
	t.Parallel()
	fe := quarterTopEdge(1)
	fe.r1 = 1
	torus := fe.surface(geom.Vector{}, geom.Vector{}, false)
	same := fe.varyingSurface(false)
	for i := range 5 {
		for j := range 5 {
			u, v := float64(i)/4, float64(j)/4
			if d := torus.PointAt(u, v).Minus(same.PointAt(u, v)).Magnitude(); d > 1e-9 {
				t.Fatalf("at (%g, %g) the varying surface is %g off the torus", u, v, d)
			}
		}
	}

	fe.r1 = 2
	if !fe.varies() {
		t.Fatal("a radius of 1 going to 2 doesn't vary")
	}
	s := fe.varyingSurface(false)
	for k, u := range []float64{0, 1} {
		side := fe.varyingTangentCurve(k, s.PointAt(u, 0), s.PointAt(u, 1))
		for j := range 5 {
			v := float64(j) / 4
			if d := side.PointAt(v).Minus(s.PointAt(u, v)).Magnitude(); d > 1e-9 {
				t.Errorf("side %d at %g is %g off the surface", k, v, d)
			}
		}
	}
	// The end of radius 2: on the top 2 in from the cylinder, and on the
	// cylinder 2 down from the top.
	end := fe.curve.Finish()
	for k, want := range []geom.Vector{end.Minus(geom.Vector{Y: 2}), end.Minus(geom.Vector{Z: 2})} {
		if got := s.PointAt(float64(k), 1); !got.Equals(want) {
			t.Errorf("the end's point on face %d is %v, want %v", k, got, want)
		}
	}
	// Every section is tangent to both faces: the top's normal at the
	// side on it, the cylinder's at the side on it.
	for j := range 9 {
		v := float64(j) / 8
		n0 := s.NormalAt(0, v).WithMagnitude(1)
		if math.Abs(math.Abs(n0.Z)-1) > 1e-9 {
			t.Errorf("at %g the face isn't tangent to the top: normal %v", v, n0)
		}
		p, n1 := s.PointAt(1, v), s.NormalAt(1, v).WithMagnitude(1)
		radial := geom.Vector{X: p.X, Y: p.Y}.WithMagnitude(1)
		if math.Abs(math.Abs(n1.Dot(radial))-1) > 1e-9 {
			t.Errorf("at %g the face isn't tangent to the cylinder: normal %v", v, n1)
		}
	}
	if err := fe.fitsBoth(); err != nil {
		t.Error(err)
	}
	fe.r1 = 6
	if err := fe.fitsBoth(); err == nil {
		t.Error("a radius of 6 at the end fits a cylinder of 5")
	}
}
