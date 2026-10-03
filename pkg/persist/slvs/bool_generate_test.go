package slvs

import (
	"math"

	"github.com/telesapr/telesapr/pkg/srf"
	"path/filepath"
	"runtime"
	"testing"
)

// boolFixtureStats holds the actual stats we compute per fixture.
type boolFixtureStats struct {
	Triangles int
	VMin      [3]float64
	VMax      [3]float64
	Volume    float64 // signed volume of the last (active) mesh group
	// Naked and self-intersecting edges of the active group's display
	// mesh, as the C++ fixture tests find them.
	NakedEdges   int
	Inters, Leak bool
	TotalVol     float64 // sum of all mesh-group running volumes
}

// booleanFixtureGoldens holds, per fixture, the Go-port triangle
// count (a regression baseline, not a C++ value) and the bounding box
// of the active group's mesh, which is the model's exact extent.
//
// The active group in the C++ binary is SK.GetGroup(activeGroup),
// which corresponds to the last group in topological order for
// freshly-loaded fixtures. Triangles are summed across every mesh
// group's running mesh.
var booleanFixtureGoldens = map[string]boolFixtureStats{
	"boolean_coplanar_union": {
		Triangles: 118,
		VMin:      [3]float64{0, -5, -10},
		VMax:      [3]float64{30, 20, 20},
	},
	"boolean_knife_edge": {
		Triangles: 40,
		VMin:      [3]float64{-1, -1.6126052390506485, 0},
		VMax:      [3]float64{1.5498346036820765, 1, 1},
	},
	"boolean_tangent_crossing": {
		Triangles: 94,
		VMin:      [3]float64{-30, -30, -30},
		VMax:      [3]float64{30, 30, 30},
	},
	"boolean_tangent_edge": {
		Triangles: 48,
		VMin:      [3]float64{0, 0, 0},
		VMax:      [3]float64{80, 80, 80},
	},
	"boolean_tangent_fillet": {
		Triangles: 88,
		VMin:      [3]float64{0, -2, -1},
		VMax:      [3]float64{1, 0, 0},
	},
	"boolean_tangent_spline": {
		Triangles: 94,
		VMin:      [3]float64{-30, -30, -30},
		VMax:      [3]float64{30, 30, 30},
	},
}

// booleanFixtureCppGoldens holds the C++ reference volumes produced
// by the C++ SolveSpace CLI testsuite. Each fixture's value comes
// from `m->CalculateVolume() / golden = 1.0` assertions in
// solvespace/test/group/<name>/test.cpp. The C++ testsuite is the
// ground truth; the Go port's volumes should converge to these as
// the boolean-CSG pipeline matures.
//
// They were read from the C++ testsuite's `CalculateVolume() / N == 1.0`
// checks; since the C++ comparison was frozen they are this project's
// goldens, changed by hand when the kernel changes a volume on purpose.
var booleanFixtureCppGoldens = map[string]float64{
	"boolean_coplanar_union":   13250.0,
	"boolean_knife_edge":       1.8303900357690596,
	"boolean_tangent_crossing": 205477.058604742,
	"boolean_tangent_edge":     296000.0,
	"boolean_tangent_fillet":   0.9730161212,
	"boolean_tangent_spline":   204638.6271882122,
}

// cppGoldenRatioTol is how far stats.Volume / cpp may be from 1: the
// C++ testsuite's own criterion, |ratio - 1| < LENGTH_EPS (1e-6;
// test/harness.cpp, CheckEqualEpsilon).
const cppGoldenRatioTol = 1e-6

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
	for i := range doc.Sketch.Groups {
		g := &doc.Sketch.Groups[i]
		if !g.IsMeshGroup() {
			continue
		}
		// The mesh C++ displays, and its fixture tests check.
		g.GenerateDisplayItems(doc.Sketch)
		mesh := &g.DisplayMesh
		stats.Triangles += len(mesh.Triangles)
		var el srf.SEdgeList
		srf.SKdNodeFrom(&g.DisplayMesh).MakeCertainEdgesInto(&el,
			srf.EdgeNakedOrSelfInter, true, &stats.Inters, &stats.Leak, 0)
		stats.NakedEdges = len(el.Edges)
		if len(g.DisplayMesh.Triangles) == 0 {
			// An empty mesh would pass vacuously.
			stats.Leak = true
		}
		v := mesh.CalculateVolume()
		stats.TotalVol += v
		stats.Volume = v
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
		surfaces := 0
		for i := range doc.Sketch.Groups {
			g := &doc.Sketch.Groups[i]
			if g.IsMeshGroup() {
				surfaces += len(g.RunningShell.Surface)
			}
		}
		if surfaces == 0 {
			t.Errorf("%s: empty running shells after GenerateAll", p)
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
		// The C++ fixture tests also require the mesh to be watertight:
		// no naked or self-intersecting edges.
		if stats.NakedEdges != 0 || stats.Inters || stats.Leak {
			t.Errorf("%s: %d naked or self-intersecting edges (inters %v, leaks %v)",
				name, stats.NakedEdges, stats.Inters, stats.Leak)
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
		t.Logf("%s: triangles=%d vmin=%v vmax=%v totalVol=%g lastVol=%g",
			name, stats.Triangles, stats.VMin, stats.VMax, stats.TotalVol, stats.Volume)
	}
}

// TestParityBooleanFixtureBBoxValue asserts each boolean fixture's
// triangle count and bounding box (within LENGTH_EPS) match
// booleanFixtureGoldens.
func TestParityBooleanFixtureBBoxValue(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))))
	for name, golden := range booleanFixtureGoldens {
		path := filepath.Join(root, "testdata/group", name, "normal.slvs")
		stats, err := collectBoolStats(path)
		if err != nil {
			t.Errorf("%s: load: %v", name, err)
			continue
		}
		if stats.Triangles != golden.Triangles {
			t.Errorf("%s: triangles: got %d, want %d",
				name, stats.Triangles, golden.Triangles)
		}
		for i := 0; i < 3; i++ {
			if math.Abs(stats.VMax[i]-golden.VMax[i]) > 1e-6 {
				t.Errorf("%s: vmax drift: got %v, want %v",
					name, stats.VMax, golden.VMax)
				break
			}
		}
		for i := 0; i < 3; i++ {
			if math.Abs(stats.VMin[i]-golden.VMin[i]) > 1e-6 {
				t.Errorf("%s: vmin drift: got %v, want %v",
					name, stats.VMin, golden.VMin)
				break
			}
		}
	}
}

// TestParityBooleanVolumeVsCppGolden compares each boolean fixture's
// active-group volume against the C++ reference volume from
// solvespace/test/group/<name>/test.cpp, within the per-fixture band
// in booleanFixtureCppGoldenTolerances.
func TestParityBooleanVolumeVsCppGolden(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))))
	for name, cppGolden := range booleanFixtureCppGoldens {
		path := filepath.Join(root, "testdata/group", name, "normal.slvs")
		stats, err := collectBoolStats(path)
		if err != nil {
			t.Errorf("%s: load: %v", name, err)
			continue
		}
		if cppGolden == 0 {
			continue
		}
		var ratio float64
		if stats.Volume > 0 {
			ratio = stats.Volume / cppGolden
			t.Logf("%s: go=%.4f cpp=%.4f ratio=%.6f",
				name, stats.Volume, cppGolden, ratio)
		}
		if math.Abs(ratio-1) >= cppGoldenRatioTol {
			t.Errorf("%s: ratio %.9f, want within %g of 1", name, ratio, cppGoldenRatioTol)
		}
	}
}
