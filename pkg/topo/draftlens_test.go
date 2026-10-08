package topo_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// lensLoop is the lens where the discs of radius 5 about (0, 0) and
// (5, 5) overlap: two quarter arcs meeting at sharp corners.
func lensLoop() []srf.SBezier {
	up, down := circle(5, 0, 0)[0], circle(5, 0, 0)[0]
	down.Ctrl[0], down.Ctrl[2] = up.Ctrl[2], up.Ctrl[0]
	down.Ctrl[1] = v(0, 0, 0)
	return []srf.SBezier{up, down}
}

// discsOverlap is the area two discs of radii a and b, d apart, share.
func discsOverlap(a, b, d float64) float64 {
	if d >= a+b {
		return 0
	}
	if d <= math.Abs(a-b) {
		r := math.Min(a, b)
		return math.Pi * r * r
	}
	x := (d*d + a*a - b*b) / (2 * d)
	y := d - x
	return a*a*math.Acos(x/a) - x*math.Sqrt(a*a-x*x) + b*b*math.Acos(y/b) - y*math.Sqrt(b*b-y*y)
}

// A lens drafted (roadmap 28.2): its two arcs meet at sharp corners, so
// their cones meet in a curve of degree four, followed by a quintic;
// each arc leans by its own angle, so the lens at height z is where
// discs of radii 5 - z tan a and 5 - z tan b overlap.
func TestDraftSharpArcs(t *testing.T) {
	old := srf.ChordTolMm()
	srf.SetChordTolMm(0.001)
	defer srf.SetChordTolMm(old)
	color := core.NewRGBA(100, 100, 100, 255)
	rad := func(deg float64) float64 { return deg * math.Pi / 180 }
	d := 5 * math.Sqrt2
	for _, c := range []struct{ a, b, h float64 }{
		{5, 5, 5}, {2, 6, 5}, {0, 8, 4}, {-3, 4, 6},
	} {
		sh, err := topo.DraftedPrismAngles([][]srf.SBezier{lensLoop()}, [][]float64{{rad(c.a), rad(c.b)}},
			v(0, 0, c.h), color, 0, srf.Tolerance{})
		if err != nil {
			t.Fatalf("%v: %v", c, err)
		}
		want := integrateTo(c.h, func(z float64) float64 {
			return discsOverlap(5-z*math.Tan(rad(c.a)), 5-z*math.Tan(rad(c.b)), d)
		})
		if got := sound(t, sh); math.Abs(got-want) > 2e-3*want {
			t.Errorf("%v: volume %g, want %g", c, got, want)
		}
	}
}

// TestDraftSharpArcsSweep drafts the lens over a grid of angles for
// each arc, in and out, and heights, and requires sound solids of the
// overlapping discs' volume. The rate allowed was fixed at none before
// the seam was written.
func TestDraftSharpArcsSweep(t *testing.T) {
	old := srf.ChordTolMm()
	srf.SetChordTolMm(0.002)
	defer srf.SetChordTolMm(old)
	color := core.NewRGBA(100, 100, 100, 255)
	rad := func(deg float64) float64 { return deg * math.Pi / 180 }
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("a", -4, 0, 3, 8),
			sweep.Floats("b", -4, 0, 3, 8),
			sweep.Floats("h", 3, 6),
		},
		MaxFailRate: 0,
		Sample:      12,
	}
	grid.Run(func(p sweep.Point) error {
		a, b, h := p.Float("a"), p.Float("b"), p.Float("h")
		sh, err := topo.DraftedPrismAngles([][]srf.SBezier{lensLoop()}, [][]float64{{rad(a), rad(b)}}, v(0, 0, h), color, 0, srf.Tolerance{})
		if err != nil {
			return err
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		r := check.Solid(sh, &m)
		if !r.Sound() {
			return fmt.Errorf("unsound: %s", r.String())
		}
		want := integrateTo(h, func(z float64) float64 {
			return discsOverlap(5-z*math.Tan(rad(a)), 5-z*math.Tan(rad(b)), 5*math.Sqrt2)
		})
		if math.Abs(r.Volume-want) > 3e-3*want {
			return fmt.Errorf("volume %g, want %g", r.Volume, want)
		}
		return nil
	}).Require(t)
}
