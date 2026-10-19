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
