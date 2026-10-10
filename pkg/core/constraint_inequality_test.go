package core

import (
	"math"
	"testing"
)

// inequalitySketch is a line from (0, 0) to (10, 0) and a point at (x,
// y), in 3d, and the constraint of type typ on them with value v; a
// point on the segment has its param t at 0.
func inequalitySketch(typ ConstraintType, x, y, v float64) (*testSketch, *Constraint) {
	sk := newTestSketch()
	var next uint32 = 0x100
	point := func(x, y float64) HEntity {
		next++
		h := HEntity{V: next}
		sk.add(Entity{H: h, Type: EntityPointIn3D,
			Param: [8]HParam{sk.param(next<<4|1, x), sk.param(next<<4|2, y), sk.param(next<<4|3, 0)}})
		return h
	}
	a, b, p := point(0, 0), point(10, 0), point(x, y)
	line := HEntity{V: 0x20}
	sk.add(Entity{H: line, Type: EntityLineSegment, Point: [MaxPointsInEntity]HEntity{a, b}})
	c := &Constraint{}
	c.H, c.Type, c.Workplane, c.ValA = HConstraint{V: 1}, typ, NoEntity, v
	if typ == ConstraintPtOnSegment {
		c.PtA, c.EntityA = p, line
		c.ValP = c.H.Param(0)
		sk.param(c.ValP.V, 0)
	} else {
		c.PtA, c.PtB = a, p
	}
	return sk, c
}

// A bounded distance is crossed by how far the distance is past its
// value, and writes its equation only while held, which then holds at
// the bound.
func TestDistanceBoundCrossed(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		limit Limit
		x, y  float64
		v     float64
		by    float64
	}{
		{LimitMin, 30, 40, 20, -30},
		{LimitMin, 3, 4, 20, 15},
		{LimitMin, 12, 16, 20, 0},
		{LimitMax, 3, 4, 20, -15},
		{LimitMax, 30, 40, 20, 30},
	} {
		sk, c := inequalitySketch(ConstraintPtPtDistance, tc.x, tc.y, tc.v)
		c.Limit = tc.limit
		by, bound := c.Crossed(sk)
		if math.Abs(by-tc.by) > 1e-12 || bound != BoundHeld {
			t.Errorf("%v at (%g, %g): crossed by %g at %d, want %g", tc.limit, tc.x, tc.y, by, bound, tc.by)
		}
		if IsCrossed(by) != (tc.by > 0) || IsAtBound(by) != (tc.by == 0) {
			t.Errorf("%v at (%g, %g): crossed %v, at the bound %v", tc.limit, tc.x, tc.y, IsCrossed(by), IsAtBound(by))
		}
		var eqs []Equation
		c.GenerateEquations(sk, &eqs, false)
		if len(eqs) != 0 {
			t.Errorf("%v free: %d equations", tc.limit, len(eqs))
		}
		c.Bound = BoundHeld
		eqs = nil
		c.GenerateEquations(sk, &eqs, false)
		sign := -1.0
		if tc.limit == LimitMin {
			sign = 1
		}
		if len(eqs) != 1 || math.Abs(EvalExpr(sk, eqs[0].Expr)+tc.by*sign) > 1e-12 {
			t.Errorf("%v held: %d equations", tc.limit, len(eqs))
		}
	}
}

// Any dimension takes a limit, crossed by its own measure: a point's
// distance from a line, signed, and an angle, whose equation runs in
// its cosine; a constraint that is no dimension doesn't.
func TestOtherBoundsCrossed(t *testing.T) {
	t.Parallel()
	sk, c := inequalitySketch(ConstraintPtLineDistance, 3, 4, 0)
	c.PtA, c.PtB, c.EntityA = c.PtB, NoEntity, HEntity{V: 0x20}
	c.ModifyToSatisfy(sk)
	if math.Abs(c.ValA-4) > 1e-12 {
		t.Fatalf("the point is %g from the line", c.ValA)
	}
	c.ValA, c.Limit = 6, LimitMin
	if by, _ := c.Crossed(sk); math.Abs(by-2) > 1e-12 {
		t.Errorf("4 from a line, at least 6: crossed by %g", by)
	}

	sk, _ = inequalitySketch(ConstraintPtPtDistance, 0, 0, 0)
	var next uint32 = 0x200
	point := func(x, y float64) HEntity {
		next++
		h := HEntity{V: next}
		sk.add(Entity{H: h, Type: EntityPointIn3D,
			Param: [8]HParam{sk.param(next<<4|1, x), sk.param(next<<4|2, y), sk.param(next<<4|3, 0)}})
		return h
	}
	l1, l2 := HEntity{V: 0x30}, HEntity{V: 0x31}
	sk.add(Entity{H: l1, Type: EntityLineSegment, Point: [MaxPointsInEntity]HEntity{point(0, 0), point(1, 0)}})
	sk.add(Entity{H: l2, Type: EntityLineSegment, Point: [MaxPointsInEntity]HEntity{point(0, 0), point(1, 1)}})
	a := &Constraint{}
	a.H, a.Type, a.Workplane, a.EntityA, a.EntityB = HConstraint{V: 2}, ConstraintAngle, NoEntity, l1, l2
	a.ModifyToSatisfy(sk)
	if math.Abs(a.ValA-45) > 1e-9 {
		t.Fatalf("the angle is %g", a.ValA)
	}
	for _, tc := range []struct {
		limit   Limit
		v       float64
		crossed bool
	}{
		{LimitMin, 60, true}, {LimitMin, 30, false}, {LimitMax, 30, true}, {LimitMax, 60, false},
	} {
		a.ValA, a.Limit = tc.v, tc.limit
		if by, _ := a.Crossed(sk); IsCrossed(by) != tc.crossed {
			t.Errorf("45 degrees, %v %g: crossed by %g", tc.limit, tc.v, by)
		}
	}
	if (&ConstraintBase{Type: ConstraintHorizontal}).CanLimit() || !(&ConstraintBase{Type: ConstraintDiameter}).CanLimit() {
		t.Error("CanLimit")
	}
	if LimitMin.String() != "min" || LimitMax.String() != "max" || LimitNone.String() != "" {
		t.Error("Limit.String")
	}
}

