package topo

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A drafted extrusion's curved side taken away runs out past the solid
// by the thickness. Where it meets a kept side at an inside corner, as
// a round lobe on a block's edge or on a disc's rim does, running out
// would cut into the wall under the kept side. The corner, where the
// kept side's leaning plane or cone meets the curved side's cone, moves
// along a curve going up, so no plane of a drafted prism follows it; a
// revolve's inside steps through its corner (shellrevolve_step.go), a
// drafted one can't. Instead the wall under the kept side ends a little
// short of the corner: the inside crosses the kept side on a plane
// square to it there, leaning to follow the corner's chord from the
// inside's foot to its head, set back from the corner's path by its sag
// and a margin, so it cuts the kept side cleanly and never the curved
// one. Outside the kept side it turns back along a plane a little
// outside it, parallel to a flat side or touching a round one's
// circle moved out where the step crosses it, to the curved side run
// out. Both planes are drafted, so the inside stays one drafted prism.
// A round kept side is crossed square to it where the step is at the
// inside's foot only: going up the step slides along the tangent there,
// off square by the step's travel over the circle's radius, which the
// margin and the checks below take.

// draftStep says at which corners, with the curve before it and the
// curve after it in a loop's order, a curve's inside steps so.
type draftStep struct{ prev, next bool }

// keptSide is the side kept beside a step, a line or an arc, with the
// corner at its end (lineFirst) or start: where along it lies toward
// the corner and into its material, at a point of it moved or not.
type keptSide struct {
	p         profilePiece
	lineFirst bool
	n         geom.Vector
}

// toward is the direction along the side at q toward the corner.
func (k keptSide) toward(q geom.Vector) geom.Vector {
	if !k.p.arc {
		e := k.p.b.Minus(k.p.a).WithMagnitude(1)
		if !k.lineFirst {
			e = e.ScaledBy(-1)
		}
		return e
	}
	t := pieceTangentAt(k.p, q.Minus(k.p.centre).WithMagnitude(1), k.n)
	if !k.lineFirst {
		t = t.ScaledBy(-1)
	}
	return t
}

// into is the direction at q into the side's material.
func (k keptSide) into(q geom.Vector) geom.Vector {
	if !k.p.arc {
		return k.p.into
	}
	r := q.Minus(k.p.centre).WithMagnitude(1)
	if !k.p.outward {
		r = r.ScaledBy(-1)
	}
	return r
}

// back is the point of the side moved by d (movedPiece) a length l back
// from c on it, away from the corner.
func (k keptSide) back(d float64, c geom.Vector, l float64) geom.Vector {
	m := movedPiece(k.p, d)
	if !m.arc {
		return c.Minus(k.toward(c).ScaledBy(l))
	}
	// Back along the arc: against its travel when the corner is its end.
	turn := l / m.r
	if (m.angle > 0) == k.lineFirst {
		turn = -turn
	}
	r := c.Minus(m.centre)
	u := r.WithMagnitude(1)
	v := k.n.WithMagnitude(1).Cross(u)
	return m.centre.Plus(u.ScaledBy(m.r * math.Cos(turn))).Plus(v.ScaledBy(m.r * math.Sin(turn)))
}

// inMaterial reports whether p lies in the side's material, moved by d,
// by more than LengthEps: below a line, on the material's side of a
// circle.
func (k keptSide) inMaterial(p geom.Vector, d float64) bool {
	m := movedPiece(k.p, d)
	if !m.arc {
		return p.Minus(m.a).Dot(m.into) > geom.LengthEps
	}
	dist := p.Minus(m.centre).Magnitude()
	if m.outward {
		return dist > m.r+geom.LengthEps
	}
	return dist < m.r-geom.LengthEps
}

// length is how long the side is on the neutral plane.
func (k keptSide) length() float64 {
	if !k.p.arc {
		return k.p.b.Minus(k.p.a).Magnitude()
	}
	return k.p.r * math.Abs(k.p.angle)
}

