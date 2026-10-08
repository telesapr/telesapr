package slvs

import (
	"math"

	"path/filepath"
	"runtime"
	"testing"

	"github.com/telesapr/telesapr/pkg/srf"
)

// boolFixtureStats is what collectBoolStats measures of a fixture.
type boolFixtureStats struct {
	Triangles int
	VMin      [3]float64
	VMax      [3]float64
	Volume    float64 // signed volume of the last (active) mesh group
	// Naked and self-intersecting edges of the last group's display
	// mesh.
	NakedEdges   int
	Inters, Leak bool
	TotalVol     float64 // sum of all mesh-group running volumes
}

// booleanFixtureGoldens holds, per fixture, the number of triangles of
// every mesh group's display mesh together, and the bounding box of the
// last group's, which is the model's exact extent.
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

// booleanFixtureCppGoldens is the volume of each fixture's last group,
// recorded while the kernel was verified against SolveSpace; changed by
// hand when the kernel changes a volume on purpose.
var booleanFixtureCppGoldens = map[string]float64{
	"boolean_coplanar_union":   13250.0,
	"boolean_knife_edge":       1.8303900357690596,
	"boolean_tangent_crossing": 205477.058604742,
	"boolean_tangent_edge":     296000.0,
	"boolean_tangent_fillet":   0.9730161212,
	"boolean_tangent_spline":   204638.6271882122,
}

// cppGoldenRatioTol is how far a volume may be from its record, as a
// ratio.
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
		// The mesh the window shows.
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
			vmax, vmin := mesh.Bounding()
			stats.VMax = [3]float64{vmax.X, vmax.Y, vmax.Z}
			stats.VMin = [3]float64{vmin.X, vmin.Y, vmin.Z}
		}
	}
	return stats, nil
}

// TestBooleanFixtures regenerates each Boolean fixture and requires
// the recorded number of triangles, bounding box and volume of its
// display mesh (booleanFixtureGoldens, booleanFixtureCppGoldens), and
// a watertight mesh with no self-intersection.
func TestBooleanFixtures(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))))
	for name, golden := range booleanFixtureGoldens {
		t.Run(name, func(t *testing.T) {
			stats, err := collectBoolStats(filepath.Join(root, "testdata/group", name, "normal.slvs"))
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if stats.Triangles != golden.Triangles {
				t.Errorf("triangles: got %d, want %d", stats.Triangles, golden.Triangles)
			}
			if stats.NakedEdges != 0 || stats.Inters || stats.Leak {
				t.Errorf("%d naked or self-intersecting edges (inters %v, leaks %v)",
					stats.NakedEdges, stats.Inters, stats.Leak)
			}
			for i := range 3 {
				if math.Abs(stats.VMax[i]-golden.VMax[i]) > 1e-6 || math.Abs(stats.VMin[i]-golden.VMin[i]) > 1e-6 {
					t.Errorf("bounding box %v to %v, want %v to %v", stats.VMin, stats.VMax, golden.VMin, golden.VMax)
					break
				}
			}
			want := booleanFixtureCppGoldens[name]
			if ratio := stats.Volume / want; math.Abs(ratio-1) >= cppGoldenRatioTol {
				t.Errorf("volume %.6f, want %.6f (ratio %.9f)", stats.Volume, want, ratio)
			}
		})
	}
}
