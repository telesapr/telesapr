package sketch_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/internal/golden"
	"github.com/spacesolve/spacesolve/pkg/internal/par"
	"github.com/spacesolve/spacesolve/pkg/script/api"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

// saved is the document's file.
func saved(t *testing.T, d *api.Doc) []byte {
	t.Helper()
	file := filepath.Join(t.TempDir(), "doc.spcs")
	if err := d.Save(file); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// roundTrip saves the document, opens the file and saves again, and
// requires the same bytes; it returns the opened document.
func roundTrip(t *testing.T, d *api.Doc) *api.Doc {
	t.Helper()
	file := filepath.Join(t.TempDir(), "doc.spcs")
	if err := d.Save(file); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(file)
	back, err := api.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	if again := saved(t, back); !bytes.Equal(first, again) {
		t.Fatalf("the file changed on a round trip:\n%s\n----\n%s", first, again)
	}
	return back
}

// undoRedo takes back the last step and does it again, requiring the
// file before the step after the undo and the same file after the
// redo.
func undoRedo(t *testing.T, d *api.Doc, before []byte) {
	t.Helper()
	after := saved(t, d)
	if !d.Undo() {
		t.Fatal("nothing to undo")
	}
	if got := saved(t, d); !bytes.Equal(got, before) {
		t.Fatalf("undo didn't restore the file:\n%s\n----\n%s", got, before)
	}
	if !d.Redo() {
		t.Fatal("nothing to redo")
	}
	if got := saved(t, d); !bytes.Equal(got, after) {
		t.Fatalf("redo didn't restore the file:\n%s\n----\n%s", got, after)
	}
}

// step starts an undo step, as a console line does.
func step(d *api.Doc) { d.Host().(*api.SketchHost).BeginEval() }

// forceToMesh builds g as a triangle mesh.
func forceToMesh(d *api.Doc, g api.Group) {
	d.With(func(sk *sketch.Sketch) {
		sk.GetGroup(g.H).ForceToMesh = true
		for i := range sk.Groups {
			sk.Groups[i].Clean = false
		}
		sk.ScheduleGenerateAll()
	})
	d.Regen()
}

// fineChord makes the document's meshes fine enough for volumes to
// come within a few parts in a thousand of the exact ones.
func fineChord(d *api.Doc) {
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0.002 })
}

// threeFeatures builds a model with a mirror, a chamfer and a sweep,
// and returns its file.
func threeFeatures(t *testing.T) []byte {
	d := api.New()
	d.SketchIn(d.Workplane("ZX"), "path")
	path := []api.Entity{
		d.Line(api.V(3, 0, 0), api.V(3, 0, 10)),
		d.Arc(api.V(9, 0, 10), api.V(3, 0, 10), api.V(9, 0, 16)),
	}
	d.SketchIn(d.Workplane("XY"), "profile")
	d.Rect(api.V(1, -2, 0), api.V(5, 2, 0))
	sw := d.Sweep(path, "")
	d.Mirror(d.Workplane("YZ"), "")
	c := d.Chamfer([]api.Entity{lineAlong(t, sw, api.V(1, -2, 0), api.V(5, -2, 0))}, 0.5, "")
	if r := c.Check(); !r.Sound() {
		t.Fatalf("three features: %s", r.String())
	}
	return saved(t, d)
}

// The features give the same file on one goroutine as on eight.
func TestSolidFeaturesParallel(t *testing.T) {
	old := par.SetWorkers(1)
	one := threeFeatures(t)
	par.SetWorkers(8)
	eight := threeFeatures(t)
	par.SetWorkers(old)
	if !bytes.Equal(one, eight) {
		t.Fatal("one goroutine and eight differ")
	}
}

