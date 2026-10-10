package ui

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// A minimum or a maximum distance is labelled by what it bounds; the
// vector font has no ≥ or ≤.
func TestDistanceBoundLabels(t *testing.T) {
	ss, _ := newTestApp(t)
	for _, tc := range []struct {
		c    core.Constraint
		want string
	}{
		{core.Constraint{ConstraintBase: core.ConstraintBase{Type: core.ConstraintMinDistance, ValA: 40}}, "min 40.00"},
		{core.Constraint{ConstraintBase: core.ConstraintBase{Type: core.ConstraintMaxDistance, ValA: 12.5}}, "max 12.50"},
		{core.Constraint{ConstraintBase: core.ConstraintBase{Type: core.ConstraintMaxDistance, ValA: 5, Name: "reach"}}, "reach = max 5.00"},
	} {
		if got := ss.constraintLabel(&tc.c); got != tc.want {
			t.Errorf("%v: %q, want %q", tc.c.Type, got, tc.want)
		}
	}
}
