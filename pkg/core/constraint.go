package core

import "github.com/spacesolve/spacesolve/pkg/geom"

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

	ValA    float64
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
}

func (c *ConstraintBase) Equals(other *ConstraintBase) bool {
	return c.Type == other.Type &&
		c.Group == other.Group &&
		c.Workplane == other.Workplane &&
		c.ValA == other.ValA &&
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
