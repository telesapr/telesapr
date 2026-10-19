package sketch

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// boxPart is a part of the boxes from lo to hi, assembled.
func boxPart(boxes ...[2]geom.Vector) *part {
	var sh srf.SShell
	for _, b := range boxes {
		lo, hi := b[0], b[1]
		var one, both srf.SShell
		one.MakePrism([]geom.Vector{{X: lo.X, Y: lo.Y}, {X: hi.X, Y: lo.Y}, {X: hi.X, Y: hi.Y}, {X: lo.X, Y: hi.Y}},
			geom.Vector{Z: lo.Z}, geom.Vector{Z: hi.Z}, core.RGBA{})
		both.MakeFromAssemblyOf(&sh, &one)
		sh = both
	}
	p := &part{shell: &sh, whole: newPiece(&sh)}
	p.lo, p.hi = shellBox(&sh)
	return p
}

// A part of several pieces is checked piece by piece: a block reaching both blocks of an assembly that stand in each
// other shares the sum of what it shares with each, which counts their
// common part twice and is approximate; one reaching one block alone
// shares what it shares with that, exactly.
func TestOverlapByPieces(t *testing.T) {
	t.Parallel()
	asm := boxPart([2]geom.Vector{{}, {X: 10, Y: 10, Z: 10}}, [2]geom.Vector{{X: 5}, {X: 15, Y: 10, Z: 10}})
	if n := len(asm.piecesOf()); n != 2 {
		t.Fatalf("%d pieces", n)
	}
	for _, c := range []struct {
		x0, x1, want float64
		approximate  bool
	}{
		{2, 6, 300, true},
		{12, 14, 120, false},
	} {
		b := boxPart([2]geom.Vector{{X: c.x0, Y: 2}, {X: c.x1, Y: 8, Z: 10}})
		f := asm.overlapByPieces(b, Interference{})
		if f.Failed || f.Approximate != c.approximate || math.Abs(f.Volume-c.want) > 1e-6 {
			t.Errorf("a block from x %g to %g: %g mm³, approximate %v, failed %v; want %g", c.x0, c.x1,
				f.Volume, f.Approximate, f.Failed, c.want)
		}
	}
}
