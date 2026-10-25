package sketch

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// TestGroupSketchAccessorsViaSketchPackage verifies accessors work.
func TestGroupSketchAccessorsViaSketchPackage(t *testing.T) {
	t.Parallel()
	g := &Group{H: core.HGroup{V: 1}}
	if g.IsMeshGroup() {
		t.Errorf("default group should not be mesh group")
	}
	if g.IsVisible() {
		t.Errorf("default group should not be visible")
	}
}

// TestGroupIsMeshGroupTrueForAllMeshTypes verifies all mesh group types.
func TestGroupIsMeshGroupTrueForAllMeshTypes(t *testing.T) {
	t.Parallel()
	for _, gt := range []GroupType{
		GroupExtrude, GroupLathe, GroupRevolve, GroupHelix,
		GroupRotate, GroupTranslate,
	} {
		g := &Group{Type: gt}
		if !g.IsMeshGroup() {
			t.Errorf("%v should be a mesh group", gt)
		}
	}
}

// TestGroupIsMeshGroupFalseForNonMeshTypes verifies drawing groups
// are not mesh groups.
func TestGroupIsMeshGroupFalseForNonMeshTypes(t *testing.T) {
	t.Parallel()
	for _, gt := range []GroupType{
		GroupDrawing3D, GroupDrawingWorkplane, GroupLinked,
	} {
		g := &Group{Type: gt}
		if g.IsMeshGroup() {
			t.Errorf("%v should not be a mesh group", gt)
		}
	}
}

// TestGroupIsTriangleMeshAssemblyBasic checks SolveSpace's rule: a linked
// group whose file brought a mesh and no shell.
func TestGroupIsTriangleMeshAssemblyBasic(t *testing.T) {
	t.Parallel()
	g := &Group{Type: GroupLinked}
	g.ImpMesh.AddTriangle3(srf.STriMeta{}, geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	if !g.IsTriangleMeshAssembly() {
		t.Errorf("linked group with only a mesh should be a triangle mesh assembly")
	}
	g.ImpShell.Surface = append(g.ImpShell.Surface, srf.SSurface{})
	if g.IsTriangleMeshAssembly() {
		t.Errorf("linked group with a shell should not be a triangle mesh assembly")
	}
	g = &Group{Type: GroupExtrude, ForceToMesh: true, MeshCombine: CombineAssemble}
	if g.IsTriangleMeshAssembly() {
		t.Errorf("only linked groups are triangle mesh assemblies")
	}
}
