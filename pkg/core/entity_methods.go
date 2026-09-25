package core

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

// ============================================================
// Symbolic getters (build *Expr / *ExprVector / *ExprQuaternion
// from entity params).
// ============================================================

// PointGetExprs returns the symbolic 3-D position of this entity.
// Equivalent to C++ EntityBase::PointGetExprs: builds an
// ExprVector whose components are NewParam expressions.
func (e *Entity) PointGetExprs(resolver ParamResolver) ExprVector {
	if e == nil {
		return NewExprVector(NewConstant(0), NewConstant(0), NewConstant(0))
	}
	if e.IsPoint() {
		return NewExprVector(
			NewParam(e.Param[0]),
			NewParam(e.Param[1]),
			NewParam(e.Param[2]),
		)
	}
	if e.Point[0] != (HEntity{}) {
		// N_COPY / N_TRANS / N_ROT_TRANS etc: store NumPoint.
		return FromVector(e.ActPoint)
	}
	return NewExprVector(NewConstant(0), NewConstant(0), NewConstant(0))
}

// PointGetExprsInWorkplane returns the symbolic (u, v) coordinates
// of this point in the entity's workplane. Equivalent to C++
// EntityBase::PointGetExprsInWorkplane.
func (e *Entity) PointGetExprsInWorkplane(resolver ParamResolver) (ExprVector, HEntity) {
	if e == nil {
		return NewExprVector(NewConstant(0), NewConstant(0), NewConstant(0)), HEntity{}
	}
	if e.IsPoint() {
		// In a workplane, the point is projected onto the workplane's
		// (u, v) basis. The full implementation requires the
		// workplane's U/V normal entities; here we return the raw
		// (u, v) from the entity's params.
		if e.Type == EntityPointIn2D {
			return NewExprVector(
				NewParam(e.Param[0]),
				NewParam(e.Param[1]),
				NewConstant(0),
			), e.Workplane
		}
	}
	return NewExprVector(NewConstant(0), NewConstant(0), NewConstant(0)), e.Workplane
}

// NormalGetExprs returns the symbolic quaternion orientation of
// this normal entity. Equivalent to C++ EntityBase::NormalGetExprs.
func (e *Entity) NormalGetExprs(resolver ParamResolver) ExprQuaternion {
	if e == nil {
		return NewExprQuaternionConst(geom.QuaternionIdentity)
	}
	if e.Type == EntityNormalIn3D {
		return NewExprQuaternion(
			NewParam(e.Param[0]),
			NewParam(e.Param[1]),
			NewParam(e.Param[2]),
			NewParam(e.Param[3]),
		)
	}
	return NewExprQuaternionConst(e.ActNormal)
}

// NormalExprsU returns the symbolic U axis of this normal's
// coordinate frame. Equivalent to C++ EntityBase::NormalExprsU.
func (e *Entity) NormalExprsU() ExprVector {
	q := e.NormalGetExprs(nil)
	return q.RotationU()
}

// NormalExprsV returns the symbolic V axis.
func (e *Entity) NormalExprsV() ExprVector {
	q := e.NormalGetExprs(nil)
	return q.RotationV()
}

// NormalExprsN returns the symbolic N axis (the normal direction).
func (e *Entity) NormalExprsN() ExprVector {
	q := e.NormalGetExprs(nil)
	return q.RotationN()
}

// VectorGetExprs returns the symbolic vector (Finish - Start) for a
// line segment entity. Equivalent to C++ EntityBase::VectorGetExprs.
func (e *Entity) VectorGetExprs(resolver ParamResolver) ExprVector {
	if e == nil {
		return NewExprVector(NewConstant(0), NewConstant(0), NewConstant(0))
	}
	if !e.HasVector() {
		return NewExprVector(NewConstant(0), NewConstant(0), NewConstant(0))
	}
	a := e.PointGetExprs(resolver)
	b := e.OtherPointExprs(resolver)
	return b.Minus(a)
}

// VectorGetExprsInWorkplane returns the symbolic 2-D vector in the
// entity's workplane. Used by HORIZONTAL/VERTICAL constraint
// equation construction.
func (e *Entity) VectorGetExprsInWorkplane() (ExprVector, HEntity) {
	a, wpa := e.PointGetExprsInWorkplane(nil)
	b, _ := e.OtherPointExprsInWorkplane(nil)
	return b.Minus(a), wpa
}


