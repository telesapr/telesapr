package api

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
)

// guidedProfiles draws circles of radius 10 at z 0 and 40 and, in a
// sketch between them, a cubic from the bottom one's point (10, 0, 0)
// to the top one's (10, 0, 40), bowed out by bow; the top sketch is
// active.
func guidedProfiles(d *Doc, bow float64) (bottom, top Group, guide Entity) {
	bottom = d.SketchIn(d.Workplane("XY"), "bottom")
	d.Circle(V(0, 0, 0), 10)
	d.Sketch3D("guide")
	guide = d.Cubic(V(10, 0, 0), V(10+bow, 0, 13), V(10+bow, 0, 27), V(10, 0, 40))
	d.Sketch3D("")
	at := d.Point(V(0, 0, 40))
	top = d.SketchAt(at, V(1, 0, 0), V(0, 1, 0), "top")
	d.Circle(V(0, 0, 40), 10)
	return bottom, top, guide
}

// A loft held to a guide (roadmap 28.3): its sections move sideways
// with the guide, so its volume stays the cylinder's (Cavalieri) while
// it reaches out to the guide; undone with the loft; the same after
// saving and loading.
func TestLoftGuide(t *testing.T) {
	d := New()
	d.SetChordTolerance(0.005)
	bottom, top, guide := guidedProfiles(d, 8)
	loft := d.Loft(top, bottom, "duct")
	if got := loft.Guides(); len(got) != 0 {
		t.Fatalf("a new loft has guides %v", got)
	}
	loft.SetGuides([]Entity{guide})
	if p := loft.Problem(); p != "" {
		t.Fatal(p)
	}
	if got := loft.Guides(); len(got) != 1 || got[0].H != guide.H {
		t.Errorf("guides %v, want %v", got, guide)
	}
	r := loft.Check()
	want := math.Pi * 100 * 40
	if !r.Sound() || math.Abs(r.Volume-want) > 2e-3*want {
		t.Fatalf("the guided loft: %s, want volume %g", r.Summary(), want)
	}
	// The cubic reaches x = 16 half way up; the loft's far side with it.
	if _, hi := loft.Bounds(); hi.X < 15.9 {
		t.Errorf("the loft reaches only x = %g", hi.X)
	}
	path := filepath.Join(t.TempDir(), "duct.tlspr")
	if err := d.Save(path); err != nil {
		t.Fatal(err)
	}
	if !d.Undo() {
		t.Fatal("no undo")
	}
	if g := d.Groups(); len(g) == 0 || g[len(g)-1].Name() == "duct" {
		t.Error("the loft is still there after undo")
	}
	e := New()
	if err := e.Load(path); err != nil {
		t.Fatal(err)
	}
	e.SetChordTolerance(0.005)
	again := e.Group("duct")
	if len(again.Guides()) != 1 {
		t.Fatalf("loaded guides %v", again.Guides())
	}
	if r2 := again.Check(); !r2.Sound() || math.Abs(r2.Volume-r.Volume) > 1e-6*r.Volume {
		t.Errorf("loaded: %s, saved volume %g", r2.Summary(), r.Volume)
	}
	// None takes them away.
	again.SetGuides(nil)
	if len(again.Guides()) != 0 || again.Problem() != "" {
		t.Errorf("guides %v after taking them away, problem %q", again.Guides(), again.Problem())
	}
}

// A guide that leaves a profile is the loft's problem, with how far
// (the loft runs from the top, so the bottom is its last profile);
// one drawn after the loft is refused.
func TestLoftGuideProblems(t *testing.T) {
	d := New()
	bottom := d.SketchIn(d.Workplane("XY"), "bottom")
	d.Circle(V(0, 0, 0), 10)
	d.Sketch3D("stray")
	stray := d.Cubic(V(12, 0, 0), V(14, 0, 13), V(14, 0, 27), V(10, 0, 40))
	d.Sketch3D("")
	top := d.SketchAt(d.Point(V(0, 0, 40)), V(1, 0, 0), V(0, 1, 0), "top")
	d.Circle(V(0, 0, 40), 10)
	loft := d.Loft(top, bottom, "")
	loft.SetGuides([]Entity{stray})
	if p := loft.Problem(); !strings.Contains(p, "doesn't end on the last profile") || !strings.Contains(p, "2 mm") {
		t.Errorf("problem %q", p)
	}
	if r := loft.Check(); r.Sound() {
		t.Errorf("a loft whose guide is off its profile is sound: %s", r.Summary())
	}
	d.Sketch3D("late")
	late := d.Cubic(V(10, 0, 0), V(14, 0, 13), V(14, 0, 27), V(10, 0, 40))
	func() {
		defer func() {
			if err, ok := recover().(error); !ok || !strings.Contains(err.Error(), "after the loft") {
				t.Errorf("a guide drawn after the loft: %v", err)
			}
		}()
		loft.SetGuides([]Entity{late})
	}()
}

func TestLoftGuideMisuse(t *testing.T) {
	d := New()
	_, top, guide := guidedProfiles(d, 8)
	bottom := d.Group("bottom")
	loft := d.Loft(top, bottom, "")
	d.SketchIn(d.Workplane("XY"), "other")
	circle := d.Circle(V(0, 0, 0), 3)
	for name, fn := range map[string]func(){
		"guides of a sketch":     func() { top.SetGuides([]Entity{guide}) },
		"read a sketch's guides": func() { top.Guides() },
		"a circle as a guide":    func() { loft.SetGuides([]Entity{circle}) },
		"a guide that is gone":   func() { loft.SetGuides([]Entity{d.Entity(0x7fff0001)}) },
	} {
		func() {
			defer func() {
				if _, ok := recover().(error); !ok {
					t.Errorf("%s: recovered no error", name)
				}
			}()
			fn()
		}()
	}
}