// A point on a segment starts at its foot on the segment, between the
// ends; it is crossed by how far t is past the end it is nearer, and
// held at an end it writes t's equation too.
func TestPtOnSegment(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		x, y  float64
		t     float64
		bound int
	}{
		{2.5, 3, 0.25, BoundStart},
		{-4, 1, 0, BoundStart},
		{10, 0, 1, BoundEnd},
		{25, 1, 1, BoundEnd},
		{7, -2, 0.7, BoundEnd},
	} {
		sk, c := inequalitySketch(ConstraintPtOnSegment, tc.x, tc.y, 0)
		c.SeedParam(sk)
		if got := sk.Param(c.ValP).Val; math.Abs(got-tc.t) > 1e-12 {
			t.Errorf("at (%g, %g): t %g, want %g", tc.x, tc.y, got, tc.t)
		}
		if _, bound := c.Crossed(sk); bound != tc.bound {
			t.Errorf("at (%g, %g): nearer bound %d, want %d", tc.x, tc.y, bound, tc.bound)
		}
		for bound, want := range []int{3, 4, 4} {
			c.Bound = bound
			var eqs []Equation
			c.GenerateEquations(sk, &eqs, false)
			if len(eqs) != want {
				t.Errorf("held at %d: %d equations, want %d", bound, len(eqs), want)
			}
		}
	}
	sk, c := inequalitySketch(ConstraintPtOnSegment, 0, 0, 0)
	sk.Param(c.ValP).Val = 1.5
	if by, bound := c.Crossed(sk); by != 5 || bound != BoundEnd {
		t.Errorf("t 1.5: crossed by %g at %d, want 5 at the end", by, bound)
	}
	sk.Param(c.ValP).Val = -0.2
	if by, bound := c.Crossed(sk); by != 2 || bound != BoundStart {
		t.Errorf("t -0.2: crossed by %g at %d, want 2 at the start", by, bound)
	}
}

// The inequalities are named and known as such; one made to satisfy
// the sketch takes the distance as it is, and a reference one writes
// no equation.
func TestInequalityKinds(t *testing.T) {
	t.Parallel()
	for _, c := range []ConstraintBase{
		{Type: ConstraintPtPtDistance, Limit: LimitMin}, {Type: ConstraintAngle, Limit: LimitMax}, {Type: ConstraintPtOnSegment},
	} {
		if !c.IsInequality() {
			t.Errorf("%v isn't an inequality", c.Type)
		}
	}
	if c := (ConstraintBase{H: HConstraint{V: 3}, Type: ConstraintPtPtDistance, Limit: LimitMin}); c.DescriptionString() != "c003-min-pt-pt-distance" {
		t.Errorf("described as %q", c.DescriptionString())
	}
	if (&ConstraintBase{Type: ConstraintPtOnLine}).IsInequality() {
		t.Error("a point on a line is an inequality")
	}

	sk, c := inequalitySketch(ConstraintPtPtDistance, 3, 4, 20)
	c.Limit = LimitMin
	c.ModifyToSatisfy(sk)
	if c.ValA != 5 {
		t.Errorf("a minimum distance made to satisfy the sketch is %g, want 5", c.ValA)
	}
	c.Reference, c.Bound = true, BoundHeld
	var eqs []Equation
	c.GenerateEquations(sk, &eqs, false)
	if len(eqs) != 0 {
		t.Errorf("a reference minimum writes %d equations", len(eqs))
	}
	if by, bound := (&Constraint{}).Crossed(sk); !math.IsInf(by, -1) || bound != BoundFree {
		t.Errorf("no inequality: crossed by %g at %d", by, bound)
	}

	sk, c = inequalitySketch(ConstraintPtOnSegment, 3, 4, 0)
	c.ModifyToSatisfy(sk)
	if got := sk.Param(c.ValP).Val; got != 0.3 {
		t.Errorf("a point on a segment made to satisfy the sketch is at t %g, want 0.3", got)
	}
	c.ValP = HParam{V: 0x7777}
	c.SeedParam(sk) // no such param: nothing to do
	if by, _ := c.Crossed(sk); !math.IsInf(by, -1) {
		t.Errorf("no param: crossed by %g", by)
	}
}
