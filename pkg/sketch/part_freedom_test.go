package sketch_test

import (
	"errors"
	"math"
	"path/filepath"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// onAxis reports whether f turns about the upright through p, and only
// turns.
func onAxis(f api.PartFreedom, p api.Vec) bool {
	return f.Turns == 1 && f.Slides == 0 && f.Invariant == "axis" && !f.Moving &&
		near3(f.Dir, api.V(0, 0, 1)) && math.Hypot(f.Point.X-p.X, f.Point.Y-p.Y) < 1e-6
}

// The parts of a four-bar, closed by a point on a line, by a hinge, or
// driven by its crank: the crank turns about the ground's first pin and
// the rocker about its second, each a fixed axis; the coupler turns
// too, about a point that moves as the linkage does, which is no
// invariant. Driven, the counts are the same, the drive named.
func TestPartFreedomFourBar(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		o    fourBarOpts
	}{
		{"on a line", fourBarOpts{moving: true, ground: 60}},
		{"by a hinge", fourBarOpts{moving: true, ground: 60, closeByHinge: true}},
		{"driven", fourBarOpts{moving: true, driven: true, ground: 60}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, ground, crank, coupler, rocker, _ := buildFourBar(t, tc.o)
			ug := uprights(t, ground)
			var want []string
			if tc.o.driven {
				want = []string{"angle"}
			}
			for _, c := range []struct {
				g    api.Group
				axis *api.Vec
			}{{crank, new(ug[0].Start().Pos())}, {coupler, nil}, {rocker, new(ug[60].Start().Pos())}} {
				f := c.g.AsLink().Freedom()
				if !slices.Equal(f.DrivenBy, want) {
					t.Errorf("%s driven by %v, want %v", c.g.Name(), f.DrivenBy, want)
				}
				if c.axis != nil && !onAxis(f, *c.axis) {
					t.Errorf("%s: %+v, want a turn about the upright through %v", c.g.Name(), f, *c.axis)
				}
				if c.axis == nil && (f.Turns != 1 || f.Slides != 0 || !f.Moving || f.Invariant != "none") {
					t.Errorf("%s: %+v, want a turn about a moving axis", c.g.Name(), f)
				}
			}
		})
	}
}

// Each mate leaves a part its freedom: a pin in a hole (Concentric)
// turns about and slides along the hole's axis; with its face flush it
// only turns; flush alone it moves in the face's plane; a point on a
// point (a ball) turns about it; a screw screws along its axis, its
// lead a turn.
func TestPartFreedomMates(t *testing.T) {
	t.Parallel()
	all := func(api.Entity) bool { return true }
	for _, tc := range []struct {
		name          string
		mate          func(d *api.Doc, a, b api.Group) api.Vec
		turns, slides int
		invariant     string
		lead          float64
	}{
		{"concentric", func(d *api.Doc, a, b api.Group) api.Vec {
			d.Assembly.Concentric(first(t, a, "circle", all), first(t, b, "circle", all))
			return first(t, a, "circle", all).Center().Pos()
		}, 1, 1, "axis", 0},
		{"pin", func(d *api.Doc, a, b api.Group) api.Vec {
			d.Assembly.Concentric(first(t, a, "circle", all), first(t, b, "circle", all))
			_, topA := ends(a)
			botB, _ := ends(b)
			d.Assembly.Flush(topA, botB)
			return first(t, a, "circle", all).Center().Pos()
		}, 1, 0, "axis", 0},
		{"flush", func(d *api.Doc, a, b api.Group) api.Vec {
			_, topA := ends(a)
			botB, _ := ends(b)
			d.Assembly.Flush(topA, botB)
			return topA.FacePoint()
		}, 1, 2, "plane", 0},
		{"ball", func(d *api.Doc, a, b api.Group) api.Vec {
			p := first(t, a, "point", all)
			d.Constrain.Coincident(p, first(t, b, "point", all))
			return p.Pos()
		}, 2 + 1, 0, "point", 0},
		{"screw", func(d *api.Doc, a, b api.Group) api.Vec {
			d.Sketch.Anywhere3D()
			d.Assembly.Screw(first(t, a, "circle", all), first(t, b, "circle", all), 2)
			return first(t, a, "circle", all).Center().Pos()
		}, 1, 0, "axis", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d, a, b := matePart(t)
			at := tc.mate(d, a, b)
			f := b.AsLink().Freedom()
			if f.Turns != tc.turns || f.Slides != tc.slides || f.Invariant != tc.invariant || f.Moving ||
				math.Abs(math.Abs(f.Lead)-tc.lead) > 1e-6 {
				t.Fatalf("%+v; want %d turns, %d slides, a fixed %s, lead %g", f, tc.turns, tc.slides, tc.invariant, tc.lead)
			}
			off := at.Minus(f.Point)
			switch tc.invariant {
			case "axis":
				off = off.Cross(f.Dir)
			case "plane":
				off = f.Dir.ScaledBy(off.Dot(f.Dir))
			}
			if off.Magnitude() > 1e-6 {
				t.Errorf("%s through %v, %v: %v is %g off it", tc.invariant, f.Point, f.Dir, at, off.Magnitude())
			}
		})
	}
}

