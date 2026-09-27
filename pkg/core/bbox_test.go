package core

import (
	"testing"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

func TestBBoxConstructionAndInclude(t *testing.T) {
	p0 := geom.Vector{X: 5, Y: -1, Z: 3}
	p1 := geom.Vector{X: -2, Y: 4, Z: 1}

	b := NewBBox(p0, p1)
	if b.MinP != (geom.Vector{X: -2, Y: -1, Z: 1}) {
		t.Fatalf("MinP unexpected: %+v", b.MinP)
	}
	if b.MaxP != (geom.Vector{X: 5, Y: 4, Z: 3}) {
		t.Fatalf("MaxP unexpected: %+v", b.MaxP)
	}
	if origin := b.GetOrigin(); origin != (geom.Vector{X: 1.5, Y: 1.5, Z: 2}) {
		t.Fatalf("GetOrigin is the centre, got %+v", origin)
	}
	if extents := b.GetExtents(); extents != (geom.Vector{X: 3.5, Y: 2.5, Z: 1}) {
		t.Fatalf("GetExtents unexpected: %+v", extents)
	}

	b.Include(geom.Vector{X: 10, Y: 2, Z: -1}, 0.5)
	if b.MinP.Z != -1.5 {
		t.Fatalf("Include should expand minimum Z: got %f", b.MinP.Z)
	}
	if b.MaxP.X != 10.5 {
		t.Fatalf("Include should expand maximum X: got %f", b.MaxP.X)
	}

	b.Include(geom.Vector{X: -5, Y: 8, Z: 4}, 1.0)
	if b.MinP.X != -6 {
		t.Fatalf("second Include should tighten minimum X: got %f", b.MinP.X)
	}
	if b.MaxP.Y != 9 {
		t.Fatalf("second Include should expand maximum Y: got %f", b.MaxP.Y)
	}
	if b.MaxP.Z != 5 {
		t.Fatalf("second Include should expand maximum Z: got %f", b.MaxP.Z)
	}
}

func TestBBoxOverlaps(t *testing.T) {
	a := NewBBox(geom.Vector{X: 0, Y: 0, Z: 0}, geom.Vector{X: 1, Y: 1, Z: 1})
	b := NewBBox(geom.Vector{X: 0.5, Y: 0.5, Z: 0.5}, geom.Vector{X: 2, Y: 2, Z: 2})
	c := NewBBox(geom.Vector{X: 2, Y: 2, Z: 2}, geom.Vector{X: 3, Y: 3, Z: 3})

	if !a.Overlaps(b) {
		t.Fatal("expected overlapping boxes to report true")
	}
	if a.Overlaps(c) {
		t.Fatal("expected disjoint boxes to report false")
	}
	if b.Overlaps(c) {
		t.Fatal("boxes that only touch don't overlap, as in C++ (b vs c)")
	}
	if !b.Contains(geom.Point2d{U: 2.1, V: 1}, 0.2) || b.Contains(geom.Point2d{U: 2.3, V: 1}, 0.2) {
		t.Fatal("Contains")
	}
}
