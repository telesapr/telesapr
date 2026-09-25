package srf

import (
	"testing"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

// TestIntersectAgainstTwoPlanes verifies that two intersecting
// planes produce at least one intersection curve.
func TestIntersectAgainstTwoPlanes(t *testing.T) {
	s1 := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	s2 := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Z: 1})
	s1.NewH = s1.H
	s2.NewH = s2.H
	into := &SShell{}
	s1.IntersectAgainst(&s2, nil, nil, into)
	// The intersection of the XY plane (z=0) and XZ plane (y=0) is
	// the X axis. We expect at least one curve.
	if len(into.Curve) == 0 {
		t.Fatalf("expected at least 1 intersection curve, got 0")
	}
}

// TestAddExactIntersectionCurve verifies that AddExactIntersectionCurve
// appends a curve to the result shell.
func TestAddExactIntersectionCurve(t *testing.T) {
	s1 := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	s2 := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Z: 1})
	s1.NewH = s1.H
	s2.NewH = s2.H
	into := &SShell{}
	sb := SBezierFrom2(geom.Vector{}, geom.Vector{X: 1})
	s1.AddExactIntersectionCurve(&sb, &s2, nil, nil, into)
	if len(into.Curve) != 1 {
		t.Fatalf("expected 1 curve, got %d", len(into.Curve))
	}
}
