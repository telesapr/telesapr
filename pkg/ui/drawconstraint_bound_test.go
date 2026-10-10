package ui

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// A bound is labelled by which it is, on any dimension; the vector font
// has no ≥ or ≤.
func TestDistanceBoundLabels(t *testing.T) {
	ss, _ := newTestApp(t)
	for _, tc := range []struct {
		c    core.Constraint
		want string
	}{
		{core.Constraint{ConstraintBase: core.ConstraintBase{Type: core.ConstraintPtPtDistance, ValA: 40, Limit: core.LimitMin}}, "min 40.00"},
		{core.Constraint{ConstraintBase: core.ConstraintBase{Type: core.ConstraintPtPtDistance, ValA: 12.5, Limit: core.LimitMax}}, "max 12.50"},
		{core.Constraint{ConstraintBase: core.ConstraintBase{Type: core.ConstraintPtPtDistance, ValA: 5, Name: "reach", Limit: core.LimitMax}}, "reach = max 5.00"},
		{core.Constraint{ConstraintBase: core.ConstraintBase{Type: core.ConstraintAngle, ValA: 30, Limit: core.LimitMin}}, "min 30°"},
		{core.Constraint{ConstraintBase: core.ConstraintBase{Type: core.ConstraintDiameter, ValA: 10, Limit: core.LimitMax, Other: true}}, "max R5.00"},
	} {
		if got := ss.constraintLabel(&tc.c); got != tc.want {
			t.Errorf("%v: %q, want %q", tc.c.Type, got, tc.want)
		}
	}
}
