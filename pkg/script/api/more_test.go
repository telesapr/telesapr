package api

import (
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/ttf"
)

// ring draws a 10 by 10 square from 10 to 20 mm off a construction
// axis along y, and returns the axis.
func ring(d *Doc) Entity {
	axis := d.Line(V(0, 0, 0), V(0, 10, 0))
	axis.SetConstruction(true)
	d.Rect(V(10, 0, 0), V(20, 10, 0))
	return axis
}

// A revolve turns by the angle it is given.
func TestRevolveAngle(t *testing.T) {
	d := New()
	g := d.Revolve(ring(d), 90, "")
	// Pappus: the square's area times the arc its centre runs.
	want := 100 * 15 * math.Pi / 2
	if v := g.Volume(); math.Abs(v-want) > want*0.01 {
		t.Errorf("a quarter turn has the volume %g, want about %g", v, want)
	}
	if r := g.Check(); !r.Sound() {
		t.Errorf("the quarter turn is not sound: %s", r.String())
	}
}

// A helix turns by its angle and moves along its axis by its length.
func TestHelix(t *testing.T) {
	d := New()
	g := d.Helix(ring(d), 270, 30, "spring")
	if g.Kind() != "helix" || g.Name() != "spring" || d.Active().H != g.H {
		t.Fatalf("the helix is %v (%s)", g, g.Kind())
	}
	// Pappus holds for a profile swept along a helix about an axis in
	// its plane: area times the angle times the centre's radius.
	want := 100 * 15 * 3 * math.Pi / 2
	if v := g.Volume(); math.Abs(v-want) > want*0.01 {
		t.Errorf("the helix has the volume %g, want about %g", v, want)
	}
	if lo, hi := g.Bounds(); !near(hi.Y-lo.Y, 40) {
		t.Errorf("the helix is %g long along its axis, want 30 + 10", hi.Y-lo.Y)
	}
	if r := g.Check(); !r.Sound() {
		t.Errorf("the helix is not sound: %s", r.String())
	}
}

// Text is placed with its bottom left corner where it is asked, as
// high as asked, and as wide as its font makes it.
func TestText(t *testing.T) {
	d := New()
	text := d.Text("Abc", "", V(5, 5, 0), 10)
	if text.Kind() != "text" {
		t.Fatalf("the text is a %s", text.Kind())
	}
	aspect := ttf.Fonts.AspectRatio(ttf.EmbeddedFont, "Abc", false)
	pts := text.Points()
	if len(pts) != 4 {
		t.Fatalf("the text has %d points", len(pts))
	}
	for i, want := range []Vec{V(5, 15, 0), V(5, 5, 0), V(5+10*aspect, 5, 0), V(5+10*aspect, 15, 0)} {
		if p := pts[i].Pos(); !near(p.X, want.X) || !near(p.Y, want.Y) {
			t.Errorf("point %d of the text is at %v, want %v", i, p, want)
		}
	}
	if !d.Active().Solved() {
		t.Errorf("the text doesn't solve")
	}
}

// An image is construction, placed like text, as wide as its
// proportions.
func TestImage(t *testing.T) {
	d := New()
	path := filepath.Join("..", "..", "..", "testdata", "request", "line_segment", "normal.png")
	img := d.Image(path, V(0, 0, 0), 30)
	if img.Kind() != "image" || !img.Construction() {
		t.Fatalf("the image is a %s, construction %t", img.Kind(), img.Construction())
	}
	var aspect float64
	d.With(func(sk *sketch.Sketch) {
		for _, im := range sk.Images {
			aspect = float64(im.Width) / float64(im.Height)
		}
	})
	if p := img.Points()[2].Pos(); !near(p.X, 30*aspect) || !near(p.Y, 0) {
		t.Errorf("the image's bottom right corner is at %v, want (%g, 0)", p, 30*aspect)
	}
}