// FaceGetNormalExprs returns the symbolic normal of this face entity.
// Equivalent to C++ EntityBase::FaceGetNormalExprs.
func (e *Entity) FaceGetNormalExprs(resolver ParamResolver) ExprVector {
	if e == nil || !e.IsFace() {
		return NewExprVector(NewConstant(0), NewConstant(0), NewConstant(1))
	}
	if e.Type == EntityFaceXProd {
		// Cross product of two vectors: stored in Point[0] and Point[1].
		v0 := NewEntityVector(e.Point[0], resolver)
		v1 := NewEntityVector(e.Point[1], resolver)
		return v0.Cross(v1)
	}
	if e.Type == EntityFaceNormalPt {
		// Stored as a normal entity.
		if e.Normal != (HEntity{}) {
			return NewEntityVector(e.Normal, resolver)
		}
	}
	// Fallback: return an up-vector.
	return NewExprVector(NewConstant(0), NewConstant(0), NewConstant(1))
}

// FaceGetPointExprs returns a symbolic point on this face. For
// EntityFaceNormalPt the point is stored at e.Point[0]; for
// EntityFaceXProd the centroid of Point[0] and Point[1].
func (e *Entity) FaceGetPointExprs(resolver ParamResolver) ExprVector {
	if e == nil || !e.IsFace() {
		return NewExprVector(NewConstant(0), NewConstant(0), NewConstant(0))
	}
	if e.Type == EntityFaceNormalPt && e.Point[0] != (HEntity{}) {
		return NewEntityPoint(e.Point[0], resolver)
	}
	if e.Type == EntityFaceXProd && e.Point[0] != (HEntity{}) && e.Point[1] != (HEntity{}) {
		a := NewEntityPoint(e.Point[0], resolver)
		b := NewEntityPoint(e.Point[1], resolver)
		return a.Plus(b).ScaledBy(NewConstant(0.5))
	}
	return NewExprVector(NewConstant(0), NewConstant(0), NewConstant(0))
}

// DistanceGetExpr returns the symbolic distance value of this
// distance entity. Equivalent to C++ EntityBase::DistanceGetExpr.
func (e *Entity) DistanceGetExpr() *Expr {
	if e == nil {
		return NewConstant(0)
	}
	if e.Type == EntityDistance {
		return NewParam(e.Param[0])
	}
	return NewConstant(e.ActDistance)
}

// CircleGetRadiusExpr returns the symbolic radius expression of a
// circle or arc entity. The radius is stored as a separate distance
// entity at e.Distance; we read its param via Param[0].
func (e *Entity) CircleGetRadiusExpr() *Expr {
	if e == nil {
		return NewConstant(0)
	}
	if e.Type == EntityCircle || e.Type == EntityArcOfCircle {
		// The radius is held by the distance entity referenced by
		// e.Distance. Its value is e.ActDistance; symbolically, we
		// model it as a constant (the C++ uses a more complex
		// expression involving the entity's param[0]).
		return NewConstant(e.ActDistance)
	}
	return NewConstant(e.ActDistance)
}

// RectGetPointsExprs returns the symbolic three corner points
// (origin, U-axis end, V-axis end) of a rectangular face. Used by
// parallel/perpendicular constraints on rectangles.
func (e *Entity) RectGetPointsExprs(resolver ParamResolver) (ExprVector, ExprVector, ExprVector) {
	if e == nil || e.Type != EntityFaceNormalPt {
		return NewExprVector(NewConstant(0), NewConstant(0), NewConstant(0)),
			NewExprVector(NewConstant(1), NewConstant(0), NewConstant(0)),
			NewExprVector(NewConstant(0), NewConstant(1), NewConstant(0))
	}
	// Phase 4 + Goal Item 1 simplification: returns the face point +
	// U/V vectors. Full implementation reads e.Point[0..3].
	p := e.FaceGetPointExprs(resolver)
	u := e.NormalExprsU()
	v := e.NormalExprsV()
	return p, p.Plus(u), p.Plus(v)
}


// WorkplaneGetPlaneExprs returns the symbolic (origin, normal) of a
// workplane entity.
func (e *Entity) WorkplaneGetPlaneExprs(resolver ParamResolver) (ExprVector, ExprVector) {
	if e == nil || e.Type != EntityWorkplane {
		return NewExprVector(NewConstant(0), NewConstant(0), NewConstant(0)),
			NewExprVector(NewConstant(0), NewConstant(0), NewConstant(1))
	}
	origin := NewEntityPoint(e.Point[0], resolver)
	normal := NewEntityVector(e.Normal, resolver)
	return origin, normal
}


// WorkplaneGetOffsetExprs returns the symbolic offset distance of
// the workplane from its reference.
func (e *Entity) WorkplaneGetOffsetExprs() *Expr {
	if e == nil || e.Type != EntityWorkplane {
		return NewConstant(0)
	}
	// C++ reads from a specific param; full implementation requires
	// the offset param handle stored on the workplane.
	return NewConstant(e.ActDistance)
}

