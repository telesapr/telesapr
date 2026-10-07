package core

import (
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/libm"
)

// This file ports solvespace/src/constrainteq.cpp: the equations each
// constraint imposes, written with the EntityBase accessors.

// sketchParams evaluates expressions against the sketch's current
// param values, as C++ Expr::Eval does through SK.
type sketchParams struct {
	sk EntitySketch
}

func (s sketchParams) ParamValue(h HParam) (float64, bool) {
	p := s.sk.Param(h)
	if p == nil {
		return 0, false
	}
	return p.Val, true
}

// EvalExpr evaluates expr at the sketch's current param values (C++
// Expr::Eval).
func EvalExpr(sk EntitySketch, expr *Expr) float64 {
	return expr.Eval(sketchParams{sk: sk}, nil)
}

// HasLabel reports whether the constraint is a dimension with a value
// label (C++ ConstraintBase::HasLabel).
func (c *Constraint) HasLabel() bool {
	switch c.Type {
	case ConstraintPtLineDistance, ConstraintPtPlaneDistance,
		ConstraintPtFaceDistance, ConstraintPtPtDistance,
		ConstraintProjPtDistance, ConstraintDiameter,
		ConstraintLengthRatio, ConstraintArcArcLenRatio,
		ConstraintArcLineLenRatio, ConstraintLengthDifference,
		ConstraintArcArcDifference, ConstraintArcLineDifference,
		ConstraintAngle, ConstraintComment:
		return true
	}
	return false
}

// VectorsParallel3d is zero when a is p times b (C++
// ConstraintBase::VectorsParallel3d).
func VectorsParallel3d(a, b ExprVector, p HParam) ExprVector {
	return a.Minus(b.ScaledBy(NewParam(p)))
}

// PointLineDistance is the distance from point hpt to line hln, signed
// when projected into wrkpl (C++ ConstraintBase::PointLineDistance).
func PointLineDistance(sk EntitySketch, wrkpl, hpt, hln HEntity) *Expr {
	ln := mustEntity(sk, hln)
	a := mustEntity(sk, ln.Point[0])
	b := mustEntity(sk, ln.Point[1])

	p := mustEntity(sk, hpt)

	if IsFreeIn3D(wrkpl) {
		ep := p.PointGetExprs(sk)

		ea := a.PointGetExprs(sk)
		eb := b.PointGetExprs(sk)
		eab := ea.Minus(eb)
		m := eab.Magnitude()

		return eab.Cross(ea.Minus(ep)).Magnitude().Div(m)
	}
	ua, va := a.PointGetExprsInWorkplaneUV(sk, wrkpl)
	ub, vb := b.PointGetExprsInWorkplaneUV(sk, wrkpl)

	du := ua.Minus(ub)
	dv := va.Minus(vb)

	u, v := p.PointGetExprsInWorkplaneUV(sk, wrkpl)

	m := du.Square().Plus(dv.Square()).Sqrt()

	proj := dv.Times(ua.Minus(u)).Minus(du.Times(va.Minus(v)))

	return proj.Div(m)
}

// PointPlaneDistance is the signed distance from p to workplane hpl (C++
// ConstraintBase::PointPlaneDistance).
func PointPlaneDistance(sk EntitySketch, p ExprVector, hpl HEntity) *Expr {
	n, d := mustEntity(sk, hpl).WorkplaneGetPlaneExprs(sk)
	return p.Dot(n).Minus(d)
}

// DirectionCosine is the cosine of the angle between two vectors, or
// between their projections into wrkpl (C++
// ConstraintBase::DirectionCosine).
func DirectionCosine(sk EntitySketch, wrkpl HEntity, ae, be ExprVector) *Expr {
	if IsFreeIn3D(wrkpl) {
		mags := ae.Magnitude().Times(be.Magnitude())
		return ae.Dot(be).Div(mags)
	}
	w := mustEntity(sk, wrkpl)
	u := w.NormalEntity(sk).NormalExprsU(sk)
	v := w.NormalEntity(sk).NormalExprsV(sk)
	ua := u.Dot(ae)
	va := v.Dot(ae)
	ub := u.Dot(be)
	vb := v.Dot(be)
	maga := ua.Square().Plus(va.Square()).Sqrt()
	magb := ub.Square().Plus(vb.Square()).Sqrt()
	dot := ua.Times(ub).Plus(va.Times(vb))
	return dot.Div(maga.Times(magb))
}

