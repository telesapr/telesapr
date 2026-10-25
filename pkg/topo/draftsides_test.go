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

// uniform is angle for every curve of loops.
func uniform(loops [][]srf.SBezier, angle float64) [][]float64 {
	out := make([][]float64, len(loops))
	for k, l := range loops {
		for range l {
			out[k] = append(out[k], angle)
		}
	}
	return out
}

// integrateTo is the integral of f over [0, h], by Simpson's rule in many
// steps.
func integrateTo(h float64, f func(z float64) float64) float64 {
	const n = 400
	s := f(0) + f(h)
	for i := 1; i < n; i++ {
		w := 2.0
		if i%2 == 1 {
			w = 4
		}
		s += w * f(h*float64(i)/n)
	}
	return s * h / (3 * n)
}

// dShape is a half disc of radius r on y >= 0: two quarter arcs and the
// diameter, which meets them at sharp corners.
func dShape(r float64) []srf.SBezier {
	d := circle(r, 0, 0)[:2]
	return append(d, srf.SBezierFrom2(v(-r, 0, 0), v(r, 0, 0)))
}

// segment is the area of the disc of radius r on y >= c.
func segment(r, c float64) float64 {
	if c >= r {
		return 0
	}
	return r*r*math.Acos(c/r) - c*math.Sqrt(r*r-c*c)
}

// A side per angle (roadmap 28.2): the uniform cases as DraftedPrism
// makes them; one side of a block drafted; a D, whose diameter meets its
// arc at sharp corners, drafted all round, along the diameter only and
// along the arc only.
func TestDraftedPrismAngles(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.001, 200)
	color := core.NewRGBA(100, 100, 100, 255)
	rad := func(deg float64) float64 { return deg * math.Pi / 180 }
	square := polyLoop(v(0, 0, 0), v(20, 0, 0), v(20, 10, 0), v(0, 10, 0))
	tanOf := func(deg float64) float64 { return math.Tan(rad(deg)) }
	for _, c := range []struct {
		name   string
		loops  [][]srf.SBezier
		angles [][]float64
		h      float64
		area   func(z float64) float64
		round  float64
	}{
		{"block", [][]srf.SBezier{square}, uniform([][]srf.SBezier{square}, rad(5)), 10,
			func(z float64) float64 { x := z * tanOf(5); return (20 - 2*x) * (10 - 2*x) }, 0},
		{"one side", [][]srf.SBezier{square}, [][]float64{{rad(10), 0, 0, 0}}, 10,
			func(z float64) float64 { return 20 * (10 - z*tanOf(10)) }, 0},
		{"two sides", [][]srf.SBezier{square}, [][]float64{{rad(10), rad(-5), 0, 0}}, 10,
			func(z float64) float64 { return (20 + z*tanOf(5)) * (10 - z*tanOf(10)) }, 0},
		{"rounded box", [][]srf.SBezier{roundedLoop(20, 10, 3)}, uniform([][]srf.SBezier{roundedLoop(20, 10, 3)}, rad(7)), 10,
			func(z float64) float64 { x := z * tanOf(7); return roundedArea(20-2*x, 10-2*x, 3-x) }, 2 * math.Pi * 3},
		{"D all round", [][]srf.SBezier{dShape(10)}, uniform([][]srf.SBezier{dShape(10)}, rad(5)), 10,
			func(z float64) float64 { x := z * tanOf(5); return segment(10-x, x) }, math.Pi * 10},
		{"D's diameter", [][]srf.SBezier{dShape(10)}, [][]float64{{0, 0, rad(20)}}, 10,
			func(z float64) float64 { return segment(10, z*tanOf(20)) }, math.Pi * 10},
		{"D's arc", [][]srf.SBezier{dShape(10)}, [][]float64{{rad(8), rad(8), 0}}, 10,
			func(z float64) float64 { return segment(10-z*tanOf(8), 0) }, math.Pi * 10},
		{"D leaning out", [][]srf.SBezier{dShape(10)}, uniform([][]srf.SBezier{dShape(10)}, rad(-6)), 10,
			func(z float64) float64 { x := z * tanOf(-6); return segment(10-x, x) }, math.Pi * 10},
	} {
		for _, way := range []float64{1, -1} {
			sh, err := topo.DraftedPrismAngles(c.loops, c.angles, v(0, 0, way*c.h), color, 1<<30, tol)
			if err != nil {
				t.Errorf("%s (%g): %v", c.name, way, err)
				continue
			}
			want := integrateTo(c.h, c.area)
			if got := sound(t, sh); math.Abs(got-want) > 0.001*c.round*c.h+1e-6*want {
				t.Errorf("%s (%g): volume %g, want %g", c.name, way, got, want)
			}
		}
	}
}

