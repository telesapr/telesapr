package api

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/sweep"
)

// curvedPlate is a solid 4 thick on z = 0 whose top a circular fillet's
// foot runs off into a curved side, and the circle on the top it rounds.
type curvedPlate struct {
	name string
	// make draws the solid, with the rod or hole of radius rho centred
	// at c, and gives its last group and volume.
	make func(d *Doc) (Group, float64)
	// c and rho are the rounded circle's centre and radius; hole is set
	// where it is a hole's rim, not a rod's foot.
	c    Vec
	rho  float64
	hole bool
	// inside is the solid's outline seen from above.
	inside func(x, y float64) bool
}

// rodUp stands a rod of radius rho at (x, y) through the solid, from 6
// below its bottom to 6 above its top, and gives the volume it adds.
func rodUp(d *Doc, x, y, rho float64) (Group, float64) {
	d.Features.Sketch3D("under")
	d.Features.SketchIn(d.Sketch.NewWorkplane(V(0, 0, -6), V(1, 0, 0), V(0, 1, 0)), "")
	d.Sketch.Circle(V(x, y, -6), rho)
	return d.Features.Extrude(16, "rod"), math.Pi * rho * rho * 12
}

// curvedPlates are the solids the sweep tries: a disc of radius 8 with
// a rod of radius 4 off its centre, or a hole of radius 4, whose foot
// runs past the disc's rim; a block with a bore of radius 8 beside a
// rod of radius 3, whose foot runs into the bore; and a plate whose end
// is a half disc of radius 8, whose rim is one arc between flat sides.
var curvedPlates = []curvedPlate{
	{
		name: "disc rod", c: V(2, 0, 4), rho: 4,
		make: func(d *Doc) (Group, float64) {
			d.Features.SketchIn(d.Workplane("XY"), "")
			d.Sketch.Circle(V(0, 0, 0), 8)
			d.Features.Extrude(4, "disc")
			g, v := rodUp(d, 2, 0, 4)
			return g, math.Pi*64*4 + v
		},
		inside: func(x, y float64) bool { return x*x+y*y < 64 },
	},
	{
		name: "disc hole", c: V(2, 0, 4), rho: 4, hole: true,
		make: func(d *Doc) (Group, float64) {
			d.Features.SketchIn(d.Workplane("XY"), "")
			d.Sketch.Circle(V(0, 0, 0), 8)
			d.Sketch.Circle(V(2, 0, 0), 4)
			return d.Features.Extrude(4, "disc"), math.Pi * (64 - 16) * 4
		},
		inside: func(x, y float64) bool { return x*x+y*y < 64 },
	},
	{
		name: "bore rod", c: V(12, 0, 4), rho: 3,
		make: func(d *Doc) (Group, float64) {
			d.Features.SketchIn(d.Workplane("XY"), "")
			d.Sketch.Rect(V(-20, -20, 0), V(20, 20, 0))
			d.Sketch.Circle(V(0, 0, 0), 8)
			d.Features.Extrude(4, "block")
			g, v := rodUp(d, 12, 0, 3)
			return g, (1600-math.Pi*64)*4 + v
		},
		inside: func(x, y float64) bool { return x*x+y*y > 64 },
	},
	{
		name: "half disc rod", c: V(2, 0, 4), rho: 4,
		make: func(d *Doc) (Group, float64) {
			d.Features.SketchIn(d.Workplane("XY"), "")
			d.Sketch.Polyline(false, V(0, 8, 0), V(-8, 8, 0), V(-8, -8, 0), V(0, -8, 0))
			d.Sketch.Arc(V(0, 0, 0), V(0, -8, 0), V(0, 8, 0))
			d.Features.Extrude(4, "plate")
			g, v := rodUp(d, 2, 0, 4)
			return g, (128+math.Pi*32)*4 + v
		},
		inside: func(x, y float64) bool { return x < 0 && x > -8 && y > -8 && y < 8 || x >= 0 && x*x+y*y < 64 },
	},
}

