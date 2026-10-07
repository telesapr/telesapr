package topo

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A drafted prism of an angle per side (roadmap 28.2): each piece of
// the profile leans by its own angle, 0 leaving it upright, so that
// some faces of an extrusion are drafted and the rest not. Where an arc
// meets a line at a sharp corner, the cone of the arc's side and the
// plane of the line's meet in a conic, which a rational quadratic
// follows exactly: the arc's side is the cone over enough of its turn
// to hold both ends of that curve, trimmed by it, and the plane is
// trimmed by it too. Two arcs at a sharp corner meet in a curve of
// degree four, which quintics through points of it follow within
// seamTol (arcArcSeam); pieces that meet tangent must lean alike, or
// the sides would part there.

// DraftedPrismAngles is DraftedPrism with angles[k][i] the angle in
// radians of the side of loop k's curve i.
func DraftedPrismAngles(loops [][]srf.SBezier, angles [][]float64, along geom.Vector, color core.RGBA, origin uint32, tol srf.Tolerance) (*srf.SShell, error) {
	h := along.Magnitude()
	if h < geom.LengthEps {
		return nil, errors.New("the extrusion has no length")
	}
	if len(loops) == 0 {
		return nil, errors.New("the profile has no loops")
	}
	if len(angles) != len(loops) {
		return nil, errors.New("the angles don't match the loops")
	}
	n := along.ScaledBy(1 / h)
	var sides []draftLoop
	for k, loop := range loops {
		if len(angles[k]) != len(loop) {
			return nil, errors.New("the angles don't match the loops")
		}
		as := append([]float64(nil), angles[k]...)
		for _, a := range as {
			if math.IsNaN(a) || math.Abs(a) >= math.Pi/2-1e-6 {
				return nil, fmt.Errorf("a draft of %.6g° leans the sides flat", a*180/math.Pi)
			}
		}
		if err := checkLoop(loop); err != nil {
			return nil, err
		}
		nl := newell(loop)
		if nl.Magnitude() == 0 || nl.WithMagnitude(1).Cross(n).Magnitude() > 1e-6 {
			return nil, errors.New("the profile does not lie square to the extrusion")
		}
		// The outer loop counter-clockwise about along, the holes
		// clockwise: the material lies on the left of each.
		if (nl.Dot(n) > 0) != (k == 0) {
			loop = reverseLoop(loop)
			for i, j := 0, len(as)-1; i < j; i, j = i+1, j-1 {
				as[i], as[j] = as[j], as[i]
			}
		}
		pieces, err := draftPieces(loop, n)
		if err != nil {
			return nil, err
		}
		ds := make([]float64, len(pieces))
		for i, a := range as {
			ds[i] = h * math.Tan(a)
		}
		dl, err := draftLoopOf(pieces, ds, along, n)
		if err != nil {
			return nil, err
		}
		sides = append(sides, dl)
	}

	sh := &srf.SShell{Tol: tol}
	next := origin
	add := func(s srf.SSurface, loops [][]srf.SBezier) error {
		s.Color = color
		s.Origin = next
		if origin != 0 {
			next++
		}
		_, err := AddFace(sh, s, loops)
		return err
	}
	var bottom, top [][]srf.SBezier
	for _, dl := range sides {
		bottom, top = append(bottom, dl.bottom), append(top, dl.top)
	}
	if err := add(capPlane(bottom[0], n.ScaledBy(-1)), bottom); err != nil {
		return nil, err
	}
	for _, dl := range sides {
		for _, f := range dl.faces {
			if err := add(f.s, [][]srf.SBezier{f.loop}); err != nil {
				return nil, shellErr(f.loop[0].Start(), "a drafted side can't be put in: %v", err)
			}
		}
	}
	var far [][]srf.SBezier
	for _, l := range top {
		far = append(far, reverseLoop(l))
	}
	if err := add(capPlane(top[0], n), far); err != nil {
		return nil, err
	}
	t, err := Build(sh)
	if err != nil {
		return nil, err
	}
	if probs := t.Validate(); len(probs) > 0 {
		return nil, fmt.Errorf("the drafted solid is not closed: %s", probs[0].Msg)
	}
	return sh, nil
}

