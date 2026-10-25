package srf

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// capArea is the area the cap fills with the odd winding rule.
func capArea(p *SPolygon) float64 {
	var c SPolygon
	c.Normal = p.Normal
	for _, sc := range p.Contours {
		c.Contours = append(c.Contours, SContour{Points: append([]SPoint(nil), sc.Points...)})
	}
	c.FixContourDirections()
	return math.Abs(c.SignedArea())
}

func plane(n geom.Vector, through geom.Vector) HalfSpace {
	n = n.WithMagnitude(1)
	return HalfSpace{N: n, D: n.Dot(through)}
}

func TestMeshSection(t *testing.T) {
	t.Parallel()
	cube := boxMesh(geom.Vector{}, geom.Vector{X: 10, Y: 10, Z: 10})
	var holed SMesh
	holed.MakeFromDifferenceOf(cube, boxMesh(geom.Vector{X: 3, Y: 3, Z: -1}, geom.Vector{X: 7, Y: 7, Z: 11}))

	for _, tc := range []struct {
		name  string
		mesh  *SMesh
		keep  HalfSpace
		loops int
		area  float64
	}{
		{"level", cube, plane(geom.Vector{Z: 1}, geom.Vector{Z: 4}), 1, 100},
		{"flipped", cube, plane(geom.Vector{Z: -1}, geom.Vector{Z: 4}), 1, 100},
		{"oblique", cube, plane(geom.Vector{X: 1, Y: 1}, geom.Vector{X: 5, Y: 5}), 1, 100 * math.Sqrt2},
		{"corner", cube, plane(geom.Vector{X: 1, Y: 1, Z: 1}, geom.Vector{X: 10, Y: 10, Z: 5}), 1,
			// A plane cutting off a corner: a triangle of side 5*sqrt2.
			math.Sqrt(3) / 4 * 50},
		{"missing", cube, plane(geom.Vector{Z: 1}, geom.Vector{Z: 20}), 0, 0},
		{"hole", &holed, plane(geom.Vector{Z: 1}, geom.Vector{Z: 5}), 2, 100 - 16},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var p SPolygon
			closed := tc.mesh.MakeSectionPolygonInto(tc.keep, &p)
			if !closed {
				t.Error("a loop is open")
			}
			if len(p.Contours) != tc.loops {
				t.Fatalf("%d loops, want %d", len(p.Contours), tc.loops)
			}
			if !p.Normal.Equals(tc.keep.N) {
				t.Errorf("normal %v, want %v", p.Normal, tc.keep.N)
			}
			if a := capArea(&p); math.Abs(a-tc.area) > 1e-6 {
				t.Errorf("area %v, want %v", a, tc.area)
			}
			for _, sc := range p.Contours {
				for _, sp := range sc.Points {
					if d := tc.keep.N.Dot(sp.P) - tc.keep.D; math.Abs(d) > 1e-9 {
						t.Fatalf("cap point %v off the plane by %v", sp.P, d)
					}
				}
			}
		})
	}
}

// Vertices on the plane that each triangle places a few ulps to one
// side or the other, as the meshes of a turned surface's quarters do
// along a seam in a plane through its axis, are taken as on it: the
// loop closes and lies in the plane.
func TestMeshSectionSeamOffPlane(t *testing.T) {
	t.Parallel()
	m := boxMesh(geom.Vector{}, geom.Vector{X: 10, Y: 10, Z: 10})
	for i := range m.Triangles {
		tr := &m.Triangles[i]
		for k, p := range []*geom.Vector{&tr.A, &tr.B, &tr.C} {
			if p.Z == 0 {
				p.Z = float64(1-2*((i+k)%2)) * 1e-12
			}
		}
	}
	var p SPolygon
	keep := plane(geom.Vector{Z: 1}, geom.Vector{})
	if !m.MakeSectionPolygonInto(keep, &p) || len(p.Contours) != 1 {
		t.Fatalf("%d contours, closed: want the bottom face's one", len(p.Contours))
	}
	if a := capArea(&p); math.Abs(a-100) > 1e-6 {
		t.Errorf("area %v, want 100", a)
	}
	for _, sp := range p.Contours[0].Points {
		if sp.P.Z != 0 {
			t.Fatalf("cap point %v off the plane", sp.P)
		}
	}
}

// A mesh with a hole gives an open loop, and says so.
func TestMeshSectionLeaky(t *testing.T) {
	t.Parallel()
	m := boxMesh(geom.Vector{}, geom.Vector{X: 10, Y: 10, Z: 10})
	// Drop a triangle of the -y side.
	m.Triangles = append(m.Triangles[:4], m.Triangles[5:]...)
	var p SPolygon
	if m.MakeSectionPolygonInto(plane(geom.Vector{Z: 1}, geom.Vector{Z: 5}), &p) {
		t.Error("a leaky mesh gave closed loops")
	}
	if len(p.Contours) != 1 {
		t.Errorf("%d contours, want the one open", len(p.Contours))
	}
}

// The assembler finds ends that meet within LengthEps but not exactly.
func TestAssembleLoopsNearEnds(t *testing.T) {
	t.Parallel()
	var el SEdgeList
	pts := []geom.Vector{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}
	for i := range pts {
		a, b := pts[i], pts[(i+1)%len(pts)]
		if i == 1 {
			a.X += geom.LengthEps / 10
		}
		el.AddEdge(a, b, 0, 0, 0)
	}
	var p SPolygon
	if !assembleLoops(&el, &p) || len(p.Contours) != 1 || len(p.Contours[0].Points) != 5 {
		t.Errorf("assembled %+v", p.Contours)
	}
}

// A face cut away by the section isn't hit.
func TestFirstIntersectionIn(t *testing.T) {
	t.Parallel()
	m := boxMesh(geom.Vector{}, geom.Vector{X: 10, Y: 10, Z: 10})
	for i := range m.Triangles {
		m.Triangles[i].Meta.Face = FaceID(i/2 + 1)
	}
	// The ray goes from the eye into the screen, and the hit is the
	// face it leaves the cube through, the bottom: the view's rays run
	// that way round (see GraphicsWindow.HitTestMakeSelection).
	from, dir := geom.Vector{X: 5, Y: 5, Z: 20}, geom.Vector{Z: -1}
	bottom := m.FirstIntersectionWith(from, dir)
	if bottom != 1 {
		t.Fatalf("face %d, want the bottom, 1", bottom)
	}
	if f := m.FirstIntersectionIn(from, dir, &HalfSpace{N: geom.Vector{Z: 1}, D: 5}); f != bottom {
		t.Errorf("face %d with the top half cut away, want %d", f, bottom)
	}
	if f := m.FirstIntersectionIn(from, dir, &HalfSpace{N: geom.Vector{Z: -1}, D: -5}); f != 0 {
		t.Errorf("face %d with the bottom half cut away, want none", f)
	}
}
