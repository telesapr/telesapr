package sketch

import (
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// TestGroupGenerateShellAndMeshDispatch verifies the type dispatch.
func TestGroupGenerateShellAndMeshDispatch(t *testing.T) {
	sk := NewSketch()
	cases := []GroupType{
		GroupDrawing3D,
		GroupDrawingWorkplane,
		GroupLinked,
		GroupExtrude,
		GroupLathe,
		GroupHelix,
		GroupRotate,
		GroupTranslate,
	}
	for _, gt := range cases {
		g := &Group{H: core.HGroup{V: 1}, Type: gt}
		// Should not panic even without proper setup.
		g.GenerateShellAndMesh(sk)
		_ = sk
	}
}

// TestGroupGenerateForBooleanNullArgs tests no-arg path.
func TestGroupGenerateForBooleanNullArgs(t *testing.T) {
	g := &Group{H: core.HGroup{V: 1}, Type: GroupExtrude}
	out := &srf.SMesh{}
	g.GenerateForBoolean(nil, out)
	// No-op when prev is nil.
}

// TestGroupGenerateForStepAndRepeatNullArgs tests no-arg path.
func TestGroupGenerateForStepAndRepeatNullArgs(t *testing.T) {
	g := &Group{H: core.HGroup{V: 1}, Type: GroupRotate}
	out := &srf.SMesh{}
	g.GenerateForStepAndRepeat(nil, out)
	// No-op when src is nil.
}

// TestGroupGenerateDisplayItemsNoError tests no-panic dispatch.
func TestGroupGenerateDisplayItemsNoError(t *testing.T) {
	g := &Group{H: core.HGroup{V: 1}, Type: GroupExtrude}
	g.GenerateDisplayItems()
}

// TestGroupGenerateLoopsNoError tests no-panic.
func TestGroupGenerateLoopsNoError(t *testing.T) {
	sk := NewSketch()
	g := &Group{H: core.HGroup{V: 1}, Type: GroupDrawing3D}
	g.GenerateLoops()
	_ = sk
}

// TestGroupGenerateLinkedShell tests the Linked branch.
func TestGroupGenerateLinkedShell(t *testing.T) {
	sk := NewSketch()
	g := &Group{H: core.HGroup{V: 1}, Type: GroupLinked}
	g.generateLinkedShell(sk)
	// ThisShell/ThisMesh should be cleared to empty (no imported data yet).
	if len(g.ThisShell.Surface) != 0 {
		t.Errorf("Linked shell should be empty (no imported data), got %d surfaces",
			len(g.ThisShell.Surface))
	}
}

// TestGroupAnnotateExtrudeFacesNullArgs tests no-arg path.
func TestGroupAnnotateExtrudeFacesNullArgs(t *testing.T) {
	g := &Group{H: core.HGroup{V: 1}, Type: GroupExtrude}
	g.annotateExtrudeFaces(nil, nil, geom.Vector{}, geom.Vector{})
	// Should not panic.
}


// TestGroupAnnotateExtrudeFacesTopMatchesWithSource verifies top-face detection with source.
func TestGroupAnnotateExtrudeFacesTopMatchesWithSource(t *testing.T) {
	sk := NewSketch()
	// Add source group.
	src := &Group{H: core.HGroup{V: 2}, Type: GroupDrawing3D}
	sk.Groups = append(sk.Groups, *src)
	// Add extrude group with OpA pointing to source.
	g := &Group{H: core.HGroup{V: 1}, Type: GroupExtrude, OpA: core.HGroup{V: 2}}
	shell := &srf.SShell{}
	shell.Surface = append(shell.Surface, srf.FromPlane(
		geom.Vector{Z: 1}, geom.Vector{X: 1}, geom.Vector{Y: 1},
	))
	g.annotateExtrudeFaces(sk, shell, geom.Vector{}, geom.Vector{Z: 1})
	if shell.Surface[0].Face != FaceIDTop {
		t.Errorf("expected top face annotation, got %v", shell.Surface[0].Face)
	}
}
