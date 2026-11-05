package sketch

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// At a corner beside a cubic, the two runs' profiles at the corner are
// one profile turned about the corner's axis, so they share the points
// where the profile's outline crosses that axis, and there the two
// runs' skins have the same tangent plane: a round profile's tubes
// touch tangent at those two points, and the curves they meet in cross
// there. The marcher can't follow a curve through such a point (its
// direction, the normals' cross product, vanishes), so the points are
// handed to the Booleans that join the runs (srf.SShell.Touch), which
// end every branch there.

// cornerTouches are the points where the outline of set, placed at the
// corner of turn k by at, crosses the turn's axis through the corner.
func (sr *sweepRuns) cornerTouches(set *srf.SBezierLoopSet, at srf.Rigid, k int, tol srf.Tolerance) []geom.Vector {
	turn := sr.turns[k]
	c := turn.Center
	// w is square to the axis in the corner's section: the outline
	// crosses the axis where its component along w from the corner is 0.
	w := sr.into[k].Cross(turn.Axis).WithMagnitude(1)
	g := func(p geom.Vector) float64 { return w.Dot(p.Minus(c)) }
	moved := srf.TransformedLoopSet(set, at)
	loops := append([]srf.SBezierLoop{moved.Outer}, moved.Holes...)
	var out []geom.Vector
	add := func(p geom.Vector) {
		if !slices.ContainsFunc(out, func(q geom.Vector) bool { return q.Minus(p).Magnitude() < tol.ChordTolMm() }) {
			out = append(out, p)
		}
	}
	for _, l := range loops {
		for i := range l.Curves {
			sb := &l.Curves[i]
			const steps = 32
			t0, g0 := 0.0, g(sb.PointAt(0))
			for j := 1; j <= steps; j++ {
				t1 := float64(j) / steps
				g1 := g(sb.PointAt(t1))
				switch {
				case math.Abs(g0) < geom.LengthEps:
					add(sb.PointAt(t0))
				case g0*g1 < 0:
					lo, hi, glo := t0, t1, g0
					for range 60 {
						m := (lo + hi) / 2
						if gm := g(sb.PointAt(m)); gm*glo <= 0 {
							hi = m
						} else {
							lo, glo = m, gm
						}
					}
					add(sb.PointAt((lo + hi) / 2))
				}
				t0, g0 = t1, g1
			}
			if math.Abs(g0) < geom.LengthEps {
				add(sb.PointAt(1))
			}
		}
	}
	return out
}
