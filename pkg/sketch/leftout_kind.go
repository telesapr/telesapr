package sketch

import (
	"slices"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
)

// This file says what kind of entity a reference of a left-out item
// must be, from how the item uses it: a diameter's entity is a circle
// or an arc, a point-on-face's a face. The file that lacks the entity
// doesn't say what it was; how it was used does, and a mend takes only
// an entity of that kind in its place.

// mendRole is what kind of entity a reference of a left-out item must
// be, for a mend to put another in its place.
type mendRole int

const (
	mendPoint mendRole = iota
	mendWorkplane
	mendCurve
	mendLine
	mendCircle
	mendArc
	mendCubic
	mendFace
	mendNormal
	mendVector
	mendArcOrCubic
	mendPlane
	mendAny
)

// String names the kind for a message.
func (r mendRole) String() string {
	return [...]string{"a point", "a workplane", "a curve", "a line", "a circle or an arc",
		"an arc", "a cubic", "a face", "a normal", "a line or a normal", "an arc or a cubic",
		"a workplane or a planar face", "an entity"}[r]
}

// fits reports whether e can take the place of a reference of role r.
func (r mendRole) fits(e *core.Entity) bool {
	switch r {
	case mendPoint:
		return e.IsPoint()
	case mendWorkplane:
		return e.IsWorkplane()
	case mendLine:
		return e.Type == core.EntityLineSegment
	case mendCircle:
		return e.IsCircle()
	case mendArc:
		return e.Type == core.EntityArcOfCircle
	case mendCubic:
		return e.Type == core.EntityCubic || e.Type == core.EntityCubicPeriodic
	case mendFace:
		return e.IsFace()
	case mendNormal:
		return e.IsNormal()
	case mendVector:
		return e.HasVector()
	case mendArcOrCubic:
		return e.Type == core.EntityArcOfCircle || e.Type == core.EntityCubic
	case mendPlane:
		return e.IsWorkplane() || e.IsFace()
	case mendAny:
		return true
	default:
		switch e.Type {
		case core.EntityLineSegment, core.EntityCubic, core.EntityCubicPeriodic,
			core.EntityCircle, core.EntityArcOfCircle:
			return true
		}
		return false
	}
}

// constraintRoles is the kind each of a constraint's entities EntityA
// to EntityD must be, as its equations use them; a slot it doesn't use
// is mendAny, never missing, its handle being none.
func constraintRoles(t core.ConstraintType) [4]mendRole {
	r := [4]mendRole{mendAny, mendAny, mendAny, mendAny}
	switch t {
	case core.ConstraintPtPlaneDistance, core.ConstraintPtInPlane, core.ConstraintSymmetric:
		r[0] = mendWorkplane
	case core.ConstraintPtLineDistance, core.ConstraintPtOnLine, core.ConstraintPtOnSegment,
		core.ConstraintSymmetricLine, core.ConstraintHorizontal, core.ConstraintVertical:
		r[0] = mendLine
	case core.ConstraintPtFaceDistance, core.ConstraintPtOnFace:
		r[0] = mendFace
	case core.ConstraintProjPtDistance:
		r[0] = mendVector
	case core.ConstraintEqualLengthLines, core.ConstraintLengthRatio, core.ConstraintLengthDifference,
		core.ConstraintEqLenPtLineD, core.ConstraintEqPtLnDistances:
		r[0], r[1] = mendLine, mendLine
	case core.ConstraintEqualLineArcLen, core.ConstraintArcLineLenRatio, core.ConstraintArcLineDifference:
		r[0], r[1] = mendLine, mendArc
	case core.ConstraintArcArcLenRatio, core.ConstraintArcArcDifference:
		r[0], r[1] = mendArc, mendArc
	case core.ConstraintAtMidpoint:
		r[0], r[1] = mendLine, mendWorkplane
	case core.ConstraintDiameter, core.ConstraintPtOnCircle:
		r[0] = mendCircle
	case core.ConstraintEqualRadius:
		r[0], r[1] = mendCircle, mendCircle
	case core.ConstraintSameOrientation:
		r[0], r[1] = mendNormal, mendNormal
	case core.ConstraintAngle, core.ConstraintParallel, core.ConstraintPerpendicular:
		r[0], r[1] = mendVector, mendVector
	case core.ConstraintEqualAngle:
		r = [4]mendRole{mendVector, mendVector, mendVector, mendVector}
	case core.ConstraintAxisAngle:
		r[0] = mendNormal
	case core.ConstraintArcLineTangent:
		r[0], r[1] = mendArc, mendLine
	case core.ConstraintCubicLineTangent:
		r[0], r[1] = mendCubic, mendLine
	case core.ConstraintCurveCurveTangent:
		r[0], r[1] = mendArcOrCubic, mendArcOrCubic
	}
	return r
}

// predefRoles is the kind a group's Predef.EntityB and EntityC must
// be, as the group's generation uses them: a lathe's, a revolve's, a
// helix's or a rotation's axis is a line or a normal, an extrusion's
// or a translation's plane a workplane, a mirror's a workplane or a
// planar face, and a workplane made of two lines takes two lines or
// normals, one through a point and a normal a normal. What a group's
// kind doesn't use is mendAny.
func (g *Group) predefRoles() (b, c mendRole) {
	switch g.Type {
	case GroupDrawingWorkplane:
		switch g.Subtype {
		case GroupWorkplaneByLineSegments:
			return mendVector, mendVector
		case GroupWorkplaneByPointNormal:
			return mendNormal, mendAny
		}
	case GroupLathe, GroupRevolve, GroupHelix, GroupRotate:
		return mendVector, mendAny
	case GroupExtrude, GroupTranslate:
		return mendWorkplane, mendAny
	case GroupMirror:
		return mendPlane, mendAny
	}
	return mendAny, mendAny
}

// LeftOutMissingKind says what kind of entity he, one LeftOutMissing
// gives, must be replaced by, from how what was left out uses it: "a
// point", "a circle or an arc", "a face". An entity used in several
// roles names each.
func (sk *Sketch) LeftOutMissingKind(he core.HEntity) string {
	if sk.LeftOut == nil {
		return ""
	}
	var kinds []string
	for _, r := range sk.LeftOut.mendRefs() {
		if *r.at == he && !slices.Contains(kinds, r.role.String()) {
			kinds = append(kinds, r.role.String())
		}
	}
	return strings.Join(kinds, " and ")
}
