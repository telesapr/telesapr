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

// A constraint whose value differs from one the sketch has by less than
// the solver holds a dimension to is that one: measured where the other
// holds the sketch, it reads the last bits of a solve that moved
// nothing. Further off, it is another.
func TestEqualConstraintWithinTolerance(t *testing.T) {
	t.Parallel()
	sk := NewSketch()
	sk.NewFile()
	sk.GenerateAll()
	g := sk.Groups[len(sk.Groups)-1].H
	c := core.Constraint{Group: g, Type: core.ConstraintComment}
	c.Comment = "measured"
	c.ValA = 12.803459707901933
	near, far := c, c
	near.ValA = 12.803459707901935
	far.ValA = 12.8035
	hs, err := sk.AddConstraints([]core.Constraint{c, near, far})
	if err != nil {
		t.Fatal(err)
	}
	if hs[1] != hs[0] {
		t.Errorf("a value 2e-15 off made constraint %v beside %v", hs[1], hs[0])
	}
	if hs[2] == hs[0] {
		t.Errorf("a value 4e-5 off was taken for constraint %v", hs[0])
	}
}
