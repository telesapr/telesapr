package core

import "math"

// An inequality is a dimension with a Limit, or a point on a segment,
// and is solved by an active set. While it holds it writes
// no equation, and leaves the sketch every degree of freedom it had;
// where the sketch would cross it, the solve holds it at its bound as
// an equality, which takes one. Which bound holds, if any, is the
// constraint's Bound, which the sketch works out at each solve
// (Sketch.SolveGroupAndReport): it solves with none held, then holds
// the worst crossed one and solves again, until none is crossed.
//
// A point on a segment is on its line at the constraint's own param t
// (ValP), as a point on a line is; held at an end, t is 0 or 1.

// Limit is which side of its value a bounded dimension keeps to.
type Limit int

// Limits a dimension can have.
const (
	LimitNone Limit = iota
	LimitMin
	LimitMax
)

// String names the limit as files and descriptions do.
func (l Limit) String() string {
	switch l {
	case LimitMin:
		return "min"
	case LimitMax:
		return "max"
	}
	return ""
}

// CanLimit reports whether the constraint is a dimension that can be
// a bound: a distance of any kind, a diameter or radius, an angle.
func (c *ConstraintBase) CanLimit() bool {
	switch c.Type {
	case ConstraintPtPtDistance, ConstraintProjPtDistance, ConstraintPtLineDistance,
		ConstraintPtPlaneDistance, ConstraintPtFaceDistance, ConstraintDiameter, ConstraintAngle:
		return true
	}
	return false
}

// Bounds an inequality can be held at.
const (
	// BoundFree is an inequality that holds by itself.
	BoundFree = 0
	// BoundHeld is a bounded dimension held at its value.
	BoundHeld = 1
	// BoundStart and BoundEnd are a point held at a segment's ends.
	BoundStart = 1
	BoundEnd   = 2
)

// atBound is how near its bound an inequality may be and still count
// as at it: the solver's tolerance on an equation. Held there it takes
// the freedom a bound takes, so that the freedom counted doesn't
// depend on which side of the bound rounding left the sketch.
const atBound = LengthEps / 1e2

// IsInequality reports whether the constraint is an inequality, held
// at its bound only where the sketch would cross it.
func (c *ConstraintBase) IsInequality() bool {
	return c.Type == ConstraintPtOnSegment || c.Limit != LimitNone
}

// ptOnSegmentEquations appends the equations of a ConstraintPtOnSegment
// constraint: the point on the line at t along it, and t at an end
// where the point is held there.
func (c *Constraint) ptOnSegmentEquations(sk EntitySketch, l *[]Equation) {
	c.ptOnLineEquations(sk, l)
	switch c.Bound {
	case BoundStart:
		c.addEq(l, NewParam(c.ValP), 3)
	case BoundEnd:
		c.addEq(l, NewParam(c.ValP).Minus(NewConstant(1)), 3)
	}
}

// Crossed is how far the sketch is past the inequality's bound, in
// mm, and the bound it is past or nearest: positive where it is
// crossed, and within atBound of zero at the bound (IsAtBound). For a
// point on a segment it reads t, and so is meant after a solve has put
// the point on the line.
func (c *Constraint) Crossed(sk EntitySketch) (by float64, bound int) {
	if c.Limit != LimitNone {
		// The dimension's equation is its measure less its value,
		// which is how far it is past a maximum; an angle's runs in
		// its cosine, which falls as the angle grows.
		var l []Equation
		c.GenerateEquations(sk, &l, true)
		if len(l) != 1 {
			return math.Inf(-1), BoundFree
		}
		by = EvalExpr(sk, l[0].Expr)
		if c.Type == ConstraintAngle {
			by = -by
		}
		if c.Limit == LimitMin {
			by = -by
		}
		// A negative value is a signed distance on the other side of
		// a line or a plane, and the bound is on its size: further
		// that way is more.
		if c.ValA < 0 {
			by = -by
		}
		return by, BoundHeld
	}
	p := sk.Param(c.ValP)
	if c.Type != ConstraintPtOnSegment || p == nil {
		return math.Inf(-1), BoundFree
	}
	ln := mustEntity(sk, c.EntityA)
	length := EvalExpr(sk, DistanceExpr(sk, c.Workplane, ln.Point[0], ln.Point[1]))
	if p.Val < 0.5 {
		return -p.Val * length, BoundStart
	}
	return (p.Val - 1) * length, BoundEnd
}

// IsCrossed reports whether a crossing of by, as Crossed measures it,
// is past the bound by more than the solver's tolerance.
func IsCrossed(by float64) bool { return by > atBound }

// IsAtBound reports whether a crossing of by, as Crossed measures it,
// leaves the inequality at its bound, within the solver's tolerance.
func IsAtBound(by float64) bool { return math.Abs(by) <= atBound }

// SeedParam gives a point on a segment its place along it, nearest the
// point and between the ends, so that the first solve moves it no more
// than it must. It does nothing for other constraints, or before the
// param is made.
func (c *Constraint) SeedParam(sk EntitySketch) {
	if c.Type != ConstraintPtOnSegment || sk.Param(c.ValP) == nil {
		return
	}
	ln := mustEntity(sk, c.EntityA)
	ea := mustEntity(sk, ln.Point[0]).PointGetExprsInWorkplane(sk, c.Workplane)
	eb := mustEntity(sk, ln.Point[1]).PointGetExprsInWorkplane(sk, c.Workplane)
	ep := mustEntity(sk, c.PtA).PointGetExprsInWorkplane(sk, c.Workplane)
	ab := eb.Minus(ea)
	t := 0.0
	if len2 := EvalExpr(sk, ab.Dot(ab)); len2 > 0 {
		t = EvalExpr(sk, ab.Dot(ep.Minus(ea))) / len2
	}
	setParamVal(sk, c.ValP, min(max(t, 0), 1))
}