// The guide's lid, driven by angle and kept between 0 and 110°, turns
// about its hinge, driven by angle: the bounds are let go in counting,
// so that a lid held shut by its least angle still turns.
func TestPartFreedomLid(t *testing.T) {
	t.Parallel()
	d, err := api.Open(filepath.Join("..", "..", "testdata", "ui", "mechanism", "boxlid.tlspr"))
	if err != nil {
		t.Fatal(err)
	}
	gs := d.Groups()
	lid := gs[len(gs)-1]
	for _, angle := range []float64{0, 60} {
		setParam(t, d, "angle", angle)
		f := lid.AsLink().Freedom()
		if f.Turns != 1 || f.Slides != 0 || f.Invariant != "axis" || f.Moving || !slices.Equal(f.DrivenBy, []string{"angle"}) {
			t.Errorf("angle %g: %+v, want a turn about the hinge, driven by angle", angle, f)
		}
	}
}

// examples/scripts/engine's slider-crank: the crank turns about its
// bearing, the rod about an axis that moves, and the piston slides up
// the cylinder, all driven by the crank's angle, here and a quarter turn
// on.
func TestPartFreedomEngine(t *testing.T) {
	t.Parallel()
	d, crank, rod, piston, bearing := engine(t)
	if f := crank.AsLink().Freedom(); !onAxis(f, bearing) || !slices.Equal(f.DrivenBy, []string{"angle"}) {
		t.Errorf("crank: %+v, want a turn about the bearing at %v, driven", f, bearing)
	}
	if f := rod.AsLink().Freedom(); f.Turns != 1 || f.Slides != 0 || !f.Moving {
		t.Errorf("rod: %+v, want a turn about a moving axis", f)
	}
	f := piston.AsLink().Freedom()
	if f.Turns != 0 || f.Slides != 1 || f.Invariant != "direction" || !near3(f.Dir, api.V(0, 1, 0)) {
		t.Errorf("piston: %+v, want a slide along y", f)
	}
	if f.Text != "slides along (0, 1, 0), driven by angle" {
		t.Errorf("piston: %q", f.Text)
	}
	// A quarter turn on, the same.
	setParam(t, d, "angle", 90)
	if f := crank.AsLink().Freedom(); !onAxis(f, bearing) {
		t.Errorf("at 90°, crank: %+v", f)
	}
	if f := piston.AsLink().Freedom(); f.Invariant != "direction" || !near3(f.Dir, api.V(0, 1, 0)) {
		t.Errorf("at 90°, piston: %+v", f)
	}
}