// ringInside is the volume of the ring a fillet (or chamfer, flat) of
// size r adds round a circle of radius rho centred at (cx, cy) on the
// plane z = 4 (a rod's foot), or takes from it below (a hole's rim),
// within the solid's outline inside: its cross section integrated
// round the axis, at each radius over the angle inside the outline.
func ringInside(cx, cy, rho, r float64, flat bool, inside func(x, y float64) bool) float64 {
	const n, m = 2000, 1440
	h := r / n
	var v float64
	for i := range n {
		d := rho + (float64(i)+0.5)*h
		var turn float64
		for k := range m {
			phi := (float64(k) + 0.5) / m * 2 * math.Pi
			if inside(cx+d*math.Cos(phi), cy+d*math.Sin(phi)) {
				turn += 2 * math.Pi / m
			}
		}
		// The section's height at d: under the chamfer's line, or
		// outside the fillet's circle about (rho + r, r).
		s := r - (d - rho)
		if !flat {
			dr := d - (rho + r)
			s = r - math.Sqrt(r*r-dr*dr)
		}
		v += d * turn * h * s
	}
	return v
}

// A circular fillet or chamfer whose foot runs off its face into a side
// that is curved, a cylinder square to the face: past a disc's rim from
// a rod's foot or a hole's rim, into a bore beside a rod, past a half
// disc's arc between flat sides. Made as past a flat side: the face
// widened past the whole of the side's cylinder by a slab, the fillet
// made, and the widened solid cut back by the cylinder
// (fillet_past_side_curved.go); refused as running off the face before
// 2026-11-09. Every case sound and within 5e-4 of the ring's integral
// within the outline. The ordinary run takes each solid's fillet and
// chamfer at one size each, the size turning from solid to solid; -short
// a disc's rod fillet and a bore's chamfer; make test-sweeps all of them.
func TestFilletPastCurvedSide(t *testing.T) {
	t.Parallel()
	for i, p := range curvedPlates {
		for _, flat := range []bool{false, true} {
			for k, r := range []float64{2.2, 2.5, 3, 3.5} {
				if testing.Short() && !sweep.Full() && !(i == 0 && !flat && r == 2.5) && !(i == 2 && flat && r == 2.5) {
					continue
				}
				if !testing.Short() && !sweep.Full() && k != (i+2*b2i(flat))%4 {
					continue
				}
				what := map[bool]string{false: "fillet", true: "chamfer"}[flat]
				t.Run(fmt.Sprintf("%s/%s %g", p.name, what, r), func(t *testing.T) {
					t.Parallel()
					checkFilletCurved(t, p, flat, r)
				})
			}
		}
	}
}

// checkFilletCurved rounds p's circle at size r and checks the result is
// sound and holds the ring within p's outline.
func checkFilletCurved(t *testing.T, p curvedPlate, flat bool, r float64) {
	t.Helper()
	d := New()
	d.Edit.SetChordTolerance(0.003)
	base, vol := p.make(d)
	if msg := base.Problem(); msg != "" {
		t.Fatal(msg)
	}
	e := base.Solid().EdgeAt(p.c.Plus(V(p.rho, 0, 0)))
	msg, g := func() (msg string, g Group) {
		defer func() {
			if x := recover(); x != nil {
				msg = fmt.Sprint(x)
			}
		}()
		if flat {
			g = d.Features.ChamferSolid([]SolidEdge{e}, r, "")
		} else {
			g = d.Features.FilletSolid([]SolidEdge{e}, r, "")
		}
		return g.Problem(), g
	}()
	if msg != "" {
		t.Fatalf("not made: %s", msg)
	}
	if rep := g.Analyze().Check(); !rep.Sound() {
		t.Fatalf("unsound: %s", rep.Summary())
	}
	ring := ringInside(p.c.X, p.c.Y, p.rho, r, flat, p.inside)
	want := vol + ring
	if p.hole {
		want = vol - ring
	}
	if got := g.Analyze().Volume(); math.Abs(got-want)/want > 5e-4 {
		t.Errorf("volume %.4f, want %.4f (off by %.1e)", got, want, math.Abs(got-want)/want)
	}
}
