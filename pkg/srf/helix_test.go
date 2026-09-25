package srf

import (
	"math"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

func TestSShellMakeFromHelixOfProducesNonEmptyShell(t *testing.T) {
	// Build a small rectangular contour in the XY plane.
	loop := &SBezierLoopSet{}
	loop.Outer.Tag = 1
	loop.Outer.Curves = []SBezier{
		{Degree: 1, Ctrl: [4]geom.Vector{{0, 0, 0}, {1, 0, 0}}, Weight: [4]float64{1, 1}},
		{Degree: 1, Ctrl: [4]geom.Vector{{1, 0, 0}, {1, 1, 0}}, Weight: [4]float64{1, 1}},
		{Degree: 1, Ctrl: [4]geom.Vector{{1, 1, 0}, {0, 1, 0}}, Weight: [4]float64{1, 1}},
		{Degree: 1, Ctrl: [4]geom.Vector{{0, 1, 0}, {0, 0, 0}}, Weight: [4]float64{1, 1}},
	}
	var s SShell
	s.MakeFromHelixOf(loop, geom.Vector{0.5, 0.5, 0}, geom.Vector{Z: 1}, 2, 4.0, core.NewRgbaColor(255, 0, 0, 255))
	if len(s.Surface) == 0 {
		t.Fatalf("helix produced 0 surfaces")
	}
	// 12 segments × 1 curve × 4 contours = 48 surfaces.
	if len(s.Surface) != 48 {
		t.Logf("helix surfaces: %d (expected 48)", len(s.Surface))
	}
	// First surface should be roughly at theta=0 offset=0; second at
	// theta=π/6 offset=1/3 (turns=2, pitch=4).
	first := s.Surface[0]
	last := s.Surface[len(s.Surface)-1]
	if math.Abs(first.PointAt(0, 0).Z) > 1e-6 {
		t.Errorf("first helix surface starts at Z=0; got %v", first.PointAt(0, 0).Z)
	}
	// Last surface (start of final segment) has Z = (ns-1)/ns *
	// turns * pitch (the partial segment start). End of revolution
	// (v=1) at start of segment (u=0) gives the full axial offset
	// for that segment start: (ns-1)/ns * turns * pitch.
	if math.Abs(last.PointAt(0, 0).Z-(11.0/12.0)*2.0*4.0) > 0.5 {
		t.Errorf("last helix surface PointAt(0,0) Z ≈ %v, want ≈ 7.33", last.PointAt(0, 0).Z)
	}
	// End of revolution (v=1) at end of segment (u=1) is the full
	// axial extent: turns * pitch.
	if math.Abs(last.PointAt(1, 1).Z-2.0*4.0) > 0.5 {
		t.Errorf("last helix surface PointAt(1,1) Z ≈ %v, want ≈ 8", last.PointAt(1, 1).Z)
	}
}