// PointInThreeSpace is the point at (u, v) in workplane (C++
// ConstraintBase::PointInThreeSpace).
func PointInThreeSpace(sk EntitySketch, workplane HEntity, u, v *Expr) ExprVector {
	w := mustEntity(sk, workplane)

	ub := w.NormalEntity(sk).NormalExprsU(sk)
	vb := w.NormalEntity(sk).NormalExprsV(sk)
	ob := w.WorkplaneGetOffsetExprs(sk)

	return ub.ScaledBy(u).Plus(vb.ScaledBy(v)).Plus(ob)
}

// ProjectVectorInto projects v into the plane of workplane wrkpl (C++
// Vector::ProjectVectorInto).
func ProjectVectorInto(sk EntitySketch, v geom.Vector, wrkpl HEntity) geom.Vector {
	w := mustEntity(sk, wrkpl)
	u := w.NormalEntity(sk).NormalU(sk)
	vv := w.NormalEntity(sk).NormalV(sk)

	up := v.Dot(u)
	vp := v.Dot(vv)

	return u.ScaledBy(up).Plus(vv.ScaledBy(vp))
}

// ModifyToSatisfy changes the constraint's value (or its valP) so that
// it holds for the sketch as it stands; used for reference dimensions
// (C++ ConstraintBase::ModifyToSatisfy).
func (c *Constraint) ModifyToSatisfy(sk EntitySketch) {
	switch c.Type {
	case ConstraintAngle:
		a := mustEntity(sk, c.EntityA).VectorGetNum(sk)
		b := mustEntity(sk, c.EntityB).VectorGetNum(sk)
		if c.Other {
			a = a.ScaledBy(-1)
		}
		if !IsFreeIn3D(c.Workplane) {
			a = ProjectVectorInto(sk, a, c.Workplane)
			b = ProjectVectorInto(sk, b, c.Workplane)
		}
		cs := a.Dot(b) / (a.Magnitude() * b.Magnitude())
		c.ValA = libm.Acos(cs) * 180 / math.Pi

	case ConstraintPtOnLine:
		eln := mustEntity(sk, c.EntityA)
		ea := mustEntity(sk, eln.Point[0])
		eb := mustEntity(sk, eln.Point[1])
		ep := mustEntity(sk, c.PtA)
		exp := ep.PointGetExprsInWorkplane(sk, c.Workplane)
		exa := ea.PointGetExprsInWorkplane(sk, c.Workplane)
		exb := eb.PointGetExprsInWorkplane(sk, c.Workplane)
		exba := exb.Minus(exa)
		setParamVal(sk, c.ValP, EvalExpr(sk, exba.Dot(exp.Minus(exa)))/EvalExpr(sk, exba.Dot(exba)))

	default:
		// We'll fix these ones up by looking at their symbolic equation;
		// that means no extra work.
		var l []Equation
		// Generate the equations even if this is a reference dimension
		c.GenerateEquations(sk, &l, true)
		if len(l) != 1 {
			panic("Expected constraint to generate a single equation")
		}

		// These equations are written in the form f(...) - d = 0, where
		// d is the value of the valA.
		c.ValA += EvalExpr(sk, l[0].Expr)
	}
}

func (c *Constraint) addEq(l *[]Equation, expr *Expr, index int) {
	*l = append(*l, Equation{H: c.H.Equation(index), Expr: expr})
}

// addEqVector adds v's components, the z one only in 3d (C++ AddEq with
// an ExprVector).
func (c *Constraint) addEqVector(l *[]Equation, v ExprVector, baseIndex int) {
	c.addEq(l, v.X, baseIndex)
	c.addEq(l, v.Y, baseIndex+1)
	if IsFreeIn3D(c.Workplane) {
		c.addEq(l, v.Z, baseIndex+2)
	}
}

