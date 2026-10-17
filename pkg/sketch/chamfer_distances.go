package sketch

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
)

// chamferGroup is chamfer h, or an error when h is not one.
func (sk *Sketch) chamferGroup(h core.HGroup) (*Group, error) {
	g := sk.Group(h)
	if g == nil || g.Type != GroupChamfer {
		return nil, errors.New("the group is not a chamfer")
	}
	return g, nil
}

// SetChamferSecondDistance makes chamfer h one of two distances: ValB on
// each edge's first face and mm on the second; 0 makes it of one
// distance again. It is what the group's screen and scripts both call.
func (sk *Sketch) SetChamferSecondDistance(h core.HGroup, mm float64) error {
	g, err := sk.chamferGroup(h)
	if err != nil {
		return err
	}
	if mm < 0 || math.IsNaN(mm) || math.IsInf(mm, 0) {
		return fmt.Errorf("a chamfer's second distance must be more than zero, or zero for the first again, not %g", mm)
	}
	g.ValC, g.ExprC = mm, ""
	sk.MarkGroupDirty(g.H, false)
	return nil
}

// SwapChamferDistances swaps the two distances of chamfer h, which
// chamfers each edge the other way round. It is an error for a chamfer
// of one distance, which has nothing to swap.
func (sk *Sketch) SwapChamferDistances(h core.HGroup) error {
	g, err := sk.chamferGroup(h)
	if err != nil {
		return err
	}
	if g.ValC == 0 && g.ExprC == "" {
		return errors.New("the chamfer has one distance")
	}
	g.ValB, g.ValC = g.ValC, g.ValB
	g.ExprB, g.ExprC = g.ExprC, g.ExprB
	sk.MarkGroupDirty(g.H, false)
	return nil
}

// SameChamferDistances makes the second distance of chamfer h its first
// again.
func (sk *Sketch) SameChamferDistances(h core.HGroup) error {
	return sk.SetChamferSecondDistance(h, 0)
}
