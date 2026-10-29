package sketch_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// chordArea is ∫ √(r² - y²) dy from -h to h.
func chordArea(r, h float64) float64 {
	prim := func(y float64) float64 { return (y*math.Sqrt(r*r-y*y) + r*r*math.Asin(y/r)) / 2 }
	return prim(h) - prim(-h)
}

// lathedCylinder turns a rectangle in ZX about the Z axis: a cylinder of
// radius r from z 0 to h.
func lathedCylinder(d *api.Doc, r, h float64) api.Group {
	d.Features.SketchIn(d.Workplane("ZX"), "")
	axis := d.Sketch.Line(api.V(0, 0, -2), api.V(0, 0, h+2))
	axis.SetConstruction(true)
	d.Sketch.Polyline(true, api.V(0, 0, 0), api.V(r, 0, 0), api.V(r, 0, h), api.V(0, 0, h))
	return d.Features.Lathe(axis, "")
}

// A turned disc with a block standing up through its middle, open at
// the block's top: the inside is the disc's and the block's joined,
// with no wall between them, the block's foot on the disc's bottom a
// wall too.
func TestShellLatheAndBlock(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Edit.SetChordTolerance(0.005)
	lathedCylinder(d, 10, 10)
	d.Features.SketchIn(d.Workplane("XY"), "")
	blk := block(d, -3, -3, 3, 3, 16)
	if p := blk.Problem(); p != "" {
		t.Fatal(p)
	}
	step(d)
	before := saved(t, d)
	top := blk.Solid().FaceAt(api.V(0, 0, 16))
	sh := d.Features.ShellSolid([]api.SolidFace{top}, 1, "")
	if p := sh.Problem(); p != "" {
		t.Fatal(p)
	}
	want := math.Pi*100*10 + 36*6 - (math.Pi*81*8 + 16*7)
	if r := sh.Analyze().Check(); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("shelled disc and block: %s, want volume %g", r.String(), want)
	}
	undoRedo(t, d, before)
}

// A turned cylinder with a square boss out of its side, the boss drawn
// from the axis, open at the cylinder's top: the boss's foot, inside the
// cylinder, is moved out, so the boss's inside runs into the
// cylinder's.
func TestShellLatheAndBoss(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Edit.SetChordTolerance(0.005)
	cyl := lathedCylinder(d, 10, 20)
	d.Features.SketchIn(d.Workplane("YZ"), "")
	d.Sketch.Rect(api.V(0, -3, 7), api.V(0, 3, 13))
	boss := d.Features.Extrude(15, "")
	if p := boss.Problem(); p != "" {
		t.Fatal(p)
	}
	top := cyl.Solid().FaceAt(api.V(1, 1, 20))
	sh := d.Features.ShellSolid([]api.SolidFace{top}, 1, "")
	if p := sh.Problem(); p != "" {
		t.Fatal(p)
	}
	outer := math.Pi*100*20 + 6*(15*6-chordArea(10, 3))
	inner := math.Pi*81*19 + 4*(14*4-chordArea(9, 2))
	want := outer - inner
	if r := sh.Analyze().Check(); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("shelled cylinder and boss: %s, want volume %g", r.String(), want)
	}
}

// A plate with a turned post sunk into it, open at the post's top: the
// post's foot moved out by the thickness stays in the plate, but leaves
// a wall below it thinner than the thickness, so the shell is refused;
// sunk less deep, the wall keeps the thickness and the shell is made.
func TestShellLatheThinWall(t *testing.T) {
	t.Parallel()
	shell := func(foot float64) func() api.Group {
		d := api.New()
		d.Edit.SetChordTolerance(0.005)
		d.Features.SketchIn(d.Workplane("XY"), "")
		block(d, -20, -20, 20, 20, 4)
		d.Features.SketchIn(d.Workplane("ZX"), "")
		axis := d.Sketch.Line(api.V(0, 0, -2), api.V(0, 0, foot+12))
		axis.SetConstruction(true)
		d.Sketch.Polyline(true, api.V(0, 0, foot), api.V(3, 0, foot), api.V(3, 0, foot+10), api.V(0, 0, foot+10))
		post := d.Features.Lathe(axis, "")
		if p := post.Problem(); p != "" {
			t.Fatal(p)
		}
		top := post.Solid().FaceAt(api.V(1, 1, foot+10))
		return func() api.Group { return d.Features.ShellSolid([]api.SolidFace{top}, 1, "") }
	}
	// The foot at 1.5 moved out to 0.5, half the thickness over the
	// plate's bottom.
	refusedWith(t, "leaves a wall thinner", func() { shell(1.5)() })
	sh := shell(2.5)()
	if p := sh.Problem(); p != "" {
		t.Fatal(p)
	}
	want := 40*40*4 - 38*38*2 + math.Pi*(9*8.5-4*9.5)
	if r := sh.Analyze().Check(); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("shelled plate and post: %s, want volume %g", r.String(), want)
	}
}

