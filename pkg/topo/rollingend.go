package topo

import (
	"math"
	"sort"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// The ends of an open rolling-ball fillet (roadmap 22.2): where the
// chain of edges between the two curved faces meets a flat face, as a
// band's side on a cone ends at the band's end and at the base.
//
// The fillet ends in the flat face's plane. Its last section is where
// the surface the ball sweeps meets that plane: an oblique cut through a
// round tube, which is a conic, the ellipse a plane cuts from a
// cylinder where the edge runs straight. It is made as the rational
// quadratic from the points the fillet touches the two faces in the
// plane, its tangents there the faces' and the plane's meeting, through
// the point where the arcs' middles meet the plane: lying in the plane
// exactly, it is the flat face's new edge. Near the end the sections are
// such conics in planes that turn, as they go in along the edge, from
// the face's to square across the edge, slowly enough that they don't
// cross where the fillet is; further in they are the ball's arcs. The
// ball runs
// on past the edge's end outside the solid, on the faces' surfaces
// carried on past their patches' sides, as far as the oblique cut
// needs.
//
// Each side's face is cut back along its edge with the flat face to
// where the fillet touches it, and that piece of the edge goes from the
// flat face too; the flat face takes the conic instead. Where a side's
// face goes on past the flat face's plane, as a cone above a band's
// end, the fillet's end stands out of the flat face: the flat face
// grows by the fillet's section, up to the curve where that side's face
// meets the plane, from the vertex to the point the fillet touches,
// which both faces then share.

// rbEnd is the flat face an open chain ends at, at vertex v: its plane,
// its normal n pointing on past the end, and its edges from v with side
// k's face, a[k].
type rbEnd struct {
	v, f int
	n    geom.Vector
	d    float64
	a    [2]int
	face [2]int
}

// rbEndAt is the flat face the chain, through edge at vertex v between
// faces[0] and faces[1], ends at: the third face at v, whose edges with
// the two are the only others there.
func (t *Topology) rbEndAt(v int, faces [2]int, edge int) (*rbEnd, error) {
	var others []int
	for _, x := range t.Vertices[v].Edges {
		if x != edge {
			others = append(others, x)
		}
	}
	if len(others) != 2 {
		return nil, errNotRolling
	}
	end := &rbEnd{v: v, f: -1, a: [2]int{-1, -1}, face: faces}
	for k := 0; k < 2; k++ {
		for _, x := range others {
			fs := t.EdgeFaces(x)
			g := -1
			switch {
			case fs[0] == faces[k]:
				g = fs[1]
			case fs[1] == faces[k]:
				g = fs[0]
			}
			if g < 0 || g == faces[1-k] || end.f >= 0 && g != end.f {
				continue
			}
			end.f, end.a[k] = g, x
		}
		if end.a[k] < 0 {
			return nil, errNotRolling
		}
	}
	if end.a[0] == end.a[1] {
		return nil, errNotRolling
	}
	n, ok := t.planarNormal(end.f)
	if !ok {
		return nil, filletErr(t.Vertices[v].P, "the edge ends where it meets a curved face: a fillet between curved "+
			"faces ends where its edge meets a flat face")
	}
	end.n = n.WithMagnitude(1)
	end.d = end.n.Dot(t.Vertices[v].P)
	return end, nil
}

// rbSpec is one of the fillet's sections: the ball's at arc length s
// (end < 0), or the conic in the plane lam of the way from end end's
// flat face's to square across the edge.
type rbSpec struct {
	end int
	lam float64
	s   float64
}

// endDir is the way arc length runs on past end k: back at the start,
// on at the end.
func endDir(k int) float64 {
	if k == 0 {
		return -1
	}
	return 1
}

// planeRoot is the arc length, near s0, whose section's point g gives
// lies in a plane: g is the point's height over it, which rises the way
// dir runs.
func (rb *rollingBall) planeRoot(g func(rbSection) float64, s0, dir float64, at geom.Vector) (float64, rbSection, error) {
	step := rb.r / 4
	in, out := math.NaN(), math.NaN()
	sec, err := rb.section(s0)
	if err != nil {
		return 0, sec, err
	}
	if g(sec) > 0 {
		out = s0
	} else {
		in = s0
	}
	cur := s0
	for i := 0; math.IsNaN(in) || math.IsNaN(out); i++ {
		if i == 64 {
			return 0, sec, filletErr(at, "the fillet's end on the flat face its edge ends at can't be found")
		}
		if math.IsNaN(in) {
			cur -= dir * step
		} else {
			cur += dir * step
		}
		if sec, err = rb.section(cur); err != nil {
			return 0, sec, err
		}
		if g(sec) > 0 {
			out = cur
		} else {
			in = cur
		}
	}
	for it := 0; it < 100 && math.Abs(out-in) > 1e-13*math.Max(1, rb.length); it++ {
		m := (in + out) / 2
		if sec, err = rb.section(m); err != nil {
			return 0, sec, err
		}
		if g(sec) > 0 {
			out = m
		} else {
			in = m
		}
	}
	s := (in + out) / 2
	sec, err = rb.section(s)
	return s, sec, err
}

// endSection is the fillet's section lam of the way in from end k: at
// 0 the conic in the flat face's plane, through the points it touches
// the faces there; past 0 the conic in the plane through the points the
// ball touches each face and its arc's middle, each at an arc length
// moved that share of the way from where it lies in the flat face's
// plane to the first of the ball's own sections, whose arc lam 1 would
// give. Its arc length is the mean of the three, and the innermost of
// them comes back too.
func (rb *rollingBall) endSection(k int, lam float64) (rbSection, float64, error) {
	end := rb.ends[k]
	dir := endDir(k)
	s0 := 0.0
	if k == 1 {
		s0 = rb.length
	}
	at := rb.t.Vertices[end.v].P
	if lam != 0 {
		var ss [3]float64
		var secs [3]rbSection
		for i := range ss {
			ss[i] = (1-lam)*rb.endS[k][i] + lam*rb.target[k]
			sec, err := rb.section(ss[i])
			if err != nil {
				return rbSection{}, 0, err
			}
			secs[i] = sec
		}
		f0, f1 := secs[0].foot[0], secs[1].foot[1]
		m := secs[2].arc.PointAt(0.5)
		nrm := f1.p.Minus(f0.p).Cross(m.Minus(f0.p))
		if rb.flat {
			// A chamfer's three lie in its own plane at a straight edge:
			// the plane through the two points turned that share of the
			// way from the flat face's to square across the edge.
			_, tn := rb.along(ss[2])
			w := end.n.ScaledBy(1 - lam).Plus(tn.ScaledBy(dir * lam))
			e := f1.p.Minus(f0.p).WithMagnitude(1)
			nrm = w.Minus(e.ScaledBy(w.Dot(e)))
		}
		if nrm.Magnitude() < 1e-12 {
			return rbSection{}, 0, filletErr(m, "the fillet's sections near the flat face its edge ends at fold here")
		}
		nrm = nrm.WithMagnitude(1)
		if nrm.Dot(end.n) < 0 {
			nrm = nrm.ScaledBy(-1)
		}
		sec, err := rb.conicIn(nrm, nrm.Dot(f0.p), [2]rbFoot{f0, f1}, m)
		if err != nil {
			return sec, 0, err
		}
		sec.s = (ss[0] + ss[1] + ss[2]) / 3
		inner := math.Max(ss[0], math.Max(ss[1], ss[2]))
		if k == 1 {
			inner = math.Min(ss[0], math.Min(ss[1], ss[2]))
		}
		return sec, inner, nil
	}
	nrm, d := end.n, end.d
	var ss [3]float64
	var feet [2]rbFoot
	for i := 0; i < 2; i++ {
		s, sec, err := rb.planeRoot(func(x rbSection) float64 { return nrm.Dot(x.foot[i].p) - d }, s0, dir, at)
		if err != nil {
			return rbSection{}, 0, err
		}
		ss[i], feet[i] = s, sec.foot[i]
	}
	sm, msec, err := rb.planeRoot(func(x rbSection) float64 { return nrm.Dot(x.arc.PointAt(0.5)) - d }, s0, dir, at)
	if err != nil {
		return rbSection{}, 0, err
	}
	ss[2] = sm
	sec, err := rb.conicIn(nrm, d, feet, msec.arc.PointAt(0.5))
	if err != nil {
		return sec, 0, err
	}
	rb.endS[k] = ss
	sec.s = (ss[0] + ss[1] + ss[2]) / 3
	inner := math.Max(ss[0], math.Max(ss[1], ss[2]))
	if k == 1 {
		inner = math.Min(ss[0], math.Min(ss[1], ss[2]))
	}
	return sec, inner, nil
}

// conicIn is the section in the plane of normal nrm at height d: the
// conic from the points feet, put into the plane, its tangents there the
// plane's meeting with the faces', through m; for a chamfer, the
// quadratic through the three.
func (rb *rollingBall) conicIn(nrm geom.Vector, d float64, feet [2]rbFoot, m geom.Vector) (rbSection, error) {
	proj := func(p geom.Vector) geom.Vector { return p.Minus(nrm.ScaledBy(nrm.Dot(p) - d)) }
	q0, q1 := proj(feet[0].p), proj(feet[1].p)
	m = proj(m)
	feet[0].p, feet[1].p = q0, q1
	sec := rbSection{c: m, foot: feet}
	if rb.flat {
		// Near its ends a chamfer's rulings join the points where the
		// oblique planes meet the curves it touches the faces along.
		sec.arc = rb.arcOf(m, q0, q1)
		return sec, nil
	}
	// The tangents where the cut meets each face: in the plane and the
	// face's tangent plane, which is the fillet's there.
	t0, t1 := nrm.Cross(feet[0].n), nrm.Cross(feet[1].n)
	mid, ok := lineMeet(q0, t0, q1, t1)
	if !ok {
		return sec, filletErr(m, "the fillet's end on the flat face turns half round or more here")
	}
	mid = proj(mid)
	// The conic through m: m's weights against the control points give
	// the middle one's weight.
	e1, e2, y := q0.Minus(mid), q1.Minus(mid), m.Minus(mid)
	a11, a12, a22 := e1.Dot(e1), e1.Dot(e2), e2.Dot(e2)
	b1, b2 := e1.Dot(y), e2.Dot(y)
	det := a11*a22 - a12*a12
	alpha := (a22*b1 - a12*b2) / det
	gamma := (a11*b2 - a12*b1) / det
	beta := 1 - alpha - gamma
	if !(alpha > 0 && gamma > 0 && beta > 0) {
		return sec, filletErr(m, "the fillet's end on the flat face is not a simple arc here")
	}
	w := beta / (2 * math.Sqrt(alpha*gamma))
	sec.arc = srf.BSplineCurve{Deg: 2, Knots: []float64{0, 0, 0, 1, 1, 1},
		Ctrl: []geom.Vector{q0, mid, q1}, Weight: []float64{1, w, 1}}
	return sec, nil
}

// endDerivative is the derivative of the conics' homogeneous control
// points, per unit of arc length, as end k's plane turns in from the
// flat face's.
func (rb *rollingBall) endDerivative(k int) ([]geom.Vector4, error) {
	ramp := math.Abs(rb.target[k] - rb.endS[k][2])
	h := 1e-4 * math.Min(1, rb.r/math.Max(ramp, 1e-9))
	a, _, err := rb.endSection(k, -h)
	if err != nil {
		return nil, err
	}
	b, _, err := rb.endSection(k, h)
	if err != nil {
		return nil, err
	}
	ha, hb := homog(a.arc), homog(b.arc)
	out := make([]geom.Vector4, len(ha))
	for i := range ha {
		out[i] = hb[i].Minus(ha[i]).ScaledBy(1 / (b.s - a.s))
	}
	return out, nil
}

// blendOpen is blend for an open chain.
func (rb *rollingBall) blendOpen() (*srf.BSplineSurface, []rbSection, []rbCrossing, error) {
	last := len(rb.pts) - 1
	beyond := [2]geom.Vector{rb.tan[0].ScaledBy(-1), rb.tan[last]}
	var inner [2]float64
	var deriv [2][]geom.Vector4
	var at0 [2]rbSection
	for k, end := range rb.ends {
		if end.n.Dot(beyond[k]) < 0 {
			end.n, end.d = end.n.ScaledBy(-1), -end.d
		}
		if end.n.Dot(beyond[k]) < 0.05 {
			return nil, nil, nil, filletErr(rb.t.Vertices[end.v].P, "the edge meets the flat face it ends at almost "+
				"along it: a fillet between curved faces ends where its edge crosses a flat face")
		}
		sec, in, err := rb.endSection(k, 0)
		if err != nil {
			return nil, nil, nil, err
		}
		at0[k], inner[k] = sec, in
	}
	lo, hi := inner[0], inner[1]
	avail := hi - lo
	gap := math.Min(rb.r/8, avail/8)
	if !(avail > 0) || gap < 1e-6*rb.r {
		return nil, nil, nil, filletErr(midpoint(rb.pts), "the edge is too short for a fillet this large between "+
			"the faces it ends at")
	}
	// The planes turn from the flat face's to square over a length that
	// keeps them from crossing within a few radii of the edge, and takes
	// in the oblique cut's spread.
	var ramp [2]float64
	for k, end := range rb.ends {
		theta := math.Acos(math.Min(1, end.n.Dot(beyond[k])))
		ramp[k] = math.Min(4*rb.r*theta+gap, avail/3)
	}
	r0, r1 := lo+ramp[0], hi-ramp[1]
	rb.target = [2]float64{r0, r1}
	for k := range rb.ends {
		var err error
		if deriv[k], err = rb.endDerivative(k); err != nil {
			return nil, nil, nil, err
		}
	}
	if r1 < r0 {
		return nil, nil, nil, filletErr(midpoint(rb.pts), "the edge is too short for a fillet this large between "+
			"the faces it ends at")
	}
	// Where the points the ball touches cross seams, between the conics.
	n := int(math.Min(2000, math.Max(64, 4*(r1-r0)/rb.r)))
	scan := make([]rbSection, n+1)
	for i := range scan {
		sec, err := rb.section(r0 + (r1-r0)*float64(i)/float64(n))
		if err != nil {
			return nil, nil, nil, err
		}
		scan[i] = sec
	}
	at, pairs, sides, err := rb.crossings(scan)
	if err != nil {
		return nil, nil, nil, err
	}
	var cross []rbCrossing
	breaks := []float64{r0}
	for i, s := range at {
		cross = append(cross, rbCrossing{s: s, faces: pairs[i], side: sides[i]})
		breaks = append(breaks, s)
	}
	breaks = append(breaks, r1)
	sort.Float64s(breaks)
	h := math.Min(rb.length/12, 2*rb.r)
	var specs []rbSpec
	const steps = 4
	for i := 0; i < steps; i++ {
		specs = append(specs, rbSpec{end: 0, lam: float64(i) / steps})
	}
	for i := 0; i+1 < len(breaks); i++ {
		a, b := breaks[i], breaks[i+1]
		if b-a < 1e-6*rb.length {
			continue
		}
		m := int(math.Ceil((b - a) / h))
		for j := 0; j < m; j++ {
			specs = append(specs, rbSpec{end: -1, s: a + (b-a)*float64(j)/float64(m)})
		}
	}
	specs = append(specs, rbSpec{end: -1, s: r1})
	for i := steps - 1; i >= 0; i-- {
		specs = append(specs, rbSpec{end: 1, lam: float64(i) / steps})
	}
	cache := map[rbSpec]rbSection{}
	eval := func(sp rbSpec) (rbSection, error) {
		if sec, ok := cache[sp]; ok {
			return sec, nil
		}
		var sec rbSection
		var err error
		if sp.end < 0 {
			sec, err = rb.section(sp.s)
		} else {
			sec, _, err = rb.endSection(sp.end, sp.lam)
		}
		if err == nil {
			cache[sp] = sec
		}
		return sec, err
	}
	for round := 0; ; round++ {
		secs := make([]rbSection, len(specs))
		for i, sp := range specs {
			sec, err := eval(sp)
			if err != nil {
				return nil, nil, nil, err
			}
			secs[i] = sec
		}
		s0, s1 := secs[0].s, secs[len(secs)-1].s
		ts := make([]float64, len(secs))
		curves := make([]srf.BSplineCurve, len(secs))
		for i := range secs {
			ts[i] = (secs[i].s - s0) / (s1 - s0)
			curves[i] = secs[i].arc
			if i > 0 && !(ts[i] > ts[i-1]) {
				return nil, nil, nil, filletErr(secs[i].c, "the fillet's sections near the flat face it ends at "+
					"cross: the face meets the edge at too shallow an angle for this radius")
			}
		}
		ts[0], ts[len(ts)-1] = 0, 1
		d0, d1 := make([]geom.Vector4, 3), make([]geom.Vector4, 3)
		for i := range d0 {
			d0[i], d1[i] = deriv[0][i].ScaledBy(s1-s0), deriv[1][i].ScaledBy(s1-s0)
		}
		sf, err := srf.SkinThrough(curves, ts, d0, d1)
		if err != nil {
			return nil, nil, nil, err
		}
		var more []rbSpec
		worst, where := 0.0, geom.Vector{}
		for i := 0; i+1 < len(secs); i++ {
			a, b := specs[i], specs[i+1]
			sm := (secs[i].s + secs[i+1].s) / 2
			var e float64
			var w geom.Vector
			for _, u := range []float64{0.25, 0.5, 0.75} {
				su := secs[i].s + (secs[i+1].s-secs[i].s)*u
				eu, wu := rb.misfit(sf, ts[i]+(ts[i+1]-ts[i])*u, su, a.end >= 0 || b.end >= 0)
				if eu > e {
					e, w = eu, wu
				}
			}
			if e <= rb.tol {
				continue
			}
			if e > worst {
				worst, where = e, w
			}
			switch {
			case a.end < 0 && b.end < 0:
				more = append(more, rbSpec{end: -1, s: sm})
			case a.end >= 0 && b.end >= 0:
				more = append(more, rbSpec{end: a.end, lam: (a.lam + b.lam) / 2})
			default:
				pl, rg := a, b
				if b.end >= 0 {
					pl, rg = b, a
				}
				if pl.lam >= 1 {
					more = append(more, rbSpec{end: -1, s: (cache[pl].s + rg.s) / 2})
				} else {
					more = append(more, rbSpec{end: pl.end, lam: (pl.lam + 1) / 2})
				}
			}
		}
		if len(more) == 0 {
			return sf, secs, cross, nil
		}
		if round == 10 || len(specs)+len(more) > 600 {
			return nil, nil, nil, filletErr(where, "the fillet's surface can't be fitted to within %.3g mm here (%.3g)",
				rb.tol, worst)
		}
		specs = append(specs, more...)
		for _, sp := range more {
			if _, err := eval(sp); err != nil {
				return nil, nil, nil, err
			}
		}
		sort.SliceStable(specs, func(i, j int) bool { return cache[specs[i]].s < cache[specs[j]].s })
	}
}

// rbEndCut is where an open fillet's end meets the flat face: the end,
// the points it touches the two faces at, its edge there, and for a side
// whose face goes on past the plane, the curve from the vertex to that
// side's point.
type rbEndCut struct {
	end  *rbEnd
	q    [2]geom.Vector
	arc  srf.BSplineCurve
	goes [2][]srf.SBezier
	// off is where the end runs off a side's face onto the next face
	// (roadmap 22.3), or nil.
	off *rbRunOff
}

// cutEnd splits each side's edge with the flat face where the fillet's
// end section arc touches it; or, where it touches the side's face past
// that edge, finds the curve the plane cuts from the face up to it.
func (rb *rollingBall) cutEnd(sh *srf.SShell, end *rbEnd, arc srf.BSplineCurve) (rbEndCut, error) {
	t := rb.t
	cut := rbEndCut{end: end, q: [2]geom.Vector{arc.Ctrl[0], arc.Ctrl[len(arc.Ctrl)-1]}, arc: arc}
	v := t.Vertices[end.v].P
	for k := 0; k < 2; k++ {
		q := cut.q[k]
		switch alongFrom(q, v, t.Edges[end.a[k]].Points) {
		case 1:
			ro, err := rb.runOffAt(end, k)
			if err != nil || cut.off != nil {
				return cut, filletErr(q, "the fillet's end runs off the face on side %d here, past the end of its edge with "+
					"the flat face the edge ends at, and the face there is curved or meets the others otherwise: make it smaller",
					k+1)
			}
			cut.off = ro
			continue
		case -1:
			c, err := rb.planeCut(end, k, v, q)
			if err != nil {
				return cut, err
			}
			cut.goes[k] = c
			continue
		}
		if err := splitNearest(sh, q, v); err != nil {
			return cut, filletErr(q, "the fillet is wider than the face's edge with the flat face here: make it smaller")
		}
	}
	return cut, nil
}

// alongFrom says where q, which lies on the curve pts follow from its
// end v, lies: along it (0), on past v (-1), or on past its far end (1),
// by which point of the polyline is nearest q.
func alongFrom(q, v geom.Vector, pts []geom.Vector) int {
	best, bestD := geom.Vector{}, math.Inf(1)
	for i := 1; i < len(pts); i++ {
		a, b := pts[i-1], pts[i]
		ab := b.Minus(a)
		u := 0.0
		if l2 := ab.MagSquared(); l2 > 0 {
			u = math.Max(0, math.Min(1, q.Minus(a).Dot(ab)/l2))
		}
		p := a.Plus(ab.ScaledBy(u))
		if d := p.Minus(q).Magnitude(); d < bestD {
			best, bestD = p, d
		}
	}
	far := pts[0]
	if far.Minus(v).Magnitude() < pts[len(pts)-1].Minus(v).Magnitude() {
		far = pts[len(pts)-1]
	}
	switch {
	case best.Minus(v).Magnitude() <= 10*geom.LengthEps && q.Minus(v).Magnitude() > 10*geom.LengthEps:
		return -1
	case best.Minus(far).Magnitude() <= 10*geom.LengthEps && q.Minus(far).Magnitude() > 10*geom.LengthEps:
		return 1
	}
	return 0
}

// planeCut is the curve where side k's face meets end's plane, from a
// to b: cubics whose control points lie in the plane, each within a
// tenth of the chord tolerance of the face.
func (rb *rollingBall) planeCut(end *rbEnd, k int, a, b geom.Vector) ([]srf.SBezier, error) {
	proj := func(p geom.Vector) geom.Vector { return p.Minus(end.n.ScaledBy(end.n.Dot(p) - end.d)) }
	// The point of the curve near x, and the curve's way there.
	on := func(x geom.Vector) (geom.Vector, geom.Vector, bool) {
		var f rbFoot
		for it := 0; it < 100; it++ {
			f = rb.foot(k, x)
			y := proj(f.p)
			moved := y.Minus(x).Magnitude()
			x = y
			if moved < 1e-12*math.Max(1, rb.r) {
				break
			}
		}
		f = rb.foot(k, x)
		return x, end.n.Cross(f.n), f.p.Minus(x).Magnitude() < 1e-7*math.Max(1, rb.r)
	}
	chord := b.Minus(a)
	if chord.Magnitude() < geom.LengthEps {
		return nil, nil
	}
	tol := math.Max(srf.ChordTolMm()/10, 1e-6)
	for pieces := 1; pieces <= 64; pieces *= 2 {
		var out []srf.SBezier
		ok := true
		prev := a
		_, d0, _ := on(a)
		for i := 1; i <= pieces && ok; i++ {
			lam := float64(i) / float64(pieces)
			p := b
			var d1 geom.Vector
			if i < pieces {
				var good bool
				if p, d1, good = on(a.Plus(chord.ScaledBy(lam))); !good {
					return nil, filletErr(p, "the fillet's end can't follow where the face on side %d meets the flat face's "+
						"plane here", k+1)
				}
			} else {
				_, d1, _ = on(b)
			}
			l := p.Minus(prev).Magnitude()
			if d0.Dot(chord) < 0 {
				d0 = d0.ScaledBy(-1)
			}
			if d1.Dot(chord) < 0 {
				d1 = d1.ScaledBy(-1)
			}
			c1 := proj(prev.Plus(d0.WithMagnitude(l / 3)))
			c2 := proj(p.Minus(d1.WithMagnitude(l / 3)))
			sb := srf.SBezierFrom4(prev, c1, c2, p)
			for _, u := range []float64{0.25, 0.5, 0.75} {
				x := sb.PointAt(u)
				if rb.foot(k, x).p.Minus(x).Magnitude() > tol {
					ok = false
				}
			}
			out = append(out, sb)
			prev, d0 = p, d1
		}
		if ok {
			return out, nil
		}
	}
	return nil, filletErr(b, "the fillet's end can't follow where the face on side %d meets the flat face's plane here", k+1)
}

// closeEnd puts the fillet's end into the flat face: its edges from the
// end's vertex to the points cut go, the conic between them comes in,
// and a side's curve in the plane, where its face goes on past it, comes
// into both that face and the flat one.
func (rb *rollingBall) closeEnd(sh *srf.SShell, cut rbEndCut, h *core.HSCurve) error {
	t := rb.t
	v := t.Vertices[cut.end.v].P
	fs := sh.FindSurface(t.Faces[cut.end.f].Surface)
	if fs == nil {
		return filletErr(v, "the flat face the edge ends at is not in the shell")
	}
	// The flat face's loop runs from side 0 to the vertex, and on to side
	// 1: then the fillet's end goes from side 0's point to side 1's.
	edgeCurve := func(k int) map[core.HSCurve]bool {
		ed := &t.Edges[cut.end.a[k]]
		m := map[core.HSCurve]bool{ed.Curve: true}
		for _, ce := range ed.CoEdges {
			m[t.CoEdges[ce].Curve] = true
		}
		return m
	}
	forward, seen := true, false
	var kept []srf.STrimBy
	dropped := 0
	for _, tb := range fs.Trim {
		gone := false
		for k := 0; k < 2; k++ {
			q := cut.q[k]
			isDrop := cut.goes[k] == nil &&
				(tb.StartP.Equals(q) && tb.FinishP.Equals(v) || tb.StartP.Equals(v) && tb.FinishP.Equals(q))
			if isDrop {
				gone = true
				dropped++
			}
			if k == 0 && (isDrop || cut.goes[0] != nil && edgeCurve(0)[tb.Curve] &&
				(tb.StartP.Equals(v) || tb.FinishP.Equals(v))) {
				forward, seen = tb.FinishP.Equals(v), true
			}
		}
		if !gone {
			kept = append(kept, tb)
		}
	}
	want := 0
	for k := 0; k < 2; k++ {
		if cut.goes[k] == nil {
			want++
		}
	}
	if !seen || dropped != want {
		return filletErr(v, "the flat face the edge ends at doesn't run along both faces' edges here")
	}
	add := func(s *srf.SSurface, other core.HSSurface, sb srf.SBezier, fwd bool) {
		var pts []geom.Vector
		sb.MakePwlIntoPoints(&pts, 0, 0)
		pts[0], pts[len(pts)-1] = sb.Start(), sb.Finish()
		nc := srf.SCurve{H: *h, Exact: sb, IsExact: true, Deg: sb.Degree, SurfA: s.H, SurfB: other}
		h.V++
		for _, p := range pts {
			nc.Points = append(nc.Points, srf.SCurvePt{Vertex: true, P: p})
		}
		tb := srf.STrimBy{Curve: nc.H, StartP: sb.Start(), FinishP: sb.Finish(), Start: 0, Finish: 1}
		if !fwd {
			tb.Backwards, tb.StartP, tb.FinishP = true, sb.Finish(), sb.Start()
		}
		s.Trim = append(s.Trim, tb)
		sh.Curve = append(sh.Curve, nc)
	}
	bz, err := cut.arc.Beziers()
	if err != nil || len(bz) != 1 {
		return filletErr(v, "the fillet's end is not one curve")
	}
	fs.Trim = kept
	add(fs, core.HSSurface{}, bz[0], forward)
	// A side's curve in the plane, from the vertex to its point: in the
	// flat face's loop, from the vertex out for side 0 when it runs on,
	// into it for side 1; in the side's face the other way round from
	// its own edge with the flat face.
	for k := 0; k < 2; k++ {
		if cut.goes[k] == nil {
			continue
		}
		ss := sh.FindSurface(t.Faces[cut.end.face[k]].Surface)
		if ss == nil {
			return filletErr(v, "a face at the edge is not in the shell")
		}
		inSide := false
		found := false
		for _, tb := range ss.Trim {
			if edgeCurve(k)[tb.Curve] && (tb.StartP.Equals(v) || tb.FinishP.Equals(v)) {
				// The side's edge with the flat face leaves the vertex: the
				// curve comes into it.
				inSide, found = tb.StartP.Equals(v), true
			}
		}
		if !found {
			return filletErr(v, "the face on side %d doesn't meet the flat face at the edge's end", k+1)
		}
		fwdFlat := forward == (k == 0)
		for _, sb := range cut.goes[k] {
			// The pieces run from the vertex to the point.
			hFlat := *h
			add(fs, ss.H, sb, fwdFlat)
			// The side's face runs the same curve the other way: share it.
			tb := srf.STrimBy{Curve: hFlat, StartP: sb.Start(), FinishP: sb.Finish(), Start: 0, Finish: 1}
			if inSide {
				tb.Backwards, tb.StartP, tb.FinishP = true, sb.Finish(), sb.Start()
			}
			ss.Trim = append(ss.Trim, tb)
		}
		forget(ss)
	}
	forget(fs)
	return nil
}
