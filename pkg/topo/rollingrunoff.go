package topo

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// An open fillet's end that runs off a face (roadmap 22.3): the end's
// section, in the flat end face's plane, reaches past the far end w of
// that face's edge with one side's face, as a wide chamfer of a band's
// side on a cone does past the band's top. The fillet then goes on to
// the next face there, when it is flat too: the fillet is cut by that
// face's plane as well. The fillet's end is then its end section up to
// the next face's plane, at p1, and the curve where that plane cuts the
// fillet, from p1 to where the side's contact curve crosses the plane,
// p2.
//
// Made so far is the end of a fillet that fills a corner, whose p1 lies
// on past w along the line the end face and the next face meet on: both
// faces grow, the end face by the fillet's end section, the next face
// by the fillet's cut, and they share the line from p1 to w. The side's
// face is cut back along the contact curve up to p2, and its edges with
// the end face and with the next face, up to p2, go. A fillet that takes
// material away, whose p1 lies on the end face's edge with the next
// face, is refused.
//
// The fillet's surface is fitted as for any open end, its end section in
// the end face's plane (the side's surface carried on past the next
// face); its patches that cross the next face's plane are cut along it.

// rbRunOff is where an open end runs off side j's face: the far end w of
// that face's edge with the end face, the flat face n past it, its
// plane np·x = nd with np pointing away from the solid's part the
// fillet keeps, n's edges with side j's face and with the end face, and
// p1 and p2.
type rbRunOff struct {
	j            int
	w            int
	n            int
	np           geom.Vector
	nd           float64
	sideN, endN  int
	p1, p2       geom.Vector
	first, after int // the patches the next face's plane cuts, [first, after)
	// p1At is where the plane cuts the end section; p1 is w instead when
	// that is within 100 LengthEps of it, so that the end face and the
	// next face don't share a line too short to mesh (the mesher merges
	// points that near in a face's (u, v)), and the curves' ends moved
	// there stay within an edge's tolerance of where they were.
	p1At geom.Vector
}

// h is x's height over the next face's plane.
func (ro *rbRunOff) h(x geom.Vector) float64 { return ro.np.Dot(x) - ro.nd }

// runOffAt is where end's section runs off side j's face past the far
// end of its edge with the end face: the face there, which must be flat
// and meet both side j's face and the end face at that vertex.
func (rb *rollingBall) runOffAt(end *rbEnd, j int) (*rbRunOff, error) {
	t := rb.t
	ed := &t.Edges[end.a[j]]
	w := ed.V[0]
	if w == end.v {
		w = ed.V[1]
	}
	ro := &rbRunOff{j: j, w: w, n: -1, sideN: -1, endN: -1}
	if len(t.Vertices[w].Edges) != 3 {
		return nil, errNotRolling
	}
	for _, x := range t.Vertices[w].Edges {
		if x == end.a[j] {
			continue
		}
		fs := t.EdgeFaces(x)
		other := func(f int) int {
			if fs[0] == f {
				return fs[1]
			}
			if fs[1] == f {
				return fs[0]
			}
			return -1
		}
		if g := other(end.face[j]); g >= 0 {
			ro.sideN = x
			if ro.n >= 0 && ro.n != g {
				return nil, errNotRolling
			}
			ro.n = g
		} else if g := other(end.f); g >= 0 {
			ro.endN = x
			if ro.n >= 0 && ro.n != g {
				return nil, errNotRolling
			}
			ro.n = g
		}
	}
	if ro.sideN < 0 || ro.endN < 0 || ro.n == end.face[1-j] {
		return nil, errNotRolling
	}
	n, ok := t.planarNormal(ro.n)
	if !ok {
		return nil, errNotRolling
	}
	ro.np = n.WithMagnitude(1)
	ro.nd = ro.np.Dot(t.Vertices[w].P)
	if hv := ro.h(t.Vertices[end.v].P); hv > 0 {
		ro.np, ro.nd = ro.np.ScaledBy(-1), -ro.nd
	} else if hv == 0 {
		return nil, errNotRolling
	}
	return ro, nil
}

// bezierRoot is the parameter in [a, b] where h along sb is 0, h being
// of opposite signs at a and b.
func bezierRoot(sb *srf.SBezier, h func(geom.Vector) float64, a, b float64) float64 {
	ha := h(sb.PointAt(a))
	for it := 0; it < 200 && b-a > 1e-15; it++ {
		m := (a + b) / 2
		hm := h(sb.PointAt(m))
		if hm == 0 {
			return m
		}
		if (hm < 0) == (ha < 0) {
			a, ha = m, hm
		} else {
			b = m
		}
	}
	return (a + b) / 2
}

