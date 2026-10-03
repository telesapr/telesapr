package sketch_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/internal/sweep"
	"github.com/spacesolve/spacesolve/pkg/script/api"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

// A 20 x 10 x 10 block open at the top, walls 1 thick: the inside is
// 18 x 8 x 9.
func TestShellOpenBox(t *testing.T) {
	d := api.New()
	ex := block(d, 0, 0, 20, 10, 10)
	top := ex.Solid().FaceAt(api.V(10, 5, 10))
	step(d)
	before := saved(t, d)
	sh := d.Shell([]api.Entity{top.Entity()}, 1, "")
	if p := sh.Problem(); p != "" {
		t.Fatal(p)
	}
	r := sh.Check()
	if want := 2000.0 - 18*8*9; !r.Sound() || math.Abs(r.Volume-want) > 1e-6 || sh.Kind() != "shell" {
		t.Fatalf("shelled block: %s, want volume %g", r.String(), want)
	}
	undoRedo(t, d, before)
	// It depends on the extrusion whose face it takes away.
	dependsOnEx := false
	for _, g := range sh.DependsOn() {
		dependsOnEx = dependsOnEx || g.Name() == ex.Name()
	}
	if !dependsOnEx {
		t.Fatalf("the shell depends on %v, not on %s", sh.DependsOn(), ex.Name())
	}
	if err := sh.SetExpr(sketch.GroupPitch, "1+1"); err != nil {
		t.Fatal(err)
	}
	d.Regen()
	if v := sh.Check().Volume; sh.Thickness() != 2 || math.Abs(v-(2000-16*6*8)) > 1e-6 {
		t.Fatalf("at thickness %g the volume is %g", sh.Thickness(), v)
	}
	if err := sh.SetExpr(sketch.GroupPitch, "0"); err == nil {
		t.Fatal("a thickness of 0 was taken")
	}
	back := roundTrip(t, d)
	bs := back.Group(sh.Name())
	if bs.Kind() != "shell" || bs.Expr(sketch.GroupPitch) != "1+1" || math.Abs(bs.Check().Volume-(2000-16*6*8)) > 1e-6 {
		t.Fatalf("reopened: %s, %q", bs.Kind(), bs.Expr(sketch.GroupPitch))
	}
	// The cavity's edges have names.
	s := sh.Solid()
	if len(s.Problems()) != 0 || len(s.Faces()) != 11 {
		t.Fatalf("topology: %v, %d faces", s.Problems(), len(s.Faces()))
	}
	for _, e := range s.Edges() {
		if e.Name() == "" {
			t.Fatal("an edge with no name")
		}
	}
}

// A block with a square hole cut through it, open at the top: the
// hole's walls are walls of the shell too, and its faces come from the
// Boolean, picked by where they are.
func TestShellHoledBlock(t *testing.T) {
	d := api.New()
	block(d, 0, 0, 30, 30, 10)
	d.SketchIn(d.Workplane("XY"), "")
	d.Rect(api.V(10, 10, 0), api.V(20, 20, 0))
	cut := d.Extrude(10, "")
	cut.SetCombine(api.Difference)
	sh := d.ShellSolid([]api.SolidFace{cut.Solid().FaceAt(api.V(5, 5, 10))}, 1, "")
	r := sh.Check()
	if want := 8000.0 - (28*28-12*12)*9; !r.Sound() || math.Abs(r.Volume-want) > 1e-6 {
		t.Fatalf("shelled block with a hole: %s, want volume %g", r.String(), want)
	}
}

func TestShellRefused(t *testing.T) {
	d := api.New()
	ex := block(d, 0, 0, 20, 10, 10)
	top := ex.Solid().FaceAt(api.V(10, 5, 10))
	for name, c := range map[string]struct {
		fn   func()
		want string
	}{
		"no faces":             {func() { d.Shell(nil, 1, "") }, "no faces"},
		"zero":                 {func() { d.Shell([]api.Entity{top.Entity()}, 0, "") }, "more than zero"},
		"walls meet":           {func() { d.Shell([]api.Entity{top.Entity()}, 5, "") }, "walls meet"},
		"not a face":           {func() { d.Shell([]api.Entity{ex.Entities()[0]}, 1, "") }, "not a face"},
		"thickness of extrude": {func() { ex.Thickness() }, "shell"},
	} {
		func() {
			defer func() {
				err, ok := recover().(error)
				if !ok || !strings.Contains(err.Error(), c.want) {
					t.Errorf("%s: %v, want an error saying %q", name, err, c.want)
				}
			}()
			c.fn()
		}()
	}
	d2 := api.New()
	d2.SketchIn(d2.Workplane("XY"), "")
	d2.Circle(api.V(0, 0, 0), 5)
	cyl := d2.Extrude(10, "")
	func() {
		defer func() {
			if err, ok := recover().(error); !ok || !strings.Contains(err.Error(), "curved") {
				t.Errorf("a cylinder: %v", err)
			}
		}()
		d2.ShellSolid([]api.SolidFace{cyl.Solid().FaceAt(api.V(0, 0, 10))}, 1, "")
	}()
}

// A shell whose walls stop fitting, as when the solid is made thinner,
// is a problem on the group, and the solid passes through.
func TestShellProblem(t *testing.T) {
	d := api.New()
	sides := d.Rect(api.V(0, 0, 0), api.V(10, 10, 0))
	width := d.Length(sides[1], 10)
	ex := d.Extrude(5, "")
	sh := d.Shell([]api.Entity{ex.Solid().FaceAt(api.V(5, 5, 5)).Entity()}, 2, "")
	width.Set(3)
	if p := sh.Problem(); !strings.Contains(p, "walls meet") {
		t.Fatalf("problem %q", p)
	}
	if v := sh.Check().Volume; math.Abs(v-3*10*5) > 1e-6 {
		t.Fatalf("with a problem the shell took %g away", 150-v)
	}
}

// Boxes opened at the top over their sizes and the thickness, through
// the group, against the inside's volume; the rate allowed, 0, was
// fixed before the group was built.
func TestShellGroupSweep(t *testing.T) {
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("width", 4, 20),
			sweep.Floats("height", 3, 30),
			sweep.Floats("k", 0.05, 0.3, 0.45),
		},
		MaxFailRate: 0,
		Sample:      6,
	}
	grid.Run(func(p sweep.Point) error {
		w, h := p.Float("width"), p.Float("height")
		thick := p.Float("k") * math.Min(math.Min(w, 10), h)
		d := api.New()
		ex := block(d, 0, 0, w, 10, h)
		sh := d.Shell([]api.Entity{ex.Solid().FaceAt(api.V(w/2, 5, h)).Entity()}, thick, "")
		r := sh.Check()
		want := w*10*h - (w-2*thick)*(10-2*thick)*(h-thick)
		switch {
		case sh.Problem() != "":
			return fmt.Errorf("problem: %s", sh.Problem())
		case !r.Sound():
			return fmt.Errorf("unsound: %s", r.String())
		case math.Abs(r.Volume-want) > 1e-6*want:
			return fmt.Errorf("volume %g, want %g", r.Volume, want)
		}
		return nil
	}).Require(t)
}
