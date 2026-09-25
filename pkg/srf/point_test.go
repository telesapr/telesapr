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

// TestSPointListAddTag verifies tag allocation.
func TestSPointListAddTag(t *testing.T) {
	list := SPointList{}
	tag1 := list.AddTag()
	tag2 := list.AddTag()
	if tag1 == tag2 {
		t.Errorf("consecutive tags should differ, got %d, %d", tag1, tag2)
	}
}

// TestSPointListClear verifies Clear resets length.
func TestSPointListClear(t *testing.T) {
	list := SPointList{}
	list.AddTag()
	list.AddTag()
	list.Clear()
	if list.Len() != 0 {
		t.Errorf("Len after Clear = %d, want 0", list.Len())
	}
}

// TestSPointListIndexOf verifies IndexOf lookup.
func TestSPointListIndexOf(t *testing.T) {
	list := SPointList{}
	list.Points = append(list.Points, SPoint{P: geom.Vector{X: 1}})
	list.Points = append(list.Points, SPoint{P: geom.Vector{X: 2}})
	idx := list.IndexOf(geom.Vector{X: 2})
	if idx != 1 {
		t.Errorf("IndexOf({X:2}) = %d, want 1", idx)
	}
	idx = list.IndexOf(geom.Vector{X: 99})
	if idx >= 0 {
		t.Errorf("IndexOf(missing) = %d, want <0", idx)
	}
}

// TestSPointListContains verifies Contains.
func TestSPointListContains(t *testing.T) {
	list := SPointList{}
	list.Points = append(list.Points, SPoint{P: geom.Vector{X: 1}})
	if !list.Contains(0) {
		t.Errorf("Contains(0) should be true after Add")
	}
	if list.Contains(5) {
		t.Errorf("Contains(5) should be false (out of range)")
	}
}