// runOffCuts finds, for an end that runs off, the patches the next
// face's plane cuts, where it cuts the end section (p1) and side j's
// contact curve (p2), and splits the next face's edges with the end face
// and with side j's face there.
func (rb *rollingBall) runOffCuts(sh *srf.SShell, row []srf.SSurface, k int, ro *rbRunOff) error {
	t := rb.t
	m := len(row)
	w := t.Vertices[ro.w].P
	// The end section, from side 0 to side 1, at the chain's end.
	endP, endSide := 0, 0
	if k == 1 {
		endP, endSide = m-1, 2
	}
	sec := row[endP].EdgeCurve(endSide)
	h0, h1 := ro.h(sec.Start()), ro.h(sec.Finish())
	inSide, outSide := h0, h1
	if ro.j == 0 {
		inSide, outSide = h1, h0
	}
	if !(inSide < 0 && outSide > 0) {
		return filletErr(sec.PointAt(0.5), "the fillet's end runs off the faces on both sides here: make it smaller")
	}
	ro.p1 = sec.PointAt(bezierRoot(&sec, ro.h, 0, 1))
	ro.p1At = ro.p1
	if ro.p1.Minus(w).Magnitude() < 100*geom.LengthEps {
		ro.p1 = w
	}
	switch alongFrom(ro.p1At, w, t.Edges[ro.endN].Points) {
	case 1:
		return filletErr(ro.p1, "the fillet's end runs off the face past the next one too here: make it smaller")
	case 0:
		// The end face's edge with the next face cut back: a fillet that
		// takes material away, where no case has yet shown the result
		// sound.
		return filletErr(ro.p1, "the fillet's end runs off the face on side %d here onto the next face, which a fillet "+
			"that takes material away can't yet follow: make it smaller", ro.j+1)
	}
	// Side j's contact curve, in from the end, to where it comes back
	// under the plane.
	ui := 3
	if ro.j == 1 {
		ui = 1
	}
	found := false
	for i := 0; i < m && !found; i++ {
		b := m - 1 - i
		if k == 0 {
			b = i
		}
		c := row[b].EdgeCurve(ui)
		// Its end toward the chain's end, and the other.
		farT, nearT := 1.0, 0.0
		if k == 0 {
			farT, nearT = 0, 1
		}
		if ro.h(c.PointAt(nearT)) > 0 {
			continue
		}
		if ro.h(c.PointAt(farT)) <= 0 {
			return filletErr(c.PointAt(farT), "the fillet's contact with the face on side %d crosses the next face's plane "+
				"more than once", ro.j+1)
		}
		ro.p2 = c.PointAt(bezierRoot(&c, ro.h, math.Min(farT, nearT), math.Max(farT, nearT)))
		if k == 1 {
			ro.first, ro.after = b, m
		} else {
			ro.first, ro.after = 0, b+1
		}
		found = true
	}
	if !found {
		return filletErr(sec.PointAt(0.5), "the fillet runs off the face on side %d all along", ro.j+1)
	}
	if alongFrom(ro.p2, w, t.Edges[ro.sideN].Points) != 0 {
		return filletErr(ro.p2, "the fillet runs off the face past the next one too here: make it smaller")
	}
	if err := splitNearest(sh, ro.p2, w); err != nil {
		return filletErr(ro.p2, "the fillet can't cut the next face's edge here: make it smaller")
	}
	return nil
}

// snapEnds moves the curves' ends at from to to.
func snapEnds(curves []srf.SBezier, from, to geom.Vector) {
	for i := range curves {
		c := &curves[i]
		if c.Ctrl[0].Equals(from) {
			c.Ctrl[0] = to
		}
		if c.Ctrl[c.Degree].Equals(from) {
			c.Ctrl[c.Degree] = to
		}
	}
}

