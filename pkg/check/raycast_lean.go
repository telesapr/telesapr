package check

import (
	"cmp"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// leanSweep is a profile whose plane does not hold the axis, turned about
// it and moved along it: the points x that some angle th, from th0 to th1,
// takes into the profile when x is turned back by th about w through c
// and moved back d0 + slope (th - th0) along w. A lathe, a revolve and a
// sweep's arc have no slope. Nothing here is a surface of the kernel's:
// membership is that definition, solved for th.
//
// Turned back by th, a point q from c stays on the profile's plane,
// n·(x - c) = k, where
//
//	f(th) = nz (z - d(th)) + m (X cos th + Y sin th) - k = 0,
//
// n = nz w + m a, b = w × a, and q = z w + X a + Y b. f turns only where
// its derivative is nothing, which is an arc sine, so each stretch
// between has one root at most, found by bisection.
//
// The profile moves through its own plane forward at some points and back
// at others where the velocity's part along the plane's normal, linear in
// the point, changes sign. A line of the plane divides them; where it
// crosses the profile, what the line sweeps bounds the solid too.
type leanSweep struct {
	pr        profile // in its own plane, from the loop set's point
	c, w      geom.Vector
	a, b      geom.Vector
	nz, m, k  float64
	th0, th1  float64
	d0, slope float64
	// A point of the profile's plane, c + z w + X a + Y b, has the
	// profile's coordinates z uw + X ua + Y ub - u0, and the same with v.
	uw, ua, ub, u0 float64
	vw, va, vb, v0 float64
	zLo, zHi, rHi  float64
	// edges are the polygons' edges, from c, and the pieces of the
	// dividing line inside the profile: what sweeps out the boundary
	// between the caps.
	edges  [][2]geom.Vector
	lo, hi geom.Vector
}

// throughAxis reports whether the loop set's plane holds the axis through
// c along w, as a profile in the plane through it does; newRevolution and
// newHelix work in that plane.
func throughAxis(sbls *srf.SBezierLoopSet, c, w geom.Vector) bool {
	n := sbls.Normal.WithMagnitude(1)
	return math.Abs(n.Dot(w)) <= 1e-9 && math.Abs(sbls.Point.Minus(c).Dot(n)) <= 1e-6
}

// newLeanSweep is the loop set swept about the axis through c along the
// unit vector w from the angle th0, moved d0 along it, to th1, moved d1.
// It is false where it turns through no angle.
func newLeanSweep(sbls *srf.SBezierLoopSet, c, w geom.Vector, th0, th1, d0, d1 float64) (*leanSweep, bool) {
	if th1 < th0 {
		th0, th1, d0, d1 = th1, th0, d1, d0
	}
	if th1 == th0 {
		return nil, false
	}
	n := sbls.Normal.WithMagnitude(1)
	eu := n.NormalDir(0).WithMagnitude(1)
	ev := n.Cross(eu)
	ls := &leanSweep{c: c, w: w, th0: th0, th1: th1, d0: d0, slope: (d1 - d0) / (th1 - th0)}
	// A moving sweep tries every edge at every degree of the turn: a
	// round profile is cut ten times as coarse there, still a hundredth of
	// any mesh's chord, for a third of the edges.
	tol := float64(profileTol)
	if ls.slope != 0 {
		tol *= 10
	}
	ls.pr = newProfileAt(sbls, sbls.Point, eu, ev, tol)
	ls.nz = n.Dot(w)
	square := n.Minus(w.ScaledBy(ls.nz))
	if ls.m = square.Magnitude(); ls.m > 1e-12 {
		ls.a = square.ScaledBy(1 / ls.m)
	} else {
		ls.m, ls.a = 0, w.NormalDir(0).WithMagnitude(1)
	}
	ls.b = w.Cross(ls.a)
	p0 := sbls.Point.Minus(c)
	ls.k = n.Dot(p0)
	ls.uw, ls.ua, ls.ub, ls.u0 = w.Dot(eu), ls.a.Dot(eu), ls.b.Dot(eu), p0.Dot(eu)
	ls.vw, ls.va, ls.vb, ls.v0 = w.Dot(ev), ls.a.Dot(ev), ls.b.Dot(ev), p0.Dot(ev)

	// The profile's corners from c: their heights and their distance
	// from the axis, and the edges between them.
	at := func(v [2]float64) geom.Vector { return p0.Plus(eu.ScaledBy(v[0])).Plus(ev.ScaledBy(v[1])) }
	ls.zLo, ls.zHi = math.Inf(1), math.Inf(-1)
	for _, poly := range ls.pr.polys {
		for i, v := range poly {
			q := at(v)
			z := q.Dot(w)
			ls.zLo, ls.zHi = math.Min(ls.zLo, z), math.Max(ls.zHi, z)
			ls.rHi = math.Max(ls.rHi, q.Minus(w.ScaledBy(z)).Magnitude())
			ls.edges = append(ls.edges, [2]geom.Vector{q, at(poly[(i+1)%len(poly)])})
		}
	}
	// The velocity along the plane's normal at a point q from c is
	// q·across + slope nz, linear over the plane.
	across := n.Cross(w)
	for _, s := range dividingLine(ls.pr.polys, p0.Dot(across)+ls.slope*ls.nz, eu.Dot(across), ev.Dot(across)) {
		ls.edges = append(ls.edges, [2]geom.Vector{at(s[0]), at(s[1])})
	}

	ls.lo = geom.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	ls.hi = ls.lo.ScaledBy(-1)
	for _, z := range []float64{ls.zLo + math.Min(d0, d1), ls.zHi + math.Max(d0, d1)} {
		for _, s := range []float64{-1, 1} {
			for _, t := range []float64{-1, 1} {
				p := c.Plus(w.ScaledBy(z)).Plus(ls.a.ScaledBy(s * ls.rHi)).Plus(ls.b.ScaledBy(t * ls.rHi))
				p.MakeMaxMin(&ls.hi, &ls.lo)
			}
		}
	}
	return ls, true
}

// dividingLine is the pieces of the line g0 + gu u + gv v = 0 inside the
// polygons, by the even-odd rule along it.
func dividingLine(polys [][][2]float64, g0, gu, gv float64) [][2][2]float64 {
	if gu == 0 && gv == 0 {
		return nil
	}
	type hit struct {
		along float64
		p     [2]float64
	}
	var hits []hit
	for _, poly := range polys {
		for i := range poly {
			p, q := poly[i], poly[(i+1)%len(poly)]
			gp, gq := g0+gu*p[0]+gv*p[1], g0+gu*q[0]+gv*q[1]
			if (gp > 0) == (gq > 0) {
				continue
			}
			t := gp / (gp - gq)
			x := [2]float64{p[0] + t*(q[0]-p[0]), p[1] + t*(q[1]-p[1])}
			hits = append(hits, hit{along: gu*x[1] - gv*x[0], p: x})
		}
	}
	slices.SortFunc(hits, func(a, b hit) int { return cmp.Compare(a.along, b.along) })
	var out [][2][2]float64
	for i := 0; i+1 < len(hits); i += 2 {
		out = append(out, [2][2]float64{hits[i].p, hits[i+1].p})
	}
	return out
}

func (ls *leanSweep) inside(x geom.Vector) bool {
	q := x.Minus(ls.c)
	z := q.Dot(ls.w)
	X, Y := q.Dot(ls.a), q.Dot(ls.b)
	r := math.Hypot(X, Y)
	if r > ls.rHi {
		return false
	}
	// The angles at which the point could be at one of the profile's
	// heights.
	t0, t1 := ls.th0, ls.th1
	switch {
	case ls.slope != 0:
		s0 := ls.th0 + (z-ls.d0-ls.zHi)/ls.slope
		s1 := ls.th0 + (z-ls.d0-ls.zLo)/ls.slope
		t0, t1 = math.Max(t0, math.Min(s0, s1)), math.Min(t1, math.Max(s0, s1))
	case z-ls.d0 < ls.zLo || z-ls.d0 > ls.zHi:
		return false
	}
	if t0 > t1 {
		return false
	}
	height := func(th float64) float64 { return z - ls.d0 - ls.slope*(th-ls.th0) }
	f := func(th float64) float64 {
		sin, cos := math.Sincos(th)
		return ls.nz*height(th) + ls.m*(X*cos+Y*sin) - ls.k
	}
	var buf [16]float64
	cuts := append(buf[:0], t0, t1)
	if mr := ls.m * r; mr > 0 {
		if s := ls.nz * ls.slope / mr; math.Abs(s) <= 1 {
			psi, turn := math.Atan2(Y, X), math.Asin(s)
			for _, th := range [2]float64{psi - turn, psi - math.Pi + turn} {
				th += 2 * math.Pi * math.Ceil((t0-th)/(2*math.Pi))
				for ; th < t1; th += 2 * math.Pi {
					cuts = append(cuts, th)
				}
			}
		}
	}
	slices.Sort(cuts)
	for i := 1; i < len(cuts); i++ {
		a, b := cuts[i-1], cuts[i]
		fa, fb := f(a), f(b)
		if (fa > 0) == (fb > 0) && fa != 0 && fb != 0 {
			continue
		}
		for range 60 {
			mid := (a + b) / 2
			if mid <= a || mid >= b {
				break
			}
			if fm := f(mid); (fm > 0) == (fa > 0) {
				a, fa = mid, fm
			} else {
				b = mid
			}
		}
		th := (a + b) / 2
		sin, cos := math.Sincos(th)
		h, xa, xb := height(th), X*cos+Y*sin, Y*cos-X*sin
		if ls.pr.contains(h*ls.uw+xa*ls.ua+xb*ls.ub-ls.u0, h*ls.vw+xa*ls.va+xb*ls.vb-ls.v0) {
			return true
		}
	}
	return false
}

// crossings are where the ray meets the two caps and what each of the
// edges sweeps. A sweep that only turns sweeps from an edge a plane
// square to the axis or a quadric, met in closed form; one that moves
// along the axis too sweeps a ruled helicoid, met at the angles
// movedCrossings finds.
func (ls *leanSweep) crossings(o, d geom.Vector, out []float64) []float64 {
	if _, _, ok := boxSpan(ls.lo, ls.hi, o, d); !ok {
		return out
	}
	q := o.Minus(ls.c)
	if ls.slope != 0 || ls.th1-ls.th0 < 2*math.Pi {
		// The profile's plane moved to each end.
		for _, th := range [2]float64{ls.th0, ls.th1} {
			sin, cos := math.Sincos(th)
			n := ls.w.ScaledBy(ls.nz).Plus(ls.a.ScaledBy(ls.m * cos)).Plus(ls.b.ScaledBy(ls.m * sin))
			if dn := d.Dot(n); dn != 0 {
				rise := ls.d0 + ls.slope*(th-ls.th0)
				out = append(out, (ls.k+rise*ls.nz-q.Dot(n))/dn)
			}
		}
	}
	if ls.slope != 0 {
		return ls.movedCrossings(q, d, out)
	}
	for _, e := range ls.edges {
		out = turnedSegmentCrossings(e[0], e[1], ls.w, q.Minus(ls.w.ScaledBy(ls.d0)), d, out)
	}
	return out
}

// leanAngle is how far apart the angles are at which movedCrossings
// looks for a change of sign. A line meets what one edge sweeps at few
// angles of a turn; two closer than this are a ray grazing it.
const leanAngle = math.Pi / 180

// movedCrossings appends where the line q + s d, q from c, meets what
// each edge sweeps while it turns and moves along the axis. At the angle
// th the edge starts at p(th) from q and runs along r(th), and the two
// lines meet where
//
//	g(th) = p(th) · (r(th) × d) = 0,
//
// a function of the angle alone (edgeMeets): its changes of sign are
// closed in on, and each is kept where the lines meet on the edge.
func (ls *leanSweep) movedCrossings(q, d geom.Vector, out []float64) []float64 {
	steps := max(1, int(math.Ceil((ls.th1-ls.th0)/leanAngle)))
	dth := (ls.th1 - ls.th0) / float64(steps)
	turns := make([][2]float64, steps+1)
	for i := range turns {
		sin, cos := math.Sincos(ls.th0 + float64(i)*dth)
		turns[i] = [2]float64{cos, sin}
	}
	w := ls.w
	// The axis's part of the motion, from q, at the angle nothing.
	from := w.ScaledBy(ls.d0 - ls.slope*ls.th0).Minus(q)
	for _, edge := range ls.edges {
		// A vector turned by th is its part along w, cos th of its part
		// square to w, and sin th of w × that: p = p0 + th slope w +
		// cos th pc + sin th ps, and r the same without the axis's motion.
		a, e := edge[0], edge[1].Minus(edge[0])
		aw, ew := w.ScaledBy(a.Dot(w)), w.ScaledBy(e.Dot(w))
		pc, rc := a.Minus(aw), e.Minus(ew)
		ps, rs := w.Cross(pc), w.Cross(rc)
		p0 := aw.Plus(from)
		g := newEdgeMeets(p0, w.ScaledBy(ls.slope), pc, ps, ew.Cross(d), rc.Cross(d), rs.Cross(d))
		prev := g.at(ls.th0, turns[0][0], turns[0][1])
		for i := 1; i <= steps; i++ {
			next := g.at(ls.th0+float64(i)*dth, turns[i][0], turns[i][1])
			if (next > 0) == (prev > 0) {
				prev = next
				continue
			}
			th := g.root(ls.th0+float64(i-1)*dth, ls.th0+float64(i)*dth, prev, next)
			prev = next
			// Where they meet, p + mu r = s d.
			sin, cos := math.Sincos(th)
			p := p0.Plus(w.ScaledBy(ls.slope * th)).Plus(pc.ScaledBy(cos)).Plus(ps.ScaledBy(sin))
			r := ew.Plus(rc.ScaledBy(cos)).Plus(rs.ScaledBy(sin))
			n := r.Cross(d)
			nn := n.Dot(n)
			if nn == 0 {
				continue
			}
			if mu := d.Cross(p).Dot(n) / nn; mu >= -1e-9 && mu <= 1+1e-9 {
				out = append(out, r.Cross(p).Dot(n)/nn)
			}
		}
	}
	return out
}

// edgeMeets is g(th) = p(th) · m(th) multiplied out in th, cos th and
// sin th, for p = p0 + th k + cos th pc + sin th ps and m = m0 +
// cos th mc + sin th ms with k·m0 = 0: the coefficients are the edge's
// and the ray's, so that each angle costs a handful of products.
type edgeMeets struct {
	one, c, s, tc, ts, cc, ss, cs float64
}

func newEdgeMeets(p0, k, pc, ps, m0, mc, ms geom.Vector) edgeMeets {
	return edgeMeets{
		one: p0.Dot(m0),
		c:   p0.Dot(mc) + pc.Dot(m0),
		s:   p0.Dot(ms) + ps.Dot(m0),
		tc:  k.Dot(mc),
		ts:  k.Dot(ms),
		cc:  pc.Dot(mc),
		ss:  ps.Dot(ms),
		cs:  pc.Dot(ms) + ps.Dot(mc),
	}
}

func (g *edgeMeets) at(th, cos, sin float64) float64 {
	return g.one + cos*(g.c+th*g.tc+cos*g.cc+sin*g.cs) + sin*(g.s+th*g.ts+sin*g.ss)
}

// root closes in on g's change of sign between lo and hi, where it is
// flo and fhi, by false position, halving the value kept at an end that
// stays twice (the Illinois rule).
func (g *edgeMeets) root(lo, hi, flo, fhi float64) float64 {
	side := 0
	for range 100 {
		th := hi - fhi*(hi-lo)/(fhi-flo)
		if !(th > lo && th < hi) {
			th = (lo + hi) / 2
		}
		if th <= lo || th >= hi || hi-lo <= 1e-14*(1+math.Abs(lo)) {
			return th
		}
		sin, cos := math.Sincos(th)
		f := g.at(th, cos, sin)
		switch {
		case f == 0:
			return th
		case (f > 0) == (flo > 0):
			lo, flo = th, f
			if side < 0 {
				fhi /= 2
			}
			side = -1
		default:
			hi, fhi = th, f
			if side > 0 {
				flo /= 2
			}
			side = 1
		}
	}
	return (lo + hi) / 2
}

// turnedSegmentCrossings appends where the line q + s d meets what the
// segment from a to b sweeps, turned all the way about the axis along w
// through the origin: its point at the height z is as far from the axis
// as the segment's there.
func turnedSegmentCrossings(a, b, w, q, d geom.Vector, out []float64) []float64 {
	za, zb := a.Dot(w), b.Dot(w)
	ra, rb := a.Minus(w.ScaledBy(za)), b.Minus(w.ScaledBy(zb))
	delta := rb.Minus(ra)
	z0, zd := q.Dot(w), d.Dot(w)
	if math.Abs(zb-za) <= 1e-9*(1+delta.Magnitude()) {
		// Square to the axis: a plane.
		if zd != 0 {
			out = append(out, (za-z0)/zd)
		}
		return out
	}
	// The segment's parameter at the ray's height is mu0 + mu1 s.
	mu0, mu1 := (z0-za)/(zb-za), zd/(zb-za)
	rho0, rhod := q.Minus(w.ScaledBy(z0)), d.Minus(w.ScaledBy(zd))
	e0, e1 := ra.Plus(delta.ScaledBy(mu0)), delta.ScaledBy(mu1)
	n := len(out)
	out = appendRoots(out, rhod.Dot(rhod)-e1.Dot(e1), 2*(rho0.Dot(rhod)-e0.Dot(e1)), rho0.Dot(rho0)-e0.Dot(e0))
	kept := out[:n]
	for _, s := range out[n:] {
		if mu := mu0 + mu1*s; mu >= -1e-9 && mu <= 1+1e-9 {
			kept = append(kept, s)
		}
	}
	return kept
}

func (ls *leanSweep) bounds(lo, hi *geom.Vector) {
	ls.lo.MakeMaxMin(hi, lo)
	ls.hi.MakeMaxMin(hi, lo)
}
