package srf

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// A plane patch is carried on over a curve that runs past its sides,
// to exactly the box of the curve in its parameters, and left as it is
// for one that stays on it; a curved patch is never carried on.
func TestOverCurves(t *testing.T) {
	t.Parallel()
	v := func(x, y float64) geom.Vector { return geom.Vector{X: x, Y: y} }
	line := func(a, b geom.Vector) SBezier {
		return SBezier{Degree: 1, Ctrl: [MaxDegree + 1]geom.Vector{a, b}, Weight: [MaxDegree + 1]float64{1, 1}}
	}
	s := SSurface{DegM: 1, DegN: 1}
	s.Ctrl[0][0], s.Ctrl[1][0], s.Ctrl[0][1], s.Ctrl[1][1] = v(0, 0), v(10, 0), v(0, 5), v(10, 5)
	for i := range 2 {
		for j := range 2 {
			s.Weight[i][j] = 1
		}
	}
	if _, ok := s.OverCurves([]SBezier{line(v(2, 1), v(8, 4))}); ok {
		t.Error("carried on over a curve on the patch")
	}
	over, ok := s.OverCurves([]SBezier{line(v(2, 1), v(14, 7)), line(v(-3, 2), v(2, 1))})
	if !ok {
		t.Fatal("not carried on over a curve past the patch")
	}
	for _, c := range []struct {
		u, v float64
		want geom.Vector
	}{{0, 0, v(-3, 0)}, {1, 0, v(14, 0)}, {0, 1, v(-3, 7)}, {1, 1, v(14, 7)}} {
		if got := over.PointAt(c.u, c.v); !got.Equals(c.want) {
			t.Errorf("at (%g, %g): %v, want %v", c.u, c.v, got, c.want)
		}
	}
	curved := s
	curved.DegM = 2
	curved.Ctrl[2][0], curved.Ctrl[2][1] = v(20, 0), v(20, 5)
	curved.Weight[2][0], curved.Weight[2][1] = 1, 1
	if _, ok := curved.OverCurves([]SBezier{line(v(2, 1), v(30, 7))}); ok {
		t.Error("carried a curved patch on")
	}
}