// draftPieces are loopPieces, but arcs may meet lines and arcs at sharp
// corners.
func draftPieces(loop []srf.SBezier, n geom.Vector) ([]profilePiece, error) {
	pieces, err := loopPieces(loop, n)
	if err != nil {
		return nil, err
	}
	return pieces, nil
}

// loopPieces are the pieces of a loop in the plane of normal n, lines
// and arcs, its material on the left of each.
func loopPieces(loop []srf.SBezier, n geom.Vector) ([]profilePiece, error) {
	var pieces []profilePiece
	for _, sb := range loop {
		a, m, b := sb.Start(), sb.PointAt(0.5), sb.Finish()
		p := profilePiece{a: a, b: b}
		if sb.Degree == 1 || m.DistanceToLine(a, b.Minus(a)) < geom.LengthEps {
			p.into = n.Cross(b.Minus(a)).WithMagnitude(1)
			pieces = append(pieces, p)
			continue
		}
		c, ok := circleThrough(a, m, b, n)
		r := c.Minus(a).Magnitude()
		if !ok || sb.Degree != 2 || math.Abs(sb.PointAt(0.25).Minus(c).Magnitude()-r) > geom.LengthEps {
			return nil, shellErr(m, "a curve of the profile is neither a line nor an arc; a draft leans lines and arcs")
		}
		p.arc, p.centre, p.r = true, c, r
		ra, rb := a.Minus(c), b.Minus(c)
		p.angle = math.Atan2(ra.Cross(rb).Dot(n), ra.Dot(rb))
		if rm := m.Minus(c); math.Signbit(ra.Cross(rm).Dot(n)) != math.Signbit(p.angle) {
			p.angle -= math.Copysign(2*math.Pi, p.angle)
		}
		radial := m.Minus(c).WithMagnitude(1)
		left := n.Cross(pieceTangentAt(p, radial, n))
		p.outward = left.Dot(radial) > 0
		p.into = radial
		if !p.outward {
			p.into = radial.ScaledBy(-1)
		}
		pieces = append(pieces, p)
	}
	return pieces, nil
}

// smoothCorner reports whether p runs on into q with no corner.
func smoothCorner(p, q profilePiece, n geom.Vector) bool {
	tp, tq := pieceTangent(p, false, n), pieceTangent(q, true, n)
	return tp.Cross(tq).Magnitude() <= 1e-6 && tp.Dot(tq) > 0
}

// draftLoop is a loop's drafted sides and its two ends' loops.
type draftLoop struct {
	bottom, top []srf.SBezier
	faces       []draftFace
}

type draftFace struct {
	s    srf.SSurface
	loop []srf.SBezier
}

