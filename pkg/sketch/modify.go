package sketch

import (
	"errors"
	"math"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/libm"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// This file ports the editing tools of solvespace/src/modify.cpp:
// rounding a corner with a tangent arc, and splitting lines and curves
// where they intersect. The selection is passed in, and C++'s error
// messages are returned instead of shown.

// TangentArcSettings are the user's settings for MakeTangentArc (C++
// SS.tangentArcManual, tangentArcRadius and tangentArcModify).
type TangentArcSettings struct {
	// Manual uses Radius; otherwise the radius is chosen automatically.
	Manual bool
	Radius float64
	// Modify trims the original entities instead of making them
	// construction and adding trimmed copies.
	Modify bool
}

// isReasonable is C++ IsReasonable, which despite its name is true for
// NaN and huge values.
func isReasonable(x float64) bool {
	return math.IsNaN(x) || x > 1e11 || x < -1e11
}

// LockedInWorkplane is whether the active group sketches in a workplane
// (C++ GraphicsWindow::LockedInWorkplane).
func (sk *Sketch) LockedInWorkplane() bool {
	return sk.ActiveWorkplane() != core.FreeIn3D
}

// parametricCurve is a line or arc by its parametric equation, for
// finding tangent arcs numerically (C++ GraphicsWindow::ParametricCurve).
type parametricCurve struct {
	isLine                    bool
	p0, p1                    geom.Vector
	u, v                      geom.Vector
	r, theta0, theta1, dtheta float64
}

func (sk *Sketch) makeParametricCurve(he core.HEntity, reverse bool) parametricCurve {
	pc := parametricCurve{}
	e := sk.MustGetEntity(he)
	if e.Type == core.EntityLineSegment {
		pc.isLine = true
		pc.p0 = e.EndpointStart(sk)
		pc.p1 = e.EndpointFinish(sk)
		if reverse {
			pc.p0, pc.p1 = pc.p1, pc.p0
		}
	} else if e.Type == core.EntityArcOfCircle {
		pc.isLine = false
		pc.p0 = sk.MustGetEntity(e.Point[0]).PointGetNum(sk)
		pe := sk.MustGetEntity(e.Point[1]).PointGetNum(sk)
		pc.r = pe.Minus(pc.p0).Magnitude()
		pc.theta0, pc.theta1, pc.dtheta = e.ArcGetAngles(sk)
		if reverse {
			pc.theta0, pc.theta1 = pc.theta1, pc.theta0
			pc.dtheta = -pc.dtheta
		}
		wrkpln := sk.MustGetEntity(e.Workplane).NormalEntity(sk)
		pc.u = wrkpln.NormalU(sk)
		pc.v = wrkpln.NormalV(sk)
	} else {
		panic("Unexpected entity type")
	}
	return pc
}

func (pc *parametricCurve) lengthForAuto() float64 {
	if pc.isLine {
		// Allow a third of the line to disappear with auto radius
		return pc.p1.Minus(pc.p0).Magnitude() / 3
	}
	// But only a twentieth of the arc; shorter means fewer numerical
	// problems since the curve is more linear over shorter sections.
	return (math.Abs(pc.dtheta) * pc.r) / 20
}

func (pc *parametricCurve) pointAt(t float64) geom.Vector {
	if pc.isLine {
		return pc.p0.Plus(pc.p1.Minus(pc.p0).ScaledBy(t))
	}
	theta := pc.theta0 + pc.dtheta*t
	return pc.p0.Plus(pc.u.ScaledBy(pc.r * libm.Cos(theta)).Plus(pc.v.ScaledBy(pc.r * libm.Sin(theta))))
}

func (pc *parametricCurve) tangentAt(t float64) geom.Vector {
	if pc.isLine {
		return pc.p1.Minus(pc.p0)
	}
	theta := pc.theta0 + pc.dtheta*t
	tv := pc.u.ScaledBy(-pc.r * libm.Sin(theta)).Plus(pc.v.ScaledBy(pc.r * libm.Cos(theta)))
	tv = tv.ScaledBy(pc.dtheta)
	return tv
}

// createRequestTrimmedTo changes or copies entity orig and connects it
// to arc: at t on this curve, to the arc's finish if arcFinish. With
// reuseOrig, orig's finish is moved if pointf, else its start (C++
// ParametricCurve::CreateRequestTrimmedTo).
func (sk *Sketch) createRequestTrimmedTo(pc *parametricCurve, t float64,
	reuseOrig bool, orig, arc core.HEntity, arcFinish, pointf bool) {
	var e *core.Entity
	if pc.isLine {
		if reuseOrig {
			e = sk.MustGetEntity(orig)
			i := 0
			if pointf {
				i = 1
			}
			sk.MustGetEntity(e.Point[i]).PointForceTo(sk, pc.pointAt(t))
			sk.constrainPointIfCoincident(e.Point[i])
		} else {
			hr := sk.AddRequestOfType(RequestLineSegment, false)
			e = sk.MustGetEntity(hr.Entity(0))
			sk.MustGetEntity(e.Point[0]).PointForceTo(sk, pc.pointAt(t))
			sk.MustGetEntity(e.Point[1]).PointForceTo(sk, pc.pointAt(1))
			sk.constrainPointIfCoincident(e.Point[0])
			sk.constrainPointIfCoincident(e.Point[1])
			sk.Constrain(core.ConstraintPtOnLine,
				hr.Entity(1), core.NoEntity, orig, core.NoEntity, false, false)
		}
		sk.Constrain(core.ConstraintArcLineTangent,
			core.NoEntity, core.NoEntity,
			arc, e.H, arcFinish, false)
	} else {
		if reuseOrig {
			e = sk.MustGetEntity(orig)
			i := 1
			if pointf {
				i = 2
			}
			sk.MustGetEntity(e.Point[i]).PointForceTo(sk, pc.pointAt(t))
			sk.constrainPointIfCoincident(e.Point[i])
		} else {
			hr := sk.AddRequestOfType(RequestArcOfCircle, false)
			e = sk.MustGetEntity(hr.Entity(0))
			sk.MustGetEntity(e.Point[0]).PointForceTo(sk, pc.p0)
			if pc.dtheta > 0 {
				sk.MustGetEntity(e.Point[1]).PointForceTo(sk, pc.pointAt(t))
				sk.MustGetEntity(e.Point[2]).PointForceTo(sk, pc.pointAt(1))
			} else {
				sk.MustGetEntity(e.Point[2]).PointForceTo(sk, pc.pointAt(t))
				sk.MustGetEntity(e.Point[1]).PointForceTo(sk, pc.pointAt(1))
			}
			sk.constrainPointIfCoincident(e.Point[0])
			sk.constrainPointIfCoincident(e.Point[1])
			sk.constrainPointIfCoincident(e.Point[2])
		}
		// The tangency constraint alone is enough to fully constrain it,
		// so there's no need for more.
		sk.Constrain(core.ConstraintCurveCurveTangent,
			core.NoEntity, core.NoEntity,
			arc, e.H, arcFinish, pc.dtheta < 0)
	}
}

// constrainPointIfCoincident constrains hpt coincident to a point in its
// group and workplane that is numerically coincident with it, if one
// exists (C++ ParametricCurve::ConstrainPointIfCoincident).
func (sk *Sketch) constrainPointIfCoincident(hpt core.HEntity) {
	pt := sk.MustGetEntity(hpt)
	ptv := pt.PointGetNum(sk)

	// C++ walks the entities in handle order and takes the first.
	var found core.HEntity
	for i := range sk.Entities {
		e := &sk.Entities[i]
		if e.H == pt.H {
			continue
		}
		if !e.IsPoint() {
			continue
		}
		if e.Group != pt.Group {
			continue
		}
		if e.Workplane != pt.Workplane {
			continue
		}

		ev := e.PointGetNum(sk)
		if !ev.Equals(ptv) {
			continue
		}

		if found.V == 0 || e.H.V < found.V {
			found = e.H
		}
	}
	if found.V != 0 {
		sk.ConstrainCoincident(hpt, found)
	}
}

// MakeTangentArc rounds the corner at point hpt, where two
// non-construction line segments or arcs join, with a tangent arc (C++
// GraphicsWindow::MakeTangentArc).
func (sk *Sketch) MakeTangentArc(hpt core.HEntity, settings TangentArcSettings) error {
	if !sk.LockedInWorkplane() {
		return errors.New("Must be sketching in workplane to create tangent arc.")
	}

	// The point corresponding to the vertex to be rounded.
	pshared := sk.MustGetEntity(hpt).PointGetNum(sk)

	// First, find two requests (that are not construction, and that are
	// in our group and workplane) that generate entities that have an
	// endpoint at our vertex to be rounded.
	c := 0
	var hreq [2]core.HRequest
	var hent [2]core.HEntity
	var pointf [2]bool
	for i := range sk.Requests {
		r := &sk.Requests[i]
		if r.Group != sk.ActiveGroup() {
			continue
		}
		if r.Workplane != sk.ActiveWorkplane() {
			continue
		}
		if r.Construction {
			continue
		}
		if r.Type != RequestLineSegment && r.Type != RequestArcOfCircle {
			continue
		}

		e := sk.MustGetEntity(r.H.Entity(0))
		ps := e.EndpointStart(sk)
		pf := e.EndpointFinish(sk)

		if ps.Equals(pshared) || pf.Equals(pshared) {
			if c < 2 {
				// We record the entity and request handles, and whether
				// the vertex to be rounded is the start or finish of
				// this entity.
				hent[c] = e.H
				hreq[c] = r.H
				pointf[c] = pf.Equals(pshared)
			}
			c++
		}
	}
	if c != 2 {
		return errors.New("To create a tangent arc, select a point where two " +
			"non-construction lines or circles in this group and " +
			"workplane join.")
	}

	wrkpl := sk.MustGetEntity(sk.ActiveWorkplane())
	wn := wrkpl.NormalEntity(sk).NormalN(sk)

	// Based on these two entities, we make the objects that we'll use to
	// numerically find the tangent arc.
	var pc [2]parametricCurve
	pc[0] = sk.makeParametricCurve(hent[0], pointf[0])
	pc[1] = sk.makeParametricCurve(hent[1], pointf[1])

	var pinter geom.Vector
	r, vv := 0.0, 0.0
	// We now do Newton iterations to find the tangent arc, and its
	// positions t back along the two curves, starting from shared point
	// of the curves at t = 0. Lots of iterations helps convergence, and
	// this is still ~10 ms for everything.
	iters := 1000
	t := [2]float64{0, 0}
	var tp [2]float64
	for i := 0; i < iters+20; i++ {
		p0 := pc[0].pointAt(t[0])
		p1 := pc[1].pointAt(t[1])
		t0 := pc[0].tangentAt(t[0])
		t1 := pc[1].tangentAt(t[1])

		pinter = geom.AtIntersectionOfLines(p0, p0.Plus(t0), p1, p1.Plus(t1))

		// The sign of vv determines whether shortest distance is
		// clockwise or anti-clockwise.
		v := wn.Cross(t0).WithMagnitude(1)
		vv = t1.Dot(v)

		dot := t0.WithMagnitude(1).Dot(t1.WithMagnitude(1))
		theta := libm.Acos(dot)

		if settings.Manual {
			r = settings.Radius
		} else {
			r = 200 / viewScale
			// Set the radius so that no more than one third of the
			// line segment disappears.
			r = geom.Min(r, pc[0].lengthForAuto()*libm.Tan(theta/2))
			r = geom.Min(r, pc[1].lengthForAuto()*libm.Tan(theta/2))
		}
		// We are source-stepping the radius, to improve convergence. So
		// ramp that for most of the iterations, and then do a few at
		// the end with that constant for polishing.
		if i < iters {
			r *= 0.1 + 0.9*float64(i)/float64(iters)
		}

		// The distance from the intersection of the lines to the endpoint
		// of the arc, along each line.
		el := r / libm.Tan(theta/2)

		// Compute the endpoints of the arc, for each curve
		pa0 := pinter.Plus(t0.WithMagnitude(el))
		pa1 := pinter.Plus(t1.WithMagnitude(el))

		tp[0] = t[0]
		tp[1] = t[1]

		// And convert those points to parameter values along the curve.
		t[0] += pa0.Minus(p0).DivProjected(t0)
		t[1] += pa1.Minus(p1).DivProjected(t1)
	}

	// Stupid check for convergence, and for an out of range result (as
	// we would get, for example, if the line is too short to fit the
	// rounding arc).
	if math.Abs(tp[0]-t[0]) > 1e-3 || math.Abs(tp[1]-t[1]) > 1e-3 ||
		t[0] < 0.01 || t[1] < 0.01 ||
		t[0] > 0.99 || t[1] > 0.99 ||
		isReasonable(t[0]) || isReasonable(t[1]) {
		return errors.New("Couldn't round this corner. Try a smaller radius, or try " +
			"creating the desired geometry by hand with tangency " +
			"constraints.")
	}

	// Compute the location of the center of the arc
	center := pc[0].pointAt(t[0])
	v0inter := pinter.Minus(center)
	var a, b int
	if vv < 0 {
		a, b = 1, 2
		center = center.Minus(v0inter.Cross(wn).WithMagnitude(r))
	} else {
		a, b = 2, 1
		center = center.Plus(v0inter.Cross(wn).WithMagnitude(r))
	}

	sk.UndoRemember()

	if settings.Modify {
		// Delete the coincident constraint for the removed point.
		kept := sk.Constraints[:0]
		for _, cs := range sk.Constraints {
			if cs.Group == sk.ActiveGroup() &&
				cs.Workplane == sk.ActiveWorkplane() &&
				cs.Type == core.ConstraintPointsCoincident &&
				sk.MustGetEntity(cs.PtA).PointGetNum(sk).Equals(pshared) {
				continue
			}
			kept = append(kept, cs)
		}
		sk.Constraints = kept
	} else {
		// Make the original entities construction, or delete them
		// entirely, according to user preference.
		sk.GetRequest(hreq[0]).Construction = true
		sk.GetRequest(hreq[1]).Construction = true
	}

	// Create and position the new tangent arc.
	harc := sk.AddRequestOfType(RequestArcOfCircle, false)
	earc := sk.MustGetEntity(harc.Entity(0))
	hearc := earc.H

	sk.MustGetEntity(earc.Point[0]).PointForceTo(sk, center)
	sk.MustGetEntity(earc.Point[a]).PointForceTo(sk, pc[0].pointAt(t[0]))
	sk.MustGetEntity(earc.Point[b]).PointForceTo(sk, pc[1].pointAt(t[1]))

	// Modify or duplicate the original entities and connect them to the
	// tangent arc.
	sk.createRequestTrimmedTo(&pc[0], t[0], settings.Modify,
		hent[0], hearc, b == 1, pointf[0])
	sk.createRequestTrimmedTo(&pc[1], t[1], settings.Modify,
		hent[1], hearc, a == 1, pointf[1])
	return nil
}

// SplitLine splits line segment he at pinter into two, and returns the
// point where they join (C++ GraphicsWindow::SplitLine).
func (sk *Sketch) SplitLine(he core.HEntity, pinter geom.Vector) core.HEntity {
	// Save the original endpoints, since we're about to delete this
	// entity.
	e01 := sk.MustGetEntity(he)
	hep0, hep1 := e01.Point[0], e01.Point[1]
	p0 := sk.MustGetEntity(hep0).PointGetNum(sk)
	p1 := sk.MustGetEntity(hep1).PointGetNum(sk)

	if p0.Equals(pinter) {
		return hep0
	}

	if p1.Equals(pinter) {
		return hep1
	}

	// Add the two line segments this one gets split into.
	r0i := sk.AddRequestOfType(RequestLineSegment, false)
	ri1 := sk.AddRequestOfType(RequestLineSegment, false)

	e0i := sk.MustGetEntity(r0i.Entity(0))
	ei1 := sk.MustGetEntity(ri1.Entity(0))
	e0iPt, ei1Pt := e0i.Point, ei1.Point

	sk.MustGetEntity(e0iPt[0]).PointForceTo(sk, p0)
	sk.MustGetEntity(e0iPt[1]).PointForceTo(sk, pinter)
	sk.MustGetEntity(ei1Pt[0]).PointForceTo(sk, pinter)
	sk.MustGetEntity(ei1Pt[1]).PointForceTo(sk, p1)

	sk.ReplacePointInConstraints(hep0, e0iPt[0])
	sk.ReplacePointInConstraints(hep1, ei1Pt[1])
	sk.ConstrainCoincident(e0iPt[1], ei1Pt[0])
	return e0iPt[1]
}

// SplitCircle splits circle or arc he at pinter: a circle into a 360
// degree arc, an arc into two. It returns the point at the split (C++
// GraphicsWindow::SplitCircle).
func (sk *Sketch) SplitCircle(he core.HEntity, pinter geom.Vector) core.HEntity {
	circle := sk.MustGetEntity(he)
	if circle.Type == core.EntityCircle {
		// Start with an unbroken circle, split it into a 360 degree arc.
		center := sk.MustGetEntity(circle.Point[0]).PointGetNum(sk)

		hr := sk.AddRequestOfType(RequestArcOfCircle, false)

		arcPt := sk.MustGetEntity(hr.Entity(0)).Point

		sk.MustGetEntity(arcPt[0]).PointForceTo(sk, center)
		sk.MustGetEntity(arcPt[1]).PointForceTo(sk, pinter)
		sk.MustGetEntity(arcPt[2]).PointForceTo(sk, pinter)

		sk.ConstrainCoincident(arcPt[1], arcPt[2])
		return arcPt[1]
	}

	// Start with an arc, break it in to two arcs
	hc, hs, hf := circle.Point[0], circle.Point[1], circle.Point[2]
	center := sk.MustGetEntity(hc).PointGetNum(sk)
	start := sk.MustGetEntity(hs).PointGetNum(sk)
	finish := sk.MustGetEntity(hf).PointGetNum(sk)

	hr0 := sk.AddRequestOfType(RequestArcOfCircle, false)
	hr1 := sk.AddRequestOfType(RequestArcOfCircle, false)

	arc0Pt := sk.MustGetEntity(hr0.Entity(0)).Point
	arc1Pt := sk.MustGetEntity(hr1.Entity(0)).Point

	sk.MustGetEntity(arc0Pt[0]).PointForceTo(sk, center)
	sk.MustGetEntity(arc0Pt[1]).PointForceTo(sk, start)
	sk.MustGetEntity(arc0Pt[2]).PointForceTo(sk, pinter)

	sk.MustGetEntity(arc1Pt[0]).PointForceTo(sk, center)
	sk.MustGetEntity(arc1Pt[1]).PointForceTo(sk, pinter)
	sk.MustGetEntity(arc1Pt[2]).PointForceTo(sk, finish)

	sk.ReplacePointInConstraints(hs, arc0Pt[1])
	sk.ReplacePointInConstraints(hf, arc1Pt[2])
	sk.ConstrainCoincident(arc0Pt[2], arc1Pt[1])
	return arc0Pt[2]
}

// SplitCubic splits cubic he at pinter into cubic segments, and returns
// the point at the split (C++ GraphicsWindow::SplitCubic).
func (sk *Sketch) SplitCubic(he core.HEntity, pinter geom.Vector) core.HEntity {
	// Save the original endpoints, since we're about to delete this
	// entity.
	e01 := sk.MustGetEntity(he)
	var sbl srf.SBezierList
	GenerateEntityBeziers(e01, &sbl, &Runtime{Sketch: sk})

	hep0 := e01.Point[0]
	hep1 := e01.Point[3+e01.ExtraPoints]
	var hep0n, hep1n, hepin core.HEntity // the new start and finish points, and the intersection point

	// The curve may consist of multiple cubic segments. So find which
	// one contains the intersection point.
	for i := range sbl.Curves {
		sb := &sbl.Curves[i]
		if sb.Degree != 3 {
			panic("Expected a cubic bezier")
		}

		t, _ := sb.ClosestPointTo(pinter, false)
		if pinter.Equals(sb.PointAt(t)) {
			// Split that segment at the intersection.
			var b0i, bi1 srf.SBezier
			b01 := *sb
			b01.SplitAt(t, &b0i, &bi1)

			// Add the two cubic segments this one gets split into.
			r0i := sk.AddRequestOfType(RequestCubic, false)
			ri1 := sk.AddRequestOfType(RequestCubic, false)

			e0iPt := sk.MustGetEntity(r0i.Entity(0)).Point
			ei1Pt := sk.MustGetEntity(ri1.Entity(0)).Point

			for j := 0; j <= 3; j++ {
				sk.MustGetEntity(e0iPt[j]).PointForceTo(sk, b0i.Ctrl[j])
			}
			for j := 0; j <= 3; j++ {
				sk.MustGetEntity(ei1Pt[j]).PointForceTo(sk, bi1.Ctrl[j])
			}

			sk.ConstrainCoincident(e0iPt[3], ei1Pt[0])
			if i == 0 {
				hep0n = e0iPt[0]
			}
			hep1n = ei1Pt[3]
			hepin = e0iPt[3]
		} else {
			r := sk.AddRequestOfType(RequestCubic, false)
			ePt := sk.MustGetEntity(r.Entity(0)).Point

			for j := 0; j <= 3; j++ {
				sk.MustGetEntity(ePt[j]).PointForceTo(sk, sb.Ctrl[j])
			}

			if i == 0 {
				hep0n = ePt[0]
			}
			hep1n = ePt[3]
		}
	}

	sk.ReplacePointInConstraints(hep0, hep0n)
	sk.ReplacePointInConstraints(hep1, hep1n)
	return hepin
}

// SplitEntity splits line, circle or cubic he at pinter, deletes the
// request it came from (unless that's construction or in another
// group), and returns the point at the split (C++
// GraphicsWindow::SplitEntity).
func (sk *Sketch) SplitEntity(he core.HEntity, pinter geom.Vector) (core.HEntity, error) {
	e := sk.MustGetEntity(he)

	var ret core.HEntity
	if e.IsCircle() {
		ret = sk.SplitCircle(he, pinter)
	} else if e.Type == core.EntityLineSegment {
		ret = sk.SplitLine(he, pinter)
	} else if e.Type == core.EntityCubic || e.Type == core.EntityCubicPeriodic {
		ret = sk.SplitCubic(he, pinter)
	} else {
		return core.NoEntity, errors.New("Couldn't split this entity; lines, circles, or cubics only.")
	}

	// Finally, delete the request that generated the original entity.
	if he.IsFromRequest() && he == he.Request().Entity(0) {
		hr := he.Request()
		// Only delete the original request if we actually made a split
		// (i.e. the split point is not from the original request)
		if hr != ret.Request() {
			r := sk.GetRequest(hr)
			// If the user wants to keep the old entities around, they can
			// just mark them construction first.
			if r.Group == sk.ActiveGroup() && !r.Construction {
				r.Tag = 1
				sk.DeleteTaggedRequests()
			}
		}
	}

	return ret, nil
}

// SplitLinesOrCurves splits the selected entities where they intersect:
// two lines, circles, arcs or cubics, or one of them and a point on it.
// Of several intersections it takes the one the view shows closest to
// mouse, in the default view's screen coordinates (C++
// GraphicsWindow::SplitLinesOrCurves).
func (sk *Sketch) SplitLinesOrCurves(selection []Selection, mouse geom.Point2d) error {
	if !sk.LockedInWorkplane() {
		return errors.New("Must be sketching in workplane to split.")
	}

	// C++ GraphicsWindow::GroupSelection, the counts used here.
	var points, lineSegments, circlesOrArcs, cubics, periodicCubics int
	var point, entity []core.HEntity
	for _, s := range selection {
		if s.Entity.V == 0 {
			continue
		}
		e := sk.MustGetEntity(s.Entity)
		if e.IsPoint() {
			points++
			point = append(point, s.Entity)
		} else {
			entity = append(entity, s.Entity)
		}
		switch e.Type {
		case core.EntityLineSegment:
			lineSegments++
		case core.EntityCubic:
			cubics++
		case core.EntityCubicPeriodic:
			periodicCubics++
		case core.EntityArcOfCircle, core.EntityCircle:
			circlesOrArcs++
		}
	}
	n := lineSegments + circlesOrArcs + cubics + periodicCubics
	if !((n == 2 && points == 0) || (n == 1 && points == 1)) {
		return errors.New("Select two entities that intersect each other " +
			"(e.g. two lines/circles/arcs or a line/circle/arc and a point).")
	}

	splitAtPoint := points == 1
	ha := entity[0]
	var hb core.HEntity
	if splitAtPoint {
		hb = point[0]
	} else {
		hb = entity[1]
	}

	ea := sk.MustGetEntity(ha)
	eb := sk.MustGetEntity(hb)
	var pi geom.Vector

	// First, decide the point where we're going to make the split.
	foundInters := false
	if splitAtPoint {
		// One of the entities is a point, and this point must be on the
		// other entity. Verify that a corresponding point-coincident
		// constraint exists for the point/entity.
		var p0, p1 geom.Vector
		if ea.Type == core.EntityLineSegment {
			p0 = ea.EndpointStart(sk)
			p1 = ea.EndpointFinish(sk)
		}

		for i := range sk.Constraints {
			sk.Constraints[i].Tag = 0
		}

		for i := range sk.Constraints {
			c := &sk.Constraints[i]
			if c.PtA.Request() == hb.Request() &&
				c.EntityA.Request() == ha.Request() {
				pi = sk.MustGetEntity(c.PtA).PointGetNum(sk)

				if ea.Type == core.EntityLineSegment && !pi.OnLineSegment(p0, p1, geom.LengthEps) {
					// The point isn't between line endpoints, so there
					// isn't an actual intersection.
					continue
				}

				c.Tag = 1
				foundInters = true
				break
			}
		}
	} else {
		// Compute the possibly-rational Bezier curves for each of these
		// non-point entities...
		rt := &Runtime{Sketch: sk}
		var sbla, sblb srf.SBezierList
		GenerateEntityBeziers(ea, &sbla, rt)
		GenerateEntityBeziers(eb, &sblb, rt)
		// ... and then compute the points where they intersect, based on
		// those curves.
		var inters srf.SPointList
		sbla.AllIntersectionsWith(&sblb, &inters)

		// If there's multiple points, then take the one closest to the
		// mouse pointer.
		if len(inters.Points) > 0 {
			dmin := geom.VeryPositive
			for _, sp := range inters.Points {
				d := ProjectPoint(sp.P).DistanceTo(mouse)
				if d < dmin {
					dmin = d
					pi = sp.P
				}
			}

			foundInters = true
		}
	}

	// Then, actually split the entities.
	if !foundInters {
		return errors.New("Can't split; no intersection found.")
	}

	sk.UndoRemember()

	// Remove any constraints we're going to replace.
	kept := sk.Constraints[:0]
	for _, c := range sk.Constraints {
		if c.Tag == 0 {
			kept = append(kept, c)
		}
	}
	sk.Constraints = kept

	// C++ shows SplitEntity's error and carries on.
	hia, err := sk.SplitEntity(ha, pi)
	// SplitEntity adds the coincident constraints to join the split
	// halves of each original entity; and then we add the constraint to
	// join the two entities together at the split point.
	if splitAtPoint {
		// Remove datum point, as it has now been superseded by the split
		// point.
		for i := range sk.Requests {
			sk.Requests[i].Tag = 0
		}
		for i := range sk.Requests {
			r := &sk.Requests[i]
			if r.H == hb.Request() {
				if r.Type == RequestDatumPoint {
					// Delete datum point.
					r.Tag = 1
					sk.FixConstraintsForRequestBeingDeleted(r.H)
				} else {
					// Add constraint if not datum point, but endpoint of
					// line/arc etc.
					sk.ConstrainCoincident(hia, hb)
				}
				break
			}
		}
		keptR := sk.Requests[:0]
		for _, r := range sk.Requests {
			if r.Tag == 0 {
				keptR = append(keptR, r)
			}
		}
		sk.Requests = keptR
	} else {
		// Split second non-point entity and add constraint.
		hib, errB := sk.SplitEntity(hb, pi)
		if err == nil {
			err = errB
		}
		if hia.V != 0 && hib.V != 0 {
			sk.ConstrainCoincident(hia, hib)
		}
	}
	return err
}

// ProjectPoint projects p to the screen in the default view, which is
// orthographic (C++ GraphicsWindow::ProjectPoint).
func ProjectPoint(p geom.Vector) geom.Point2d {
	p = p.Plus(viewOffset)
	return geom.Point2d{
		U: p.Dot(viewProjRight) * viewScale,
		V: p.Dot(viewProjUp) * viewScale,
	}
}
