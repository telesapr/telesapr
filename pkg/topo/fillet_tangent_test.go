package topo_test

import (
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

// quarter is the arc of radius rad about c from angle a0 a quarter turn
// on, at height z.
func quarter(c geom.Vector, rad, a0, z float64) srf.SBezier {
	a1 := a0 + math.Pi/2
	p0 := v(c.X+rad*math.Cos(a0), c.Y+rad*math.Sin(a0), z)
	p1 := v(c.X+rad*math.Cos(a1), c.Y+rad*math.Sin(a1), z)
	m := v(c.X+rad*math.Sqrt2*math.Cos(a0+math.Pi/4), c.Y+rad*math.Sqrt2*math.Sin(a0+math.Pi/4), z)
	sb := srf.SBezierFrom3(p0, m, p1)
	sb.Weight[1] = math.Sqrt2 / 2
	return sb
}

// roundedRect is the outline of a w x l rectangle with its corner at
// the origin and its corners rounded at radius rad, those in round
// (counted anticlockwise from the origin's), at height z, anticlockwise.
func roundedRect(w, l, rad float64, round [4]bool, z float64) []srf.SBezier {
	corner := []geom.Vector{v(0, 0, z), v(w, 0, z), v(w, l, z), v(0, l, z)}
	// Into the rectangle from each corner, along each side.
	in := []geom.Vector{v(1, 1, 0), v(-1, 1, 0), v(-1, -1, 0), v(1, -1, 0)}
	var out []srf.SBezier
	for i := range 4 {
		c, n := corner[i], corner[(i+1)%4]
		d := n.Minus(c).WithMagnitude(1)
		a, b := c, n
		if round[i] {
			a = c.Plus(d.ScaledBy(rad))
		}
		if round[(i+1)%4] {
			b = n.Minus(d.ScaledBy(rad))
		}
		out = append(out, srf.SBezierFrom2(a, b))
		if j := (i + 1) % 4; round[j] {
			ctr := corner[j].Plus(v(in[j].X*rad, in[j].Y*rad, 0))
			out = append(out, quarter(ctr, rad, float64(j-2)*math.Pi/2, z))
		}
	}
	return out
}

// roundedPrism is a prism of height h over a rounded rectangle.
func roundedPrism(w, l, rad, h float64, round [4]bool) *srf.SShell {
	var sh srf.SShell
	var ls srf.SBezierLoopSet
	ls.Outer.Curves = roundedRect(w, l, rad, round, 0)
	ls.Normal = v(0, 0, -1)
	sh.MakeFromExtrusionOf(&ls, v(0, 0, 0), v(0, 0, h), core.NewRgbaColor(100, 100, 100, 255))
	return &sh
}

// A rounded rectangle's top edge, one line picked: the fillet goes on
// round the arcs and lines it meets smoothly, the whole outline, and
// takes the spandrel along the path of its centroid: the straight
// lengths and a circle of radius rad less the centroid's distance.
func TestFilletRoundedOutline(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	for _, c := range []struct{ rad, r float64 }{{5, 1}, {5, 3}, {3, 2.5}} {
		tol := 0.002 * c.r
		srf.SetChordTolMm(tol)
		sh := roundedPrism(30, 20, c.rad, 10, [4]bool{true, true, true, true})
		before := meshVolume(sh)
		if err := topo.FilletPicks(sh, []topo.FilletPick{{A: v(c.rad, 0, 10), B: v(30-c.rad, 0, 10)}}, c.r, 1<<30, false); err != nil {
			t.Fatalf("%+v: %v", c, err)
		}
		a, cd := spandrel(c.r)
		want := a * (2*(30-2*c.rad) + 2*(20-2*c.rad) + 2*math.Pi*(c.rad-cd))
		got := before - sound(t, sh)
		if allow := tol * 100 * 2 * c.r; math.Abs(got-want) > allow {
			t.Errorf("%+v: took %g, want %g within %g", c, got, want, allow)
		}
	}
}

func TestFilletOutlineRefused(t *testing.T) {
	// A half disc: its arc meets its line at a corner.
	var sh srf.SShell
	var ls srf.SBezierLoopSet
	ls.Outer.Curves = []srf.SBezier{srf.SBezierFrom2(v(-5, 0, 0), v(5, 0, 0)), quarter(v(0, 0, 0), 5, 0, 0), quarter(v(0, 0, 0), 5, math.Pi/2, 0)}
	ls.Normal = v(0, 0, -1)
	sh.MakeFromExtrusionOf(&ls, v(0, 0, 0), v(0, 0, 5), core.NewRgbaColor(100, 100, 100, 255))
	err := topo.FilletPicks(&sh, []topo.FilletPick{{A: v(-5, 0, 5), B: v(5, 0, 5)}, {At: v(0, 5, 5), Arc: true}}, 1, 1<<30, false)
	if err == nil || !strings.Contains(err.Error(), "must meet smoothly") {
		t.Errorf("half disc: %v", err)
	}
}

// TestFilletOutlineSweep rounds or chamfers the top of a prism over a
// rectangle with some corners rounded, picked by one line, over the
// corners rounded, their radius and the cut's size, against the volume
// along the centroid's path: the chain ends at sharp corners. The rate allowed, 0, was set before.
func TestFilletOutlineSweep(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("corners", "one", "two", "all"),
			sweep.Strings("cut", "fillet", "pocket"),
			sweep.Floats("rad", 2, 6),
			sweep.Floats("k", 0.2, 0.5, 0.9),
		},
		MaxFailRate: 0,
		Sample:      18,
	}
	grid.Run(func(p sweep.Point) error {
		rad, k := p.Float("rad"), p.Float("k")
		round := map[string][4]bool{"one": {false, true, false, false}, "two": {true, true, false, false}, "all": {true, true, true, true}}[p.String("corners")]
		pocket := p.String("cut") == "pocket"
		n := 0
		for _, b := range round {
			if b {
				n++
			}
		}
		r := k * math.Min(rad, 4)
		tol := 0.002 * r
		srf.SetChordTolMm(tol)
		var sh *srf.SShell
		if pocket {
			// The outline as a pocket's in a block: its floor is filled.
			var block, cut srf.SShell
			block.MakePrism([]geom.Vector{v(-10, -10, 0), v(40, -10, 0), v(40, 30, 0), v(-10, 30, 0)}, v(0, 0, 0), v(0, 0, 15),
				core.NewRgbaColor(100, 100, 100, 255))
			var moved srf.SShell
			moved.MakeFromTransformationOf(roundedPrism(30, 20, rad, 20, round), v(0, 0, 5), geom.QuaternionIdentity, 1)
			cut.MakeFromDifferenceOf(&block, &moved)
			sh = &cut
		} else {
			sh = roundedPrism(30, 20, rad, 10, round)
		}
		z := 10.0
		if pocket {
			z = 5
		}
		before := meshVolume(sh)
		// The bottom line, from its start to its end.
		x0, x1 := 0.0, 30.0
		if round[0] {
			x0 = rad
		}
		if round[1] {
			x1 = 30 - rad
		}
		if err := topo.FilletPicks(sh, []topo.FilletPick{{A: v(x0, 0, z), B: v(x1, 0, z)}}, r, 1<<30, false); err != nil {
			return err
		}
		a, cd := spandrel(r)
		// The chain: the rounded corners and the sides between them; it
		// runs on round a closed outline, else ends at sharp corners.
		var length float64
		switch p.String("corners") {
		case "all":
			length = 2*(30-2*rad) + 2*(20-2*rad)
		case "two":
			// The bottom side, both its corners' arcs and the sides up to
			// the sharp corners, which end on the top side's face.
			length = (30 - 2*rad) + 2*(20-rad)
		case "one":
			length = (30 - rad) + (20 - rad)
		}
		arcs := float64(n) * math.Pi / 2
		want := a * (length + arcs*(rad-cd))
		if pocket {
			want = -a * (length + arcs*(rad-cd))
		}
		tp, err := topo.Build(sh)
		if err != nil {
			return err
		}
		if probs := tp.Validate(); len(probs) > 0 {
			return fmt.Errorf("%s", probs[0].Msg)
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		rep := check.Solid(sh, &m)
		allow := tol*120*2*r + 1e-9*before
		switch {
		case !rep.Sound():
			return fmt.Errorf("unsound: %s", rep.String())
		case math.Abs(before-rep.Volume-want) > allow:
			return fmt.Errorf("took %g, want %g within %g", before-rep.Volume, want, allow)
		}
		return nil
	}).Require(t)
}