// draftLoopOf builds the sides of pieces, piece i moved into the
// material by ds[i] over along.
func draftLoopOf(pieces []profilePiece, ds []float64, along, n geom.Vector) (draftLoop, error) {
	var dl draftLoop
	k := len(pieces)
	moved := make([]profilePiece, k)
	for i, p := range pieces {
		q := p
		d := ds[i]
		if p.arc {
			if p.outward {
				q.r = p.r + d
			} else {
				q.r = p.r - d
			}
			if q.r < geom.LengthEps {
				return dl, shellErr(p.a, "a draft of %.6g over %.6g takes an arc of the profile away", math.Atan(d/along.Magnitude())*180/math.Pi, along.Magnitude())
			}
		} else {
			q.a, q.b = p.a.Plus(p.into.ScaledBy(d)), p.b.Plus(p.into.ScaledBy(d))
		}
		moved[i] = q
	}
	what := "the draft"
	// Where each piece meets the one before, moved, in the profile's
	// plane; and the seam between their sides.
	corner := make([]geom.Vector, k)
	for i := range pieces {
		prev := (i + k - 1) % k
		p0, q0 := pieces[prev], pieces[i]
		v := q0.a
		if smoothCorner(p0, q0, n) {
			if math.Abs(ds[prev]-ds[i]) > geom.LengthEps {
				return dl, shellErr(v, "two pieces of the profile that meet tangent lean differently; "+
					"there the sides would part")
			}
		}
		c, err := meetMoved(moved[prev], moved[i], v, p0, q0, ds[i], n, what)
		if err != nil {
			return dl, err
		}
		corner[i] = c
	}
	seams := make([][]srf.SBezier, k)
	for i := range pieces {
		prev := (i + k - 1) % k
		s, err := draftSeam(pieces[prev], pieces[i], moved[prev], moved[i], pieces[i].a, corner[i].Plus(along), ds[prev], ds[i], along, n)
		if err != nil {
			return dl, err
		}
		seams[i] = s
	}
	for i, p := range pieces {
		next := (i + 1) % k
		a, b := p.a, p.b
		ta, tb := corner[i].Plus(along), corner[next].Plus(along)
		start := seams[i]  // bottom a to top ta
		end := seams[next] // bottom b to top tb
		startDown := reversedChain(start)
		if !p.arc {
			if tb.Minus(ta).Dot(b.Minus(a)) <= 0 {
				return dl, shellErr(a, "the draft makes the sides meet, closing up the far end")
			}
			bot, tp := srf.SBezierFrom2(a, b), srf.SBezierFrom2(ta, tb)
			rtp := tp
			rtp.Reverse()
			dl.bottom = append(dl.bottom, bot)
			dl.top = append(dl.top, tp)
			trims := append(append(append([]srf.SBezier{bot}, end...), rtp), startDown...)
			dl.faces = append(dl.faces, draftFace{ruledPatch(bot, tp), trims})
			continue
		}
		faces, bot, tp, err := draftArcSides(p, moved[i], ta, tb, start, end, along, n)
		if err != nil {
			return dl, err
		}
		dl.bottom = append(dl.bottom, bot...)
		dl.top = append(dl.top, tp...)
		dl.faces = append(dl.faces, faces...)
	}
	return dl, nil
}

// arcFrame measures turns about an arc piece's centre along its way:
// phi of a point is the angle from its start, growing along it.
type arcFrame struct {
	c, u0, axis geom.Vector
}

func frameOf(p profilePiece, n geom.Vector) arcFrame {
	axis := n.WithMagnitude(1)
	if p.angle < 0 {
		axis = axis.ScaledBy(-1)
	}
	return arcFrame{c: p.centre, u0: p.a.Minus(p.centre).WithMagnitude(1), axis: axis}
}

// phi is the turn to x's direction from the start, in (-pi, pi] about
// the start but continued past it for an arc that turns further.
func (f arcFrame) phi(x geom.Vector, near float64) float64 {
	d := x.Minus(f.c)
	d = d.Minus(f.axis.ScaledBy(d.Dot(f.axis)))
	a := math.Atan2(f.u0.Cross(d).Dot(f.axis), f.u0.Dot(d))
	for a < near-math.Pi {
		a += 2 * math.Pi
	}
	for a > near+math.Pi {
		a -= 2 * math.Pi
	}
	return a
}

// point is the point at turn phi on the circle of radius r about the
// centre moved by lift.
func (f arcFrame) point(phi, r float64, lift geom.Vector) geom.Vector {
	return f.c.Plus(f.u0.RotatedAroundAxis(f.axis, phi).ScaledBy(r)).Plus(lift)
}

// arcFromTo is the arc of radius r about the centre lifted by lift,
// from turn a to turn b, in one rational quadratic (b - a under half a
// turn).
func (f arcFrame) arcFromTo(a, b, r float64, lift geom.Vector) srf.SBezier {
	step := b - a
	w := math.Cos(step / 2)
	pa, pb := f.point(a, r, lift), f.point(b, r, lift)
	mid := f.u0.RotatedAroundAxis(f.axis, a+step/2).ScaledBy(r / w)
	return arc(pa, f.c.Plus(mid).Plus(lift), pb, w)
}

