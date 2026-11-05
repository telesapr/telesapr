package topo

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A drafted extrusion's curved side taken away runs out past the solid
// by the thickness. Where it meets a kept flat side at an inside
// corner, as a round lobe on a block's edge does, running out would cut
// into the wall under the flat side. The corner, where the flat side's
// leaning plane meets the curved side's cone, moves along a conic going
// up, so no plane of a drafted prism follows it; a revolve's inside
// steps through its corner (shellrevolve_step.go), a drafted one can't.
// Instead the wall under the flat side ends a little short of the
// corner: the inside crosses the flat side on a plane square to it,
// leaning to follow the corner's chord from the inside's foot to its
// head, set back from the conic by its sag and a margin, so it cuts the
// flat side cleanly and never the curved one. Above the flat side it
// turns back along a plane parallel to it, a little outside, to the
// curved side run out. Both planes are drafted, so the inside stays one
// drafted prism.

// draftStep says at which corners, with the curve before it and the
// curve after it in a loop's order, a curve's inside steps so.
type draftStep struct{ prev, next bool }

// draftStepAt is the inside's way round the inside corner between the
// kept line pieces[iLine] and the curved piece pieces[iArc] taken away,
// for the inside drafted from distance from to distance to along the
// way n (pieces are on the neutral plane; as are their angles, ds how
// far each moves at from). lineFirst says the line comes before the
// arc. lineEnd is where the moved line ends, arcStart where the moved
// arc starts (the other way round when the arc is first), and curves
// the pieces between, in the loop's order, at the neutral plane, with
// their angles; short is the most by which the wall under the line
// stops short of the corner, at the inside's foot and head.
func draftStepAt(pieces []profilePiece, as, ds []float64, iLine, iArc int, lineFirst bool, n geom.Vector,
	from, to, thick float64, tol srf.Tolerance) (lineEnd, arcStart geom.Vector, curves []srf.SBezier, angles []float64, short float64, err error) {
	L, A := pieces[iLine], pieces[iArc]
	tanL, tanA := math.Tan(as[iLine]), math.Tan(as[iArc])
	v := L.b
	eOut := L.b.Minus(L.a).WithMagnitude(1)
	if !lineFirst {
		v, eOut = L.a, eOut.ScaledBy(-1)
	}
	into := L.into
	fail := func() (geom.Vector, geom.Vector, []srf.SBezier, []float64, float64, error) {
		return v, v, nil, nil, 0, shellErr(v, "a curved face taken away meets a flat face kept at an inside corner "+
			"too tight for the inside to step round; a shell opens a drafted extrusion's curved faces "+
			"at an inside corner only beside a flat face, with room for the step")
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
	c1, ok := cornerAt(to)
	if !ok {
		return fail()
	}
	h := to - from
	slope := c1.Minus(c0).Dot(eOut) / h
	// How far the conic falls short of its chord, going toward the
	// corner along the line, at its worst.
	const samples = 64
	sag := 0.0
	for k := 1; k < samples; k++ {
		s := from + h*float64(k)/samples
		c, ok := cornerAt(s)
		if !ok {
			return fail()
		}
		sag = max(sag, slope*(s-from)-c.Minus(c0).Dot(eOut))
	}
	wL := thick / math.Cos(as[iLine])
	margin := max(0.02*wL, 4*tol.ChordTolMm(), 100*geom.LengthEps)
	back := sag + margin
	if back > 0.5*L.b.Minus(L.a).Magnitude() {
		return fail()
	}
	// The way above the line leaves the step a little above it, within
	// the wedge outside the corner, between the line going back from it
	// and the arc leaving it: as high as the narrowest of the wedge and
	// of how far the step stands back from the corner, going up, allow.
	lift := math.Inf(1)
	for k := 0; k <= samples; k++ {
		s := from + h*float64(k)/samples
		c, _ := cornerAt(s)
		tA := n.WithMagnitude(1).Cross(c.Minus(A.centre).WithMagnitude(1))
		if (A.angle < 0) == lineFirst {
			tA = tA.ScaledBy(-1)
		}
		wedge := math.Atan2(-tA.Dot(into), -tA.Dot(eOut))
		if wedge <= 1e-3 {
			return fail()
		}
		stand := back - (slope*(s-from) - c.Minus(c0).Dot(eOut))
		lift = min(lift, 0.5*stand*math.Min(1, math.Tan(wedge/2)))
	}

	// At from: under on the moved line, over above the line, and where the
	// way back along the line meets the arc run out.
	foot := c0.Minus(eOut.ScaledBy(back))
	under := foot.Plus(into.ScaledBy(ds[iLine] - from*tanL))
	over := foot.Minus(into.ScaledBy(lift))
	ma := movedPiece(A, ds[iArc])
	cands := lineCircle(over, eOut.ScaledBy(-1), ma.centre, ma.r)
	var out geom.Vector
	found := false
	for _, q := range cands {
		t := q.Minus(over).Dot(eOut.ScaledBy(-1))
		if t > geom.LengthEps && (!found || t < out.Minus(over).Dot(eOut.ScaledBy(-1))) {
			out, found = q, true
		}
	}
	if !found {
		return fail()
	}
	// The way must stay clear of the solid at every height: over above
	// the line and outside the arc, and so the way back along the line.
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
		c, _ := cornerAt(s)
		cut := slope*(s-from) - back // where the step crosses the line, from c0
		if cut > c.Minus(c0).Dot(eOut)-margin/2 {
			return fail()
		}
		// over and out moved as the inside's prism moves them: along the
		// line with the step, square to it with the line.
		dv := into.ScaledBy((s - from) * tanL)
		ov := over.Plus(eOut.ScaledBy(slope * (s - from))).Plus(dv)
		mo := movedPiece(A, ds[iArc]+(s-from)*tanA)
		cs := lineCircle(ov, eOut.ScaledBy(-1), mo.centre, mo.r)
		if len(cs) == 0 {
			return fail()
		}
		end := cs[0]
		for _, q := range cs[1:] {
			if q.Minus(ov).Dot(eOut.ScaledBy(-1)) > 0 && (end.Minus(ov).Dot(eOut.ScaledBy(-1)) <= 0 ||
				q.Minus(ov).Dot(eOut.ScaledBy(-1)) < end.Minus(ov).Dot(eOut.ScaledBy(-1))) {
				end = q
			}
		}
		for j := range 9 {
			if p := ov.Plus(end.Minus(ov).ScaledBy(float64(j) / 8)); inArc(p, s) {
				return fail()
			}
		}
	}

	// In the loop's order, each piece's material on its left: the step
	// leans with the corner along the line, the way back with the line.
	leanOf := func(a, b, dir geom.Vector, rate float64) float64 {
		left := n.Cross(b.Minus(a)).WithMagnitude(1)
		return math.Atan(rate * left.Dot(dir))
	}
	if lineFirst {
		curves = []srf.SBezier{srf.SBezierFrom2(under, over), srf.SBezierFrom2(over, out)}
		angles = []float64{leanOf(under, over, eOut, slope), leanOf(over, out, into, tanL)}
		return under, out, curves, angles, back, nil
	}
	curves = []srf.SBezier{srf.SBezierFrom2(out, over), srf.SBezierFrom2(over, under)}
	angles = []float64{leanOf(out, over, into, tanL), leanOf(over, under, eOut, slope)}
	return under, out, curves, angles, back, nil
}
