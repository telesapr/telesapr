package srf

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// A long edge with a point of another mesh 1e-4 beside its middle (a
// T-junction): the two triangles beyond end at that point, so the
// three edges along the line have no mates, and the long edge runs past
// the open edges' ends. Without the point beside it, or with nothing
// open, it does not.
func TestRunsAlongOpen(t *testing.T) {
	t.Parallel()
	v := func(x, y float64) geom.Vector { return geom.Vector{X: x, Y: y} }
	long := SMesh{Triangles: []STriangle{
		{A: v(0, 0), B: v(2, 0), C: v(1, 1)},
	}}
	mid := v(1, -1e-4)
	beyond := SMesh{Triangles: []STriangle{
		{A: mid, B: v(0, 0), C: v(1, -1)},
		{A: v(2, 0), B: mid, C: v(1, -1)},
	}}
	var all SMesh
	all.Triangles = append(append(all.Triangles, long.Triangles...), beyond.Triangles...)
	kd := SKdNodeFrom(&all)
	cnt := 0
	open := unmatedEdges(&all, kd, &cnt)
	if len(open) == 0 {
		t.Fatal("no open edges found at the T-junction")
	}
	if !runsAlongOpen(&long, open, 1e-3) {
		t.Error("the long edge runs past the open edges' ends, not found")
	}
	if runsAlongOpen(&long, open, 1e-5) {
		t.Error("the point 1e-4 beside the long edge counted within 1e-5")
	}
	if runsAlongOpen(&long, nil, 1e-3) {
		t.Error("with nothing open, an edge counted as running along it")
	}

	// Two triangles sharing their edge: every edge but the outline is
	// mated, and the outline's edges end at each other's ends.
	square := SMesh{Triangles: []STriangle{
		{A: v(0, 0), B: v(1, 0), C: v(1, 1)},
		{A: v(0, 0), B: v(1, 1), C: v(0, 1)},
	}}
	kd = SKdNodeFrom(&square)
	open = unmatedEdges(&square, kd, &cnt)
	if len(open) != 4 {
		t.Errorf("a square of two triangles has %d open edges, want its 4 sides", len(open))
	}
	if runsAlongOpen(&square, open, 1e-3) {
		t.Error("a square's edges counted as running along its own sides")
	}
}
