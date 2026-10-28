package topo

import (
	"cmp"
	"math"
	"slices"

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

// rbEnd is the face an open chain ends at, at vertex v: its plane, its
// normal n pointing on past the end, and its edges from v with side k's
// face, a[k]. A curved end face (curved) has n its normal at v, and
// sign turns its surface's normal to point on past the end.
type rbEnd struct {
	v, f   int
	n      geom.Vector
	d      float64
	a      [2]int
	face   [2]int
	curved bool
	sign   float64
	// mitre marks an end that meets another end across a crease, the
	// chain's own other end or one of another chain (rollingmitre.go):
	// mate's end mateK. ban is, on the side whose face is creased there,
	// the face across the crease, which the ball isn't taken onto near
	// it, and -1 on the side whose face both chains share.
	mitre bool
	ban   [2]int
	mate  *rollingBall
	mateK int
	// shared is the mitre, the last section of both ends, once one of
	// them is fitted: the other copies it.
	shared *rbSection
	// kink is, at the larger end of a mitre of two radii, the arc
	// parameter of the ball's arcs where its mitre turns from the mate's
	// swept surface onto the mate's wall, end face f, whose normal
	// wallSign turns to point on past the end (rollingkink.go).
	kink     float64
	wallSign float64
	// fade marks an end where the crease fades out part way along the
	// edge, at p: the faces turn less and less, and the fillet narrows to
	// nothing. It ends there with no face of its own, its last section
	// closed by the two short lines from its feet to p.
	fade bool
	p    geom.Vector
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
	end := &rbEnd{v: v, f: -1, a: [2]int{-1, -1}, face: faces, ban: [2]int{-1, -1}}
	for k := range 2 {
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
		// A curved end face: the fillet ends where the surface the ball
		// sweeps meets it (curvedEnd).
		s := t.Surface(end.f)
		if s == nil {
			return nil, errNotRolling
		}
		_, _, p := nearestOnWithin(s, t.Vertices[v].P, 0)
		var u, w float64
		s.ClosestPointToUV(p, &u, &w, false)
		if n = s.NormalAt(u, w); n.Magnitude() == 0 {
			return nil, filletErr(t.Vertices[v].P, "the curved face the edge ends at has no normal here")
		}
		end.curved, end.sign = true, 1
	}
	end.n = n.WithMagnitude(1)
	end.d = end.n.Dot(t.Vertices[v].P)
	return end, nil
}

// endHeight is how far p lies on past end's face, along its normal at
// the point of it nearest p: for a flat face its height over the plane.
func (rb *rollingBall) endHeight(end *rbEnd, p geom.Vector) float64 {
	if end.mitre {
		return rb.mitreHeight(slices.Index(rb.ends[:], end), p)
	}
	if !end.curved {
		return end.n.Dot(p) - end.d
	}
	s := rb.t.Surface(end.f)
	u, v, q := nearestOnWithin(s, p, 0.5)
	n := s.NormalAt(u, v)
	if n.Magnitude() == 0 {
		return end.n.Dot(p.Minus(q))
	}
	return end.sign * n.WithMagnitude(1).Dot(p.Minus(q))
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
		ref := end.n
		if end.mitre {
			// The plane that halves a mitre's turn may lean against the
			// chain's way a few radii in, where the edge curves: past the
			// end is on along the chain there.
			_, tn := rb.along(ss[2])
			ref = tn.ScaledBy(dir)
		}
		if nrm.Dot(ref) < 0 {
			nrm = nrm.ScaledBy(-1)
		}
		sec, err := rb.conicIn(nrm, nrm.Dot(f0.p), [2]rbFoot{f0, f1}, m)
		if err != nil {
			return sec, 0, err
		}
		sec.s = (ss[0] + ss[1] + ss[2]) / 3
		sec.footS = [2]float64{ss[0], ss[1]}
		inner := math.Max(ss[0], math.Max(ss[1], ss[2]))
		if k == 1 {
			inner = math.Min(ss[0], math.Min(ss[1], ss[2]))
		}
		return sec, inner, nil
	}
	nrm, d := end.n, end.d
	var ss [3]float64
	var feet [2]rbFoot
	for i := range 2 {
		var s float64
		var sec rbSection
		var err error
		switch {
		case rb.kinkedEnd(k) && end.ban[i] >= 0:
			// The larger end's foot on its creased side is where it
			// crosses the mate's wall: at the crease.
			s, sec, err = rb.wallRoot(k, func(x rbSection) float64 { return rb.wallHeight(k, x.foot[i].p) })
		case end.mitre:
			s, sec, err = rb.mitreFoot(k, i)
		default:
			s, sec, err = rb.planeRoot(func(x rbSection) float64 { return rb.endHeight(end, x.foot[i].p) }, s0, dir, at)
		}
		if err != nil {
			return rbSection{}, 0, err
		}
		ss[i], feet[i] = s, sec.foot[i]
	}
	mid := func(x rbSection) float64 { return rb.endHeight(end, x.arc.PointAt(0.5)) }
	var sm float64
	var msec rbSection
	var err error
	switch {
	case rb.kinkedEnd(k):
		sm, msec, err = rb.kinkRoot(k, 0.5)
	case end.mitre:
		sm, msec, err = rb.mitreRoot(k, mid)
	default:
		sm, msec, err = rb.planeRoot(mid, s0, dir, at)
	}
	if err != nil {
		return rbSection{}, 0, err
	}
	ss[2] = sm
	if rb.kinkedEnd(k) {
		// The kinked mitre is fitted through its cuts in two spans, as
		// the sections near it are.
		rb.endS[k] = ss
		return rb.mitreSectionWithin(k, 0, 1)
	}
	m := msec.arc.PointAt(0.5)
	if end.curved {
		// On a curved face the three points lie on its surface, and the
		// end is the conic in the plane through them, as the sections
		// further in are: it must keep to the face within the tolerance.
		nrm = feet[1].p.Minus(feet[0].p).Cross(m.Minus(feet[0].p))
		if rb.flat {
			// A chamfer's three lie on a line: the plane through it nearest
			// the face's.
			e := feet[1].p.Minus(feet[0].p).WithMagnitude(1)
			nrm = end.n.Minus(e.ScaledBy(end.n.Dot(e)))
		}
		if nrm.Magnitude() < 1e-12 {
			return rbSection{}, 0, filletErr(m, "the fillet's end on the curved face its edge ends at folds here")
		}
		if nrm = nrm.WithMagnitude(1); nrm.Dot(end.n) < 0 {
			nrm = nrm.ScaledBy(-1)
		}
		d = nrm.Dot(feet[0].p)
	}
	sec, err := rb.conicIn(nrm, d, feet, m)
	if err != nil {
		return sec, 0, err
	}
	rb.endS[k] = ss
	sec.s = (ss[0] + ss[1] + ss[2]) / 3
	sec.footS = [2]float64{ss[0], ss[1]}
	if end.mitre {
		// As the sections near it are reckoned (mitreSection).
		ms, err := rb.mitreRoots(k)
		if err != nil {
			return sec, 0, err
		}
		sum := 0.0
		for _, x := range ms {
			sum += x
		}
		sec.s = sum / float64(len(ms))
	}
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
	sec := rbSection{c: m, foot: feet, cutN: nrm, cutD: d}
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
	sect := rb.endSection
	if rb.ends[k].mitre {
		// Fitted sections: a step the fits' last digits don't swamp.
		h, sect = mitreStep, rb.mitreDerivative
	}
	a, _, err := sect(k, -h)
	if err != nil {
		return nil, err
	}
	b, _, err := sect(k, h)
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

// fadeDerivative is the derivative along the edge, in homogeneous
// coordinates, of the ball's sections at end k where the crease fades
// out (rbEnd.fade), one-sided, from sections in from it.
func (rb *rollingBall) fadeDerivative(k int) ([]geom.Vector4, error) {
	s, in := 0.0, 1.0
	if k == 1 {
		s, in = rb.length, -1
	}
	h := math.Min(rb.r/16, rb.length/256)
	var hs [3][]geom.Vector4
	for i := range 3 {
		sec, err := rb.section(s + in*float64(i)*h)
		if err != nil {
			return nil, err
		}
		hs[i] = homog(sec.arc)
	}
	out := make([]geom.Vector4, len(hs[0]))
	for i := range out {
		// (f(s+2h) - 4 f(s+h) + 3 f(s)) / -2h, with h signed inwards.
		d := hs[0][i].ScaledBy(-3).Plus(hs[1][i].ScaledBy(4)).Minus(hs[2][i])
		out[i] = d.ScaledBy(1 / (2 * in * h))
	}
	return out, nil
}

// blendOpen is blend for an open chain.
func (rb *rollingBall) blendOpen() (*srf.BSplineSurface, []rbSection, []rbCrossing, error) {
	if rb.mitred() && rb.ends[0].mate == rb {
		// Its own ends meet: the mitre is fitted again with the sections.
		rb.ends[0].shared, rb.ends[1].shared = nil, nil
	}
	last := len(rb.pts) - 1
	beyond := [2]geom.Vector{rb.tan[0].ScaledBy(-1), rb.tan[last]}
	var inner [2]float64
	var deriv [2][]geom.Vector4
	var at0 [2]rbSection
	for k, end := range rb.ends {
		if end.fade {
			// No face: the ball's own sections run to the end.
			inner[k] = rb.length * float64(k)
			continue
		}
		if end.n.Dot(beyond[k]) < 0 {
			end.n, end.d, end.sign = end.n.ScaledBy(-1), -end.d, -end.sign
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
		if end.fade {
			continue
		}
		theta := math.Acos(math.Min(1, end.n.Dot(beyond[k])))
		ramp[k] = math.Min(4*rb.r*theta+gap, avail/3)
	}
	r0, r1 := lo+ramp[0], hi-ramp[1]
	rb.target = [2]float64{r0, r1}
	quartic := rb.anyCurvedEnd() || rb.fitCuts
	for k := range rb.ends {
		var err error
		if rb.ends[k].fade {
			deriv[k], err = rb.fadeDerivative(k)
		} else {
			deriv[k], err = rb.endDerivative(k)
		}
		if err != nil {
			return nil, nil, nil, err
		}
		if quartic {
			deriv[k] = elevateH(deriv[k])
		}
		if len(deriv[k]) == endDeg+1 {
			deriv[k] = rb.splitDerivative(deriv[k])
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
	slices.Sort(breaks)
	h := math.Min(rb.length/12, 2*rb.r)
	var specs []rbSpec
	const steps = 4
	for i := range steps {
		if rb.ends[0].fade {
			break
		}
		specs = append(specs, rbSpec{end: 0, lam: float64(i) / steps})
	}
	for i := 0; i+1 < len(breaks); i++ {
		a, b := breaks[i], breaks[i+1]
		if b-a < 1e-6*rb.length {
			continue
		}
		m := int(math.Ceil((b - a) / h))
		for j := range m {
			specs = append(specs, rbSpec{end: -1, s: a + (b-a)*float64(j)/float64(m)})
		}
	}
	specs = append(specs, rbSpec{end: -1, s: r1})
	for i := steps - 1; i >= 0; i-- {
		if rb.ends[1].fade {
			break
		}
		specs = append(specs, rbSpec{end: 1, lam: float64(i) / steps})
	}
	cache := map[rbSpec]rbSection{}
	eval := func(sp rbSpec) (rbSection, error) {
		if sec, ok := cache[sp]; ok {
			return sec, nil
		}
		var sec rbSection
		var err error
		switch {
		case sp.end < 0:
			sec, err = rb.section(sp.s)
		case sp.lam != 0 && rb.ends[sp.end].mitre:
			// Already of the sections' degree and spans.
			sec, _, err = rb.mitreSection(sp.end, sp.lam)
			if err == nil {
				cache[sp] = sec
			}
			return sec, err
		default:
			sec, _, err = rb.endSection(sp.end, sp.lam)
		}
		if err == nil && quartic {
			switch {
			case sp.end >= 0 && sp.lam == 0 && rb.kinkedEnd(sp.end):
				// Fitted already, in two spans, before any chain was blended:
				// the smaller end copies its first.
				if sh := rb.ends[sp.end].shared; sh != nil {
					sec.arc, sec.foot = sh.arc, sh.foot
				} else {
					shared := sec
					rb.ends[sp.end].shared = &shared
				}
			case sp.end >= 0 && sp.lam == 0 && rb.ends[sp.end].mitre && rb.mateShared(sp.end) != nil:
				// The mitre is both ends' last section, fitted by the end
				// that came first.
				sec = rb.fromMate(sp.end, sec)
			case sp.end >= 0 && sp.lam == 0 && rb.ends[sp.end].mitre:
				if sec, err = rb.curvedEndSection(sp.end, sec); err == nil {
					shared := sec
					rb.ends[sp.end].shared = &shared
				}
			case sp.end >= 0 && sp.lam == 0 && rb.ends[sp.end].curved:
				sec, err = rb.curvedEndSection(sp.end, sec)
			case sp.end >= 0 && rb.fitCuts:
				sec, err = rb.planeCutSection(sp.end, sec)
			default:
				sec.arc = rb.splitSection(raised(sec.arc))
			}
		}
		if err == nil {
			cache[sp] = sec
		}
		return sec, err
	}
	prevWorst := math.Inf(1)
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
		d0, d1 := make([]geom.Vector4, len(deriv[0])), make([]geom.Vector4, len(deriv[1]))
		for i := range d0 {
			d0[i], d1[i] = deriv[0][i].ScaledBy(s1-s0), deriv[1][i].ScaledBy(s1-s0)
		}
		sf, err := srf.SkinThrough(curves, ts, d0, d1)
		if err != nil {
			return nil, nil, nil, err
		}
		var more []rbSpec
		worst, where := 0.0, geom.Vector{}
		endWorst := false
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
				endWorst = a.end >= 0 && b.end >= 0
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
		// Between conics the misfit that halving doesn't shrink is the
		// conics' own (blend's fitCuts): no use going on.
		stalled := !rb.fitCuts && round >= 2 && endWorst && worst > 0.9*prevWorst
		prevWorst = worst
		if stalled || round == 10 || len(specs)+len(more) > 600 {
			return nil, nil, nil, filletErr(where, "the fillet's surface can't be fitted to within %.3g mm here (%.3g)",
				rb.tol, worst)
		}
		specs = append(specs, more...)
		for _, sp := range more {
			if _, err := eval(sp); err != nil {
				return nil, nil, nil, err
			}
		}
		slices.SortStableFunc(specs, func(a, b rbSpec) int { return cmp.Compare(cache[a].s, cache[b].s) })
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
	for k := range 2 {
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
			if end.mitre {
				return cut, filletErr(q, "the face on side %d goes on past the crease the edge ends at here, "+
					"and the fillet's end would stand out of it", k+1)
			}
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
// tenth of the chord tolerance of the face. On a curved end face it is
// where side k's face meets the end's surface carried on past the face,
// which grows over what lies between it and the fillet's end; the
// cubics' control points then lie on that surface, and the curve within
// the tolerance of it too.
func (rb *rollingBall) planeCut(end *rbEnd, k int, a, b geom.Vector) ([]srf.SBezier, error) {
	// near is the end's point nearest p and its normal there.
	near := func(p geom.Vector) (geom.Vector, geom.Vector) {
		return p.Minus(end.n.ScaledBy(end.n.Dot(p) - end.d)), end.n
	}
	if end.curved {
		s := rb.t.Surface(end.f)
		near = func(p geom.Vector) (geom.Vector, geom.Vector) {
			u, v, q := nearestOnWithin(s, p, runOffExt)
			n := s.NormalAt(u, v)
			if n.Magnitude() == 0 {
				return q, end.n
			}
			return q, n.WithMagnitude(1)
		}
	}
	proj := func(p geom.Vector) geom.Vector {
		q, _ := near(p)
		return q
	}
	// The point of the curve near x, and the curve's way there.
	on := func(x geom.Vector) (geom.Vector, geom.Vector, bool) {
		var f rbFoot
		for range 100 {
			f = rb.foot(k, x)
			y := proj(f.p)
			moved := y.Minus(x).Magnitude()
			x = y
			if moved < 1e-12*math.Max(1, rb.r) {
				break
			}
		}
		f = rb.foot(k, x)
		_, n := near(x)
		return x, n.Cross(f.n), f.p.Minus(x).Magnitude() < 1e-7*math.Max(1, rb.r)
	}
	chord := b.Minus(a)
	if chord.Magnitude() < geom.LengthEps {
		return nil, nil
	}
	tol := math.Max(rb.t.shell.Tol.ChordTolMm()/10, 1e-6)
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
				if rb.foot(k, x).p.Minus(x).Magnitude() > tol || proj(x).Minus(x).Magnitude() > tol {
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
		for k := range 2 {
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
	for k := range 2 {
		if cut.goes[k] == nil {
			want++
		}
	}
	if !seen || dropped != want {
		return filletErr(v, "the flat face the edge ends at doesn't run along both faces' edges here")
	}
	add := func(s *srf.SSurface, other core.HSSurface, sb srf.SBezier, fwd bool) {
		var pts []geom.Vector
		sb.MakePwlIntoPoints(&pts, rb.t.shell.Tol, 0)
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
	if err != nil || len(bz) == 0 {
		return filletErr(v, "the fillet's end is not a curve")
	}
	fs.Trim = kept
	if cut.end.curved && (cut.goes[0] != nil || cut.goes[1] != nil) {
		// A curved end face grows past its edge with a side's face that
		// goes on past it: its surface is carried on to hold the fillet's
		// end and the side's curve on it.
		var pts []geom.Vector
		for _, b := range bz {
			pts = append(pts, b.Start(), b.PointAt(0.5), b.Finish())
		}
		for k := range 2 {
			for _, sb := range cut.goes[k] {
				pts = append(pts, sb.PointAt(0.5), sb.Finish())
			}
		}
		growSurface(fs, pts)
	}
	for _, b := range bz {
		add(fs, core.HSSurface{}, b, forward)
	}
	// A side's curve in the plane, from the vertex to its point: in the
	// flat face's loop, from the vertex out for side 0 when it runs on,
	// into it for side 1; in the side's face the other way round from
	// its own edge with the flat face.
	for k := range 2 {
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

// The end of a fillet on a curved face (roadmap 22.5) is not planar, as
// the curve the face's surface cuts from the surface the ball sweeps
// bows out of any plane, and a conic can only keep to the face as far as
// the face is flat across the fillet. So where an end is curved every
// section is raised to degree 5, which keeps the conics' shapes exactly,
// and that end's section is the quintic fitted to where the swept
// surface meets the face: its ends where the ball touches the faces
// there, its tangents there along the face and each side's, and its
// middle points on the swept surface and the face within the fit's
// tolerance.

// endDeg is the degree of the sections of a fillet with a curved end.
const endDeg = 5

// anyCurvedEnd reports whether an end of the chain is on a curved face.
func (rb *rollingBall) anyCurvedEnd() bool {
	for _, end := range rb.ends {
		if end != nil && end.curved {
			return true
		}
	}
	return false
}

// elevateH raises a quadratic's homogeneous control points, or their
// derivatives, to degree endDeg.
func elevateH(h []geom.Vector4) []geom.Vector4 {
	for deg := len(h) - 1; deg < endDeg; deg++ {
		out := make([]geom.Vector4, deg+2)
		out[0], out[deg+1] = h[0], h[deg]
		for i := 1; i <= deg; i++ {
			a := float64(i) / float64(deg+1)
			out[i] = h[i-1].ScaledBy(a).Plus(h[i].ScaledBy(1 - a))
		}
		h = out
	}
	return h
}

// fromHomog is the Bezier curve of homogeneous control points h.
func fromHomog(h []geom.Vector4) srf.BSplineCurve {
	n := len(h) - 1
	c := srf.BSplineCurve{Deg: n}
	for i := 0; i <= n; i++ {
		c.Knots = append(c.Knots, 0)
	}
	for i := 0; i <= n; i++ {
		c.Knots = append(c.Knots, 1)
	}
	for _, x := range h {
		c.Ctrl = append(c.Ctrl, x.PerspectiveProject())
		c.Weight = append(c.Weight, x.W)
	}
	return c
}

// raised is a section's arc at degree endDeg.
func raised(c srf.BSplineCurve) srf.BSplineCurve {
	if c.Deg >= endDeg {
		return c
	}
	return fromHomog(elevateH(homog(c)))
}

// bezierKnots are the knots of one Bézier span of degree deg on [0, 1].
func bezierKnots(deg int) []float64 {
	k := make([]float64, 2*(deg+1))
	for i := deg + 1; i < len(k); i++ {
		k[i] = 1
	}
	return k
}

// splitSection is c, a section of degree endDeg, split at its middle
// into two spans when the fillet's sections are (splitEnds): the same
// curve, so that it skins with the end sections fitted in two.
func (rb *rollingBall) splitSection(c srf.BSplineCurve) srf.BSplineCurve {
	if len(c.Ctrl) != endDeg+1 {
		return c
	}
	if len(rb.kinks) > 0 {
		for _, t := range rb.kinks {
			for range endDeg {
				c = c.WithKnot(t)
			}
		}
		return c
	}
	if !rb.splitEnds || rb.copiesKink() {
		return c
	}
	return c.WithKnot(0.5)
}

// splitDerivative is d, the derivative across an end of one span's
// homogeneous control points, split as splitSection splits a section.
func (rb *rollingBall) splitDerivative(d []geom.Vector4) []geom.Vector4 {
	knots := bezierKnots(endDeg)
	if len(rb.kinks) > 0 {
		for _, t := range rb.kinks {
			for range endDeg {
				knots, d = srf.InsertKnotH(endDeg, knots, d, t)
			}
		}
		return d
	}
	if !rb.splitEnds || rb.copiesKink() {
		return d
	}
	_, d = srf.InsertKnotH(endDeg, knots, d, 0.5)
	return d
}

// curvedEndSection is end k's section on its curved face: sec, the
// conic through where the swept surface meets the face at the two sides
// and the middle, refitted as a quintic of the conic's raised weights.
func (rb *rollingBall) curvedEndSection(k int, sec rbSection) (rbSection, error) {
	end := rb.ends[k]
	s := rb.t.Surface(end.f)
	if rb.flat {
		// A chamfer meets the sides' faces at an angle, not tangent: the
		// curve's way at its ends is along the face and the chamfer's own
		// surface, through its ruling and the curve each side's foot
		// runs along.
		sec.foot = rb.chamferFootNormals(sec)
	}
	// The curve's way at its ends: along the face and the side's.
	tanAt := func(q, n geom.Vector) geom.Vector {
		if end.mitre {
			// Where the mitre meets the crease the two swept surfaces cross,
			// and their normals give its way. They are tangent where it
			// meets the face they share, and give none: the plane of the
			// three points the conic went through does there.
			if t := rb.mitreNormal(k, q).Cross(n); t.Magnitude() > mitreCrossing {
				return t
			}
			return sec.cutN.Cross(n)
		}
		u, v, _ := nearestOnWithin(s, q, 0.5)
		return s.NormalAt(u, v).Cross(n)
	}
	return rb.fitCut(k, sec, end.mitre, tanAt, func(p geom.Vector) float64 { return rb.endHeight(end, p) },
		"the fillet's end on the curved face its edge ends at",
		"the fillet's end strays %.3g mm from the curved face its edge ends at here: make it smaller")
}

// planeCutSection is end k's section sec, a conic in its plane, refitted
// as a quintic to where the swept surface meets the plane, its ends and
// their ways there the conic's: along a curved edge the surface is no
// tube, and the conic only near the cut.
func (rb *rollingBall) planeCutSection(k int, sec rbSection) (rbSection, error) {
	nrm, d := sec.cutN, sec.cutD
	if rb.flat {
		// The cut's way at a foot is along the chamfer, not the face.
		sec.foot = rb.chamferFootNormals(sec)
	}
	return rb.fitCut(k, sec, false, func(_, n geom.Vector) geom.Vector { return nrm.Cross(n) },
		func(p geom.Vector) float64 { return nrm.Dot(p) - d }, "the fillet's end section near the face its edge ends at",
		"the fillet's end section near the face its edge ends at strays %.3g mm from its plane here: make it smaller")
}

// fitCut is sec refitted as a quintic of the conic's raised weights to
// where the swept surface meets the cut, height's zero: its ends the
// conic's, its ways there along tanAt's (from an end and the side's
// face's normal there), and its middle points on the swept surface and
// the cut within the fit's tolerance. what names the section in errors,
// and strays says, of a height, that it strays from the cut. A mitre's
// section (mitre) finds its points by mitreRoot, and its end on the face
// both sides share may turn anywhere in that face.
func (rb *rollingBall) fitCut(k int, sec rbSection, mitre bool, tanAt func(q, n geom.Vector) geom.Vector,
	height func(geom.Vector) float64, what, strays string) (rbSection, error) {
	at := sec.arc.PointAt(0.5)
	c := rb.splitSection(raised(sec.arc))
	// The ball's own arcs' weights, at the ball's own parameters: where
	// the sides' faces are flat the swept surface is a tube, its arcs
	// circles, and only the height along it is left to fit.
	ball, err := rb.section(sec.s)
	if err != nil {
		return sec, err
	}
	w := rb.splitSection(raised(ball.arc)).Weight
	c.Weight = w
	// The last control point: endDeg, or one more for two spans.
	n := len(c.Ctrl) - 1
	q0, q1 := c.Ctrl[0], c.Ctrl[n]
	way := func(q, n, toward geom.Vector) geom.Vector {
		t := tanAt(q, n)
		if t.Magnitude() == 0 {
			return toward.Minus(q)
		}
		if t.Dot(toward.Minus(q)) < 0 {
			t = t.ScaledBy(-1)
		}
		return t.WithMagnitude(1)
	}
	t0 := way(q0, sec.foot[0].n, c.Ctrl[1])
	t1 := way(q1, sec.foot[1].n, c.Ctrl[n-1])
	// The ways the second and last but one control points may move from
	// the ends: along the tangent, or, at a mitre's end on the face both
	// sides share, where the two swept surfaces are tangent and give no
	// way, anywhere in the face's tangent plane (mitreFreeEnd).
	ways := [2][]geom.Vector{{t0}, {t1}}
	for i := range 2 {
		if mitre && rb.ends[k].ban[i] < 0 && !rb.flat {
			nf := sec.foot[i].n
			ways[i] = []geom.Vector{nf.NormalDir(0), nf.NormalDir(1)}
		}
	}
	// Where the swept surface meets the cut, along the ball's arcs.
	dir := endDir(k)
	nPts := 9
	if n > endDeg {
		nPts = 17
	}
	var ts []float64
	var xs []geom.Vector
	skipped := 0
	for j := 1; j < nPts; j++ {
		tj := float64(j) / float64(nPts)
		g := func(x rbSection) float64 { return height(x.arc.PointAt(tj)) }
		var xsec rbSection
		if mitre {
			_, xsec, err = rb.mitreRoot(k, g)
		} else {
			_, xsec, err = rb.planeRoot(g, sec.s, dir, at)
		}
		if err != nil {
			// An oblique cut may meet the swept surface at a point near a
			// side far along the chain: up to two such points are left
			// out, the curve held there by its ends and checked against
			// the cut below.
			if skipped++; skipped > maxSkippedCuts {
				return sec, err
			}
			continue
		}
		ts = append(ts, tj)
		xs = append(xs, xsec.arc.PointAt(tj))
	}
	fit, us, ok := fitThrough(c, w, ways, ts, xs)
	if !ok {
		return sec, filletErr(at, "%s can't be fitted here", what)
	}
	// The curve is both faces' edge: it keeps to them as their meshes do,
	// within half the chord tolerance.
	tol := math.Max(rb.t.shell.Tol.ChordTolMm()/2, 1e-6)
	for j, p := range xs {
		if d := fit.PointAt(us[j]).Minus(p).Magnitude(); d > tol {
			return sec, filletErr(p, "%s can't be fitted within %.3g mm here (%.3g): make it smaller", what, tol, d)
		}
	}
	for _, u := range []float64{0.0625, 0.1875, 0.3125, 0.4375, 0.5625, 0.6875, 0.8125, 0.9375} {
		p := fit.PointAt(u)
		if h := math.Abs(height(p)); h > tol {
			return sec, filletErr(p, strays, h)
		}
	}
	sec.arc = fit
	return sec, nil
}

// fitThrough is c, a section of weights w, refitted through the points
// xs at first parameters ts: its ends c's, the second and last but one
// control points moved from them along ways, the points between free;
// the fixed weights make the weighted residual linear in them. Each
// point's parameter is then the nearest on the curve, and the fit again.
// It gives the curve and the points' parameters, and false where the
// equations have no solution.
func fitThrough(c srf.BSplineCurve, w []float64, ways [2][]geom.Vector, ts []float64, xs []geom.Vector) (srf.BSplineCurve, []float64, bool) {
	n := len(c.Ctrl) - 1
	q0, q1 := c.Ctrl[0], c.Ctrl[n]
	nw := len(ways[0]) + len(ways[1])
	// One span's basis is Bernstein's, as it was before two spans were
	// fitted, which keeps the last digits of what one span made.
	basis := c.Basis
	if n == endDeg {
		basis = func(u float64) []float64 {
			b := make([]float64, endDeg+1)
			for i := range b {
				b[i] = binomial(endDeg, i) * math.Pow(u, float64(i)) * math.Pow(1-u, float64(endDeg-i))
			}
			return b
		}
	}
	nu := nw + 3*(n-3)
	us := append([]float64(nil), ts...)
	fit := c
	for range 10 {
		ata := make([][]float64, nu)
		for r := range ata {
			ata[r] = make([]float64, nu)
		}
		atb := make([]float64, nu)
		for j, x := range xs {
			b := basis(us[j])
			wt := 0.0
			for i := range b {
				wt += w[i] * b[i]
			}
			fixed := q0.ScaledBy(w[0]*b[0] + w[1]*b[1]).Plus(q1.ScaledBy(w[n-1]*b[n-1] + w[n]*b[n]))
			rhs := x.ScaledBy(wt).Minus(fixed)
			var cols []geom.Vector
			for _, t := range ways[0] {
				cols = append(cols, t.ScaledBy(w[1]*b[1]))
			}
			for _, t := range ways[1] {
				cols = append(cols, t.ScaledBy(w[n-1]*b[n-1]))
			}
			for i := 2; i <= n-2; i++ {
				f := w[i] * b[i]
				cols = append(cols, geom.Vector{X: f}, geom.Vector{Y: f}, geom.Vector{Z: f})
			}
			for r := range nu {
				for q := range nu {
					ata[r][q] += cols[r].Dot(cols[q])
				}
				atb[r] += cols[r].Dot(rhs)
			}
		}
		x, ok := solveDense(ata, atb)
		if !ok {
			return fit, us, false
		}
		p1, pn := q0, q1
		for j, t := range ways[0] {
			p1 = p1.Plus(t.ScaledBy(x[j]))
		}
		for j, t := range ways[1] {
			pn = pn.Plus(t.ScaledBy(x[len(ways[0])+j]))
		}
		ctrl := []geom.Vector{q0, p1}
		for i := 2; i <= n-2; i++ {
			o := nw + 3*(i-2)
			ctrl = append(ctrl, geom.Vector{X: x[o], Y: x[o+1], Z: x[o+2]})
		}
		ctrl = append(ctrl, pn, q1)
		fit.Ctrl = ctrl
		for j, p := range xs {
			us[j] = nearestParam(&fit, p, us[j])
		}
	}
	return fit, us, true
}

// maxSkippedCuts is how many of a fitted section's points fitCut may go
// without, where the cut doesn't meet the swept surface near the end.
const maxSkippedCuts = 2

// nearestParam is the parameter of curve c's point nearest p, from u.
func nearestParam(c *srf.BSplineCurve, p geom.Vector, u float64) float64 {
	h := 1e-4
	for range 20 {
		a, b := math.Max(0, u-h), math.Min(1, u+h)
		d := c.PointAt(b).Minus(c.PointAt(a)).ScaledBy(1 / (b - a))
		if d.MagSquared() == 0 {
			break
		}
		step := p.Minus(c.PointAt(u)).Dot(d) / d.MagSquared()
		u = math.Max(0, math.Min(1, u+step))
		if math.Abs(step) < 1e-12 {
			break
		}
	}
	return u
}

// binomial is n choose k.
func binomial(n, k int) float64 {
	b := 1.0
	for i := 1; i <= k; i++ {
		b = b * float64(n-k+i) / float64(i)
	}
	return b
}

// solveDense solves a x = b by elimination with partial pivoting.
func solveDense(a [][]float64, b []float64) ([]float64, bool) {
	n := len(b)
	x := make([]float64, n)
	for c := range n {
		p := c
		for r := c + 1; r < n; r++ {
			if math.Abs(a[r][c]) > math.Abs(a[p][c]) {
				p = r
			}
		}
		if math.Abs(a[p][c]) < 1e-300 {
			return x, false
		}
		a[c], a[p] = a[p], a[c]
		b[c], b[p] = b[p], b[c]
		for r := c + 1; r < n; r++ {
			f := a[r][c] / a[c][c]
			for q := c; q < n; q++ {
				a[r][q] -= f * a[c][q]
			}
			b[r] -= f * b[c]
		}
	}
	for r := n - 1; r >= 0; r-- {
		s := b[r]
		for q := r + 1; q < n; q++ {
			s -= a[r][q] * x[q]
		}
		x[r] = s / a[r][r]
	}
	return x, true
}

// chamferNormals are sec's feet with, in place of the sides' faces'
// normals, the chamfer's own there: square to its ruling and to the way
// the foot runs along the chain, from the sections either side.
func (rb *rollingBall) chamferNormals(sec rbSection) [2]rbFoot {
	feet := sec.foot
	ruling := feet[1].p.Minus(feet[0].p)
	_, d := rb.along(sec.s)
	h := 1e-3 * math.Max(1, rb.r)
	a, aerr := rb.section(sec.s - h)
	b, berr := rb.section(sec.s + h)
	for k := range feet {
		way := d
		if aerr == nil && berr == nil {
			if w := b.foot[k].p.Minus(a.foot[k].p); w.Magnitude() > 0 {
				way = w
			}
		}
		if n := ruling.Cross(way); n.Magnitude() > 0 {
			feet[k].n = n.WithMagnitude(1)
		}
	}
	return feet
}

// chamferFootNormals is chamferNormals for an end section, which cuts
// the chamfer obliquely: each foot lies on the chamfer's section of its
// own arc length (footS), whose ruling and whose foot's way along the
// edge give the chamfer's normal there, not the end section's chord.
func (rb *rollingBall) chamferFootNormals(sec rbSection) [2]rbFoot {
	feet := sec.foot
	h := 1e-3 * math.Max(1, rb.r)
	for i := range feet {
		si := sec.footS[i]
		at, err := rb.section(si)
		a, aerr := rb.section(si - h)
		b, berr := rb.section(si + h)
		if err != nil || aerr != nil || berr != nil {
			return rb.chamferNormals(sec)
		}
		ruling := at.foot[1].p.Minus(at.foot[0].p)
		way := b.foot[i].p.Minus(a.foot[i].p)
		if n := ruling.Cross(way); n.Magnitude() > 0 {
			feet[i].n = n.WithMagnitude(1)
		}
	}
	return feet
}
