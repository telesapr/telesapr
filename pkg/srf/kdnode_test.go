package srf

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

func TestGlibcRandMatchesC(t *testing.T) {
	// glibc: srand(0); rand() x 5.
	want := []int{1804289383, 846930886, 1681692777, 1714636915, 1957747793}
	r := newGlibcRand(0)
	for i, w := range want {
		if got := r.rand(); got != w {
			t.Fatalf("rand #%d = %d, want %d", i, got, w)
		}
	}
}

// tetrahedron adds a closed, outward-facing tetrahedron at o to m.
func tetrahedron(m *SMesh, o geom.Vector, s float64) {
	a := o
	b := o.Plus(geom.Vector{X: s})
	c := o.Plus(geom.Vector{Y: s})
	d := o.Plus(geom.Vector{Z: s})
	m.AddTriangle3(STriMeta{}, a, c, b)
	m.AddTriangle3(STriMeta{}, a, b, d)
	m.AddTriangle3(STriMeta{}, a, d, c)
	m.AddTriangle3(STriMeta{}, b, c, d)
}

func certainEdges(m *SMesh) (n int, inters, leaks bool) {
	var el SEdgeList
	SKdNodeFrom(m).MakeCertainEdgesInto(&el, EdgeNakedOrSelfInter, true, &inters, &leaks, 0)
	return len(el.Edges), inters, leaks
}

func TestMakeCertainEdgesNakedAndSelfInter(t *testing.T) {
	var closed SMesh
	tetrahedron(&closed, geom.Vector{}, 1)
	if n, inters, leaks := certainEdges(&closed); n != 0 || inters || leaks {
		t.Errorf("closed tetrahedron: %d edges, inters %v, leaks %v", n, inters, leaks)
	}

	// Drop a face: its three edges are naked.
	open := SMesh{Triangles: closed.Triangles[:3]}
	if n, inters, leaks := certainEdges(&open); n != 3 || inters || !leaks {
		t.Errorf("open tetrahedron: %d edges, inters %v, leaks %v; want 3 naked", n, inters, leaks)
	}

	// Two interpenetrating tetrahedra: closed, but self-intersecting.
	var crossing SMesh
	tetrahedron(&crossing, geom.Vector{}, 1)
	tetrahedron(&crossing, geom.Vector{X: 0.2, Y: 0.2, Z: 0.2}, 1)
	if n, inters, leaks := certainEdges(&crossing); n == 0 || !inters || leaks {
		t.Errorf("crossing tetrahedra: %d edges, inters %v, leaks %v; want intersections", n, inters, leaks)
	}
}
