package srf

import (
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// joinOrphans joins two curves that run along one exact curve between
// the same ends, each left with one kept surface where the other was
// dropped, into one curve between the two kept surfaces, along the
// points of both (roadmap K3). A shell's inside that leaves the solid's
// edge at an inside corner of a revolve (pkg/topo, shellrevolve_step.go)
// has its own edge there: the solid's face keeps its edge with the face
// taken away, the inside's wall end its edge with the part of the inside
// that lay outside, both exact circles but with their points chosen
// apart (a cove's edge takes one more near its seam). Left so, the two
// faces' meshes do not meet point for point and the shell is open along
// the corner; joined, they meet on one curve. Two such curves along the
// same points are left apart: their meshes meet as they are.
func (s *SShell) joinOrphans(dropped map[core.HSSurface]bool) {
	type orphan struct {
		i    int
		kept core.HSSurface
	}
	used := map[core.HSCurve]bool{}
	for _, sf := range s.Surface {
		if dropped[sf.H] {
			continue
		}
		for _, t := range sf.Trim {
			used[t.Curve] = true
		}
	}
	var orphans []orphan
	for i := range s.Curve {
		c := &s.Curve[i]
		if !c.IsExact || len(c.Points) < 2 || !used[c.H] || dropped[c.SurfA] == dropped[c.SurfB] {
			continue
		}
		kept := c.SurfA
		if dropped[kept] {
			kept = c.SurfB
		}
		orphans = append(orphans, orphan{i, kept})
	}
	joined := map[int]bool{}
	for x, oc := range orphans {
		if joined[oc.i] {
			continue
		}
		for _, od := range orphans[x+1:] {
			if joined[od.i] || od.kept == oc.kept {
				continue
			}
			c, d := &s.Curve[oc.i], &s.Curve[od.i]
			same, ok := alongOneCurve(c, d)
			if !ok {
				continue
			}
			// Along the same points the two faces' meshes already meet;
			// joined only where the points differ and the mesh would open.
			union := unionPoints(c.Points, d.Points)
			if len(union) == len(c.Points) && len(union) == len(d.Points) {
				continue
			}
			c.Points = union
			if dropped[c.SurfA] {
				c.SurfA = od.kept
			} else {
				c.SurfB = od.kept
			}
			for i := range s.Surface {
				sf := &s.Surface[i]
				if sf.H != od.kept {
					continue
				}
				for k := range sf.Trim {
					t := &sf.Trim[k]
					if t.Curve != d.H {
						continue
					}
					t.Curve = c.H
					if !same {
						t.Backwards = !t.Backwards
					}
				}
			}
			joined[oc.i], joined[od.i] = true, true
			break
		}
	}
}

// alongOneCurve reports whether c and d run between the same two ends
// along one exact curve, each point of either on the other's exact
// form, and same when they run the same way.
func alongOneCurve(c, d *SCurve) (same, ok bool) {
	c0, c1 := c.Points[0].P, c.Points[len(c.Points)-1].P
	d0, d1 := d.Points[0].P, d.Points[len(d.Points)-1].P
	switch {
	case c0.Equals(d0) && c1.Equals(d1):
		same = true
	case c0.Equals(d1) && c1.Equals(d0):
	default:
		return false, false
	}
	if c0.Equals(c1) {
		return false, false
	}
	on := func(e *SBezier, pts []SCurvePt) bool {
		for _, p := range pts {
			t, conv := e.ClosestPointTo(p.P, false)
			if !conv || e.PointAt(t).Minus(p.P).Magnitude() > geom.LengthEps {
				return false
			}
		}
		return true
	}
	return same, on(&c.Exact, d.Points) && on(&d.Exact, c.Points)
}

// unionPoints is the points of a, with those of b that a lacks put in
// along a's run: each on the piece of a's polyline nearest it, in order
// from that piece's start.
func unionPoints(a, b []SCurvePt) []SCurvePt {
	type extra struct {
		piece int
		along float64
		pt    SCurvePt
	}
	var extras []extra
	for _, q := range b {
		if slices.ContainsFunc(a, func(p SCurvePt) bool { return p.P.Equals(q.P) }) {
			continue
		}
		best, bestD, along := 0, -1.0, 0.0
		for j := range len(a) - 1 {
			p0, p1 := a[j].P, a[j+1].P
			seg := p1.Minus(p0)
			s := q.P.Minus(p0).Dot(seg) / seg.MagSquared()
			s = min(max(s, 0), 1)
			if dist := p0.Plus(seg.ScaledBy(s)).Minus(q.P).Magnitude(); bestD < 0 || dist < bestD {
				best, bestD, along = j, dist, s
			}
		}
		extras = append(extras, extra{best, along, q})
	}
	if len(extras) == 0 {
		return a
	}
	slices.SortStableFunc(extras, func(x, y extra) int {
		if x.piece != y.piece {
			return x.piece - y.piece
		}
		switch {
		case x.along < y.along:
			return -1
		case x.along > y.along:
			return 1
		}
		return 0
	})
	out := make([]SCurvePt, 0, len(a)+len(extras))
	k := 0
	for j, p := range a {
		out = append(out, p)
		for k < len(extras) && extras[k].piece == j {
			out = append(out, extras[k].pt)
			k++
		}
	}
	return out
}