// A corner is rounded with a tangent arc.
func TestTangentArc(t *testing.T) {
	d := New()
	lines := d.Polyline(false, V(0, 0, 0), V(20, 0, 0), V(20, 20, 0))
	d.TangentArc(lines[0].End(), 5, true)
	kinds := map[string]int{}
	for _, e := range d.Active().Entities() {
		kinds[e.Kind()]++
	}
	if kinds["arc"] != 1 || kinds["line"] != 2 {
		t.Errorf("after rounding the corner the group has %v", kinds)
	}
	for _, e := range d.Active().Entities() {
		if e.Kind() == "arc" && !near(e.Radius(), 5) {
			t.Errorf("the arc's radius is %g, want 5", e.Radius())
		}
	}
	if !d.Undo() {
		t.Fatal("can't undo the tangent arc")
	}
	for _, e := range d.Active().Entities() {
		if e.Kind() == "arc" {
			t.Errorf("the arc is still there after undo")
		}
	}
}

// Two crossing lines are split into four.
func TestSplit(t *testing.T) {
	d := New()
	a := d.Line(V(-10, 0, 0), V(10, 0, 0))
	b := d.Line(V(0, -10, 0), V(0, 10, 0))
	d.Split(a, b, V(0, 0, 0))
	n := 0
	for _, e := range d.Active().Entities() {
		if e.Kind() == "line" {
			n++
		}
	}
	if n != 4 {
		t.Errorf("after splitting there are %d line segments, want 4", n)
	}
}

// An angle can measure the other, supplementary angle.
func TestOtherAngle(t *testing.T) {
	d := New()
	a := d.Line(V(0, 0, 0), V(10, 0, 0))
	b := d.Line(V(0, 0, 0), V(10, 10, 0))
	angle := d.Angle(a, b, 45)
	if angle.Other() {
		t.Fatal("a new angle measures the other angle")
	}
	angle.SetOther(true)
	if !angle.Other() || !near(angle.Value(), 135) {
		t.Errorf("the other angle is %g (other %t), want 135", angle.Value(), angle.Other())
	}
	if c := angle.Type(); c != core.ConstraintAngle {
		t.Errorf("the angle became a %v", c)
	}
}

// A screw thread on a rod is a sound solid, and its options reach it.
func TestThread(t *testing.T) {
	d := New()
	rod := d.Circle(V(0, 0, 0), 4)
	d.Extrude(30, "rod")
	before := len(d.Groups())
	// Made as a script's step, which is one step of undo.
	d.Host().(*SketchHost).BeginEval()
	g := d.Thread(rod, ThreadOptions{Length: 12})
	if len(d.Groups()) != before+4 || d.Active().H != g.H || g.Kind() != "helix" {
		t.Fatalf("the thread made %d groups, active %v (%s)", len(d.Groups())-before, d.Active(), g.Kind())
	}
	if !strings.HasPrefix(g.Name(), "M8-") {
		t.Errorf("the nearest thread to 8 mm is named %q, want M8", g.Name())
	}
	r := g.Check()
	if !r.Sound() {
		t.Fatalf("the thread is not sound: %s", r.String())
	}
	// The rod is turned down and the ridge added: less than the rod,
	// more than the core.
	rodVol, core := math.Pi*16*30, math.Pi*sq(3.2)*30
	if r.Volume >= rodVol || r.Volume <= core {
		t.Errorf("the threaded rod's volume is %g, want between %g and %g", r.Volume, core, rodVol)
	}
	if ThreadNearest(8) != "M8" || ThreadNearest(6.3) != "M6" {
		t.Errorf("nearest threads: %s %s", ThreadNearest(8), ThreadNearest(6.3))
	}
	if !d.Undo() || len(d.Groups()) != before {
		t.Errorf("undo leaves %d groups, want %d", len(d.Groups()), before)
	}

	// A nut: an internal thread bores the circle it is made on.
	n := New()
	outside := n.Circle(V(0, 0, 0), 7)
	n.Extrude(6.5, "nut")
	nut := n.Thread(outside, ThreadOptions{Designation: "M8", Internal: true, Through: true, Length: 6.5})
	if r := nut.Check(); !r.Sound() || r.Volume >= math.Pi*49*6.5 {
		t.Errorf("the nut is not sound, or not bored: %s", r.String())
	}

	// A left-hand screw.
	l := New()
	lrod := l.Circle(V(0, 0, 0), 4)
	l.Extrude(30, "rod")
	lh := l.Thread(lrod, ThreadOptions{Designation: "M8-LH", Length: 12})
	if r := lh.Check(); !r.Sound() || !strings.Contains(lh.Name(), "LH") {
		t.Errorf("the left-hand thread %q is not sound: %s", lh.Name(), r.String())
	}
}