// A block with a round boss standing on it, both extrusions from one
// sketch's plane, open at the boss's top: a tube down into the block's
// inside. Neither part is turned: the block is hollowed as a solid of
// flat faces, the boss as a prism.
func TestShellBlockAndRoundBoss(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Edit.SetChordTolerance(0.005)
	block(d, -10, -10, 10, 10, 10)
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Circle(api.V(0, 0, 0), 4)
	boss := d.Features.Extrude(16, "")
	top := boss.Solid().FaceAt(api.V(1, 1, 16))
	sh := d.Features.ShellSolid([]api.SolidFace{top}, 1, "")
	if p := sh.Problem(); p != "" {
		t.Fatal(p)
	}
	want := 4000 + math.Pi*16*6 - (18*18*8 + math.Pi*9*7)
	if r := sh.Analyze().Check(); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("shelled block and boss: %s, want volume %g", r.String(), want)
	}
}

// A turned cup with a lug revolved a sixth of a turn about its axis,
// from inside the cup's wall out past it, open at the cup's top: the
// lug's inner side, inside the cup, is moved out, its outer side and top
// and bottom in, and its inside cut back by its ends' planes moved in.
func TestShellLatheAndLug(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Edit.SetChordTolerance(0.005)
	cup := lathedCylinder(d, 10, 10)
	d.Features.SketchIn(d.Workplane("ZX"), "")
	axis := d.Sketch.Line(api.V(0, 0, -2), api.V(0, 0, 12))
	axis.SetConstruction(true)
	d.Sketch.Polyline(true, api.V(8, 0, 2), api.V(14, 0, 2), api.V(14, 0, 6), api.V(8, 0, 6))
	lug := d.Features.Revolve(axis, 60, "")
	if p := lug.Problem(); p != "" {
		t.Fatal(p)
	}
	top := cup.Solid().FaceAt(api.V(1, 1, 10))
	sh := d.Features.ShellSolid([]api.SolidFace{top}, 1, "")
	if p := sh.Problem(); p != "" {
		t.Fatal(p)
	}
	// The lug's inside past the cup's, r from 9 to 13, less a strip 1
	// wide beside each end, 2 high.
	const n = 20000
	area := 0.0
	for i := range n {
		r := 9 + 4*(float64(i)+0.5)/n
		area += r * (math.Pi/3 - 2*math.Asin(1/r)) * 4 / n
	}
	want := math.Pi*100*10 + math.Pi/6*(196-100)*4 - (math.Pi*81*9 + 2*area)
	if r := sh.Analyze().Check(); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("shelled cup and lug: %s, want volume %g", r.String(), want)
	}
}

// A round prism whose side lies wholly inside a turned cylinder with a
// bevelled top, its ends on the cylinder's: the side can't be moved out
// as a prism's, and the shell is refused, naming it.
func TestShellJoinedBuriedSide(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Features.SketchIn(d.Workplane("ZX"), "")
	axis := d.Sketch.Line(api.V(0, 0, -2), api.V(0, 0, 22))
	axis.SetConstruction(true)
	d.Sketch.Polyline(true, api.V(0, 0, 0), api.V(10, 0, 0), api.V(10, 0, 18), api.V(8, 0, 20), api.V(0, 0, 20))
	cyl := d.Features.Lathe(axis, "")
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Circle(api.V(0, 0, 0), 4)
	pin := d.Features.Extrude(20, "")
	if p := pin.Problem(); p != "" {
		t.Fatal(p)
	}
	top := cyl.Solid().FaceAt(api.V(6, 0, 20))
	refusedWith(t, "a side of a prism lies inside", func() { d.Features.ShellSolid([]api.SolidFace{top}, 1, "") })
}
