package core

import (
	"fmt"
	"math"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

// This file ports solvespace/src/entity.cpp: the symbolic and numeric
// accessors of EntityBase. The C++ reaches the sketch through the
// global SK; here it is passed in as an EntitySketch.

// EntitySketch is the part of the sketch that EntityBase methods read,
// standing in for the C++ global SK.
type EntitySketch interface {
	GetEntity(h HEntity) *Entity
	GetParam(h HParam) *Param
}

// EntityVisibility is implemented by sketches that can answer the C++
// Entity::IsVisible, which depends on UI state. CalculateNumerical
// uses it when forExport is set.
type EntityVisibility interface {
	EntityIsVisible(e *Entity) bool
}

// IsFreeIn3D reports whether wrkpl means "free in 3d". The C++
// FREE_IN_3D is handle 0; the Go loader also uses FreeIn3D.
func IsFreeIn3D(wrkpl HEntity) bool {
	return wrkpl.V == 0 || wrkpl == FreeIn3D
}

func mustEntity(sk EntitySketch, h HEntity) *Entity {
	e := sk.GetEntity(h)
	if e == nil {
		panic(fmt.Sprintf("entity %08x not found", h.V))
	}
	return e
}

func paramVal(sk EntitySketch, h HParam) float64 {
	p := sk.GetParam(h)
	if p == nil {
		panic(fmt.Sprintf("param %08x not found", h.V))
	}
	return p.Val
}

func setParamVal(sk EntitySketch, h HParam, v float64) {
	p := sk.GetParam(h)
	if p == nil {
		panic(fmt.Sprintf("param %08x not found", h.V))
	}
	p.Val = v
}

func vectorFromParams(sk EntitySketch, x, y, z HParam) geom.Vector {
	return geom.Vector{X: paramVal(sk, x), Y: paramVal(sk, y), Z: paramVal(sk, z)}
}

func quaternionFromParams(sk EntitySketch, w, x, y, z HParam) geom.Quaternion {
	return geom.Quaternion{W: paramVal(sk, w), X: paramVal(sk, x), Y: paramVal(sk, y), Z: paramVal(sk, z)}
}

func exprVectorFromParams(x, y, z HParam) ExprVector {
	return ExprVector{X: NewParam(x), Y: NewParam(y), Z: NewParam(z)}
}

func exprQuaternionFromParams(w, x, y, z HParam) ExprQuaternion {
	return ExprQuaternion{W: NewParam(w), X: NewParam(x), Y: NewParam(y), Z: NewParam(z)}
}

func numNormalVector(q geom.Quaternion) geom.Vector {
	return geom.Vector{X: q.X, Y: q.Y, Z: q.Z}
}

func unexpectedType(e *Entity) string {
	return fmt.Sprintf("unexpected entity type %d (entity %08x)", e.Type, e.H.V)
}

// NormalEntity returns the entity's normal (C++ EntityBase::Normal).
func (e *Entity) NormalEntity(sk EntitySketch) *Entity {
	return mustEntity(sk, e.Normal)
}

func (e *Entity) VectorGetExprsInWorkplane(sk EntitySketch, wrkpl HEntity) ExprVector {
	if e.IsFace() {
		return e.FaceGetNormalExprs()
	}
	switch e.Type {
	case EntityLineSegment:
		return mustEntity(sk, e.Point[0]).PointGetExprsInWorkplane(sk, wrkpl).Minus(
			mustEntity(sk, e.Point[1]).PointGetExprsInWorkplane(sk, wrkpl))

	case EntityNormalIn3D, EntityNormalIn2D, EntityNormalNCopy, EntityNormalNRot, EntityNormalNRotAA:
		ev := e.NormalExprsN(sk)
		if IsFreeIn3D(wrkpl) {
			return ev
		}
		// Get the offset and basis vectors for this weird exotic csys.
		w := mustEntity(sk, wrkpl)
		wu := w.NormalEntity(sk).NormalExprsU(sk)
		wv := w.NormalEntity(sk).NormalExprsV(sk)

		// Get our coordinates in three-space, and project them into that
		// coordinate system.
		return ExprVector{X: ev.Dot(wu), Y: ev.Dot(wv), Z: NewConstant(0)}
	}
	panic(unexpectedType(e))
}

func (e *Entity) VectorGetExprs(sk EntitySketch) ExprVector {
	return e.VectorGetExprsInWorkplane(sk, NoEntity)
}

func (e *Entity) VectorGetNum(sk EntitySketch) geom.Vector {
	if e.IsFace() {
		return e.FaceGetNormalNum(sk)
	}
	switch e.Type {
	case EntityLineSegment:
		return mustEntity(sk, e.Point[0]).PointGetNum(sk).Minus(
			mustEntity(sk, e.Point[1]).PointGetNum(sk))

	case EntityNormalIn3D, EntityNormalIn2D, EntityNormalNCopy, EntityNormalNRot, EntityNormalNRotAA:
		return e.NormalN(sk)
	}
	panic(unexpectedType(e))
}

func (e *Entity) VectorGetRefPoint(sk EntitySketch) geom.Vector {
	if e.IsFace() {
		return e.FaceGetPointNum(sk)
	}
	switch e.Type {
	case EntityLineSegment:
		return mustEntity(sk, e.Point[0]).PointGetNum(sk).Plus(
			mustEntity(sk, e.Point[1]).PointGetNum(sk)).ScaledBy(0.5)

	case EntityNormalIn3D, EntityNormalIn2D, EntityNormalNCopy, EntityNormalNRot, EntityNormalNRotAA:
		return mustEntity(sk, e.Point[0]).PointGetNum(sk)
	}
	panic(unexpectedType(e))
}

func (e *Entity) VectorGetStartPoint(sk EntitySketch) geom.Vector {
	switch e.Type {
	case EntityLineSegment:
		return mustEntity(sk, e.Point[1]).PointGetNum(sk)

	case EntityNormalIn3D, EntityNormalIn2D, EntityNormalNCopy, EntityNormalNRot, EntityNormalNRotAA:
		return mustEntity(sk, e.Point[0]).PointGetNum(sk)
	}
	panic(unexpectedType(e))
}

func (e *Entity) CircleGetRadiusExpr(sk EntitySketch) *Expr {
	switch e.Type {
	case EntityCircle:
		return mustEntity(sk, e.Distance).DistanceGetExpr()
	case EntityArcOfCircle:
		return DistanceExpr(sk, e.Workplane, e.Point[0], e.Point[1])
	}
	panic(unexpectedType(e))
}

func (e *Entity) CircleGetRadiusNum(sk EntitySketch) float64 {
	switch e.Type {
	case EntityCircle:
		return mustEntity(sk, e.Distance).DistanceGetNum(sk)
	case EntityArcOfCircle:
		c := mustEntity(sk, e.Point[0]).PointGetNum(sk)
		pa := mustEntity(sk, e.Point[1]).PointGetNum(sk)
		return pa.Minus(c).Magnitude()
	}
	panic(unexpectedType(e))
}

// ArcGetAngles returns the start angle, finish angle and swept angle
// of an arc, in its normal's (u, v) basis.
func (e *Entity) ArcGetAngles(sk EntitySketch) (thetaa, thetab, dtheta float64) {
	if e.Type != EntityArcOfCircle {
		panic(unexpectedType(e))
	}
	q := e.NormalEntity(sk).NormalGetNum(sk)
	u, v := q.RotationU(), q.RotationV()

	c := mustEntity(sk, e.Point[0]).PointGetNum(sk)
	pa := mustEntity(sk, e.Point[1]).PointGetNum(sk)
	pb := mustEntity(sk, e.Point[2]).PointGetNum(sk)

	c2 := c.Project2d(u, v)
	pa2 := pa.Project2d(u, v).Minus(c2)
	pb2 := pb.Project2d(u, v).Minus(c2)

	thetaa = math.Atan2(pa2.V, pa2.U)
	thetab = math.Atan2(pb2.V, pb2.U)
	dtheta = thetab - thetaa
	// If the endpoints are coincident, call it a full arc, not a zero arc;
	// useful concept to have when splitting
	for dtheta < 1e-6 {
		dtheta += 2 * math.Pi
	}
	for dtheta > 2*math.Pi {
		dtheta -= 2 * math.Pi
	}
	return thetaa, thetab, dtheta
}

func (e *Entity) CubicGetStartNum(sk EntitySketch) geom.Vector {
	return mustEntity(sk, e.Point[0]).PointGetNum(sk)
}

func (e *Entity) CubicGetFinishNum(sk EntitySketch) geom.Vector {
	return mustEntity(sk, e.Point[3+e.ExtraPoints]).PointGetNum(sk)
}

func (e *Entity) CubicGetStartTangentExprs(sk EntitySketch) ExprVector {
	pon := mustEntity(sk, e.Point[0]).PointGetExprs(sk)
	poff := mustEntity(sk, e.Point[1]).PointGetExprs(sk)
	return pon.Minus(poff)
}

func (e *Entity) CubicGetFinishTangentExprs(sk EntitySketch) ExprVector {
	pon := mustEntity(sk, e.Point[3+e.ExtraPoints]).PointGetExprs(sk)
	poff := mustEntity(sk, e.Point[2+e.ExtraPoints]).PointGetExprs(sk)
	return pon.Minus(poff)
}

func (e *Entity) CubicGetStartTangentNum(sk EntitySketch) geom.Vector {
	pon := mustEntity(sk, e.Point[0]).PointGetNum(sk)
	poff := mustEntity(sk, e.Point[1]).PointGetNum(sk)
	return pon.Minus(poff)
}

func (e *Entity) CubicGetFinishTangentNum(sk EntitySketch) geom.Vector {
	pon := mustEntity(sk, e.Point[3+e.ExtraPoints]).PointGetNum(sk)
	poff := mustEntity(sk, e.Point[2+e.ExtraPoints]).PointGetNum(sk)
	return pon.Minus(poff)
}

func (e *Entity) WorkplaneGetOffsetExprs(sk EntitySketch) ExprVector {
	return mustEntity(sk, e.Point[0]).PointGetExprs(sk)
}

func (e *Entity) WorkplaneGetOffset(sk EntitySketch) geom.Vector {
	return mustEntity(sk, e.Point[0]).PointGetNum(sk)
}

// WorkplaneGetPlaneExprs returns the plane n dot p = dn.
func (e *Entity) WorkplaneGetPlaneExprs(sk EntitySketch) (n ExprVector, dn *Expr) {
	if e.Type != EntityWorkplane {
		panic(unexpectedType(e))
	}
	n = e.NormalEntity(sk).NormalExprsN(sk)

	p0 := mustEntity(sk, e.Point[0]).PointGetExprs(sk)
	// The plane is n dot (p - p0) = 0, or
	//              n dot p - n dot p0 = 0
	// so dn = n dot p0
	dn = p0.Dot(n)
	return n, dn
}

func (e *Entity) DistanceGetNum(sk EntitySketch) float64 {
	switch e.Type {
	case EntityDistance:
		return paramVal(sk, e.Param[0])
	case EntityDistanceNCopy:
		return e.NumDistance
	}
	panic(unexpectedType(e))
}

func (e *Entity) DistanceGetExpr() *Expr {
	switch e.Type {
	case EntityDistance:
		return NewParam(e.Param[0])
	case EntityDistanceNCopy:
		return NewConstant(e.NumDistance)
	}
	panic(unexpectedType(e))
}

func (e *Entity) DistanceForceTo(sk EntitySketch, v float64) {
	switch e.Type {
	case EntityDistance:
		setParamVal(sk, e.Param[0], v)
	case EntityDistanceNCopy:
		// do nothing, it's locked
	default:
		panic(unexpectedType(e))
	}
}

func (e *Entity) NormalGetNum(sk EntitySketch) geom.Quaternion {
	var q geom.Quaternion
	switch e.Type {
	case EntityNormalIn3D:
		q = quaternionFromParams(sk, e.Param[0], e.Param[1], e.Param[2], e.Param[3])

	case EntityNormalIn2D:
		wrkpl := mustEntity(sk, e.Workplane)
		norm := mustEntity(sk, wrkpl.Normal)
		q = norm.NormalGetNum(sk)

	case EntityNormalNCopy:
		q = e.NumNormal

	case EntityNormalNRot:
		q = quaternionFromParams(sk, e.Param[0], e.Param[1], e.Param[2], e.Param[3])
		q = q.Times(e.NumNormal)

	case EntityNormalNRotAA:
		q = e.GetAxisAngleQuaternion(sk, 0)
		q = q.Times(e.NumNormal)

	default:
		panic(unexpectedType(e))
	}
	return q
}

func (e *Entity) NormalForceTo(sk EntitySketch, q geom.Quaternion) {
	switch e.Type {
	case EntityNormalIn3D:
		setParamVal(sk, e.Param[0], q.W)
		setParamVal(sk, e.Param[1], q.X)
		setParamVal(sk, e.Param[2], q.Y)
		setParamVal(sk, e.Param[3], q.Z)

	case EntityNormalIn2D, EntityNormalNCopy:
		// There's absolutely nothing to do; these are locked.

	case EntityNormalNRot:
		qp := q.Times(e.NumNormal.Inverse())
		setParamVal(sk, e.Param[0], qp.W)
		setParamVal(sk, e.Param[1], qp.X)
		setParamVal(sk, e.Param[2], qp.Y)
		setParamVal(sk, e.Param[3], qp.Z)

	case EntityNormalNRotAA:
		// Not sure if I'll bother implementing this one

	default:
		panic(unexpectedType(e))
	}
}

func (e *Entity) NormalU(sk EntitySketch) geom.Vector { return e.NormalGetNum(sk).RotationU() }
func (e *Entity) NormalV(sk EntitySketch) geom.Vector { return e.NormalGetNum(sk).RotationV() }
func (e *Entity) NormalN(sk EntitySketch) geom.Vector { return e.NormalGetNum(sk).RotationN() }

func (e *Entity) NormalExprsU(sk EntitySketch) ExprVector { return e.NormalGetExprs(sk).RotationU() }
func (e *Entity) NormalExprsV(sk EntitySketch) ExprVector { return e.NormalGetExprs(sk).RotationV() }
func (e *Entity) NormalExprsN(sk EntitySketch) ExprVector { return e.NormalGetExprs(sk).RotationN() }

func (e *Entity) NormalGetExprs(sk EntitySketch) ExprQuaternion {
	var q ExprQuaternion
	switch e.Type {
	case EntityNormalIn3D:
		q = exprQuaternionFromParams(e.Param[0], e.Param[1], e.Param[2], e.Param[3])

	case EntityNormalIn2D:
		wrkpl := mustEntity(sk, e.Workplane)
		norm := mustEntity(sk, wrkpl.Normal)
		q = norm.NormalGetExprs(sk)

	case EntityNormalNCopy:
		q = NewExprQuaternionConst(e.NumNormal)

	case EntityNormalNRot:
		orig := NewExprQuaternionConst(e.NumNormal)
		q = exprQuaternionFromParams(e.Param[0], e.Param[1], e.Param[2], e.Param[3])
		q = q.Times(orig)

	case EntityNormalNRotAA:
		orig := NewExprQuaternionConst(e.NumNormal)
		q = e.GetAxisAngleQuaternionExprs(0)
		q = q.Times(orig)

	default:
		panic(unexpectedType(e))
	}
	return q
}

func (e *Entity) PointForceParamTo(sk EntitySketch, p geom.Vector) {
	switch e.Type {
	case EntityPointIn3D:
		setParamVal(sk, e.Param[0], p.X)
		setParamVal(sk, e.Param[1], p.Y)
		setParamVal(sk, e.Param[2], p.Z)

	case EntityPointIn2D:
		setParamVal(sk, e.Param[0], p.X)
		setParamVal(sk, e.Param[1], p.Y)

	default:
		panic(unexpectedType(e))
	}
}

func (e *Entity) PointForceTo(sk EntitySketch, p geom.Vector) {
	switch e.Type {
	case EntityPointIn3D:
		setParamVal(sk, e.Param[0], p.X)
		setParamVal(sk, e.Param[1], p.Y)
		setParamVal(sk, e.Param[2], p.Z)

	case EntityPointIn2D:
		c := mustEntity(sk, e.Workplane)
		p = p.Minus(c.WorkplaneGetOffset(sk))
		setParamVal(sk, e.Param[0], p.Dot(c.NormalEntity(sk).NormalU(sk)))
		setParamVal(sk, e.Param[1], p.Dot(c.NormalEntity(sk).NormalV(sk)))

	case EntityPointNTrans:
		if e.TimesApplied == 0 {
			break
		}
		trans := p.Minus(e.NumPoint).ScaledBy(1.0 / float64(e.TimesApplied))
		setParamVal(sk, e.Param[0], trans.X)
		setParamVal(sk, e.Param[1], trans.Y)
		setParamVal(sk, e.Param[2], trans.Z)

	case EntityPointNRotTrans:
		// Force only the translation; leave the rotation unchanged. But
		// remember that we're working with respect to the rotated
		// point.
		trans := p.Minus(e.PointGetQuaternion(sk).Rotate(e.NumPoint))
		setParamVal(sk, e.Param[0], trans.X)
		setParamVal(sk, e.Param[1], trans.Y)
		setParamVal(sk, e.Param[2], trans.Z)

	case EntityPointNRotAA:
		// Force only the angle; the axis and center of rotation stay
		offset := vectorFromParams(sk, e.Param[0], e.Param[1], e.Param[2])
		normal := vectorFromParams(sk, e.Param[4], e.Param[5], e.Param[6])
		u, v := normal.NormalDir(0), normal.NormalDir(1)
		po, numo := p.Minus(offset), e.NumPoint.Minus(offset)
		thetap := math.Atan2(v.Dot(po), u.Dot(po))
		thetan := math.Atan2(v.Dot(numo), u.Dot(numo))
		thetaf := thetap - thetan
		thetai := paramVal(sk, e.Param[3]) * float64(e.TimesApplied) * 2
		dtheta := thetaf - thetai
		// Take the smallest possible change in the actual step angle,
		// in order to avoid jumps when you cross from +pi to -pi
		for dtheta < -math.Pi {
			dtheta += 2 * math.Pi
		}
		for dtheta > math.Pi {
			dtheta -= 2 * math.Pi
		}
		// this extra *2 explains the mystery *4
		setParamVal(sk, e.Param[3], (thetai+dtheta)/(float64(e.TimesApplied)*2))

	case EntityPointNRotAxisTrans:
		if e.TimesApplied == 0 {
			break
		}
		// is the point on the rotation axis?
		offset := vectorFromParams(sk, e.Param[0], e.Param[1], e.Param[2])
		normal := vectorFromParams(sk, e.Param[4], e.Param[5], e.Param[6]).WithMagnitude(1.0)
		check := e.NumPoint.Minus(offset).Cross(normal)
		if check.Dot(check) < LengthEps { // if so, do extrusion style drag
			trans := p.Minus(e.NumPoint)
			setParamVal(sk, e.Param[7], trans.Dot(normal)/float64(e.TimesApplied))
		} else { // otherwise do rotation style
			u, v := normal.NormalDir(0), normal.NormalDir(1)
			po, numo := p.Minus(offset), e.NumPoint.Minus(offset)
			thetap := math.Atan2(v.Dot(po), u.Dot(po))
			thetan := math.Atan2(v.Dot(numo), u.Dot(numo))
			thetaf := thetap - thetan
			thetai := paramVal(sk, e.Param[3]) * float64(e.TimesApplied) * 2
			dtheta := thetaf - thetai
			// Take the smallest possible change in the actual step angle,
			// in order to avoid jumps when you cross from +pi to -pi
			for dtheta < -math.Pi {
				dtheta += 2 * math.Pi
			}
			for dtheta > math.Pi {
				dtheta -= 2 * math.Pi
			}
			// this extra *2 explains the mystery *4
			setParamVal(sk, e.Param[3], (thetai+dtheta)/(float64(e.TimesApplied)*2))
		}

	case EntityPointNCopy:
		// Nothing to do; it's a static copy

	default:
		panic(unexpectedType(e))
	}
}

func (e *Entity) PointGetNum(sk EntitySketch) geom.Vector {
	var p geom.Vector
	switch e.Type {
	case EntityPointIn3D:
		p = vectorFromParams(sk, e.Param[0], e.Param[1], e.Param[2])

	case EntityPointIn2D:
		c := mustEntity(sk, e.Workplane)
		u := c.NormalEntity(sk).NormalU(sk)
		v := c.NormalEntity(sk).NormalV(sk)
		p = u.ScaledBy(paramVal(sk, e.Param[0]))
		p = p.Plus(v.ScaledBy(paramVal(sk, e.Param[1])))
		p = p.Plus(c.WorkplaneGetOffset(sk))

	case EntityPointNTrans:
		trans := vectorFromParams(sk, e.Param[0], e.Param[1], e.Param[2])
		p = e.NumPoint.Plus(trans.ScaledBy(float64(e.TimesApplied)))

	case EntityPointNRotTrans:
		offset := vectorFromParams(sk, e.Param[0], e.Param[1], e.Param[2])
		q := e.PointGetQuaternion(sk)
		p = q.Rotate(e.NumPoint)
		p = p.Plus(offset)

	case EntityPointNRotAA:
		offset := vectorFromParams(sk, e.Param[0], e.Param[1], e.Param[2])
		q := e.PointGetQuaternion(sk)
		p = e.NumPoint.Minus(offset)
		p = q.Rotate(p)
		p = p.Plus(offset)

	case EntityPointNRotAxisTrans:
		offset := vectorFromParams(sk, e.Param[0], e.Param[1], e.Param[2])
		displace := vectorFromParams(sk, e.Param[4], e.Param[5], e.Param[6]).
			WithMagnitude(paramVal(sk, e.Param[7])).ScaledBy(float64(e.TimesApplied))
		q := e.PointGetQuaternion(sk)
		p = e.NumPoint.Minus(offset)
		p = q.Rotate(p)
		p = p.Plus(offset).Plus(displace)

	case EntityPointNCopy:
		p = e.NumPoint

	default:
		panic(unexpectedType(e))
	}
	return p
}

func (e *Entity) PointGetExprs(sk EntitySketch) ExprVector {
	var r ExprVector
	switch e.Type {
	case EntityPointIn3D:
		r = exprVectorFromParams(e.Param[0], e.Param[1], e.Param[2])

	case EntityPointIn2D:
		c := mustEntity(sk, e.Workplane)
		u := c.NormalEntity(sk).NormalExprsU(sk)
		v := c.NormalEntity(sk).NormalExprsV(sk)
		r = c.WorkplaneGetOffsetExprs(sk)
		r = r.Plus(u.ScaledBy(NewParam(e.Param[0])))
		r = r.Plus(v.ScaledBy(NewParam(e.Param[1])))

	case EntityPointNTrans:
		orig := FromVector(e.NumPoint)
		trans := exprVectorFromParams(e.Param[0], e.Param[1], e.Param[2])
		r = orig.Plus(trans.ScaledBy(NewConstant(float64(e.TimesApplied))))

	case EntityPointNRotTrans:
		orig := FromVector(e.NumPoint)
		trans := exprVectorFromParams(e.Param[0], e.Param[1], e.Param[2])
		q := exprQuaternionFromParams(e.Param[3], e.Param[4], e.Param[5], e.Param[6])
		orig = q.Rotate(orig)
		r = orig.Plus(trans)

	case EntityPointNRotAA:
		orig := FromVector(e.NumPoint)
		trans := exprVectorFromParams(e.Param[0], e.Param[1], e.Param[2])
		q := e.GetAxisAngleQuaternionExprs(3)
		orig = orig.Minus(trans)
		orig = q.Rotate(orig)
		r = orig.Plus(trans)

	case EntityPointNRotAxisTrans:
		orig := FromVector(e.NumPoint)
		trans := exprVectorFromParams(e.Param[0], e.Param[1], e.Param[2])
		displace := exprVectorFromParams(e.Param[4], e.Param[5], e.Param[6]).
			WithMagnitude(NewConstant(1.0)).ScaledBy(NewConstant(float64(e.TimesApplied))).ScaledBy(NewParam(e.Param[7]))

		q := e.GetAxisAngleQuaternionExprs(3)
		orig = orig.Minus(trans)
		orig = q.Rotate(orig)
		r = orig.Plus(trans).Plus(displace)

	case EntityPointNCopy:
		r = FromVector(e.NumPoint)

	default:
		panic(unexpectedType(e))
	}
	return r
}

// PointGetExprsInWorkplaneUV returns this point's (u, v) coordinates
// in wrkpl (C++ PointGetExprsInWorkplane(hEntity, Expr **, Expr **)).
func (e *Entity) PointGetExprsInWorkplaneUV(sk EntitySketch, wrkpl HEntity) (u, v *Expr) {
	if e.Type == EntityPointIn2D && e.Workplane == wrkpl {
		// They want our coordinates in the form that we've written them,
		// very nice.
		return NewParam(e.Param[0]), NewParam(e.Param[1])
	}
	// Get the offset and basis vectors for this weird exotic csys.
	w := mustEntity(sk, wrkpl)
	wp := w.WorkplaneGetOffsetExprs(sk)
	wu := w.NormalEntity(sk).NormalExprsU(sk)
	wv := w.NormalEntity(sk).NormalExprsV(sk)

	// Get our coordinates in three-space, and project them into that
	// coordinate system.
	ev := e.PointGetExprs(sk)
	ev = ev.Minus(wp)
	return ev.Dot(wu), ev.Dot(wv)
}

func (e *Entity) PointGetExprsInWorkplane(sk EntitySketch, wrkpl HEntity) ExprVector {
	if IsFreeIn3D(wrkpl) {
		return e.PointGetExprs(sk)
	}
	var r ExprVector
	r.X, r.Y = e.PointGetExprsInWorkplaneUV(sk, wrkpl)
	r.Z = NewConstant(0.0)
	return r
}

func (e *Entity) PointForceQuaternionTo(sk EntitySketch, q geom.Quaternion) {
	if e.Type != EntityPointNRotTrans {
		panic(unexpectedType(e))
	}
	setParamVal(sk, e.Param[3], q.W)
	setParamVal(sk, e.Param[4], q.X)
	setParamVal(sk, e.Param[5], q.Y)
	setParamVal(sk, e.Param[6], q.Z)
}

func (e *Entity) GetAxisAngleQuaternion(sk EntitySketch, param0 int) geom.Quaternion {
	theta := float64(e.TimesApplied) * paramVal(sk, e.Param[param0+0])
	s, c := math.Sin(theta), math.Cos(theta)
	return geom.Quaternion{
		W: c,
		X: s * paramVal(sk, e.Param[param0+1]),
		Y: s * paramVal(sk, e.Param[param0+2]),
		Z: s * paramVal(sk, e.Param[param0+3]),
	}
}

func (e *Entity) GetAxisAngleQuaternionExprs(param0 int) ExprQuaternion {
	theta := NewConstant(float64(e.TimesApplied)).Times(NewParam(e.Param[param0+0]))
	c, s := theta.Cos(), theta.Sin()
	return ExprQuaternion{
		W: c,
		X: s.Times(NewParam(e.Param[param0+1])),
		Y: s.Times(NewParam(e.Param[param0+2])),
		Z: s.Times(NewParam(e.Param[param0+3])),
	}
}

func (e *Entity) PointGetQuaternion(sk EntitySketch) geom.Quaternion {
	switch e.Type {
	case EntityPointNRotAA, EntityPointNRotAxisTrans:
		return e.GetAxisAngleQuaternion(sk, 3)
	case EntityPointNRotTrans:
		return quaternionFromParams(sk, e.Param[3], e.Param[4], e.Param[5], e.Param[6])
	}
	panic(unexpectedType(e))
}

func (e *Entity) FaceGetNormalExprs() ExprVector {
	var r ExprVector
	switch e.Type {
	case EntityFaceNormalPt:
		v := numNormalVector(e.NumNormal)
		r = FromVector(v.WithMagnitude(1))

	case EntityFaceXProd:
		vc := exprVectorFromParams(e.Param[0], e.Param[1], e.Param[2])
		vn := FromVector(numNormalVector(e.NumNormal))
		r = vc.Cross(vn)
		r = r.WithMagnitude(NewConstant(1.0))

	case EntityFaceNRotTrans:
		// The numerical normal vector gets the rotation; the numerical
		// normal has magnitude one, and the rotation doesn't change that,
		// so there's no need to fix it up.
		r = FromVector(numNormalVector(e.NumNormal))
		q := exprQuaternionFromParams(e.Param[3], e.Param[4], e.Param[5], e.Param[6])
		r = q.Rotate(r)

	case EntityFaceNTrans:
		r = FromVector(numNormalVector(e.NumNormal))

	case EntityFaceNRotAA, EntityFaceRotNormalPt:
		r = FromVector(numNormalVector(e.NumNormal))
		q := e.GetAxisAngleQuaternionExprs(3)
		r = q.Rotate(r)

	default:
		panic(unexpectedType(e))
	}
	return r
}

func (e *Entity) FaceGetNormalNum(sk EntitySketch) geom.Vector {
	var r geom.Vector
	switch e.Type {
	case EntityFaceNormalPt:
		r = numNormalVector(e.NumNormal)

	case EntityFaceXProd:
		vc := vectorFromParams(sk, e.Param[0], e.Param[1], e.Param[2])
		vn := numNormalVector(e.NumNormal)
		r = vc.Cross(vn)

	case EntityFaceNRotTrans:
		// The numerical normal vector gets the rotation
		r = numNormalVector(e.NumNormal)
		q := quaternionFromParams(sk, e.Param[3], e.Param[4], e.Param[5], e.Param[6])
		r = q.Rotate(r)

	case EntityFaceNTrans:
		r = numNormalVector(e.NumNormal)

	case EntityFaceNRotAA, EntityFaceRotNormalPt:
		r = numNormalVector(e.NumNormal)
		q := e.GetAxisAngleQuaternion(sk, 3)
		r = q.Rotate(r)

	default:
		panic(unexpectedType(e))
	}
	return r.WithMagnitude(1)
}

func (e *Entity) FaceGetPointExprs(sk EntitySketch) ExprVector {
	var r ExprVector
	switch e.Type {
	case EntityFaceNormalPt, EntityFaceRotNormalPt:
		r = mustEntity(sk, e.Point[0]).PointGetExprs(sk)

	case EntityFaceXProd:
		r = FromVector(e.NumPoint)

	case EntityFaceNRotTrans:
		// The numerical point gets the rotation and translation.
		trans := exprVectorFromParams(e.Param[0], e.Param[1], e.Param[2])
		q := exprQuaternionFromParams(e.Param[3], e.Param[4], e.Param[5], e.Param[6])
		r = FromVector(e.NumPoint)
		r = q.Rotate(r)
		r = r.Plus(trans)

	case EntityFaceNRotAxisTrans:
		orig := FromVector(e.NumPoint)
		trans := exprVectorFromParams(e.Param[0], e.Param[1], e.Param[2])
		displace := exprVectorFromParams(e.Param[4], e.Param[5], e.Param[6]).
			WithMagnitude(NewParam(e.Param[7])).ScaledBy(NewConstant(float64(e.TimesApplied)))
		q := e.GetAxisAngleQuaternionExprs(3)
		orig = orig.Minus(trans)
		orig = q.Rotate(orig)
		r = orig.Plus(trans).Plus(displace)

	case EntityFaceNTrans:
		trans := exprVectorFromParams(e.Param[0], e.Param[1], e.Param[2])
		r = FromVector(e.NumPoint)
		r = r.Plus(trans.ScaledBy(NewConstant(float64(e.TimesApplied))))

	case EntityFaceNRotAA:
		trans := exprVectorFromParams(e.Param[0], e.Param[1], e.Param[2])
		q := e.GetAxisAngleQuaternionExprs(3)
		r = FromVector(e.NumPoint)
		r = r.Minus(trans)
		r = q.Rotate(r)
		r = r.Plus(trans)

	default:
		panic(unexpectedType(e))
	}
	return r
}

func (e *Entity) FaceGetPointNum(sk EntitySketch) geom.Vector {
	var r geom.Vector
	switch e.Type {
	case EntityFaceNormalPt, EntityFaceRotNormalPt:
		r = mustEntity(sk, e.Point[0]).PointGetNum(sk)

	case EntityFaceXProd:
		r = e.NumPoint

	case EntityFaceNRotTrans:
		// The numerical point gets the rotation and translation.
		trans := vectorFromParams(sk, e.Param[0], e.Param[1], e.Param[2])
		q := quaternionFromParams(sk, e.Param[3], e.Param[4], e.Param[5], e.Param[6])
		r = q.Rotate(e.NumPoint)
		r = r.Plus(trans)

	case EntityFaceNRotAxisTrans:
		offset := vectorFromParams(sk, e.Param[0], e.Param[1], e.Param[2])
		displace := vectorFromParams(sk, e.Param[4], e.Param[5], e.Param[6]).
			WithMagnitude(paramVal(sk, e.Param[7])).ScaledBy(float64(e.TimesApplied))
		// C++ calls PointGetQuaternion here, which asserts for face
		// types; the axis-angle quaternion is what it would compute.
		q := e.GetAxisAngleQuaternion(sk, 3)
		r = e.NumPoint.Minus(offset)
		r = q.Rotate(r)
		r = r.Plus(offset).Plus(displace)

	case EntityFaceNTrans:
		trans := vectorFromParams(sk, e.Param[0], e.Param[1], e.Param[2])
		r = e.NumPoint.Plus(trans.ScaledBy(float64(e.TimesApplied)))

	case EntityFaceNRotAA:
		trans := vectorFromParams(sk, e.Param[0], e.Param[1], e.Param[2])
		q := e.GetAxisAngleQuaternion(sk, 3)
		r = e.NumPoint.Minus(trans)
		r = q.Rotate(r)
		r = r.Plus(trans)

	default:
		panic(unexpectedType(e))
	}
	return r
}

func (e *Entity) EndpointStart(sk EntitySketch) geom.Vector {
	switch e.Type {
	case EntityLineSegment:
		return mustEntity(sk, e.Point[0]).PointGetNum(sk)
	case EntityCubic:
		return e.CubicGetStartNum(sk)
	case EntityArcOfCircle:
		return mustEntity(sk, e.Point[1]).PointGetNum(sk)
	}
	panic(unexpectedType(e))
}

func (e *Entity) EndpointFinish(sk EntitySketch) geom.Vector {
	switch e.Type {
	case EntityLineSegment:
		return mustEntity(sk, e.Point[1]).PointGetNum(sk)
	case EntityCubic:
		return e.CubicGetFinishNum(sk)
	case EntityArcOfCircle:
		return mustEntity(sk, e.Point[2]).PointGetNum(sk)
	}
	panic(unexpectedType(e))
}

func pointInPlane(sk EntitySketch, h HEntity, norm geom.Vector, distance float64) bool {
	p := mustEntity(sk, h).PointGetNum(sk)
	return math.Abs(norm.Dot(p)-distance) < LengthEps
}

func (e *Entity) IsInPlane(sk EntitySketch, norm geom.Vector, distance float64) bool {
	switch e.Type {
	case EntityLineSegment:
		return pointInPlane(sk, e.Point[0], norm, distance) &&
			pointInPlane(sk, e.Point[1], norm, distance)

	case EntityCubic, EntityCubicPeriodic:
		periodic := e.Type == EntityCubicPeriodic
		n := e.ExtraPoints
		if periodic {
			n = 3 + e.ExtraPoints
		}
		for i := 0; i < n; i++ {
			if !pointInPlane(sk, e.Point[i], norm, distance) {
				return false
			}
		}
		return true

	case EntityCircle, EntityArcOfCircle:
		// If it is an (arc of) a circle, check whether the normals
		// are parallel and the mid point is in the plane.
		n := e.NormalEntity(sk).NormalN(sk)
		if !norm.Equals(n) && !norm.Equals(n.Negated()) {
			return false
		}
		return pointInPlane(sk, e.Point[0], norm, distance)

	case EntityTTFText:
		n := e.NormalEntity(sk).NormalN(sk)
		if !norm.Equals(n) && !norm.Equals(n.Negated()) {
			return false
		}
		return pointInPlane(sk, e.Point[0], norm, distance) &&
			pointInPlane(sk, e.Point[1], norm, distance)
	}
	return false
}

func (e *Entity) RectGetPointsExprs(sk EntitySketch) (eb, ec ExprVector) {
	if e.Type != EntityTTFText && e.Type != EntityImage {
		panic(unexpectedType(e))
	}
	a := mustEntity(sk, e.Point[0])
	o := mustEntity(sk, e.Point[1])

	// Write equations for each point in the current workplane.
	// This reduces the complexity of resulting equations.
	ea := a.PointGetExprsInWorkplane(sk, e.Workplane)
	eo := o.PointGetExprsInWorkplane(sk, e.Workplane)

	// Take perpendicular vector and scale it by aspect ratio.
	eu := ea.Minus(eo)
	ev := ExprVector{X: eu.Y, Y: eu.X.Negate(), Z: eu.Z}.ScaledBy(NewConstant(e.AspectRatio))

	eb = eo.Plus(ev)
	ec = eo.Plus(eu).Plus(ev)
	return eb, ec
}

// Equation is one equation generated by an entity or a group, with
// the handle C++ AddEq gives it.
type Equation struct {
	H    HEquation
	Expr *Expr
}

// GenerateEquations appends the equations this entity imposes (C++
// EntityBase::GenerateEquations). constraints is the sketch's
// constraint list, which the arc case searches.
func (e *Entity) GenerateEquations(sk EntitySketch, constraints []Constraint, l *[]Equation) {
	addEq := func(expr *Expr, index int) {
		*l = append(*l, Equation{H: e.H.Equation(index), Expr: expr})
	}
	switch e.Type {
	case EntityNormalIn3D:
		q := e.NormalGetExprs(sk)
		addEq(q.Magnitude().Minus(NewConstant(1)), 0)

	case EntityArcOfCircle:
		// If this is a copied entity, with its point already fixed
		// with respect to each other, then we don't want to generate
		// the distance constraint!
		if mustEntity(sk, e.Point[0]).Type != EntityPointIn2D {
			break
		}

		// If the two endpoints of the arc are constrained coincident
		// (to make a complete circle), then our distance constraint
		// would be redundant and therefore overconstrain things.
		for i := range constraints {
			con := &constraints[i]
			if con.Group == e.Group &&
				con.Type == ConstraintPointsCoincident &&
				((con.PtA == e.Point[1] && con.PtB == e.Point[2]) ||
					(con.PtA == e.Point[2] && con.PtB == e.Point[1])) {
				return
			}
		}

		ra := DistanceExpr(sk, e.Workplane, e.Point[0], e.Point[1])
		rb := DistanceExpr(sk, e.Workplane, e.Point[0], e.Point[2])
		addEq(ra.Minus(rb), 0)

	case EntityImage, EntityTTFText:
		if mustEntity(sk, e.Point[0]).Type != EntityPointIn2D {
			break
		}
		b := mustEntity(sk, e.Point[2])
		c := mustEntity(sk, e.Point[3])
		eb := b.PointGetExprsInWorkplane(sk, e.Workplane)
		ec := c.PointGetExprsInWorkplane(sk, e.Workplane)

		ebp, ecp := e.RectGetPointsExprs(sk)

		beq := eb.Minus(ebp)
		addEq(beq.X, 0)
		addEq(beq.Y, 1)
		ceq := ec.Minus(ecp)
		addEq(ceq.X, 2)
		addEq(ceq.Y, 3)
	}
}

// CalculateNumerical caches the entity's numeric state in its Act*
// fields (C++ Entity::CalculateNumerical, drawentity.cpp).
func (e *Entity) CalculateNumerical(sk EntitySketch, forExport bool) {
	if e.IsPoint() {
		e.ActPoint = e.PointGetNum(sk)
	}
	if e.IsNormal() {
		e.ActNormal = e.NormalGetNum(sk)
	}
	if e.Type == EntityDistance || e.Type == EntityDistanceNCopy {
		e.ActDistance = e.DistanceGetNum(sk)
	}
	if e.IsFace() {
		e.ActPoint = e.FaceGetPointNum(sk)
		n := e.FaceGetNormalNum(sk)
		e.ActNormal = geom.Quaternion{W: 0, X: n.X, Y: n.Y, Z: n.Z}
	}
	if forExport {
		// Visibility in copied linked entities follows source file
		if vis, ok := sk.(EntityVisibility); ok {
			e.ActVisible = vis.EntityIsVisible(e)
		} else {
			e.ActVisible = !e.ForceHidden
		}
	} else {
		// Copied entities within a file are always visible
		e.ActVisible = true
	}
}

// DistanceExpr is the true distance between two points, or the
// distance projected into wrkpl (C++ ConstraintBase::Distance).
func DistanceExpr(sk EntitySketch, wrkpl, hpa, hpb HEntity) *Expr {
	pa := mustEntity(sk, hpa)
	pb := mustEntity(sk, hpb)
	if !pa.IsPoint() || !pb.IsPoint() {
		panic("Expected two points to measure projected distance between")
	}

	if IsFreeIn3D(wrkpl) {
		// This is true distance
		ea := pa.PointGetExprs(sk)
		eb := pb.PointGetExprs(sk)
		eab := ea.Minus(eb)
		return eab.Magnitude()
	}
	// This is projected distance, in the given workplane.
	au, av := pa.PointGetExprsInWorkplaneUV(sk, wrkpl)
	bu, bv := pb.PointGetExprsInWorkplaneUV(sk, wrkpl)

	du := au.Minus(bu)
	dv := av.Minus(bv)

	return du.Square().Plus(dv.Square()).Sqrt()
}
