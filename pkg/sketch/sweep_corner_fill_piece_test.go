package sketch

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// box is the box from lo to hi.
func box(lo, hi geom.Vector) *srf.SShell {
	p := []geom.Vector{{X: lo.X, Y: lo.Y}, {X: lo.X, Y: hi.Y}, {X: hi.X, Y: hi.Y}, {X: hi.X, Y: lo.Y}}
	sbls := &srf.SBezierLoopSet{Normal: geom.Vector{Z: 1}, Point: p[0]}
	for i := range p {
		sbls.Outer.Curves = append(sbls.Outer.Curves, srf.SBezierFrom2(p[i], p[(i+1)%4]))
	}
	sh := &srf.SShell{}
	sh.MakeFromExtrusionOf(sbls, geom.Vector{Z: lo.Z}, geom.Vector{Z: hi.Z}, core.RGBA{})
	return sh
}

// A fill of two pieces apart, as an arc carried a quarter turn makes
// where it comes back over the other piece far from the corner, keeps
// only the piece at the corner; a fill of one piece is kept whole.
func TestNearestPiece(t *testing.T) {
	t.Parallel()
	near, far := box(geom.Vector{}, geom.Vector{X: 1, Y: 1, Z: 1}), box(geom.Vector{X: 10}, geom.Vector{X: 12, Y: 2, Z: 2})
	var both srf.SShell
	both.MakeFromUnionOf(near, far)
	if n := len(both.Pieces()); n != 2 {
		t.Fatalf("the union has %d pieces, want 2", n)
	}
	for _, c := range []struct {
		at   geom.Vector
		want float64
	}{
		{geom.Vector{X: 0.5, Y: 0.5, Z: 0}, 1},
		{geom.Vector{X: 11, Y: 1, Z: 2}, 8},
	} {
		if v := meshVolume(nearestPiece(&both, c.at)); math.Abs(v-c.want) > 1e-9 {
			t.Errorf("at %v: the piece kept has volume %g, want %g", c.at, v, c.want)
		}
	}
	if got := nearestPiece(near, geom.Vector{X: 50}); got != near {
		t.Errorf("a fill of one piece was not kept whole")
	}
}
