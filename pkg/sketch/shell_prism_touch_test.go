package sketch_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// blockWithBump extrudes, from one sketch, a 40 × 40 square whose side
// at x 20 bulges out round a circle of radius 3 about (x, 0): a prism
// whose offset inside, at a thickness of 1, is the square of 38 and the
// circle of 2, touching at x 21.
func blockWithBump(d *api.Doc, x float64) api.Group {
	d.Edit.SetChordTolerance(0.005)
	s := math.Sqrt(9 - (x-20)*(x-20))
	d.Sketch.Polyline(false, api.V(20, s, 0), api.V(20, 20, 0), api.V(-20, 20, 0), api.V(-20, -20, 0),
		api.V(20, -20, 0), api.V(20, -s, 0))
	d.Sketch.Arc(api.V(x, 0, 0), api.V(20, -s, 0), api.V(20, s, 0))
	return d.Features.Extrude(10, "")
}

// A prism whose inside touches itself, its offset circle meeting its
// offset side at one point, is refused: the walls either side would
// meet along a line, which is no closed solid. With the circle crossing
// the side, the inside is one and the shell is made.
func TestShellPrismInsideTouches(t *testing.T) {
	t.Parallel()
	shell := func(x float64) api.Group {
		d := api.New()
		ex := blockWithBump(d, x)
		if p := ex.Problem(); p != "" {
			t.Fatal(p)
		}
		return d.Features.ShellSolid([]api.SolidFace{ex.Solid().FaceAt(api.V(-10, -10, 10))}, 1, "")
	}
	refusedWith(t, "touches itself", func() { shell(21) })
	sh := shell(20.5)
	if p := sh.Problem(); p != "" {
		t.Fatal(p)
	}
	// beyond is the part of a circle of radius r past a chord d from its
	// centre, on the centre's side.
	beyond := func(r, d float64) float64 {
		return math.Pi*r*r - (r*r*math.Acos(d/r) - d*math.Sqrt(r*r-d*d))
	}
	// The outline: the square and the circle of 3 past x 20. The inside,
	// 9 deep: the square of 38 and the circle of 2 past x 19.
	want := (1600+beyond(3, 0.5))*10 - (38*38+beyond(2, 1.5))*9
	if r := sh.Analyze().Check(); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Errorf("shelled block with a bump: %s, want volume %g", r.String(), want)
	}
}

// The same touch made by a block and a post joined, both extruded from
// one plane: refused, where it was made unsound.
func TestShellBlockAndPostInsideTouches(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Edit.SetChordTolerance(0.005)
	blk := block(d, -20, -20, 20, 20, 10)
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Circle(api.V(21, 0, 0), 3)
	if p := d.Features.Extrude(10, "").Problem(); p != "" {
		t.Fatal(p)
	}
	refusedWith(t, "no closed solid", func() {
		d.Features.ShellSolid([]api.SolidFace{blk.Solid().FaceAt(api.V(-10, -10, 10))}, 1, "")
	})
}
