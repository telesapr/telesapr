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

// spherePoleTaken is the volume a fillet of radius r, or a chamfer of
// distance r (flat), of spherePolePocket's rim edge takes, integrated
// from its section: the rim is a circle about the axis through the
// ball's centre square to the wall, so the ball, the wall and the
// fillet are all turned about it, the section the same in each half
// plane through it, swept round the axis between the pocket's side
// walls (|u| <= 5). The fillet's ball touches the wall's line a = w0
// and the ball's circle a² + ρ² = 400; the chamfer's line joins the
// points r from the rim's on each. Within 1e-5 of OpenCASCADE's where
// that is sound (TestFilletPastPole's table).
func spherePoleTaken(w0, r float64, flat bool) float64 {
	const big = 20.0
	rim := math.Sqrt(big*big - w0*w0)
	// The foot on the ball's circle, and the section's lower side at a.
	var footA float64
	var lower func(a float64) float64
	if flat {
		t := math.Atan2(rim, w0) - 2*math.Asin(r/(2*big))
		footA = big * math.Cos(t)
		footR := big * math.Sin(t)
		lower = func(a float64) float64 { return rim - r + (footR-rim+r)*(a-w0)/(footA-w0) }
	} else {
		ca := w0 + r
		cr := math.Sqrt((big-r)*(big-r) - ca*ca)
		footA = ca * big / (big - r)
		lower = func(a float64) float64 { return cr + math.Sqrt(math.Max(0, r*r-(a-ca)*(a-ca))) }
	}
	simpson := func(f func(float64) float64, a, b float64, n int) float64 {
		h := (b - a) / float64(n)
		s := f(a) + f(b)
		for i := 1; i < n; i++ {
			s += float64(2+2*(i%2)) * f(a+float64(i)*h)
		}
		return s * h / 3
	}
	// The ring of radius ρ about the axis between the side walls.
	ring := func(rho float64) float64 { return rho * (math.Pi - 2*math.Acos(5/rho)) }
	return simpson(func(a float64) float64 {
		lo, hi := lower(a), math.Sqrt(big*big-a*a)
		if hi <= lo {
			return 0
		}
		return simpson(ring, lo, hi, 200)
	}, w0, footA, 2000)
}

// spherePoleThrough is the w0 at which the foot of a fillet of radius r,
// or a chamfer of distance r, of spherePolePocket's rim passes through
// the pole.
func spherePoleThrough(r float64, flat bool) float64 {
	if flat {
		return -20 * math.Sin(2*math.Asin(r/40))
	}
	return -r
}

// A fillet or chamfer of spherePolePocket's rim whose foot passes
// within a speck of the pole (twice the fit's tolerance, 0.002 mm here),
// short of it, through it or past it, goes through the pole: the two
// seams it would cross a speck from it are crossed at the pole, the
// section there put on it, and the patch between them taken whole
// (nearPoles, passPole). Before 2026-11-19 a foot through the pole, or
// past it by less than the chord tolerance, was refused by name, and one
// a hair short of it came out open (the chamfer of 1 at w0 -1 passes
// 3.1e-4 short; the seams cut as before, every case short or past by
// 6e-4 or less came out open, by 0.001 or more all were sound). The
// offsets are of w0 from where the foot passes through the pole, the
// seams' crossings √2 times as far from it. OpenCASCADE refuses the
// fillets through the pole and a chamfer a hair past it, and its
// chamfers short of the pole by up to a tenth take up to 0.4% less than
// the section's integral, where those past it and further off agree with
// it within 1e-5: ours are checked against the integral
// (spherePoleTaken), within 4e-4 of it where the section was put on the
// pole and 1.2e-4 elsewhere.
func TestFilletNearPole(t *testing.T) {
	if testing.Short() {
		t.Skip("fillets and chamfers of a ball's pocket a hair from its pole, slow")
	}
	t.Parallel()
	type tc struct {
		r, off float64
		flat   bool
	}
	var cases []tc
	for _, r := range []float64{0.5, 1, 1.5} {
		for _, flat := range []bool{false, true} {
			for _, off := range []float64{-0.05, -0.008, -0.004, -0.0016, -0.0012, -0.001, -6e-4, -3e-4, -1e-5, 0,
				1e-5, 3e-4, 6e-4, 0.001, 0.0012, 0.0016, 0.004, 0.008, 0.05} {
				cases = append(cases, tc{r, off, flat})
			}
		}
	}
	// And the chamfer of 1 at w0 -1, OpenCASCADE's 5.078064 there 0.35%
	// under the integral.
	cases = append(sweep.Pick(cases, 20), tc{1, -1 - spherePoleThrough(1, true), true})
	for _, c := range cases {
		w0 := spherePoleThrough(c.r, c.flat) + c.off
		t.Run(fmt.Sprintf("r%g flat %v off %g", c.r, c.flat, c.off), func(t *testing.T) {
			t.Parallel()
			sh := spherePolePocket(w0, fine(0.01))
			before := surfaceVolume(t, sh)
			if err := topo.FilletPicks(sh, spherePoleRim(w0), c.r, 1<<30, c.flat); err != nil {
				t.Fatal(err)
			}
			sound(t, sh)
			took, want := before-surfaceVolume(t, sh), spherePoleTaken(w0, c.r, c.flat)
			if d := took/want - 1; math.Abs(d) > 5e-4 {
				t.Errorf("took %.6f, integral %.6f, %+.5f", took, want, d)
			}
		})
	}
}
