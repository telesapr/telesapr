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
// become redundant.
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

// AddConstraintRemember adds c with a new handle, and its params, and
// marks its group dirty.
func (sk *Sketch) AddConstraintRemember(c *core.Constraint, rememberForUndo bool) core.HConstraint {
	if rememberForUndo {
		sk.UndoRemember()
	}

	c.H = sk.AddConstraint(*c)
	GenerateConstraint(sk.Constraint(c.H), &sk.Params)
	sk.Constraint(c.H).SeedParam(sk)

	sk.MarkGroupDirty(c.Group, false)
	sk.Group(c.Group).DofCheckOk = false
	return c.H
}

// Constrain adds a constraint of the given type in the active group and
// workplane.
func (sk *Sketch) Constrain(typ core.ConstraintType, ptA, ptB, entityA, entityB core.HEntity,
	other, other2 bool) core.HConstraint {
	c := core.Constraint{
		Group:     sk.ActiveGroup(),
		Workplane: sk.ActiveWorkplane(),
		Type:      typ,
		PtA:       ptA,
		PtB:       ptB,
		EntityA:   entityA,
		EntityB:   entityB,
		Other:     other,
		Other2:    other2}
	return sk.AddConstraintRemember(&c, false)
}

// TryConstrain is Constrain, except that a constraint that would be
// redundant isn't added; it then returns no constraint.
func (sk *Sketch) TryConstrain(typ core.ConstraintType, ptA, ptB, entityA, entityB core.HEntity,
	other, other2 bool) core.HConstraint {
	howBefore, rankBefore := sk.TestRankForGroup(sk.ActiveGroup())
	hc := sk.Constrain(typ, ptA, ptB, entityA, entityB, other, other2)
	howAfter, rankAfter := sk.TestRankForGroup(sk.ActiveGroup())
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

// RemoveConstraint deletes constraint hc.
func (sk *Sketch) RemoveConstraint(hc core.HConstraint) {
	for i := range sk.Constraints {
		if sk.Constraints[i].H == hc {
			sk.Constraints = append(sk.Constraints[:i], sk.Constraints[i+1:]...)
			return
		}
	}
}

// ConstrainCoincident makes points ptA and ptB coincident.
func (sk *Sketch) ConstrainCoincident(ptA, ptB core.HEntity) core.HConstraint {
	return sk.Constrain(core.ConstraintPointsCoincident, ptA, ptB,
		core.NoEntity, core.NoEntity, false, false)
}

// ConstrainArcLineTangent sets which end of arc c makes tangent to
// line: the one they share, else arcEndpoint.
func (sk *Sketch) ConstrainArcLineTangent(c *core.Constraint, line, arc, arcEndpoint *core.Entity) error {
	l0 := sk.MustEntity(line.Point[0]).PointGetNum(sk)
	l1 := sk.MustEntity(line.Point[1]).PointGetNum(sk)
	a1 := sk.MustEntity(arc.Point[1]).PointGetNum(sk)
	a2 := sk.MustEntity(arc.Point[2]).PointGetNum(sk)
	switch {
	case l0.Equals(a1) || l1.Equals(a1):
		c.Other = false

	case l0.Equals(a2) || l1.Equals(a2):
		c.Other = true

	case arcEndpoint != nil:
		p := arcEndpoint.PointGetNum(sk)
		switch {
		case a1.Equals(p):
			c.Other = false

		case a2.Equals(p):
			c.Other = true

		default:
			return errors.New("the point you selected does not belong to the arc. " +
				"The arc and line segment do not share an end point.\n\n" +
				"Select the end point of the arc at which you want " +
				"it to be tangent to the line")
		}

	default:
		return errors.New("the tangent arc and line segment must share an " +
			"endpoint. Constrain them with Constrain -> " +
			"On Point before constraining tangent.\n\n" +
			"Alternatively select the end point of the arc " +
			"at which you want it to be tangent to the line")
	}
	return nil
}

// ConstrainCubicLineTangent sets which end of cubic c makes tangent to
// line.
func (sk *Sketch) ConstrainCubicLineTangent(c *core.Constraint, line, cubic, curveEndpoint *core.Entity) error {
	l0 := sk.MustEntity(line.Point[0]).PointGetNum(sk)
	l1 := sk.MustEntity(line.Point[1]).PointGetNum(sk)
	as := cubic.CubicGetStartNum(sk)
	af := cubic.CubicGetFinishNum(sk)

	switch {
	case l0.Equals(as) || l1.Equals(as):
		c.Other = false

	case l0.Equals(af) || l1.Equals(af):
		c.Other = true

	case curveEndpoint != nil:
		p := curveEndpoint.PointGetNum(sk)
		switch {
		case as.Equals(p):
			c.Other = false

		case af.Equals(p):
			c.Other = true

		default:
			return errors.New("the point you selected is not an end point of the cubic spline. " +
				"The spline and line segment do not share an end point.\n\n" +
				"Select the end point of the spline at which you want " +
				"it to be tangent to the line")
		}

	default:
		return errors.New("the tangent cubic spline and line segment must share an " +
			"endpoint. Constrain them with Constrain -> " +
			"On Point before constraining tangent.\n\n" +
			"Alternatively select the end point of the cubic spline " +
			"at which you want it to be tangent to the line")
	}
	return nil
}

// ConstrainCurveCurveTangent sets at which ends c makes curves eA and
// eB tangent: the ends they share, else epA and epB.
func (sk *Sketch) ConstrainCurveCurveTangent(c *core.Constraint, eA, eB, epA, epB *core.Entity) error {
	as, af := eA.EndpointStart(sk), eA.EndpointFinish(sk)
	bs, bf := eB.EndpointStart(sk), eB.EndpointFinish(sk)
	switch {
	case as.Equals(bs):
		c.Other, c.Other2 = false, false

	case as.Equals(bf):
		c.Other, c.Other2 = false, true

	case af.Equals(bs):
		c.Other, c.Other2 = true, false

	case af.Equals(bf):
		c.Other, c.Other2 = true, true

	case epA != nil && epB != nil:
		pa, pb := epA.PointGetNum(sk), epB.PointGetNum(sk)
		switch {
		case (as.Equals(pa) && bs.Equals(pb)) || (as.Equals(pb) && bs.Equals(pa)):
			c.Other, c.Other2 = false, false

		case (as.Equals(pa) && bf.Equals(pb)) || (as.Equals(pb) && bf.Equals(pa)):
			c.Other, c.Other2 = false, true

		case (af.Equals(pa) && bs.Equals(pb)) || (af.Equals(pb) && bs.Equals(pa)):
			c.Other, c.Other2 = true, false

		case (af.Equals(pa) && bf.Equals(pb)) || (af.Equals(pb) && bf.Equals(pa)):
			c.Other, c.Other2 = true, true

		default:
			return errors.New("the points you selected are not end points of the two curves. " +
				"The curves do not share an end point.\n\n" +
				"Select the end points of both curves at which you want " +
				"them to be tangent to each other")
		}

	default:
		return errors.New("the curves must share an endpoint. Constrain them " +
			"with Constrain -> On Point before constraining " +
			"tangent.\n\n" +
			"Alternatively select the end points of both " +
			"curves at which you want the curves to be tangent")
	}
	return nil
}
