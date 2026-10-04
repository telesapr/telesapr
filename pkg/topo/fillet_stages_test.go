package topo_test

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// boxUprights are the 10 x 20 x 30 box's four upright edges, each with
// radius r.
func boxUprights(r float64) []topo.FilletPick {
	var out []topo.FilletPick
	for _, c := range [][2]float64{{0, 0}, {10, 0}, {10, 20}, {0, 20}} {
		out = append(out, topo.FilletPick{A: v(c[0], c[1], 0), B: v(c[0], c[1], 30), R: r})
	}
	return out
}

// boxStaged is the volume of the box with its upright edges rounded at
// R and its top ring at r < R round them: the ring's section along the
// lines left between the arcs, and a quarter turn of it about each arc's
// axis at its centroid (Pappus).
func boxStaged(R, r float64) float64 {
	a, c := spandrel(r)
	return 6000 - 4*30*R*R*(1-math.Pi/4) - a*(2*(10-2*R)+2*(20-2*R)) - a*2*math.Pi*(R-c)
}

// squareBoss is a 40 x 30 x 10 block with a 20 x 10 boss 10 high on its
// top, and the picks of the boss's upright edges at radius R and of its
// foot's edges at r.
func squareBoss(R, r float64) (*srf.SShell, []topo.FilletPick) {
	var block, boss, sh srf.SShell
	block.MakePrism([]geom.Vector{v(0, 0, 0), v(40, 0, 0), v(40, 30, 0), v(0, 30, 0)}, v(0, 0, 0), v(0, 0, 10),
		core.NewRgbaColor(100, 100, 100, 255))
	boss.MakePrism([]geom.Vector{v(10, 10, 9), v(30, 10, 9), v(30, 20, 9), v(10, 20, 9)}, v(0, 0, 0), v(0, 0, 11),
		core.NewRgbaColor(100, 100, 100, 255))
	sh.MakeFromUnionOf(&block, &boss)
	var picks []topo.FilletPick
	cs := [][2]float64{{10, 10}, {30, 10}, {30, 20}, {10, 20}}
	for i, c := range cs {
		n := cs[(i+1)%4]
		picks = append(picks, topo.FilletPick{A: v(c[0], c[1], 10), B: v(c[0], c[1], 20), R: R},
			topo.FilletPick{A: v(c[0], c[1], 10), B: v(n[0], n[1], 10), R: r})
	}
	return &sh, picks
}

// bossStaged is the square boss's volume with its upright edges rounded
// at R, convex, and its foot filled at r round them: the foot's section
// along the lines and a quarter turn of it about each upright's axis, on
// the far side of the wall.
func bossStaged(R, r float64) float64 {
	a, c := spandrel(r)
	return 14000 - 4*10*R*R*(1-math.Pi/4) + a*(2*(20-2*R)+2*(10-2*R)) + a*2*math.Pi*(R+c)
}

// Corners where rounded edges of different radii, or convex and concave
// ones, meet, rounded in stages (roadmap 22.4): a box's upright edges at
// one radius and its top ring at a smaller one; a square boss's upright
// edges, convex, and its foot, concave, at radii either way round.
func TestFilletStages(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	const tol = 0.002
	srf.SetChordTolMm(tol)
	sh := box()
	if err := topo.FilletPicks(sh, append(boxUprights(3), boxRing(30, 1)...), 1, 1<<30, false); err != nil {
		t.Fatal(err)
	}
	if got, want := sound(t, sh), boxStaged(3, 1); math.Abs(got-want) > tol*math.Pi/2*(4*3*30+2*60)*2 {
		t.Errorf("box: volume %g, want %g", got, want)
	}
	for _, rr := range [][2]float64{{3, 1}, {2, 2}, {1, 3}} {
		sh, picks := squareBoss(rr[0], rr[1])
		if err := topo.FilletPicks(sh, picks, 1, 1<<30, false); err != nil {
			t.Errorf("boss %v: %v", rr, err)
			continue
		}
		if got, want := sound(t, sh), bossStaged(rr[0], rr[1]); math.Abs(got-want) > tol*math.Pi/2*(4*rr[0]*10+rr[1]*60)*2 {
			t.Errorf("boss %v: volume %g, want %g", rr, got, want)
		}
	}
}

// What neither the stages nor a setback rounds is refused, saying why:
// two top edges of different radii mitred over an upright edge left
// sharp, where the blend would need a crease. A top ring larger than
// the uprights it would go round is set back (TestFilletSetback).
func TestFilletStagesRefused(t *testing.T) {
	picks := []topo.FilletPick{{A: v(0, 0, 30), B: v(10, 0, 30), R: 2}, {A: v(10, 0, 30), B: v(10, 20, 30)}}
	err := topo.FilletPicks(box(), picks, 1, 1<<30, false)
	if err == nil || !strings.Contains(err.Error(), "left sharp") {
		t.Errorf("a mitre of two radii: %v", err)
	}
	var fx *topo.FilletError
	if !errors.As(err, &fx) || fx.At.Z != 30 {
		t.Errorf("the refusal is not marked at the top: %v", err)
	}
}

// TestFilletStagesSweep rounds the box's uprights and top ring, and the
// square boss's uprights and foot, at radii over a grid with the ring's
// or foot's radius the smaller for the box; the volume is the formula's.
// The rate allowed, 0, was set before the stages were built.
func TestFilletStagesSweep(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("shape", "box", "boss"),
			sweep.Floats("upright", 1.5, 3, 4.5),
			sweep.Floats("share", 0.2, 0.5, 0.9),
		},
		MaxFailRate: 0,
		Sample:      8,
	}
	grid.Run(func(p sweep.Point) error {
		R := p.Float("upright")
		r := R * p.Float("share")
		const tol = 0.002
		srf.SetChordTolMm(tol)
		var sh *srf.SShell
		var picks []topo.FilletPick
		var want, allow float64
		if p.String("shape") == "box" {
			sh, picks = box(), append(boxUprights(R), boxRing(30, r)...)
			want, allow = boxStaged(R, r), tol*math.Pi/2*(4*R*30+r*60)*2
		} else {
			sh, picks = squareBoss(R, r)
			want, allow = bossStaged(R, r), tol*math.Pi/2*(4*R*10+r*60)*2
		}
		if err := topo.FilletPicks(sh, picks, 1, 1<<30, false); err != nil {
			return err
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		rep := check.Solid(sh, &m)
		switch {
		case !rep.Sound():
			return fmt.Errorf("unsound: %s", rep.String())
		case math.Abs(rep.Volume-want) > allow:
			return fmt.Errorf("volume %g, want %g within %g", rep.Volume, want, allow)
		}
		return nil
	}).Require(t)
}
