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
// chamfers each edge the other way round, its own distances too. It is
// an error for a chamfer of one distance, which has nothing to swap.
func (sk *Sketch) SwapChamferDistances(h core.HGroup) error {
	g, err := sk.chamferGroup(h)
	if err != nil {
		return err
	}
	if g.ValC == 0 && g.ExprC == "" && len(g.EdgeSeconds) == 0 {
		return errors.New("the chamfer has one distance")
	}
	// An edge with a distance of its own gets both of its own, swapped.
	var radii, seconds []EdgeRadius
	for i := range len(g.Edges) + len(g.SolidEdges) {
		v1, e1, own1 := g.EdgeRadius(i)
		v2, e2, own2 := g.EdgeSecond(i)
		if !own1 && !own2 {
			continue
		}
		if v2 == 0 && e2 == "" {
			v2, e2 = v1, e1
		}
		radii = append(radii, EdgeRadius{Edge: i, Val: v2, Expr: e2})
		seconds = append(seconds, EdgeRadius{Edge: i, Val: v1, Expr: e1})
	}
	g.EdgeRadii, g.EdgeSeconds = radii, seconds
	g.ValB, g.ValC = g.ValC, g.ValB
	g.ExprB, g.ExprC = g.ExprC, g.ExprB
	if g.ValB == 0 && g.ExprB == "" {
		// A chamfer of one distance whose edges' own second distances
		// were swapped: its distance stays its first.
		g.ValB, g.ExprB = g.ValC, g.ExprC
		g.ValC, g.ExprC = 0, ""
	}
	sk.MarkGroupDirty(g.H, false)
	return nil
}

// SameChamferDistances makes the second distance of chamfer h its first
// again, on every edge: the edges' own second distances go too.
func (sk *Sketch) SameChamferDistances(h core.HGroup) error {
	if err := sk.SetChamferSecondDistance(h, 0); err != nil {
		return err
	}
	sk.Group(h).EdgeSeconds = nil
	return nil
}
