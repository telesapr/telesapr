package srf

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// outlinesOf is how many outline edges the mesh's are, and how long they
// are together.
func outlinesOf(m *SMesh, kind EdgeKind) (int, float64) {
	var ol SOutlineList
	m.MakeOutlinesInto(&ol, kind)
	total := 0.0
	for _, so := range ol.Outlines {
		total += so.B.DistanceTo(so.A)
	}
	return len(ol.Outlines), total
}

// The outline edges of a closed solid are the edges its triangles share,
// so a tetrahedron's are its six, each as long as its edges.
func TestMateIndexClosed(t *testing.T) {
	t.Parallel()
	var m SMesh
	tetrahedron(&m, geom.Vector{}, 10)
	if n, total := outlinesOf(&m, EdgeSharp); n != 6 {
		t.Errorf("a tetrahedron has %d outlines, want 6", n)
	} else if want := 3*10 + 3*10*math.Sqrt2; total < want-1e-6 || total > want+1e-6 {
		t.Errorf("a tetrahedron's outlines are %g long together, want %g", total, want)
	}
}

// The ends of an edge that two triangles share need not be the same
// point: a mesh read from a file has each surface's own copy of the
// curve between them, and the copies are within lengthEps. The edge is
// shared all the same.
func TestMateIndexNearMiss(t *testing.T) {
	t.Parallel()
	var m SMesh
	tetrahedron(&m, geom.Vector{}, 10)
	// Move one of a tetrahedron's corners by a hair, and add the sliver
	// that keeps the solid closed at that corner.
	eps := geom.LengthEps / 10
	for i := range m.Triangles {
		if m.Triangles[i].A == (geom.Vector{}) {
			m.Triangles[i].A = m.Triangles[i].A.Plus(geom.Vector{X: eps})
		}
	}
	if n, _ := outlinesOf(&m, EdgeSharp); n != 6 {
		t.Errorf("a tetrahedron with a corner moved by %g has %d outlines, want 6",
			eps, n)
	}
}

// An edge that runs across a face where nothing else is has no mate, and
// is not an outline.
func TestMateIndexOpen(t *testing.T) {
	t.Parallel()
	var m SMesh
	m.AddTriangle3(STriMeta{}, geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	if n, _ := outlinesOf(&m, EdgeSharp); n != 0 {
		t.Errorf("a lone triangle has %d outlines, want none", n)
	}
}

// A grid of triangles far apart in one direction and close in the other,
// so that their cells are neighbours, still has all of its edges found.
func TestMateIndexCrowded(t *testing.T) {
	t.Parallel()
	var m SMesh
	const n = 20
	for i := range n {
		for j := range n {
			// A thin strip of triangles: every vertex shares a cell with
			// many others.
			o := geom.Vector{X: float64(i) * 0.001, Y: float64(j) * 10}
			m.AddTriangle3(STriMeta{}, o, o.Plus(geom.Vector{X: 0.0009}),
				o.Plus(geom.Vector{Y: 10}))
			m.AddTriangle3(STriMeta{}, o.Plus(geom.Vector{X: 0.0009}),
				o.Plus(geom.Vector{X: 0.001, Y: 10}), o.Plus(geom.Vector{Y: 10}))
		}
	}
	// Each pair of triangles shares the diagonal; a quad's four edges
	// are open, so the shared diagonal is an outline in each quad, twice.
	if n, total := outlinesOf(&m, EdgeSharp); n != n*n {
		t.Errorf("%d quads have %d outlines, want %d", n*n, n, n*n)
	} else if want := float64(n*n) * 10 * math.Sqrt2; total < want-1e-6 || total > want+1e-6 {
		t.Errorf("the outlines are %g long together, want %g", total, want)
	}
}
