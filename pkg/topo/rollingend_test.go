package topo_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// prism is the 10 x 20 profile at z = 0 moved along by: a box, or a
// leaning prism whose side edges meet its ends at a slant, built at tol.
func prism(by geom.Vector, tol srf.Tolerance) *srf.SShell {
	sh := srf.SShell{Tol: tol}
	sh.MakePrism([]geom.Vector{v(0, 0, 0), v(10, 0, 0), v(10, 20, 0), v(0, 20, 0)}, v(0, 0, 0), by,
		core.NewRGBA(100, 100, 100, 255))
	return &sh
}

// The rolling ball along an open edge, ending at a flat face at each
// end, on edges whose results are known: a box's side edge, square to
// its ends, and a leaning prism's, whose ends cut the fillet in
// ellipses. A fillet of radius r takes r²(1 - π/4) across the edge, a
// chamfer at distance r takes r²/2, along the edge's whole length.
func TestRollingBallOpenEdge(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		by   geom.Vector
		flat bool
	}{
		{v(0, 0, 30), false}, {v(0, 0, 30), true},
		{v(0, 6, 30), false}, {v(0, 6, 30), true},
		{v(8, 6, 30), false}, {v(8, 6, 30), true},
	} {
		const r = 2.0
		sh := prism(c.by, fine(0.002))
		before := meshVolume(sh)
		a := v(10, 0, 0)
		if err := topo.RollingBallAlong(sh, a, a.Plus(c.by), r, c.flat); err != nil {
			t.Errorf("along %v flat %t: %v", c.by, c.flat, err)
			continue
		}
		// The angle between the faces, square to the edge: each face's
		// way from the edge, the profile's sides, less its part along it.
		dir := c.by.WithMagnitude(1)
		ua, ub := v(-1, 0, 0), v(0, 1, 0)
		ua = ua.Minus(dir.ScaledBy(ua.Dot(dir))).WithMagnitude(1)
		ub = ub.Minus(dir.ScaledBy(ub.Dot(dir))).WithMagnitude(1)
		phi := math.Acos(ua.Dot(ub))
		across := r * r * (1/math.Tan(phi/2) - (math.Pi-phi)/2)
		if c.flat {
			across = r * r * math.Sin(phi) / 2
		}
		want := across * c.by.Magnitude()
		got := before - sound(t, sh)
		// The mesh's chords cut into the round, by their sag over its arc.
		allow := 1e-6 * want
		if !c.flat {
			allow = 2.0 / 3 * 0.002 * (math.Pi - phi) * r * c.by.Magnitude()
		}
		t.Logf("along %v flat %t: took %.6g, want %.6g", c.by, c.flat, got, want)
		if math.Abs(got-want) > allow {
			t.Errorf("along %v flat %t: took %g, want %g within %.3g", c.by, c.flat, got, want, allow)
		}
	}
}
