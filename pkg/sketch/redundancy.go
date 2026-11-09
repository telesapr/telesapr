package sketch

import (
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
)

// A group whose constraints over-determine each other solves, but its
// Jacobian has lost rank: some of its constraints say again what others
// say. The solver names each such dependency as a whole
// (solver.System.DependentConstraints), from the left null space of the
// Jacobian at the solution, rather than one constraint a search
// happened to try first, so that the user sees which constraints are at
// odds and chooses which to drop. Group.Solved.Groups keeps them.

// RedundantGroups are the dependencies of group hg's constraints after
// its last solve, each the constraints that over-determine each other,
// in handle order with the point-coincident ones last, those the sketch
// no longer has left out; none when the group is not redundant.
func (sk *Sketch) RedundantGroups(hg core.HGroup) [][]core.HConstraint {
	g := sk.Group(hg)
	if g == nil || g.Solved.How != core.SolveRedundantOkay {
		return nil
	}
	var out [][]core.HConstraint
	for _, hs := range g.Solved.Groups {
		var kept []core.HConstraint
		for _, h := range hs {
			if sk.Constraint(h) != nil {
				kept = append(kept, h)
			}
		}
		if len(kept) > 0 {
			out = append(out, kept)
		}
	}
	return out
}

// RedundancyText says which of group hg's constraints over-determine
// each other, a clause for each dependency, as "c001-horizontal,
// c002-horizontal and c003-parallel over-determine each other"; "" when
// the group is not redundant.
func (sk *Sketch) RedundancyText(hg core.HGroup) string {
	var clauses []string
	for _, hs := range sk.RedundantGroups(hg) {
		names := make([]string, len(hs))
		for i, h := range hs {
			names[i] = sk.Constraint(h).DescriptionString()
		}
		verb := " over-determine each other"
		if len(names) == 1 {
			verb = " fixes nothing not fixed already"
		}
		clauses = append(clauses, joinAnd(names)+verb)
	}
	return strings.Join(clauses, "; ")
}

// joinAnd joins names as "a, b and c".
func joinAnd(names []string) string {
	if len(names) <= 1 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}
