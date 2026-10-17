package sketch

import (
	"errors"
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// ErrNoConstraint is what an edit returns for a constraint the sketch
// does not have.
var ErrNoConstraint = errors.New("no such constraint")

// SetConstraintLabelOffset puts the label of a dimension or a comment
// at off from the place the constraint measures: what dragging the
// label does, on every move of the mouse. It changes how the sketch is
// drawn, not what it holds, so nothing is regenerated. The caller
// remembers the state for undo.
func (sk *Sketch) SetConstraintLabelOffset(hc core.HConstraint, off geom.Vector) error {
	c := sk.Constraint(hc)
	if c == nil {
		return fmt.Errorf("constraint %08x: %w", hc.V, ErrNoConstraint)
	}
	if !c.HasLabel() {
		return fmt.Errorf("%s has no label to place", c.DescriptionString())
	}
	c.Disp.Offset = off
	return nil
}

// SetConstraintShowAsRadius makes a diameter dimension show as a
// radius, or as a diameter again. The value it holds is the diameter
// either way.
func (sk *Sketch) SetConstraintShowAsRadius(hc core.HConstraint, radius bool) error {
	c := sk.Constraint(hc)
	if c == nil {
		return fmt.Errorf("constraint %08x: %w", hc.V, ErrNoConstraint)
	}
	if c.Type != core.ConstraintDiameter {
		return fmt.Errorf("%s isn't a diameter", c.DescriptionString())
	}
	c.Other = radius
	return nil
}
