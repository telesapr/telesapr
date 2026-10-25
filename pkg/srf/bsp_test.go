package srf

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

func TestSBsp3BuildFromMesh(t *testing.T) {
	t.Parallel()
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

// TestSelfIntersectingPolygon checks the edge kd-tree's crossing count
// through SPolygon.SelfIntersecting: a bowtie crosses itself, a square
// doesn't, in any plane.
func TestSelfIntersectingPolygon(t *testing.T) {
	t.Parallel()
	square := []geom.Vector{{}, {X: 10}, {X: 10, Y: 10}, {Y: 10}}
	bowtie := []geom.Vector{{}, {X: 10, Y: 10}, {X: 10}, {Y: 10}}
	// The same in the yz plane, where a test on x and y alone sees
	// nothing.
	yz := func(pts []geom.Vector) []geom.Vector {
		out := make([]geom.Vector, len(pts))
		for i, p := range pts {
			out[i] = geom.Vector{Y: p.X, Z: p.Y}
		}
		return out
	}
	for _, tc := range []struct {
		name string
		pts  []geom.Vector
		want bool
	}{
		{"square", square, false},
		{"bowtie", bowtie, true},
		{"square yz", yz(square), false},
		{"bowtie yz", yz(bowtie), true},
	} {
		var p SPolygon
		p.AddEmptyContour()
		c := &p.Contours[0]
		for _, pt := range tc.pts {
			c.AddPoint(pt)
		}
		c.AddPoint(tc.pts[0])
		var at geom.Vector
		if got := p.SelfIntersecting(&at); got != tc.want {
			t.Errorf("%s: self-intersecting %v, want %v", tc.name, got, tc.want)
		} else if got {
			// The bowtie's diagonals cross at its centre.
			want := tc.pts[0].Plus(tc.pts[1]).ScaledBy(0.5)
			if !at.Equals(want) {
				t.Errorf("%s: crossing at %v, want %v", tc.name, at, want)
			}
		}
	}
}

// TestSBsp3InsertHowCoplanar tests InsertHow with coplanar classification.
func TestSBsp3InsertHowCoplanar(t *testing.T) {
	t.Parallel()
	tri := STriangle{A: geom.Vector{}, B: geom.Vector{X: 1}, C: geom.Vector{Y: 1}}
	root := &SBsp3{N: geom.Vector{Z: 1}, D: 0, Tri: tri}
	root.InsertHow(BspCoplanar, &tri, nil)
	if root.More == nil {
		t.Errorf("InsertHow Coplanar should chain More")
	}
}

// TestSBsp3InsertHowPos tests InsertHow with positive classification.
func TestSBsp3InsertHowPos(t *testing.T) {
	t.Parallel()
	tri := STriangle{A: geom.Vector{Z: 1}, B: geom.Vector{X: 1, Z: 1}, C: geom.Vector{Y: 1, Z: 1}}
	root := &SBsp3{N: geom.Vector{Z: 1}, D: 0, Tri: tri}
	root.InsertHow(BspPos, &tri, nil)
	if root.Pos == nil {
		t.Errorf("InsertHow Pos should create Pos subtree")
	}
}

// TestSBsp3InsertHowNeg tests InsertHow with negative classification.
func TestSBsp3InsertHowNeg(t *testing.T) {
	t.Parallel()
	tri := STriangle{A: geom.Vector{Z: -1}, B: geom.Vector{X: 1, Z: -1}, C: geom.Vector{Y: 1, Z: -1}}
	root := &SBsp3{N: geom.Vector{Z: 1}, D: 0, Tri: tri}
	root.InsertHow(BspNeg, &tri, nil)
	if root.Neg == nil {
		t.Errorf("InsertHow Neg should create Neg subtree")
	}
}
