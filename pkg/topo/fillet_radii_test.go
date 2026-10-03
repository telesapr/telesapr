package topo_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// boxRing is the four edges of the 10 x 20 x 30 box at height z, each
// with radius r.
func boxRing(z, r float64) []topo.FilletPick {
	c := []geom.Vector{v(0, 0, z), v(10, 0, z), v(10, 20, z), v(0, 20, z)}
	var out []topo.FilletPick
	for i := range c {
		out = append(out, topo.FilletPick{A: c[i], B: c[(i+1)%4], R: r})
	}
	return out
}

// Edges of one group with radii of their own: the box's top ring at one
// radius and its foot at another, each ring mitred at its corners; the
// volume each ring takes as it would alone.
func TestFilletRadii(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	srf.SetChordTolMm(0.002)
	sh := box()
	picks := append(boxRing(30, 3), boxRing(0, 0)...)
	if err := topo.FilletPicks(sh, picks, 1, 1<<30, false); err != nil {
		t.Fatal(err)
	}
	want := 6000 - cornerLoss(3, 60, 4, 0) - cornerLoss(1, 60, 4, 0)
	if got := sound(t, sh); math.Abs(got-want) > 0.002*math.Pi/2*4*60*2 {
		t.Errorf("volume %g, want %g", got, want)
	}
}

func TestFilletRadiiRefused(t *testing.T) {
	sh := box()
	picks := []topo.FilletPick{{A: v(0, 0, 30), B: v(10, 0, 30), R: 2}, {A: v(10, 0, 30), B: v(10, 20, 30)}}
	err := topo.FilletPicks(sh, picks, 1, 1<<30, false)
	if err == nil || !strings.Contains(err.Error(), "radius 1 and 2 meet") {
		t.Errorf("a mitre of two radii: %v", err)
	}
	picks[1].R = -1
	if err := topo.FilletPicks(box(), picks, 1, 1<<30, false); err == nil || !strings.Contains(err.Error(), "more than zero") {
		t.Errorf("a radius below zero: %v", err)
	}
	// A rounded outline's chain takes the radius of the edge picked.
	ro := roundedPrism(30, 20, 5, 10, [4]bool{true, true, true, true})
	if err := topo.FilletPicks(ro, []topo.FilletPick{{A: v(5, 0, 10), B: v(25, 0, 10), R: 2}}, 1, 1<<30, false); err != nil {
		t.Errorf("a chain at its own radius: %v", err)
	}
}

// TestFilletRadiiSweep rounds edges at radii of their own in one edit:
// a box's top long edges at one and its foot's short edges at another,
// which don't meet, or a cylinder's top and foot circles, over the two
// radii; the volume is the sum of each edge's. The rate allowed, 0, was
// set before.
func TestFilletRadiiSweep(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("shape", "box", "cylinder"),
			sweep.Floats("top", 0.5, 2, 4.5),
			sweep.Floats("foot", 0.5, 1.5, 3),
		},
		MaxFailRate: 0,
		Sample:      12,
	}
	grid.Run(func(p sweep.Point) error {
		top, foot := p.Float("top"), p.Float("foot")
		tol := 0.002
		srf.SetChordTolMm(tol)
		var sh *srf.SShell
		var picks []topo.FilletPick
		var want, allow float64
		if p.String("shape") == "box" {
			sh = box()
			picks = []topo.FilletPick{
				{A: v(0, 0, 30), B: v(0, 20, 30), R: top}, {A: v(10, 0, 30), B: v(10, 20, 30), R: top},
				{A: v(0, 0, 0), B: v(10, 0, 0), R: foot}, {A: v(0, 20, 0), B: v(10, 20, 0), R: foot},
			}
			want = 6000 - filletLoss(top, math.Pi/2, 20)*2 - filletLoss(foot, math.Pi/2, 10)*2
			allow = tol * math.Pi / 2 * (2*top*20 + 2*foot*10) * 2
		} else {
			sh = cylinder(8, 12)
			picks = []topo.FilletPick{
				{At: v(8*math.Cos(1), 8*math.Sin(1), 12), Arc: true, R: top},
				{At: v(8*math.Cos(1), 8*math.Sin(1), 0), Arc: true, R: foot},
			}
			at, ct := spandrel(top)
			af, cf := spandrel(foot)
			want = math.Pi*64*12 - at*2*math.Pi*(8-ct) - af*2*math.Pi*(8-cf)
			allow = tol * 2 * math.Pi * 8 * (12 + 2*math.Pi*8)
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
