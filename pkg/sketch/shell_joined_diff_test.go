package sketch_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// A block with a round pocket turned into its bottom, from below it, open
// at its top: the inside is the block's less the pocket's cutter grown by
// the thickness, so a wall of the thickness stands round the pocket and
// over its floor. (Extruded from the block's bottom, the cutter is flush
// with it, and at this tolerance any Boolean after such a cut leaves
// naked edges round the pocket's mouth: the kernel's, not the shell's.)
func TestShellBlockWithPocket(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Edit.SetChordTolerance(0.005)
	body := block(d, -20, -20, 20, 20, 20)
	d.Features.SketchIn(d.Workplane("ZX"), "")
	axis := d.Sketch.Line(api.V(0, 0, -4), api.V(0, 0, 12))
	axis.SetConstruction(true)
	d.Sketch.Polyline(true, api.V(0, 0, -2), api.V(5, 0, -2), api.V(5, 0, 10), api.V(0, 0, 10))
	pocket := d.Features.Lathe(axis, "")
	pocket.SetCombine(sketch.CombineDifference)
	if p := pocket.Problem(); p != "" {
		t.Fatal(p)
	}
	step(d)
	before := saved(t, d)
	top := body.Solid().FaceAt(api.V(15, 15, 20))
	sh := d.Features.ShellSolid([]api.SolidFace{top}, 1, "")
	if p := sh.Problem(); p != "" {
		t.Fatal(p)
	}
	want := 32000 - 250*math.Pi - (38*38*19 - 36*math.Pi*10)
	if r := sh.Analyze().Check(); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("shelled block with a pocket: %s, want volume %g", r.String(), want)
	}
	undoRedo(t, d, before)
}

// A turned cone intersected with a block that cuts a flat down its side,
// open at the flat: the inside is the cone moved in, the flat's plane
// moved out past the flat, so the inside runs through it.
func TestShellConeCutFlat(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Edit.SetChordTolerance(0.005)
	d.Features.SketchIn(d.Workplane("ZX"), "")
	axis := d.Sketch.Line(api.V(0, 0, 0), api.V(0, 0, 24))
	axis.SetConstruction(true)
	d.Sketch.Polyline(true, api.V(0, 0, 2), api.V(10, 0, 2), api.V(0, 0, 22))
	d.Features.Lathe(axis, "")
	d.Features.SketchIn(d.Workplane("XY"), "")
	cut := block(d, -20, -20, 5, 20, 25)
	cut.SetCombine(sketch.CombineIntersection)
	if p := cut.Problem(); p != "" {
		t.Fatal(p)
	}
	flat := cut.Solid().FaceAt(api.V(5, 0, 4))
	sh := d.Features.ShellSolid([]api.SolidFace{flat}, 1, "")
	if p := sh.Problem(); p != "" {
		t.Fatal(p)
	}
	// The disc of radius r on the side x < 5 of the flat.
	part := func(r float64) float64 {
		if r <= 0 {
			return 0
		}
		if r <= 5 {
			return math.Pi * r * r
		}
		return math.Pi*r*r/2 + chordArea(r, 5)
	}
	const n = 20000
	want := 0.0
	for i := range n {
		z := 2 + 20*(float64(i)+0.5)/n
		r := 10 - (z-2)/2
		want += part(r) * 20 / n
		if z > 3 {
			want -= part(r-math.Sqrt(5)/2) * 20 / n
		}
	}
	if r := sh.Analyze().Check(); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("shelled cone cut flat: %s, want volume %g", r.String(), want)
	}
}

// A block with a half turn revolved down into its top and taken away,
// open at its bottom: the cutter is grown by the thickness, its ends'
// plane moved out too, so a wall of the thickness stands round the cut.
func TestShellBlockWithHalfTurnCut(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Edit.SetChordTolerance(0.005)
	body := block(d, -20, -20, 20, 20, 20)
	d.Features.SketchIn(d.Workplane("ZX"), "")
	axis := d.Sketch.Line(api.V(0, 0, 12), api.V(0, 0, 28))
	axis.SetConstruction(true)
	d.Sketch.Polyline(true, api.V(0, 0, 15), api.V(5, 0, 15), api.V(5, 0, 25), api.V(0, 0, 25))
	cut := d.Features.Revolve(axis, 180, "")
	cut.SetCombine(sketch.CombineDifference)
	if p := cut.Problem(); p != "" {
		t.Fatal(p)
	}
	step(d)
	before := saved(t, d)
	bottom := body.Solid().FaceAt(api.V(15, 15, 0))
	sh := d.Features.ShellSolid([]api.SolidFace{bottom}, 1, "")
	if p := sh.Problem(); p != "" {
		t.Fatal(p)
	}
	want := 32000 - 62.5*math.Pi - (38*38*19 - 5*(18*math.Pi+chordArea(6, 1)))
	if r := sh.Analyze().Check(); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("shelled block with a half turn cut: %s, want volume %g", r.String(), want)
	}
	undoRedo(t, d, before)
}

// A block with a round pocket turned into its bottom, open at the
// pocket's floor, which lies on the lathe taken away: the lathe's floor
// is moved into the pocket, and the inside reaches through it round a
// ring of the thickness.
func TestShellBlockOpenAtLathedFloor(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Edit.SetChordTolerance(0.005)
	block(d, -20, -20, 20, 20, 20)
	d.Features.SketchIn(d.Workplane("ZX"), "")
	axis := d.Sketch.Line(api.V(0, 0, -4), api.V(0, 0, 12))
	axis.SetConstruction(true)
	d.Sketch.Polyline(true, api.V(0, 0, -2), api.V(5, 0, -2), api.V(5, 0, 10), api.V(0, 0, 10))
	pocket := d.Features.Lathe(axis, "")
	pocket.SetCombine(sketch.CombineDifference)
	if p := pocket.Problem(); p != "" {
		t.Fatal(p)
	}
	floor := pocket.Solid().FaceAt(api.V(1, 1, 10))
	sh := d.Features.ShellSolid([]api.SolidFace{floor}, 1, "")
	if p := sh.Problem(); p != "" {
		t.Fatal(p)
	}
	want := 32000 - 250*math.Pi - (38*38*18 - 36*math.Pi*8 - 25*math.Pi)
	if r := sh.Analyze().Check(); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("shelled block open at a lathed pocket's floor: %s, want volume %g", r.String(), want)
	}
}
