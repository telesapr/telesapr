package api

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinkStep(t *testing.T) {
	d := New()
	g, err := d.Link(filepath.Join("..", "..", "..", "testdata", "step", "shapes", "cylinder.step"), "")
	if err != nil {
		t.Fatal(err)
	}
	if g.Name() != "cylinder" || g.Kind() != "link" {
		t.Errorf("group %s, a %s", g.Name(), g.Kind())
	}
	info := g.LinkInfo()
	if len(info) != 3 || !strings.HasPrefix(info[0], "STEP AP214") || g.LinkProblem() != "" {
		t.Errorf("info %q, problem %q", info, g.LinkProblem())
	}
	r := g.Check()
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

	csg, err := d.Link(filepath.Join("..", "..", "..", "testdata", "step", "corpus", "ap203e2_csg.stp"), "csg")
	if err != nil {
		t.Fatal(err)
	}
	if p := csg.LinkProblem(); !strings.Contains(p, "CSG_SOLID") {
		t.Errorf("problem %q", p)
	}

	ext := d.Sketch3D("not a link")
	for name, fn := range map[string]func(){
		"info of a sketch":    func() { ext.LinkInfo() },
		"problem of a sketch": func() { ext.LinkProblem() },
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
	g, err := d.Link(filepath.Join("..", "..", "..", "testdata", "step", "shapes", "cylinder.step"), "")
	if err != nil {
		t.Fatal(err)
	}
	after := names(d.Groups())
	volume := g.Check().Volume
	if !d.Undo() {
		t.Fatal("no undo step")
	}
	if got := names(d.Groups()); got != before {
		t.Fatalf("undo left %q, want %q", got, before)
	}
	if !d.Redo() {
		t.Fatal("no redo step")
	}
	if got := names(d.Groups()); got != after {
		t.Fatalf("redo gave %q, want %q", got, after)
	}
	if v := d.Active().Check().Volume; v != volume {
		t.Errorf("after redo the link's volume is %g, was %g", v, volume)
	}
}