func sq(x float64) float64 { return x * x }

// What can't be done panics with an error that says why.
func TestMisuseMore(t *testing.T) {
	d := New()
	line := d.Line(V(0, 0, 0), V(10, 0, 0))
	far := d.Line(V(0, 5, 0), V(10, 5, 0))
	length := d.Length(line, 10)
	for name, fn := range map[string]func(){
		"text 0 high":           func() { d.Text("x", "", V(0, 0, 0), 0) },
		"text in a lost font":   func() { d.Text("x", "no-such-font.ttf", V(0, 0, 0), 5) },
		"image 0 high":          func() { d.Image("x.png", V(0, 0, 0), 0) },
		"image not there":       func() { d.Image("no-such-image.png", V(0, 0, 0), 5) },
		"tangent arc radius":    func() { d.TangentArc(line.Start(), -1, false) },
		"tangent arc on a line": func() { d.TangentArc(line, 1, false) },
		"tangent arc no corner": func() { d.TangentArc(line.Start(), 1, false) },
		"split parallel lines":  func() { d.Split(line, far, V(0, 0, 0)) },
		"other of a length":     func() { length.SetOther(true) },
		"helix about a point":   func() { d.Helix(line.Start(), 360, 10, "") },
		"thread on a line":      func() { d.Thread(line, ThreadOptions{}) },
		"thread designation":    func() { d.Thread(d.Circle(V(0, 0, 0), 4), ThreadOptions{Designation: "Q9"}) },
		"thread length":         func() { d.Thread(d.Circle(V(0, 0, 0), 4), ThreadOptions{Length: -1}) },
		"text in 3d":            func() { d.Free3D(); d.Text("x", "", V(0, 0, 0), 5) },
	} {
		func() {
			defer func() {
				r := recover()
				if _, ok := r.(error); !ok {
					t.Errorf("%s: recovered %v, want an error", name, r)
				}
			}()
			fn()
		}()
	}
}

// A bare sketch keeps the chord tolerance a script sets, and meshes
// finer for a finer one.
func TestChordTolerance(t *testing.T) {
	d := New()
	d.Rect(V(0, 0, 0), V(10, 10, 0))
	d.Extrude(10, "")
	d.SketchIn(d.Workplane("XY"), "")
	d.Circle(V(5, 5, 0), 3)
	cut := d.Extrude(10, "")
	cut.SetCombine(Difference)
	coarse := cut.Check().Triangles
	d.SetChordTolerance(0.01)
	if d.ChordTolerance() != 0.01 {
		t.Fatalf("chord tolerance %g", d.ChordTolerance())
	}
	if fine := cut.Check().Triangles; fine <= coarse {
		t.Errorf("%d triangles at 0.01%%, %d at the default", fine, coarse)
	}
	func() {
		defer func() {
			if err, ok := recover().(error); !ok || !strings.Contains(err.Error(), "more than zero") {
				t.Errorf("a zero tolerance: %v", err)
			}
		}()
		d.SetChordTolerance(0)
	}()
}
