package sketch

import (
	"errors"
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
)

// ErrCannotLimit is returned for a limit a constraint can't take: one
// that is no distance, diameter or angle, or a limit that is none of
// core.LimitNone, LimitMin and LimitMax.
var ErrCannotLimit = errors.New("can't be a minimum or a maximum")

// SetConstraintLimit makes the dimension hc a bound (core.LimitMin,
// core.LimitMax) or a dimension again (core.LimitNone). The window's
// Minimum and Maximum on a selected dimension and a script's
// Constraint.SetLimit both come here, so that a bound is the same
// whichever way it is made. A dimension SettleConstraint made a
// reference only because it would have over-constrained the group
// becomes a bound that holds, as a bound added at once does (a script
// makes a bound as a dimension first, and a second angle at a corner
// that has a bound already is redundant as a dimension); one the user
// made a reference stays one. A bound made a dimension again is made a
// reference if it over-constrains the group, as a new dimension is.
func (sk *Sketch) SetConstraintLimit(hc core.HConstraint, l core.Limit) error {
	c := sk.Constraint(hc)
	if c == nil {
		return fmt.Errorf("setting the limit of constraint %08x: %w", hc.V, ErrNoConstraint)
	}
	if !c.CanLimit() {
		return fmt.Errorf("%s: %w", c.DescriptionString(), ErrCannotLimit)
	}
	switch l {
	case core.LimitNone, core.LimitMin, core.LimitMax:
	default:
		return fmt.Errorf("no limit %d: %w", l, ErrCannotLimit)
	}
	wasBound := c.Limit != core.LimitNone
	if l != core.LimitNone && !wasBound && c.Reference && sk.redundantAsDimension(hc) {
		sk.Constraint(hc).Reference = false
	}
	c = sk.Constraint(hc)
	c.Limit = l
	sk.MarkGroupDirty(c.Group, false)
	if l == core.LimitNone && wasBound && !c.Reference && sk.redundantAsDimension(hc) {
		sk.Constraint(hc).Reference = true
	}
	return nil
}

// redundantAsDimension reports whether hc, held as a dimension and no
// reference, would over-constrain its group: what SettleConstraint
// makes a reference for. hc is left as it was.
func (sk *Sketch) redundantAsDimension(hc core.HConstraint) bool {
	c := sk.Constraint(hc)
	g := sk.Group(c.Group)
	if g == nil || g.AllowRedundant {
		return false
	}
	ref, limit := c.Reference, c.Limit
	c.Reference, c.Limit = false, core.LimitNone
	how, _ := sk.TestRankForGroup(c.Group)
	c = sk.Constraint(hc)
	c.Reference, c.Limit = ref, limit
	return how == core.SolveRedundantOkay
}
