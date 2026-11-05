package topo

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Beside a hollow round side kept, an arc whose material lies outside
// its circle, the way outside the side can't be a plane: a line leaving
// the circle along its tangent runs into the material beyond by x²/2r,
// both where the step has slid up along the tangent and along the way
// back to the curved side run out, and the wedge outside the corner
// rarely leaves room for that. The way is instead a cone about the
// side's axis, the side's circle moved out by the lift and drafted as
// the side is, so it keeps its distance from the side at every height
// and wherever the step crosses it.

// hollowSide reports whether k is a round side whose material lies
// outside its circle, which hollowWay follows.
func (k keptSide) hollowSide() bool { return k.p.arc && k.p.outward }

// wayRadius is the radius of the way outside the hollow side k, moved
// by d (movedPiece), the lift outside it.
func (k keptSide) wayRadius(d, lift float64) float64 { return movedPiece(k.p, d).r - lift }

// hollowWayEnd is where the way about the hollow side's centre, through
// o, meets the circle about c of radius r: the nearest crossing going
// back from the corner (against e at o) within half a turn, or else the
// nearest going toward it, where the step is set back past where that
// circle crosses the way; with the way's turn about n.
func (k keptSide) hollowWayEnd(o, e, c geom.Vector, r float64) (geom.Vector, float64, bool) {
	ro := o.Minus(k.p.centre)
	rw := ro.Magnitude()
	nn := k.n.WithMagnitude(1)
	// back is the sign of the turn about n that goes against e.
	back := 1.0
	if nn.Cross(ro).Dot(e) > 0 {
		back = -1
	}
	cs := circleCircle(k.p.centre, rw, c, r, k.n)
	for _, way := range []float64{back, -back} {
		var best geom.Vector
		bt := math.Inf(1)
		for _, q := range cs {
			rq := q.Minus(k.p.centre)
			t := way * math.Atan2(ro.Cross(rq).Dot(nn), ro.Dot(rq))
			if t*rw > geom.LengthEps && t < bt {
				best, bt = q, t
			}
		}
		if !math.IsInf(bt, 1) {
			return best, way * bt, true
		}
	}
	return o, 0, false
}

// hollowWayAt is the way outside the hollow side k at a height where the
// side has moved by d and the step's crossing of the way lies near o:
// o put on the way's circle, and the way's end on the curved side's
// circle about c of radius r run out, with the way's turn about n.
func (k keptSide) hollowWayAt(o, e geom.Vector, d, lift float64, c geom.Vector, r float64) (start, end geom.Vector, turn float64, ok bool) {
	rw := k.wayRadius(d, lift)
	if rw <= geom.LengthEps {
		return o, o, 0, false
	}
	start = k.p.centre.Plus(o.Minus(k.p.centre).WithMagnitude(rw))
	end, turn, ok = k.hollowWayEnd(start, e, c, r)
	return start, end, turn, ok
}

// hollowWayPoint is the point a fraction f of the way from start, the
// way turning by turn about the side's centre.
func (k keptSide) hollowWayPoint(start geom.Vector, turn, f float64) geom.Vector {
	return start.Minus(k.p.centre).RotatedAroundAxis(k.n.WithMagnitude(1), turn*f).Plus(k.p.centre)
}

// hollowWayCurves is the way from over to out, turning by turn about
// the hollow side's centre, in the loop's order (lineFirst: from over),
// and its draft angle: the side's own, its material, the inside, lying
// on the far side from the side's.
func (k keptSide) hollowWayCurves(over, out geom.Vector, turn, sideAngle float64) ([]srf.SBezier, []float64) {
	a, t := over, turn
	if !k.lineFirst {
		a, t = out, -turn
	}
	cs := arcPieces(k.p.centre, a, t, k.n, 0)
	as := make([]float64, len(cs))
	for i := range as {
		as[i] = -sideAngle
	}
	return cs, as
}
