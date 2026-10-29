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
// end half of them each side of the kink, the kink among them. At an end
// that isn't kinked, of a chain whose sections are split at kinks all the
// same (the other end's, or where a mate's mitre it copies is split),
// each span between them takes its share of the steps, so that every
// span has points to be fitted to.
func (rb *rollingBall) mitreTs(k int) []float64 {
	ts := make([]float64, mitreGrid+1)
	if rb.kinkedEnd(k) {
		kt, h := rb.ends[k].kink, mitreGrid/2
		for j := range h + 1 {
			ts[j] = kt * float64(j) / float64(h)
			ts[h+j] = kt + (1-kt)*float64(j)/float64(h)
		}
		return ts
	}
	bounds := append(append([]float64{0}, rb.kinks...), 1)
	spans := len(bounds) - 1
	j := 0
	for i := range spans {
		steps := mitreGrid / spans
		if i == spans-1 {
			steps = mitreGrid - j
		}
		a, b := bounds[i], bounds[i+1]
		for q := range steps {
			ts[j+q] = a + (b-a)*float64(q)/float64(steps)
		}
		j += steps
	}
	ts[mitreGrid] = 1
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
// 0 to its side 1. The mate's other kink, where it has one, may lie in
// it, splitting it: the span is one curve all the same.
func (rb *rollingBall) mateSpan(k int) srf.BSplineCurve {
	end := rb.ends[k]
	me := end.mate.ends[end.mateK]
	if end.mate.sharedSide(end.mateK) == 0 {
		return spanOf(me.shared.arc, 0, me.kink)
	}
	return spanOf(me.shared.arc, me.kink, 1)
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

// mateWallSpans are the second span of end k's kinked mate's mitre, on
// this chain's wall, from the kink to the crease: in pieces where the
// mate's sections are split by its other kinks, as its patches meet the
// wall.
func (rb *rollingBall) mateWallSpans(k int) []srf.SBezier {
	end := rb.ends[k]
	me := end.mate.ends[end.mateK]
	arc := me.shared.arc
	lo, hi := me.kink, 1.0
	back := end.mate.sharedSide(end.mateK) == 1
	if back {
		// From the crease to the kink: run the other way.
		lo, hi = 0, me.kink
	}
	cuts := []float64{lo}
	for _, x := range arc.Knots {
		if x > lo+kinkMerge && x < hi-kinkMerge && x != cuts[len(cuts)-1] {
			cuts = append(cuts, x)
		}
	}
	cuts = append(cuts, hi)
	var out []srf.SBezier
	for i := range len(cuts) - 1 {
		c := spanOf(arc, cuts[i], cuts[i+1])
		var sb srf.SBezier
		sb.Degree = endDeg
		for j := range endDeg + 1 {
			sb.Ctrl[j], sb.Weight[j] = c.Ctrl[j], c.Weight[j]
		}
		if back {
			sb.Reverse()
		}
		out = append(out, sb)
	}
	if back {
		slices.Reverse(out)
	}
	return out
}

// spanOf is c from a to b as one span of degree endDeg. Every knot of c
// inside it must have been put in without changing the curve, as the
// other kinks are put in a kinked mitre (kinkedFit), so that c's
// homogeneous points there are one polynomial: it is found again from
// endDeg+1 of them, spread evenly.
func spanOf(c srf.BSplineCurve, a, b float64) srf.BSplineCurve {
	n := endDeg + 1
	if i := slices.IndexFunc(c.Knots, func(x float64) bool { return x > a+kinkMerge }); i > 0 && c.Knots[i] >= b-kinkMerge {
		// One span already: its own control points.
		return srf.BSplineCurve{Deg: endDeg, Knots: bezierKnots(endDeg),
			Ctrl: slices.Clone(c.Ctrl[i-n : i]), Weight: slices.Clone(c.Weight[i-n : i])}
	}
	var rhs [4][]float64
	for d := range rhs {
		rhs[d] = make([]float64, n)
	}
	m := make([][]float64, n)
	for j := range n {
		u := float64(j) / endDeg
		m[j] = make([]float64, n)
		for i := range n {
			m[j][i] = binomial(endDeg, i) * math.Pow(u, float64(i)) * math.Pow(1-u, float64(endDeg-i))
		}
		bs := c.Basis(a + (b-a)*u)
		for i, bi := range bs {
			w := 1.0
			if c.Weight != nil {
				w = c.Weight[i]
			}
			p := c.Ctrl[i]
			rhs[0][j] += bi * w * p.X
			rhs[1][j] += bi * w * p.Y
			rhs[2][j] += bi * w * p.Z
			rhs[3][j] += bi * w
		}
	}
	var sol [4][]float64
	for d := range sol {
		mc := make([][]float64, n)
		for j := range m {
			mc[j] = slices.Clone(m[j])
		}
		x, ok := solveDense(mc, rhs[d])
		if !ok {
			panic("topo: Bernstein interpolation singular")
		}
		sol[d] = x
	}
	out := srf.BSplineCurve{Deg: endDeg, Knots: bezierKnots(endDeg), Ctrl: make([]geom.Vector, n), Weight: make([]float64, n)}
	for i := range n {
		w := sol[3][i]
		out.Weight[i] = w
		out.Ctrl[i] = geom.Vector{X: sol[0][i] / w, Y: sol[1][i] / w, Z: sol[2][i] / w}
	}
	// The ends exactly: the curve's own points there.
	out.Ctrl[0], out.Ctrl[endDeg] = knotPoint(c, a), knotPoint(c, b)
	return out
}

// knotPoint is c's point at t: where t is a knot of full multiplicity,
// as the ends and kinks are, the control point there, which the curves
// the point ends are made from hold too.
func knotPoint(c srf.BSplineCurve, t float64) geom.Vector {
	switch {
	case t == c.Knots[0]:
		return c.Ctrl[0]
	case t == c.Knots[len(c.Knots)-1]:
		return c.Ctrl[len(c.Ctrl)-1]
	}
	if i := slices.Index(c.Knots, t); i > 0 && i+c.Deg <= len(c.Knots) && c.Knots[i+c.Deg-1] == t {
		return c.Ctrl[i-1]
	}
	return c.PointAt(t)
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
	if end.mate.flat {
		// A chamfer's line: between its feet.
		d := sec.foot[1].p.Minus(sec.foot[0].p)
		l := p.Minus(sec.foot[0].p).Dot(d) / d.MagSquared()
		return l >= 0 && l <= 1
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
