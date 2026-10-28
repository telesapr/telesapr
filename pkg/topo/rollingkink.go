package topo

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A mitre of two radii: where chains of different radii meet across a
// crease, the smaller ball touches its wall at its own depth and the
// larger at its, so the curve where the two swept surfaces meet doesn't
// reach the crease. The smaller chain's end is a mitre as any, against
// the larger's swept surface, its foot on its own wall where that
// surface crosses it. The larger chain's end is kinked: each point of
// its ball's arcs is cut at the first it comes to, from inside, of the
// smaller's swept surface and the smaller's wall carried across the
// crease. From the face both share the mitre runs down to where the two
// cuts meet, on the smaller's wall, and from there on along the wall to
// the crease at the larger's depth. Every section of the larger chain
// has a corner at the arc parameter of that point (rollingBall.kinks):
// its patches are two rows, and the mitre's section is fitted in two
// spans, the first of which the smaller end copies; the smaller's wall
// is cut back along the second.

// kinkedEnd reports whether end k of the chain is the larger at a mitre
// of two radii.
func (rb *rollingBall) kinkedEnd(k int) bool {
	end := rb.ends[k]
	return !rb.closed && end != nil && end.mitre && end.mate != rb && end.mate.r < rb.r
}

// mateKinked reports whether end k's mate is the larger at a mitre of
// two radii, so that end k copies the first span of its kinked mitre.
func (rb *rollingBall) mateKinked(k int) bool {
	end := rb.ends[k]
	return !rb.closed && end != nil && end.mitre && end.mate != rb && end.mate.r > rb.r
}

// sharedSide is the side of end k whose face both chains share, at a
// mitre: the side with no ban.
func (rb *rollingBall) sharedSide(k int) int {
	if rb.ends[k].ban[0] < 0 {
		return 0
	}
	return 1
}

// wallHeight is how far p lies past the face end k ends at, the mate's
// wall, along that face's normal at its point nearest p, the normal
// turned to point on past the end.
func (rb *rollingBall) wallHeight(k int, p geom.Vector) float64 {
	end := rb.ends[k]
	s := rb.t.Surface(end.f)
	u, v, q := nearestOnWithin(s, p, 0.5)
	n := s.NormalAt(u, v)
	if n.Magnitude() == 0 {
		return math.NaN()
	}
	return end.wallSign * n.WithMagnitude(1).Dot(p.Minus(q))
}

// kinkRoot is the arc length near kinked end k where the point at
// parameter t of the ball's arcs is cut, and its section: the root of
// its height over the mate's swept surface on the shared side of the
// kink, over the mate's wall on the other.
func (rb *rollingBall) kinkRoot(k int, t float64) (float64, rbSection, error) {
	end := rb.ends[k]
	sh := rb.sharedSide(k)
	if (t < end.kink) == (sh == 0) {
		return rb.mitreRoot(k, func(x rbSection) float64 { return rb.mitreHeight(k, x.arc.PointAt(t)) })
	}
	return rb.wallRoot(k, func(x rbSection) float64 { return rb.wallHeight(k, x.arc.PointAt(t)) })
}

// wallRoot is the arc length near end k where the height g gives, over
// the mate's wall, is zero, and its section.
func (rb *rollingBall) wallRoot(k int, g func(rbSection) float64) (float64, rbSection, error) {
	s0 := 0.0
	if k == 1 {
		s0 = rb.length
	}
	return rb.planeRoot(g, s0, endDir(k), rb.t.Vertices[rb.ends[k].v].P)
}

// findKink sets kinked end k's wall's normal's sign and the arc
// parameter of its kink: where, going from the shared side's foot to
// the creased side's, the cut by the mate's swept surface leaves the
// mate's fillet, at its foot on its wall.
func (rb *rollingBall) findKink(k int) error {
	end := rb.ends[k]
	at := rb.t.Vertices[end.v].P
	s := rb.t.Surface(end.f)
	if s == nil {
		return filletErr(at, "the face across the crease is not in the shell")
	}
	u, v, _ := nearestOnWithin(s, at, 0.5)
	n := s.NormalAt(u, v)
	if n.Magnitude() == 0 {
		return filletErr(at, "the face across the crease has no normal here")
	}
	end.wallSign = 1
	if n.Dot(rb.beyond(k)) < 0 {
		end.wallSign = -1
	}
	// mitreFirst reports whether at t the swept surface cuts first: where
	// the cut lies on the mate's fillet, between its feet. Past the kink
	// the two swept surfaces still meet, but on the mate's ball beyond
	// its foot on the wall, which the mate's fillet is tangent to there.
	mitreFirst := func(t float64) bool {
		_, sec, err := rb.mitreRoot(k, func(x rbSection) float64 { return rb.mitreHeight(k, x.arc.PointAt(t)) })
		return err == nil && rb.onMateArc(k, sec.arc.PointAt(t))
	}
	shared, creased := 0.02, 0.98
	if rb.sharedSide(k) == 1 {
		shared, creased = creased, shared
	}
	if !mitreFirst(shared) || mitreFirst(creased) {
		return filletErr(at, "the fillets of two radii don't meet in a mitre here: pick them one at a time")
	}
	for range 50 {
		m := (shared + creased) / 2
		if mitreFirst(m) {
			shared = m
		} else {
			creased = m
		}
	}
	end.kink = (shared + creased) / 2
	return nil
}

