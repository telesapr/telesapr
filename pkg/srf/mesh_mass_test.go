package srf

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// A cube far from the origin has its closed-form volume, centre and
// inertia, s⁵/6 about each axis through the centre and no products:
// the moments are taken about the box's middle, so its distance from
// the origin costs nothing.
func TestMeshMassCube(t *testing.T) {
	t.Parallel()
	o := geom.Vector{X: 1e4, Y: -3e3, Z: 250}
	const s = 2.0
	mm := boxMesh(o, o.Plus(geom.Vector{X: s, Y: s, Z: s})).Mass()
	if math.Abs(mm.Volume-s*s*s) > 1e-9 {
		t.Errorf("volume %g, want %g", mm.Volume, s*s*s)
	}
	if c := o.Plus(geom.Vector{X: 1, Y: 1, Z: 1}); mm.CenterOfMass.Minus(c).Magnitude() > 1e-9 {
		t.Errorf("centre %v, want %v", mm.CenterOfMass, c)
	}
	want := math.Pow(s, 5) / 6
	for r := range 3 {
		for q := range 3 {
			w := 0.0
			if r == q {
				w = want
			}
			if math.Abs(mm.Inertia[r][q]-w) > 1e-9 {
				t.Errorf("inertia[%d][%d] %g, want %g", r, q, mm.Inertia[r][q], w)
			}
		}
	}
}

// An empty mesh, and one that encloses nothing, have no mass.
func TestMeshMassEmpty(t *testing.T) {
	t.Parallel()
	if mm := (&SMesh{}).Mass(); mm != (MeshMass{}) {
		t.Errorf("empty mesh: %+v", mm)
	}
	m := &SMesh{}
	a, b, c := geom.Vector{X: 1}, geom.Vector{Y: 1}, geom.Vector{Z: 1}
	m.AddTriangle3(STriMeta{}, a, b, c)
	m.AddTriangle3(STriMeta{}, a, c, b)
	if mm := m.Mass(); mm.Volume != 0 || mm.CenterOfMass != (geom.Vector{}) {
		t.Errorf("a triangle both ways: %+v", mm)
	}
}
