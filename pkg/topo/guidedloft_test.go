package topo_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// guideXZ is a guide in the plane y = 0 from z = 0 to z = 20, at x(z).
func guideXZ(x func(z float64) float64) topo.LoftGuide {
	var g topo.LoftGuide
	const n = 800
	for i := 0; i <= n; i++ {
		z := 20 * float64(i) / n
		g = append(g, v(x(z), 0, z))
	}
	return g
}

// guidedSolid is the guided loft's solid, which must be sound, and its
// volume.
func guidedSolid(t *testing.T, loops [][]srf.SBezier, ends topo.LoftEnds, guides ...topo.LoftGuide) float64 {
	t.Helper()
	sh, err := topo.MakeGuidedLoft(loops, 0, ends, guides, core.NewRGBA(100, 100, 100, 255), 1<<30, srf.Tolerance{})
	if err != nil {
		t.Fatal(err)
	}
	var m srf.SMesh
	sh.TriangulateInto(&m)
	r := check.Solid(sh, &m)
	if !r.Sound() {
		t.Fatalf("unsound: %s", r.String())
	}
	return r.Volume
}

// One guide moves each section within its plane: circles stay circles
// of their radius at their height, so the volume is the cylinder's
// whatever the guide's bend (Cavalieri), and the surface holds the
// guide.
func TestGuidedLoftOneGuide(t *testing.T) {
	old := srf.ChordTolMm()
	srf.SetChordTolMm(0.001)
	defer srf.SetChordTolMm(old)
	g := guideXZ(func(z float64) float64 { return 5 + 6*z/20 + 3*math.Sin(math.Pi*z/20) })
	loops := [][]srf.SBezier{circleAt(5, 0, 0, 0), circleAt(5, 6, 0, 20)}
	want := math.Pi * 25 * 20
	if vol := guidedSolid(t, loops, topo.LoftEnds{}, g); math.Abs(vol-want) > 2e-3*want {
		t.Errorf("volume %g, want %g", vol, want)
	}
	bs, err := topo.GuidedLoftSurface(loops, 0, topo.LoftEnds{}, []topo.LoftGuide{g})
	if err != nil {
		t.Fatal(err)
	}
	worst := 0.0
	for _, z := range []float64{1, 3.3, 7, 10, 12.5, 16, 19} {
		p := v(5+6*z/20+3*math.Sin(math.Pi*z/20), 0, z)
		c := bs.IsoV(vAtHeight(bs, z))
		best := math.Inf(1)
		for i := range 4001 {
			best = math.Min(best, c.PointAt(float64(i)/4000).Minus(p).Magnitude())
		}
		worst = math.Max(worst, best)
	}
	if worst > 0.02 {
		t.Errorf("the surface is %g mm off the guide", worst)
	}
	// Free, the same profiles make an oblique cylinder of that volume
	// too; the guide bends it, the free one doesn't pass (8, 0, 10).
	free, err := topo.SmoothLoftSurfaceEnds(loops, 0, topo.LoftEnds{})
	if err != nil {
		t.Fatal(err)
	}
	if p := free.PointAt(0, 0.5); math.Abs(p.X-8) > 1e-6 {
		t.Errorf("the free loft at half way: %v", p)
	}
	if p := bs.PointAt(0, vAtHeight(bs, 10)); math.Abs(p.X-11) > 0.02 {
		t.Errorf("the guided loft at half way: %v, want x = 11", p)
	}
}

// Two guides scale each section too: guides at ±r(z) make a solid of
// revolution of radius r(z).
func TestGuidedLoftTwoGuides(t *testing.T) {
	old := srf.ChordTolMm()
	srf.SetChordTolMm(0.001)
	defer srf.SetChordTolMm(old)
	r := func(z float64) float64 { return 5 + 2*math.Sin(math.Pi*z/20) }
	a := guideXZ(r)
	b := guideXZ(func(z float64) float64 { return -r(z) })
	loops := [][]srf.SBezier{circleAt(5, 0, 0, 0), circleAt(5, 0, 0, 20)}
	want := math.Pi*540 + 800
	if vol := guidedSolid(t, loops, topo.LoftEnds{}, a, b); math.Abs(vol-want) > 2e-3*want {
		t.Errorf("volume %g, want %g", vol, want)
	}
	// Either way round.
	rev := make(topo.LoftGuide, len(b))
	for i, p := range b {
		rev[len(b)-1-i] = p
	}
	if vol := guidedSolid(t, loops, topo.LoftEnds{}, a, rev); math.Abs(vol-want) > 2e-3*want {
		t.Errorf("a guide given from the last profile: volume %g, want %g", vol, want)
	}
}

