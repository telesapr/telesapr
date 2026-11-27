package sketch

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// polygonSet is the loop set of the polygon through ps, of normal n.
func polygonSet(n geom.Vector, ps ...geom.Vector) *srf.SBezierLoopSet {
	s := &srf.SBezierLoopSet{Normal: n, Point: ps[0]}
	for i := range ps {
		s.Outer.Curves = append(s.Outer.Curves, srf.SBezierFrom2(ps[i], ps[(i+1)%len(ps)]))
	}
	return s
}

// The test by which a lathe, revolve, helix or sweep's arc is refused:
// a profile reaching both sides of the line on its plane where the
// motion runs along it is, one that touches the line, lies on one side
// of it, or lies in a plane through the axis is not, nor is one square
// to the axis.
func TestTurnStraddle(t *testing.T) {
	t.Parallel()
	y := geom.Vector{Y: 1}
	beside := func(z0, z1 float64) *srf.SBezierLoopSet {
		return polygonSet(geom.Vector{X: 1},
			geom.Vector{X: 5, Z: z0}, geom.Vector{X: 5, Z: z1}, geom.Vector{X: 5, Y: 6, Z: z1}, geom.Vector{X: 5, Y: 6, Z: z0})
	}
	through := func(x0, x1 float64) *srf.SBezierLoopSet {
		return polygonSet(geom.Vector{Z: 1},
			geom.Vector{X: x0}, geom.Vector{X: x1}, geom.Vector{X: x1, Y: 6}, geom.Vector{X: x0, Y: 6})
	}
	// On the plane x + y = 5, which the helix's advance along y crosses:
	// by 40 a turn it outruns the turn's motion back through the plane.
	leaning := polygonSet(geom.Vector{X: 1, Y: 1}.WithMagnitude(1),
		geom.Vector{X: 5, Z: -2}, geom.Vector{X: 5, Z: 6}, geom.Vector{X: 2, Y: 3, Z: 6}, geom.Vector{X: 2, Y: 3, Z: -2})
	square := polygonSet(y, geom.Vector{X: -2}, geom.Vector{X: 4}, geom.Vector{X: 4, Z: 3})
	// A quarter of a circle of radius 3 about (5, 3, 3), through its
	// lowest point (5, 3, 0), closed by its chord: rational, its lowest
	// point inside it, its ends at z = 3 - 3/√2.
	h := math.Sqrt(0.5)
	arc := srf.SBezier{Degree: 2}
	arc.Ctrl[0], arc.Ctrl[1], arc.Ctrl[2] = geom.Vector{X: 5, Y: 3 - 3*h, Z: 3 - 3*h}, geom.Vector{X: 5, Y: 3, Z: 3 - 6*h}, geom.Vector{X: 5, Y: 3 + 3*h, Z: 3 - 3*h}
	arc.Weight[0], arc.Weight[1], arc.Weight[2] = 1, h, 1
	moon := func(dz float64) *srf.SBezierLoopSet {
		a := arc
		for i := range 3 {
			a.Ctrl[i].Z -= dz
		}
		s := &srf.SBezierLoopSet{Normal: geom.Vector{X: 1}, Point: a.Ctrl[0]}
		s.Outer.Curves = []srf.SBezier{a, srf.SBezierFrom2(a.Ctrl[2], a.Ctrl[0])}
		return s
	}
	for name, c := range map[string]struct {
		set     *srf.SBezierLoopSet
		da, dx  float64
		refused bool
	}{
		"across":             {set: beside(-2, 6), da: 2 * math.Pi, refused: true},
		"across helix":       {set: beside(-2, 6), da: 3, dx: 20, refused: true},
		"one side":           {set: beside(0.5, 6), da: 2 * math.Pi},
		"touching":           {set: beside(0, 6), da: 2 * math.Pi},
		"touching below":     {set: beside(-6, 0), da: -math.Pi / 2},
		"helix pushed aside": {set: leaning, da: 1, dx: 40},
		"helix leaning":      {set: leaning, da: 1, dx: 1, refused: true},
		"through the axis":   {set: through(-2, 4), da: 2 * math.Pi},
		"thread":             {set: through(4, 5), da: 2 * math.Pi, dx: 1.5},
		"square":             {set: square, da: 1, dx: 3},
		"arc touching":       {set: moon(0), da: 1},
		"arc over":           {set: moon(1e-4), da: 1, refused: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			at, refused := turnStraddle(c.set, geom.Vector{}, y, c.da, c.dx)
			if refused != c.refused {
				t.Fatalf("refused %v, want %v", refused, c.refused)
			}
			if refused && c.dx == 0 && (math.Abs(at.X-5) > 1e-9 || math.Abs(at.Z) > 1e-9) {
				t.Errorf("marked at %v, off the line x = 5, z = 0", at)
			}
		})
	}
}