// draftArcSides are the faces of an arc piece's drafted side, a cone
// (moved is the piece at the far end, before it is lifted by along),
// from the seam start (bottom a to top ta) to the seam end (bottom b to
// top tb): the cone over the turns that hold both, in parts of at most
// a quarter turn, each trimmed to the side; and the side's bottom and
// top curves.
func draftArcSides(p, moved profilePiece, ta, tb geom.Vector, start, end []srf.SBezier, along, n geom.Vector) ([]draftFace, []srf.SBezier, []srf.SBezier, error) {
	f := frameOf(p, n)
	turn := math.Abs(p.angle)
	pa, pb := 0.0, turn
	qa := f.phi(ta, 0)
	qb := f.phi(tb, turn)
	lo, hi := math.Min(pa, qa), math.Max(pb, qb)
	// Where a seam is a conic, the cone goes on past it, so that the
	// mesher meets the curve inside the patch, with points of the patch
	// beside it, not along the patch's edge.
	const margin = 0.1
	if math.Abs(pa-qa) > 1e-12 {
		lo -= margin
	}
	if math.Abs(pb-qb) > 1e-12 {
		hi += margin
	}
	if qb-qa < 1e-9 {
		return nil, nil, nil, shellErr(p.a, "the draft takes an arc of the profile away at the far end")
	}
	parts := quarters(hi - lo)
	bounds := make([]float64, parts+1)
	for j := range bounds {
		bounds[j] = lo + (hi-lo)*float64(j)/float64(parts)
	}
	rb, rt := p.r, moved.r
	var faces []draftFace
	var bottom, top []srf.SBezier
	for j := range parts {
		x0, x1 := bounds[j], bounds[j+1]
		b0, b1 := math.Max(pa, x0), math.Min(pb, x1)
		t0, t1 := math.Max(qa, x0), math.Min(qb, x1)
		if b1-b0 < 1e-9 || t1-t0 < 1e-9 {
			return nil, nil, nil, shellErr(p.a, "the draft moves a corner too far round an arc of the profile")
		}
		surf := ruledPatch(f.arcFromTo(x0, x1, rb, geom.Vector{}), f.arcFromTo(x0, x1, rt, along))
		if j == 0 && math.Abs(pa-qa) > 1e-12 || j == parts-1 && math.Abs(pb-qb) > 1e-12 {
			// A part a conic trims: of degree two along its rulings too,
			// so that the mesher puts points of the surface beside the
			// curve rather than fanning chords of it.
			surf = elevateRuling(surf)
			surf.AvoidFlatEars = true
		}
		bot := f.arcFromTo(b0, b1, rb, geom.Vector{})
		tp := f.arcFromTo(t0, t1, rt, along)
		// The ends the seams share, exactly.
		if b0 == pa {
			bot.Ctrl[0] = p.a
		}
		if b1 == pb {
			bot.Ctrl[bot.Degree] = p.b
		}
		if t0 == qa {
			tp.Ctrl[0] = ta
		}
		if t1 == qb {
			tp.Ctrl[tp.Degree] = tb
		}
		rtp := tp
		rtp.Reverse()
		var right, left []srf.SBezier
		if j == parts-1 {
			right = end
		} else {
			right = []srf.SBezier{srf.SBezierFrom2(f.point(x1, rb, geom.Vector{}), f.point(x1, rt, along))}
		}
		if j == 0 {
			left = start
		} else {
			left = []srf.SBezier{srf.SBezierFrom2(f.point(x0, rb, geom.Vector{}), f.point(x0, rt, along))}
		}
		trims := append(append(append([]srf.SBezier{bot}, right...), rtp), reversedChain(left)...)
		faces = append(faces, draftFace{surf, trims})
		bottom = append(bottom, bot)
		top = append(top, tp)
	}
	return faces, bottom, top, nil
}

