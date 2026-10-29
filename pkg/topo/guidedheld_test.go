package topo_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// heldVolume is the volume of sh, which must be sound.
func heldVolume(t *testing.T, sh *srf.SShell) float64 {
	t.Helper()
	vol, err := soundVolume(sh)
	if err != nil {
		t.Fatal(err)
	}
	return vol
}

// nearestOnPieces is how far p is from the nearest of the surfaces.
func nearestOnPieces(bss []*srf.BSplineSurface, p geom.Vector) float64 {
	best := math.Inf(1)
	for _, bs := range bss {
		best = min(best, nearestOnSurface(bs, p))
	}
	return best
}

// Closed lofts held by guides and square at once: a ring of eight
// circles under a guide over their tops swelling up between them, or
// two swelling it top and bottom, held square at one profile, at two and
// at every one. Each is sound, of a piece from each held
// profile round to the next, leaving and reaching a held profile along
// the way round there, holding its guides within a thousandth of a
// millimetre, and its volume near the free ring's where the guide only
// moves the sections up. A guide winding round the profile crosses the
// held profiles slanting: the ring is still sound and near its guide.
func TestClosedGuidedLoftHeld(t *testing.T) {
	t.Parallel()
	const n, big, r = 8, 20.0, 4.0
	tol := srf.Tol(0.005, 100)
	col := core.NewRGBA(100, 100, 100, 255)
	top := func(float64) float64 { return math.Pi / 2 }
	bottom := func(float64) float64 { return -math.Pi / 2 }
	bulge := func(a float64) float64 { s := math.Sin(n * a / 2); return 1.5 * s * s }
	flat := func(float64) float64 { return 0 }
	loops := ringProfiles(n, big, func(int) []srf.SBezier { return circle(r, 0, 0) })
	free, err := topo.MakeClosedLoft(loops, col, 9, tol)
	if err != nil {
		t.Fatal(err)
	}
	freeVol := heldVolume(t, free)
	up := []topo.LoftGuide{ringGuide(big, r, top, bulge)}
	out := []topo.LoftGuide{ringGuide(big, r, top, bulge), ringGuide(big, r, bottom, bulge)}
	winding := []topo.LoftGuide{ringGuide(big, r, func(a float64) float64 { return math.Pi/2 + a }, flat)}
	for _, c := range []struct {
		name   string
		guides []topo.LoftGuide
		held   []float64
		volume float64 // within 2% of it, or 0
		off    float64 // how far off its guides it may be
	}{
		{"one up, one held", up, []float64{0, 0, 1}, freeVol, 1e-3},
		{"one up, two held", up, []float64{0, 0.8, 0, 0, 0, 1.2}, freeVol, 1e-3},
		{"one up, the seam's held", up, []float64{1}, freeVol, 1e-3},
		{"two out, every one held", out, []float64{1, 1, 1, 1, 1, 1, 1, 1}, 0, 1e-3},
		{"winding, two held", winding, []float64{0, 1, 0, 0, 0, 1}, freeVol, 0.05},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			bss, err := topo.ClosedGuidedLoftSurfaces(loops, c.held, c.guides)
			if err != nil {
				t.Fatal(err)
			}
			var at []int
			for j, x := range c.held {
				if x != 0 {
					at = append(at, j)
				}
			}
			if len(bss) != len(at) {
				t.Fatalf("%d pieces, want %d", len(bss), len(at))
			}
			for p, bs := range bss {
				a := 2 * math.Pi * float64(at[p]) / n
				way := v(-math.Sin(a), math.Cos(a), 0)
				prev := bss[(p+len(bss)-1)%len(bss)]
				for _, u := range []float64{0.1, 0.4, 0.75} {
					if _, s := partials(bs, u, 0, 0, 1); s.WithMagnitude(1).Minus(way).Magnitude() > 1e-4 {
						t.Errorf("piece %d at u %g leaves along %v, not %v", p, u, s, way)
					}
					if _, s := partials(prev, u, 1, 0, 1); s.WithMagnitude(1).Minus(way).Magnitude() > 1e-4 {
						t.Errorf("piece %d at u %g reaches along %v, not %v", p, u, s, way)
					}
				}
			}
			worst := 0.0
			for _, g := range c.guides {
				for i := 0; i < len(g); i += 40 {
					worst = max(worst, nearestOnPieces(bss, g[i]))
				}
			}
			if worst > c.off {
				t.Errorf("the surface is %.4g mm off its guides", worst)
			}
			sh, err := topo.MakeClosedGuidedLoftHeld(loops, c.held, c.guides, col, srf.OriginFor(9, 0), tol)
			if err != nil {
				t.Fatal(err)
			}
			vol := heldVolume(t, sh)
			if c.volume != 0 && math.Abs(vol-c.volume) > 0.02*c.volume {
				t.Errorf("volume %.4f, want %.4f within 2%%", vol, c.volume)
			}
		})
	}
	// None held is the guided ring; no guides, the held ring.
	bs, err := topo.ClosedGuidedLoftSurface(loops, up)
	if err != nil {
		t.Fatal(err)
	}
	if bss, err := topo.ClosedGuidedLoftSurfaces(loops, make([]float64, n), up); err != nil || len(bss) != 1 || bss[0].PointAt(0.3, 0.3) != bs.PointAt(0.3, 0.3) {
		t.Errorf("none held: %d pieces, %v", len(bss), err)
	}
	if bss, err := topo.ClosedGuidedLoftSurfaces(loops, []float64{0, 1, 0, 1}, nil); err != nil || len(bss) != 2 {
		t.Errorf("no guides: %d pieces, %v", len(bss), err)
	}
	if _, err := topo.ClosedGuidedLoftSurfaces(loops, []float64{math.NaN()}, up); err == nil {
		t.Error("held by NaN")
	}
	if _, err := topo.ClosedGuidedLoftSurfaces(loops, []float64{0, 1}, []topo.LoftGuide{up[0], up[0], up[0]}); err == nil {
		t.Error("three guides")
	}
}

