package srf

import (
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
)

// touchSinks are the points of the operands' Touch that lie on both s
// and b: where the two surfaces are known to touch tangent, and the
// curves they meet in cross (roadmap K3). There the normals are
// parallel, the marcher's direction, their cross product, shrinks to
// nothing and turns over, and a march that reaches the point goes back
// the way it came, or its step, grown where the surfaces lie within the
// tolerance of each other, jumps along them. A point the feature that
// built the operands knows (a sweep's corner beside a cubic, where its
// two runs' tubes share the profile's points on the turn's axis) is
// taken as an end every branch marched towards it reaches.
func (s *SSurface) touchSinks(b *SSurface, agnstA, agnstB *SShell) []geom.Vector {
	var out []geom.Vector
	tol := s.Tol.ChordTolMm()
	for _, sh := range []*SShell{agnstA, agnstB} {
		if sh == nil {
			continue
		}
		for _, p := range sh.Touch {
			if offPatch(s, p, tol) || offPatch(b, p, tol) {
				continue
			}
			if !slices.ContainsFunc(out, func(q geom.Vector) bool { return q.Minus(p).Magnitude() < tol }) {
				out = append(out, p)
			}
		}
	}
	return out
}

// foundSinks are the points of found, the touching points the Boolean
// found itself (SShell.touchFound), that lie on both s and b and are
// not among sinks already.
func (s *SSurface) foundSinks(b *SSurface, found, sinks []geom.Vector) []geom.Vector {
	var out []geom.Vector
	for _, p := range found {
		if offPatch(s, p, geom.LengthEps) || offPatch(b, p, geom.LengthEps) {
			continue
		}
		if !slices.ContainsFunc(sinks, func(q geom.Vector) bool { return q.Equals(p) }) {
			out = append(out, p)
		}
	}
	return out
}

// dropGrazingSeeds removes the seeds within tol of a point in found, or
// whose edge's chord (chords, by the seed) runs past one, within tol or
// an eighth of its length of it, and are no further from it than the
// chord is long: the touching curve's chord crosses the surface a
// little way either side of the point, as far as the chord sags, which
// past MaxSegments is more than the tolerance (0.0002 either side of a
// ledge's foot at 9e-5).
func dropGrazingSeeds(seeds []SPoint, found []geom.Vector, chords map[geom.Vector][2]geom.Vector, tol float64) []SPoint {
	return slices.DeleteFunc(seeds, func(sp SPoint) bool {
		return slices.ContainsFunc(found, func(q geom.Vector) bool {
			d := sp.P.Minus(q).Magnitude()
			if d < tol {
				return true
			}
			ch, ok := chords[sp.P]
			if !ok {
				return false
			}
			l := ch[1].Minus(ch[0]).Magnitude()
			return d <= l && distanceToSegment(q, ch[0], ch[1]) <= max(tol, l/8)
		})
	})
}

// dropSeedsAt removes the seeds within reach of a sink: an edge through
// the touching point gives a seed there, and a march from it has no
// direction to go.
func dropSeedsAt(seeds []SPoint, sinks []geom.Vector, reach float64) []SPoint {
	return slices.DeleteFunc(seeds, func(sp SPoint) bool {
		return slices.ContainsFunc(sinks, func(q geom.Vector) bool { return sp.P.Minus(q).Magnitude() < reach })
	})
}

// atSink reports the sink the march from start to next comes within
// reach of, if any.
func atSink(sinks []geom.Vector, start, next geom.Vector, reach float64) (geom.Vector, bool) {
	for _, q := range sinks {
		if q.OnLineSegment(start, next, reach) || next.Minus(q).Magnitude() < reach {
			return q, true
		}
	}
	return geom.Vector{}, false
}

// unionTouch is the touching points of a and b, each once.
func unionTouch(a, b []geom.Vector) []geom.Vector {
	out := slices.Clone(a)
	for _, p := range b {
		if !slices.ContainsFunc(out, func(q geom.Vector) bool { return q.Equals(p) }) {
			out = append(out, p)
		}
	}
	return out
}

// knownTouches are the touching points the operands' features gave
// (SShell.Touch), each once.
func knownTouches(a, b *SShell) []geom.Vector {
	var out []geom.Vector
	for _, sh := range []*SShell{a, b} {
		if sh != nil {
			out = unionTouch(out, sh.Touch)
		}
	}
	return out
}

// snapToKnownTouches splits the segment from a to b at each known
// touching point it passes within reach of, in place of the splits
// lying within reach of that point. An edge of one operand running
// through the point (a fitted tube's seam, which passes where the
// profile crosses the turn's axis) meets the other operand's surface
// there at a grazing angle, both being tangent: its crossing, refined,
// lands a little along it, a hair from where the marched curves end at
// the sink (2e-4 at a sweep's tube coming back across the line before
// its corner), and the two vertices leave the mesh split there.
func snapToKnownTouches(il []SInter, known []geom.Vector, a, b geom.Vector, reach float64) []SInter {
	for _, q := range known {
		if !q.OnLineSegment(a, b, reach) || q.EqualsTol(a, splitEndTol) || q.EqualsTol(b, splitEndTol) {
			continue
		}
		il = slices.DeleteFunc(il, func(o SInter) bool { return o.P.Minus(q).Magnitude() < reach })
		il = append(il, SInter{P: q})
	}
	return il
}