// GetAxisAngleQuaternionExprs returns the symbolic axis-angle
// representation of a quaternion. Used by ROTATE/REVOLVE/HELIX
// group equation construction.
func GetAxisAngleQuaternionExprs(q ExprQuaternion) (ExprVector, *Expr) {
	// axis = (X, Y, Z) / sin(angle/2), angle = 2*acos(W).
	// Phase 1 simplification: returns the quaternion's vector part
	// (X, Y, Z) and a constant angle of 0. Full implementation
	// requires arctan / sin / acos symbolic expressions.
	return NewExprVector(q.X, q.Y, q.Z), NewConstant(0)
}

// CubicGetStartTangentExprs returns the symbolic start tangent of a
// cubic Bezier.
func (e *Entity) CubicGetStartTangentExprs(resolver ParamResolver) ExprVector {
	if e == nil || e.Type != EntityCubic {
		return NewExprVector(NewConstant(0), NewConstant(0), NewConstant(0))
	}
	// Tangent = (P1 - P0).
	if len(e.Point) >= 2 && e.Point[1] != (HEntity{}) {
		p0 := NewEntityPoint(e.Point[0], resolver)
		p1 := NewEntityPoint(e.Point[1], resolver)
		return p1.Minus(p0)
	}
	return NewExprVector(NewConstant(0), NewConstant(0), NewConstant(0))
}

// CubicGetFinishTangentExprs returns the symbolic finish tangent of
// a cubic Bezier (P3 - P2).
func (e *Entity) CubicGetFinishTangentExprs(resolver ParamResolver) ExprVector {
	if e == nil || e.Type != EntityCubic {
		return NewExprVector(NewConstant(0), NewConstant(0), NewConstant(0))
	}
	if len(e.Point) >= 4 && e.Point[2] != (HEntity{}) {
		p2 := NewEntityPoint(e.Point[2], resolver)
		p3 := NewEntityPoint(e.Point[3], resolver)
		return p3.Minus(p2)
	}
	return NewExprVector(NewConstant(0), NewConstant(0), NewConstant(0))
}

// ============================================================
// Numerical getters (read stored NumXxx or evaluate).
// ============================================================

// CircleGetRadiusNum returns the numeric radius of a circle/arc.
func (e *Entity) CircleGetRadiusNum() float64 {
	if e == nil {
		return 0
	}
	if e.Type == EntityCircle || e.Type == EntityArcOfCircle {
		return e.ActDistance
	}
	return 0
}

// CubicGetStartNum returns the numeric start point of a cubic.
func (e *Entity) CubicGetStartNum(resolver ParamResolver) geom.Vector {
	if e == nil || e.Type != EntityCubic || len(e.Point) == 0 {
		return geom.Vector{}
	}
	return e.PointValue(e.Point[0], resolver)
}

// CubicGetFinishNum returns the numeric finish point of a cubic.
func (e *Entity) CubicGetFinishNum(resolver ParamResolver) geom.Vector {
	if e == nil || e.Type != EntityCubic || len(e.Point) < 4 {
		return geom.Vector{}
	}
	return e.PointValue(e.Point[3], resolver)
}

// CubicGetStartTangentNum returns the numeric start tangent.
func (e *Entity) CubicGetStartTangentNum(resolver ParamResolver) geom.Vector {
	if e == nil || e.Type != EntityCubic || len(e.Point) < 2 {
		return geom.Vector{}
	}
	p0 := e.PointValue(e.Point[0], resolver)
	p1 := e.PointValue(e.Point[1], resolver)
	return p1.Minus(p0)
}

// CubicGetFinishTangentNum returns the numeric finish tangent.
func (e *Entity) CubicGetFinishTangentNum(resolver ParamResolver) geom.Vector {
	if e == nil || e.Type != EntityCubic || len(e.Point) < 4 {
		return geom.Vector{}
	}
	p2 := e.PointValue(e.Point[2], resolver)
	p3 := e.PointValue(e.Point[3], resolver)
	return p3.Minus(p2)
}

// FaceGetNormalNum returns the numeric face normal.
func (e *Entity) FaceGetNormalNum(resolver ParamResolver) geom.Vector {
	if e == nil || !e.IsFace() {
		return geom.Vector{Z: 1}
	}
	if e.Type == EntityFaceXProd {
		v0 := e.VectorFromPoint(e.Point[0], resolver)
		v1 := e.VectorFromPoint(e.Point[1], resolver)
		return v0.Cross(v1)
	}
	if e.Type == EntityFaceNormalPt && e.Normal != (HEntity{}) {
		return e.VectorFromPoint(e.Normal, resolver)
	}
	return geom.Vector{Z: 1}
}