// engine builds examples/scripts/engine's mechanism: a frame with a
// bearing and the cylinder's axis, held, and a crank, a rod and a
// piston moving in a mechanism, pinned and flush, the piston's pin on
// the cylinder's axis and its own line parallel to it in the XY
// workplane. It returns the bearing's centre.
func engine(t *testing.T) (d *api.Doc, crank, rod, piston api.Group, bearingAt api.Vec) {
	t.Helper()
	dir := t.TempDir()
	d = api.New()
	part := func(name string, draw func(p *api.Doc)) api.Group {
		p := api.New()
		draw(p)
		p.Features.Extrude(4, "")
		path := filepath.Join(dir, name+".tlspr")
		if err := p.File.Save(path); err != nil {
			t.Fatal(err)
		}
		g, err := d.Assembly.Link(path, name)
		if err != nil {
			t.Fatal(err)
		}
		return g
	}
	frame := part("frame", func(p *api.Doc) {
		p.Sketch.Rect(api.V(0, 0, 0), api.V(30, 120, 0))
		p.Sketch.Circle(api.V(15, 15, 0), 3)
		p.Sketch.Line(api.V(15, 40, 0), api.V(15, 115, 0)).SetConstruction(true)
	})
	crank = part("crank", func(p *api.Doc) {
		p.Sketch.Rect(api.V(0, 0, 0), api.V(24, 10, 0))
		p.Sketch.Circle(api.V(5, 5, 0), 3)
		p.Sketch.Circle(api.V(17, 5, 0), 3)
	})
	rod = part("rod", func(p *api.Doc) {
		p.Sketch.Rect(api.V(0, 0, 0), api.V(60, 10, 0))
		p.Sketch.Circle(api.V(5, 5, 0), 3)
		p.Sketch.Circle(api.V(55, 5, 0), 3)
	})
	piston = part("piston", func(p *api.Doc) {
		p.Sketch.Rect(api.V(0, 0, 0), api.V(16, 20, 0))
		p.Sketch.Circle(api.V(8, 10, 0), 3)
		p.Sketch.Line(api.V(8, 0, 0), api.V(8, 20, 0)).SetConstruction(true)
	})
	for _, g := range []api.Group{crank, rod, piston} {
		g.AsLink().SetMoves(true)
	}
	origin := func(g api.Group) api.Vec {
		x0, y0 := math.Inf(1), math.Inf(1)
		for _, e := range g.Entities() {
			if e.Kind() == "line" {
				x0, y0 = math.Min(x0, e.Start().Pos().X), math.Min(y0, e.Start().Pos().Y)
			}
		}
		return api.V(x0, y0, 0)
	}
	hole := func(g api.Group, x, y float64) api.Entity {
		o := origin(g)
		return first(t, g, "circle", func(e api.Entity) bool {
			c := e.Center().Pos()
			return math.Abs(c.X-o.X-x) < 1e-6 && math.Abs(c.Y-o.Y-y) < 1e-6 && math.Abs(c.Z) < 1e-6
		})
	}
	axisLine := func(g api.Group, x float64) api.Entity {
		o := origin(g)
		return first(t, g, "line", func(e api.Entity) bool {
			s, f := e.Start().Pos(), e.End().Pos()
			return math.Abs(s.X-o.X-x) < 1e-6 && math.Abs(f.X-o.X-x) < 1e-6 && math.Abs(s.Z) < 1e-6 &&
				math.Abs(f.Z) < 1e-6 && math.Abs(s.Y-f.Y) > 10
		})
	}
	turn := func(g api.Group, deg float64) {
		c, s := math.Cos(deg*math.Pi/180), math.Sin(deg*math.Pi/180)
		g.AsLink().SetOrientation(api.V(c, s, 0), api.V(-s, c, 0))
	}
	bearing, cylinder := hole(frame, 15, 15), axisLine(frame, 15)
	mainHole, crankPin := hole(crank, 5, 5), hole(crank, 17, 5)
	bigEnd, smallEnd := hole(rod, 5, 5), hole(rod, 55, 5)
	pistonPin, pistonAxis := hole(piston, 8, 10), axisLine(piston, 8)
	_, frameTop := ends(frame)
	crankBottom, crankTop := ends(crank)
	rodBottom, rodTop := ends(rod)
	pistonBottom, _ := ends(piston)
	turn(crank, 90)
	turn(rod, 80)
	d.Sketch.Anywhere3D()
	d.Assembly.HingeDriven(bearing, mainHole)
	d.Assembly.Flush(frameTop, crankBottom)
	d.Assembly.Concentric(crankPin, bigEnd)
	d.Assembly.Flush(crankTop, rodBottom)
	d.Assembly.Concentric(smallEnd, pistonPin)
	d.Assembly.Flush(rodTop, pistonBottom)
	d.Sketch.InWorkplane(d.Workplane("XY"))
	d.Constrain.On(pistonPin.Center(), cylinder)
	d.Constrain.Parallel(pistonAxis, cylinder)
	d.Sketch.Anywhere3D()
	if !piston.Solved() || piston.Dof() != 0 {
		t.Fatalf("the engine: solved %v, %d DOF", piston.Solved(), piston.Dof())
	}
	return d, crank, rod, piston, bearing.Center().Pos()
}

