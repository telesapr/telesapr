package topo_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// turnedTo is profile, in the XZ plane, turned about the Z axis by a
// radians, as a lathe drawn in another plane through the axis has it.
func turnedTo(profile *srf.SBezierLoopSetSet, a float64) *srf.SBezierLoopSetSet {
	out := &srf.SBezierLoopSetSet{}
	turn := func(l srf.SBezierLoop) srf.SBezierLoop {
		var c srf.SBezierLoop
		for _, sb := range l.Curves {
			for k := 0; k <= sb.Degree; k++ {
				sb.Ctrl[k] = sb.Ctrl[k].RotatedAroundAxis(v(0, 0, 1), a)
			}
			c.Curves = append(c.Curves, sb)
		}
		return c
	}
	for _, set := range profile.Sets {
		n := set
		n.Outer = turn(set.Outer)
		n.Holes = nil
		for _, h := range set.Holes {
			n.Holes = append(n.Holes, turn(h))
		}
		n.Normal = set.Normal.RotatedAroundAxis(v(0, 0, 1), a)
		n.Point = set.Point.RotatedAroundAxis(v(0, 0, 1), a)
		out.Sets = append(out.Sets, n)
	}
	return out
}

// Lathes about one axis joined: the solid is the whole turn of their
// profiles joined, and is hollowed by that profile. Volumes by the
// cylinders the profiles' rectangles turn into.
func TestShellJoinedRevolution(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.001, 20)
	cases := []struct {
		name  string
		a, b  [][2]float64
		turnB float64
		op    srf.SurfaceCombineAs
		top   float64
		want  float64
	}{
		// A cup and a flange about its foot, drawn a quarter turn round:
		// the inside is the flange's ring 9 across and 2 high and the
		// cup's 4 across and 7 high above it.
		{"cup and flange", [][2]float64{{0, 0}, {5, 0}, {5, 10}, {0, 10}},
			[][2]float64{{5, 0}, {10, 0}, {10, 4}, {5, 4}}, math.Pi / 2, srf.SurfaceUnion, 10,
			math.Pi * (100*4 + 25*6 - 81*2 - 16*7)},
		// A groove turned out of a cylinder, drawn on the other side of
		// the axis: the inside follows the groove, 1 in from it. (Its
		// profile reaches past the cylinder: one ending on the cylinder's
		// side leaves a Boolean the kernel can't make of the solids.)
		{"grooved", [][2]float64{{0, 0}, {10, 0}, {10, 10}, {0, 10}},
			[][2]float64{{8, 4}, {12, 4}, {12, 6}, {8, 6}}, math.Pi, srf.SurfaceDiff, 10,
			math.Pi * (100*10 - 36*2 - (81*9 - 32*4))},
	}
	for _, c := range cases {
		pa := meridian(t, c.a, nil, tol)
		pb := turnedTo(meridian(t, c.b, nil, tol), c.turnB)
		sa, _ := turned(pa, 0, 2*math.Pi, tol)
		sb := &srf.SShell{Tol: tol}
		sb.MakeFromRevolutionOf(&pb.Sets[0], v(0, 0, 0), v(0, 0, 1), core.NewRGBA(100, 100, 100, 255), noGroup{})
		sh := &srf.SShell{Tol: tol}
		sh.MakeFromBoolean(sa, sb, c.op)
		sh.MergeCoincidentSurfaces()
		rev, err := topo.JoinedRevolution(v(0, 0, 0), v(0, 0, 1),
			[]topo.RevolvedPart{{Profile: pa, Combine: srf.SurfaceUnion}, {Profile: pb, Combine: c.op}}, tol)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		tp := build(t, sh)
		out, err := topo.ShellRevolved(sh, rev, facesOn(t, tp, v(0, 0, c.top), v(0, 0, 1)), 1, 1<<30)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got := sound(t, out); math.Abs(got-c.want) > 0.002*c.want {
			t.Errorf("%s: volume %g, want %g", c.name, got, c.want)
		}
	}
	// A profile across the axis is no half of a revolution.
	across := meridian(t, [][2]float64{{-1, 0}, {5, 0}, {5, 10}, {-1, 10}}, nil, tol)
	if _, err := topo.JoinedRevolution(v(0, 0, 0), v(0, 0, 1), []topo.RevolvedPart{{Profile: across, Combine: srf.SurfaceUnion}}, tol); err == nil {
		t.Error("a profile across the axis: no error")
	}
}

// Profiles of arcs and lines joined: turned, the joined profile is the
// solid the kernel's Boolean makes of the lathes. A ball on a stem, the
// stem's end crossing the ball's arc; their union is hollowed too.
func TestJoinedRevolutionArcs(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.001, 20)
	ball := [][2]float64{{0, 5}, {5, 10}, {0, 15}}
	arcs := map[int][2]float64{0: {0, 10}, 1: {0, 10}}
	stem := [][2]float64{{0, 0}, {3, 0}, {3, 10}, {0, 10}}
	for _, op := range []srf.SurfaceCombineAs{srf.SurfaceUnion, srf.SurfaceDiff, srf.SurfaceInter} {
		pa := meridian(t, ball, arcs, tol)
		pb := turnedTo(meridian(t, stem, nil, tol), math.Pi/3)
		sa, _ := turned(pa, 0, 2*math.Pi, tol)
		sb := &srf.SShell{Tol: tol}
		sb.MakeFromRevolutionOf(&pb.Sets[0], v(0, 0, 0), v(0, 0, 1), core.NewRGBA(100, 100, 100, 255), noGroup{})
		sh := &srf.SShell{Tol: tol}
		sh.MakeFromBoolean(sa, sb, op)
		want := sound(t, sh)
		rev, err := topo.JoinedRevolution(v(0, 0, 0), v(0, 0, 1),
			[]topo.RevolvedPart{{Profile: pa, Combine: srf.SurfaceUnion}, {Profile: pb, Combine: op}}, tol)
		if err != nil {
			t.Fatalf("%v: %v", op, err)
		}
		again, _ := turned(rev.Profile, 0, 2*math.Pi, tol)
		if got := sound(t, again); math.Abs(got-want) > 0.002*want {
			t.Errorf("%v: the joined profile turned is %g, the lathes joined %g", op, got, want)
		}
		if op != srf.SurfaceUnion {
			continue
		}
		// Hollowed, open at the stem's foot: a ball 1 thick on a tube.
		tp := build(t, sh)
		out, err := topo.ShellRevolved(sh, rev, facesOn(t, tp, v(0, 0, 0), v(0, 0, 1)), 1, 1<<30)
		if err != nil {
			t.Fatalf("ball on a stem: %v", err)
		}
		// What a ball of radius r about z = 10 leaves of a cylinder of
		// radius c from z = 0 up into it.
		below := func(r, c float64) float64 {
			return 2 * math.Pi * (5*c*c + (math.Pow(r*r-c*c, 1.5)-r*r*r)/3)
		}
		walls := 4*math.Pi/3*(125-64) + below(5, 3) - below(4, 2)
		if got := sound(t, out); math.Abs(got-walls) > 0.002*walls {
			t.Errorf("ball on a stem hollowed: volume %g, want %g", got, walls)
		}
	}
}
