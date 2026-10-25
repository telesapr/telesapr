package topo_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// lathed turns the outline pts, in the XZ plane at x >= 0, a full turn
// about the z axis, at the tolerance tol: a lathe's curves take their
// points when it is built.
func lathed(pts []geom.Vector, tol srf.Tolerance) *srf.SShell {
	sh := srf.SShell{Tol: tol}
	var ls srf.SBezierLoopSet
	for i := range pts {
		ls.Outer.Curves = append(ls.Outer.Curves, srf.SBezierFrom2(pts[i], pts[(i+1)%len(pts)]))
	}
	ls.Normal = v(0, 1, 0)
	sh.MakeFromRevolutionOf(&ls, v(0, 0, 0), v(0, 0, 1), core.NewRGBA(100, 100, 100, 255), nil)
	return &sh
}

// frustum is a cone of radius r0 at its foot and r1 at its top, h high,
// lathed at tol.
func frustum(r0, r1, h float64, tol srf.Tolerance) *srf.SShell {
	return lathed([]geom.Vector{v(0, 0, 0), v(r0, 0, 0), v(r1, 0, h), v(0, 0, h)}, tol)
}

// countersunk is a 40 x 40 x 10 plate with a hole through it that
// widens from radius r0 at its foot to r1 at its top, built at tol.
func countersunk(r0, r1 float64, tol srf.Tolerance) *srf.SShell {
	plate, out := srf.SShell{Tol: tol}, srf.SShell{Tol: tol}
	plate.MakePrism([]geom.Vector{v(-20, -20, 0), v(20, -20, 0), v(20, 20, 0), v(-20, 20, 0)}, v(0, 0, 0), v(0, 0, 10),
		core.NewRGBA(100, 100, 100, 255))
	// The tool runs past the plate along the same cone.
	k := (r1 - r0) / 10
	tool := lathed([]geom.Vector{v(0, 0, -1), v(r0-k, 0, -1), v(r1+k, 0, 11), v(0, 0, 11)}, tol)
	out.MakeFromDifferenceOf(&plate, tool)
	return &out
}

// revolvedCut is the volume a fillet of radius r (or with flat a
// chamfer at distance r) takes from, or adds at, a circular edge of
// radius rad, by Pappus (a chamfer cuts at r along both faces): f and g are the directions into the two faces
// in the half plane through the axis, as (along the radius, along the
// axis), and the section lies between them.
func revolvedCut(rad, r float64, f, g [2]float64, flat bool) float64 {
	beta := math.Acos(f[0]*g[0] + f[1]*g[1])
	s := r / math.Tan(beta/2)
	if flat {
		s = r
	}
	p := [2]float64{rad, 0}
	a := [2]float64{p[0] + s*f[0], p[1] + s*f[1]}
	b := [2]float64{p[0] + s*g[0], p[1] + s*g[1]}
	if flat {
		area := s * s * math.Sin(beta) / 2
		return area * 2 * math.Pi * (p[0] + a[0] + b[0]) / 3
	}
	bis := [2]float64{f[0] + g[0], f[1] + g[1]}
	l := math.Hypot(bis[0], bis[1])
	bis = [2]float64{bis[0] / l, bis[1] / l}
	d := r / math.Sin(beta/2)
	c := [2]float64{p[0] + d*bis[0], p[1] + d*bis[1]}
	// The kite p, a, c, b, less the sector at c between a and b.
	kite := r * s
	kiteX := ((p[0]+a[0]+c[0])/3 + (p[0]+c[0]+b[0])/3) / 2
	alpha := (math.Pi - beta) / 2
	sector := alpha * r * r
	sectorX := c[0] - bis[0]*2*r*math.Sin(alpha)/(3*alpha)
	return 2 * math.Pi * (kite*kiteX - sector*sectorX)
}

func unit(x, y float64) [2]float64 {
	l := math.Hypot(x, y)
	return [2]float64{x / l, y / l}
}

// A lathed frustum's top and foot, rounded and chamfered, and a
// countersunk hole's rim: where a flat face meets a cone.
func TestFilletCone(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name   string
		sh     func(tol srf.Tolerance) *srf.SShell
		at     geom.Vector
		rad, r float64
		f, g   [2]float64
		flat   bool
	}{
		// The top: in the top's frame, the cone runs out and down.
		{"top", func(tol srf.Tolerance) *srf.SShell { return frustum(10, 6, 8, tol) }, v(6*math.Cos(1), 6*math.Sin(1), 8), 6, 1.5, unit(-1, 0), unit(4, -8), false},
		{"top chamfer", func(tol srf.Tolerance) *srf.SShell { return frustum(10, 6, 8, tol) }, v(6*math.Cos(1), 6*math.Sin(1), 8), 6, 1.5, unit(-1, 0), unit(4, -8), true},
		{"overhang", func(tol srf.Tolerance) *srf.SShell { return frustum(6, 10, 8, tol) }, v(10*math.Cos(1), 10*math.Sin(1), 8), 10, 1, unit(-1, 0), unit(-4, -8), false},
		// The foot, whose flat face looks down: the cone rises in, the axis
		// pointing down.
		{"foot", func(tol srf.Tolerance) *srf.SShell { return frustum(10, 6, 8, tol) }, v(10*math.Cos(1), 10*math.Sin(1), 0), 10, 2, unit(-1, 0), unit(-4, -8), false},
		{"countersink", func(tol srf.Tolerance) *srf.SShell { return countersunk(3, 8, tol) }, v(8*math.Cos(1), 8*math.Sin(1), 10), 8, 1, unit(1, 0), unit(-5, -10), false},
	} {
		tol := 0.002 * c.r
		sh := c.sh(srf.Tol(tol, 20))
		before := meshVolume(sh)
		if err := topo.FilletPicks(sh, []topo.FilletPick{{At: c.at, Arc: true}}, c.r, 1<<30, c.flat); err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		want := revolvedCut(c.rad, c.r, c.f, c.g, c.flat)
		got := before - sound(t, sh)
		if allow := tol * 2 * math.Pi * (c.rad + c.r) * 4 * c.r; math.Abs(got-want) > allow {
			t.Errorf("%s: took %g, want %g within %g", c.name, got, want, allow)
		}
	}
}