// Through three profiles, the guide passing the middle one, and with
// the ends set square.
func TestGuidedLoftThroughProfiles(t *testing.T) {
	old := srf.ChordTolMm()
	srf.SetChordTolMm(0.001)
	defer srf.SetChordTolMm(old)
	g := guideXZ(func(z float64) float64 { return 5 + 2*math.Sin(math.Pi*z/10) })
	loops := [][]srf.SBezier{circleAt(5, 0, 0, 0), circleAt(5, 0, 0, 10), circleAt(5, 0, 0, 20)}
	want := math.Pi * 25 * 20
	if vol := guidedSolid(t, loops, topo.LoftEnds{}, g); math.Abs(vol-want) > 2e-3*want {
		t.Errorf("volume %g, want %g", vol, want)
	}
	if vol := guidedSolid(t, loops, topo.LoftEnds{Start: 0.5, End: 0.5}, g); math.Abs(vol-want) > 5e-3*want {
		t.Errorf("ends square: volume %g, want %g", vol, want)
	}
}

func TestGuidedLoftRefusals(t *testing.T) {
	loops := [][]srf.SBezier{circleAt(5, 0, 0, 0), circleAt(5, 0, 0, 10), circleAt(5, 0, 0, 20)}
	on := guideXZ(func(z float64) float64 { return 5 })
	for _, c := range []struct {
		guides []topo.LoftGuide
		want   string
	}{
		{[]topo.LoftGuide{guideXZ(func(z float64) float64 { return 6 })}, "doesn't start on the first profile"},
		{[]topo.LoftGuide{guideXZ(func(z float64) float64 { return 5 + z*(20-z)/100 })}, "misses profile 2"},
		{[]topo.LoftGuide{guideXZ(func(z float64) float64 { return 5 + z/20 })}, "doesn't end on the last profile"},
		{[]topo.LoftGuide{on, on, on}, "two guides at most"},
		{[]topo.LoftGuide{on, on}, "meet profile 1 at one point"},
		{[]topo.LoftGuide{{v(5, 0, 0)}}, "has no length"},
	} {
		_, err := topo.GuidedLoftSurface(loops, 0, topo.LoftEnds{}, c.guides)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("want %q, got %v", c.want, err)
		}
	}
}

// TestGuidedLoftSweep lofts circles of radius 4 through two or three
// profiles, their centres moved aside along a parabola, held to one
// guide that bows out between the profiles or to two that widen the
// loft, ends free or square; it requires sound solids whose volume is
// the integral of the circles' areas, r(z) set by the guides. The rate
// allowed was fixed at none before the guides were written.
func TestGuidedLoftSweep(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	srf.SetChordTolMm(0.002)
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("profiles", 2, 3),
			sweep.Floats("guides", 1, 2),
			sweep.Floats("bow", 0, 1.5, 3), // 4 pinches three profiles to a point
			sweep.Floats("aside", 0, 6),
			sweep.Floats("ends", 0, 0.5),
		},
		MaxFailRate: 0,
	}
	grid.Run(func(p sweep.Point) error {
		n := int(p.Float("profiles"))
		aside, bow := p.Float("aside"), p.Float("bow")
		const h = 15.0
		k := math.Pi * float64(n-1) / h
		centre := func(z float64) float64 { return aside * (z / h) * (z / h) }
		var loops [][]srf.SBezier
		for i := range n {
			z := h * float64(i) / float64(n-1)
			loops = append(loops, circleAt(4, centre(z), 0, z))
		}
		at := func(side float64) topo.LoftGuide {
			var g topo.LoftGuide
			for i := range 601 {
				z := h * float64(i) / 600
				g = append(g, v(centre(z)+side*(4+bow*math.Sin(k*z)), 0, z))
			}
			return g
		}
		guides := []topo.LoftGuide{at(1)}
		// One guide shifts the circles; two widen them too.
		r := func(z float64) float64 { return 4 }
		if p.Float("guides") == 2 {
			guides = append(guides, at(-1))
			r = func(z float64) float64 { return 4 + bow*math.Sin(k*z) }
		}
		e := p.Float("ends")
		sh, err := topo.MakeGuidedLoft(loops, 0, topo.LoftEnds{Start: e, End: e}, guides,
			core.NewRGBA(1, 1, 1, 255), 1<<30, srf.Tolerance{})
		if err != nil {
			return err
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		rep := check.Solid(sh, &m)
		if !rep.Sound() {
			return fmt.Errorf("unsound: %s", rep.String())
		}
		want := 0.0
		const steps = 3000
		for i := range steps {
			z := h * (float64(i) + 0.5) / steps
			want += math.Pi * r(z) * r(z) * h / steps
		}
		allow := 0.003
		if e != 0 {
			allow = 0.006
		}
		if math.Abs(rep.Volume-want) > allow*want {
			return fmt.Errorf("volume %g, want %g", rep.Volume, want)
		}
		return nil
	}).Require(t)
}
