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

// The axis is a normal, its u and v the frame the angle is counted in,
// or a line segment: the axis along it, through PtA, which a hinge's
// pin is. A line has no frame of its own, so one is made square to it
// (geom.Vector.NormalDir) where the line is drawn in its part: a line
// of a linked part, both its ends the part's points, keeps that frame
// as the part turns, by the part's turn (its quaternion), so that the
// angle is counted from the same side of the part at every turn. A line
// of anything else is taken where it is, as constants: a line of a
// group before, which the solve holds.

// axisLine reports whether the constraint's axis is a line segment,
// and its ends.
func (c *Constraint) axisLine(sk EntitySketch) (a, b *Entity, ok bool) {
	n := mustEntity(sk, c.EntityA)
	if n.Type != EntityLineSegment {
		return nil, nil, false
	}
	return mustEntity(sk, n.Point[0]), mustEntity(sk, n.Point[1]), true
}

// turnsWithPart reports whether a line from a to b is a line of one
// linked part, both its ends placed by the part's move and turn.
func turnsWithPart(a, b *Entity) bool {
	return a.Type == EntityPointNRotTrans && b.Type == EntityPointNRotTrans && a.Param[3] == b.Param[3]
}

// lineFrame is the frame square to the line from a to b where the line
// is drawn: in its part's own coordinates for a line of a part
// (turnsWithPart), else where it is now.
func lineFrame(sk EntitySketch, a, b *Entity) (u, v geom.Vector) {
	d := b.PointGetNum(sk).Minus(a.PointGetNum(sk))
	if turnsWithPart(a, b) {
		d = b.NumPoint.Minus(a.NumPoint)
	}
	d = d.WithMagnitude(1)
	return d.NormalDir(0), d.NormalDir(1)
}

// axisFrame is the constraint's axis point and its frame's u and v, as
// expressions.
func (c *Constraint) axisFrame(sk EntitySketch) (o, u, v ExprVector) {
	o = mustEntity(sk, c.PtA).PointGetExprs(sk)
	a, b, ok := c.axisLine(sk)
	if !ok {
		n := mustEntity(sk, c.EntityA)
		return o, n.NormalExprsU(sk), n.NormalExprsV(sk)
	}
	lu, lv := lineFrame(sk, a, b)
	if !turnsWithPart(a, b) {
		return o, FromVector(lu), FromVector(lv)
	}
	q := exprQuaternionFromParams(a.Param[3], a.Param[4], a.Param[5], a.Param[6])
	return o, q.Rotate(FromVector(lu)), q.Rotate(FromVector(lv))
}

// AxisFrameNum is a ConstraintAxisAngle's axis direction and the u and
// v its angle is counted in, where they are now, unit vectors: a
// normal's, or a line's (axisFrame).
func (c *Constraint) AxisFrameNum(sk EntitySketch) (n, u, v geom.Vector) {
	a, b, ok := c.axisLine(sk)
	if !ok {
		e := mustEntity(sk, c.EntityA)
		return e.NormalN(sk), e.NormalU(sk), e.NormalV(sk)
	}
	u, v = lineFrame(sk, a, b)
	if turnsWithPart(a, b) {
		q := a.PointGetQuaternion(sk)
		q = q.ScaledBy(1 / q.Magnitude())
		u, v = q.Rotate(u), q.Rotate(v)
	}
	return u.Cross(v), u, v
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
	_, u, v := c.AxisFrameNum(sk)
	o := mustEntity(sk, c.PtA).PointGetNum(sk)
	d := mustEntity(sk, c.PtB).PointGetNum(sk).Minus(o)
	a := math.Atan2(d.Dot(v), d.Dot(u)) * 180 / math.Pi
	return a + 360*math.Round((c.ValA-a)/360)
}

// AxisAngleOffset is how far a ConstraintAxisAngle's point is from its
// axis, and the foot of that on the axis: what a drawing of it needs.
func (c *Constraint) AxisAngleOffset(sk EntitySketch) (foot geom.Vector, r float64) {
	n, _, _ := c.AxisFrameNum(sk)
	o := mustEntity(sk, c.PtA).PointGetNum(sk)
	d := mustEntity(sk, c.PtB).PointGetNum(sk).Minus(o)
	foot = o.Plus(n.ScaledBy(d.Dot(n)))
	return foot, d.Minus(n.ScaledBy(d.Dot(n))).Magnitude()
}
