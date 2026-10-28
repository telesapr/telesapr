package sketch_test

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// frustumIn is the volume of the frustum the lathe of TestShellLathe
// keeps inside: below z = 8, above z = 1, and inside its cone moved in
// by 1, 8x + 4z = 80 - √80.
func frustumIn() float64 {
	c := 80 - math.Sqrt(80)
	r1, r2 := (c-4)/8, (c-32)/8
	return math.Pi * 7 / 3 * (r1*r1 + r1*r2 + r2*r2)
}

// A frustum turned by a lathe, open at its top, walls 1 thick: the
// inside is the profile offset, its cone moved in square to itself, and
// the volume a frustum's. The shell is one undo step and comes back
// from the file.
func TestShellLathe(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Edit.SetChordTolerance(0.005)
	d.Features.SketchIn(d.Workplane("ZX"), "")
	axis := d.Sketch.Line(api.V(0, 0, -2), api.V(0, 0, 12))
	axis.SetConstruction(true)
	d.Sketch.Polyline(true, api.V(0, 0, 0), api.V(10, 0, 0), api.V(6, 0, 8), api.V(0, 0, 8))
	lathe := d.Features.Lathe(axis, "")
	step(d)
	before := saved(t, d)
	top := lathe.Solid().FaceAt(api.V(1, 1, 8))
	sh := d.Features.ShellSolid([]api.SolidFace{top}, 1, "")
	if p := sh.Problem(); p != "" {
		t.Fatal(p)
	}
	want := math.Pi*8/3*(100+60+36) - frustumIn()
	r := sh.Analyze().Check()
	if !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("shelled frustum: %s, want volume %g", r.String(), want)
	}
	undoRedo(t, d, before)
	// The file holds the solid's curves at the sketch's tolerance, which
	// a reopened file has at its default.
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0; sk.ScheduleGenerateAll() })
	back := roundTrip(t, d)
	if br := back.Group(sh.Name()).Analyze().Check(); !br.Sound() || math.Abs(br.Volume-want) > 0.01*want {
		t.Fatalf("reopened: %s", br.String())
	}
	// Thicker than the frustum's top allows: a problem on the group.
	sh.AsShell().SetThickness(7)
	if p := sh.Problem(); !strings.Contains(p, "thickness") {
		t.Errorf("walls that meet: problem %q", p)
	}
}

// A rectangle revolved a quarter turn, open at its top, its ends kept:
// the inside is cut back by each end's plane moved in.
func TestShellRevolveQuarter(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Edit.SetChordTolerance(0.005)
	d.Features.SketchIn(d.Workplane("ZX"), "")
	axis := d.Sketch.Line(api.V(0, 0, -2), api.V(0, 0, 12))
	axis.SetConstruction(true)
	d.Sketch.Polyline(true, api.V(0, 0, 0), api.V(5, 0, 0), api.V(5, 0, 10), api.V(0, 0, 10))
	rev := d.Features.Revolve(axis, 90, "")
	if p := rev.Problem(); p != "" {
		t.Fatal(p)
	}
	// The top's middle, an eighth of a turn the way the revolve went.
	y := 2.0
	if lo, _ := rev.Analyze().Bounds(); lo.Y < -1 {
		y = -2
	}
	top := rev.Solid().FaceAt(api.V(2, y, 10))
	sh := d.Features.ShellSolid([]api.SolidFace{top}, 1, "")
	if p := sh.Problem(); p != "" {
		t.Fatal(p)
	}
	// The inside: of a quarter of the disc of radius 4, what lies 1 or
	// more from both its straight sides, 9 high.
	prim := func(x float64) float64 { return (x*math.Sqrt(16-x*x) + 16*math.Asin(x/4)) / 2 }
	in := (prim(math.Sqrt(15)) - prim(1) - (math.Sqrt(15) - 1)) * 9
	want := math.Pi*25*10/4 - in
	if r := sh.Analyze().Check(); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("shelled quarter: %s, want volume %g", r.String(), want)
	}
}

// A cone revolved three quarters of a turn, open at its base, its ends
// kept: they meet at the axis in a reflex corner, so the inside keeps
// the thickness from each end and is rounded about the axis. (A
// rectangle's revolve is a prism, which the prism's shell takes.)
func TestShellRevolveThreeQuarters(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Edit.SetChordTolerance(0.005)
	d.Features.SketchIn(d.Workplane("ZX"), "")
	axis := d.Sketch.Line(api.V(0, 0, -2), api.V(0, 0, 12))
	axis.SetConstruction(true)
	d.Sketch.Polyline(true, api.V(0, 0, 0), api.V(6, 0, 0), api.V(0, 0, 8))
	rev := d.Features.Revolve(axis, 270, "")
	if p := rev.Problem(); p != "" {
		t.Fatal(p)
	}
	base := rev.Solid().FaceAt(api.V(-2, 0, 0))
	sh := d.Features.ShellSolid([]api.SolidFace{base}, 1, "")
	if p := sh.Problem(); p != "" {
		t.Fatal(p)
	}
	if r := sh.Analyze().Check(); !r.Sound() || math.Abs(r.Volume-reflexConeShell()) > 0.002*r.Volume {
		t.Fatalf("shelled three quarters: %s, want volume %g", r.String(), reflexConeShell())
	}
}

// reflexConeShell is the volume of the walls of
// TestShellRevolveThreeQuarters: three quarters of the cone of radius 6
// and height 8, less the inside, the cone with its slant moved in by 1
// (8x + 6z = 38), cut back by a strip 1 wide beside each end and three
// quarters of a turn's disc of radius 1 about the axis between them,
// summed over its height.
func reflexConeShell() float64 {
	strip := func(r float64) float64 { return (math.Sqrt(r*r-1) + r*r*math.Asin(1/r)) / 2 }
	turn := 1.5 * math.Pi
	h := 38.0 / 6
	const n = 20000
	in := 0.0
	for i := range n {
		r := 38.0 / 8 * (1 - (float64(i)+0.5)/n)
		if r > 1 {
			in += turn/2*r*r - 2*strip(r) - (turn-math.Pi)/2
		}
	}
	return math.Pi*36*8/3*3/4 - in*h/n
}

// A lathe after which another solid is added is no longer one
// revolution: its shell is refused as a curved solid's.
func TestShellLatheAndMore(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Features.SketchIn(d.Workplane("ZX"), "")
	axis := d.Sketch.Line(api.V(0, 0, -2), api.V(0, 0, 12))
	axis.SetConstruction(true)
	d.Sketch.Polyline(true, api.V(0, 0, 0), api.V(10, 0, 0), api.V(6, 0, 8), api.V(0, 0, 8))
	d.Features.Lathe(axis, "")
	d.Features.SketchIn(d.Workplane("XY"), "")
	more := block(d, 20, 0, 30, 10, 5)
	defer func() {
		err, ok := recover().(error)
		if !ok || !strings.Contains(err.Error(), "curved") {
			t.Errorf("%v, want an error saying the solid is curved", err)
		}
	}()
	d.Features.ShellSolid([]api.SolidFace{more.Solid().FaceAt(api.V(25, 5, 5))}, 1, "")
}