func TestDraftedPrismAnglesRefused(t *testing.T) {
	t.Parallel()
	color := core.NewRGBA(100, 100, 100, 255)
	rad := func(deg float64) float64 { return deg * math.Pi / 180 }
	rounded := roundedLoop(20, 10, 3)
	uneven := uniform([][]srf.SBezier{rounded}, rad(5))
	uneven[0][1] = rad(2)
	for name, c := range map[string]struct {
		loops  [][]srf.SBezier
		angles [][]float64
		want   string
	}{
		"tangent pieces lean differently": {[][]srf.SBezier{rounded}, uneven, "lean differently"},
		"too few angles":                  {[][]srf.SBezier{rounded}, [][]float64{{0}}, "don't match"},
		"flat":                            {[][]srf.SBezier{dShape(5)}, [][]float64{{rad(90), 0, 0}}, "flat"},
	} {
		_, err := topo.DraftedPrismAngles(c.loops, c.angles, v(0, 0, 5), color, 0, srf.Tolerance{})
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want an error saying %q", name, err, c.want)
		}
	}
}

// TestDraftAnglesSweep drafts a D and a block a side at a time over
// several angles and heights. The rate allowed was fixed at none before
// the sides were written.
func TestDraftAnglesSweep(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.001, 200)
	color := core.NewRGBA(100, 100, 100, 255)
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("which", "all", "diameter", "arc"),
			sweep.Floats("degrees", -15, -4, 2, 9, 25),
			sweep.Floats("height", 2, 8),
		},
		MaxFailRate: 0,
		Sample:      12,
		Parallel:    true,
	}
	grid.Run(func(p sweep.Point) error {
		deg, h := p.Float("degrees"), p.Float("height")
		a := deg * math.Pi / 180
		tan := math.Tan(a)
		var angles []float64
		var area func(z float64) float64
		switch p.String("which") {
		case "all":
			angles = []float64{a, a, a}
			area = func(z float64) float64 { return segment(10-z*tan, z*tan) }
		case "diameter":
			angles = []float64{0, 0, a}
			area = func(z float64) float64 { return segment(10, z*tan) }
		default:
			angles = []float64{a, a, 0}
			area = func(z float64) float64 { return segment(10-z*tan, 0) }
		}
		sh, err := topo.DraftedPrismAngles([][]srf.SBezier{dShape(10)}, [][]float64{angles}, v(0, 0, h), color, 1<<30, tol)
		if err != nil {
			return err
		}
		want := integrateTo(h, area)
		got, err := soundVolume(sh)
		if err != nil {
			return err
		}
		if math.Abs(got-want) > 0.001*math.Pi*10*h+1e-6*want {
			return fmt.Errorf("volume %g, want %g", got, want)
		}
		return nil
	}).Require(t)
}

// soundVolume is sound's check without a test to fail, for a sweep's
// points, which run on goroutines of their own: the volume of sh's mesh,
// or what is wrong with its topology or its mesh.
func soundVolume(sh *srf.SShell) (float64, error) {
	tp, err := topo.Build(sh)
	if err != nil {
		return 0, err
	}
	if probs := tp.Validate(); len(probs) > 0 {
		return 0, fmt.Errorf("%s at %v", probs[0].Msg, probs[0].At)
	}
	var m srf.SMesh
	sh.TriangulateInto(&m)
	r := check.Solid(sh, &m)
	if !r.Sound() {
		return 0, fmt.Errorf("unsound: %s", r.String())
	}
	if e, me := tp.Euler(), topo.MeshEuler(&m); e != me {
		return 0, fmt.Errorf("Euler %d, the mesh's %d", e, me)
	}
	return r.Volume, nil
}