// TestSolidGoldens builds a model for each variant of the mirror, the
// chamfer and the sweep, and compares its file with a golden in
// testdata/solid; scripts/regen-golden.sh solid rewrites them. Each
// golden opens and saves to the same bytes.
func TestSolidGoldens(t *testing.T) {
	variants := map[string]func(d *api.Doc){
		"mirror-apart": func(d *api.Doc) {
			block(d, 10, 0, 20, 10, 5)
			d.Mirror(d.Workplane("YZ"), "")
		},
		"mirror-face-reflection-only": func(d *api.Doc) {
			ex := block(d, 0, 0, 10, 10, 5)
			m := d.Mirror(ex.Faces()[0], "")
			m.SetKeepOriginal(false)
		},
		"mirror-pocket": func(d *api.Doc) {
			block(d, -20, 0, 20, 10, 5)
			d.SketchIn(d.Workplane("XY"), "")
			d.Circle(api.V(10, 5, 0), 2)
			d.Extrude(5, "").SetCombine(api.Difference)
			d.Mirror(d.Workplane("YZ"), "")
		},
		"chamfer-corner": func(d *api.Doc) {
			ex := block(d, 0, 0, 10, 10, 10)
			d.Chamfer([]api.Entity{
				lineAlong(t, ex, api.V(0, 0, 10), api.V(10, 0, 10)),
				lineAlong(t, ex, api.V(10, 0, 10), api.V(10, 10, 10)),
				lineAlong(t, ex, api.V(10, 0, 0), api.V(10, 0, 10)),
			}, 1, "")
		},
		"chamfer-parameter": func(d *api.Doc) {
			if err := d.SetParameter("c", "1.5"); err != nil {
				t.Fatal(err)
			}
			ex := block(d, 0, 0, 10, 10, 5)
			c := d.Chamfer([]api.Entity{lineAlong(t, ex, api.V(0, 0, 5), api.V(10, 0, 5))}, 1, "")
			if err := c.SetExpr(sketch.GroupPitch, "c*2"); err != nil {
				t.Fatal(err)
			}
		},
		"sweep-pipe": func(d *api.Doc) {
			path := pipePath(d, 10, 8, 12)
			d.SketchIn(d.Workplane("XY"), "profile")
			d.Circle(api.V(0, 0, 0), 2)
			d.Sweep(path, "")
		},
		"sweep-ring-down": func(d *api.Doc) {
			d.SketchIn(d.Workplane("ZX"), "path")
			path := []api.Entity{
				d.Line(api.V(0, 0, 0), api.V(0, 0, -10)),
				d.Arc(api.V(8, 0, -10), api.V(8, 0, -18), api.V(0, 0, -10)),
				d.Line(api.V(8, 0, -18), api.V(20, 0, -18)),
			}
			d.SketchIn(d.Workplane("XY"), "profile")
			d.Circle(api.V(0, 0, 0), 3)
			d.Circle(api.V(0, 0, 0), 2)
			d.Sweep(path, "")
		},
	}
	for name, build := range variants {
		t.Run(name, func(t *testing.T) {
			d := api.New()
			build(d)
			if p := d.Active().Problem(); p != "" {
				t.Fatalf("problem: %s", p)
			}
			if r := d.Active().Check(); !r.Sound() {
				t.Fatalf("unsound: %s", r.String())
			}
			roundTrip(t, d)
			golden.Check(t, filepath.Join("..", "..", "testdata", "solid", name+".spcs"), saved(t, d))
		})
	}
}

// benchRegen times a whole regeneration of the model build makes.
func benchRegen(b *testing.B, build func(d *api.Doc)) {
	d := api.New()
	build(d)
	for b.Loop() {
		d.With(func(sk *sketch.Sketch) {
			for i := range sk.Groups {
				sk.Groups[i].Clean = false
			}
			sk.ScheduleGenerateAll()
		})
		d.Regen()
	}
}

func BenchmarkMirror(b *testing.B) {
	benchRegen(b, func(d *api.Doc) {
		block(d, -20, 0, 20, 10, 5)
		d.SketchIn(d.Workplane("XY"), "")
		d.Circle(api.V(10, 5, 0), 2)
		d.Extrude(5, "").SetCombine(api.Difference)
		d.Mirror(d.Workplane("YZ"), "")
	})
}

func BenchmarkChamfer(b *testing.B) {
	benchRegen(b, func(d *api.Doc) {
		d.Rect(api.V(0, 0, 0), api.V(40, 30, 0))
		ex := d.Extrude(10, "")
		var top []api.Entity
		for _, e := range ex.Entities() {
			if e.Kind() == "line" && e.Start().Pos().Z == 10 && e.End().Pos().Z == 10 {
				top = append(top, e)
			}
		}
		d.Chamfer(top, 2, "")
	})
}

func BenchmarkSweep(b *testing.B) {
	benchRegen(b, func(d *api.Doc) {
		path := pipePath(d, 10, 8, 12)
		d.SketchIn(d.Workplane("XY"), "profile")
		d.Circle(api.V(0, 0, 0), 3)
		d.Circle(api.V(0, 0, 0), 2)
		d.Sweep(path, "")
	})
}
