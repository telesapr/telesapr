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