// contactPiece is side k's contact curve along patch b, or the part of
// it an end that runs off keeps, and whether any is kept.
func (rb *rollingBall) contactPiece(row []srf.SSurface, cuts []rbEndCut, k, b int) (srf.SBezier, bool) {
	ui := 3
	if k == 1 {
		ui = 1
	}
	sb := row[b].EdgeCurve(ui)
	for _, cut := range cuts {
		ro := cut.off
		if ro == nil || ro.j != k || b < ro.first || b >= ro.after {
			continue
		}
		if ro.h(sb.Start()) > 0 && ro.h(sb.Finish()) > 0 {
			return sb, false
		}
		t := bezierRoot(&sb, ro.h, 0, 1)
		if ro.h(sb.Start()) <= 0 {
			sb = sb.Sub(0, t)
			sb.Ctrl[sb.Degree] = ro.p2
		} else {
			sb = sb.Sub(t, 1)
			sb.Ctrl[0] = ro.p2
		}
	}
	return sb, true
}

// patchLoop is patch p's outline as rollingBallFillet puts it in, round
// in (u, v): along v = 0, u = 1, back along v = 1 and u = 0.
func patchLoop(p *srf.SSurface) []srf.SBezier {
	e0, e1, e2, e3 := p.EdgeCurve(0), p.EdgeCurve(1), p.EdgeCurve(2), p.EdgeCurve(3)
	e2.Reverse()
	e3.Reverse()
	return []srf.SBezier{e0, e1, e2, e3}
}

// patchSideUV is the (u, v) at t along side i of patchLoop.
func patchSideUV(i int, t float64) (float64, float64) {
	switch i {
	case 0:
		return t, 0
	case 1:
		return 1, t
	case 2:
		return 1 - t, 1
	}
	return 0, 1 - t
}

// clipPatch is the part of patch p under the plane ro gives, as a loop
// round it (patchLoop's sides cut where they cross the plane, and the
// curves across the patch in the plane between), and those curves
// across; no loop when none of it is under.
func clipPatch(p *srf.SSurface, ro *rbRunOff, tol float64) ([]srf.SBezier, []srf.SBezier, error) {
	type seg struct {
		side   int
		t0, t1 float64
		sb     srf.SBezier
		in     bool
	}
	var segs []seg
	anyOut := false
	for i, sb := range patchLoop(p) {
		const n = 64
		ts := []float64{0}
		prev := ro.h(sb.PointAt(0))
		for s := 1; s <= n; s++ {
			x := float64(s) / n
			hv := ro.h(sb.PointAt(x))
			if prev < 0 && hv > 0 || prev > 0 && hv < 0 {
				ts = append(ts, bezierRoot(&sb, ro.h, float64(s-1)/n, x))
			}
			prev = hv
		}
		ts = append(ts, 1)
		for s := 1; s < len(ts); s++ {
			a, b := ts[s-1], ts[s]
			if b-a < 1e-12 {
				continue
			}
			in := ro.h(sb.PointAt((a+b)/2)) <= 0
			anyOut = anyOut || !in
			piece := sb.Sub(a, b)
			segs = append(segs, seg{side: i, t0: a, t1: b, sb: piece, in: in})
		}
	}
	if !anyOut {
		return patchLoop(p), nil, nil
	}
	start := -1
	for i := range segs {
		if segs[i].in && !segs[(i+len(segs)-1)%len(segs)].in {
			start = i
			break
		}
	}
	if start < 0 {
		return nil, nil, nil
	}
	var loop, across []srf.SBezier
	for c := range segs {
		sg := segs[(start+c)%len(segs)]
		if !sg.in {
			continue
		}
		loop = append(loop, sg.sb)
		next := segs[(start+c+1)%len(segs)]
		if next.in {
			continue
		}
		// Out from here: across the patch in the plane to where the outline
		// comes back under it.
		d := c + 1
		for !segs[(start+d)%len(segs)].in {
			d++
		}
		back := segs[(start+d)%len(segs)]
		u0, v0 := patchSideUV(sg.side, sg.t1)
		u1, v1 := patchSideUV(back.side, back.t0)
		cs, err := acrossPatch(p, ro, u0, v0, u1, v1, sg.sb.Finish(), back.sb.Start(), tol)
		if err != nil {
			return nil, nil, err
		}
		loop = append(loop, cs...)
		across = append(across, cs...)
	}
	return loop, across, nil
}

