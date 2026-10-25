package srf

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// TestSShellFindCurveNull verifies nil-safe FindCurve.
func TestSShellFindCurveNull(t *testing.T) {
	t.Parallel()
	var s *SShell
	if s.FindCurve(core.HSCurve{V: 1}) != nil {
		t.Errorf("nil shell should return nil")
	}
}

// TestSShellFindCurveEmpty verifies FindCurve on empty shell.
func TestSShellFindCurveEmpty(t *testing.T) {
	t.Parallel()
	s := &SShell{}
	if s.FindCurve(core.HSCurve{V: 1}) != nil {
		t.Errorf("empty shell should return nil")
	}
}

// TestSShellFindCurveRoundTrip verifies FindCurve finds a curve by handle.
func TestSShellFindCurveRoundTrip(t *testing.T) {
	t.Parallel()
	s := &SShell{}
	s.Curve = append(s.Curve, SCurve{H: core.HSCurve{V: 7}})
	got := s.FindCurve(core.HSCurve{V: 7})
	if got == nil {
		t.Fatalf("FindCurve returned nil")
	}
	if got.H.V != 7 {
		t.Errorf("H = %v, want 7", got.H.V)
	}
}

// TestSShellFindSurfaceRoundTrip verifies FindSurface by handle.
func TestSShellFindSurfaceRoundTrip(t *testing.T) {
	t.Parallel()
	s := &SShell{}
	s.Surface = append(s.Surface, SSurface{H: core.HSSurface{V: 3}})
	got := s.FindSurface(core.HSSurface{V: 3})
	if got == nil {
		t.Fatalf("FindSurface returned nil")
	}
	if got.H.V != 3 {
		t.Errorf("H = %v, want 3", got.H.V)
	}
}

// TestSShellFindCurveBySurface verifies surface-to-curves lookup.
func TestSShellFindCurveBySurface(t *testing.T) {
	t.Parallel()
	s := &SShell{}
	s.Surface = append(s.Surface, SSurface{H: core.HSSurface{V: 5}})
	s.Curve = append(s.Curve, SCurve{H: core.HSCurve{V: 1}, SurfA: core.HSSurface{V: 5}})
	s.Curve = append(s.Curve, SCurve{H: core.HSCurve{V: 2}, SurfB: core.HSSurface{V: 5}})
	s.Curve = append(s.Curve, SCurve{H: core.HSCurve{V: 3}, SurfA: core.HSSurface{V: 99}})
	got := s.FindCurveBySurface(core.HSSurface{V: 5})
	if len(got) != 2 {
		t.Errorf("expected 2 curves, got %d", len(got))
	}
}

// TestSTrimByEntireCurveFromShellNull verifies nil-shell path.
func TestSTrimByEntireCurveFromShellNull(t *testing.T) {
	t.Parallel()
	stb := STrimByEntireCurveFromShell(nil, core.HSCurve{V: 1}, false)
	if stb.Start != 0 || stb.Finish != 1 {
		t.Errorf("expected full range, got [%v, %v]", stb.Start, stb.Finish)
	}
}

// TestSTrimByEntireCurveFromShellEmpty verifies empty-shell path.
func TestSTrimByEntireCurveFromShellEmpty(t *testing.T) {
	t.Parallel()
	shell := &SShell{}
	stb := STrimByEntireCurveFromShell(shell, core.HSCurve{V: 1}, false)
	if stb.Start != 0 || stb.Finish != 1 {
		t.Errorf("expected full range, got [%v, %v]", stb.Start, stb.Finish)
	}
}

// TestSTrimByEntireCurveFromShellForward verifies forward range.
func TestSTrimByEntireCurveFromShellForward(t *testing.T) {
	t.Parallel()
	shell := &SShell{}
	shell.Curve = append(shell.Curve, SCurve{
		H: core.HSCurve{V: 1},
		Points: []SCurvePt{
			{P: geom.Vector{}},
			{P: geom.Vector{X: 5}},
		},
	})
	stb := STrimByEntireCurveFromShell(shell, core.HSCurve{V: 1}, false)
	if !stb.StartP.Equals(geom.Vector{}) {
		t.Errorf("StartP = %v, want origin", stb.StartP)
	}
	if !stb.FinishP.Equals(geom.Vector{X: 5}) {
		t.Errorf("FinishP = %v, want (5,0,0)", stb.FinishP)
	}
	if stb.Backwards {
		t.Errorf("forward range should have Backwards=false")
	}
}

// TestSTrimByEntireCurveFromShellBackward verifies backward range.
func TestSTrimByEntireCurveFromShellBackward(t *testing.T) {
	t.Parallel()
	shell := &SShell{}
	shell.Curve = append(shell.Curve, SCurve{
		H: core.HSCurve{V: 1},
		Points: []SCurvePt{
			{P: geom.Vector{}},
			{P: geom.Vector{X: 5}},
		},
	})
	stb := STrimByEntireCurveFromShell(shell, core.HSCurve{V: 1}, true)
	if !stb.StartP.Equals(geom.Vector{X: 5}) {
		t.Errorf("backward StartP = %v, want (5,0,0)", stb.StartP)
	}
	if !stb.FinishP.Equals(geom.Vector{}) {
		t.Errorf("backward FinishP = %v, want origin", stb.FinishP)
	}
	if !stb.Backwards {
		t.Errorf("backward range should have Backwards=true")
	}
}
