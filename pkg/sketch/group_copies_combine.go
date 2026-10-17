package sketch

import (
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
)

// groupConstraintCount is how many constraints belong to the group.
func (sk *Sketch) groupConstraintCount(hg core.HGroup) int {
	n := 0
	for i := range sk.Constraints {
		if sk.Constraints[i].Group == hg {
			n++
		}
	}
	return n
}

// SetGroupCopies sets a step and repeat or pattern group's number of
// copies from typed text, as SetGroupText does for GroupCopies. A
// rotate group with no constraints of its own also gets, with respace
// set, the angle that spaces its copies evenly over one turn: with
// nothing to hold the angle, the old one would leave the copies bunched
// or overlapping at the new count. A constrained group keeps its angle,
// because moving the solver's starting guess would break convergence. A
// script states its angle itself and passes false, so that the angle it
// gave stays whatever the count.
func (sk *Sketch) SetGroupCopies(hg core.HGroup, text string, respace bool) error {
	if err := sk.SetGroupText(hg, GroupCopies, text); err != nil {
		return err
	}
	g := sk.Group(hg)
	if respace && g.Type == GroupRotate && sk.groupConstraintCount(hg) == 0 {
		copies := g.ValA
		if g.SkipFirst {
			copies++
		}
		sk.MustParam(hg.Param(3)).Val = math.Pi / (2 * copies)
	}
	return nil
}

// SetGroupCombine sets how a group's solid combines with the solid so
// far. A new extrude group is placed for a union; with flip set and no
// constraints to hold the vector, switching between the union and
// assemble modes and the difference and intersection modes turns the
// vector round, so the solid goes the other way into the material, as
// the group screen does for someone who has drawn nothing else. A
// script states its extrusion direction itself and passes false, so
// that the depth it gave is the depth it gets in any mode.
func (sk *Sketch) SetGroupCombine(hg core.HGroup, as CombineAs, flip bool) error {
	g := sk.Group(hg)
	if g == nil {
		return fmt.Errorf("no group %08x", hg.V)
	}
	if !g.HasCombine() {
		return ErrNoCombine
	}
	if as < CombineUnion || as > CombineIntersection {
		return fmt.Errorf("combine mode %d is not known", int(as))
	}
	if flip && g.Type == GroupExtrude && g.MeshCombine != as && sk.groupConstraintCount(hg) == 0 {
		cut := func(c CombineAs) bool { return c == CombineDifference || c == CombineIntersection }
		if cut(g.MeshCombine) != cut(as) {
			g.ExtrusionForceVectorTo(sk, g.ExtrusionGetVector(sk).Negated())
		}
	}
	g.MeshCombine = as
	return nil
}
