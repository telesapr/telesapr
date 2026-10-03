package srf

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

func TestSBezierPointOnThisAndCurveIntersects(t *testing.T) {
	// Two line segments that cross at (0.5, 0.5, 0): a vertical
	// from (0.5, 0, 0) to (0.5, 1, 0) and a horizontal from (0, 0.5, 0)
	// to (1, 0.5, 0). PointOnThisAndCurve should converge to (0.5, 0.5, 0).
	a := SBezier{Degree: 1}
	a.Ctrl[0] = geom.Vector{X: 0.5, Y: 0, Z: 0}
	a.Ctrl[1] = geom.Vector{X: 0.5, Y: 1, Z: 0}
	a.Weight[0] = 1
	a.Weight[1] = 1
	b := SBezier{Degree: 1}
	b.Ctrl[0] = geom.Vector{X: 0, Y: 0.5, Z: 0}
	b.Ctrl[1] = geom.Vector{X: 1, Y: 0.5, Z: 0}
	b.Weight[0] = 1
	b.Weight[1] = 1
	p := geom.Vector{X: 0.5, Y: 0.5, Z: 0}
	if !a.PointOnThisAndCurve(&b, &p) {
		t.Fatalf("PointOnThisAndCurve failed to converge")
	}
	if math.Abs(p.X-0.5) > 1e-9 || math.Abs(p.Y-0.5) > 1e-9 {
		t.Errorf("intersection point: got %v want (0.5, 0.5, 0)", p)
	}
}

func TestSBezierPointOnNonparallelCurveRejectsParallel(t *testing.T) {
	// Two parallel line segments: should return false.
	a := SBezier{Degree: 1}
	a.Ctrl[0] = geom.Vector{X: 0, Y: 0, Z: 0}
	a.Ctrl[1] = geom.Vector{X: 1, Y: 0, Z: 0}
	a.Weight[0] = 1
	a.Weight[1] = 1
	b := SBezier{Degree: 1}
	b.Ctrl[0] = geom.Vector{X: 0, Y: 1, Z: 0}
	b.Ctrl[1] = geom.Vector{X: 1, Y: 1, Z: 0}
	b.Weight[0] = 1
	b.Weight[1] = 1
	p := geom.Vector{X: 0.5, Y: 0.5, Z: 0}
	if a.PointOnNonparallelCurve(&b, &p) {
		t.Errorf("parallel lines: expected rejection, got p=%v", p)
	}
}

func TestSBezierPointOnNonparallelCurveAcceptsCrossing(t *testing.T) {
	a := SBezier{Degree: 1}
	a.Ctrl[0] = geom.Vector{X: 0, Y: 0.5, Z: 0}
	a.Ctrl[1] = geom.Vector{X: 1, Y: 0.5, Z: 0}
	a.Weight[0] = 1
	a.Weight[1] = 1
	b := SBezier{Degree: 1}
	b.Ctrl[0] = geom.Vector{X: 0.5, Y: 0, Z: 0}
	b.Ctrl[1] = geom.Vector{X: 0.5, Y: 1, Z: 0}
	b.Weight[0] = 1
	b.Weight[1] = 1
	p := geom.Vector{X: 0.5, Y: 0.5, Z: 0}
	if !a.PointOnNonparallelCurve(&b, &p) {
		t.Errorf("crossing lines: expected acceptance, got false")
	}
	if math.Abs(p.X-0.5) > 1e-9 || math.Abs(p.Y-0.5) > 1e-9 {
		t.Errorf("crossing lines: got %v want (0.5, 0.5, 0)", p)
	}
}
