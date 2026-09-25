package slvs

import (
	"path/filepath"
	"runtime"
	"testing"
)

// boolFixtureStats holds the actual stats we compute per fixture.
type boolFixtureStats struct {
	Triangles int
	VMin      [3]float64
	VMax      [3]float64
}

// booleanFixtureGoldens is the recorded baseline of expected
// triangle counts. The values come from the Go port's first run
// after the SLVS parser was taught to associate entities with their
// parent group via the workplane reference (Phase E).
var booleanFixtureGoldens = map[string]boolFixtureStats{
	"boolean_coplanar_union":   {Triangles: 12288},
	"boolean_knife_edge":        {Triangles: 6656},
	"boolean_tangent_crossing": {Triangles: 5120},
	"boolean_tangent_edge":     {Triangles: 6656},
	"boolean_tangent_fillet":    {Triangles: 4608},
	"boolean_tangent_spline":    {Triangles: 5120},
}

// collectBoolStats loads a fixture and reports its mesh statistics.
func collectBoolStats(path string) (boolFixtureStats, error) {
	doc, err := LoadFile(path)
	if err != nil {
		return boolFixtureStats{}, err
	}
	if doc.Sketch == nil {
		return boolFixtureStats{}, nil
	}
	doc.Sketch.GenerateAll()
	var stats boolFixtureStats
	for _, g := range doc.Sketch.Groups {
		if !g.IsMeshGroup() {
			continue
		}
		mesh := &g.RunningMesh
		stats.Triangles += len(mesh.Triangles)
		if len(mesh.Triangles) > 0 {
			vmax, vmin := mesh.GetBounding()
			stats.VMax = [3]float64{vmax.X, vmax.Y, vmax.Z}
			stats.VMin = [3]float64{vmin.X, vmin.Y, vmin.Z}
		}
	}
	return stats, nil
}

// TestBooleanFixtureLoadsAndGenerates runs every boolean_* fixture
// through LoadFile + Sketch::GenerateAll and asserts that the load
// succeeds, GenerateAll completes without panic, and the resulting
// mesh is non-empty. This is the parity-smoke gate for Phase E.
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

// TestParityBooleanFixtures asserts that each boolean fixture produces
// exactly the recorded triangle count. This is a stronger version of
// the smoke gate: it catches regressions in the boolean-CSG chain
// that produce non-empty but malformed meshes.
func TestParityBooleanFixtures(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))))
	for name, golden := range booleanFixtureGoldens {
		path := filepath.Join(root, "testdata/group", name, "normal.slvs")
		stats, err := collectBoolStats(path)
		if err != nil {
			t.Errorf("%s: load: %v", name, err)
			continue
		}
		if stats.Triangles == 0 {
			t.Errorf("%s: no triangles after GenerateAll", name)
			continue
		}
		if stats.Triangles != golden.Triangles {
			t.Errorf("%s: triangle count drift: got %d, want %d",
				name, stats.Triangles, golden.Triangles)
		}
	}
}

// TestParityBooleanFixtureBBox records the bounding boxes from the
// current run so they can be promoted to golden values. Run with
// `go test -run TestParityBooleanFixtureBBox -v` to print the boxes.
func TestParityBooleanFixtureBBox(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))))
	names := []string{
		"boolean_coplanar_union",
		"boolean_knife_edge",
		"boolean_tangent_crossing",
		"boolean_tangent_edge",
		"boolean_tangent_fillet",
		"boolean_tangent_spline",
	}
	for _, name := range names {
		path := filepath.Join(root, "testdata/group", name, "normal.slvs")
		stats, err := collectBoolStats(path)
		if err != nil {
			t.Errorf("%s: load: %v", name, err)
			continue
		}
		t.Logf("%s: triangles=%d vmin=%v vmax=%v",
			name, stats.Triangles, stats.VMin, stats.VMax)
	}
}
