package srf

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

func v3(x, y, z float64) geom.Vector { return geom.Vector{X: x, Y: y, Z: z} }

func TestMeshTreeNearestBoxes(t *testing.T) {
	t.Parallel()
	a := NewMeshTree(boxMesh(v3(0, 0, 0), v3(10, 10, 10)))
	cases := []struct {
		name   string
		lo, hi geom.Vector
		want   float64
	}{
		{"face to face", v3(12, 2, 2), v3(20, 8, 8), 2},
		{"edge to edge", v3(13, 14, 0), v3(20, 20, 10), 5},
		{"corner to corner", v3(11, 12, 12), v3(20, 20, 20), 3},
		{"overlapping", v3(5, 5, 5), v3(15, 15, 15), 0},
	}
	for _, c := range cases {
		b := NewMeshTree(boxMesh(c.lo, c.hi))
		n, ok := a.Nearest(b, math.Inf(1))
		if !ok || math.Abs(n.Dist-c.want) > 1e-9 {
			t.Errorf("%s: nearest %v (%v), want %v", c.name, n.Dist, ok, c.want)
			continue
		}
		if c.want > 0 && math.Abs(n.PA.Minus(n.PB).Magnitude()-c.want) > 1e-9 {
			t.Errorf("%s: points %v %v are not %v apart", c.name, n.PA, n.PB, c.want)
		}
		if _, ok := a.Nearest(b, c.want); ok && c.want > 0 {
			t.Errorf("%s: found a pair nearer than the limit %v", c.name, c.want)
		}
	}
	if _, ok := a.Nearest(NewMeshTree(&SMesh{}), math.Inf(1)); ok {
		t.Error("an empty mesh has a nearest point")
	}
}

// TestMeshTreeNearestBrute holds the tree's nearest pair of random
// triangle soups to the least of every pair.
func TestMeshTreeNearestBrute(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewPCG(1, 2))
	soup := func(off geom.Vector, n int) *SMesh {
		m := &SMesh{}
		for range n {
			c := off.Plus(v3(r.Float64()*10, r.Float64()*10, r.Float64()*10))
			p := func() geom.Vector { return c.Plus(v3(r.Float64()*2-1, r.Float64()*2-1, r.Float64()*2-1)) }
			m.AddTriangle(STriangle{A: p(), B: p(), C: p()})
		}
		return m
	}
	for k := range 20 {
		ma, mb := soup(v3(0, 0, 0), 60), soup(v3(float64(k)/2, 3, 0), 50)
		want := math.Inf(1)
		for i := range ma.Triangles {
			for j := range mb.Triangles {
				d, _, _ := triangleDistance(&ma.Triangles[i], &mb.Triangles[j])
				want = min(want, d)
			}
		}
		n, ok := NewMeshTree(ma).Nearest(NewMeshTree(mb), math.Inf(1))
		if !ok || math.Abs(n.Dist-want) > 1e-12 {
			t.Fatalf("soup %d: tree %v, every pair %v", k, n.Dist, want)
		}
	}
}

func TestTriangleDistance(t *testing.T) {
	t.Parallel()
	tri := STriangle{A: v3(0, 0, 0), B: v3(4, 0, 0), C: v3(0, 4, 0)}
	cases := []struct {
		name string
		o    STriangle
		want float64
	}{
		{"above the inside", STriangle{A: v3(1, 1, 3), B: v3(2, 1, 5), C: v3(1, 2, 5)}, 3},
		{"through it", STriangle{A: v3(1, 1, -1), B: v3(1, 1, 1), C: v3(2, 1, 1)}, 0},
		{"edge to edge", STriangle{A: v3(3, 3, -1), B: v3(3, 3, 1), C: v3(5, 5, 0)}, math.Sqrt2},
		{"beside a vertex", STriangle{A: v3(-2, -1, 0), B: v3(-3, -1, 0), C: v3(-2, -3, 0)}, math.Sqrt(5)},
		{"of no area", STriangle{A: v3(0, -1, 0), B: v3(0, -2, 0), C: v3(0, -3, 0)}, 1},
	}
	for _, c := range cases {
		d, p, q := triangleDistance(&tri, &c.o)
		if math.Abs(d-c.want) > 1e-9 || math.Abs(p.Minus(q).Magnitude()-d) > 1e-9 {
			t.Errorf("%s: %v at %v %v, want %v", c.name, d, p, q, c.want)
		}
	}
	if q := closestOnTriangle(v3(5, 5, 5), v3(0, 0, 0), v3(1, 1, 1), v3(2, 2, 2)); math.Abs(q.Minus(v3(2, 2, 2)).Magnitude()) > 1e-12 {
		t.Errorf("nearest point of a triangle of no area %v", q)
	}
	if p, q := closestOnSegments(v3(0, 0, 0), v3(0, 0, 0), v3(1, 0, 0), v3(1, 0, 0)); p.X != 0 || q.X != 1 {
		t.Errorf("points as segments %v %v", p, q)
	}
	if p, q := closestOnSegments(v3(0, 0, 0), v3(0, 0, 0), v3(1, -1, 0), v3(1, 1, 0)); p.X != 0 || q != v3(1, 0, 0) {
		t.Errorf("point and segment %v %v", p, q)
	}
	if p, q := closestOnSegments(v3(1, -1, 0), v3(1, 1, 0), v3(0, 0, 0), v3(0, 0, 0)); p != v3(1, 0, 0) || q.X != 0 {
		t.Errorf("segment and point %v %v", p, q)
	}
	if p, q := closestOnSegments(v3(0, 0, 0), v3(1, 0, 0), v3(3, 1, 0), v3(5, 1, 0)); p != v3(1, 0, 0) || q != v3(3, 1, 0) {
		t.Errorf("parallel segments %v %v", p, q)
	}
}

