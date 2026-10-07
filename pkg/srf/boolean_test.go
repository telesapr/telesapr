package srf

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

func TestSShellAssembly(t *testing.T) {
	// Build two non-overlapping shells.
	a := &SShell{}
	b := &SShell{}
	s1 := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	s2 := FromPlane(geom.Vector{Z: 10}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	s1.Color = core.RgbaColor{Red: 255}
	s2.Color = core.RgbaColor{Green: 255}
	a.Surface = append(a.Surface, s1)
	b.Surface = append(b.Surface, s2)
	combined := &SShell{}
	combined.MakeFromAssemblyOf(a, b)
	if len(combined.Surface) != 2 {
		t.Fatalf("expected 2 surfaces after assembly, got %d", len(combined.Surface))
	}
}

func TestSShellUnion(t *testing.T) {
	a := &SShell{}
	b := &SShell{}
	s1 := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	s2 := FromPlane(geom.Vector{Z: 5}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	a.Surface = append(a.Surface, s1)
	b.Surface = append(b.Surface, s2)
	combined := &SShell{}
	combined.MakeFromUnionOf(a, b)
	if combined.BooleanFailed {
		t.Fatalf("union should not fail")
	}
}

func TestSShellDifference(t *testing.T) {
	// a and b are disjoint boxes; a - b should equal a.
	boxA := extrudedBox(geom.Vector{}, geom.Vector{X: 1, Y: 1, Z: 1})
	boxB := extrudedBox(geom.Vector{X: 5, Y: 5, Z: 5}, geom.Vector{X: 6, Y: 6, Z: 6})
	combined := &SShell{}
	combined.MakeFromDifferenceOf(boxA, boxB)
	if combined.BooleanFailed {
		t.Fatalf("a-b (disjoint boxes) failed")
	}
	if v := shellVolume(combined); math.Abs(v-1) > 1e-9 {
		t.Fatalf("a-b (disjoint boxes) volume = %v, want 1", v)
	}
}

func TestSShellDifferenceEmpty(t *testing.T) {
	// a and b are the SAME box; a - b should be empty.
	boxA := extrudedBox(geom.Vector{}, geom.Vector{X: 1, Y: 1, Z: 1})
	boxB := extrudedBox(geom.Vector{}, geom.Vector{X: 1, Y: 1, Z: 1})
	combined := &SShell{}
	combined.MakeFromDifferenceOf(boxA, boxB)
	if v := shellVolume(combined); math.Abs(v) > 1e-9 {
		t.Fatalf("a-a (same box) volume = %v, want 0", v)
	}
}

// TestSShellIntersectionOverlappingBoxes verifies intersection of
// two overlapping boxes is non-empty.
func TestSShellIntersectionOverlappingBoxes(t *testing.T) {
	a := extrudedBox(geom.NewVector(0, 0, 0), geom.NewVector(2, 2, 2))
	b := extrudedBox(geom.NewVector(1, 1, 1), geom.NewVector(3, 3, 3))
	var out SShell
	out.MakeFromIntersectionOf(a, b)
	if len(out.Surface) != 6 || math.Abs(shellVolume(&out)-1) > 1e-9 {
		t.Fatalf("intersection of overlapping boxes: %d surfaces, volume %g", len(out.Surface), shellVolume(&out))
	}
}

// TestSShellAssemblyOfThreeShells verifies that MakeFromAssemblyOf
// concatenates multiple shells.
func TestSShellAssemblyOfThreeShells(t *testing.T) {
	var out SShell
	for i := range 3 {
		b := makeBoxShellForTest(
			geom.NewVector(float64(i*2), 0, 0),
			geom.NewVector(float64(i*2+1), 1, 1))
		out.MakeFromAssemblyOf(&out, b)
	}
	if len(out.Surface) == 0 {
		t.Fatalf("assembly of 3 boxes is empty")
	}
}

// TestSShellDifferenceOnEmptyInputs verifies that booleanDifference
// on two empty shells leaves the result empty (no surfaces).
func TestSShellDifferenceOnEmptyInputs(t *testing.T) {
	var a, b, out SShell
	out.MakeFromDifferenceOf(&a, &b)
	if len(out.Surface) != 0 {
		t.Errorf("expected empty difference, got %d surfaces", len(out.Surface))
	}
}

// TestSShellIntersectionOnEmptyInputs verifies that
// booleanIntersection on two empty shells leaves the result empty.
func TestSShellIntersectionOnEmptyInputs(t *testing.T) {
	var a, b, out SShell
	out.MakeFromIntersectionOf(&a, &b)
	if len(out.Surface) != 0 {
		t.Errorf("expected empty intersection, got %d surfaces", len(out.Surface))
	}
}

// TestSShellUnionOnEmptyInputs verifies that booleanUnion on two
// empty shells leaves the result empty.
func TestSShellUnionOnEmptyInputs(t *testing.T) {
	var a, b, out SShell
	out.MakeFromUnionOf(&a, &b)
	if len(out.Surface) != 0 {
		t.Errorf("expected empty union, got %d surfaces", len(out.Surface))
	}
}
