package api

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// archBar is a bar along x from 0 to 10, y from -w to w, its sides from
// z = -1 up to 3 and its top an arc about an axis along x rising sag
// above them; its bottom is cut by the plane through z = b0-1 at x = 0
// and z = -1 at x = 10, so that its sides are lowest at x = 0. It gives
// the convex edge of the side at y = w with the top, and the volume
// before.
func archBar(d *Doc, w, sag, b0 float64) (SolidEdge, float64) {
	d.Features.SketchIn(d.Workplane("YZ"), "")
	c := (w*w - sag*sag) / (2 * sag)
	d.Sketch.Line(V(0, -w, 3), V(0, -w, -1))
	d.Sketch.Line(V(0, -w, -1), V(0, w, -1))
	d.Sketch.Line(V(0, w, -1), V(0, w, 3))
	d.Sketch.Arc(V(0, 0, 3-c), V(0, w, 3), V(0, -w, 3))
	d.Features.Extrude(10, "bar")
	d.Features.SketchIn(d.Workplane("ZX"), "")
	zAt := func(x float64) float64 { return b0*(1-x/10) - 1 }
	d.Sketch.Polyline(true, V(-1, 0, -3), V(11, 0, -3), V(11, 0, zAt(11)), V(-1, 0, zAt(-1)))
	cut := d.Features.Extrude(2*w+2, "slope")
	cut.SetTwoSided(true)
	cut.SetCombine(sketch.CombineDifference)
	return cut.Solid().EdgeAt(V(5, w, 3)), cut.Analyze().Check().Volume
}

// archRemoved is what a blend of size r of archBar's edge takes away,
// integrated: across the edge the blend's section is the same along it
// (the bar is an extrusion along x), and the sloped bottom cuts it at a
// height that depends on x alone. The section's width at height z,
// from the chamfer's line or the ball's arc out to the side or the top,
// times the length of the bar whose bottom lies under z, integrated
// over z.
func archRemoved(w, sag, b0, r float64, chamfer bool) float64 {
	c := (w*w - sag*sag) / (2 * sag)
	big, cz := c+sag, 3-c
	topY := func(z float64) float64 { return math.Sqrt(math.Max(0, big*big-(z-cz)*(z-cz))) }
	// The blend's inner boundary, the section's left at height z, from
	// its foot on the side at zLo to its foot on the top at zHi.
	var left func(z float64) float64
	var zLo, zHi float64
	if chamfer {
		// The foot on the top at chord r from the edge.
		a0 := math.Atan2(w, c)
		lo, hi := 0.0, a0
		for range 200 {
			m := (lo + hi) / 2
			if math.Hypot(big*math.Sin(m)-w, cz+big*math.Cos(m)-3) > r {
				lo = m
			} else {
				hi = m
			}
		}
		ya, za := big*math.Sin(lo), cz+big*math.Cos(lo)
		zLo, zHi = 3-r, za
		left = func(z float64) float64 { return w + (ya-w)*(z-zLo)/(zHi-zLo) }
	} else {
		yc := w - r
		zc := cz + math.Sqrt((big-r)*(big-r)-yc*yc)
		zLo, zHi = zc, cz+(zc-cz)*big/(big-r)
		left = func(z float64) float64 { return yc + math.Sqrt(math.Max(0, r*r-(z-zc)*(z-zc))) }
	}
	const n = 200000
	sum := 0.0
	for i := range n {
		z := zLo + (float64(i)+0.5)/n*(zHi-zLo)
		width := math.Min(w, topY(z)) - left(z)
		// The length of the bar whose bottom, b0(1-x/10)-1, lies under z.
		length := math.Max(0, math.Min(10, 10-10*(1-(z+1)/b0)))
		if width > 0 {
			sum += width * length
		}
	}
	return sum * (zHi - zLo) / n
}

// Blends of a convex edge, an arched bar's top edge, whose end runs off
// the bar's side past its sloped bottom, which is lower at the end than
// the blend reaches: the blend takes material away, and its end section
// crosses the bottom on the end face's own edge with it (topo's
// runOffAway). Refused before 2026-11-09 as a fillet "that takes
// material away"; no test reached it before, the convex edges tried
// being box fillets refused before the rolling ball. The bottom and the
// end face lose their corner with the side to the blend's cut and end
// section. Sound, and taking away what archRemoved integrates: chamfers
// within 0.05% (0.02% measured: the arc's mesh), fillets within 0.1%
// (0.07% measured, as much as the same fillet of a bar whose bottom is
// too low for it to run off: the meshes' chords). The ordinary run takes one
// size a kind on each bar, -short one of each; make test-sweeps all.
func TestBlendRunsOffTakingMaterialAway(t *testing.T) {
	t.Parallel()
	type tc struct {
		chamfer       bool
		w, sag, b0, r float64
	}
	var cases []tc
	for _, chamfer := range []bool{true, false} {
		for _, bar := range [][3]float64{{1, 0.5, 3.5}, {1, 0.5, 3.9}, {3, 0.5, 3.6}, {3, 0.5, 3.8}} {
			sizes := []float64{0.8, 1}
			if bar[0] == 3 {
				sizes = []float64{1, 1.5}
			}
			if !chamfer && bar[0] == 1 {
				// The top's arc is too tight for a ball that reaches past
				// the bottom.
				continue
			}
			for i, r := range sizes {
				if i == 0 || sweep.Full() {
					cases = append(cases, tc{chamfer, bar[0], bar[1], bar[2], r})
				}
			}
		}
	}
	if (testing.Short() || fixtures.RaceDetector) && !sweep.Full() {
		cases = []tc{{true, 1, 0.5, 3.9, 0.8}, {false, 3, 0.5, 3.8, 1}}
	}
	for _, c := range cases {
		kind := "fillet"
		if c.chamfer {
			kind = "chamfer"
		}
		t.Run(fmt.Sprintf("%s of %g, bar %g sag %g bottom %g", kind, c.r, c.w, c.sag, c.b0), func(t *testing.T) {
			t.Parallel()
			d := New()
			d.Edit.SetChordTolerance(0.003)
			d.Edit.SetMaxSegments(200)
			edge, before := archBar(d, c.w, c.sag, c.b0)
			g, err := blendOrRefusal(d, edge, c.r, c.chamfer)
			if err != nil {
				t.Fatal(err)
			}
			rep := g.Analyze().Check()
			if !rep.Sound() {
				t.Fatal(rep.Summary())
			}
			removed, want := before-rep.Volume, archRemoved(c.w, c.sag, c.b0, c.r, c.chamfer)
			tol := 0.0005 * want
			if !c.chamfer {
				tol = 0.001 * want
			}
			if math.Abs(removed-want) > tol {
				t.Errorf("took away %g, %g integrated", removed, want)
			}
		})
	}
}