// arcLengthExprs returns the radius and the swept angle of arc as
// expressions, choosing the inverse trig function that stays well
// conditioned for the arc's current angle. C++ writes this out inline
// in each arc-length constraint.
func arcLengthExprs(sk EntitySketch, arc *Entity) (r, theta *Expr) {
	// And get the arc radius, and the cosine of its angle
	ao := mustEntity(sk, arc.Point[0])
	as := mustEntity(sk, arc.Point[1])
	af := mustEntity(sk, arc.Point[2])

	aos := as.PointGetExprs(sk).Minus(ao.PointGetExprs(sk))
	aof := af.PointGetExprs(sk).Minus(ao.PointGetExprs(sk))
	r = aof.Magnitude()

	n := arc.NormalEntity(sk).NormalExprsN(sk)
	u := aos.WithMagnitude(NewConstant(1.0))
	v := n.Cross(u)
	// so in our new csys, we start at (1, 0, 0)
	costheta := aof.Dot(u).Div(r)
	sintheta := aof.Dot(v).Div(r)

	_, _, dtheta := arc.ArcGetAngles(sk)
	switch {
	case dtheta < 3*math.Pi/4:
		theta = costheta.ACos()

	case dtheta < 5*math.Pi/4:
		// As the angle crosses pi, cos theta is not invertible;
		// so use the sine to stop blowing up
		theta = NewConstant(math.Pi).Minus(sintheta.ASin())

	default:
		theta = NewConstant(2 * math.Pi).Minus(costheta.ACos())
	}
	return r, theta
}