// draftSeam is the curve where the side of p (before) meets the side of
// q (after), from v at the bottom to tv at the top: a line between two
// planes, or along a generator where the sides meet smoothly; where a
// line meets an arc at a sharp corner, the conic where the plane meets
// the cone.
func draftSeam(p, q, pm, qm profilePiece, v, tv geom.Vector, dp, dq float64, along, n geom.Vector) ([]srf.SBezier, error) {
	line := []srf.SBezier{srf.SBezierFrom2(v, tv)}
	if (!p.arc && !q.arc) || smoothCorner(p, q, n) {
		return line, nil
	}
	if p.arc && q.arc {
		return arcArcSeam(p, q, pm, qm, v, tv, along, n)
	}
	ln, ar, arm := p, q, qm
	if p.arc {
		ln, ar, arm = q, p, pm
	}
	// The line's own offset.
	dl := dp
	if p.arc {
		dl = dq
	}
	h := along.Magnitude()
	nz := along.ScaledBy(1 / h)
	// The plane of the line's side: through its bottom and its top.
	topA := ln.a.Plus(ln.into.ScaledBy(dl)).Plus(along)
	np := ln.b.Minus(ln.a).Cross(topA.Minus(ln.a)).WithMagnitude(1)
	// The cone: about the arc's axis, its radius from rb at the bottom
	// to rt at the top.
	rb, rt := ar.r, arm.r
	coneNormal := func(x geom.Vector) geom.Vector {
		d := x.Minus(ar.centre)
		radial := d.Minus(nz.ScaledBy(d.Dot(nz))).WithMagnitude(1)
		gen := along.Plus(radial.ScaledBy(rt - rb))
		circ := nz.Cross(radial)
		return gen.Cross(circ)
	}
	t0 := np.Cross(coneNormal(v))
	t2 := np.Cross(coneNormal(tv))
	// The point where the tangents meet, in the plane.
	p1, ok := lineMeetIn(v, t0, tv, t2)
	if !ok {
		return line, nil
	}
	// A third point: at half the height, where the plane's trace meets
	// the circle there, nearest the middle of the chord.
	mid := v.Plus(tv).ScaledBy(0.5)
	lift := along.ScaledBy(0.5)
	rm := (rb + rt) / 2
	la := ln.a.Plus(ln.into.ScaledBy(dl / 2)).Plus(lift)
	cands := lineCircle(la, ln.b.Minus(ln.a), ar.centre.Plus(lift), rm)
	if len(cands) == 0 {
		return line, shellErr(v, "the draft's sides don't meet at a corner of an arc")
	}
	qx := cands[0]
	for _, c := range cands[1:] {
		if c.Minus(mid).Magnitude() < qx.Minus(mid).Magnitude() {
			qx = c
		}
	}
	// A conic that is a line: the sides meet along one.
	if qx.DistanceToLine(v, tv.Minus(v)) < 1e-9 && p1.DistanceToLine(v, tv.Minus(v)) < 1e-9 {
		return line, nil
	}
	// Q = a v + b p1 + c tv; then the weight is b / (2 sqrt(a c)).
	a, b, c, ok := barycentric(qx, v, p1, tv)
	if !ok || a <= 0 || c <= 0 || b <= 0 {
		return line, shellErr(v, "the draft's sides meet in a curve the kernel can't follow at a corner of an arc")
	}
	w := b / (2 * math.Sqrt(a*c))
	sb := arc(v, p1, tv, w)
	// It must lie on the cone: check at quarters.
	for _, t := range []float64{0.25, 0.5, 0.75} {
		x := sb.PointAt(t)
		hz := x.Minus(ar.centre).Dot(nz) / h
		d := x.Minus(ar.centre)
		rad := d.Minus(nz.ScaledBy(d.Dot(nz))).Magnitude()
		if math.Abs(rad-(rb+(rt-rb)*hz)) > 1e-6 {
			return line, shellErr(x, "the draft's seam at a corner of an arc leaves the cone by %g", math.Abs(rad-(rb+(rt-rb)*hz)))
		}
	}
	return []srf.SBezier{sb}, nil
}