// acrossPatch is the curve in ro's plane on patch p from a, at (u0, v0),
// to b, at (u1, v1): cubics whose control points lie in the plane, each
// within tol of the patch.
func acrossPatch(p *srf.SSurface, ro *rbRunOff, u0, v0, u1, v1 float64, a, b geom.Vector, tol float64) ([]srf.SBezier, error) {
	clamp := func(x float64) float64 { return math.Max(0, math.Min(1, x)) }
	// The point of the patch in the plane near (u, v), and the curve's
	// way there.
	on := func(u, v float64) (geom.Vector, geom.Vector) {
		for range 60 {
			x := p.PointAt(u, v)
			hv := ro.h(x)
			var tu, tv geom.Vector
			p.TangentsAt(u, v, &tu, &tv, true)
			gu, gv := ro.np.Dot(tu), ro.np.Dot(tv)
			g2 := gu*gu + gv*gv
			if g2 == 0 || math.Abs(hv) < 1e-13 {
				break
			}
			u, v = clamp(u-hv*gu/g2), clamp(v-hv*gv/g2)
		}
		x := p.PointAt(u, v)
		return x.Minus(ro.np.ScaledBy(ro.h(x))), ro.np.Cross(p.NormalAt(u, v))
	}
	proj := func(x geom.Vector) geom.Vector { return x.Minus(ro.np.ScaledBy(ro.h(x))) }
	off := func(x geom.Vector) float64 {
		var u, v float64
		p.ClosestPointToUV(x, &u, &v, false)
		return p.PointAt(u, v).Minus(x).Magnitude()
	}
	chord := b.Minus(a)
	if chord.Magnitude() < geom.LengthEps {
		return nil, nil
	}
	for pieces := 1; pieces <= 64; pieces *= 2 {
		var out []srf.SBezier
		ok := true
		prev := a
		_, d0 := on(u0, v0)
		for i := 1; i <= pieces && ok; i++ {
			lam := float64(i) / float64(pieces)
			x, d1 := on(u0+(u1-u0)*lam, v0+(v1-v0)*lam)
			if i == pieces {
				x = b
			}
			l := x.Minus(prev).Magnitude()
			if d0.Dot(chord) < 0 {
				d0 = d0.ScaledBy(-1)
			}
			if d1.Dot(chord) < 0 {
				d1 = d1.ScaledBy(-1)
			}
			c1 := proj(prev.Plus(d0.WithMagnitude(l / 3)))
			c2 := proj(x.Minus(d1.WithMagnitude(l / 3)))
			sb := srf.SBezierFrom4(prev, c1, c2, x)
			for _, s := range []float64{0.25, 0.5, 0.75} {
				if off(sb.PointAt(s)) > tol {
					ok = false
				}
			}
			out = append(out, sb)
			prev, d0 = x, d1
		}
		if ok {
			return out, nil
		}
	}
	return nil, filletErr(a, "the fillet can't follow where the next face's plane cuts it here")
}