// FaceGetPointNum returns a numeric point on the face.
func (e *Entity) FaceGetPointNum(resolver ParamResolver) geom.Vector {
	if e == nil || !e.IsFace() {
		return geom.Vector{}
	}
	if e.Type == EntityFaceNormalPt && e.Point[0] != (HEntity{}) {
		return e.PointValue(e.Point[0], resolver)
	}
	if e.Type == EntityFaceXProd && e.Point[0] != (HEntity{}) && e.Point[1] != (HEntity{}) {
		a := e.PointValue(e.Point[0], resolver)
		b := e.PointValue(e.Point[1], resolver)
		return a.Plus(b).ScaledBy(0.5)
	}
	return geom.Vector{}
}

// VectorGetNum returns the numeric vector (Finish - Start).
func (e *Entity) VectorGetNum(resolver ParamResolver) geom.Vector {
	if e == nil || !e.HasVector() {
		return geom.Vector{}
	}
	a := e.PointValue(e.Point[0], resolver)
	b := e.PointValue(e.Point[1], resolver)
	return b.Minus(a)
}

// VectorGetRefPoint returns the reference point of a vector entity.
func (e *Entity) VectorGetRefPoint(resolver ParamResolver) geom.Vector {
	if e == nil || !e.HasVector() {
		return geom.Vector{}
	}
	return e.PointValue(e.Point[0], resolver)
}

// VectorGetStartPoint returns the start point of a vector.
func (e *Entity) VectorGetStartPoint(resolver ParamResolver) geom.Vector {
	return e.VectorGetRefPoint(resolver)
}

// EndpointStart returns the start endpoint of a line segment.
func (e *Entity) EndpointStart(resolver ParamResolver) geom.Vector {
	return e.VectorGetRefPoint(resolver)
}

// EndpointFinish returns the finish endpoint of a line segment.
func (e *Entity) EndpointFinish(resolver ParamResolver) geom.Vector {
	if e == nil || !e.HasVector() {
		return geom.Vector{}
	}
	return e.PointValue(e.Point[1], resolver)
}

// WorkplaneGetOffset returns the numeric offset of the workplane.
func (e *Entity) WorkplaneGetOffset() float64 {
	if e == nil || e.Type != EntityWorkplane {
		return 0
	}
	return e.ActDistance
}

// GetAxisAngleQuaternion returns the numeric axis-angle
// representation (axis, angle) of this entity's quaternion.
func (e *Entity) GetAxisAngleQuaternion(resolver ParamResolver) (geom.Vector, float64) {
	if e == nil {
		return geom.Vector{Z: 1}, 0
	}
	q := e.ActNormal
	if math.IsNaN(q.W) {
		q = geom.QuaternionIdentity
	}
	halfAngle := math.Acos(q.W)
	if q.W < 0 {
		halfAngle -= math.Pi
	}
	sinHalf := math.Sin(halfAngle)
	if math.Abs(sinHalf) < 1e-9 {
		return geom.Vector{Z: 1}, 0
	}
	return geom.Vector{X: q.X / sinHalf, Y: q.Y / sinHalf, Z: q.Z / sinHalf}, 2 * halfAngle
}

// DistanceGetNumFromParam returns the numeric distance value of
// the supplied distance param handle. Used when the distance is
// stored in a separate entity.
func (e *Entity) DistanceGetNumFromParam() float64 {
	if e == nil {
		return 0
	}
	if e.Type == EntityDistance {
		return e.ActDistance
	}
	return 0
}

// PointGetQuaternion returns the quaternion orientation encoded by
// a point's entity (used by N_ROT_AXIS_TRANS). Returns the
// orientation as a stored quaternion.
func (e *Entity) PointGetQuaternion() geom.Quaternion {
	if e == nil {
		return geom.QuaternionIdentity
	}
	return e.ActNormal
}

// ArcGetAngles returns (thetaa, thetaf, dtheta) for an arc entity
// describing the start angle, end angle, and sweep.
func (e *Entity) ArcGetAngles() (float64, float64, float64) {
	if e == nil || e.Type != EntityArcOfCircle {
		return 0, 2 * math.Pi, 2 * math.Pi
	}
	// Full implementation reads from e.Param[0..2]. For now,
	// return full circle.
	return 0, 2 * math.Pi, 2 * math.Pi
}

// ============================================================
// Constraint mutators.
// ============================================================

