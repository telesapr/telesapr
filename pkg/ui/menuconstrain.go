package ui

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// MenuConstrain makes a constraint of the selection (C++
// Constraint::MenuConstrain).
func (ss *Application) MenuConstrain(id Command) {
	gw := ss.GW
	sk := ss.SK
	var newcons []*core.Constraint

	c := core.Constraint{
		Group:     gw.activeGroupH(),
		Workplane: gw.ActiveWorkplane()}

	gw.GroupSelection()
	gs := &gw.gs

	// add is Constraint::AddConstraint on a copy of c, kept in newcons.
	add := func(nc core.Constraint, remember bool) {
		ss.addConstraint(&nc, remember)
		newcons = append(newcons, &nc)
	}
	errorf := func(msg string) { ss.Error("%s", msg) }

	switch id {
	case CmdDistanceDia, CmdRefDistance:
		switch {
		case gs.points == 2 && gs.n == 2:
			c.Type = core.ConstraintPtPtDistance
			c.PtA = gs.point[0]
			c.PtB = gs.point[1]
		case gs.lineSegments == 1 && gs.n == 1:
			c.Type = core.ConstraintPtPtDistance
			e := sk.Entity(gs.entity[0])
			c.PtA = e.Point[0]
			c.PtB = e.Point[1]
		case gs.vectors == 1 && gs.points == 2 && gs.n == 3:
			c.Type = core.ConstraintProjPtDistance
			c.PtA = gs.point[0]
			c.PtB = gs.point[1]
			c.EntityA = gs.vector[0]
		case gs.workplanes == 1 && gs.points == 1 && gs.n == 2:
			c.Type = core.ConstraintPtPlaneDistance
			c.PtA = gs.point[0]
			c.EntityA = gs.entity[0]
		case gs.lineSegments == 1 && gs.points == 1 && gs.n == 2:
			c.Type = core.ConstraintPtLineDistance
			c.PtA = gs.point[0]
			c.EntityA = gs.entity[0]
		case gs.faces == 1 && gs.points == 1 && gs.n == 2:
			c.Type = core.ConstraintPtFaceDistance
			c.PtA = gs.point[0]
			c.EntityA = gs.face[0]
		case gs.circlesOrArcs == 1 && gs.n == 1:
			c.Type = core.ConstraintDiameter
			c.EntityA = gs.entity[0]
			arc := sk.Entity(gs.entity[0])
			if arc.Type == core.EntityArcOfCircle && !ss.ArcDimDefaultDiameter {
				c.Other = true
			}
		default:
			errorf("Bad selection for distance / diameter constraint. This " +
				"constraint can apply to:\n\n" +
				"    * two points (distance between points)\n" +
				"    * a line segment (length)\n" +
				"    * two points and a line segment or normal (projected distance)\n" +
				"    * a workplane and a point (minimum distance)\n" +
				"    * a line segment and a point (minimum distance)\n" +
				"    * a plane face and a point (minimum distance)\n" +
				"    * a circle or an arc (diameter)\n")
			return
		}
		if c.Type == core.ConstraintPtPtDistance || c.Type == core.ConstraintProjPtDistance {
			a := sk.Entity(c.PtA).PointGetNum(sk)
			b := sk.Entity(c.PtB).PointGetNum(sk)
			c.Disp.Offset = sk.DistanceLabelOffset(a, b)
		} else {
			c.Disp.Offset = geom.Vector{}
		}
		if id == CmdRefDistance {
			c.Reference = true
		}
		c.ValA = 0
		c.ModifyToSatisfy(sk)
		add(c, true)

	case CmdOnEntity:
		var pending []core.Constraint
		switch {
		case gs.points >= 2 && gs.points == gs.n:
			c.Type = core.ConstraintPointsCoincident
			c.PtA = gs.point[0]
			for k := 1; k < gs.points; k++ {
				c.PtB = gs.point[k]
				pending = append(pending, c)
			}
		case gs.points == 1 && gs.workplanes == 1 && gs.n == 2:
			c.Type = core.ConstraintPtInPlane
			c.PtA = gs.point[0]
			c.EntityA = gs.entity[0]
			pending = append(pending, c)
		case gs.points == 1 && gs.lineSegments == 1 && gs.n == 2:
			c.Type = core.ConstraintPtOnLine
			c.PtA = gs.point[0]
			c.EntityA = gs.entity[0]
			pending = append(pending, c)
		case gs.points == 1 && gs.circlesOrArcs == 1 && gs.n == 2:
			c.Type = core.ConstraintPtOnCircle
			c.PtA = gs.point[0]
			c.EntityA = gs.entity[0]
			pending = append(pending, c)
		case gs.points == 1 && gs.faces >= 1 && gs.n == gs.points+gs.faces:
			c.Type = core.ConstraintPtOnFace
			c.PtA = gs.point[0]
			for k := range gs.faces {
				c.EntityA = gs.face[k]
				pending = append(pending, c)
			}
		default:
			errorf("Bad selection for on point / curve / plane constraint. " +
				"This constraint can apply to:\n\n" +
				"    * two or more points (points coincident)\n" +
				"    * a point and a workplane (point in plane)\n" +
				"    * a point and a line segment (point on line)\n" +
				"    * a point and a circle or arc (point on curve)\n" +
				"    * a point and one to three plane faces (point on face(s))\n")
			return
		}
		for _, nc := range pending {
			add(nc, true)
		}

	case CmdEqual:
		var pending []core.Constraint
		switch {
		case gs.lineSegments >= 2 && gs.lineSegments == gs.n:
			c.Type = core.ConstraintEqualLengthLines
			c.EntityA = gs.entity[0]
			for k := 1; k < len(gs.entity); k++ {
				c.EntityB = gs.entity[k]
				pending = append(pending, c)
			}
		case gs.lineSegments == 2 && gs.points == 2 && gs.n == 4:
			c.Type = core.ConstraintEqPtLnDistances
			c.EntityA = gs.entity[0]
			c.PtA = gs.point[0]
			c.EntityB = gs.entity[1]
			c.PtB = gs.point[1]
			pending = append(pending, c)
		case gs.lineSegments == 1 && gs.points == 2 && gs.n == 3:
			// The same line segment for the distances, but different
			// points.
			c.Type = core.ConstraintEqPtLnDistances
			c.EntityA = gs.entity[0]
			c.PtA = gs.point[0]
			c.EntityB = gs.entity[0]
			c.PtB = gs.point[1]
			pending = append(pending, c)
		case gs.lineSegments == 2 && gs.points == 1 && gs.n == 3:
			c.Type = core.ConstraintEqLenPtLineD
			c.EntityA = gs.entity[0]
			c.EntityB = gs.entity[1]
			c.PtA = gs.point[0]
			pending = append(pending, c)
		case gs.circlesOrArcs >= 2 && gs.circlesOrArcs == gs.n:
			c.Type = core.ConstraintEqualRadius
			c.EntityA = gs.entity[0]
			for k := 1; k < len(gs.entity); k++ {
				c.EntityB = gs.entity[k]
				pending = append(pending, c)
			}
		case gs.arcs == 1 && gs.lineSegments == 1 && gs.n == 2:
			c.Type = core.ConstraintEqualLineArcLen
			if sk.Entity(gs.entity[0]).Type == core.EntityArcOfCircle {
				c.EntityA = gs.entity[1]
				c.EntityB = gs.entity[0]
			} else {
				c.EntityA = gs.entity[0]
				c.EntityB = gs.entity[1]
			}
			pending = append(pending, c)
		default:
			errorf("Bad selection for equal length / radius constraint. " +
				"This constraint can apply to:\n\n" +
				"    * two or more line segments (equal length)\n" +
				"    * two line segments and two points (equal point-line distances)\n" +
				"    * a line segment and two points (equal point-line distances)\n" +
				"    * a line segment, and a point and line segment " +
				"(point-line distance equals length)\n" +
				"    * two or more circles or arcs (equal radius)\n" +
				"    * a line segment and an arc (line segment length equals arc length)\n")
			return
		}
		ss.UndoRemember()
		for _, nc := range pending {
			if nc.Type == core.ConstraintEqualAngle {
				// Infer the nearest supplementary angle from the sketch.
				a1 := sk.Entity(c.EntityA).VectorGetNum(sk)
				b1 := sk.Entity(c.EntityB).VectorGetNum(sk)
				a2 := sk.Entity(c.EntityC).VectorGetNum(sk)
				b2 := sk.Entity(c.EntityD).VectorGetNum(sk)
				if a1.Dot(b1)*a2.Dot(b2) < 0 {
					nc.Other = true
				}
			}
			add(nc, false)
		}

	case CmdRatio, CmdDifference:
		ratio := id == CmdRatio
		switch {
		case gs.lineSegments == 2 && gs.n == 2:
			c.Type = core.ConstraintLengthDifference
			if ratio {
				c.Type = core.ConstraintLengthRatio
			}
			c.EntityA = gs.entity[0]
			c.EntityB = gs.entity[1]
		case gs.arcs == 2 && gs.n == 2:
			c.Type = core.ConstraintArcArcDifference
			if ratio {
				c.Type = core.ConstraintArcArcLenRatio
			}
			c.EntityA = gs.entity[0]
			c.EntityB = gs.entity[1]
		case gs.lineSegments == 1 && gs.arcs == 1 && gs.n == 2:
			c.Type = core.ConstraintArcLineDifference
			if ratio {
				c.Type = core.ConstraintArcLineLenRatio
			}
			if sk.Entity(gs.entity[0]).Type == core.EntityArcOfCircle {
				c.EntityA = gs.entity[1]
				c.EntityB = gs.entity[0]
			} else {
				c.EntityA = gs.entity[0]
				c.EntityB = gs.entity[1]
			}
		default:
			what := "difference"
			if ratio {
				what = "ratio"
			}
			errorf("Bad selection for length " + what + " constraint. This " +
				"constraint can apply to:\n\n" +
				"    * two line segments\n" +
				"    * two arcs\n" +
				"    * one arc and one line segment\n")
			return
		}
		c.ValA = 0
		c.ModifyToSatisfy(sk)
		add(c, true)

	case CmdAtMidpoint:
		switch {
		case gs.lineSegments == 1 && gs.points == 1 && gs.n == 2:
			c.Type = core.ConstraintAtMidpoint
			c.EntityA = gs.entity[0]
			c.PtA = gs.point[0]
			// If a point is at-midpoint, then no reason to also constrain
			// it on-line; so auto-remove that. Handle as one undo group.
			ss.UndoRemember()
			sk.DeleteAllConstraintsFor(core.ConstraintPtOnLine, c.EntityA, c.PtA)
			add(c, false)
		case gs.lineSegments == 1 && gs.workplanes == 1 && gs.n == 2:
			c.Type = core.ConstraintAtMidpoint
			i := 0
			if sk.Entity(gs.entity[0]).IsWorkplane() {
				i = 1
			}
			c.EntityA = gs.entity[i]
			c.EntityB = gs.entity[1-i]
			add(c, true)
		default:
			errorf("Bad selection for at midpoint constraint. This " +
				"constraint can apply to:\n\n" +
				"    * a line segment and a point (point at midpoint)\n" +
				"    * a line segment and a workplane (line's midpoint on plane)\n")
			return
		}

	case CmdSymmetric:
		switch {
		case gs.points == 2 && ((gs.workplanes == 1 && gs.n == 3) || gs.n == 2):
			if gs.entities > 0 {
				c.EntityA = gs.entity[0]
			}
			c.PtA = gs.point[0]
			c.PtB = gs.point[1]
			c.Type = core.ConstraintSymmetric
		case gs.lineSegments == 1 && ((gs.workplanes == 1 && gs.n == 2) || gs.n == 1):
			var line *core.Entity
			if sk.Entity(gs.entity[0]).IsWorkplane() {
				line = sk.Entity(gs.entity[1])
				c.EntityA = gs.entity[0]
			} else {
				line = sk.Entity(gs.entity[0])
			}
			c.PtA = line.Point[0]
			c.PtB = line.Point[1]
			c.Type = core.ConstraintSymmetric
		case gw.LockedInWorkplane() && gs.lineSegments == 2 && gs.n == 2:
			l0 := sk.Entity(gs.entity[0])
			l1 := sk.Entity(gs.entity[1])
			if l1.Group != gw.activeGroupH() || (l1.Construction && !l0.Construction) {
				l0, l1 = l1, l0
			}
			c.PtA = l1.Point[0]
			c.PtB = l1.Point[1]
			c.EntityA = l0.H
			c.Type = core.ConstraintSymmetricLine
		case gw.LockedInWorkplane() && gs.lineSegments == 1 && gs.points == 2 && gs.n == 3:
			c.PtA = gs.point[0]
			c.PtB = gs.point[1]
			c.EntityA = gs.entity[0]
			c.Type = core.ConstraintSymmetricLine
		default:
			errorf("Bad selection for symmetric constraint. This constraint " +
				"can apply to:\n\n" +
				"    * two points or a line segment " +
				"(symmetric about workplane's coordinate axis)\n" +
				"    * line segment, and two points or a line segment " +
				"(symmetric about line segment)\n" +
				"    * workplane, and two points or a line segment " +
				"(symmetric about workplane)\n")
			return
		}
		// We may remove constraints so remember manually
		if c.EntityA == core.NoEntity {
			// Horizontal / vertical symmetry, implicit symmetry plane normal
			// to the workplane
			if core.IsFreeIn3D(c.Workplane) {
				errorf("A workplane must be active when constraining " +
					"symmetric without an explicit symmetry plane.")
				return
			}
			pa := sk.Entity(c.PtA).PointGetNum(sk)
			pb := sk.Entity(c.PtB).PointGetNum(sk)
			dp := pa.Minus(pb)
			norm := sk.Entity(c.Workplane).NormalEntity(sk)
			u, v := norm.NormalU(sk), norm.NormalV(sk)
			if math.Abs(dp.Dot(u)) > math.Abs(dp.Dot(v)) {
				c.Type = core.ConstraintSymmetricHoriz
			} else {
				c.Type = core.ConstraintSymmetricVert
			}
			if gs.lineSegments == 1 {
				// If this line segment is already constrained horiz or
				// vert, then auto-remove that redundant constraint. Handle
				// as one undo group.
				ss.UndoRemember()
				sk.DeleteAllConstraintsFor(core.ConstraintHorizontal, gs.entity[0], core.NoEntity)
				sk.DeleteAllConstraintsFor(core.ConstraintVertical, gs.entity[0], core.NoEntity)
				add(c, false)
				break
			}
		}
		add(c, true)

	case CmdVertical, CmdHorizontal:
		if id == CmdHorizontal {
			c.Type = core.ConstraintHorizontal
		} else {
			c.Type = core.ConstraintVertical
		}
		if core.IsFreeIn3D(c.Workplane) {
			errorf("Activate a workplane (with Sketch -> In Workplane) before " +
				"applying a horizontal or vertical constraint.")
			return
		}
		var pending []core.Constraint
		switch {
		case gs.lineSegments > 0 && gs.lineSegments == gs.n:
			for _, enti := range gs.entity {
				c.EntityA = enti
				pending = append(pending, c)
			}
		case gs.points >= 2 && gs.n == gs.points:
			c.PtA = gs.point[0]
			for k := 1; k < gs.points; k++ {
				c.PtB = gs.point[k]
				pending = append(pending, c)
			}
		default:
			errorf("Bad selection for horizontal / vertical constraint. " +
				"This constraint can apply to:\n\n" +
				"    * two or more points\n" +
				"    * one or more line segments\n")
			return
		}
		ss.UndoRemember()
		for _, nc := range pending {
			add(nc, false)
		}

	case CmdOrientedSame:
		if gs.anyNormals == 2 && gs.n == 2 {
			c.Type = core.ConstraintSameOrientation
			c.EntityA = gs.anyNormal[0]
			c.EntityB = gs.anyNormal[1]
		} else {
			errorf("Bad selection for same orientation constraint. This " +
				"constraint can apply to:\n\n" +
				"    * two normals\n")
			return
		}
		ss.UndoRemember()

		nfree := sk.Entity(c.EntityA)
		nref := sk.Entity(c.EntityB)
		if nref.Group == gw.activeGroupH() {
			nref, nfree = nfree, nref
		}
		if nfree.Group == gw.activeGroupH() && nref.Group != gw.activeGroupH() {
			// nfree is free, and nref is locked (since it came from a
			// previous group); so let's force nfree aligned to nref, and
			// make convergence easy
			ru, rv := nref.NormalU(sk), nref.NormalV(sk)
			fu, fv := nfree.NormalU(sk), nfree.NormalV(sk)

			if math.Abs(fu.Dot(ru)) < math.Abs(fu.Dot(rv)) {
				// There might be an odd*90 degree rotation about the normal
				// vector; allow that, since the numerical constraint does
				ru, rv = rv, ru
			}
			if fu.Dot(ru) > 0 {
				fu = ru
			} else {
				fu = ru.ScaledBy(-1)
			}
			if fv.Dot(rv) > 0 {
				fv = rv
			} else {
				fv = rv.ScaledBy(-1)
			}
			nfree.NormalForceTo(sk, geom.QuaternionFromBasis(fu, fv))
		}
		add(c, false)

	case CmdOtherAngle:
		if gs.constraints == 1 && gs.n == 0 {
			cc := sk.Constraint(gs.constraint[0])
			if cc.Type == core.ConstraintAngle {
				ss.UndoRemember()
				cc = sk.Constraint(gs.constraint[0])
				cc.Other = !cc.Other
				cc.ModifyToSatisfy(sk)
				break
			}
			if cc.Type == core.ConstraintEqualAngle {
				ss.UndoRemember()
				cc = sk.Constraint(gs.constraint[0])
				cc.Other = !cc.Other
				sk.MarkGroupDirty(cc.Group, false)
				break
			}
		}
		errorf("Must select an angle constraint.")
		return

	case CmdReference:
		if gs.constraints == 1 && gs.n == 0 {
			cc := sk.Constraint(gs.constraint[0])
			if cc.HasLabel() && cc.Type != core.ConstraintComment {
				ss.UndoRemember()
				cc = sk.Constraint(gs.constraint[0])
				cc.Reference = !cc.Reference
				sk.MarkGroupDirty(cc.Group, true)
				break
			}
		}
		errorf("Must select a constraint with associated label.")
		return

	case CmdAngle, CmdRefAngle:
		switch {
		case gs.vectors == 3 && gs.n == 3:
			c.Type = core.ConstraintEqualAngle
			c.EntityA = gs.vector[0]
			c.EntityB = gs.vector[1]
			c.EntityC = gs.vector[1]
			c.EntityD = gs.vector[2]
		case gs.vectors == 4 && gs.n == 4:
			c.Type = core.ConstraintEqualAngle
			c.EntityA = gs.vector[0]
			c.EntityB = gs.vector[1]
			c.EntityC = gs.vector[2]
			c.EntityD = gs.vector[3]
		case gs.vectors == 2 && gs.n == 2:
			c.Type = core.ConstraintAngle
			c.EntityA = gs.vector[0]
			c.EntityB = gs.vector[1]
			c.ValA = 0
		default:
			errorf("Bad selection for angle constraint. This constraint " +
				"can apply to:\n\n" +
				"Angle between:\n" +
				"    * two line segments\n" +
				"    * a line segment and a normal\n" +
				"    * two normals\n" +
				"\nEqual angles:\n" +
				"    * four line segments or normals " +
				"(equal angle between A,B and C,D)\n" +
				"    * three line segments or normals " +
				"(equal angle between A,B and B,C)\n")
			return
		}

		ea := sk.Entity(c.EntityA)
		eb := sk.Entity(c.EntityB)
		if ea.Type == core.EntityLineSegment && eb.Type == core.EntityLineSegment {
			a0 := sk.Entity(ea.Point[0]).PointGetNum(sk)
			a1 := sk.Entity(ea.Point[1]).PointGetNum(sk)
			b0 := sk.Entity(eb.Point[0]).PointGetNum(sk)
			b1 := sk.Entity(eb.Point[1]).PointGetNum(sk)
			switch {
			case a0.Equals(b0) || a1.Equals(b1):
				// okay, vectors should be drawn in same sense
			case a0.Equals(b1) || a1.Equals(b0):
				// vectors are in opposite sense
				c.Other = true
			}
			// Otherwise no shared point; not clear which intersection to
			// draw.
		}
		if id == CmdRefAngle {
			c.Reference = true
		}
		c.ModifyToSatisfy(sk)
		add(c, true)

	case CmdParallel:
		var pending []core.Constraint
		switch {
		case gs.faces == 2 && gs.n == 2:
			c.Type = core.ConstraintParallel
			c.EntityA = gs.face[0]
			c.EntityB = gs.face[1]
			pending = append(pending, c)
		case gs.vectors > 1 && gs.vectors == gs.n:
			c.Type = core.ConstraintParallel
			c.EntityA = gs.vector[0]
			for k := 1; k < len(gs.vector); k++ {
				c.EntityB = gs.vector[k]
				pending = append(pending, c)
			}
		case gs.lineSegments == 1 && gs.arcs == 1 && (gs.n == 2 || (gs.points == 1 && gs.n == 3)):
			line := sk.Entity(gs.entity[0])
			arc := sk.Entity(gs.entity[1])
			if line.Type == core.EntityArcOfCircle {
				line, arc = arc, line
			}
			var ep *core.Entity
			if gs.points == 1 {
				ep = sk.Entity(gs.point[0])
			}
			if err := sk.ConstrainArcLineTangent(&c, line, arc, ep); err != nil {
				ss.Report(err)
				return
			}
			c.Type = core.ConstraintArcLineTangent
			c.EntityA = arc.H
			c.EntityB = line.H
			pending = append(pending, c)
		case gs.lineSegments == 1 && gs.cubics == 1 && (gs.n == 2 || (gs.points == 1 && gs.n == 3)):
			line := sk.Entity(gs.entity[0])
			cubic := sk.Entity(gs.entity[1])
			if line.Type == core.EntityCubic {
				line, cubic = cubic, line
			}
			var ep *core.Entity
			if gs.points == 1 {
				ep = sk.Entity(gs.point[0])
			}
			if err := sk.ConstrainCubicLineTangent(&c, line, cubic, ep); err != nil {
				ss.Report(err)
				return
			}
			c.Type = core.ConstraintCubicLineTangent
			c.EntityA = cubic.H
			c.EntityB = line.H
			pending = append(pending, c)
		case gs.cubics+gs.arcs == 2 && (gs.n == 2 || (gs.points == 2 && gs.n == 4)):
			if !gw.LockedInWorkplane() {
				errorf("Curve-curve tangency must apply in workplane.")
				return
			}
			eA := sk.Entity(gs.entity[0])
			eB := sk.Entity(gs.entity[1])
			var epA, epB *core.Entity
			if gs.points == 2 {
				epA = sk.Entity(gs.point[0])
				epB = sk.Entity(gs.point[1])
			}
			if err := sk.ConstrainCurveCurveTangent(&c, eA, eB, epA, epB); err != nil {
				ss.Report(err)
				return
			}
			c.Type = core.ConstraintCurveCurveTangent
			c.EntityA = eA.H
			c.EntityB = eB.H
			pending = append(pending, c)
		default:
			errorf("Bad selection for parallel / tangent constraint. This " +
				"constraint can apply to:\n\n" +
				"    * two faces\n" +
				"    * two or more line segments (parallel)\n" +
				"    * one or more line segments and one or more normals (parallel)\n" +
				"    * two or more normals (parallel)\n" +
				"    * two line segments, arcs, or beziers, that share " +
				"an endpoint (tangent)\n" +
				"    * two line segments, arcs, or beziers, that do not share " +
				"an endpoint and the end point(s) of the curve(s) (tangent)\n")
			return
		}
		ss.UndoRemember()
		for _, nc := range pending {
			add(nc, false)
		}

	case CmdPerpendicular:
		switch {
		case gs.faces == 2 && gs.n == 2:
			c.Type = core.ConstraintPerpendicular
			c.EntityA = gs.face[0]
			c.EntityB = gs.face[1]
		case gs.vectors == 2 && gs.n == 2:
			c.Type = core.ConstraintPerpendicular
			c.EntityA = gs.vector[0]
			c.EntityB = gs.vector[1]
		default:
			errorf("Bad selection for perpendicular constraint. This " +
				"constraint can apply to:\n\n" +
				"    * two faces\n" +
				"    * two line segments\n" +
				"    * a line segment and a normal\n" +
				"    * two normals\n")
			return
		}
		add(c, true)

	case CmdWhereDragged:
		if gs.points == 1 && gs.n == 1 {
			c.Type = core.ConstraintWhereDragged
			c.PtA = gs.point[0]
		} else {
			errorf("Bad selection for lock point where dragged constraint. " +
				"This constraint can apply to:\n\n" +
				"    * a point\n")
			return
		}
		add(c, true)

	case CmdComment:
		if gs.points == 1 && gs.n == 1 {
			c.Type = core.ConstraintComment
			c.PtA = gs.point[0]
			c.Group = gw.activeGroupH()
			c.Workplane = gw.ActiveWorkplane()
			c.Comment = "NEW COMMENT -- DOUBLE-CLICK TO EDIT"
			add(c, true)
		} else {
			gw.pending.operation = PendingCommand
			gw.pending.command = CmdComment
			gw.pending.description = "click center of comment text"
			ss.ScheduleShowTW()
		}

	default:
		panic("ui: unexpected menu ID")
	}

	for _, nc := range newcons {
		for i := range sk.Constraints {
			cc := &sk.Constraints[i]
			// The bases are compared, and not the label or the offset.
			if nc.H != cc.H && nc.ConstraintBase.Equals(&cc.ConstraintBase) { //nolint:staticcheck // QF1008: the base is named on purpose
				// Oops, we already have this exact constraint. Remove the
				// one we just added.
				sk.RemoveConstraint(nc.H)
				gw.ClearSelection()
				// And now select the old one, to give feedback.
				gw.MakeSelectedConstraint(cc.H)
				return
			}
		}

		if constraint := sk.Constraint(nc.H); constraint != nil {
			if sk.TestRankForGroup(nc.Group, nil) == core.SolveRedundantOkay &&
				!sk.Group(gw.activeGroupH()).AllowRedundant && constraint.HasLabel() {
				constraint.Reference = true
			}
		}

		if (id == CmdDistanceDia || id == CmdAngle || id == CmdRatio || id == CmdDifference) &&
			ss.ImmediatelyEditDimension {
			gw.EditConstraint(nc.H)
		}
	}

	gw.ClearSelection()
}
