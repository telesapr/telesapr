package sketch_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// ringOnBlock is a block 20 by 20 and 10 high beside the Z axis, x from
// 2 to 22, and a ring 5 to 10 out and 10 high revolved about the axis by
// degrees from the ZX plane, away from the block, so the ring's end in
// that plane stands on the block's side and the other is in the air, a
// wall. It returns the block's group and the ring's.
func ringOnBlock(d *api.Doc, degrees float64) (block, ring api.Group) {
	turn := func(d *api.Doc, deg float64) api.Group {
		d.Features.SketchIn(d.Workplane("ZX"), "")
		axis := d.Sketch.Line(api.V(0, 0, -2), api.V(0, 0, 12))
		axis.SetConstruction(true)
		d.Sketch.Polyline(true, api.V(5, 0, 0), api.V(10, 0, 0), api.V(10, 0, 10), api.V(5, 0, 10))
		return d.Features.Revolve(axis, deg, "")
	}
	// Which way a revolve turns: a quarter turn in a drawing of its own.
	lo, _ := turn(api.New(), 90).Analyze().Bounds()
	// The block on the side the ring turns away from.
	y0, y1 := -20.0, 0.0
	if lo.Y < -1 {
		y0, y1 = 0, 20
	}
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Rect(api.V(2, y0, 0), api.V(22, y1, 0))
	block = d.Features.Extrude(10, "")
	return block, turn(d, degrees)
}

// A block with a ring revolved three quarters of a turn or a half turn
// on its side, open at the block's top: the ring's end on the block is
// moved out into it, the end in the air a wall, which runs on round the
// axis. The block's top and the ring's are one face, in one plane, so
// both are open: the ring's inside is 6 to 9 out and 1 to 10 high, past
// the planes of its ends moved: its area in the ring's plan by rows, the
// ring turned from +x towards +y and the block below y = 0.
func TestShellRingOnBlock(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		degrees float64
		in      func(x, y float64) bool
	}{
		{270, func(x, y float64) bool { return !(y < -1 && x > 0) && !(math.Abs(x) <= 1 && y <= 0) }},
		{180, func(x, y float64) bool { return (x > 1 && y > -1) || (x <= 1 && y > 1) }},
	} {
		d := api.New()
		d.Edit.SetChordTolerance(0.005)
		b, ring := ringOnBlock(d, c.degrees)
		if p := ring.Problem(); p != "" {
			t.Fatalf("%g degrees: %s", c.degrees, p)
		}
		y := -10.0
		if lo, _ := b.Analyze().Bounds(); lo.Y > -1 {
			y = 10
		}
		top := b.Solid().FaceAt(api.V(12, y, 10))
		sh := d.Features.ShellSolid([]api.SolidFace{top}, 1, "")
		if p := sh.Problem(); p != "" {
			t.Fatalf("%g degrees: %s", c.degrees, p)
		}
		const n = 2000
		area := 0.0
		for i := range n {
			for j := range n {
				x, y := -9+18*(float64(i)+0.5)/n, -9+18*(float64(j)+0.5)/n
				if r := math.Hypot(x, y); r >= 6 && r <= 9 && c.in(x, y) {
					area += 18 * 18 / float64(n*n)
				}
			}
		}
		solid := 4000 + 75*c.degrees*math.Pi/180/2*10
		want := solid - (18*18*9 + 9*area)
		if r := sh.Analyze().Check(); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
			t.Errorf("%g degrees: %s, want volume %g", c.degrees, r.String(), want)
		}
	}
}
