package slvs

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
)

// TestParityAnalysisFixtures asserts that every analysis_* fixture
// loads and solves successfully. This is the structural parity gate
// for the analysis fixture family (Phase E).
func TestParityAnalysisFixtures(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))))
	analysisRoot := filepath.Join(root, "testdata/analysis")
	entries, err := os.ReadDir(analysisRoot)
	if err != nil {
		t.Fatalf("readDir: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := entry.Name()
		path := filepath.Join(analysisRoot, dir, "normal.slvs")
		doc, err := LoadFile(path)
		if err != nil {
			t.Errorf("%s: load: %v", dir, err)
			continue
		}
		doc.Sketch.GenerateAll()
		for i, g := range doc.Sketch.Groups {
			if !g.Clean {
				t.Errorf("%s group %d: not clean", dir, i)
			}
			if g.Solved.How != core.SolveOkay && g.Solved.How != core.SolveRedundantOkay {
				t.Errorf("%s group %d: solve failed: How=%d", dir, i, g.Solved.How)
			}
		}
	}
}

// ContourAreaSignAndMagnitude returns the signed area of the
// closed contour formed by the line-segment entities in the
// fixture. Uses the shoelace formula on the line endpoints in
// order. A non-zero result confirms that the contour forms a
// closed polygon with non-trivial area.
func TestContourAreaSignAndMagnitude(t *testing.T) {
	doc, err := LoadFile("/home/msorc/projects/spacesolve/testdata/analysis/contour_area/normal.slvs")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	doc.Sketch.GenerateAll()
	// Collect line segments that belong to the workplane group.
	// Their endpoints come from POINT_IN_2D entities with non-zero
	// ActPoint values.
	var pts []struct{ X, Y float64 }
	seen := make(map[core.HEntity]bool)
	for _, e := range doc.Sketch.Entities {
		if e.Type != 11000 { // LINE_SEGMENT
			continue
		}
		for _, pe := range []*core.HEntity{&e.Point[0], &e.Point[1]} {
			if seen[*pe] {
				continue
			}
			seen[*pe] = true
			for _, pe2 := range doc.Sketch.Entities {
				if pe2.H == *pe && (pe2.Type == 2001 || pe2.Type == 2000) {
					pts = append(pts, struct{ X, Y float64 }{pe2.ActPoint.X, pe2.ActPoint.Y})
					break
				}
			}
		}
	}
	if len(pts) < 3 {
		t.Skipf("not enough points: %d", len(pts))
	}
	// Deduplicate by (X, Y) — request.Generate creates duplicate
	// entities for the same handle, so without dedup the shoelace
	// formula walks each edge twice.
	uniquePts := dedupPts(pts)
	if len(uniquePts) < 3 {
		t.Skipf("not enough unique points: %d", len(uniquePts))
	}
	// Order the unique points by walking the line segments.
	ordered, ok := orderLineSegments(doc, uniquePts)
	if !ok || len(ordered) < 3 {
		t.Skipf("could not order contour (have %d points)", len(ordered))
	}
	area := shoelace(ordered)
	if area == 0 {
		t.Errorf("contour area is zero; expected non-zero")
	}
	// The fixture is a 20x20 square centered at origin. After
	// deduplication the shoelace formula should give exactly 400.
	const expected = 400.0
	got := absF(area)
	if got < expected-1 || got > expected+1 {
		t.Errorf("contour area: got %v, want %v (within 1.0)", got, expected)
	}
	t.Logf("contour area magnitude: %v (signed: %v, unique points: %d)",
		got, area, len(ordered))
}

func dedupPts(pts []struct{ X, Y float64 }) []struct{ X, Y float64 } {
	seen := make(map[[2]uint64]bool)
	var out []struct{ X, Y float64 }
	for _, p := range pts {
		key := [2]uint64{mathFloatBits(p.X), mathFloatBits(p.Y)}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, p)
	}
	return out
}

// mathFloatBits returns the bit pattern of f as a uint64. Two
// floats that compare equal also have equal bit patterns.
func mathFloatBits(f float64) uint64 {
	return mathFloat64bits(f)
}

func shoelace(pts []struct{ X, Y float64 }) float64 {
	if len(pts) < 3 {
		return 0
	}
	var sum float64
	for i := range pts {
		j := (i + 1) % len(pts)
		sum += pts[i].X*pts[j].Y - pts[j].X*pts[i].Y
	}
	return sum / 2
}

func absF(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// orderLineSegments walks the line segments to form an ordered
// contour. It uses the heuristic that segments share endpoints
// (start of one matches end of another).
func orderLineSegments(doc *Document, pts []struct{ X, Y float64 }) ([]struct{ X, Y float64 }, bool) {
	// Build edges: pairs of unique indices.
	type edge struct{ A, B int }
	var edges []edge
	for _, e := range doc.Sketch.Entities {
		if e.Type != 11000 {
			continue
		}
		a := pointIndex(pts, doc, e.Point[0])
		b := pointIndex(pts, doc, e.Point[1])
		if a < 0 || b < 0 || a == b {
			continue
		}
		edges = append(edges, edge{a, b})
	}
	if len(edges) == 0 {
		return nil, false
	}
	// Build adjacency.
	adj := make(map[int][]int)
	for _, e := range edges {
		adj[e.A] = append(adj[e.A], e.B)
		adj[e.B] = append(adj[e.B], e.A)
	}
	// Walk from the first edge.
	start := edges[0].A
	ordered := []struct{ X, Y float64 }{pts[start]}
	prev := -1
	cur := start
	for len(ordered) < len(pts) {
		nexts := adj[cur]
		var next int = -1
		for _, n := range nexts {
			if n != prev {
				next = n
				break
			}
		}
		if next < 0 {
			break
		}
		ordered = append(ordered, pts[next])
		prev = cur
		cur = next
	}
	return ordered, true
}

func pointIndex(pts []struct{ X, Y float64 }, doc *Document, h core.HEntity) int {
	for _, e := range doc.Sketch.Entities {
		if e.H == h && (e.Type == 2001 || e.Type == 2000) {
			for i, p := range pts {
				if p.X == e.ActPoint.X && p.Y == e.ActPoint.Y {
					return i
				}
			}
		}
	}
	return -1
}

func mathFloat64bits(f float64) uint64 {
	return math.Float64bits(f)
}
