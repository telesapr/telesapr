package srf

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// Two squares side by side, the left one in two triangles and the right
// one with a vertex in the middle of the edge they share: a T-junction,
// which SnapTJunctions mends by splitting the left one's triangle.
func TestSnapTJunctions(t *testing.T) {
	t.Parallel()
	v := func(x, y float64) geom.Vector { return geom.Vector{X: x, Y: y} }
	up := geom.Vector{Z: 1}
	tri := func(a, b, c geom.Vector) STriangle { return STriangle{A: a, B: b, C: c, NA: up, NB: up, NC: up} }
	var m SMesh
	m.AddTriangle(tri(v(0, 0), v(1, 0), v(1, 2)))
	m.AddTriangle(tri(v(0, 0), v(1, 2), v(0, 2)))
	m.AddTriangle(tri(v(1, 0), v(2, 0), v(1, 1)))
	m.AddTriangle(tri(v(2, 0), v(2, 2), v(1, 1)))
	m.AddTriangle(tri(v(1, 1), v(2, 2), v(1, 2)))
	naked := func() int {
		var el SEdgeList
		var inter, leaks bool
		SKdNodeFrom(&m).MakeCertainEdgesInto(&el, EdgeNakedOrSelfInter, false, &inter, &leaks, 0)
		n := 0
		for _, e := range el.Edges {
			if e.A.X > 0.5 && e.A.X < 1.5 && e.B.X > 0.5 && e.B.X < 1.5 {
				n++
			}
		}
		return n
	}
	if naked() == 0 {
		t.Fatal("no T-junction to mend")
	}
	m.SnapTJunctions()
	if len(m.Triangles) != 6 || naked() != 0 {
		t.Errorf("%d triangles, %d naked edges inside", len(m.Triangles), naked())
	}
	for _, tr := range m.Triangles {
		if !tr.NA.Equals(up) || !tr.NB.Equals(up) || !tr.NC.Equals(up) {
			t.Errorf("normals %v %v %v", tr.NA, tr.NB, tr.NC)
		}
	}
	// A closed mesh, or one with nothing on its naked edges, is kept.
	before := len(m.Triangles)
	m.SnapTJunctions()
	if len(m.Triangles) != before {
		t.Errorf("mended twice: %d triangles", len(m.Triangles))
	}
	if normalBetween(v(0, 0), v(1, 0), geom.Vector{}, up, v(0.5, 0)) != (geom.Vector{}) {
		t.Error("a normal between none and one")
	}
}
