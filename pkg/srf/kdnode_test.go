package srf

import (
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// TestGlibcRandSequence pins the first values of glibcRand(0), glibc's
// srand(0) then rand(): the kd-tree and the BSP shuffle a mesh's
// triangles in this order, and the goldens record what follows from it.
func TestGlibcRandSequence(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

// An edge of a flat face passing a sliver of the next face, the sliver
// all but in the face's plane, crosses the sliver's plane well clear of
// it: the mesh checker took that for a crossing (NIST_MBE_PMI_5 and _6),
// measuring the miss across the edge, which nearly lies in the plane,
// rather than in the plane. An edge that does cross is still one.
func TestEdgeNearlyInSliverPlane(t *testing.T) {
	t.Parallel()
	sliver := STriangle{
		A: geom.Vector{X: 158.75, Y: 31.75, Z: 101.6},
		B: geom.Vector{X: 158.75, Y: 31.75000089985327, Z: 101.59760958843633},
		C: geom.Vector{X: 157.0382729581044, Y: 31.750000000000004, Z: 101.5229410090769},
	}
	fan := STriangle{
		A: geom.Vector{X: 155.28360385166215, Y: 31.75, Z: 101.28196726834231},
		B: geom.Vector{X: 136.525, Y: 31.75, Z: 136.525},
		C: geom.Vector{X: 153.50823600736678, Y: 31.75, Z: 100.86465015347855},
	}
	var m SMesh
	m.AddTriangle(sliver)
	m.AddTriangle(fan)
	var inter SEdgeList
	SKdNodeFrom(&m).MakeCertainEdgesInto(&inter, EdgeSelfInter, true, nil, nil, 0)
	if len(inter.Edges) != 0 {
		t.Errorf("%d edges taken to cross the sliver, want none: %v", len(inter.Edges), inter.Edges)
	}

	// A plate and a needle through its middle.
	var m2 SMesh
	m2.AddTriangle(STriangle{A: geom.Vector{}, B: geom.Vector{X: 10}, C: geom.Vector{Y: 10}})
	m2.AddTriangle(STriangle{A: geom.Vector{X: 2, Y: 2, Z: -1}, B: geom.Vector{X: 2, Y: 2, Z: 1}, C: geom.Vector{X: 2.1, Y: 2, Z: 1}})
	inter = SEdgeList{}
	SKdNodeFrom(&m2).MakeCertainEdgesInto(&inter, EdgeSelfInter, true, nil, nil, 0)
	if len(inter.Edges) == 0 {
		t.Error("a needle through a plate not taken to cross it")
	}
}

// The naked and crossing edges are looked for in chunks of triangles at
// once (badEdgesInto): the list is the one a loop over the triangles
// with FindEdgeOn makes, in its order, on a mesh of several chunks and
// a deep tree with gaps and crossings scattered through it.
func TestBadEdgesAsInOrder(t *testing.T) {
	t.Parallel()
	var m SMesh
	for i := range 20 {
		for j := range 20 {
			o := geom.Vector{X: 2 * float64(i), Y: 2 * float64(j)}
			tetrahedron(&m, o, 1)
			if (i+j)%7 == 0 {
				m.Triangles = m.Triangles[:len(m.Triangles)-1]
			}
			if (i*j)%11 == 3 {
				tetrahedron(&m, o.Plus(geom.Vector{X: 0.2, Y: 0.2, Z: 0.2}), 1)
			}
		}
	}
	if len(m.Triangles) < 3*badEdgesChunk {
		t.Fatalf("%d triangles, want several chunks", len(m.Triangles))
	}
	for _, how := range []EdgeKind{EdgeNakedOrSelfInter, EdgeSelfInter} {
		root := SKdNodeFrom(&m)
		var want SEdgeList
		var wantInter, wantLeaky bool
		var tris []*STriangle
		root.ClearTags()
		root.ListTrianglesInto(&tris)
		cnt := 1234
		for _, tr := range tris {
			for j := range 3 {
				a, b := tr.vertex(j), tr.vertex((j+1)%3)
				var info EdgeOnInfo
				root.FindEdgeOn(a, b, cnt, true, &info)
				if how == EdgeNakedOrSelfInter && info.Count != 1 {
					var back EdgeOnInfo
					root.FindEdgeOn(b, a, -cnt, true, &back)
					if info.Count != back.Count {
						want.AddEdge(a, b, 0, 0, 0)
						wantLeaky = true
					}
				}
				if info.IntersectsMesh {
					want.AddEdge(a, b, 0, 0, 0)
					wantInter = true
				}
				cnt++
			}
		}
		var got SEdgeList
		var inter, leaky bool
		root.MakeCertainEdgesInto(&got, how, true, &inter, &leaky, 0)
		if len(want.Edges) == 0 || !slices.Equal(got.Edges, want.Edges) || inter != wantInter || leaky != wantLeaky {
			t.Errorf("kind %v: %d edges (inter %v, leaky %v), want %d in order (inter %v, leaky %v)",
				how, len(got.Edges), inter, leaky, len(want.Edges), wantInter, wantLeaky)
		}
	}
}
