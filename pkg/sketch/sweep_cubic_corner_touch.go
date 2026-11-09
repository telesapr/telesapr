package sketch

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
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
	// w is square to the axis in the corner's section: the outline
	// crosses the axis where its component along w from the corner is 0.
	w := sr.into[k].Cross(turn.Axis).WithMagnitude(1)
	return outlineOnLine(set, at, turn.Center, w, tol)
}

// crossingTouches are the points where two runs' tubes touch tangent
// because the path comes back through itself: where a piece of one run
// crosses a piece of another, both runs' sections there contain the
// line through the crossing square to both tangents, and a point of the
// outline lying on that line in both sections is a point where the two
// skins share their tangent plane. A round profile's runs crossing so
// (a cubic bent back over the line before its corner) meet in curves
// crossing there, which, as at the corner, the marcher can't follow
// through; no feature but the sweep knows the points.
func (sr *sweepRuns) crossingTouches(set *srf.SBezierLoopSet, starts []srf.Rigid, tol srf.Tolerance) []geom.Vector {
	var out []geom.Vector
	for a := range sr.runs {
		for b := a + 1; b < len(sr.runs); b++ {
			pas, pbs := sr.runs[a].Pieces(), sr.runs[b].Pieces()
			for i := range pas {
				for j := range pbs {
					for _, x := range pieceCrossings(&pas[i], &pbs[j], tol) {
						out = append(out, sr.sharedOnNormal(set, starts, a, b, float64(i)+x[0], float64(j)+x[1], tol)...)
					}
				}
			}
		}
	}
	return out
}

// pieceCrossings are the parameters on pa and pb where they cross, but
// for a point both share as an end (the corner between two runs, whose
// touches cornerTouches gives).
func pieceCrossings(pa, pb *srf.SBezier, tol srf.Tolerance) [][2]float64 {
	var spl srf.SPointList
	pa.AllIntersectionsWith(pb, &spl, tol)
	var out [][2]float64
	for _, sp := range spl.Points {
		p := sp.P
		end := func(b *srf.SBezier) bool { return p.Equals(b.Start()) || p.Equals(b.Finish()) }
		if end(pa) && end(pb) {
			continue
		}
		ta, _ := pa.ClosestPointTo(p, false)
		tb, _ := pb.ClosestPointTo(p, false)
		out = append(out, [2]float64{ta, tb})
	}
	return out
}

// sharedOnNormal are the points of the outline lying on the line square
// to both runs' tangents through where run a at sa and run b at sb
// cross, in both runs' sections there.
func (sr *sweepRuns) sharedOnNormal(set *srf.SBezierLoopSet, starts []srf.Rigid, a, b int, sa, sb float64, tol srf.Tolerance) []geom.Vector {
	ra, rb := starts[a].Then(sr.runs[a].Rigid(sa)), starts[b].Then(sr.runs[b].Rigid(sb))
	pa, ta := pathAt(sr.runs[a], sa)
	_, tb := pathAt(sr.runs[b], sb)
	n := ta.Cross(tb)
	if n.Magnitude() < 1e-3 {
		// Crossing tangent: the tubes overlap along the path, no
		// point.
		return nil
	}
	n = n.WithMagnitude(1)
	onA := outlineOnLine(set, ra, pa, ta.Cross(n).WithMagnitude(1), tol)
	onB := outlineOnLine(set, rb, pa, tb.Cross(n).WithMagnitude(1), tol)
	var out []geom.Vector
	for _, p := range onA {
		if slices.ContainsFunc(onB, func(q geom.Vector) bool { return q.Minus(p).Magnitude() < tol.ChordTolMm() }) {
			out = append(out, p)
		}
	}
	return out
}

// pathAt is the path's point and unit tangent at s.
func pathAt(sp *topo.SweepPath, s float64) (geom.Vector, geom.Vector) {
	ps := sp.Pieces()
	i := max(0, min(int(math.Floor(s)), len(ps)-1))
	t := s - float64(i)
	return ps[i].PointAt(t), ps[i].TangentAt(t).WithMagnitude(1)
}

// outlineOnLine are the points where the outline of set, placed by at,
// crosses the line through c square to w, in the section's plane: where
// its component along w from c is 0.
func outlineOnLine(set *srf.SBezierLoopSet, at srf.Rigid, c, w geom.Vector, tol srf.Tolerance) []geom.Vector {
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
