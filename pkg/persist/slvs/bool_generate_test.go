package slvs

import (
	"path/filepath"
	"runtime"
	"testing"
)

// TestBooleanFixtureLoadsAndGenerates runs every boolean_* fixture
// through LoadFile + Sketch::GenerateAll and asserts that the load
// succeeds and GenerateAll completes without panic. This is the
// parity-smoke gate for Phase E.
func TestBooleanFixtureLoadsAndGenerates(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))))
	paths := []string{
		filepath.Join(root, "testdata/group/boolean_coplanar_union/normal.slvs"),
		filepath.Join(root, "testdata/group/boolean_knife_edge/normal.slvs"),
		filepath.Join(root, "testdata/group/boolean_tangent_crossing/normal.slvs"),
		filepath.Join(root, "testdata/group/boolean_tangent_edge/normal.slvs"),
		filepath.Join(root, "testdata/group/boolean_tangent_fillet/normal.slvs"),
		filepath.Join(root, "testdata/group/boolean_tangent_spline/normal.slvs"),
	}
	for _, p := range paths {
		doc, err := LoadFile(p)
		if err != nil {
			t.Errorf("load %s: %v", p, err)
			continue
		}
		if doc.Sketch == nil {
			t.Errorf("%s: nil sketch after load", p)
			continue
		}
		// GenerateAll must complete without panicking.
		doc.Sketch.GenerateAll()
		totalTris := 0
		for i := range doc.Sketch.Groups {
			g := &doc.Sketch.Groups[i]
			if g.IsMeshGroup() {
				totalTris += len(g.RunningMesh.Triangles)
			}
		}
		if totalTris == 0 {
			t.Errorf("%s: no triangles after GenerateAll", p)
		}
	}
}