// onPlate is a frustum standing on a 40 x 40 x 5 plate, joined to it,
// built at tol.
func onPlate(r0, r1, h float64, tol srf.Tolerance) *srf.SShell {
	plate, cone, out := srf.SShell{Tol: tol}, srf.SShell{Tol: tol}, srf.SShell{Tol: tol}
	plate.MakePrism([]geom.Vector{v(-20, -20, 0), v(20, -20, 0), v(20, 20, 0), v(-20, 20, 0)}, v(0, 0, 0), v(0, 0, 5),
		core.NewRGBA(100, 100, 100, 255))
	// From under the plate's top, along the same cone.
	k := (r1 - r0) / h
	cone.MakeFromTransformationOf(frustum(r0-k, r1, h+1, tol), v(0, 0, 4), geom.QuaternionIdentity, 1)
	out.MakeFromUnionOf(&plate, &cone)
	return &out
}

// TestFilletConeSweep rounds or chamfers where a flat face meets a cone:
// a frustum's top or foot, narrowing or widening, a countersunk hole's
// rim, and a frustum's foot on a plate (concave, filled), over the
// cone's slope and the radius, against Pappus's volumes; the rate
// allowed, 0, was set before.
func TestFilletConeSweep(t *testing.T) {
	if testing.Short() {
		t.Skip("a sweep")
	}
	t.Parallel()
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("edge", "top", "foot", "countersink", "on plate"),
			sweep.Strings("cut", "fillet", "chamfer"),
			sweep.Floats("slope", -0.6, -0.2, 0.3, 0.8),
			sweep.Floats("k", 0.1, 0.4, 0.8),
		},
		MaxFailRate: 0,
		Sample:      12,
		Parallel:    true,
	}
	grid.Run(func(p sweep.Point) error {
		slope, k := p.Float("slope"), p.Float("k")
		flat := p.String("cut") == "chamfer"
		// The cone's radius changes by slope times its height, 8.
		r0, h := 8.0, 8.0
		r1 := r0 + slope*h
		var build func(tol srf.Tolerance) *srf.SShell
		var at geom.Vector
		var rad float64
		var f, g [2]float64
		sign := 1.0
		switch p.String("edge") {
		case "top":
			build, rad, at = func(tol srf.Tolerance) *srf.SShell { return frustum(r0, r1, h, tol) }, r1, v(r1*math.Cos(1), r1*math.Sin(1), h)
			f, g = unit(-1, 0), unit(-slope, -1)
		case "foot":
			build, rad, at = func(tol srf.Tolerance) *srf.SShell { return frustum(r0, r1, h, tol) }, r0, v(r0*math.Cos(1), r0*math.Sin(1), 0)
			f, g = unit(-1, 0), unit(slope, -1)
		case "countersink":
			// The hole widens upwards, or narrows for a negative slope.
			lo, hi := 6.0, 6+math.Abs(slope)*10
			if slope < 0 {
				lo, hi = hi, lo
			}
			build, rad, at = func(tol srf.Tolerance) *srf.SShell { return countersunk(lo, hi, tol) }, hi, v(hi*math.Cos(1), hi*math.Sin(1), 10)
			f, g = unit(1, 0), unit(lo-hi, -10)
		default:
			build, rad, at = func(tol srf.Tolerance) *srf.SShell { return onPlate(r0, r1, h, tol) }, r0, v(r0*math.Cos(1), r0*math.Sin(1), 5)
			f, g = unit(1, 0), unit(slope, 1)
			sign = -1
		}
		// The largest cut that fits: its tangent circles inside the faces.
		beta := math.Acos(f[0]*g[0] + f[1]*g[1])
		room := math.Min(rad, 4) * math.Tan(beta/2)
		if g[1] != 0 {
			room = math.Min(room, 4*math.Tan(beta/2))
		}
		r := k * room
		if flat {
			r = k * math.Min(rad, 4)
		}
		// The mesh fine and its segments many: the cone's own chords, before
		// and after, would else count for more than a small cut.
		tol := 0.0005
		// Built at the tolerance: a lathe's curves take their points then.
		sh := build(srf.Tol(tol, 1000))
		before := meshVolume(sh)
		if err := topo.FilletPicks(sh, []topo.FilletPick{{At: at, Arc: true}}, r, 1<<30, flat); err != nil {
			return err
		}
		var want float64
		if flat {
			want = sign * revolvedCut(rad, r, f, g, true)
		} else {
			want = sign * revolvedCut(rad, r, f, g, false)
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
		// The chords cut inside the curved faces, a cone of about 12 by 12
		// and the cut, before and after.
		allow := tol * 2 * (2*math.Pi*12*12 + 2*math.Pi*(rad+r)*2*r)
		switch {
		case !rep.Sound():
			return fmt.Errorf("unsound: %s", rep.String())
		case math.Abs(before-rep.Volume-want) > allow:
			return fmt.Errorf("took %g, want %g within %g", before-rep.Volume, want, allow)
		}
		return nil
	}).Require(t)
}
