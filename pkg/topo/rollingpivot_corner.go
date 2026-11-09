package topo

import (
	"math"
	"slices"
	"sort"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// The corner's surface (rollingpivot.go). The ball swinging round the
// crease touches it at a depth that may first rise past the deeper
// fillet's foot, onto the crease that fillet has already taken, and
// then fall to the other's: that surface would fold. So, as
// OpenCASCADE's corners do, the corner fills the hole tangent to what is
// round it rather than following the ball exactly. Its foot on the
// crease moves evenly from one fillet's to the other's; its foot on the
// shared face is the swinging ball's, which moves smoothly; and each
// section is the arc through the two tangent to the face, which at
// either end is the fillet's own section. A chamfer's corner is ruled
// the same way: lines from the crease to the shared face, its foot
// there at the chamfer's distance from the crease as the cut swings.
// Across each end the skin takes the fillet's derivative, but at the
// crease, where its foot runs along the crease.

// pivotMaxSections is how many sections a corner is skinned through at
// most.
const pivotMaxSections = 64

// pivotSamples is how many points of the swing are worked out exactly,
// for the corner's parameter along it.
const pivotSamples = 64

// pivotFrame is what a corner is worked out from: the vertex and the
// crease's way down from it, e; the swing from u0 to u1 about it, theta,
// in the plane of u0 and w; the depths of the two fillets' feet on the
// crease; and the two fillets' ends.
type pivotFrame struct {
	c           *pivotCorner
	ra, rb      *rollingBall
	ia, ib      int
	secA, secB  rbSection
	v, e        geom.Vector
	u0, u1, w   geom.Vector
	theta       float64
	ha, hb      float64
	r           float64
	flat        bool
	path        []pivotSwing
	total       float64
	ref         srf.BSplineCurve
	refRb       *rollingBall
	first, last srf.BSplineCurve
}

// pivotSwing is a point of the swing: phi of the way round, sigma along
// its path; where it touches the shared face, d, the face's normal there
// into the solid, n, and the point whose path measures it, c: the ball's
// centre, or for a chamfer d.
type pivotSwing struct {
	phi, sigma float64
	c, d, n    geom.Vector
}

// newPivotCorner works out the corner's surface.
func newPivotCorner(a *rbBlend, ka int, b *rbBlend, kb int) (*pivotCorner, error) {
	pf, err := newPivotFrame(a, ka, b, kb)
	if err != nil {
		return nil, err
	}
	if err := pf.sample(); err != nil {
		return nil, err
	}
	if err := pf.sectionsLike(); err != nil {
		return nil, err
	}
	d0, err := pf.deriv(a, ka, pf.secA, pf.ia, endDir(ka), false)
	if err != nil {
		return nil, err
	}
	d1, err := pf.deriv(b, kb, pf.secB, pf.ib, -endDir(kb), pf.ia != pf.ib)
	if err != nil {
		return nil, err
	}
	all, err := pf.skin(d0, d1)
	if err != nil {
		return nil, err
	}
	pf.c.all = all
	return pf.c, nil
}

// newPivotFrame sets the corner's frame up from the two fillets' ends.
func newPivotFrame(a *rbBlend, ka int, b *rbBlend, kb int) (*pivotFrame, error) {
	ra, rbb := a.rb, b.rb
	t := ra.t
	vi := ra.ends[ka].v
	vp := t.Vertices[vi].P
	if ra.flat != rbb.flat {
		return nil, filletErr(vp, "a fillet and a chamfer can't meet at an inside corner")
	}
	if ra.r != rbb.r {
		// Two radii meet in a kinked mitre (insideMitres), not here.
		return nil, filletErr(vp, "fillets of two radii can't meet in a corner patch at an inside corner")
	}
	if len(ra.kinks) > 0 || len(rbb.kinks) > 0 || ra.copiesKink() || rbb.copiesKink() {
		// A chain kinked or copying a kinked mitre at one end has sections
		// a corner patch at its other end doesn't join soundly (seen on
		// the dish: x+ at 2, y+ and x- at 1): insideMitres makes such a
		// corner a kinked mitre instead, so none should come here.
		return nil, filletErr(vp, "a fillet that meets one of another radius at one inside corner can't "+
			"turn another by a corner patch: give them all one radius")
	}
	ia, ib := ra.creaseSide(ka), rbb.creaseSide(kb)
	if ia < 0 || ib < 0 {
		return nil, filletErr(vp, "the fillets at the inside corner have no crease")
	}
	crease := ra.ends[ka].a[ia]
	if !t.Straight(crease) {
		return nil, filletErr(vp, "the fillets can't yet turn an inside corner whose crease is curved")
	}
	pf := &pivotFrame{ra: ra, rb: rbb, ia: ia, ib: ib, secA: a.endSec(ka), secB: b.endSec(kb),
		v: vp, e: edgeDirAt(t, crease, vi), r: ra.r, flat: ra.flat}
	c := &pivotCorner{a: a, b: b, ka: ka, kb: kb, v: vp,
		qa: pf.secA.foot[ia].p, qb: pf.secB.foot[ib].p, da: pf.secA.foot[1-ia].p, db: pf.secB.foot[1-ib].p,
		fa: pf.secA.foot[ia].f, fb: pf.secB.foot[ib].f, fd: pf.secA.foot[1-ia].f}
	pf.c = c
	if pf.secB.foot[1-ib].f != c.fd {
		return nil, filletErr(vp, "the fillets' corner at the inside corner crosses a seam of the face they share: make them smaller")
	}
	if ia == 1 {
		c.creaseU = 1
	}
	pf.ha, pf.hb = c.qa.Minus(vp).Dot(pf.e), c.qb.Minus(vp).Dot(pf.e)
	// The way it swings, square to the crease: from a's centre to b's,
	// or a chamfer's foot on the shared face.
	from, to := pf.secA.c, pf.secB.c
	if pf.flat {
		from, to = c.da, c.db
	}
	u0, u1 := pf.square(from.Minus(c.qa)), pf.square(to.Minus(c.qb))
	if u0.Magnitude() < 1e-9*pf.r || u1.Magnitude() < 1e-9*pf.r {
		return nil, filletErr(vp, "the fillets don't stand off the crease at the inside corner")
	}
	pf.u0, pf.u1 = u0.WithMagnitude(1), u1.WithMagnitude(1)
	pf.theta = math.Acos(math.Max(-1, math.Min(1, pf.u0.Dot(pf.u1))))
	if pf.theta < 1e-6 {
		return nil, filletErr(vp, "the fillets meet at the inside corner without turning")
	}
	pf.w = pf.e.Cross(pf.u0)
	if pf.w.Dot(pf.u1) < 0 {
		pf.w = pf.w.ScaledBy(-1)
	}
	return pf, nil
}

// square is x less its part along the crease.
func (pf *pivotFrame) square(x geom.Vector) geom.Vector {
	return x.Minus(pf.e.ScaledBy(x.Dot(pf.e)))
}

// way is the direction phi of the way round, square to the crease.
func (pf *pivotFrame) way(phi float64) geom.Vector {
	return pf.u0.ScaledBy(math.Cos(phi)).Plus(pf.w.ScaledBy(math.Sin(phi)))
}

// near is the chain and side whose shared face is looked for phi of the
// way round: a's for the first half, b's for the second.
func (pf *pivotFrame) near(phi float64) (*rollingBall, int) {
	if phi > pf.theta/2 {
		return pf.rb, 1 - pf.ib
	}
	return pf.ra, 1 - pf.ia
}

// swingAt is the swing phi of the way round: the ball's, on the crease
// where it stands a radius off the shared face; for a chamfer the
// shared face's point under the crease's at a depth and a distance from
// it each moved evenly from a's to b's.
func (pf *pivotFrame) swingAt(phi float64) (pivotSwing, error) {
	rb, side := pf.near(phi)
	if pf.flat {
		c := pf.c
		f := phi / pf.theta
		hda, hdb := c.da.Minus(pf.v).Dot(pf.e), c.db.Minus(pf.v).Dot(pf.e)
		ra, rb2 := pf.square(c.da.Minus(c.qa)).Magnitude(), pf.square(c.db.Minus(c.qb)).Magnitude()
		x := pf.v.Plus(pf.e.ScaledBy(hda + (hdb-hda)*f)).Plus(pf.way(phi).ScaledBy(ra + (rb2-ra)*f))
		ft := rb.foot(side, x)
		return pivotSwing{phi: phi, c: ft.p, d: ft.p, n: ft.n}, nil
	}
	d := pf.way(phi)
	gap := func(h float64) (pivotSwing, float64) {
		cc := pf.v.Plus(pf.e.ScaledBy(h)).Plus(d.ScaledBy(pf.r))
		ft := rb.foot(side, cc)
		dist := cc.Minus(ft.p)
		return pivotSwing{phi: phi, c: cc, d: ft.p, n: dist.WithMagnitude(1)}, dist.Magnitude() - pf.r
	}
	h0 := pf.ha + (pf.hb-pf.ha)*phi/pf.theta
	sw0, g0 := gap(h0)
	if g0 == 0 {
		return sw0, nil
	}
	// Out from h0 either way to the nearest change of sign.
	step := pf.r / 32
	lo, hi := math.NaN(), math.NaN()
	for i := 1; i <= 128 && math.IsNaN(lo); i++ {
		for _, x := range []float64{h0 - float64(i)*step, h0 + float64(i)*step} {
			if _, g := gap(x); (g > 0) != (g0 > 0) {
				lo, hi = h0, x
				break
			}
		}
	}
	if math.IsNaN(lo) {
		return pivotSwing{}, filletErr(pf.v, "the ball can't swing round the inside corner here: make the fillet smaller")
	}
	for range 100 {
		m := (lo + hi) / 2
		if _, g := gap(m); (g > 0) == (g0 > 0) {
			lo = m
		} else {
			hi = m
		}
	}
	sw, _ := gap((lo + hi) / 2)
	return sw, nil
}

// sample works the swing out at pivotSamples+1 points, with the length
// of its path up to each.
func (pf *pivotFrame) sample() error {
	pf.path = pf.path[:0]
	for j := range pivotSamples + 1 {
		sw, err := pf.swingAt(pf.theta * float64(j) / pivotSamples)
		if err != nil {
			return err
		}
		if j > 0 {
			sw.sigma = pf.path[j-1].sigma + sw.c.Minus(pf.path[j-1].c).Magnitude()
		}
		pf.path = append(pf.path, sw)
	}
	pf.total = pf.path[len(pf.path)-1].sigma
	if pf.total < 1e-9*pf.r {
		return filletErr(pf.v, "the fillets' corner at the inside corner has no length")
	}
	return nil
}

// at is the swing at share t of its path's length.
func (pf *pivotFrame) at(t float64) pivotSwing {
	x := t * pf.total
	j := min(max(sort.Search(len(pf.path), func(i int) bool { return pf.path[i].sigma >= x })-1, 0), len(pf.path)-2)
	a, b := pf.path[j], pf.path[j+1]
	f := (x - a.sigma) / (b.sigma - a.sigma)
	if sw, err := pf.swingAt(a.phi + f*(b.phi-a.phi)); err == nil {
		return sw
	}
	return pivotSwing{c: a.c.Plus(b.c.Minus(a.c).ScaledBy(f)), d: a.d.Plus(b.d.Minus(a.d).ScaledBy(f)),
		n: a.n.Plus(b.n.Minus(a.n).ScaledBy(f)).WithMagnitude(1)}
}

// section is the corner's section at t, from the crease to the shared
// face: its two feet, and for a fillet the arc's centre and radius, NaN
// where none fits.
func (pf *pivotFrame) section(t float64) (q, d, centre geom.Vector, rho float64) {
	sw := pf.at(t)
	q = pf.c.qa.Plus(pf.c.qb.Minus(pf.c.qa).ScaledBy(t))
	if pf.flat {
		return q, sw.d, geom.Vector{}, 0
	}
	qd := q.Minus(sw.d)
	den := 2 * qd.Dot(sw.n)
	if den < 1e-12*pf.r {
		return q, sw.d, sw.d, math.NaN()
	}
	rho = qd.MagSquared() / den
	return q, sw.d, sw.d.Plus(sw.n.ScaledBy(rho)), rho
}

// curve is the section at t as the fillets' sections are, and whether
// it can be.
func (pf *pivotFrame) curve(t float64) (srf.BSplineCurve, bool) {
	q, d, centre, rho := pf.section(t)
	if math.IsNaN(rho) {
		return srf.BSplineCurve{}, false
	}
	f0, f1 := q, d
	if pf.ia == 1 {
		f0, f1 = d, q
	}
	x := pf.ra.arcOf(centre, f0, f1)
	if !pf.flat {
		x = arcAbout(centre, f0, f1, rho)
	}
	return pf.like(x)
}

// like is x as the fillets' sections are, of their degree and spans,
// and whether it could be made so.
func (pf *pivotFrame) like(x srf.BSplineCurve) (srf.BSplineCurve, bool) {
	if x.Deg != pf.ref.Deg {
		x = pf.refRb.splitSection(raised(x))
	}
	return x, x.Deg == pf.ref.Deg && slices.Equal(x.Knots, pf.ref.Knots)
}

// sectionsLike sets the corner's end sections, the fillets' own as the
// corner runs, of the degree and spans of the higher of the two.
func (pf *pivotFrame) sectionsLike() error {
	pf.ref, pf.refRb = pf.secA.arc, pf.ra
	if pf.secB.arc.Deg > pf.ref.Deg {
		pf.ref, pf.refRb = pf.secB.arc, pf.rb
	}
	var ok0, ok1 bool
	pf.first, ok0 = pf.like(pf.secA.arc)
	last := pf.secB.arc
	if pf.ia != pf.ib {
		last = reversedCurve(last)
	}
	pf.last, ok1 = pf.like(last)
	if !ok0 || !ok1 {
		return filletErr(pf.v, "the fillets at the inside corner are of sections too unlike to join")
	}
	return nil
}

// deriv is the derivative across the corner's end where it meets the
// end k of bl's fillet, whose section there is sec and creased side i,
// per unit of the swing's path, the corner running into (into 1) or out
// of that fillet; but at the crease, where the corner's foot runs along
// it. flip turns it the corner's way where the fillet's sections run
// the other way.
func (pf *pivotFrame) deriv(bl *rbBlend, k int, sec rbSection, i int, into float64, flip bool) ([]geom.Vector4, error) {
	rb := bl.rb
	d, err := rb.inwardDerivative(k, sec.s)
	if err != nil {
		return nil, err
	}
	if pf.ref.Deg >= endDeg && len(d) < endDeg+1 {
		d = elevateH(d)
	}
	if len(d) == endDeg+1 {
		d = pf.refRb.splitDerivative(d)
	}
	h := math.Min(rb.r/16, rb.length/256) * -endDir(k)
	in, err := rb.section(sec.s + h)
	if err != nil {
		return nil, err
	}
	// The speed of the point the swing's path follows.
	a, b := sec.c, in.c
	if pf.flat {
		a, b = sec.foot[1-i].p, in.foot[1-i].p
	}
	speed := b.Minus(a).Magnitude() / math.Abs(h)
	if speed < 1e-9 {
		return nil, filletErr(sec.c, "the fillet stands still at the inside corner")
	}
	out := make([]geom.Vector4, len(d))
	for j := range d {
		out[j] = d[j].ScaledBy(into / speed)
	}
	if flip {
		slices.Reverse(out)
	}
	ci := 0
	if pf.c.creaseU == 1 {
		ci = len(out) - 1
	}
	along := pf.c.qb.Minus(pf.c.qa).ScaledBy(1 / pf.total)
	out[ci] = geom.Vector4{X: along.X, Y: along.Y, Z: along.Z}
	return out, nil
}

// skin is the corner's surface, cut into its patches: through more
// sections until it keeps to them within the fillets' tolerance. The
// sections are the corner's own (curve), each moved in homogeneous
// coordinates by the Hermite terms that give their run the fillets'
// derivatives at its ends, d0 and d1 per unit of the swing's path, and
// leave its ends where they are; else the skin, held to the fillets'
// derivatives, would stray from them in proportion to its spacing.
func (pf *pivotFrame) skin(d0, d1 []geom.Vector4) ([][]srf.SSurface, error) {
	s0, s1 := make([]geom.Vector4, len(d0)), make([]geom.Vector4, len(d1))
	for i := range d0 {
		s0[i] = d0[i].ScaledBy(pf.total)
	}
	for i := range d1 {
		s1[i] = d1[i].ScaledBy(pf.total)
	}
	sectionAt, err := pf.moved(s0, s1)
	if err != nil {
		return nil, err
	}
	if all, ok := pf.oneSpan(s0, s1, sectionAt); ok {
		return all, nil
	}
	return pf.manySpans(s0, s1, sectionAt)
}

// moved is the corner's sections at t (curve), moved in homogeneous
// coordinates by the Hermite terms t(1-t)² and t²(t-1) that turn their
// own derivatives at the ends into s0 and s1.
func (pf *pivotFrame) moved(s0, s1 []geom.Vector4) (func(float64) (srf.BSplineCurve, bool), error) {
	c0, c1, err := pf.ownDerivatives()
	if err != nil {
		return nil, err
	}
	for i := range c0 {
		c0[i] = s0[i].Minus(c0[i])
		c1[i] = s1[i].Minus(c1[i])
	}
	return func(t float64) (srf.BSplineCurve, bool) {
		x, ok := pf.curve(t)
		if !ok {
			return x, false
		}
		h0, h1 := t*(1-t)*(1-t), t*t*(t-1)
		h := homog(x)
		for i := range h {
			h[i] = h[i].Plus(c0[i].ScaledBy(h0)).Plus(c1[i].ScaledBy(h1))
		}
		return withHomog(x, h), true
	}, nil
}

// manySpans skins the corner through sections at even steps of t, twice
// as many each time, until it keeps to sectionAt within the fillets'
// tolerance between them.
func (pf *pivotFrame) manySpans(s0, s1 []geom.Vector4, sectionAt func(float64) (srf.BSplineCurve, bool)) ([][]srf.SSurface, error) {
	for n := 4; ; n *= 2 {
		curves := []srf.BSplineCurve{pf.first}
		ts := []float64{0}
		for j := 1; j < n; j++ {
			t := float64(j) / float64(n)
			x, ok := sectionAt(t)
			if !ok {
				return nil, filletErr(pf.at(t).d, "the corner's sections can't be made like the fillets'")
			}
			curves = append(curves, x)
			ts = append(ts, t)
		}
		curves = append(curves, pf.last)
		ts = append(ts, 1)
		sf, err := srf.SkinThrough(curves, ts, s0, s1)
		if err != nil {
			return nil, filletErr(pf.v, "the fillets' corner at the inside corner can't be skinned: %v", err)
		}
		var mid []float64
		for j := 0; j+1 < len(ts); j++ {
			for _, f := range []float64{0.25, 0.5, 0.75} {
				mid = append(mid, ts[j]+(ts[j+1]-ts[j])*f)
			}
		}
		worst, where := strays(sf, mid, sectionAt)
		if worst <= pf.ra.tol {
			all, err := sf.Patches()
			if err != nil {
				return nil, filletErr(pf.v, "the fillets' corner can't be cut into patches: %v", err)
			}
			return all, nil
		}
		if 2*n > pivotMaxSections {
			return nil, filletErr(where, "the fillets' corner at the inside corner can't be fitted to within %.3g mm here (%.3g)",
				pf.ra.tol, worst)
		}
	}
}

// strays is how far sf strays at most from the sections sectionAt gives
// at ts, along each at a few points, and where.
func strays(sf *srf.BSplineSurface, ts []float64, sectionAt func(float64) (srf.BSplineCurve, bool)) (float64, geom.Vector) {
	worst, where := 0.0, geom.Vector{}
	for _, t := range ts {
		x, ok := sectionAt(t)
		if !ok {
			return math.Inf(1), sf.PointAt(0.5, t)
		}
		for _, u := range []float64{0.05, 0.25, 0.5, 0.75, 0.95} {
			p := sf.PointAt(u, t)
			if e := x.PointAt(u).Minus(p).Magnitude(); e > worst {
				worst, where = e, p
			}
		}
	}
	return worst, where
}

// oneSpan is the corner as one quintic along it, where that keeps to its
// sections within the fillets' tolerance: its first and last rows the
// end sections, the next ones in from them along the derivatives s0 and
// s1 across the ends, and the middle two such that it passes through
// the sections at a third and two thirds of the way: one patch along
// the corner, rather than a span between each two sections.
func (pf *pivotFrame) oneSpan(s0, s1 []geom.Vector4, sectionAt func(float64) (srf.BSplineCurve, bool)) ([][]srf.SSurface, bool) {
	m1, ok1 := sectionAt(1.0 / 3)
	m2, ok2 := sectionAt(2.0 / 3)
	a, b := homog(pf.first), homog(pf.last)
	if !ok1 || !ok2 || len(s0) != len(a) || len(s1) != len(b) {
		return nil, false
	}
	sf := &srf.BSplineSurface{DegU: pf.first.Deg, DegV: spanDeg, KnotsU: slices.Clone(pf.first.Knots),
		KnotsV: []float64{0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1}}
	for _, row := range quinticRows(a, b, s0, s1, homog(m1), homog(m2)) {
		var ctrl []geom.Vector
		var w []float64
		for _, p := range row {
			if !(p.W > 0) {
				return nil, false
			}
			ctrl = append(ctrl, p.PerspectiveProject())
			w = append(w, p.W)
		}
		sf.Ctrl = append(sf.Ctrl, ctrl)
		sf.Weight = append(sf.Weight, w)
	}
	var ts []float64
	for j := 1; j < 16; j++ {
		ts = append(ts, float64(j)/16)
	}
	if worst, _ := strays(sf, ts, sectionAt); worst > pf.ra.tol {
		return nil, false
	}
	all, err := sf.Patches()
	if err != nil {
		return nil, false
	}
	return all, true
}

// spanDeg is the degree of a corner of one span along it.
const spanDeg = 5

// quinticRows are, for each control point of the sections, the six
// homogeneous points along the corner of the quintic from a to b with
// derivatives s0 and s1 at its ends that passes through h1 at a third of
// the way and h2 at two thirds: the middle two solve b2 P2 + b3 P3 = S(t)
// less the other four's part at both.
func quinticRows(a, b, s0, s1, h1, h2 []geom.Vector4) [][spanDeg + 1]geom.Vector4 {
	bern := func(k int, t float64) float64 {
		return binomial(spanDeg, k) * math.Pow(t, float64(k)) * math.Pow(1-t, float64(spanDeg-k))
	}
	t1, t2 := 1.0/3, 2.0/3
	a11, a12, a21, a22 := bern(2, t1), bern(3, t1), bern(2, t2), bern(3, t2)
	det := a11*a22 - a12*a21
	rows := make([][spanDeg + 1]geom.Vector4, len(a))
	for i := range a {
		p0, p5 := a[i], b[i]
		p1, p4 := p0.Plus(s0[i].ScaledBy(1.0/spanDeg)), p5.Minus(s1[i].ScaledBy(1.0/spanDeg))
		known := func(t float64) geom.Vector4 {
			return p0.ScaledBy(bern(0, t)).Plus(p1.ScaledBy(bern(1, t))).Plus(p4.ScaledBy(bern(4, t))).Plus(p5.ScaledBy(bern(5, t)))
		}
		r1, r2 := h1[i].Minus(known(t1)), h2[i].Minus(known(t2))
		p2 := r1.ScaledBy(a22 / det).Minus(r2.ScaledBy(a12 / det))
		p3 := r2.ScaledBy(a11 / det).Minus(r1.ScaledBy(a21 / det))
		rows[i] = [spanDeg + 1]geom.Vector4{p0, p1, p2, p3, p4, p5}
	}
	return rows
}

// pivotStep is the step in t the corner's own sections are differenced
// over at its ends.
const pivotStep = 1e-4

// ownDerivatives are the derivatives in t of the corner's own sections'
// homogeneous control points at its ends, one-sided, of second order.
func (pf *pivotFrame) ownDerivatives() ([]geom.Vector4, []geom.Vector4, error) {
	at := func(t float64) ([]geom.Vector4, error) {
		x, ok := pf.curve(t)
		if !ok {
			return nil, filletErr(pf.at(t).d, "the corner's sections can't be made like the fillets'")
		}
		return homog(x), nil
	}
	ends := [2][]geom.Vector4{}
	for k, t0 := range []float64{0, 1} {
		in := 1 - 2*float64(k)
		var hs [3][]geom.Vector4
		for i := range 3 {
			h, err := at(t0 + in*float64(i)*pivotStep)
			if err != nil {
				return nil, nil, err
			}
			hs[i] = h
		}
		d := make([]geom.Vector4, len(hs[0]))
		for i := range d {
			x := hs[0][i].ScaledBy(-3).Plus(hs[1][i].ScaledBy(4)).Minus(hs[2][i])
			d[i] = x.ScaledBy(1 / (2 * in * pivotStep))
		}
		ends[k] = d
	}
	return ends[0], ends[1], nil
}

// withHomog is x with its control points and weights h, in homogeneous
// coordinates.
func withHomog(x srf.BSplineCurve, h []geom.Vector4) srf.BSplineCurve {
	out := srf.BSplineCurve{Deg: x.Deg, Knots: slices.Clone(x.Knots)}
	for _, p := range h {
		out.Ctrl = append(out.Ctrl, p.PerspectiveProject())
		out.Weight = append(out.Weight, p.W)
	}
	return out
}

// arcAbout is the arc of radius rho about c from f0 to f1.
func arcAbout(c, f0, f1 geom.Vector, rho float64) srf.BSplineCurve {
	da, db := f0.Minus(c), f1.Minus(c)
	cosT := da.Dot(db) / (da.Magnitude() * db.Magnitude())
	theta := math.Acos(math.Max(-1, math.Min(1, cosT)))
	w := math.Cos(theta / 2)
	mid := c.Plus(da.Plus(db).WithMagnitude(rho / w))
	return srf.BSplineCurve{Deg: 2, Knots: []float64{0, 0, 0, 1, 1, 1},
		Ctrl: []geom.Vector{f0, mid, f1}, Weight: []float64{1, w, 1}}
}