// draftStepAt is the inside's way round the inside corner between the
// kept line or arc pieces[iLine] and the curved piece pieces[iArc]
// taken away, for the inside drafted from distance from to distance to
// along the way n (pieces are on the neutral plane; as are their angles,
// ds how far each moves at from). lineFirst says the kept piece comes
// before the arc. lineEnd is where the moved kept piece ends, arcStart
// where the moved arc starts (the other way round when the arc is
// first), and curves the pieces between, in the loop's order, at the
// neutral plane, with their angles; short is the most by which the wall
// under the kept piece stops short of the corner, at the inside's foot
// and head.
func draftStepAt(pieces []profilePiece, as, ds []float64, iLine, iArc int, lineFirst bool, n geom.Vector,
	from, to, thick float64, tol srf.Tolerance) (lineEnd, arcStart geom.Vector, curves []srf.SBezier, angles []float64, short float64, err error) {
	L, A := pieces[iLine], pieces[iArc]
	K := keptSide{p: L, lineFirst: lineFirst, n: n}
	tanL, tanA := math.Tan(as[iLine]), math.Tan(as[iArc])
	v := L.b
	if !lineFirst {
		v = L.a
	}
	fail := func() (geom.Vector, geom.Vector, []srf.SBezier, []float64, float64, error) {
		return v, v, nil, nil, 0, shellErr(v, "a curved face taken away meets a face kept at an inside corner "+
			"too tight for the inside to step round; a shell opens a drafted extrusion's curved faces "+
			"at an inside corner only beside a face kept with room for the step")
	}
	// The solid's corner at distance s from the neutral plane.
	cornerAt := func(s float64) (geom.Vector, bool) {
		pl, pa := movedPiece(L, s*tanL), movedPiece(A, s*tanA)
		p, q, p0, q0 := pl, pa, L, A
		if !lineFirst {
			p, q, p0, q0 = pa, pl, A, L
		}
		c, err := meetMoved(p, q, v, p0, q0, 0, n, "")
		return c, err == nil
	}
	c0, ok := cornerAt(from)
	if !ok {
		return fail()
	}
	if _, ok := cornerAt(to); !ok {
		return fail()
	}
	h := to - from
	wL := thick / math.Cos(as[iLine])
	margin := max(0.02*wL, 4*tol.ChordTolMm(), 100*geom.LengthEps)
	const samples = 64
	// The step goes along e, the way toward the corner at its foot, from
	// foot, back from the corner at from; where the corner is at s, along
	// e from the foot, and how far the step's chord stands off its path.
	// The foot is found from the corner's own way at first, then again
	// from the foot's.
	e := K.toward(c0)
	var foot geom.Vector
	var slope, sag float64
	along := func(s float64) (float64, bool) {
		c, ok := cornerAt(s)
		return c.Minus(c0).Dot(e), ok
	}
	findFoot := func(back float64) {
		foot = K.back(from*tanL, c0, back)
		e = K.toward(foot)
	}
	for range 2 {
		u1, _ := along(to)
		slope = u1 / h
		sag = 0.0
		for k := 1; k < samples; k++ {
			s := from + h*float64(k)/samples
			u, ok := along(s)
			if !ok {
				return fail()
			}
			sag = max(sag, slope*(s-from)-u)
		}
		if sag+margin > 0.5*K.length() {
			return fail()
		}
		findFoot(sag + margin)
	}
	// Beside a round kept side of radius r the step slides along the
	// tangent at its foot, and the way outside the side, a plane leaning
	// with it, keeps its distance from the side's circle where it touches
	// the circle moved out; but over, where the way meets the step, has
	// slid x along it, which stands off the circle by x²/2r more, away
	// from the side's material, toward the arc's. Beside a hollow side it
	// would stand into the material: there the way follows the circle
	// (shelldraft_hollow.go), and over stays on it.
	bend := 0.0
	if L.arc && !K.hollowSide() {
		bend = 1 / (2 * movedPiece(L, from*tanL).r)
	}
	// The way outside the kept side leaves the step a little outside it,
	// within the wedge outside the corner, between the kept side going
	// back from it and the arc leaving it: as far as the narrowest of the
	// wedge and of how far the step stands back from the corner, going
	// up, less how far over strays out, allow. Where the wedge allows too
	// little, the step is set further back.
	var into geom.Vector
	var back, lift float64
	cornerPos := func(s float64) (float64, bool) {
		c, ok := cornerAt(s)
		return c.Minus(foot).Dot(e), ok
	}
	back = sag + margin
	for try := 0; ; try++ {
		if back > 0.5*K.length() || try == 8 {
			return fail()
		}
		findFoot(back)
		into = K.into(foot)
		room, rate := math.Inf(1), math.Inf(1)
		for k := 0; k <= samples; k++ {
			s := from + h*float64(k)/samples
			c, ok := cornerAt(s)
			if !ok {
				return fail()
			}
			tA := n.WithMagnitude(1).Cross(c.Minus(A.centre).WithMagnitude(1))
			if (A.angle < 0) == lineFirst {
				tA = tA.ScaledBy(-1)
			}
			ks := keptSide{p: movedPiece(L, s*tanL), lineFirst: lineFirst, n: n}
			eC, inC := ks.toward(c), ks.into(c)
			wedge := math.Atan2(-tA.Dot(inC), -tA.Dot(eC))
			if wedge <= 1e-3 {
				return fail()
			}
			pos, _ := cornerPos(s)
			x := slope * (s - from)
			stand := pos - x
			if stand <= 0 {
				return fail()
			}
			f := 0.5 * math.Min(1, math.Tan(wedge/2))
			room = min(room, stand*f-bend*x*x)
			rate = min(rate, f)
		}
		need := max(0.5*rate*margin, 10*geom.LengthEps)
		if room >= need {
			lift = room
			break
		}
		back += 1.5 * (need - room) / rate
	}

	// At from: under on the moved kept side, over outside it, and where
	// the way along it from over meets the arc run out: back from the
	// corner, where over lies within the arc's circle run out, as it does
	// beside a lobe's, or toward the corner where the step is set back
	// past where that circle crosses the kept side.
	under := foot.Plus(into.ScaledBy(ds[iLine] - from*tanL))
	over := foot.Minus(into.ScaledBy(lift))
	nearest := func(o geom.Vector, cs []geom.Vector) (geom.Vector, bool) {
		var best geom.Vector
		bt := 0.0
		found := false
		for _, q := range cs {
			t := q.Minus(o).Dot(e.ScaledBy(-1))
			if t > geom.LengthEps && (!found || t < bt) {
				best, bt, found = q, t, true
			}
		}
		if found {
			return best, true
		}
		for _, q := range cs {
			t := q.Minus(o).Dot(e.ScaledBy(-1))
			if t < -geom.LengthEps && (!found || t > bt) {
				best, bt, found = q, t, true
			}
		}
		return best, found
	}
	ma := movedPiece(A, ds[iArc])
	var out geom.Vector
	var found bool
	var turn float64
	if K.hollowSide() {
		out, turn, found = K.hollowWayEnd(over, e, ma.centre, ma.r)
	} else {
		out, found = nearest(over, lineCircle(over, e.ScaledBy(-1), ma.centre, ma.r))
	}
	if !found {
		return fail()
	}
	// The way must stay clear of the solid at every height: over outside
	// the kept side and outside the arc, and so the way back.
	inArc := func(p geom.Vector, s float64) bool {
		r := movedPiece(A, s*tanA).r
		d := p.Minus(A.centre).Magnitude()
		if A.outward {
			return d > r-geom.LengthEps
		}
		return d < r+geom.LengthEps
	}
	for k := range 17 {
		s := from + h*float64(k)/16
		pos, ok := cornerPos(s)
		if !ok || slope*(s-from) > pos-margin/2 {
			return fail()
		}
		// over and out moved as the inside's prism moves them: along e
		// with the step, square to it with the kept side.
		dv := into.ScaledBy((s - from) * tanL)
		ov := over.Plus(e.ScaledBy(slope * (s - from))).Plus(dv)
		mo := movedPiece(A, ds[iArc]+(s-from)*tanA)
		var end geom.Vector
		var found bool
		var tw float64
		if K.hollowSide() {
			ov, end, tw, found = K.hollowWayAt(ov, e, s*tanL, lift, mo.centre, mo.r)
			// The way may not turn round or shrink to nothing going up.
			found = found && tw*turn > 0 && math.Abs(tw)*K.wayRadius(s*tanL, lift) > 10*geom.LengthEps
		} else {
			end, found = nearest(ov, lineCircle(ov, e.ScaledBy(-1), mo.centre, mo.r))
		}
		if !found {
			return fail()
		}
		for j := range 9 {
			p := ov.Plus(end.Minus(ov).ScaledBy(float64(j) / 8))
			if K.hollowSide() {
				p = K.hollowWayPoint(ov, tw, float64(j)/8)
			}
			if inArc(p, s) || K.inMaterial(p, s*tanL) {
				return fail()
			}
		}
	}

	// In the loop's order, each piece's material on its left: the step
	// leans with the corner along e, the way back with the kept side.
	leanOf := func(a, b, dir geom.Vector, rate float64) float64 {
		left := n.Cross(b.Minus(a)).WithMagnitude(1)
		return math.Atan(rate * left.Dot(dir))
	}
	if K.hollowSide() {
		wc, wa := K.hollowWayCurves(over, out, turn, as[iLine])
		if lineFirst {
			curves = append([]srf.SBezier{srf.SBezierFrom2(under, over)}, wc...)
			angles = append([]float64{leanOf(under, over, e, slope)}, wa...)
			return under, out, curves, angles, back, nil
		}
		curves, angles = wc, wa
		curves = append(curves, srf.SBezierFrom2(over, under))
		angles = append(angles, leanOf(over, under, e, slope))
		return under, out, curves, angles, back, nil
	}
	if lineFirst {
		curves = []srf.SBezier{srf.SBezierFrom2(under, over), srf.SBezierFrom2(over, out)}
		angles = []float64{leanOf(under, over, e, slope), leanOf(over, out, into, tanL)}
		return under, out, curves, angles, back, nil
	}
	curves = []srf.SBezier{srf.SBezierFrom2(out, over), srf.SBezierFrom2(over, under)}
	angles = []float64{leanOf(out, over, into, tanL), leanOf(over, under, e, slope)}
	return under, out, curves, angles, back, nil
}
