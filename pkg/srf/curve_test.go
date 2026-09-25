package srf

import (
	"math"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

func TestFromTransformationOfSCurveTranslate(t *testing.T) {
	exact := SBezier{Degree: 1}
	exact.Ctrl[0] = geom.Vector{X: 0, Y: 0, Z: 0}
	exact.Ctrl[1] = geom.Vector{X: 1, Y: 0, Z: 0}
	exact.Weight[0] = 1
	exact.Weight[1] = 1
	c := &SCurve{
		H:       core.HSCurve{V: 1},
		IsExact: true,
		Exact:   exact,
		Points: []SCurvePt{
			{P: geom.Vector{X: 0, Y: 0, Z: 0}},
			{P: geom.Vector{X: 1, Y: 0, Z: 0}},
		},
	}
	out := FromTransformationOfSCurve(c, geom.Vector{X: 5, Y: 3, Z: 1}, geom.QuaternionIdentity, 1.0)
	if math.Abs(out.Points[0].P.X-5) > 1e-9 || math.Abs(out.Points[0].P.Y-3) > 1e-9 {
		t.Errorf("point 0 after translate: got %v want (5, 3, 1)", out.Points[0].P)
	}
	if math.Abs(out.Points[1].P.X-6) > 1e-9 {
		t.Errorf("point 1 after translate: got %v want (6, 3, 1)", out.Points[1].P)
	}
	// Original should be unchanged.
	if c.Points[0].P.X != 0 {
		t.Errorf("original curve mutated: %v", c.Points[0].P)
	}
}

func TestFromTransformationOfSCurveScale(t *testing.T) {
	c := &SCurve{
		Points: []SCurvePt{
			{P: geom.Vector{X: 1, Y: 0, Z: 0}},
			{P: geom.Vector{X: 2, Y: 0, Z: 0}},
		},
	}
	out := FromTransformationOfSCurve(c, geom.Vector{}, geom.QuaternionIdentity, 2.0)
	if math.Abs(out.Points[0].P.X-2) > 1e-9 || math.Abs(out.Points[1].P.X-4) > 1e-9 {
		t.Errorf("scale: got %v %v want (2,0,0) (4,0,0)", out.Points[0].P, out.Points[1].P)
	}
}

func TestQuaternionIsIdentity(t *testing.T) {
	if !geom.QuaternionIdentity.IsIdentity() {
		t.Errorf("QuaternionIdentity should be identity")
	}
	if geom.QuaternionFromAxisAngle(geom.Vector{Z: 1}, 1.0).IsIdentity() {
		t.Errorf("rotated quat should not be identity")
	}
}
