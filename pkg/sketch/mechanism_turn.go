package sketch

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// turnLever is the length, in mm, a radian of a turn pull's aim is
// weighed by against a millimetre of its anchor's: a turn the mechanism
// allows is made whole, and the anchor kept where it can be. Where the
// mechanism lets the part turn only about another axis than the one it
// is turned about, as a lid on its hinge turned about a point of its
// front, the anchor can't stay and the turn is made by its share about
// the hinge: weighed alike, the anchor would hold the turn back by
// r²/(l²+r²), a part in a thousand at 300 mm.
const turnLever = 1e4

// turnReach is the length, in mm, a step of a turn pull may move a part
// by for each radian it has still to turn: pullPart bounds a step's move
// by twice the way left to go, and a turn has no way of its own.
const turnReach = 100

// turnGoal aims a part, placed by the four params q of its turn (w, x,
// y, z, a link group's params 3 to 6), at the turn to, and where anchor
// is a point of it, that point at at: a turn about the anchor.
type turnGoal struct {
	q      [4]core.HParam
	to     geom.Quaternion
	anchor core.HEntity
	at     geom.Vector
}

// turnNow is the part's turn at the params' values now.
func (g turnGoal) turnNow(sk *Sketch) geom.Quaternion {
	v := func(i int) float64 { return sk.MustParam(g.q[i]).Val }
	return geom.Quaternion{W: v(0), X: v(1), Y: v(2), Z: v(3)}.WithMagnitude(1)
}

// rotationVector is the axis of the turn q, a unit quaternion, scaled by
// its angle in radians, taken the short way round.
func rotationVector(q geom.Quaternion) geom.Vector {
	if q.W < 0 {
		q = q.ScaledBy(-1)
	}
	v := geom.Vector{X: q.X, Y: q.Y, Z: q.Z}
	s := v.Magnitude()
	if s < 1e-300 {
		return geom.Vector{}
	}
	return v.ScaledBy(2 * math.Atan2(s, q.W) / s)
}

func (g turnGoal) residual(sk *Sketch) ([]float64, float64) {
	rho := rotationVector(g.to.Times(g.turnNow(sk).Inverse()))
	b := []float64{turnLever * rho.X, turnLever * rho.Y, turnLever * rho.Z}
	reach := turnReach * rho.Magnitude()
	if g.anchor.V != 0 {
		r := g.at.Minus(sk.Entity(g.anchor).PointGetNum(sk))
		b = append(b, r.X, r.Y, r.Z)
		reach += r.Magnitude()
	}
	return b, reach
}

// rows are, for the turn, the part's angular velocity ω for a change dq
// of its quaternion q, ω = 2·vec(dq·q⁻¹), weighed by turnLever; then the
// anchor's coordinates, as a point pull's.
func (g turnGoal) rows(sk *Sketch, hs []core.HParam) [][]float64 {
	q := g.turnNow(sk)
	// d(ω) / d(w, x, y, z), a row for each of ω's components.
	dw := [3][4]float64{
		{-q.X, q.W, -q.Z, q.Y},
		{-q.Y, q.Z, q.W, -q.X},
		{-q.Z, -q.Y, q.X, q.W},
	}
	a := make([][]float64, 3)
	for i := range 3 {
		a[i] = make([]float64, len(hs))
		for j, h := range hs {
			for c, hq := range g.q {
				if h == hq {
					a[i][j] = 2 * turnLever * dw[i][c]
				}
			}
		}
	}
	if g.anchor.V != 0 {
		a = append(a, pointGoal{he: g.anchor, to: g.at}.rows(sk, hs)...)
	}
	return a
}

// placement are the seven params that place link group hg: its move and
// its turn.
func placement(hg core.HGroup) []core.HParam {
	out := make([]core.HParam, 7)
	for i := range out {
		out[i] = hg.Param(i)
	}
	return out
}

// turnable reports whether normal he turns its part along the part's
// mechanism's freedom: a part's normal, turned by the part's turn, of a
// part that moves in a mechanism that is solved.
func (sk *Sketch) turnable(he core.HEntity) bool {
	e := sk.Entity(he)
	if e == nil || e.Type != core.EntityNormalNRot {
		return false
	}
	g := sk.Group(e.Group)
	return movesInMechanism(g) && g.IsSolvedOkay()
}

// pullNormalTo turns the part normal he is of so that the normal turns
// to q, along the mechanism's freedom (pullPart): the part turns as its
// mates let it, and the parts joined to it with it.
func (sk *Sketch) pullNormalTo(he core.HEntity, q geom.Quaternion) {
	n := sk.Entity(he)
	by := q.Times(n.NormalGetNum(sk).Inverse())
	g := turnGoal{q: [4]core.HParam{n.Param[0], n.Param[1], n.Param[2], n.Param[3]}}
	g.to = by.Times(g.turnNow(sk))
	sk.pullPart(n.Group, placement(n.Group), g)
}

// TurnPointBy turns the part that point he is a point of by r, about
// he, as Shift- or Ctrl-dragging the point in the window does, and marks
// it to be solved again. A part that moves in a mechanism turns along
// the mechanism's freedom: as near the turn as its mates let it, about
// he where they let it, the parts joined to it turning and sliding with
// it; a lid on its hinge turned about a point of its front turns about
// its hinge, one on a driven hinge turns its drive with it, within the
// drive's bounds, as a pull does, and a part with no freedom stays.
// Any other part is put
// where it is turned and then solved. Only a point placed by its part's
// move and turn, a point of a linked part, can be turned so; for any
// other it returns false and changes nothing.
func (sk *Sketch) TurnPointBy(he core.HEntity, r geom.Quaternion) bool {
	e := sk.Entity(he)
	if e == nil || e.Type != core.EntityPointNRotTrans || !(r.Magnitude() > 0) {
		return false
	}
	r = r.WithMagnitude(1)
	p := e.PointGetNum(sk)
	if sk.pullable(he) {
		g := turnGoal{q: [4]core.HParam{e.Param[3], e.Param[4], e.Param[5], e.Param[6]}, anchor: he, at: p}
		g.to = r.Times(g.turnNow(sk))
		sk.pullPart(e.Group, sk.partParams(he), g)
		return true
	}
	e.PointForceQuaternionTo(sk, r.Times(e.PointGetQuaternion(sk)))
	// About the point: the move put back so that it stays where it was.
	e.PointForceTo(sk, p)
	sk.MarkGroupDirtyByEntity(he)
	return true
}
