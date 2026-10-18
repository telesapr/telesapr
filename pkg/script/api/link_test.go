package api

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinkStep(t *testing.T) {
	d := New()
	g, err := d.Assembly.Link(filepath.Join("..", "..", "..", "testdata", "step", "shapes", "cylinder.step"), "")
	if err != nil {
		t.Fatal(err)
	}
	if g.Name() != "cylinder" || g.Kind() != "link" {
		t.Errorf("group %s, a %s", g.Name(), g.Kind())
	}
	info := g.AsLink().Info()
	if len(info) != 3 || !strings.HasPrefix(info[0], "STEP AP214") || g.AsLink().Problem() != "" {
		t.Errorf("info %q, problem %q", info, g.AsLink().Problem())
	}
	r := g.Analyze().Check()
	if want := math.Pi * 25 * 12; !r.Sound() || math.Abs(r.Volume-want) > 0.01*want {
		t.Errorf("sound %v, volume %g, want %g", r.Sound(), r.Volume, want)
	}
	// A point at each of the two vertices and two centres, besides the
	// origin.
	points := 0
	for _, e := range g.Entities() {
		if e.Kind() == "point" {
			points++
		}
	}
	if points != 5 {
		t.Errorf("%d points", points)
	}

	csg, err := d.Assembly.Link(filepath.Join("..", "..", "..", "testdata", "step", "corpus", "ap203e2_csg.stp"), "csg")
	if err != nil {
		t.Fatal(err)
	}
	if p := csg.AsLink().Problem(); !strings.Contains(p, "CSG_SOLID") {
		t.Errorf("problem %q", p)
	}

	ext := d.Features.Sketch3D("not a link")
	for name, fn := range map[string]func(){
		"info of a sketch":    func() { ext.AsLink().Info() },
		"problem of a sketch": func() { ext.AsLink().Problem() },
	} {
		func() {
			defer func() {
				if _, ok := recover().(error); !ok {
					t.Errorf("%s: no error", name)
				}
			}()
			fn()
		}()
	}
}

// Linking a STEP file is one undo step: undo takes the link group and
// its mesh away, and redo brings them back, loaded again.
func TestLinkStepUndo(t *testing.T) {
	d := New()
	step := d.Host().(*SketchHost).BeginEval
	step()
	before := names(d.Groups())
	g, err := d.Assembly.Link(filepath.Join("..", "..", "..", "testdata", "step", "shapes", "cylinder.step"), "")
	if err != nil {
		t.Fatal(err)
	}
	after := names(d.Groups())
	volume := g.Analyze().Check().Volume
	if !d.Edit.Undo() {
		t.Fatal("no undo step")
	}
	if got := names(d.Groups()); got != before {
		t.Fatalf("undo left %q, want %q", got, before)
	}
	if !d.Edit.Redo() {
		t.Fatal("no redo step")
	}
	if got := names(d.Groups()); got != after {
		t.Fatalf("redo gave %q, want %q", got, after)
	}
	if v := d.Active().Analyze().Check().Volume; v != volume {
		t.Errorf("after redo the link's volume is %g, was %g", v, volume)
	}
}

// TestLinkPlacement moves and turns a linked part and checks the orientation it reports.
func TestLinkPlacement(t *testing.T) {
	d := New()
	g, err := d.Assembly.Link(filepath.Join("..", "..", "..", "testdata", "step", "shapes", "box.step"), "")
	if err != nil {
		t.Fatal(err)
	}
	l := g.AsLink()
	near := func(a, b Vec) bool { return a.Minus(b).Magnitude() < 1e-9 }
	if u, w := l.Orientation(); !near(u, V(1, 0, 0)) || !near(w, V(0, 1, 0)) {
		t.Fatalf("orientation of a new link %v %v", u, w)
	}
	l.Move(V(5, 0, -2))
	l.Move(V(1, 2, 3))
	if o := l.Offset(); !near(o, V(6, 2, 1)) {
		t.Errorf("offset %v", o)
	}
	l.Rotate90(V(0.1, 0.2, 3))
	if u, w := l.Orientation(); !near(u, V(0, 1, 0)) || !near(w, V(-1, 0, 0)) {
		t.Errorf("after a quarter about z: %v %v", u, w)
	}
	l.Rotate(V(0, 0, 1), 90)
	if u, _ := l.Orientation(); !near(u, V(-1, 0, 0)) {
		t.Errorf("after a half about z: %v", u)
	}
	l.SetOrientation(V(0, 0, 1), V(1, 0.5, 0))
	if u, w := l.Orientation(); !near(u, V(0, 0, 1)) || !near(w, V(1, 0.5, 0).WithMagnitude(1)) {
		t.Errorf("set orientation: %v %v", u, w)
	}

	for name, fn := range map[string]func(){
		"no axis":    func() { l.Rotate90(V(0, 0, 0)) },
		"nan axis":   func() { l.Rotate(V(math.NaN(), 0, 1), 5) },
		"parallel":   func() { l.SetOrientation(V(1, 0, 0), V(2, 0, 0)) },
		"infinite":   func() { l.Move(V(math.Inf(1), 0, 0)) },
		"not a link": func() { d.Groups()[0].AsLink().Move(V(1, 0, 0)) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: no error", name)
				}
			}()
			fn()
		}()
	}
}

// TestLinkModelInPlace links a model file and finds it where the file
// draws it, whatever the view: the window puts one beside what is
// drawn, a script where it was drawn.
func TestLinkModelInPlace(t *testing.T) {
	t.Parallel()
	part := New()
	part.Features.SketchIn(part.Workplane("XY"), "")
	part.Sketch.Polyline(true, V(10, 20, 0), V(14, 20, 0), V(14, 23, 0), V(10, 23, 0))
	part.Features.Extrude(2, "")
	path := filepath.Join(t.TempDir(), "part.tlspr")
	if err := part.File.Save(path); err != nil {
		t.Fatal(err)
	}
	d := New()
	g, err := d.Assembly.Link(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if o := g.AsLink().Offset(); o.Magnitude() != 0 {
		t.Errorf("offset %v", o)
	}
	lo, hi := g.Analyze().Bounds()
	if lo.Minus(V(10, 20, 0)).Magnitude() > 1e-9 || hi.Minus(V(14, 23, 2)).Magnitude() > 1e-9 {
		t.Errorf("bounds %v %v", lo, hi)
	}
}