// mitreTs are the parameters of the ball's arcs at which the mitre's arc
// lengths near end k are found: mitreGrid steps across, or at a kinked
// end half of them each side of the kink, the kink among them.
func (rb *rollingBall) mitreTs(k int) []float64 {
	ts := make([]float64, mitreGrid+1)
	if !rb.kinkedEnd(k) {
		for j := range ts {
			ts[j] = float64(j) / mitreGrid
		}
		return ts
	}
	kt, h := rb.ends[k].kink, mitreGrid/2
	for j := range h + 1 {
		ts[j] = kt * float64(j) / float64(h)
		ts[h+j] = kt + (1-kt)*float64(j)/float64(h)
	}
	return ts
}

// kinkedFit is the section through the points xs at parameters ts of
// the ball's arc ball, from foot f0 to foot f1, fitted in two spans that
// meet at the kink (ts and xs give its point at index mitreGrid/2), each
// a quintic of the arc's raised weights: the other kinks of the chain
// are put in after.
func (rb *rollingBall) kinkedFit(k int, ball srf.BSplineCurve, f0, f1 rbFoot, ts []float64, xs []geom.Vector) (srf.BSplineCurve, []float64, []geom.Vector, bool) {
	kt := rb.ends[k].kink
	c := raised(ball)
	for range endDeg {
		c = c.WithKnot(kt)
	}
	h := mitreGrid / 2
	q := xs[h]
	free := []geom.Vector{{X: 1}, {Y: 1}, {Z: 1}}
	face := func(f rbFoot) []geom.Vector {
		if rb.flat {
			return free
		}
		return []geom.Vector{f.n.NormalDir(0), f.n.NormalDir(1)}
	}
	var spans [2]srf.BSplineCurve
	var us []float64
	var allX []geom.Vector
	for i := range 2 {
		lo, hi := 0.0, kt
		ctrl := slices.Clone(c.Ctrl[:endDeg+1])
		w := slices.Clone(c.Weight[:endDeg+1])
		ends := [2]geom.Vector{f0.p, q}
		ways := [2][]geom.Vector{face(f0), free}
		var sts []float64
		var sxs []geom.Vector
		for j := 1; j < h; j++ {
			sts = append(sts, ts[j]/kt)
			sxs = append(sxs, xs[j])
		}
		if i == 1 {
			lo, hi = kt, 1
			ctrl = slices.Clone(c.Ctrl[endDeg:])
			w = slices.Clone(c.Weight[endDeg:])
			ends = [2]geom.Vector{q, f1.p}
			ways = [2][]geom.Vector{free, face(f1)}
			sts, sxs = sts[:0], sxs[:0]
			for j := h + 1; j < len(ts)-1; j++ {
				sts = append(sts, (ts[j]-kt)/(1-kt))
				sxs = append(sxs, xs[j])
			}
		}
		ctrl[0], ctrl[endDeg] = ends[0], ends[1]
		span := srf.BSplineCurve{Deg: endDeg, Knots: bezierKnots(endDeg), Ctrl: ctrl, Weight: w}
		fit, fu, ok := fitThrough(span, w, ways, sts, sxs)
		if !ok {
			return fit, nil, nil, false
		}
		spans[i] = fit
		for j, u := range fu {
			us = append(us, lo+(hi-lo)*u)
			allX = append(allX, sxs[j])
		}
	}
	out := c
	out.Ctrl = append(slices.Clone(spans[0].Ctrl), spans[1].Ctrl[1:]...)
	for _, t := range rb.kinks {
		if math.Abs(t-kt) < 1e-12 {
			continue
		}
		for range endDeg {
			out = out.WithKnot(t)
		}
	}
	return out, us, allX, true
}

// mateSpan is the first span of end k's kinked mate's mitre, from the
// face both share to the kink, as a quintic run from this chain's side
// 0 to its side 1.
func (rb *rollingBall) mateSpan(k int) srf.BSplineCurve {
	end := rb.ends[k]
	me := end.mate.ends[end.mateK]
	arc := me.shared.arc
	span := srf.BSplineCurve{Deg: endDeg, Knots: bezierKnots(endDeg)}
	i := slices.IndexFunc(arc.Knots, func(x float64) bool { return math.Abs(x-me.kink) < 1e-12 }) - 1
	if end.mate.sharedSide(end.mateK) == 0 {
		span.Ctrl, span.Weight = slices.Clone(arc.Ctrl[:i+1]), slices.Clone(arc.Weight[:i+1])
		span.Ctrl, span.Weight = span.Ctrl[len(span.Ctrl)-endDeg-1:], span.Weight[len(span.Weight)-endDeg-1:]
	} else {
		span.Ctrl, span.Weight = slices.Clone(arc.Ctrl[i:i+endDeg+1]), slices.Clone(arc.Weight[i:i+endDeg+1])
	}
	return span
}