// Open lofts held by a guide and square to the profile between their
// ends: three circles stacked up, a guide swelling out between them and
// passing each square to its plane, the middle held. The sections keep
// their size, so the volume is the cylinder's (Cavalieri); the loft is
// two pieces meeting square to the middle profile, and holds its guide.
// Two guides scale the sections: a solid of revolution.
func TestGuidedLoftHeld(t *testing.T) {
	t.Parallel()
	x := func(z float64) float64 { s := math.Sin(math.Pi * z / 10); return 5 + 2*s*s }
	g := guideXZ(x)
	other := guideXZ(func(z float64) float64 { return -x(z) })
	loops := [][]srf.SBezier{circleAt(5, 0, 0, 0), circleAt(5, 0, 0, 10), circleAt(5, 0, 0, 20)}
	col := core.NewRGBA(100, 100, 100, 255)
	tol := srf.Tol(0.001, 20)
	for _, c := range []struct {
		name   string
		guides []topo.LoftGuide
		ends   topo.LoftEnds
		want   float64
	}{
		{"one guide", []topo.LoftGuide{g}, topo.LoftEnds{}, math.Pi * 25 * 20},
		{"one guide, ends square", []topo.LoftGuide{g}, topo.LoftEnds{Start: 0.5, End: 0.5}, math.Pi * 25 * 20},
		// r(z) = 5 + 2 sin²: π∫r² over 20 is π(25·20 + 20·10 + 4·7.5).
		{"two guides", []topo.LoftGuide{g, other}, topo.LoftEnds{}, math.Pi * (500 + 200 + 30)},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			bss, err := topo.GuidedLoftThroughSurfaces(loops, 0, c.ends, topo.LoftSides{}, []float64{0.5}, c.guides)
			if err != nil {
				t.Fatal(err)
			}
			if len(bss) != 2 {
				t.Fatalf("%d pieces, want 2", len(bss))
			}
			for _, u := range []float64{0.1, 0.4, 0.75} {
				for p, at := range []float64{1, 0} {
					if _, s := partials(bss[p], u, at, 0, 1); math.Hypot(s.X, s.Y) > 1e-4*s.Magnitude() {
						t.Errorf("piece %d at u %g meets the middle along %v, not up", p, u, s)
					}
				}
			}
			worst := 0.0
			for _, gd := range c.guides {
				for i := 0; i < len(gd); i += 20 {
					worst = max(worst, nearestOnPieces(bss, gd[i]))
				}
			}
			// Near a free end the guided loft alone is 0.018 off too.
			if worst > 0.025 {
				t.Errorf("the surface is %.4g mm off its guides", worst)
			}
			sh, err := topo.MakeGuidedLoftThrough(loops, 0, c.ends, topo.LoftSides{}, []float64{0.5}, c.guides, col, 1<<30, tol)
			if err != nil {
				t.Fatal(err)
			}
			if vol := heldVolume(t, sh); math.Abs(vol-c.want) > 3e-3*c.want {
				t.Errorf("volume %.4f, want %.4f", vol, c.want)
			}
		})
	}
	// None held is the guided loft; no guides, the loft held square.
	bs, err := topo.GuidedLoftSurface(loops, 0, topo.LoftEnds{}, []topo.LoftGuide{g})
	if err != nil {
		t.Fatal(err)
	}
	if bss, err := topo.GuidedLoftThroughSurfaces(loops, 0, topo.LoftEnds{}, topo.LoftSides{}, []float64{0}, []topo.LoftGuide{g}); err != nil || len(bss) != 1 || bss[0].PointAt(0.3, 0.3) != bs.PointAt(0.3, 0.3) {
		t.Errorf("none held: %d pieces, %v", len(bss), err)
	}
	if bss, err := topo.GuidedLoftThroughSurfaces(loops, 0, topo.LoftEnds{}, topo.LoftSides{}, []float64{1}, nil); err != nil || len(bss) != 2 {
		t.Errorf("no guides: %d pieces, %v", len(bss), err)
	}
	if _, err := topo.GuidedLoftThroughSurfaces(loops, 0, topo.LoftEnds{}, topo.LoftSides{}, []float64{0, 0}, []topo.LoftGuide{g}); err == nil {
		t.Error("held at two profiles of one")
	}
	if _, err := topo.GuidedLoftThroughSurfaces(loops, 0, topo.LoftEnds{}, topo.LoftSides{}, []float64{1}, []topo.LoftGuide{g, g, g}); err == nil {
		t.Error("three guides")
	}
}