// GenerateEquations appends the constraint's equations (C++
// ConstraintBase::GenerateEquations). Reference constraints generate
// none unless forReference is set.
func (c *Constraint) GenerateEquations(sk EntitySketch, l *[]Equation, forReference bool) {
	if c.Reference && !forReference {
		return
	}

	exA := NewConstant(c.ValA)
	switch c.Type {
	case ConstraintPtPtDistance:
		c.addEq(l, DistanceExpr(sk, c.Workplane, c.PtA, c.PtB).Minus(exA), 0)
		return

	case ConstraintProjPtDistance:
		pA := mustEntity(sk, c.PtA).PointGetExprs(sk)
		pB := mustEntity(sk, c.PtB).PointGetExprs(sk)
		dp := pB.Minus(pA)

		pp := mustEntity(sk, c.EntityA).VectorGetExprs(sk)
		pp = pp.WithMagnitude(NewConstant(1.0))

		c.addEq(l, dp.Dot(pp).Minus(exA), 0)
		return

	case ConstraintPtLineDistance:
		c.addEq(l, PointLineDistance(sk, c.Workplane, c.PtA, c.EntityA).Minus(exA), 0)
		return

	case ConstraintPtPlaneDistance:
		pt := mustEntity(sk, c.PtA).PointGetExprs(sk)
		c.addEq(l, PointPlaneDistance(sk, pt, c.EntityA).Minus(exA), 0)
		return

	case ConstraintPtFaceDistance:
		pt := mustEntity(sk, c.PtA).PointGetExprs(sk)
		f := mustEntity(sk, c.EntityA)
		p0 := f.FaceGetPointExprs(sk)
		n := f.FaceGetNormalExprs()
		c.addEq(l, pt.Minus(p0).Dot(n).Minus(exA), 0)
		return

	case ConstraintEqualLengthLines:
		a := mustEntity(sk, c.EntityA)
		b := mustEntity(sk, c.EntityB)
		c.addEq(l, DistanceExpr(sk, c.Workplane, a.Point[0], a.Point[1]).Minus(
			DistanceExpr(sk, c.Workplane, b.Point[0], b.Point[1])), 0)
		return

	// These work on distance squared, since the pt-line distances are
	// signed, and we want the absolute value.
	case ConstraintEqLenPtLineD:
		forLen := mustEntity(sk, c.EntityA)
		d1 := DistanceExpr(sk, c.Workplane, forLen.Point[0], forLen.Point[1])
		d2 := PointLineDistance(sk, c.Workplane, c.PtA, c.EntityB)
		c.addEq(l, d1.Square().Minus(d2.Square()), 0)
		return

	case ConstraintEqPtLnDistances:
		d1 := PointLineDistance(sk, c.Workplane, c.PtA, c.EntityA)
		d2 := PointLineDistance(sk, c.Workplane, c.PtB, c.EntityB)
		c.addEq(l, d1.Square().Minus(d2.Square()), 0)
		return

	case ConstraintLengthRatio:
		a := mustEntity(sk, c.EntityA)
		b := mustEntity(sk, c.EntityB)
		la := DistanceExpr(sk, c.Workplane, a.Point[0], a.Point[1])
		lb := DistanceExpr(sk, c.Workplane, b.Point[0], b.Point[1])
		c.addEq(l, la.Div(lb).Minus(exA), 0)
		return

	case ConstraintArcArcLenRatio:
		r1, theta1 := arcLengthExprs(sk, mustEntity(sk, c.EntityA))
		r2, theta2 := arcLengthExprs(sk, mustEntity(sk, c.EntityB))
		// And write the equation; (r1*theta1) / ( r2*theta2) = some ratio
		c.addEq(l, r1.Times(theta1).Div(r2.Times(theta2)).Minus(exA), 0)
		return

	case ConstraintArcLineLenRatio:
		line := mustEntity(sk, c.EntityA)
		ll := DistanceExpr(sk, c.Workplane, line.Point[0], line.Point[1])
		r1, theta1 := arcLengthExprs(sk, mustEntity(sk, c.EntityB))
		// And write the equation; (r1*theta1) / ( length) = some ratio
		c.addEq(l, r1.Times(theta1).Div(ll).Minus(exA), 0)
		return

	case ConstraintLengthDifference:
		a := mustEntity(sk, c.EntityA)
		b := mustEntity(sk, c.EntityB)
		la := DistanceExpr(sk, c.Workplane, a.Point[0], a.Point[1])
		lb := DistanceExpr(sk, c.Workplane, b.Point[0], b.Point[1])
		c.addEq(l, la.Minus(lb).Minus(exA), 0)
		return

	case ConstraintArcArcDifference:
		r1, theta1 := arcLengthExprs(sk, mustEntity(sk, c.EntityA))
		r2, theta2 := arcLengthExprs(sk, mustEntity(sk, c.EntityB))
		// And write the equation; (r1*theta1) - ( r2*theta2) = some difference
		c.addEq(l, r1.Times(theta1).Minus(r2.Times(theta2)).Minus(exA), 0)
		return

	case ConstraintArcLineDifference:
		line := mustEntity(sk, c.EntityA)
		ll := DistanceExpr(sk, c.Workplane, line.Point[0], line.Point[1])
		r1, theta1 := arcLengthExprs(sk, mustEntity(sk, c.EntityB))
		// And write the equation; (r1*theta1) - ( length) = some difference
		c.addEq(l, r1.Times(theta1).Minus(ll).Minus(exA), 0)
		return

	case ConstraintDiameter:
		circle := mustEntity(sk, c.EntityA)
		r := circle.CircleGetRadiusExpr(sk)
		c.addEq(l, r.Times(NewConstant(2)).Minus(exA), 0)
		return

	case ConstraintEqualRadius:
		c1 := mustEntity(sk, c.EntityA)
		c2 := mustEntity(sk, c.EntityB)
		c.addEq(l, c1.CircleGetRadiusExpr(sk).Minus(c2.CircleGetRadiusExpr(sk)), 0)
		return

	case ConstraintEqualLineArcLen:
		line := mustEntity(sk, c.EntityA)
		arc := mustEntity(sk, c.EntityB)

		// Get the line length
		l0 := mustEntity(sk, line.Point[0]).PointGetExprs(sk)
		l1 := mustEntity(sk, line.Point[1]).PointGetExprs(sk)
		ll := l1.Minus(l0).Magnitude()

		r, theta := arcLengthExprs(sk, arc)

		// And write the equation; r*theta = L
		c.addEq(l, r.Times(theta).Minus(ll), 0)
		return

	case ConstraintPointsCoincident:
		a := mustEntity(sk, c.PtA)
		b := mustEntity(sk, c.PtB)
		if IsFreeIn3D(c.Workplane) {
			pa := a.PointGetExprs(sk)
			pb := b.PointGetExprs(sk)
			c.addEq(l, pa.X.Minus(pb.X), 0)
			c.addEq(l, pa.Y.Minus(pb.Y), 1)
			c.addEq(l, pa.Z.Minus(pb.Z), 2)
		} else {
			au, av := a.PointGetExprsInWorkplaneUV(sk, c.Workplane)
			bu, bv := b.PointGetExprsInWorkplaneUV(sk, c.Workplane)
			c.addEq(l, au.Minus(bu), 0)
			c.addEq(l, av.Minus(bv), 1)
		}
		return

	case ConstraintPtInPlane:
		// This one works the same, whether projected or not.
		c.addEq(l, PointPlaneDistance(sk, mustEntity(sk, c.PtA).PointGetExprs(sk), c.EntityA), 0)
		return

	case ConstraintPtOnFace:
		// a plane, n dot (p - p0) = 0
		p := mustEntity(sk, c.PtA).PointGetExprs(sk)
		f := mustEntity(sk, c.EntityA)
		p0 := f.FaceGetPointExprs(sk)
		n := f.FaceGetNormalExprs()
		c.addEq(l, p.Minus(p0).Dot(n), 0)
		return

	case ConstraintPtOnLine:
		ln := mustEntity(sk, c.EntityA)
		a := mustEntity(sk, ln.Point[0])
		b := mustEntity(sk, ln.Point[1])
		p := mustEntity(sk, c.PtA)

		ep := p.PointGetExprsInWorkplane(sk, c.Workplane)
		ea := a.PointGetExprsInWorkplane(sk, c.Workplane)
		eb := b.PointGetExprsInWorkplane(sk, c.Workplane)

		ptOnLine := ea.Plus(eb.Minus(ea).ScaledBy(NewParam(c.ValP)))
		eq := ptOnLine.Minus(ep)

		c.addEqVector(l, eq, 0)
		return

	case ConstraintPtOnCircle:
		// This actually constrains the point to lie on the cylinder.
		circle := mustEntity(sk, c.EntityA)
		center := mustEntity(sk, circle.Point[0]).PointGetExprs(sk)
		pt := mustEntity(sk, c.PtA).PointGetExprs(sk)
		normal := mustEntity(sk, circle.Normal)
		u := normal.NormalExprsU(sk)
		v := normal.NormalExprsV(sk)

		du := center.Minus(pt).Dot(u)
		dv := center.Minus(pt).Dot(v)

		r := circle.CircleGetRadiusExpr(sk)

		c.addEq(l, du.Square().Plus(dv.Square()).Sqrt().Minus(r), 0)
		return

	case ConstraintAtMidpoint:
		if IsFreeIn3D(c.Workplane) {
			ln := mustEntity(sk, c.EntityA)
			a := mustEntity(sk, ln.Point[0]).PointGetExprs(sk)
			b := mustEntity(sk, ln.Point[1]).PointGetExprs(sk)
			m := a.Plus(b).ScaledBy(NewConstant(0.5))

			if c.PtA.V != 0 {
				p := mustEntity(sk, c.PtA).PointGetExprs(sk)
				c.addEq(l, m.X.Minus(p.X), 0)
				c.addEq(l, m.Y.Minus(p.Y), 1)
				c.addEq(l, m.Z.Minus(p.Z), 2)
			} else {
				c.addEq(l, PointPlaneDistance(sk, m, c.EntityB), 0)
			}
		} else {
			ln := mustEntity(sk, c.EntityA)
			a := mustEntity(sk, ln.Point[0])
			b := mustEntity(sk, ln.Point[1])

			au, av := a.PointGetExprsInWorkplaneUV(sk, c.Workplane)
			bu, bv := b.PointGetExprsInWorkplaneUV(sk, c.Workplane)
			mu := NewConstant(0.5).Times(au.Plus(bu))
			mv := NewConstant(0.5).Times(av.Plus(bv))

			if c.PtA.V != 0 {
				p := mustEntity(sk, c.PtA)
				pu, pv := p.PointGetExprsInWorkplaneUV(sk, c.Workplane)
				c.addEq(l, pu.Minus(mu), 0)
				c.addEq(l, pv.Minus(mv), 1)
			} else {
				m := PointInThreeSpace(sk, c.Workplane, mu, mv)
				c.addEq(l, PointPlaneDistance(sk, m, c.EntityB), 0)
			}
		}
		return

	case ConstraintSymmetric:
		if IsFreeIn3D(c.Workplane) {
			plane := mustEntity(sk, c.EntityA)
			ea := mustEntity(sk, c.PtA)
			eb := mustEntity(sk, c.PtB)
			a := ea.PointGetExprs(sk)
			b := eb.PointGetExprs(sk)

			// The midpoint of the line connecting the symmetric points
			// lies on the plane of the symmetry.
			m := a.Plus(b).ScaledBy(NewConstant(0.5))
			c.addEq(l, PointPlaneDistance(sk, m, plane.H), 0)

			// And projected into the plane of symmetry, the points are
			// coincident.
			au, av := ea.PointGetExprsInWorkplaneUV(sk, plane.H)
			bu, bv := eb.PointGetExprsInWorkplaneUV(sk, plane.H)
			c.addEq(l, au.Minus(bu), 1)
			c.addEq(l, av.Minus(bv), 2)
		} else {
			plane := mustEntity(sk, c.EntityA)
			a := mustEntity(sk, c.PtA)
			b := mustEntity(sk, c.PtB)

			au, av := a.PointGetExprsInWorkplaneUV(sk, c.Workplane)
			bu, bv := b.PointGetExprsInWorkplaneUV(sk, c.Workplane)
			mu := NewConstant(0.5).Times(au.Plus(bu))
			mv := NewConstant(0.5).Times(av.Plus(bv))

			m := PointInThreeSpace(sk, c.Workplane, mu, mv)
			c.addEq(l, PointPlaneDistance(sk, m, plane.H), 0)

			// Construct a vector within the workplane that is normal
			// to the symmetry pane's normal (i.e., that lies in the
			// plane of symmetry). The line connecting the points is
			// perpendicular to that constructed vector.
			w := mustEntity(sk, c.Workplane)
			u := w.NormalEntity(sk).NormalExprsU(sk)
			v := w.NormalEntity(sk).NormalExprsV(sk)

			pa := a.PointGetExprs(sk)
			pb := b.PointGetExprs(sk)
			n, _ := plane.WorkplaneGetPlaneExprs(sk)
			c.addEq(l, n.Cross(u.Cross(v)).Dot(pa.Minus(pb)), 1)
		}
		return

	case ConstraintSymmetricHoriz, ConstraintSymmetricVert:
		if IsFreeIn3D(c.Workplane) {
			panic("Unexpected horizontal/vertical symmetric constraint in 3d")
		}

		a := mustEntity(sk, c.PtA)
		b := mustEntity(sk, c.PtB)

		au, av := a.PointGetExprsInWorkplaneUV(sk, c.Workplane)
		bu, bv := b.PointGetExprsInWorkplaneUV(sk, c.Workplane)

		if c.Type == ConstraintSymmetricHoriz {
			c.addEq(l, av.Minus(bv), 0)
			c.addEq(l, au.Plus(bu), 1)
		} else {
			c.addEq(l, au.Minus(bu), 0)
			c.addEq(l, av.Plus(bv), 1)
		}
		return

	case ConstraintSymmetricLine:
		pa := mustEntity(sk, c.PtA)
		pb := mustEntity(sk, c.PtB)

		pau, pav := pa.PointGetExprsInWorkplaneUV(sk, c.Workplane)
		pbu, pbv := pb.PointGetExprsInWorkplaneUV(sk, c.Workplane)

		ln := mustEntity(sk, c.EntityA)
		la := mustEntity(sk, ln.Point[0])
		lb := mustEntity(sk, ln.Point[1])
		lau, lav := la.PointGetExprsInWorkplaneUV(sk, c.Workplane)
		lbu, lbv := lb.PointGetExprsInWorkplaneUV(sk, c.Workplane)

		dpu, dpv := pbu.Minus(pau), pbv.Minus(pav)
		dlu, dlv := lbu.Minus(lau), lbv.Minus(lav)

		// The line through the points is perpendicular to the line
		// of symmetry.
		c.addEq(l, dlu.Times(dpu).Plus(dlv.Times(dpv)), 0)

		// And the signed distances of the points to the line are
		// equal in magnitude and opposite in sign, so sum to zero
		dista := dlv.Times(lau.Minus(pau)).Minus(dlu.Times(lav.Minus(pav)))
		distb := dlv.Times(lau.Minus(pbu)).Minus(dlu.Times(lav.Minus(pbv)))
		c.addEq(l, dista.Plus(distb), 1)
		return

	case ConstraintHorizontal, ConstraintVertical:
		if IsFreeIn3D(c.Workplane) {
			panic("Unexpected horizontal/vertical constraint in 3d")
		}

		var ha, hb HEntity
		if c.EntityA.V != 0 {
			e := mustEntity(sk, c.EntityA)
			ha = e.Point[0]
			hb = e.Point[1]
		} else {
			ha = c.PtA
			hb = c.PtB
		}
		a := mustEntity(sk, ha)
		b := mustEntity(sk, hb)

		au, av := a.PointGetExprsInWorkplaneUV(sk, c.Workplane)
		bu, bv := b.PointGetExprsInWorkplaneUV(sk, c.Workplane)

		if c.Type == ConstraintHorizontal {
			c.addEq(l, av.Minus(bv), 0)
		} else {
			c.addEq(l, au.Minus(bu), 0)
		}
		return

	case ConstraintSameOrientation:
		a := mustEntity(sk, c.EntityA)
		b := mustEntity(sk, c.EntityB)

		au := a.NormalExprsU(sk)
		an := a.NormalExprsN(sk)
		bu := b.NormalExprsU(sk)
		bv := b.NormalExprsV(sk)
		bn := b.NormalExprsN(sk)

		eq := VectorsParallel3d(an, bn, c.ValP)
		c.addEq(l, eq.X, 0)
		c.addEq(l, eq.Y, 1)
		c.addEq(l, eq.Z, 2)
		d1 := au.Dot(bv)
		d2 := au.Dot(bu)
		// Allow either orientation for the coordinate system, depending
		// on how it was drawn.
		if math.Abs(EvalExpr(sk, d1)) < math.Abs(EvalExpr(sk, d2)) {
			c.addEq(l, d1, 3)
		} else {
			c.addEq(l, d2, 3)
		}
		return

	case ConstraintPerpendicular, ConstraintAngle:
		a := mustEntity(sk, c.EntityA)
		b := mustEntity(sk, c.EntityB)
		ae := a.VectorGetExprs(sk)
		be := b.VectorGetExprs(sk)
		if c.Other {
			ae = ae.ScaledBy(NewConstant(-1))
		}
		cs := DirectionCosine(sk, c.Workplane, ae, be)

		if c.Type == ConstraintAngle {
			// The direction cosine is equal to the cosine of the
			// specified angle
			rads := exA.Times(NewConstant(math.Pi / 180))
			rc := rads.Cos()
			arc := math.Abs(EvalExpr(sk, rc))
			// avoid false detection of inconsistent systems by gaining
			// up as the difference in dot products gets small at small
			// angles; doubles still have plenty of precision, only
			// problem is that rank test
			mult := 1.0
			if arc > 0.99 {
				mult = 0.01 / (1.00001 - arc)
			}
			c.addEq(l, cs.Minus(rc).Times(NewConstant(mult)), 0)
		} else {
			// The dot product (and therefore the direction cosine)
			// is equal to zero, perpendicular.
			c.addEq(l, cs, 0)
		}
		return

	case ConstraintEqualAngle:
		a := mustEntity(sk, c.EntityA)
		b := mustEntity(sk, c.EntityB)
		cc := mustEntity(sk, c.EntityC)
		d := mustEntity(sk, c.EntityD)
		ae := a.VectorGetExprs(sk)
		be := b.VectorGetExprs(sk)
		ce := cc.VectorGetExprs(sk)
		de := d.VectorGetExprs(sk)

		if c.Other {
			ae = ae.ScaledBy(NewConstant(-1))
		}

		cab := DirectionCosine(sk, c.Workplane, ae, be)
		ccd := DirectionCosine(sk, c.Workplane, ce, de)

		c.addEq(l, cab.Minus(ccd), 0)
		return

	case ConstraintArcLineTangent:
		arc := mustEntity(sk, c.EntityA)
		line := mustEntity(sk, c.EntityB)

		ac := mustEntity(sk, arc.Point[0]).PointGetExprs(sk)
		pt := arc.Point[1]
		if c.Other {
			pt = arc.Point[2]
		}
		ap := mustEntity(sk, pt).PointGetExprs(sk)

		ld := line.VectorGetExprs(sk)

		// The line is perpendicular to the radius
		c.addEq(l, ld.Dot(ac.Minus(ap)), 0)
		return

	case ConstraintCubicLineTangent:
		cubic := mustEntity(sk, c.EntityA)
		line := mustEntity(sk, c.EntityB)

		var a ExprVector
		if c.Other {
			a = cubic.CubicGetFinishTangentExprs(sk)
		} else {
			a = cubic.CubicGetStartTangentExprs(sk)
		}

		b := line.VectorGetExprs(sk)

		if IsFreeIn3D(c.Workplane) {
			eq := VectorsParallel3d(a, b, c.ValP)
			c.addEqVector(l, eq, 0)
		} else {
			w := mustEntity(sk, c.Workplane)
			wn := w.NormalEntity(sk).NormalExprsN(sk)
			c.addEq(l, a.Cross(b).Dot(wn), 0)
		}
		return

	case ConstraintCurveCurveTangent:
		parallel := true
		var dir [2]ExprVector
		for i := range 2 {
			he, oth := c.EntityA, c.Other
			if i == 1 {
				he, oth = c.EntityB, c.Other2
			}
			e := mustEntity(sk, he)

			switch e.Type {
			case EntityArcOfCircle:
				center := mustEntity(sk, e.Point[0]).PointGetExprs(sk)
				pt := e.Point[1]
				if oth {
					pt = e.Point[2]
				}
				endpoint := mustEntity(sk, pt).PointGetExprs(sk)
				dir[i] = endpoint.Minus(center)
				// We're using the vector from the center of the arc to
				// an endpoint; so that's normal to the tangent, not
				// parallel.
				parallel = !parallel
			case EntityCubic:
				if oth {
					dir[i] = e.CubicGetFinishTangentExprs(sk)
				} else {
					dir[i] = e.CubicGetStartTangentExprs(sk)
				}
			default:
				panic("Unexpected entity types for CURVE_CURVE_TANGENT")
			}
		}
		if parallel {
			w := mustEntity(sk, c.Workplane)
			wn := w.NormalEntity(sk).NormalExprsN(sk)
			c.addEq(l, dir[0].Cross(dir[1]).Dot(wn), 0)
		} else {
			c.addEq(l, dir[0].Dot(dir[1]), 0)
		}
		return

	case ConstraintParallel:
		ea := mustEntity(sk, c.EntityA)
		eb := mustEntity(sk, c.EntityB)
		a := ea.VectorGetExprsInWorkplane(sk, c.Workplane)
		b := eb.VectorGetExprsInWorkplane(sk, c.Workplane)

		if IsFreeIn3D(c.Workplane) {
			eq := VectorsParallel3d(a, b, c.ValP)
			c.addEqVector(l, eq, 0)
		} else {
			// We use expressions written in workplane csys, so we can
			// assume the workplane normal is (0, 0, 1); the pseudo-scalar
			// product is the z component of a x b.
			eq := a.X.Times(b.Y).Minus(a.Y.Times(b.X))
			c.addEq(l, eq, 0)
		}
		return

	case ConstraintWhereDragged:
		ep := mustEntity(sk, c.PtA)
		if IsFreeIn3D(c.Workplane) {
			ev := ep.PointGetExprs(sk)
			v := ep.PointGetNum(sk)

			c.addEq(l, ev.X.Minus(NewConstant(v.X)), 0)
			c.addEq(l, ev.Y.Minus(NewConstant(v.Y)), 1)
			c.addEq(l, ev.Z.Minus(NewConstant(v.Z)), 2)
		} else {
			u, v := ep.PointGetExprsInWorkplaneUV(sk, c.Workplane)
			c.addEq(l, u.Minus(NewConstant(EvalExpr(sk, u))), 0)
			c.addEq(l, v.Minus(NewConstant(EvalExpr(sk, v))), 1)
		}
		return

	case ConstraintComment:
		return
	}
	panic(fmt.Sprintf("Unexpected constraint type %d", c.Type))
}