// closeRunOff puts an end that runs off into the end face and the next
// face: the end face loses its edges with the sides' faces up to side
// 1-j's cut, and takes side 1-j's curve in its plane where that face goes
// on past it, the end section up to p1, and the line from p1 to w; the
// next face loses its edge with side j's face up to p2 and takes the
// curve across the fillet from p2 to p1 and the line.
func (rb *rollingBall) closeRunOff(sh *srf.SShell, cut rbEndCut, endSec srf.SBezier, across []srf.SBezier, h *core.HSCurve) error {
	t := rb.t
	ro := cut.off
	v, w := t.Vertices[cut.end.v].P, t.Vertices[ro.w].P
	q := cut.q[1-ro.j]
	is := func(tb srf.STrimBy, a, b geom.Vector) bool {
		return tb.StartP.Equals(a) && tb.FinishP.Equals(b) || tb.StartP.Equals(b) && tb.FinishP.Equals(a)
	}
	add := func(s *srf.SSurface, sb srf.SBezier, other core.HSSurface) core.HSCurve {
		var pts []geom.Vector
		sb.MakePwlIntoPoints(&pts, rb.t.shell.Tol, 0)
		pts[0], pts[len(pts)-1] = sb.Start(), sb.Finish()
		nc := srf.SCurve{H: *h, Exact: sb, IsExact: true, Deg: sb.Degree, SurfA: s.H, SurfB: other}
		h.V++
		for _, p := range pts {
			nc.Points = append(nc.Points, srf.SCurvePt{Vertex: true, P: p})
		}
		s.Trim = append(s.Trim, srf.STrimBy{Curve: nc.H, StartP: sb.Start(), FinishP: sb.Finish(), Start: 0, Finish: 1})
		sh.Curve = append(sh.Curve, nc)
		return nc.H
	}
	// chain runs pieces as a chain from a.
	chain := func(pieces []srf.SBezier, from geom.Vector) ([]srf.SBezier, bool) {
		left := append([]srf.SBezier(nil), pieces...)
		var out []srf.SBezier
		at := from
		for len(left) > 0 {
			found := false
			for i, sb := range left {
				if sb.Start().Equals(at) || sb.Finish().Equals(at) {
					if !sb.Start().Equals(at) {
						sb.Reverse()
					}
					out = append(out, sb)
					at = sb.Finish()
					left = append(left[:i], left[i+1:]...)
					found = true
					break
				}
			}
			if !found {
				return nil, false
			}
		}
		return out, true
	}
	// put drops the face's trims between the pairs, and adds the pieces
	// from a to b, run the way the face's kept trim at a goes on: their
	// curves, as the face runs them.
	type added struct {
		h  core.HSCurve
		sb srf.SBezier
	}
	put := func(f int, pairs [][2]geom.Vector, pieces []srf.SBezier, a, b geom.Vector) ([]added, error) {
		s := sh.FindSurface(t.Faces[f].Surface)
		if s == nil {
			return nil, filletErr(w, "a face at the fillet's end is not in the shell")
		}
		var kept []srf.STrimBy
		dropped := 0
		for _, tb := range s.Trim {
			gone := false
			for _, pr := range pairs {
				if is(tb, pr[0], pr[1]) {
					gone = true
				}
			}
			if gone {
				dropped++
				continue
			}
			kept = append(kept, tb)
		}
		if dropped != len(pairs) {
			return nil, filletErr(w, "the faces at the fillet's end don't meet as its end needs here (%d of %d edges)",
				dropped, len(pairs))
		}
		seq, ok := chain(pieces, a)
		if !ok {
			return nil, filletErr(a, "the fillet's end curves don't join here")
		}
		// The loop goes from a's trim into the pieces, or comes out of them
		// into it.
		into := false
		for _, tb := range kept {
			if tb.FinishP.Equals(a) {
				into = true
			}
		}
		s.Trim = kept
		if !into {
			seq, _ = chain(pieces, b)
		}
		var out []added
		for _, sb := range seq {
			out = append(out, added{add(s, sb, core.HSSurface{}), sb})
		}
		forget(s)
		return out, nil
	}
	j := ro.j
	pieces := append(append([]srf.SBezier(nil), cut.goes[1-j]...), endSec)
	from := q
	pairs := [][2]geom.Vector{{v, w}}
	if cut.goes[1-j] == nil {
		pairs = append(pairs, [2]geom.Vector{q, v})
	} else {
		from = v
	}
	sideS := sh.FindSurface(t.Faces[cut.end.face[1-j]].Surface)
	if sideS == nil {
		return filletErr(v, "a face at the edge is not in the shell")
	}
	got, err := put(cut.end.f, pairs, pieces, from, ro.p1)
	if err != nil {
		return err
	}
	// Side 1-j's face, where it goes on past the end face, shares the
	// curve in the plane from the vertex to its cut, running it the other
	// way: every piece but the end section, which ends at p1.
	for _, g := range got {
		if cut.goes[1-j] == nil || g.sb.Start().Equals(ro.p1) || g.sb.Finish().Equals(ro.p1) {
			continue
		}
		for i := range sh.Curve {
			if sh.Curve[i].H == g.h {
				sh.Curve[i].SurfB = sideS.H
			}
		}
		sideS.Trim = append(sideS.Trim, srf.STrimBy{Curve: g.h, StartP: g.sb.Finish(), FinishP: g.sb.Start(),
			Start: 0, Finish: 1, Backwards: true})
	}
	forget(sideS)
	if _, err := put(ro.n, [][2]geom.Vector{{w, ro.p2}}, across, ro.p2, ro.p1); err != nil {
		return err
	}
	if ro.p1.Equals(w) {
		return nil
	}
	// The line from p1 to w, the end face's and the next face's: the end
	// face runs it on from the end section, the next face the other way.
	es, ns := sh.FindSurface(t.Faces[cut.end.f].Surface), sh.FindSurface(t.Faces[ro.n].Surface)
	line := srf.SBezierFrom2(ro.p1, w)
	last := got[len(got)-1].sb
	if !last.Finish().Equals(ro.p1) {
		line.Reverse()
	}
	lh := add(es, line, ns.H)
	ns.Trim = append(ns.Trim, srf.STrimBy{Curve: lh, StartP: line.Finish(), FinishP: line.Start(), Start: 0, Finish: 1,
		Backwards: true})
	forget(es)
	forget(ns)
	return nil
}
