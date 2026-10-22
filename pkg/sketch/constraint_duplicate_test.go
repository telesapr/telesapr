package sketch

import (
	"errors"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// DuplicateConstraint adds what AddConstraints keeps to one: a second
// constraint equal to the first, with a handle of its own.
func TestDuplicateConstraint(t *testing.T) {
	t.Parallel()
	sk := NewSketch()
	sk.NewFile()
	sk.GenerateAll()
	g := sk.Groups[len(sk.Groups)-1].H
	c := core.Constraint{Group: g, Type: core.ConstraintComment}
	c.Comment = "twice"
	hs, err := sk.AddConstraints([]core.Constraint{c, c})
	if err != nil || hs[0] != hs[1] {
		t.Fatalf("AddConstraints gave %v, %v; want one constraint", hs, err)
	}
	dup, err := sk.DuplicateConstraint(hs[0])
	if err != nil || dup == hs[0] {
		t.Fatalf("DuplicateConstraint gave %v, %v", dup, err)
	}
	a, b := sk.Constraint(hs[0]), sk.Constraint(dup)
	if !a.ConstraintBase.Equals(&b.ConstraintBase) { //nolint:staticcheck // QF1008: the base is named on purpose
		t.Errorf("the copy %+v differs from %+v", b, a)
	}
	if _, err := sk.DuplicateConstraint(core.HConstraint{V: 99}); !errors.Is(err, ErrNoConstraint) {
		t.Errorf("duplicating a missing constraint gave %v", err)
	}
}
