package topo_test

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// ringGuide is a closed guide round the z axis, at angle a its point on
// the circle of radius r about where the ring's profile at that angle
// would stand at radius big, phi(a) round it from the outside, swollen
// out from its centre by swell(a).
func ringGuide(big, r float64, phi, swell func(a float64) float64) topo.LoftGuide {
	const n = 1600
	var g topo.LoftGuide
	for i := 0; i <= n; i++ {
		a := 2 * math.Pi * float64(i%n) / n
		er := v(math.Cos(a), math.Sin(a), 0)
		rr := r + swell(a)
		p := phi(a)
		g = append(g, er.ScaledBy(big+rr*math.Cos(p)).Plus(v(0, 0, rr*math.Sin(p))))
	}
	return g
}

// nearestOnSurface is how far p is from the surface: the nearest of a
// grid of its points, then narrowed.
func nearestOnSurface(bs *srf.BSplineSurface, p geom.Vector) float64 {
	best, bu, bv := math.Inf(1), 0.0, 0.0
	const nu, nv = 60, 240
	for i := 0; i <= nu; i++ {
		for j := 0; j <= nv; j++ {
			u, w := float64(i)/nu, float64(j)/nv
			if d := bs.PointAt(u, w).Minus(p).Magnitude(); d < best {
				best, bu, bv = d, u, w
			}
		}
	}
	for step := 1.0 / nu; step > 1e-9; step /= 2 {
		for _, d := range [][2]float64{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			u, w := bu+d[0]*step, bv+d[1]*step/4
			if u < 0 || u > 1 || w < 0 || w > 1 {
				continue
			}
			if dd := bs.PointAt(u, w).Minus(p).Magnitude(); dd < best {
				best, bu, bv = dd, u, w
			}
		}
	}
	return best
}

// Closed lofts round the z axis held by closed guides: a guide over the
// top of a ring of eight circles swelling up between them, two guides
// swelling the ring top and bottom, and one guide winding once round
// the profile as it goes round the ring. Each is sound, one face, its
// slope across the same either side of the seam, its volume near the
// free ring's where the guides only move the sections up, and it holds
// its guides within a thousandth of a millimetre.
func TestClosedGuidedLoft(t *testing.T) {
	t.Parallel()
	const n, big, r = 8, 20.0, 4.0
	tol := srf.Tol(0.005, 100)
	col := core.NewRGBA(100, 100, 100, 255)
	top := func(float64) float64 { return math.Pi / 2 }
	bottom := func(float64) float64 { return -math.Pi / 2 }
	bulge := func(a float64) float64 { s := math.Sin(n * a / 2); return 1.5 * s * s }
	flat := func(float64) float64 { return 0 }
	loops := ringProfiles(n, big, func(int) []srf.SBezier { return circle(r, 0, 0) })
	freeSh, err := topo.MakeClosedLoft(loops, col, 9, tol)
	if err != nil {
		t.Fatal(err)
	}
	var fm srf.SMesh
	freeSh.TriangulateInto(&fm)
	free := check.Solid(freeSh, &fm).Volume
	for _, c := range []struct {
		name   string
		guides []topo.LoftGuide
		volume float64 // within 2% of it, or 0
	}{
		{"one up", []topo.LoftGuide{ringGuide(big, r, top, bulge)}, free},
		{"two out", []topo.LoftGuide{ringGuide(big, r, top, bulge), ringGuide(big, r, bottom, bulge)}, 0},
		{"winding", []topo.LoftGuide{ringGuide(big, r, func(a float64) float64 { return math.Pi/2 + a }, flat)}, free},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			sh, err := topo.MakeClosedGuidedLoft(loops, c.guides, col, srf.OriginFor(9, 0), tol)
			if err != nil {
				t.Fatal(err)
			}
			var m srf.SMesh
			sh.TriangulateInto(&m)
			rep := check.Solid(sh, &m)
			if !rep.Sound() {
				t.Fatalf("%s", rep.String())
			}
			if c.volume != 0 && math.Abs(rep.Volume-c.volume) > 0.02*c.volume {
				t.Errorf("volume %.4f, want %.4f within 2%%", rep.Volume, c.volume)
			}
			bs, err := topo.ClosedGuidedLoftSurface(loops, c.guides)
			if err != nil {
				t.Fatal(err)
			}
			for _, u := range []float64{0.1, 0.4, 0.75} {
				_, s0 := partials(bs, u, 0, 0, 1)
				_, s1 := partials(bs, u, 1, 0, 1)
				if d := s0.Minus(s1).Magnitude(); d > 1e-4*s0.Magnitude() {
					t.Errorf("at u %g the slope across is %v at the start, %v at the end", u, s0, s1)
				}
			}
			worst := 0.0
			for _, g := range c.guides {
				for i := 0; i < len(g); i += 40 {
					worst = max(worst, nearestOnSurface(bs, g[i]))
				}
			}
			if worst > 1e-3 {
				t.Errorf("the surface is %.4g mm off its guides", worst)
			}
		})
	}
}

// Closed guides that can't hold a ring say why: a guide that doesn't
// close, one that misses a profile, and three.
func TestClosedGuidedLoftRefusals(t *testing.T) {
	t.Parallel()
	const n, big, r = 8, 20.0, 4.0
	loops := ringProfiles(n, big, func(int) []srf.SBezier { return circle(r, 0, 0) })
	top := ringGuide(big, r, func(float64) float64 { return math.Pi / 2 }, func(float64) float64 { return 0 })
	for _, c := range []struct {
		name   string
		guides []topo.LoftGuide
		want   string
	}{
		{"open", []topo.LoftGuide{top[:len(top)/2]}, "doesn't close"},
		{"missing", []topo.LoftGuide{ringGuide(big, r+1, func(float64) float64 { return math.Pi / 2 }, func(float64) float64 { return 0 })}, "first profile"},
		{"three", []topo.LoftGuide{top, top, top}, "two guides at most"},
	} {
		_, err := topo.ClosedGuidedLoftSurface(loops, c.guides)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want an error saying %q", c.name, err, c.want)
		}
	}
}