// IsProjectible reports whether the constraint applies in a workplane,
// projected into it (C++ ConstraintBase::IsProjectible).
func (c *Constraint) IsProjectible() bool {
	switch c.Type {
	case ConstraintPointsCoincident, ConstraintPtPtDistance, ConstraintPtLineDistance,
		ConstraintPtOnLine, ConstraintEqualLengthLines, ConstraintEqLenPtLineD,
		ConstraintEqPtLnDistances, ConstraintEqualAngle, ConstraintLengthRatio,
		ConstraintArcArcLenRatio, ConstraintArcLineLenRatio, ConstraintLengthDifference,
		ConstraintArcArcDifference, ConstraintArcLineDifference, ConstraintSymmetric,
		ConstraintSymmetricHoriz, ConstraintSymmetricVert, ConstraintSymmetricLine,
		ConstraintAtMidpoint, ConstraintHorizontal, ConstraintVertical, ConstraintAngle,
		ConstraintParallel, ConstraintPerpendicular, ConstraintWhereDragged, ConstraintComment:
		return true

	case ConstraintPtPlaneDistance, ConstraintPtFaceDistance, ConstraintProjPtDistance,
		ConstraintPtInPlane, ConstraintPtOnFace, ConstraintEqualLineArcLen, ConstraintDiameter,
		ConstraintPtOnCircle, ConstraintSameOrientation, ConstraintCubicLineTangent,
		ConstraintCurveCurveTangent, ConstraintArcLineTangent, ConstraintEqualRadius:
		return false
	}
	panic("core: impossible constraint type")
}
