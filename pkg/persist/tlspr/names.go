package tlspr

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/thread"
)

// The names the format gives enumerations. They belong to the format:
// they never change when the interface renames something.

// enum maps values of one enumeration to names and back.
type enum[T comparable] struct {
	what   string
	names  map[T]string
	values map[string]T
}

func newEnum[T comparable](what string, names map[T]string) enum[T] {
	e := enum[T]{what: what, names: names, values: make(map[string]T, len(names))}
	for v, n := range names {
		if _, dup := e.values[n]; dup {
			panic("tlspr: duplicate " + what + " name " + n)
		}
		e.values[n] = v
	}
	return e
}

var groupTypes = newEnum("group type", map[sketch.GroupType]string{
	sketch.GroupDrawing3D:        "sketch-3d",
	sketch.GroupDrawingWorkplane: "sketch-in-plane",
	sketch.GroupExtrude:          "extrude",
	sketch.GroupLathe:            "lathe",
	sketch.GroupRevolve:          "revolve",
	sketch.GroupHelix:            "helix",
	sketch.GroupRotate:           "rotate",
	sketch.GroupTranslate:        "translate",
	sketch.GroupLinked:           "link",
	sketch.GroupMirror:           "mirror",
	sketch.GroupChamfer:          "chamfer",
	sketch.GroupSweep:            "sweep",
	sketch.GroupPathPattern:      "pattern-path",
	sketch.GroupFillet:           "fillet",
	sketch.GroupShell:            "shell",
	sketch.GroupLoft:             "loft",
})

var groupSubtypes = newEnum("group subtype", map[sketch.GroupSubtype]string{
	sketch.GroupWorkplaneByPointOrtho:   "point-ortho",
	sketch.GroupWorkplaneByLineSegments: "line-segments",
	sketch.GroupWorkplaneByPointNormal:  "point-normal",
	sketch.GroupWorkplaneByFace:         "face",
	sketch.GroupOneSided:                "one-sided",
	sketch.GroupTwoSided:                "two-sided",
	sketch.GroupOneSkewed:               "one-skewed",
	sketch.GroupTwoSkewed:               "two-skewed",
})

var combines = newEnum("combine", map[sketch.CombineAs]string{
	sketch.CombineUnion:        "union",
	sketch.CombineDifference:   "difference",
	sketch.CombineAssemble:     "assemble",
	sketch.CombineIntersection: "intersection",
})

var threadForms = newEnum("thread form", map[thread.Form]string{
	thread.FormMetric:      "metric",
	thread.FormUnified:     "unified",
	thread.FormTrapezoidal: "trapezoidal",
})

var threadCrests = newEnum("thread crest", map[thread.Crest]string{
	thread.CrestStandard:  "standard",
	thread.CrestTruncated: "truncated",
	thread.CrestRounded:   "rounded",
})

var requestTypes = newEnum("request type", map[sketch.RequestType]string{
	sketch.RequestWorkplane:     "workplane",
	sketch.RequestDatumPoint:    "datum-point",
	sketch.RequestLineSegment:   "line-segment",
	sketch.RequestCubic:         "cubic",
	sketch.RequestCubicPeriodic: "cubic-periodic",
	sketch.RequestCircle:        "circle",
	sketch.RequestArcOfCircle:   "arc-of-circle",
	sketch.RequestTTFText:       "ttf-text",
	sketch.RequestImage:         "image",
})

var entityTypes = newEnum("entity type", map[core.EntityType]string{
	core.EntityPointIn3D:          "point-in-3d",
	core.EntityPointIn2D:          "point-in-2d",
	core.EntityPointNTrans:        "point-n-trans",
	core.EntityPointNRotTrans:     "point-n-rot-trans",
	core.EntityPointNCopy:         "point-n-copy",
	core.EntityPointNRotAA:        "point-n-rot-aa",
	core.EntityPointNRotAxisTrans: "point-n-rot-axis-trans",
	core.EntityNormalIn3D:         "normal-in-3d",
	core.EntityNormalIn2D:         "normal-in-2d",
	core.EntityNormalNCopy:        "normal-n-copy",
	core.EntityNormalNRot:         "normal-n-rot",
	core.EntityNormalNRotAA:       "normal-n-rot-aa",
	core.EntityDistance:           "distance",
	core.EntityDistanceNCopy:      "distance-n-copy",
	core.EntityFaceNormalPt:       "face-normal-pt",
	core.EntityFaceXProd:          "face-xprod",
	core.EntityFaceNRotTrans:      "face-n-rot-trans",
	core.EntityFaceNTrans:         "face-n-trans",
	core.EntityFaceNRotAA:         "face-n-rot-aa",
	core.EntityFaceRotNormalPt:    "face-rot-normal-pt",
	core.EntityFaceNRotAxisTrans:  "face-n-rot-axis-trans",
	core.EntityWorkplane:          "workplane",
	core.EntityLineSegment:        "line-segment",
	core.EntityCubic:              "cubic",
	core.EntityCubicPeriodic:      "cubic-periodic",
	core.EntityCircle:             "circle",
	core.EntityArcOfCircle:        "arc-of-circle",
	core.EntityTTFText:            "ttf-text",
	core.EntityImage:              "image",
	core.EntitySurfaceExtruded:    "surface-extruded",
	core.EntitySurfaceLathed:      "surface-lathed",
	core.EntitySurfaceRevolved:    "surface-revolved",
})

