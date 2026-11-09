package sketch

import (
	"cmp"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
)

// A planar loop of parts closed by a hinge is redundant in 3d: the
// hinges before it already hold the last part's axis parallel to the
// ground's, and the closing hinge says so again. The rank test calls
// that redundant, as it would a dimension said twice, but the two are
// not alike. A dimension said twice can be given a value the other
// contradicts; mates, incidences with no value, say again only what
// the parts' own shapes make true, and stay consistent as the
// mechanism moves. So a mechanism whose solve converged with every
// dependency made of mates is solved, its redundancy implied; one with
// a dimension in a dependency is refused as before, and one whose mates
// cannot all hold does not converge and is refused too.

// impliedByMates reports whether the redundancy a solve of the block of
// groups hgs found is the mates' alone: the block is a mechanism (one
// of its groups moves in one), and every constraint of every
// dependency (deps, or remove where the solver named no dependency) is
// a mate's (isMateConstraint).
func (sk *Sketch) impliedByMates(hgs []core.HGroup, deps [][]core.HConstraint, remove []core.HConstraint) bool {
	if !slices.ContainsFunc(hgs, func(hg core.HGroup) bool { return movesInMechanism(sk.Group(hg)) }) {
		return false
	}
	var all []core.HConstraint
	for _, d := range deps {
		all = append(all, d...)
	}
	if len(all) == 0 {
		all = remove
	}
	if len(all) == 0 {
		return false
	}
	for _, hc := range all {
		if c := sk.Constraint(hc); c == nil || !isMateConstraint(c) {
			return false
		}
	}
	return true
}

// isMateConstraint reports whether c is of the kind a mate makes: an
// incidence with no value (coincident, on, parallel, ...), or a
// distance of zero with no expression, as Concentric and Flush put a
// point on an axis or a plane by. A dimension with a value, an
// expression or a reference is not.
func isMateConstraint(c *core.Constraint) bool {
	if c.Reference || c.Expr != "" || c.IsInequality() {
		return false
	}
	if !c.HasLabel() {
		return true
	}
	switch c.Type {
	case core.ConstraintProjPtDistance, core.ConstraintPtPlaneDistance, core.ConstraintPtLineDistance,
		core.ConstraintPtFaceDistance, core.ConstraintPtPtDistance:
		return c.ValA == 0
	}
	return false
}

// ImpliedConstraints are the constraints of group hg whose redundancy
// its last solve found implied by the mates of its mechanism
// (Group.Solved.Implied), each once in handle order; none when it found
// none.
func (sk *Sketch) ImpliedConstraints(hg core.HGroup) []core.HConstraint {
	g := sk.Group(hg)
	if g == nil || !g.Solved.Implied {
		return nil
	}
	var out []core.HConstraint
	for _, hs := range g.Solved.Groups {
		out = append(out, hs...)
	}
	if len(out) == 0 {
		out = slices.Clone(g.Solved.Remove)
	}
	out = slices.DeleteFunc(out, func(hc core.HConstraint) bool {
		c := sk.Constraint(hc)
		return c == nil || c.Group != hg
	})
	slices.SortFunc(out, func(a, b core.HConstraint) int { return cmp.Compare(a.V, b.V) })
	return slices.Compact(out)
}
