package sketch

import (
	"errors"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// The builders refuse an entity the sketch does not have, and entities
// the constraint does not take, with an error callers can branch on.
func TestConstraintBuildersRefuse(t *testing.T) {
	t.Parallel()
	sk := NewSketch()
	missing := core.HEntity{V: 12345}
	base := core.Constraint{}
	for name, err := range map[string]error{
		"tangent":       second(sk.TangentConstraint(base, missing, missing)),
		"parallel":      second(sk.ParallelConstraint(base, missing, missing)),
		"perpendicular": second(sk.PerpendicularConstraint(base, missing, missing)),
		"midpoint":      third(sk.MidpointConstraint(base, missing, missing)),
		"symmetric":     third(sk.SymmetricConstraint(base, missing, missing, core.NoEntity)),
		"comment":       second(sk.CommentConstraint(base, missing, "x")),
	} {
		if !errors.Is(err, ErrConstraintSelection) {
			t.Errorf("%s gave %v, want ErrConstraintSelection", name, err)
		}
	}
}

func second(_ core.Constraint, err error) error             { return err }
func third(_ core.Constraint, _ []Implied, err error) error { return err }

// The text edits refuse a request that is not there, and one that is not
// a text.
func TestTextEditsRefuse(t *testing.T) {
	t.Parallel()
	sk := NewSketch()
	missing := core.HRequest{V: 12345}
	if err := sk.SetTextString(missing, "x"); !errors.Is(err, ErrNoRequest) {
		t.Errorf("SetTextString of a missing request gave %v", err)
	}
	if err := sk.SetTextKerning(missing, true); !errors.Is(err, ErrNoRequest) {
		t.Errorf("SetTextKerning of a missing request gave %v", err)
	}
	if err := sk.SetTextFont(missing, "x"); !errors.Is(err, ErrNoRequest) {
		t.Errorf("SetTextFont of a missing request gave %v", err)
	}
	if err := sk.SetConstraintLabelOffset(core.HConstraint{V: 99}, core.Constraint{}.Disp.Offset); !errors.Is(err, ErrNoConstraint) {
		t.Errorf("SetConstraintLabelOffset of a missing constraint gave %v", err)
	}
	if err := sk.SetConstraintShowAsRadius(core.HConstraint{V: 99}, true); !errors.Is(err, ErrNoConstraint) {
		t.Errorf("SetConstraintShowAsRadius of a missing constraint gave %v", err)
	}
}