// reversedChain is a chain of curves run the other way.
func reversedChain(ch []srf.SBezier) []srf.SBezier {
	out := make([]srf.SBezier, len(ch))
	for i, sb := range ch {
		sb.Reverse()
		out[len(ch)-1-i] = sb
	}
	return out
}

// lineMeetIn is where the lines through a along da and b along db meet,
// lines of one plane.
func lineMeetIn(a, da, b, db geom.Vector) (geom.Vector, bool) {
	// a + s da = b + u db, least squares on s and u.
	w := b.Minus(a)
	daa, dab, dbb := da.Dot(da), da.Dot(db), db.Dot(db)
	den := daa*dbb - dab*dab
	if math.Abs(den) < 1e-14*daa*dbb {
		return geom.Vector{}, false
	}
	s := (w.Dot(da)*dbb - w.Dot(db)*dab) / den
	return a.Plus(da.ScaledBy(s)), true
}

// barycentric are q's coordinates in the triangle a b c, in its plane.
func barycentric(q, a, b, c geom.Vector) (float64, float64, float64, bool) {
	v0, v1, v2 := b.Minus(a), c.Minus(a), q.Minus(a)
	d00, d01, d11 := v0.Dot(v0), v0.Dot(v1), v1.Dot(v1)
	d20, d21 := v2.Dot(v0), v2.Dot(v1)
	den := d00*d11 - d01*d01
	if math.Abs(den) < 1e-18 {
		return 0, 0, 0, false
	}
	y := (d11*d20 - d01*d21) / den
	z := (d00*d21 - d01*d20) / den
	return 1 - y - z, y, z, true
}

// elevateRuling raises a ruled patch's degree along its rulings (m)
// from one to two, the same surface.
func elevateRuling(s srf.SSurface) srf.SSurface {
	if s.DegM != 1 {
		return s
	}
	out := s
	out.DegM = 2
	for j := 0; j <= s.DegN; j++ {
		w0, w1 := s.Weight[0][j], s.Weight[1][j]
		out.Ctrl[0][j], out.Weight[0][j] = s.Ctrl[0][j], w0
		out.Ctrl[2][j], out.Weight[2][j] = s.Ctrl[1][j], w1
		out.Ctrl[1][j] = s.Ctrl[0][j].ScaledBy(w0).Plus(s.Ctrl[1][j].ScaledBy(w1)).ScaledBy(1 / (w0 + w1))
		out.Weight[1][j] = (w0 + w1) / 2
	}
	return out
}

// seamTol is how far off either cone the seam two drafted arcs meet in
// may run.
const seamTol = 1e-6

