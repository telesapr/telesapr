package srf

import (
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// An assembly keeps where its parts' surfaces start, through a second
// assembly and a moved copy, and its display mesh is each part's mesh
// mended alone: two boxes sharing a face, as parts touch, meshed as
// one assembly, have the triangles they have meshed apart. A Boolean's
// result is one part.
func TestAssemblyParts(t *testing.T) {
	t.Parallel()
	a := extrudedBox(geom.Vector{}, geom.Vector{X: 2, Y: 2, Z: 2}, Tolerance{})
	b := extrudedBox(geom.Vector{X: 2}, geom.Vector{X: 4, Y: 2, Z: 2}, Tolerance{})
	c := extrudedBox(geom.Vector{Y: 2}, geom.Vector{X: 2, Y: 4, Z: 2}, Tolerance{})
	var ab, abc SShell
	ab.MakeFromAssemblyOf(a, b)
	abc.MakeFromAssemblyOf(&ab, c)
	n := len(a.Surface)
	if want := []int{0, n, 2 * n}; !slices.Equal(abc.parts, want) {
		t.Fatalf("parts %v, want %v", abc.parts, want)
	}
	var moved SShell
	moved.MakeFromTransformationOf(&abc, geom.Vector{X: 5}, geom.QuaternionIdentity, 1)
	if cp := abc.ExactCopy(); !slices.Equal(moved.parts, abc.parts) || !slices.Equal(cp.parts, abc.parts) {
		t.Errorf("moved %v, copied %v, want %v", moved.parts, cp.parts, abc.parts)
	}
	var whole, apart SMesh
	abc.TriangulateInto(&whole)
	for _, sh := range []*SShell{a, b, c} {
		sh.TriangulateInto(&apart)
	}
	if len(whole.Triangles) != len(apart.Triangles) {
		t.Errorf("the assembly meshed to %d triangles, its parts apart to %d", len(whole.Triangles), len(apart.Triangles))
	}
	var u SShell
	u.MakeFromUnionOf(&ab, c)
	if u.parts != nil || len(u.surfaceParts()) != 1 {
		t.Errorf("a union has parts %v", u.parts)
	}
}
