package srf

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// Two boxes assembled standing in each other are two pieces, each a
// box of its own; their union is one piece, the shell itself.
func TestShellPieces(t *testing.T) {
	a := extrudedBox(geom.Vector{}, geom.Vector{X: 2, Y: 2, Z: 2})
	b := extrudedBox(geom.Vector{X: 1}, geom.Vector{X: 3, Y: 2, Z: 2})
	var asm, union SShell
	asm.MakeFromAssemblyOf(a, b)
	ps := asm.Pieces()
	if len(ps) != 2 {
		t.Fatalf("%d pieces of two boxes assembled", len(ps))
	}
	for i, p := range ps {
		var m SMesh
		p.TriangulateInto(&m)
		if v := m.CalculateVolume(); math.Abs(v-8) > 1e-9 || len(p.Surface) != 6 {
			t.Errorf("piece %d: %d surfaces, volume %g", i, len(p.Surface), v)
		}
	}
	var x SShell
	x.MakeFromIntersectionOf(ps[0], ps[1])
	var m SMesh
	x.TriangulateInto(&m)
	if v := m.CalculateVolume(); x.BooleanFailed || math.Abs(v-4) > 1e-9 {
		t.Errorf("the pieces share %g, failed %v", v, x.BooleanFailed)
	}
	union.MakeFromUnionOf(a, b)
	if ps := union.Pieces(); len(ps) != 1 || ps[0] != &union {
		t.Errorf("the union is %d pieces", len(ps))
	}
}

// Two boxes assembled face to face, cut by a bar across the face they
// share and past a third box clear of it, are cut box by box (the cut
// of the whole fails).
func TestPiecewiseCut(t *testing.T) {
	low := extrudedBox(geom.Vector{}, geom.Vector{X: 10, Y: 10, Z: 10})
	high := extrudedBox(geom.Vector{Z: 10}, geom.Vector{X: 10, Y: 10, Z: 20})
	apart := extrudedBox(geom.Vector{X: 20}, geom.Vector{X: 30, Y: 10, Z: 10})
	bar := extrudedBox(geom.Vector{X: 2, Y: -1, Z: 5}, geom.Vector{X: 4, Y: 11, Z: 15})
	var two, asm SShell
	two.MakeFromAssemblyOf(low, high)
	asm.MakeFromAssemblyOf(&two, apart)
	for _, intersect := range []bool{false, true} {
		var cut SShell
		if !cut.MakeFromPiecewiseOf(&asm, bar, intersect) {
			t.Fatal("three boxes are one piece")
		}
		var m SMesh
		cut.TriangulateInto(&m)
		want := 3000.0 - 2*10*10
		if intersect {
			want = 2 * 10 * 10
		}
		if v := m.CalculateVolume(); cut.BooleanFailed || math.Abs(v-want) > 1e-6 || len(cut.Pieces()) != map[bool]int{false: 3, true: 2}[intersect] {
			t.Errorf("intersect %v: volume %g, want %g, failed %v, %d pieces", intersect, v, want, cut.BooleanFailed, len(cut.Pieces()))
		}
	}
	var one SShell
	if one.MakeFromPiecewiseOf(low, bar, false) {
		t.Error("a box is more than one piece")
	}
}
