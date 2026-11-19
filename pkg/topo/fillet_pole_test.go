package topo_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// spherePolePocket is a ball of radius 20 about the origin with a pocket
// 10 by 7 - |w0| cut into its top from z = 14, turned 45° about z, its
// rim edge nearest the pole w0 from it in plan: a foot on the ball a
// little wider than |w0| passes over the pole, where the ball's four
// patches meet, crossing two of their seams from it.
func spherePolePocket(w0 float64, tol srf.Tolerance) *srf.SShell {
	s2 := math.Sqrt2 / 2
	at := func(u, w float64) geom.Vector { return v(s2*(u+w), s2*(w-u), 0) }
	box := srf.SShell{Tol: tol}
	box.MakePrism([]geom.Vector{at(-5, -7), at(5, -7), at(5, w0), at(-5, w0)}, v(0, 0, 14), v(0, 0, 25),
		core.NewRGBA(100, 100, 100, 255))
	out := srf.SShell{Tol: tol}
	out.MakeFromDifferenceOf(sphereShell(20, tol), &box)
	return &out
}

// spherePoleRim is the pick of spherePolePocket's rim edge nearest the
// pole.
func spherePoleRim(w0 float64) []topo.FilletPick {
	s2 := math.Sqrt2 / 2
	return []topo.FilletPick{{Arc: true, At: v(s2*w0, s2*w0, math.Sqrt(400-w0*w0))}}
}

// A fillet or chamfer of spherePolePocket's rim edge whose foot on the
// ball passes over the pole: the seams it crosses start at the pole, not
// at the edge, and its foot runs on to the patch the edge never touches.
// Refused before 2026-11-19 ("the ball the fillet rolls doesn't touch
// the face", then "crosses a seam of a face that doesn't reach the
// edge"): the seams are now cut from the pole (seamPole), the patches
// round a pole near the chain taken among the side's faces
// (addPoleFaces). The volumes are OpenCASCADE's, measured within a box
// about the rim (scripts/fillet-sphere-pole-fixtures.py).
func TestFilletPastPole(t *testing.T) {
	t.Parallel()
	cases := sweep.Pick([]struct {
		w0, r float64
		flat  bool
		occ   float64
	}{
		{-1, 1.5, false, 4.889958},
		{-1, 2, false, 9.075355},
		{-1, 2.5, false, 14.830878},
		{-1, 1.5, true, 11.518768},
		{-1, 2, true, 20.568160},
		{-1, 2.5, true, 32.274653},
		{-0.5, 1, false, 2.215071},
		{-0.5, 1.5, false, 5.198146},
		{-0.5, 2, false, 9.655390},
		{-0.5, 2.5, false, 15.793219},
		{-0.5, 1, true, 5.097682},
		{-0.5, 1.5, true, 11.518670},
		{-0.5, 2, true, 20.561798},
		{-0.5, 2.5, true, 32.254907},
	}, 0)
	if testing.Short() {
		// One case: each takes a few seconds.
		cases = cases[:1]
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("w0 %g r%g flat %v", c.w0, c.r, c.flat), func(t *testing.T) {
			t.Parallel()
			sh := spherePolePocket(c.w0, fine(0.01))
			before := surfaceVolume(t, sh)
			if err := topo.FilletPicks(sh, spherePoleRim(c.w0), c.r, 1<<30, c.flat); err != nil {
				t.Fatal(err)
			}
			if off, where := stepEdgesOffFaces(t, sh); off > 1e-6 {
				t.Errorf("written as STEP, an edge lies %.3g off its face: %s", off, where)
			}
			sound(t, sh)
			took := before - surfaceVolume(t, sh)
			if d := took/c.occ - 1; math.Abs(d) > 5e-4 {
				t.Errorf("took %.6f, OpenCASCADE %.6f, %+.5f", took, c.occ, d)
			}
		})
	}
}

// A fillet of spherePolePocket's rim edge whose ball's point passes
// through the pole itself is refused, as OpenCASCADE refuses it: the
// crossing is the pole, of no seam's patch.
func TestFilletThroughPoleRefused(t *testing.T) {
	if testing.Short() {
		t.Skip("a fillet of a ball's pocket through its pole, refused, slow")
	}
	t.Parallel()
	for _, c := range []struct{ w0, r float64 }{{-1, 1}, {-0.5, 0.5}} {
		t.Run(fmt.Sprintf("w0 %g r%g", c.w0, c.r), func(t *testing.T) {
			t.Parallel()
			sh := spherePolePocket(c.w0, fine(0.01))
			if err := topo.FilletPicks(sh, spherePoleRim(c.w0), c.r, 1<<30, false); err == nil {
				t.Error("made")
			}
		})
	}
}
