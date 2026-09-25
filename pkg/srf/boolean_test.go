package srf

import (
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
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
	// a and b are disjoint closed box shells; a - b should equal a.
	boxA := makeBoxShell(geom.Vector{}, geom.Vector{X: 1, Y: 1, Z: 1})
	boxB := makeBoxShell(geom.Vector{X: 5, Y: 5, Z: 5}, geom.Vector{X: 6, Y: 6, Z: 6})
	combined := &SShell{}
	combined.MakeFromDifferenceOf(boxA, boxB)
	if len(combined.Surface) != 6 {
		t.Fatalf("a-b (disjoint boxes) should have 6 surfaces (a's box), got %d", len(combined.Surface))
	}
}

func TestSShellDifferenceEmpty(t *testing.T) {
	// a and b are the SAME box; a - b should be empty.
	boxA := makeBoxShell(geom.Vector{}, geom.Vector{X: 1, Y: 1, Z: 1})
	boxB := makeBoxShell(geom.Vector{}, geom.Vector{X: 1, Y: 1, Z: 1})
	combined := &SShell{}
	combined.MakeFromDifferenceOf(boxA, boxB)
	if len(combined.Surface) != 0 {
		t.Fatalf("a-a (same box) should be empty, got %d", len(combined.Surface))
	}
}

// makeBoxShell builds a closed shell representing an axis-aligned
// box from min to max. The 6 faces are the standard inward-facing
// box faces.
// makeBoxShell builds a closed shell representing an axis-aligned
// box from min to max. The 6 faces are the standard inward-facing
// box faces.
func makeBoxShell(min, max geom.Vector) *SShell {
	bilinear := func(p00, p10, p01, p11 geom.Vector) [4][4]geom.Vector {
		return [4][4]geom.Vector{
			{p00, p01, {}, {}},
			{p10, p11, {}, {}},
			{{}, {}, {}, {}},
			{{}, {}, {}, {}},
		}
	}
	bilinearWeight := [4][4]float64{{1, 1, 1, 1}, {1, 1, 1, 1}, {1, 1, 1, 1}, {1, 1, 1, 1}}

	face := func(p00, p10, p01, p11 geom.Vector) SSurface {
		return SSurface{DegM: 1, DegN: 1, Ctrl: bilinear(p00, p10, p01, p11), Weight: bilinearWeight}
	}
	
	minZ := func() geom.Vector { return geom.Vector{X: min.X, Y: min.Y, Z: min.Z} }
	maxZ := func() geom.Vector { return geom.Vector{X: max.X, Y: max.Y, Z: max.Z} }
	cMinMinZ := func() geom.Vector { return geom.Vector{X: min.X, Y: min.Y, Z: min.Z} }
	cMinMaxZ := func() geom.Vector { return geom.Vector{X: min.X, Y: min.Y, Z: max.Z} }
	_ = minZ
	_ = maxZ

	sh := &SShell{}
	// Bottom (z=min)
	sh.Surface = append(sh.Surface, face(
		cMinMinZ(),
		geom.Vector{X: max.X, Y: min.Y, Z: min.Z},
		geom.Vector{X: min.X, Y: max.Y, Z: min.Z},
		geom.Vector{X: max.X, Y: max.Y, Z: min.Z},
	))
	// Top (z=max)
	sh.Surface = append(sh.Surface, face(
		cMinMaxZ(),
		geom.Vector{X: min.X, Y: max.Y, Z: max.Z},
		geom.Vector{X: max.X, Y: min.Y, Z: max.Z},
		geom.Vector{X: max.X, Y: max.Y, Z: max.Z},
	))
	// Front (y=min)
	sh.Surface = append(sh.Surface, face(
		cMinMinZ(),
		cMinMaxZ(),
		geom.Vector{X: max.X, Y: min.Y, Z: min.Z},
		geom.Vector{X: max.X, Y: min.Y, Z: max.Z},
	))
	// Back (y=max)
	sh.Surface = append(sh.Surface, face(
		geom.Vector{X: min.X, Y: max.Y, Z: min.Z},
		geom.Vector{X: max.X, Y: max.Y, Z: min.Z},
		cMinMaxZ(),
		geom.Vector{X: max.X, Y: max.Y, Z: max.Z},
	))
	// Left (x=min)
	sh.Surface = append(sh.Surface, face(
		cMinMinZ(),
		cMinMaxZ(),
		geom.Vector{X: min.X, Y: max.Y, Z: min.Z},
		cMinMaxZ(),
	))
	// Right (x=max)
	sh.Surface = append(sh.Surface, face(
		geom.Vector{X: max.X, Y: min.Y, Z: min.Z},
		geom.Vector{X: max.X, Y: max.Y, Z: min.Z},
		geom.Vector{X: max.X, Y: min.Y, Z: max.Z},
		cMinMaxZ(),
	))

	for i := range sh.Surface {
		sh.Surface[i].H = core.HSSurface{V: uint32(i + 1)}
		sh.Surface[i].Face = FaceID(uint32(i + 1))
	}
	return sh
}

// TestSMeshUnionDisjointBoxes verifies the BSP ray-cast union of
// two disjoint closed boxes keeps all triangles.
func TestSMeshUnionDisjointBoxes(t *testing.T) {
	a := makeBoxMeshForTest(geom.Vector{}, geom.Vector{X: 1, Y: 1, Z: 1})
	b := makeBoxMeshForTest(geom.Vector{X: 5, Y: 5, Z: 5}, geom.Vector{X: 6, Y: 6, Z: 6})
	combined := &SMesh{}
	combined.MakeFromUnionOf(a, b)
	if len(combined.Triangles) != len(a.Triangles)+len(b.Triangles) {
		t.Fatalf("disjoint union should have %d triangles, got %d",
			len(a.Triangles)+len(b.Triangles), len(combined.Triangles))
	}
}

// TestSMeshDifferenceDisjointBoxes verifies the difference of two
// disjoint boxes leaves the first intact.
func TestSMeshDifferenceDisjointBoxes(t *testing.T) {
	a := makeBoxMeshForTest(geom.Vector{}, geom.Vector{X: 1, Y: 1, Z: 1})
	b := makeBoxMeshForTest(geom.Vector{X: 5, Y: 5, Z: 5}, geom.Vector{X: 6, Y: 6, Z: 6})
	diff := &SMesh{}
	diff.MakeFromDifferenceOf(a, b)
	if len(diff.Triangles) != len(a.Triangles) {
		t.Fatalf("disjoint difference should have %d triangles, got %d",
			len(a.Triangles), len(diff.Triangles))
	}
}

// makeBoxMeshForTest builds a triangulated box shell as an SMesh.
func makeBoxMeshForTest(min, max geom.Vector) *SMesh {
	sh := makeBoxShellForTest(min, max)
	m := &SMesh{}
	for i := range sh.Surface {
		sh.Surface[i].TriangulateInto(m)
	}
	return m
}