// arcArcSeam is the curve where the cones of two drafted arcs p and q
// meet at a sharp corner, from v at the bottom to tv at the top (pm and
// qm the arcs at the far end): at each height the point where the two
// circles there cross, nearest the one below. The true curve is of
// degree four; quintic Beziers, each through six points of it at
// Chebyshev-Lobatto heights, follow it in one, two, four or more pieces
// up the height, as many as keep each within seamTol of both cones.
func arcArcSeam(p, q, pm, qm profilePiece, v, tv, along, n geom.Vector) ([]srf.SBezier, error) {
	h := along.Magnitude()
	nz := along.ScaledBy(1 / h)
	at := func(t float64, near geom.Vector) (geom.Vector, bool) {
		lift := along.ScaledBy(t)
		c := circleCircle(p.centre.Plus(lift), p.r+(pm.r-p.r)*t, q.centre.Plus(lift), q.r+(qm.r-q.r)*t, n)
		if len(c) == 0 {
			return geom.Vector{}, false
		}
		if c[1].Minus(near).Magnitude() < c[0].Minus(near).Magnitude() {
			return c[1], true
		}
		return c[0], true
	}
	off := func(x geom.Vector, a, am profilePiece) float64 {
		d := x.Minus(a.centre)
		hz := d.Dot(nz) / h
		rad := d.Minus(nz.ScaledBy(d.Dot(nz))).Magnitude()
		return math.Abs(rad - (a.r + (am.r-a.r)*hz))
	}
	worst := 0.0
	var worstAt geom.Vector
	for pieces := 1; pieces <= 32; pieces *= 2 {
		var out []srf.SBezier
		from, near := v, v
		worst = 0
		ok := true
		for k := 0; k < pieces && ok; k++ {
			t0, t1 := float64(k)/float64(pieces), float64(k+1)/float64(pieces)
			to := tv
			if k+1 < pieces {
				x, found := at(t1, near)
				if !found {
					return nil, shellErr(v, "the draft's sides part at a corner of two arcs")
				}
				to = x
			}
			sb, found := quinticThrough(func(t float64, near geom.Vector) (geom.Vector, bool) {
				return at(t0+(t1-t0)*t, near)
			}, from, to)
			if !found {
				return nil, shellErr(v, "the draft's sides part at a corner of two arcs")
			}
			for j := 1; j < 20; j++ {
				x := sb.PointAt(float64(j) / 20)
				if e := math.Max(off(x, p, pm), off(x, q, qm)); e > worst {
					worst, worstAt = e, x
				}
			}
			out = append(out, sb)
			from, near = to, to
			ok = worst <= seamTol
		}
		if ok {
			return out, nil
		}
	}
	return nil, shellErr(worstAt, "the draft's seam at a corner of two arcs leaves their cones by %g", worst)
}

// quinticThrough is the quintic Bezier from a to b through the points
// at(t) of a curve at Chebyshev-Lobatto parameters in between, each
// found nearest the one before.
func quinticThrough(at func(t float64, near geom.Vector) (geom.Vector, bool), a, b geom.Vector) (srf.SBezier, bool) {
	const deg = 5
	var ts [deg + 1]float64
	for i := range ts {
		ts[i] = (1 - math.Cos(float64(i)*math.Pi/deg)) / 2
	}
	var pts [deg + 1]geom.Vector
	pts[0], pts[deg] = a, b
	prev := a
	for i := 1; i < deg; i++ {
		x, ok := at(ts[i], prev)
		if !ok {
			return srf.SBezier{}, false
		}
		pts[i], prev = x, x
	}
	// Bernstein's matrix at ts, solved for each coordinate.
	var sb srf.SBezier
	sb.Degree = deg
	for axis := range 3 {
		var m [deg + 1][deg + 2]float64
		for i := 0; i <= deg; i++ {
			for j := 0; j <= deg; j++ {
				m[i][j] = bernstein(deg, j, ts[i])
			}
			m[i][deg+1] = [3]float64{pts[i].X, pts[i].Y, pts[i].Z}[axis]
		}
		for col := 0; col <= deg; col++ {
			piv := col
			for r := col + 1; r <= deg; r++ {
				if math.Abs(m[r][col]) > math.Abs(m[piv][col]) {
					piv = r
				}
			}
			m[col], m[piv] = m[piv], m[col]
			for r := 0; r <= deg; r++ {
				if r == col {
					continue
				}
				f := m[r][col] / m[col][col]
				for c := col; c <= deg+1; c++ {
					m[r][c] -= f * m[col][c]
				}
			}
		}
		for j := 0; j <= deg; j++ {
			x := m[j][deg+1] / m[j][j]
			switch axis {
			case 0:
				sb.Ctrl[j].X = x
			case 1:
				sb.Ctrl[j].Y = x
			default:
				sb.Ctrl[j].Z = x
			}
		}
	}
	for j := 0; j <= deg; j++ {
		sb.Weight[j] = 1
	}
	sb.Ctrl[0], sb.Ctrl[deg] = a, b
	return sb, true
}

// bernstein is the Bernstein polynomial j of degree n at t.
func bernstein(n, j int, t float64) float64 {
	c := 1.0
	for k := 1; k <= j; k++ {
		c = c * float64(n-j+k) / float64(k)
	}
	return c * math.Pow(t, float64(j)) * math.Pow(1-t, float64(n-j))
}