var constraintTypes = newEnum("constraint type", map[core.ConstraintType]string{
	core.ConstraintPointsCoincident:  "pts-coincident",
	core.ConstraintPtPtDistance:      "pt-pt-distance",
	core.ConstraintPtPlaneDistance:   "pt-plane-distance",
	core.ConstraintPtLineDistance:    "pt-line-distance",
	core.ConstraintPtFaceDistance:    "pt-face-distance",
	core.ConstraintProjPtDistance:    "proj-pt-pt-distance",
	core.ConstraintPtInPlane:         "pt-in-plane",
	core.ConstraintPtOnLine:          "pt-on-line",
	core.ConstraintPtOnFace:          "pt-on-face",
	core.ConstraintPtOnSegment:       "pt-on-segment",
	core.ConstraintEqualLengthLines:  "eq-length",
	core.ConstraintLengthRatio:       "length-ratio",
	core.ConstraintEqLenPtLineD:      "eq-length-and-pt-ln-dist",
	core.ConstraintEqPtLnDistances:   "eq-pt-line-distances",
	core.ConstraintEqualAngle:        "eq-angle",
	core.ConstraintEqualLineArcLen:   "eq-line-len-arc-len",
	core.ConstraintLengthDifference:  "length-difference",
	core.ConstraintSymmetric:         "symmetric",
	core.ConstraintSymmetricHoriz:    "symmetric-h",
	core.ConstraintSymmetricVert:     "symmetric-v",
	core.ConstraintSymmetricLine:     "symmetric-line",
	core.ConstraintAtMidpoint:        "at-midpoint",
	core.ConstraintHorizontal:        "horizontal",
	core.ConstraintVertical:          "vertical",
	core.ConstraintDiameter:          "diameter",
	core.ConstraintPtOnCircle:        "pt-on-circle",
	core.ConstraintSameOrientation:   "same-orientation",
	core.ConstraintAngle:             "angle",
	core.ConstraintParallel:          "parallel",
	core.ConstraintPerpendicular:     "perpendicular",
	core.ConstraintArcLineTangent:    "arc-line-tangent",
	core.ConstraintCubicLineTangent:  "cubic-line-tangent",
	core.ConstraintCurveCurveTangent: "curve-curve-tangent",
	core.ConstraintEqualRadius:       "eq-radius",
	core.ConstraintWhereDragged:      "lock-where-dragged",
	core.ConstraintArcArcLenRatio:    "arc-arc-length-ratio",
	core.ConstraintArcLineLenRatio:   "arc-line-length-ratio",
	core.ConstraintArcArcDifference:  "arc-arc-len-difference",
	core.ConstraintArcLineDifference: "arc-line-len-difference",
	core.ConstraintAxisAngle:         "axis-angle",
	core.ConstraintComment:           "comment",
})

var unitsAs = newEnum("unit", map[core.UnitsAs]string{
	core.UnitsPixels: "pixels",
	core.UnitsMM:     "mm",
})

var stipples = newEnum("stipple", map[core.StipplePattern]string{
	core.StippleContinuous: "continuous",
	core.StippleShortDash:  "short-dash",
	core.StippleDash:       "dash",
	core.StippleLongDash:   "long-dash",
	core.StippleDashDot:    "dash-dot",
	core.StippleDashDotDot: "dash-dot-dot",
	core.StippleDot:        "dot",
	core.StippleFreehand:   "freehand",
	core.StippleZigzag:     "zigzag",
})

var textOrigins = newEnum("text origin", map[core.TextOrigin]string{
	core.TextOriginLeft:  "left",
	core.TextOriginRight: "right",
	core.TextOriginBot:   "bot",
	core.TextOriginTop:   "top",
})

var docUnits = newEnum("unit", map[core.Unit]string{
	core.UnitMM:         "mm",
	core.UnitInches:     "inch",
	core.UnitMeters:     "m",
	core.UnitFeetInches: "ft-in",
})

var limits = newEnum("limit", map[core.Limit]string{
	core.LimitMin: "min",
	core.LimitMax: "max",
})