func TestMeshTreeNearestPoint(t *testing.T) {
	t.Parallel()
	tr := NewMeshTree(boxMesh(v3(0, 0, 0), v3(10, 10, 10)))
	for _, c := range []struct {
		p    geom.Vector
		want float64
	}{
		{v3(5, 5, 13), 3},
		{v3(5, 5, 9), 1},
		{v3(13, 14, 5), 5},
	} {
		d, i, q := tr.NearestPoint(c.p)
		if i < 0 || math.Abs(d-c.want) > 1e-9 || math.Abs(q.Minus(c.p).Magnitude()-d) > 1e-9 {
			t.Errorf("nearest point to %v: %v at %v, want %v", c.p, d, q, c.want)
		}
	}
	// A point on an edge of the box is on the triangles of two faces.
	if in := tr.Within(v3(5, 0, 0), 1e-9); len(in) < 2 {
		t.Errorf("within an edge: %v", in)
	}
	if in := tr.Within(v3(5, 5, 20), 1); len(in) != 0 {
		t.Errorf("within 1 of a point 10 off: %v", in)
	}
	if in := NewMeshTree(&SMesh{}).Within(v3(0, 0, 0), 1); in != nil {
		t.Errorf("within an empty mesh: %v", in)
	}
	if d, i, _ := NewMeshTree(&SMesh{}).NearestPoint(v3(0, 0, 0)); i != -1 || !math.IsInf(d, 1) {
		t.Errorf("empty mesh: %v %v", d, i)
	}
}

func TestMeshTreeRay(t *testing.T) {
	t.Parallel()
	m := boxMesh(v3(0, 0, 0), v3(10, 4, 10))
	tr := NewMeshTree(m)
	if tr.Len() != len(m.Triangles) || tr.Triangle(0) != &m.Triangles[0] {
		t.Fatal("the tree does not hold the mesh's triangles")
	}
	h, ok := tr.Ray(v3(5, 0, 5), v3(0, 1, 0), 1e-9, -1)
	if !ok || math.Abs(h.T-4) > 1e-12 || h.Cos <= 0.99 {
		t.Errorf("ray across the box: %+v %v", h, ok)
	}
	h, ok = tr.Ray(v3(5, -3, 5), v3(0, 1, 0), 0, -1)
	if !ok || math.Abs(h.T-3) > 1e-12 || h.Cos >= -0.99 {
		t.Errorf("ray into the box: %+v %v", h, ok)
	}
	if _, ok := tr.Ray(v3(5, 6, 5), v3(0, 1, 0), 0, -1); ok {
		t.Error("ray away from the box hit it")
	}
	// Along a face's plane, inside the box's slab and outside it.
	if h, ok := tr.Ray(v3(-1, 2, 5), v3(1, 0, 0), 0, -1); !ok || math.Abs(h.T-1) > 1e-12 {
		t.Errorf("ray along x: %+v %v", h, ok)
	}
	if _, ok := tr.Ray(v3(-1, 2, 11), v3(1, 0, 0), 0, -1); ok {
		t.Error("ray over the box hit it")
	}
	if _, ok := NewMeshTree(&SMesh{}).Ray(v3(0, 0, 0), v3(1, 0, 0), 0, -1); ok {
		t.Error("an empty mesh was hit")
	}
	// Skipping the triangle hit first finds the next one along.
	first, _ := tr.Ray(v3(5, -3, 5), v3(0, 1, 0), 0, -1)
	if next, ok := tr.Ray(v3(5, -3, 5), v3(0, 1, 0), 0, first.Tri); !ok || next.Tri == first.Tri {
		t.Errorf("skip: %+v after %+v", next, first)
	}
}

func TestTriangulateBySurface(t *testing.T) {
	t.Parallel()
	sh := lathedTube(2, 5, 0, 3)
	var whole SMesh
	sh.TriangulateInto(&whole)
	// A second triangulation of one shell can differ in its last bits
	// (the surfaces keep their last projection), so a fresh one.
	ms := lathedTube(2, 5, 0, 3).TriangulateBySurface()
	if len(ms) != len(sh.Surface) {
		t.Fatalf("%d meshes for %d surfaces", len(ms), len(sh.Surface))
	}
	n := 0
	for _, m := range ms {
		for i := range m.Triangles {
			if m.Triangles[i] != whole.Triangles[n] {
				t.Fatalf("triangle %d differs from TriangulateInto's", n)
			}
			n++
		}
	}
	if n != len(whole.Triangles) {
		t.Errorf("%d triangles by surface, %d whole", n, len(whole.Triangles))
	}
}
