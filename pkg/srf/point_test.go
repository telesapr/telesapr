package srf

import (
	"testing"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

// TestSPointBasic verifies SPoint construction.
func TestSPointBasic(t *testing.T) {
	p := SPoint{Tag: 5, P: geom.Vector{X: 1, Y: 2, Z: 3}}
	if p.Tag != 5 {
		t.Errorf("Tag = %d, want 5", p.Tag)
	}
	if p.P.X != 1 || p.P.Y != 2 || p.P.Z != 3 {
		t.Errorf("P = %v, want (1,2,3)", p.P)
	}
}

// TestSPointListIncrementTagFor verifies that points equal within
// LENGTH_EPS share a tag, as in C++.
func TestSPointListIncrementTagFor(t *testing.T) {
	var list SPointList
	list.IncrementTagFor(geom.Vector{X: 1})
	list.IncrementTagFor(geom.Vector{X: 2})
	list.IncrementTagFor(geom.Vector{X: 1 + 1e-7})
	if len(list.Points) != 2 || list.Points[0].Tag != 2 || list.Points[1].Tag != 1 {
		t.Fatalf("points %+v, want (1) tag 2 and (2) tag 1", list.Points)
	}
	if list.IndexForPoint(geom.Vector{X: 2}) != 1 || list.ContainsPoint(geom.Vector{X: 99}) {
		t.Errorf("lookup by point failed")
	}
}
