package sketch

import (
	"errors"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// SetConstraintLimit refuses what can't be a bound and a limit that is
// none.
func TestSetConstraintLimitRefuses(t *testing.T) {
	t.Parallel()
	sk := NewSketch()
	sk.NewFile()
	sk.GenerateAll()
	g := sk.Groups[len(sk.Groups)-1].H
	c := core.Constraint{Group: g, Type: core.ConstraintComment}
	c.Comment = "no bound"
	hs, err := sk.AddConstraints([]core.Constraint{c})
	if err != nil {
		t.Fatal(err)
	}
	if err := sk.SetConstraintLimit(core.HConstraint{V: 99}, core.LimitMin); !errors.Is(err, ErrNoConstraint) {
		t.Errorf("a missing constraint gave %v", err)
	}
	if err := sk.SetConstraintLimit(hs[0], core.LimitMax); !errors.Is(err, ErrCannotLimit) {
		t.Errorf("a comment gave %v", err)
	}
	sk.Constraint(hs[0]).Type = core.ConstraintAngle
	if err := sk.SetConstraintLimit(hs[0], core.Limit(7)); !errors.Is(err, ErrCannotLimit) {
		t.Errorf("limit 7 gave %v", err)
	}
}
