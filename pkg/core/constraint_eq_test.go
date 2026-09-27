package core

import (
	"math"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

// TestArcLengthConstraints covers the four arc-length constraints, which
// no fixture uses: a quarter arc and a half arc of radius 1, and a
// line of length 2. ModifyToSatisfy must recover each value from the
// equation, and the equation must then hold.
func TestArcLengthConstraints(t *testing.T) {
	sk := newTestSketch()
	var next uint32 = 0x100
	point := func(x, y float64) HEntity {
		next++
		h := HEntity{V: next}
		sk.add(Entity{EntityBase: EntityBase{H: h, Type: EntityPointIn3D,
			Param: [8]HParam{sk.param(next<<4|1, x), sk.param(next<<4|2, y), sk.param(next<<4|3, 0)}}})
		return h
	}
	sk.add(Entity{EntityBase: EntityBase{H: HEntity{V: 0x10}, Type: EntityNormalNCopy, NumNormal: geom.QuaternionIdentity}})
	arc := func(v uint32, c, a, b HEntity) HEntity {
		h := HEntity{V: v}
		sk.add(Entity{EntityBase: EntityBase{H: h, Type: EntityArcOfCircle,
			Point: [MaxPointsInEntity]HEntity{c, a, b}, Normal: HEntity{V: 0x10}}})
		return h
	}
	quarter := arc(0x20, point(0, 0), point(1, 0), point(0, 1))
	half := arc(0x21, point(5, 0), point(6, 0), point(4, 0))
	line := HEntity{V: 0x22}
	sk.add(Entity{EntityBase: EntityBase{H: line, Type: EntityLineSegment,
		Point: [MaxPointsInEntity]HEntity{point(0, 5), point(2, 5)}}})

	for _, tc := range []struct {
		typ       ConstraintType
		a, b      HEntity
		wantValue float64
	}{
		{ConstraintArcArcLenRatio, quarter, half, 0.5},
		{ConstraintArcLineLenRatio, line, quarter, math.Pi / 4},
		{ConstraintArcArcDifference, quarter, half, -math.Pi / 2},
		{ConstraintArcLineDifference, line, quarter, math.Pi/2 - 2},
	} {
		c := &Constraint{ConstraintBase: ConstraintBase{H: HConstraint{V: 1}, Type: tc.typ,
			Workplane: NoEntity, EntityA: tc.a, EntityB: tc.b}}
		c.ModifyToSatisfy(sk)
		if math.Abs(c.ValA-tc.wantValue) > 1e-12 {
			t.Errorf("type %d: ModifyToSatisfy gives %g, want %g", tc.typ, c.ValA, tc.wantValue)
		}
		var eqs []Equation
		c.GenerateEquations(sk, &eqs, false)
		if len(eqs) != 1 {
			t.Fatalf("type %d: %d equations", tc.typ, len(eqs))
		}
		if r := EvalExpr(sk, eqs[0].Expr); math.Abs(r) > 1e-12 {
			t.Errorf("type %d: residual %g", tc.typ, r)
		}
	}
}
