package sketch

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// TestGroupGenerateLoopsNoError tests no-panic.
func TestGroupGenerateLoopsNoError(t *testing.T) {
	t.Parallel()
	sk := NewSketch()
	g := &Group{H: core.HGroup{V: 1}, Type: GroupDrawing3D}
	g.GenerateLoops(sk, NewRuntime(sk))
}

// TestGroupGenerateLinkedShell tests the Linked branch.
func TestGroupGenerateLinkedShell(t *testing.T) {
	t.Parallel()
	sk := NewSketch()
	g := &Group{H: core.HGroup{V: 1}, Type: GroupLinked}
	g.generateLinkedShell(sk)
	// ThisShell/ThisMesh should be cleared to empty (no imported data yet).
	if len(g.ThisShell.Surface) != 0 {
		t.Errorf("Linked shell should be empty (no imported data), got %d surfaces",
			len(g.ThisShell.Surface))
	}
}

// TestGroupGenerateLinkedShellWithImportedMesh verifies LINKED group transforms mesh.
func TestGroupGenerateLinkedShellWithImportedMesh(t *testing.T) {
	t.Parallel()
	sk := NewSketch()
	g := &Group{H: core.HGroup{V: 1}, Type: GroupLinked, Scale: 1.0}
	// Add imported mesh with one triangle.
	g.ImpMesh.Triangles = append(g.ImpMesh.Triangles, srf.STriangle{
		A: geom.Vector{}, B: geom.Vector{X: 1}, C: geom.Vector{Y: 1},
	})
	g.generateLinkedShell(sk)
	if len(g.ThisMesh.Triangles) != 1 {
		t.Errorf("expected 1 transformed triangle, got %d", len(g.ThisMesh.Triangles))
	}
}

// TestGroupGenerateLinkedShellWithImportedShell verifies LINKED group transforms shell.
func TestGroupGenerateLinkedShellWithImportedShell(t *testing.T) {
	t.Parallel()
	sk := NewSketch()
	g := &Group{H: core.HGroup{V: 1}, Type: GroupLinked, Scale: 1.0}
	g.ImpShell.Surface = append(g.ImpShell.Surface,
		srf.FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1}))
	g.generateLinkedShell(sk)
	if len(g.ThisShell.Surface) != 1 {
		t.Errorf("expected 1 transformed surface, got %d", len(g.ThisShell.Surface))
	}
}

// TestGroupGenerateLinkedShellEmpty verifies LINKED group with no imported data.
func TestGroupGenerateLinkedShellEmpty(t *testing.T) {
	t.Parallel()
	sk := NewSketch()
	g := &Group{H: core.HGroup{V: 1}, Type: GroupLinked, Scale: 1.0}
	g.generateLinkedShell(sk)
	if len(g.ThisMesh.Triangles) != 0 {
		t.Errorf("empty LINKED should produce empty mesh")
	}
}

// TestGroupParamFloat verifies paramFloat lookup.
func TestGroupParamFloat(t *testing.T) {
	t.Parallel()
	sk := NewSketch()
	sk.Params = append(sk.Params, core.Param{H: core.HParam{V: 7}, Val: 42.0})
	got := paramFloat(sk, core.HParam{V: 7})
	if got != 42.0 {
		t.Errorf("paramFloat = %v, want 42.0", got)
	}
	if paramFloat(sk, core.HParam{V: 999}) != 0 {
		t.Errorf("missing param should return 0")
	}
	if paramFloat(nil, core.HParam{V: 1}) != 0 {
		t.Errorf("nil sketch should return 0")
	}
}
