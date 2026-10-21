package core

import (
	"fmt"

	"github.com/telesapr/telesapr/pkg/geom"
)

type ConstraintType int

const (
	ConstraintUnknown          ConstraintType = 0
	ConstraintPointsCoincident ConstraintType = 20
	ConstraintPtPtDistance     ConstraintType = 30
	ConstraintPtPlaneDistance  ConstraintType = 31
	ConstraintPtLineDistance   ConstraintType = 32
	ConstraintPtFaceDistance   ConstraintType = 33
	ConstraintProjPtDistance   ConstraintType = 34
	ConstraintPtInPlane        ConstraintType = 41
	ConstraintPtOnLine         ConstraintType = 42
	ConstraintPtOnFace         ConstraintType = 43
	ConstraintPtOnSegment      ConstraintType = 44

	ConstraintEqualLengthLines ConstraintType = 50
	ConstraintLengthRatio      ConstraintType = 51
	ConstraintEqLenPtLineD     ConstraintType = 52
	ConstraintEqPtLnDistances  ConstraintType = 53
	ConstraintEqualAngle       ConstraintType = 54
	ConstraintEqualLineArcLen  ConstraintType = 55
	ConstraintLengthDifference ConstraintType = 56

	ConstraintSymmetric      ConstraintType = 60
	ConstraintSymmetricHoriz ConstraintType = 61
	ConstraintSymmetricVert  ConstraintType = 62
	ConstraintSymmetricLine  ConstraintType = 63
	ConstraintAtMidpoint     ConstraintType = 70

	ConstraintHorizontal ConstraintType = 80
	ConstraintVertical   ConstraintType = 81

	ConstraintDiameter   ConstraintType = 90
	ConstraintPtOnCircle ConstraintType = 100

	ConstraintSameOrientation   ConstraintType = 110
	ConstraintAngle             ConstraintType = 120
	ConstraintParallel          ConstraintType = 121
	ConstraintPerpendicular     ConstraintType = 122
	ConstraintArcLineTangent    ConstraintType = 123
	ConstraintCubicLineTangent  ConstraintType = 124
	ConstraintCurveCurveTangent ConstraintType = 125
	ConstraintEqualRadius       ConstraintType = 130
	// ConstraintAxisAngle puts point PtB in the half-plane through the
	// axis at PtA along normal EntityA turned ValA degrees from the
	// normal's u, about its n: a part's turn about an axis, signed and
	// of any size (constraint_axis_angle.go).
	ConstraintAxisAngle         ConstraintType = 131
	ConstraintWhereDragged      ConstraintType = 200
	ConstraintArcArcLenRatio    ConstraintType = 210
	ConstraintArcLineLenRatio   ConstraintType = 211
	ConstraintArcArcDifference  ConstraintType = 212
	ConstraintArcLineDifference ConstraintType = 213
	ConstraintComment           ConstraintType = 1000
)

func (t ConstraintType) String() string {
	switch t {
	case ConstraintPointsCoincident:
		return "PointsCoincident"
	case ConstraintPtPtDistance:
		return "PtPtDistance"
	case ConstraintPtPlaneDistance:
		return "PtPlaneDistance"
	case ConstraintPtLineDistance:
		return "PtLineDistance"
	case ConstraintPtFaceDistance:
		return "PtFaceDistance"
	case ConstraintProjPtDistance:
		return "ProjPtDistance"
	case ConstraintPtInPlane:
		return "PtInPlane"
	case ConstraintPtOnLine:
		return "PtOnLine"
	case ConstraintPtOnFace:
		return "PtOnFace"
	case ConstraintPtOnSegment:
		return "PtOnSegment"
	case ConstraintEqualLengthLines:
		return "EqualLengthLines"
	case ConstraintLengthRatio:
		return "LengthRatio"
	case ConstraintEqLenPtLineD:
		return "EqLenPtLineD"
	case ConstraintEqPtLnDistances:
		return "EqPtLnDistances"
	case ConstraintEqualAngle:
		return "EqualAngle"
	case ConstraintEqualLineArcLen:
		return "EqualLineArcLen"
	case ConstraintLengthDifference:
		return "LengthDifference"
	case ConstraintSymmetric:
		return "Symmetric"
	case ConstraintSymmetricHoriz:
		return "SymmetricHoriz"
	case ConstraintSymmetricVert:
		return "SymmetricVert"
	case ConstraintSymmetricLine:
		return "SymmetricLine"
	case ConstraintAtMidpoint:
		return "AtMidpoint"
	case ConstraintHorizontal:
		return "Horizontal"
	case ConstraintVertical:
		return "Vertical"
	case ConstraintDiameter:
		return "Diameter"
	case ConstraintPtOnCircle:
		return "PtOnCircle"
	case ConstraintSameOrientation:
		return "SameOrientation"
	case ConstraintAngle:
		return "Angle"
	case ConstraintParallel:
		return "Parallel"
	case ConstraintPerpendicular:
		return "Perpendicular"
	case ConstraintArcLineTangent:
		return "ArcLineTangent"
	case ConstraintCubicLineTangent:
		return "CubicLineTangent"
	case ConstraintCurveCurveTangent:
		return "CurveCurveTangent"
	case ConstraintEqualRadius:
		return "EqualRadius"
	case ConstraintAxisAngle:
		return "AxisAngle"
	case ConstraintWhereDragged:
		return "WhereDragged"
	case ConstraintArcArcLenRatio:
		return "ArcArcLenRatio"
	case ConstraintArcLineLenRatio:
		return "ArcLineLenRatio"
	case ConstraintArcArcDifference:
		return "ArcArcDifference"
	case ConstraintArcLineDifference:
		return "ArcLineDifference"
	case ConstraintComment:
		return "Comment"
	default:
		return "Unknown"
	}
}

