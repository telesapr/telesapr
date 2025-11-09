package core

import (
	"errors"
	"math"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

// NumericalResolver provides access to dependent entities and parameter values
// required to recompute numerical state for an entity.
type NumericalResolver interface {
	// Entity retrieves the entity for a given handle.
	Entity(HEntity) (*Entity, error)
	// ParamValue returns the numeric value for a given parameter handle.
	// The boolean reports whether the parameter was found.
	ParamValue(HParam) (float64, bool)
}

var (
	errNoResolver = errors.New("numerical resolver unavailable")
)

// CalculateNumerical recomputes the active numerical state for the entity using
// the provided resolver. Missing data gracefully falls back to the previously
// stored Act* values so that legacy fixtures (which often omit explicit
// parameter wiring) continue to behave sensibly.
func (e *Entity) CalculateNumerical(res NumericalResolver, forExport bool) error {
	if e == nil {
		return errors.New("cannot calculate numerical values for nil entity")
	}
	if res == nil {
		return errNoResolver
	}

	if e.IsPoint() {
		if v, ok := e.recalcPoint(res); ok {
			e.ActPoint = v
		}
	}

	if e.IsNormal() {
		if q, ok := e.recalcNormal(res); ok {
			e.ActNormal = q.Normal()
		}
	}

	if e.IsDistance() {
		if d, ok := e.recalcDistance(res); ok {
			e.ActDistance = d
		}
	}

	if e.IsFace() {
		if p, ok := e.recalcFacePoint(res); ok {
			e.ActPoint = p
		}
		if n, ok := e.recalcFaceNormal(res); ok {
			// Store the normal direction in the quaternion's vector part
			e.ActNormal = geom.Quaternion{W: 0, X: n.X, Y: n.Y, Z: n.Z}
		}
	}

	if forExport {
		e.ActVisible = !e.ForceHidden
	} else {
		// Copied entities inside the same document are always visible.
		e.ActVisible = true
	}

	return nil
}

func (e *Entity) recalcPoint(res NumericalResolver) (geom.Vector, bool) {
	switch e.Type {
	case EntityPointIn3D:
		if v, ok := vectorFromParams(res, e.Param[0], e.Param[1], e.Param[2]); ok {
			return v, true
		}
	case EntityPointIn2D:
		uVal, uOK := paramValue(res, e.Param[0])
		vVal, vOK := paramValue(res, e.Param[1])
		if !uOK || !vOK {
			break
		}

		origin, ok := workplaneOrigin(res, e.Workplane)
		if !ok {
			break
		}
		q, ok := normalQuaternionFromHandle(res, e.Workplane)
		if !ok {
			break
		}

		u := q.RotationU()
		v := q.RotationV()
		return origin.Plus(u.ScaledBy(uVal)).Plus(v.ScaledBy(vVal)), true

	case EntityPointNCopy:
		return e.NumPoint, !isZeroVector(e.NumPoint)

	case EntityPointNTrans:
		if delta, ok := vectorFromParams(res, e.Param[0], e.Param[1], e.Param[2]); ok {
			base := fallbackVector(e.NumPoint, e.ActPoint)
			return base.Plus(delta.ScaledBy(float64(e.TimesApplied))), true
		}

	case EntityPointNRotTrans:
		offset, ok := vectorFromParams(res, e.Param[0], e.Param[1], e.Param[2])
		if !ok {
			break
		}
		q, ok := quaternionFromParams(res, e.Param[3], e.Param[4], e.Param[5], e.Param[6])
		if !ok {
			break
		}
		base := fallbackVector(e.NumPoint, e.ActPoint)
		return q.Rotate(base).Plus(offset), true

	case EntityPointNRotAA:
		offset, ok := vectorFromParams(res, e.Param[0], e.Param[1], e.Param[2])
		if !ok {
			break
		}
		q, ok := axisAngleQuaternion(res, e, 3)
		if !ok {
			break
		}
		base := fallbackVector(e.NumPoint, e.ActPoint)
		return q.Rotate(base.Minus(offset)).Plus(offset), true

	case EntityPointNRotAxisTrans:
		offset, ok := vectorFromParams(res, e.Param[0], e.Param[1], e.Param[2])
		if !ok {
			break
		}
		dir, dirOK := vectorFromParams(res, e.Param[4], e.Param[5], e.Param[6])
		mag, magOK := paramValue(res, e.Param[7])
		if !dirOK || !magOK {
			break
		}
		q, qOK := axisAngleQuaternion(res, e, 3)
		if !qOK {
			break
		}
		displace := dir.WithMagnitude(mag).ScaledBy(float64(e.TimesApplied))
		base := fallbackVector(e.NumPoint, e.ActPoint)
		return q.Rotate(base.Minus(offset)).Plus(offset).Plus(displace), true
	}

	return e.ActPoint, false
}

func (e *Entity) recalcNormal(res NumericalResolver) (geom.Quaternion, bool) {
	switch e.Type {
	case EntityNormalIn3D:
		if q, ok := quaternionFromParams(res, e.Param[0], e.Param[1], e.Param[2], e.Param[3]); ok {
			return q, true
		}
	case EntityNormalIn2D:
		q, ok := normalQuaternionFromHandle(res, e.Workplane)
		if ok {
			return q, true
		}
	case EntityNormalNCopy:
		return e.NumNormal, !isZeroQuaternion(e.NumNormal)
	case EntityNormalNRot:
		if q, ok := quaternionFromParams(res, e.Param[0], e.Param[1], e.Param[2], e.Param[3]); ok {
			return q.Times(e.NumNormal), true
		}
	case EntityNormalNRotAA:
		if q, ok := axisAngleQuaternion(res, e, 0); ok {
			return q.Times(e.NumNormal), true
		}
	}
	return e.ActNormal, false
}

func (e *Entity) recalcDistance(res NumericalResolver) (float64, bool) {
	switch e.Type {
	case EntityDistance:
		if v, ok := paramValue(res, e.Param[0]); ok {
			return v, true
		}
	case EntityDistanceNCopy:
		return e.NumDistance, e.NumDistance != 0
	}
	return e.ActDistance, false
}

func (e *Entity) recalcFacePoint(res NumericalResolver) (geom.Vector, bool) {
	switch e.Type {
	case EntityFaceNormalPt, EntityFaceRotNormalPt:
		return entityPoint(res, e.Point[0])
	case EntityFaceXProd:
		return e.NumPoint, !isZeroVector(e.NumPoint)
	case EntityFaceNRotTrans:
		offset, ok := vectorFromParams(res, e.Param[0], e.Param[1], e.Param[2])
		if !ok {
			break
		}
		q, ok := quaternionFromParams(res, e.Param[3], e.Param[4], e.Param[5], e.Param[6])
		if !ok {
			break
		}
		base := fallbackVector(e.NumPoint, e.ActPoint)
		return q.Rotate(base).Plus(offset), true
	case EntityFaceNRotAxisTrans:
		offset, ok := vectorFromParams(res, e.Param[0], e.Param[1], e.Param[2])
		if !ok {
			break
		}
		dir, dirOK := vectorFromParams(res, e.Param[4], e.Param[5], e.Param[6])
		mag, magOK := paramValue(res, e.Param[7])
		if !dirOK || !magOK {
			break
		}
		q, qOK := axisAngleQuaternion(res, e, 3)
		if !qOK {
			break
		}
		displace := dir.WithMagnitude(mag).ScaledBy(float64(e.TimesApplied))
		base := fallbackVector(e.NumPoint, e.ActPoint)
		return q.Rotate(base.Minus(offset)).Plus(offset).Plus(displace), true
	case EntityFaceNTrans:
		if delta, ok := vectorFromParams(res, e.Param[0], e.Param[1], e.Param[2]); ok {
			base := fallbackVector(e.NumPoint, e.ActPoint)
			return base.Plus(delta.ScaledBy(float64(e.TimesApplied))), true
		}
	case EntityFaceNRotAA:
		offset, ok := vectorFromParams(res, e.Param[0], e.Param[1], e.Param[2])
		if !ok {
			break
		}
		q, qOK := axisAngleQuaternion(res, e, 3)
		if !qOK {
			break
		}
		base := fallbackVector(e.NumPoint, e.ActPoint)
		return q.Rotate(base.Minus(offset)).Plus(offset), true
	}
	return e.ActPoint, false
}

func (e *Entity) recalcFaceNormal(res NumericalResolver) (geom.Vector, bool) {
	switch e.Type {
	case EntityFaceNormalPt:
		return vectorFromQuaternion(e.NumNormal), !isZeroQuaternion(e.NumNormal)
	case EntityFaceXProd:
		if cross, ok := vectorFromParams(res, e.Param[0], e.Param[1], e.Param[2]); ok {
			n := vectorFromQuaternion(e.NumNormal)
			return cross.Cross(n), true
		}
	case EntityFaceNRotTrans:
		if q, ok := quaternionFromParams(res, e.Param[3], e.Param[4], e.Param[5], e.Param[6]); ok {
			return q.Rotate(vectorFromQuaternion(e.NumNormal)), true
		}
	case EntityFaceNTrans:
		return vectorFromQuaternion(e.NumNormal), !isZeroQuaternion(e.NumNormal)
	case EntityFaceNRotAA, EntityFaceRotNormalPt:
		if q, ok := axisAngleQuaternion(res, e, 3); ok {
			return q.Rotate(vectorFromQuaternion(e.NumNormal)), true
		}
	case EntityFaceNRotAxisTrans:
		if q, ok := axisAngleQuaternion(res, e, 3); ok {
			return q.Rotate(vectorFromQuaternion(e.NumNormal)), true
		}
	}
	return vectorFromQuaternion(e.ActNormal), false
}

func paramValue(res NumericalResolver, h HParam) (float64, bool) {
	if res == nil || h.V == 0 {
		return 0, false
	}
	return res.ParamValue(h)
}

func vectorFromParams(res NumericalResolver, hx, hy, hz HParam) (geom.Vector, bool) {
	x, okx := paramValue(res, hx)
	y, oky := paramValue(res, hy)
	z, okz := paramValue(res, hz)
	if okx && oky && okz {
		return geom.NewVector(x, y, z), true
	}
	return geom.Vector{}, false
}

func quaternionFromParams(res NumericalResolver, hw, hx, hy, hz HParam) (geom.Quaternion, bool) {
	w, okw := paramValue(res, hw)
	x, okx := paramValue(res, hx)
	y, oky := paramValue(res, hy)
	z, okz := paramValue(res, hz)
	if okw && okx && oky && okz {
		return geom.NewQuaternion(w, x, y, z), true
	}
	return geom.Quaternion{}, false
}

func axisAngleQuaternion(res NumericalResolver, e *Entity, start int) (geom.Quaternion, bool) {
	if e == nil || res == nil {
		return geom.Quaternion{}, false
	}
	if start+3 >= len(e.Param) {
		return geom.Quaternion{}, false
	}
	theta, okTheta := paramValue(res, e.Param[start])
	axis, okAxis := vectorFromParams(res, e.Param[start+1], e.Param[start+2], e.Param[start+3])
	if !okTheta || !okAxis {
		return geom.Quaternion{}, false
	}
	angle := theta * float64(e.TimesApplied)
	s := math.Sin(angle)
	c := math.Cos(angle)
	axis = axis.Normal()
	return geom.NewQuaternion(c, axis.X*s, axis.Y*s, axis.Z*s), true
}

func entityPoint(res NumericalResolver, h HEntity) (geom.Vector, bool) {
	if res == nil {
		return geom.Vector{}, false
	}
	if h.V == 0 {
		return geom.Vector{}, false
	}
	ent, err := res.Entity(h)
	if err != nil || ent == nil {
		return geom.Vector{}, false
	}
	return ent.ActPoint, !isZeroVector(ent.ActPoint)
}

func workplaneOrigin(res NumericalResolver, workplane HEntity) (geom.Vector, bool) {
	if res == nil {
		return geom.Vector{}, false
	}
	wp, err := res.Entity(workplane)
	if err != nil || wp == nil {
		return geom.Vector{}, false
	}
	if len(wp.Point) == 0 || wp.Point[0].V == 0 {
		return wp.ActPoint, !isZeroVector(wp.ActPoint)
	}
	return entityPoint(res, wp.Point[0])
}

func normalQuaternionFromHandle(res NumericalResolver, workplane HEntity) (geom.Quaternion, bool) {
	if res == nil {
		return geom.Quaternion{}, false
	}
	wp, err := res.Entity(workplane)
	if err != nil || wp == nil {
		return geom.Quaternion{}, false
	}
	if wp.Normal.V == 0 {
		return wp.ActNormal, !isZeroQuaternion(wp.ActNormal)
	}
	normEnt, err := res.Entity(wp.Normal)
	if err == nil && normEnt != nil {
		return normEnt.ActNormal, !isZeroQuaternion(normEnt.ActNormal)
	}
	return wp.ActNormal, !isZeroQuaternion(wp.ActNormal)
}

func vectorFromQuaternion(q geom.Quaternion) geom.Vector {
	return geom.NewVector(q.X, q.Y, q.Z)
}

func fallbackVector(primary, fallback geom.Vector) geom.Vector {
	if !isZeroVector(primary) {
		return primary
	}
	return fallback
}

func isZeroVector(v geom.Vector) bool {
	return math.Abs(v.X) < LengthEps &&
		math.Abs(v.Y) < LengthEps &&
		math.Abs(v.Z) < LengthEps
}

func isZeroQuaternion(q geom.Quaternion) bool {
	return math.Abs(q.W) < LengthEps &&
		math.Abs(q.X) < LengthEps &&
		math.Abs(q.Y) < LengthEps &&
		math.Abs(q.Z) < LengthEps
}