// mateCreaseEnd is where end k's kinked mate's mitre meets the crease:
// its arc's end on the mate's creased side.
func (rb *rollingBall) mateCreaseEnd(k int) geom.Vector {
	end := rb.ends[k]
	me := end.mate.ends[end.mateK]
	arc := me.shared.arc
	if end.mate.sharedSide(end.mateK) == 0 {
		return arc.Ctrl[len(arc.Ctrl)-1]
	}
	return arc.Ctrl[0]
}

// mateWallSpan is the second span of end k's kinked mate's mitre, on
// this chain's wall, from the kink to the crease.
func (rb *rollingBall) mateWallSpan(k int) srf.SBezier {
	end := rb.ends[k]
	me := end.mate.ends[end.mateK]
	arc := me.shared.arc
	i := slices.IndexFunc(arc.Knots, func(x float64) bool { return math.Abs(x-me.kink) < 1e-12 }) - 1
	var sb srf.SBezier
	sb.Degree = endDeg
	if end.mate.sharedSide(end.mateK) == 0 {
		for j := range endDeg + 1 {
			sb.Ctrl[j], sb.Weight[j] = arc.Ctrl[i+j], arc.Weight[i+j]
		}
		return sb
	}
	// From the crease to the kink: run the other way.
	for j := range endDeg + 1 {
		sb.Ctrl[j], sb.Weight[j] = arc.Ctrl[i-j], arc.Weight[i-j]
	}
	return sb
}

// onMateArc reports whether p, on the surface end k's mate sweeps, lies
// on its fillet: on the arc between its feet in the section through p.
func (rb *rollingBall) onMateArc(k int, p geom.Vector) bool {
	end := rb.ends[k]
	cs := end.mate.centresNear(end.mateK)
	j, t, ok := cs.nearest(p)
	if !ok {
		return false
	}
	s := cs.s[j]
	if t != 0 {
		s += (cs.s[j+1] - cs.s[j]) * t
	}
	sec, err := end.mate.section(s)
	if err != nil {
		return false
	}
	a, b, x := sec.foot[0].p.Minus(sec.c), sec.foot[1].p.Minus(sec.c), p.Minus(sec.c)
	n := a.Cross(b)
	return a.Cross(x).Dot(n) >= 0 && x.Cross(b).Dot(n) >= 0
}

// copiesKink reports whether an end of the chain copies a kinked
// mitre's first span: one span across, whose edge the mate's patch
// shares whole, so the chain's sections are not split at their middles.
func (rb *rollingBall) copiesKink() bool {
	return rb.mateKinked(0) || rb.mateKinked(1)
}

// matchParams is span, the first span of end k's kinked mate's mitre,
// reparametrized as near as a rational curve can be to the parameters
// of this chain's own arcs at its points: its weights scaled by c^i, the
// curve the same, c chosen so that its points at this end's mitre
// parameters come nearest the mitre's own there. The mate's arcs are
// parametrized otherwise, and the sections next to the copy, this
// chain's own, would skin twisted against it.
func (rb *rollingBall) matchParams(k int, span srf.BSplineCurve) srf.BSplineCurve {
	ms, err := rb.mitreRoots(k)
	if err != nil {
		return span
	}
	ts := rb.mitreTs(k)
	var at []float64
	var xs []geom.Vector
	for j := 1; j < len(ts)-1; j++ {
		sec, err := rb.section(ms[j])
		if err != nil {
			return span
		}
		at = append(at, ts[j])
		xs = append(xs, sec.arc.PointAt(ts[j]))
	}
	scaled := func(lc float64) srf.BSplineCurve {
		out := span
		out.Weight = slices.Clone(span.Weight)
		c := math.Exp(lc)
		for i := range out.Weight {
			out.Weight[i] *= math.Pow(c, float64(i))
		}
		return out
	}
	cost := func(lc float64) float64 {
		c := scaled(lc)
		sum := 0.0
		for j, t := range at {
			sum += c.PointAt(t).Minus(xs[j]).MagSquared()
		}
		return sum
	}
	lo, hi := -2.0, 2.0
	g := (math.Sqrt(5) - 1) / 2
	for range 80 {
		a, b := hi-g*(hi-lo), lo+g*(hi-lo)
		if cost(a) < cost(b) {
			hi = b
		} else {
			lo = a
		}
	}
	if best := (lo + hi) / 2; cost(best) < cost(0) {
		return scaled(best)
	}
	return span
}
