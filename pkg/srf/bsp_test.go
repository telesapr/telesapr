package srf

import (
	"math"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

func TestClassifyPointAgainstPlane(t *testing.T) {
	p := geom.Vector{X: 0, Y: 0, Z: 5}
	n := geom.Vector{Z: 1}
	if ClassifyPointAgainstPlane(p, n, 3) != BspPos {
		t.Fatalf("expected POS")
	}
	if ClassifyPointAgainstPlane(p, n, 7) != BspNeg {
		t.Fatalf("expected NEG")
	}
	if ClassifyPointAgainstPlane(p, n, 5) != BspCoplanar {
		t.Fatalf("expected COPLANAR")
	}
}

func TestSBsp2InsertAndIntersect(t *testing.T) {
	edge := SEdgeFrom(geom.Vector{X: -1, Y: 0}, geom.Vector{X: 1, Y: 0})
	// Start with nil root; InsertEdge creates the partitioner.
	root := (*SBsp2)(nil).InsertEdge(&edge, geom.Vector{Z: 1}, geom.Vector{X: 1})
	if root == nil {
		t.Fatalf("root should not be nil")
	}
	hit, ok := root.IntersectionWith(geom.Vector{X: 0, Y: 0, Z: -1}, geom.Vector{Z: 1})
	if !ok || math.Abs(hit.Z) > 1e-9 || math.Abs(hit.X) > 1e-9 || math.Abs(hit.Y) > 1e-9 {
		t.Fatalf("expected intersection at origin, got %v (ok=%v)", hit, ok)
	}
}

func TestSBsp3BuildFromMesh(t *testing.T) {
	m := &SMesh{}
	m.AddTriangle3(STriMeta{},
		geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	m.AddTriangle3(STriMeta{},
		geom.Vector{X: 0, Z: 1}, geom.Vector{X: 1, Z: 1}, geom.Vector{X: 1, Y: 1, Z: 1})
	bsp := SBsp3FromMesh(m)
	if bsp == nil {
		t.Fatalf("expected non-nil BSP")
	}
}

func TestSKdNodeFindClosest(t *testing.T) {
	m := &SMesh{}
	m.AddTriangle3(STriMeta{},
		geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	kd := BuildMeshKdTree(m)
	if kd == nil {
		t.Fatalf("kd-tree empty")
	}
	p := geom.Vector{X: 0.5, Y: 0.5}
	tri := FindClosestTriangle(kd, p, nil)
	if tri == nil {
		t.Fatalf("expected a triangle")
	}
	if tri.A.DistanceTo(geom.Vector{}) > 1e-9 {
		t.Fatalf("expected the first triangle to be closest")
	}
}

func TestBuildEdgeKdTreeIntersect(t *testing.T) {
	el := &SEdgeList{}
	el.AddEdge(geom.Vector{X: -10}, geom.Vector{X: 10}, 0, 0, 0)
	el.AddEdge(geom.Vector{Y: -10}, geom.Vector{Y: 10}, 0, 0, 0)
	el.AddEdge(geom.Vector{Z: -10}, geom.Vector{Z: 10}, 0, 0, 0)
	kd := BuildEdgeKdTree(el)
	if kd == nil {
		t.Fatalf("kd-tree empty")
	}
	_, edge := EdgeKdIntersect(kd, geom.Vector{X: -5, Y: -5}, geom.Vector{X: 5, Y: 5})
	if edge == nil {
		t.Fatalf("expected intersection with diagonal cross")
	}
}
