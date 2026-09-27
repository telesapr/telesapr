package srf

import (
	"math"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

// cubeMesh is the closed, outward-facing cube [o, o+s]^3 as twelve
// triangles.
func cubeMesh(o geom.Vector, s float64) *SMesh {
	p := func(x, y, z float64) geom.Vector { return o.Plus(geom.Vector{X: x * s, Y: y * s, Z: z * s}) }
	quads := [][4]geom.Vector{
		{p(0, 0, 0), p(0, 1, 0), p(1, 1, 0), p(1, 0, 0)}, // bottom, -z
		{p(0, 0, 1), p(1, 0, 1), p(1, 1, 1), p(0, 1, 1)}, // top, +z
		{p(0, 0, 0), p(1, 0, 0), p(1, 0, 1), p(0, 0, 1)}, // -y
		{p(0, 1, 0), p(0, 1, 1), p(1, 1, 1), p(1, 1, 0)}, // +y
		{p(0, 0, 0), p(0, 0, 1), p(0, 1, 1), p(0, 1, 0)}, // -x
		{p(1, 0, 0), p(1, 1, 0), p(1, 1, 1), p(1, 0, 1)}, // +x
	}
	m := &SMesh{}
	for _, q := range quads {
		m.AddTriangle3(STriMeta{}, q[0], q[1], q[2])
		m.AddTriangle3(STriMeta{}, q[0], q[2], q[3])
	}
	return m
}

// TestMeshBooleansOfCubes combines two overlapping cubes with the BSP
// mesh Booleans. After SnapToMesh, as the forced-to-mesh combine does
// it, each result must have the exact volume and be watertight.
func TestMeshBooleansOfCubes(t *testing.T) {
	for _, tc := range []struct {
		name string
		op   func(m, a, b *SMesh)
		vol  float64
	}{
		{"union", (*SMesh).MakeFromUnionOf, 15},
		{"difference", (*SMesh).MakeFromDifferenceOf, 7},
		{"intersection", (*SMesh).MakeFromIntersectionOf, 1},
	} {
		a := cubeMesh(geom.Vector{}, 2)
		b := cubeMesh(geom.Vector{X: 1, Y: 1, Z: 1}, 2)
		var out SMesh
		tc.op(&out, a, b)
		out.RemoveDegenerateTriangles()

		root := SKdNodeFrom(&out)
		root.SnapToMesh(&out)
		var snapped SMesh
		root.MakeMeshInto(&snapped)

		if v := snapped.CalculateVolume(); math.Abs(v-tc.vol) > 1e-9 {
			t.Errorf("%s: volume %.12f, want %g", tc.name, v, tc.vol)
		}
		if n, inters, leaks := certainEdges(&snapped); n != 0 || inters || leaks {
			t.Errorf("%s: %d naked or self-intersecting edges (inters %v, leaks %v)", tc.name, n, inters, leaks)
		}
	}
}

// TestMeshBooleansOfDisjointCubes: a union of disjoint cubes keeps
// every triangle of both, and a difference keeps the first cube.
func TestMeshBooleansOfDisjointCubes(t *testing.T) {
	a := cubeMesh(geom.Vector{}, 1)
	b := cubeMesh(geom.Vector{X: 5, Y: 5, Z: 5}, 1)

	var union SMesh
	union.MakeFromUnionOf(a, b)
	if len(union.Triangles) != 24 || math.Abs(union.CalculateVolume()-2) > 1e-12 {
		t.Errorf("union: %d triangles, volume %g; want 24, 2", len(union.Triangles), union.CalculateVolume())
	}

	var diff SMesh
	diff.MakeFromDifferenceOf(a, b)
	if len(diff.Triangles) != 12 || math.Abs(diff.CalculateVolume()-1) > 1e-12 {
		t.Errorf("difference: %d triangles, volume %g; want 12, 1", len(diff.Triangles), diff.CalculateVolume())
	}
}