type ConstraintBase struct {
	Tag int
	H   HConstraint

	Type      ConstraintType
	Group     HGroup
	Workplane HEntity

	ValA float64
	// Expr is the expression ValA is evaluated from at each
	// regeneration, in millimetres and degrees, or "" for a plain
	// number (see Sketch.EvaluateParameters).
	Expr string
	// Name, when not "", is what expressions read the constraint's
	// value by, as they read a parameter (Sketch.SetConstraintName).
	Name    string
	ValP    HParam
	PtA     HEntity
	PtB     HEntity
	EntityA HEntity
	EntityB HEntity
	EntityC HEntity
	EntityD HEntity
	Other   bool
	Other2  bool

	Reference bool
	Comment   string

	// Limit makes a dimension a bound, its value the least (LimitMin)
	// or the most (LimitMax) the dimension may be, instead of what it
	// is; LimitNone for a dimension.
	Limit Limit

	// Bound is the bound an inequality is held at in the solve, or
	// BoundFree: worked out afresh at each solve, and neither saved
	// nor compared.
	Bound int
}

func (c *ConstraintBase) Equals(other *ConstraintBase) bool {
	return c.Type == other.Type &&
		c.Group == other.Group &&
		c.Workplane == other.Workplane &&
		c.ValA == other.ValA &&
		c.Expr == other.Expr &&
		c.Name == other.Name &&
		c.ValP == other.ValP &&
		c.PtA == other.PtA &&
		c.PtB == other.PtB &&
		c.EntityA == other.EntityA &&
		c.EntityB == other.EntityB &&
		c.EntityC == other.EntityC &&
		c.EntityD == other.EntityD &&
		c.Other == other.Other &&
		c.Other2 == other.Other2 &&
		c.Reference == other.Reference &&
		c.Limit == other.Limit &&
		c.Comment == other.Comment
}

func (c *ConstraintBase) Clear() {}

type Constraint struct {
	ConstraintBase

	Disp struct {
		Offset geom.Vector
		Style  HStyle
	}
}

type ConstraintList []Constraint

// constraintNames are the constraint types' names, as descriptions show
// them.
var constraintNames = map[ConstraintType]string{
	ConstraintPointsCoincident:  "pts-coincident",
	ConstraintPtPtDistance:      "pt-pt-distance",
	ConstraintPtLineDistance:    "pt-line-distance",
	ConstraintPtPlaneDistance:   "pt-plane-distance",
	ConstraintPtFaceDistance:    "pt-face-distance",
	ConstraintProjPtDistance:    "proj-pt-pt-distance",
	ConstraintPtInPlane:         "pt-in-plane",
	ConstraintPtOnLine:          "pt-on-line",
	ConstraintPtOnFace:          "pt-on-face",
	ConstraintPtOnSegment:       "pt-on-segment",
	ConstraintEqualLengthLines:  "eq-length",
	ConstraintEqLenPtLineD:      "eq-length-and-pt-ln-dist",
	ConstraintEqPtLnDistances:   "eq-pt-line-distances",
	ConstraintLengthRatio:       "length-ratio",
	ConstraintArcArcLenRatio:    "arc-arc-length-ratio",
	ConstraintArcLineLenRatio:   "arc-line-length-ratio",
	ConstraintLengthDifference:  "length-difference",
	ConstraintArcArcDifference:  "arc-arc-len-difference",
	ConstraintArcLineDifference: "arc-line-len-difference",
	ConstraintAxisAngle:         "axis-angle",
	ConstraintSymmetric:         "symmetric",
	ConstraintSymmetricHoriz:    "symmetric-h",
	ConstraintSymmetricVert:     "symmetric-v",
	ConstraintSymmetricLine:     "symmetric-line",
	ConstraintAtMidpoint:        "at-midpoint",
	ConstraintHorizontal:        "horizontal",
	ConstraintVertical:          "vertical",
	ConstraintDiameter:          "diameter",
	ConstraintPtOnCircle:        "pt-on-circle",
	ConstraintSameOrientation:   "same-orientation",
	ConstraintAngle:             "angle",
	ConstraintParallel:          "parallel",
	ConstraintArcLineTangent:    "arc-line-tangent",
	ConstraintCubicLineTangent:  "cubic-line-tangent",
	ConstraintCurveCurveTangent: "curve-curve-tangent",
	ConstraintPerpendicular:     "perpendicular",
	ConstraintEqualRadius:       "eq-radius",
	ConstraintEqualAngle:        "eq-angle",
	ConstraintEqualLineArcLen:   "eq-line-len-arc-len",
	ConstraintWhereDragged:      "lock-where-dragged",
	ConstraintComment:           "comment",
}

// DescriptionString names the constraint, e.g. "c002-horizontal". A
// diameter shown as a radius is named "radius", so that its name and
// the value it shows agree.
func (c *ConstraintBase) DescriptionString() string {
	s, ok := constraintNames[c.Type]
	if !ok {
		s = "???"
	}
	if c.Type == ConstraintDiameter && c.Other {
		s = "radius"
	}
	switch c.Limit {
	case LimitMin:
		s = "min-" + s
	case LimitMax:
		s = "max-" + s
	}
	return fmt.Sprintf("c%03x-%s", c.H.V, s)
}

// IsStylable reports whether the constraint can be given a style of its
// own: only a comment can, a dimension is always drawn in the
// constraint style. The window and the scripts both ask this.
func (c *Constraint) IsStylable() bool { return c.Type == ConstraintComment }

// StyleOrDefault is the style the constraint is drawn in: its own, or
// the constraint style.
func (c *Constraint) StyleOrDefault() HStyle {
	if c.Disp.Style.V != 0 {
		return c.Disp.Style
	}
	return HStyle{V: StyleConstraint}
}