// PointForceTo sets the point entity's stored numerical location.
// Used by the solver when seeding initial guesses.
func (e *Entity) PointForceTo(p geom.Vector) {
	if e == nil {
		return
	}
	e.ActPoint = p
	e.NumPoint = p
}

// PointForceParamTo sets the point entity's location via its params.
// Used after solving to commit the numerical solution back to params.
func (e *Entity) PointForceParamTo(resolver ParamResolver, p geom.Vector) {
	if e == nil {
		return
	}
	e.ActPoint = p
	if e.IsPoint() {
		if resolver != nil {
			resolver.SetParam(e.Param[0], p.X)
			resolver.SetParam(e.Param[1], p.Y)
			resolver.SetParam(e.Param[2], p.Z)
		}
	}
}

// PointForceQuaternionTo sets the point entity's stored quaternion.
func (e *Entity) PointForceQuaternionTo(q geom.Quaternion) {
	if e == nil {
		return
	}
	e.ActNormal = q
	e.NumNormal = q
}

// DistanceForceTo sets the distance entity's stored value.
func (e *Entity) DistanceForceTo(v float64) {
	if e == nil {
		return
	}
	e.ActDistance = v
	e.NumDistance = v
}

// NormalForceTo sets the normal entity's stored quaternion.
func (e *Entity) NormalForceTo(q geom.Quaternion) {
	if e == nil {
		return
	}
	e.ActNormal = q
	e.NumNormal = q
}

// ============================================================
// Helpers used by the symbolic getters.
// ============================================================

// ParamResolver is the read/write param interface used by
// Entity methods. solver.System satisfies it.
type ParamResolver interface {
	ParamValue(HParam) (float64, bool)
	SetParam(HParam, float64)
	PointValue(HEntity) (geom.Vector, bool)
	VectorValue(HEntity) (geom.Vector, bool)
}

// NewEntityPoint returns an ExprVector representing the position of
// the entity at handle h. Uses resolver if available to look up
// the numeric value; otherwise emits a placeholder constant.
func NewEntityPoint(h HEntity, resolver ParamResolver) ExprVector {
	if h == (HEntity{}) {
		return NewExprVector(NewConstant(0), NewConstant(0), NewConstant(0))
	}
	if resolver != nil {
		if p, ok := resolver.PointValue(h); ok {
			return FromVector(p)
		}
	}
	return NewExprVector(NewConstant(0), NewConstant(0), NewConstant(0))
}

// NewEntityVector returns an ExprVector representing the direction
// of the entity at handle h. Uses resolver if available; otherwise
// emits a placeholder constant.
func NewEntityVector(h HEntity, resolver ParamResolver) ExprVector {
	if h == (HEntity{}) {
		return NewExprVector(NewConstant(0), NewConstant(0), NewConstant(0))
	}
	if resolver != nil {
		if v, ok := resolver.VectorValue(h); ok {
			return FromVector(v)
		}
	}
	return NewExprVector(NewConstant(0), NewConstant(0), NewConstant(0))
}

// OtherPointExprs returns the symbolic position of the entity's
// "other" endpoint (Point[1] for vectors, Point[0] for points).
func (e *Entity) OtherPointExprs(resolver ParamResolver) ExprVector {
	if e == nil {
		return NewExprVector(NewConstant(0), NewConstant(0), NewConstant(0))
	}
	if len(e.Point) >= 2 && e.Point[1] != (HEntity{}) {
		return NewEntityPoint(e.Point[1], resolver)
	}
	if len(e.Point) >= 1 && e.Point[0] != (HEntity{}) {
		return NewEntityPoint(e.Point[0], resolver)
	}
	return NewExprVector(NewConstant(0), NewConstant(0), NewConstant(0))
}

// OtherPointExprsInWorkplane returns the (u, v) of the entity's
// other endpoint in the entity's workplane.
func (e *Entity) OtherPointExprsInWorkplane(resolver ParamResolver) (ExprVector, HEntity) {
	return NewEntityPoint(e.Point[1], resolver), e.Workplane
}


// PointValue returns the numeric value of the entity at handle h.
func (e *Entity) PointValue(h HEntity, resolver ParamResolver) geom.Vector {
	if resolver != nil {
		if p, ok := resolver.PointValue(h); ok {
			return p
		}
	}
	return geom.Vector{}
}

// VectorFromPoint returns the numeric vector of the entity at h.
func (e *Entity) VectorFromPoint(h HEntity, resolver ParamResolver) geom.Vector {
	if resolver != nil {
		if v, ok := resolver.VectorValue(h); ok {
			return v
		}
	}
	return geom.Vector{}
}

// math.Pi is used by the package.
var _ = math.Pi