// Counting changes nothing: every param and every constraint's value
// is as it was, to the bit, and the parts are no less solved.
func TestPartFreedomChangesNothing(t *testing.T) {
	t.Parallel()
	d, _, crank, coupler, rocker, _ := fourBarDriven(t)
	d.With(func(sk *sketch.Sketch) {
		params := slices.Clone(sk.Params)
		var vals []float64
		for _, c := range sk.Constraints {
			vals = append(vals, c.ValA)
		}
		for _, g := range []api.Group{crank, coupler, rocker} {
			if _, err := sk.PartFreedoms(g.H); err != nil {
				t.Fatal(err)
			}
		}
		if !slices.Equal(params, sk.Params) {
			t.Error("counting the freedom changed the params")
		}
		for i, c := range sk.Constraints {
			if c.ValA != vals[i] {
				t.Errorf("counting the freedom changed %s from %g to %g", c.DescriptionString(), vals[i], c.ValA)
			}
		}
	})
	for _, g := range []api.Group{crank, coupler, rocker} {
		if !g.Solved() {
			t.Errorf("%s not solved after", g.Name())
		}
	}
}

// Only a solved link group has a freedom to count: a sketch is not a
// part, and a loop that cannot close is not solved.
func TestPartFreedomRefused(t *testing.T) {
	t.Parallel()
	open, _, crank, _, _, _ := buildFourBar(t, fourBarOpts{moving: true, ground: 200, closeByHinge: true})
	var err error
	open.With(func(sk *sketch.Sketch) { _, err = sk.PartFreedoms(crank.H) })
	if !errors.Is(err, sketch.ErrNotSolved) {
		t.Errorf("an open loop: %v, want %v", err, sketch.ErrNotSolved)
	}
	d := api.New()
	g := d.Features.Sketch3D("s")
	d.With(func(sk *sketch.Sketch) { _, err = sk.PartFreedoms(g.H) })
	if !errors.Is(err, sketch.ErrNotPart) {
		t.Errorf("a sketch: %v, want %v", err, sketch.ErrNotPart)
	}
	d.With(func(sk *sketch.Sketch) { _, err = sk.PartFreedoms(core.HGroup{V: 0x999}) })
	if !errors.Is(err, sketch.ErrNotPart) {
		t.Errorf("no group: %v, want %v", err, sketch.ErrNotPart)
	}
	for name, fn := range map[string]func(){
		"a sketch":  func() { g.AsLink().Freedom() },
		"open loop": func() { crank.AsLink().Freedom() },
	} {
		func() {
			defer func() {
				if _, ok := recover().(error); !ok {
					t.Errorf("Freedom of %s: no error", name)
				}
			}()
			fn()
		}()
	}
}
