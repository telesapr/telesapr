package sketch

import (
	"errors"

	"github.com/telesapr/telesapr/pkg/core"
)

// This file ports the part of solvespace/src/constraint.cpp that makes
// constraints: MenuConstrain, which turns the selection into one, is
// user interface.

// DeleteAllConstraintsFor deletes the constraints of the given type on
// entityA and ptA; used when auto-removing constraints that would
// become redundant (C++ Constraint::DeleteAllConstraintsFor).
func (sk *Sketch) DeleteAllConstraintsFor(typ core.ConstraintType, entityA, ptA core.HEntity) {
	kept := sk.Constraints[:0]
	for _, c := range sk.Constraints {
		if c.Type == typ && c.EntityA == entityA && c.PtA == ptA {
			continue
		}
		kept = append(kept, c)
	}
	sk.Constraints = kept
}

// AddConstraintRemember adds c with a new handle, and its params, and marks
// its group dirty (C++ Constraint::AddConstraint).
func (sk *Sketch) AddConstraintRemember(c *core.Constraint, rememberForUndo bool) core.HConstraint {
	if rememberForUndo {
		sk.UndoRemember()
	}

	c.H = sk.AddConstraint(*c)
	GenerateConstraint(sk.GetConstraint(c.H), &sk.Params)

	sk.MarkGroupDirty(c.Group, false)
	sk.GetGroup(c.Group).DofCheckOk = false
	return c.H
}

// Constrain adds a constraint of the given type in the active group
// and workplane (C++ Constraint::Constrain).
func (sk *Sketch) Constrain(typ core.ConstraintType, ptA, ptB, entityA, entityB core.HEntity,
	other, other2 bool) core.HConstraint {
	c := core.Constraint{}
	c.Group = sk.ActiveGroup()
	c.Workplane = sk.ActiveWorkplane()
	c.Type = typ
	c.PtA = ptA
	c.PtB = ptB
	c.EntityA = entityA
	c.EntityB = entityB
	c.Other = other
	c.Other2 = other2
	return sk.AddConstraintRemember(&c, false)
}

// TryConstrain is Constrain, except that a constraint that would be
// redundant isn't added; it then returns no constraint (C++
// Constraint::TryConstrain).
func (sk *Sketch) TryConstrain(typ core.ConstraintType, ptA, ptB, entityA, entityB core.HEntity,
	other, other2 bool) core.HConstraint {
	var rankBefore, rankAfter int
	howBefore := sk.TestRankForGroup(sk.ActiveGroup(), &rankBefore)
	hc := sk.Constrain(typ, ptA, ptB, entityA, entityB, other, other2)
	howAfter := sk.TestRankForGroup(sk.ActiveGroup(), &rankAfter)
	// There are two cases where the constraint is clearly redundant:
	//   * If the group wasn't overconstrained and now it is;
	//   * If the group was overconstrained, and adding the constraint doesn't change rank at all.
	if (howBefore == core.SolveOkay && howAfter == core.SolveRedundantOkay) ||
		(howBefore == core.SolveRedundantOkay && howAfter == core.SolveRedundantOkay &&
			rankBefore == rankAfter) {
		sk.RemoveConstraint(hc)
		hc = core.HConstraint{}
	}
	return hc
}

// RemoveConstraint deletes constraint hc (C++
// SK.constraint.RemoveById).
func (sk *Sketch) RemoveConstraint(hc core.HConstraint) {
	for i := range sk.Constraints {
		if sk.Constraints[i].H == hc {
			sk.Constraints = append(sk.Constraints[:i], sk.Constraints[i+1:]...)
			return
		}
	}
}

// ConstrainCoincident makes points ptA and ptB coincident (C++
// Constraint::ConstrainCoincident).
func (sk *Sketch) ConstrainCoincident(ptA, ptB core.HEntity) core.HConstraint {
	return sk.Constrain(core.ConstraintPointsCoincident, ptA, ptB,
		core.NoEntity, core.NoEntity, false, false)
}

