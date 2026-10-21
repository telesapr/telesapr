package core

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// An angle between two lines is held through its cosine, which can't
// tell a turn one way from the other, nor 10 degrees from 370, and
// whose slope vanishes at 0 and 180: no good for a part turning about an
// axis, as a nut on a screw. ConstraintAxisAngle holds the turn by a
// half-plane instead: the axis through PtA along normal EntityA, the
// half-plane through it at ValA degrees from the normal's u towards its
// v, and point PtB on that plane. The equation is linear in the point,
// and its slope as the part turns is the point's distance from the axis
// at every angle. The plane holds the point at ValA and at ValA+180;
// the solve starts where the point is, and keeps the side it is on.

// axisFrame is the constraint's axis point and its normal's u and v,
// as expressions.
func (c *Constraint) axisFrame(sk EntitySketch) (o, u, v ExprVector) {
	n := mustEntity(sk, c.EntityA)
	return mustEntity(sk, c.PtA).PointGetExprs(sk), n.NormalExprsU(sk), n.NormalExprsV(sk)
}

// axisAngleEquations appends the equation of a ConstraintAxisAngle
// constraint: the point's offset from the axis has no part across the
// half-plane.
func (c *Constraint) axisAngleEquations(sk EntitySketch, l *[]Equation) {
	o, u, v := c.axisFrame(sk)
	d := mustEntity(sk, c.PtB).PointGetExprs(sk).Minus(o)
	a := c.ValA * math.Pi / 180
	across := v.ScaledBy(NewConstant(math.Cos(a))).Minus(u.ScaledBy(NewConstant(math.Sin(a))))
	c.addEq(l, d.Dot(across), 0)
}

// AxisAngleNum is a ConstraintAxisAngle's angle where its point is
// now, in degrees: of those a whole turn apart, the one nearest ValA.
func (c *Constraint) AxisAngleNum(sk EntitySketch) float64 {
	n := mustEntity(sk, c.EntityA)
	o := mustEntity(sk, c.PtA).PointGetNum(sk)
	d := mustEntity(sk, c.PtB).PointGetNum(sk).Minus(o)
	a := math.Atan2(d.Dot(n.NormalV(sk)), d.Dot(n.NormalU(sk))) * 180 / math.Pi
	return a + 360*math.Round((c.ValA-a)/360)
}

// AxisAngleOffset is how far a ConstraintAxisAngle's point is from its
// axis, and the foot of that on the axis: what a drawing of it needs.
func (c *Constraint) AxisAngleOffset(sk EntitySketch) (foot geom.Vector, r float64) {
	n := mustEntity(sk, c.EntityA).NormalN(sk)
	o := mustEntity(sk, c.PtA).PointGetNum(sk)
	d := mustEntity(sk, c.PtB).PointGetNum(sk).Minus(o)
	foot = o.Plus(n.ScaledBy(d.Dot(n)))
	return foot, d.Minus(n.ScaledBy(d.Dot(n))).Magnitude()
}