// ConstrainArcLineTangent sets which end of arc c makes tangent to
// line: the one they share, else arcEndpoint (C++
// Constraint::ConstrainArcLineTangent, which reports the error to the
// user).
func (sk *Sketch) ConstrainArcLineTangent(c *core.Constraint, line, arc, arcEndpoint *core.Entity) error {
	l0 := sk.MustGetEntity(line.Point[0]).PointGetNum(sk)
	l1 := sk.MustGetEntity(line.Point[1]).PointGetNum(sk)
	a1 := sk.MustGetEntity(arc.Point[1]).PointGetNum(sk)
	a2 := sk.MustGetEntity(arc.Point[2]).PointGetNum(sk)
	if l0.Equals(a1) || l1.Equals(a1) {
		c.Other = false
	} else if l0.Equals(a2) || l1.Equals(a2) {
		c.Other = true
	} else if arcEndpoint != nil {
		p := arcEndpoint.PointGetNum(sk)
		if a1.Equals(p) {
			c.Other = false
		} else if a2.Equals(p) {
			c.Other = true
		} else {
			return errors.New("The point you selected does not belong to the arc. " +
				"The arc and line segment do not share an end point.\n\n" +
				"Select the end point of the arc at which you want " +
				"it to be tangent to the line.")
		}
	} else {
		return errors.New("The tangent arc and line segment must share an " +
			"endpoint. Constrain them with Constrain -> " +
			"On Point before constraining tangent.\n\n" +
			"Alternatively select the end point of the arc " +
			"at which you want it to be tangent to the line.")
	}
	return nil
}

// ConstrainCubicLineTangent sets which end of cubic c makes tangent to
// line (C++ Constraint::ConstrainCubicLineTangent).
func (sk *Sketch) ConstrainCubicLineTangent(c *core.Constraint, line, cubic, curveEndpoint *core.Entity) error {
	l0 := sk.MustGetEntity(line.Point[0]).PointGetNum(sk)
	l1 := sk.MustGetEntity(line.Point[1]).PointGetNum(sk)
	as := cubic.CubicGetStartNum(sk)
	af := cubic.CubicGetFinishNum(sk)

	if l0.Equals(as) || l1.Equals(as) {
		c.Other = false
	} else if l0.Equals(af) || l1.Equals(af) {
		c.Other = true
	} else if curveEndpoint != nil {
		p := curveEndpoint.PointGetNum(sk)
		if as.Equals(p) {
			c.Other = false
		} else if af.Equals(p) {
			c.Other = true
		} else {
			return errors.New("The point you selected is not an end point of the cubic spline. " +
				"The spline and line segment do not share an end point.\n\n" +
				"Select the end point of the spline at which you want " +
				"it to be tangent to the line.")
		}
	} else {
		return errors.New("The tangent cubic spline and line segment must share an " +
			"endpoint. Constrain them with Constrain -> " +
			"On Point before constraining tangent.\n\n" +
			"Alternatively select the end point of the cubic spline " +
			"at which you want it to be tangent to the line.")
	}
	return nil
}

// ConstrainCurveCurveTangent sets at which ends c makes curves eA and
// eB tangent: the ends they share, else epA and epB (C++
// Constraint::ConstrainCurveCurveTangent).
func (sk *Sketch) ConstrainCurveCurveTangent(c *core.Constraint, eA, eB, epA, epB *core.Entity) error {
	as, af := eA.EndpointStart(sk), eA.EndpointFinish(sk)
	bs, bf := eB.EndpointStart(sk), eB.EndpointFinish(sk)
	if as.Equals(bs) {
		c.Other, c.Other2 = false, false
	} else if as.Equals(bf) {
		c.Other, c.Other2 = false, true
	} else if af.Equals(bs) {
		c.Other, c.Other2 = true, false
	} else if af.Equals(bf) {
		c.Other, c.Other2 = true, true
	} else if epA != nil && epB != nil {
		pa, pb := epA.PointGetNum(sk), epB.PointGetNum(sk)
		if (as.Equals(pa) && bs.Equals(pb)) || (as.Equals(pb) && bs.Equals(pa)) {
			c.Other, c.Other2 = false, false
		} else if (as.Equals(pa) && bf.Equals(pb)) || (as.Equals(pb) && bf.Equals(pa)) {
			c.Other, c.Other2 = false, true
		} else if (af.Equals(pa) && bs.Equals(pb)) || (af.Equals(pb) && bs.Equals(pa)) {
			c.Other, c.Other2 = true, false
		} else if (af.Equals(pa) && bf.Equals(pb)) || (af.Equals(pb) && bf.Equals(pa)) {
			c.Other, c.Other2 = true, true
		} else {
			return errors.New("The points you selected are not end points of the two curves. " +
				"The curves do not share an end point.\n\n" +
				"Select the end points of both curves at which you want " +
				"them to be tangent to each other.")
		}
	} else {
		return errors.New("The curves must share an endpoint. Constrain them " +
			"with Constrain -> On Point before constraining " +
			"tangent.\n\n" +
			"Alternatively select the end points of both " +
			"curves at which you want the curves to be tangent.")
	}
	return nil
}
