package sketch_test

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/golden"
	"github.com/telesapr/telesapr/pkg/internal/par"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// saved is the document's file.
func saved(t *testing.T, d *api.Doc) []byte {
	t.Helper()
	file := filepath.Join(t.TempDir(), "doc.tlspr")
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
	file := filepath.Join(t.TempDir(), "doc.tlspr")
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
		sk.Group(g.H).ForceToMesh = true
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
		"chamfer-solid-edge": func(d *api.Doc) {
			block(d, 0, 0, 20, 10, 10)
			d.SketchIn(d.Workplane("XY"), "")
			d.Rect(api.V(5, -1, 0), api.V(15, 4, 0))
			d.Extrude(20, "").SetCombine(api.Difference)
			// Where the notch's side meets the top: no entity draws it.
			d.ChamferSolid([]api.SolidEdge{d.Active().Solid().EdgeAt(api.V(15, 2, 10))}, 1, "")
		},
		"chamfer-two-distances": func(d *api.Doc) {
			ex := block(d, 0, 0, 10, 10, 5)
			c := d.Chamfer([]api.Entity{lineAlong(t, ex, api.V(0, 0, 5), api.V(10, 0, 5))}, 1, "")
			c.SetSecondDistance(2)
		},
		"chamfer-two-distances-corner": func(d *api.Doc) {
			ex := block(d, 0, 0, 10, 10, 10)
			c := d.Chamfer([]api.Entity{
				lineAlong(t, ex, api.V(0, 0, 10), api.V(10, 0, 10)),
				lineAlong(t, ex, api.V(10, 0, 10), api.V(10, 10, 10)),
			}, 1, "")
			c.SetSecondDistance(2)
		},
		"draft-sides": func(d *api.Doc) {
			ex := block(d, 0, 0, 20, 10, 10)
			ex.SetDraft(8)
			ex.SetDraftNeutral(10)
			ex.SetDraftFaces([]api.Entity{lineAlong(t, ex, api.V(0, 0, 0), api.V(20, 0, 0))})
		},
		"draft-sharp-arc": func(d *api.Doc) {
			d.Arc(api.V(0, 0, 0), api.V(10, 0, 0), api.V(-10, 0, 0))
			d.Line(api.V(-10, 0, 0), api.V(10, 0, 0))
			d.Extrude(8, "").SetDraft(6)
		},
		"draft-lens": func(d *api.Doc) {
			d.Arc(api.V(0, 0, 0), api.V(5, 0, 0), api.V(0, 5, 0))
			d.Arc(api.V(5, 5, 0), api.V(0, 5, 0), api.V(5, 0, 0))
			d.Extrude(6, "").SetDraft(4)
		},
		"sweep-corner": func(d *api.Doc) {
			path := cornerPath(d, 10, 12, 60)
			d.SketchIn(d.Workplane("XY"), "profile")
			d.Circle(api.V(0, 0, 0), 2)
			d.Sweep(path, "")
		},
		"sweep-solid-edges": func(d *api.Doc) {
			block(d, 0, 0, 20, 10, 5)
			s := d.Active().Solid()
			front, right := s.EdgeAt(api.V(10, 0, 5)), s.EdgeAt(api.V(20, 5, 5))
			d.SketchIn(d.Workplane("YZ"), "profile")
			d.Rect(api.V(0, -0.5, 4.5), api.V(0, 0.5, 5.5))
			d.SweepEdges([]api.SolidEdge{front, right}, nil, "")
		},
		"loft-tangency": func(d *api.Doc) {
			bottom, top := twoProfiles(d, 10, 4, 6)
			d.Loft(bottom, top, "").SetTangency(1, 0.5)
			// The file keeps no chord tolerance (twoProfiles sets one).
			d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0 })
			d.Regen()
		},
		"loft-guides": func(d *api.Doc) {
			d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0.001 })
			bottom := d.SketchIn(d.Workplane("XY"), "bottom")
			d.Circle(api.V(0, 0, 0), 5)
			d.Sketch3D("guides")
			a := d.Cubic(api.V(5, 0, 0), api.V(9, 0, 4), api.V(9, 0, 8), api.V(5, 0, 12))
			b := d.Cubic(api.V(-5, 0, 0), api.V(-6, 0, 4), api.V(-6, 0, 8), api.V(-5, 0, 12))
			top := d.SketchAt(above(d, 12), api.V(1, 0, 0), api.V(0, 1, 0), "top")
			d.Circle(api.V(0, 0, 12), 5)
			d.Loft(bottom, top, "").SetGuides([]api.Entity{a, b})
			d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0 })
			d.Regen()
		},
		"pattern-ring": func(d *api.Doc) {
			d.SketchIn(d.Workplane("XY"), "ring")
			ring := d.Circle(api.V(0, 0, 0), 20)
			d.SketchIn(d.Workplane("XY"), "")
			// Beside the ring, not on it: the ring's line over a post's
			// bottom would take the hover from it (TestEveryFacePicked).
			d.Rect(api.V(21, -1, 0), api.V(23, 1, 0))
			d.Extrude(5, "post")
			d.PatternAlongPath([]api.Entity{ring}, 5, "")
		},
		"chamfer-of-chamfer": func(d *api.Doc) {
			ex := block(d, 0, 0, 10, 10, 10)
			c := d.Chamfer([]api.Entity{lineAlong(t, ex, api.V(0, 0, 10), api.V(10, 0, 10))}, 2, "")
			// The edge between the first chamfer's face, which no entity
			// names, and the front.
			d.ChamferSolid([]api.SolidEdge{c.Solid().EdgeAt(api.V(5, 0, 8))}, 0.5, "")
		},
		"fillet-edges": func(d *api.Doc) {
			ex := block(d, 0, 0, 20, 10, 10)
			d.Fillet([]api.Entity{
				lineAlong(t, ex, api.V(0, 0, 10), api.V(20, 0, 10)),
				lineAlong(t, ex, api.V(0, 10, 0), api.V(20, 10, 0)),
			}, 2, "")
		},
		"fillet-solid-edge": func(d *api.Doc) {
			block(d, 0, 0, 20, 10, 10)
			d.SketchIn(d.Workplane("XY"), "")
			d.Rect(api.V(5, -1, 0), api.V(15, 4, 0))
			d.Extrude(20, "").SetCombine(api.Difference)
			// The notch's upright edge, which no entity draws.
			d.FilletSolid([]api.SolidEdge{d.Active().Solid().EdgeAt(api.V(15, 0, 5))}, 1.5, "")
		},
		"loft-on-block": func(d *api.Doc) {
			block(d, -10, -10, 10, 10, 5)
			// A square on the block's top joined by its union to a
			// smaller one above it, turned a quarter, so its sides are
			// saddles.
			bottom := d.SketchAt(above(d, 5), api.V(1, 0, 0), api.V(0, 1, 0), "")
			d.Rect(api.V(-8, -8, 5), api.V(8, 8, 5))
			top := d.SketchAt(above(d, 15), api.V(1, 0, 0), api.V(0, 1, 0), "")
			d.Rect(api.V(-4, -4, 15), api.V(4, 4, 15))
			d.Loft(bottom, top, "").SetTurn(1)
		},
		"loft-circle-to-square": func(d *api.Doc) {
			bottom := d.SketchIn(d.Workplane("XY"), "")
			d.Circle(api.V(0, 0, 0), 5)
			top := d.SketchAt(above(d, 10), api.V(1, 0, 0), api.V(0, 1, 0), "")
			d.Rect(api.V(-3, -3, 10), api.V(3, 3, 10))
			d.Loft(bottom, top, "")
		},
		"fillet-mitre": func(d *api.Doc) {
			ex := block(d, 0, 0, 20, 10, 10)
			d.Fillet([]api.Entity{
				lineAlong(t, ex, api.V(0, 0, 10), api.V(20, 0, 10)),
				lineAlong(t, ex, api.V(20, 0, 10), api.V(20, 10, 10)),
			}, 2, "")
		},
		"fillet-corner-sphere": func(d *api.Doc) {
			ex := block(d, 0, 0, 20, 10, 10)
			d.Fillet([]api.Entity{
				lineAlong(t, ex, api.V(0, 0, 10), api.V(20, 0, 10)),
				lineAlong(t, ex, api.V(20, 0, 10), api.V(20, 10, 10)),
				lineAlong(t, ex, api.V(20, 0, 0), api.V(20, 0, 10)),
			}, 2, "")
		},
		"fillet-circle": func(d *api.Doc) {
			d.Circle(api.V(0, 0, 0), 5)
			ex := d.Extrude(10, "")
			d.Fillet([]api.Entity{circleOf(t, ex, 10)}, 2, "")
		},
		"fillet-pocket":  func(d *api.Doc) { filletPocket(d, 2) },
		"fillet-outline": func(d *api.Doc) { filletOutline(t, d) },
		"fillet-uneven":  func(d *api.Doc) { filletUneven(d) },
		"fillet-rib":     func(d *api.Doc) { filletRib(d, 3, true, true) },
		"fillet-cone":    func(d *api.Doc) { filletCone(d) },
		"fillet-tee":     func(d *api.Doc) { filletTee(d) },
		"fillet-radii":   func(d *api.Doc) { filletRadii(t, d) },
		"fillet-boss-foot": func(d *api.Doc) {
			block(d, -10, -10, 10, 10, 5)
			d.SketchAt(above(d, 5), api.V(1, 0, 0), api.V(0, 1, 0), "")
			d.Circle(api.V(0, 0, 5), 4)
			boss := d.Extrude(5, "")
			d.Fillet([]api.Entity{circleOf(t, boss, 5)}, 1.5, "")
		},
		"chamfer-circle-and-edge": func(d *api.Doc) {
			block(d, -10, -10, 10, 10, 5)
			d.SketchAt(above(d, 5), api.V(1, 0, 0), api.V(0, 1, 0), "")
			d.Circle(api.V(0, 0, 5), 4)
			boss := d.Extrude(5, "")
			s := boss.Solid()
			d.ChamferSolid([]api.SolidEdge{s.EdgeAt(api.V(4, 0, 10)), s.EdgeAt(api.V(0, -10, 5))}, 1, "")
		},
		"shell-open-box": func(d *api.Doc) {
			ex := block(d, 0, 0, 20, 10, 10)
			d.Shell([]api.Entity{ex.Solid().FaceAt(api.V(10, 5, 10)).Entity()}, 1, "")
		},
		"shell-holed-block": func(d *api.Doc) {
			block(d, 0, 0, 30, 30, 10)
			d.SketchIn(d.Workplane("XY"), "")
			d.Rect(api.V(10, 10, 0), api.V(20, 20, 0))
			cut := d.Extrude(10, "")
			cut.SetCombine(api.Difference)
			// Open at the top and at a side.
			s := cut.Solid()
			d.ShellSolid([]api.SolidFace{s.FaceAt(api.V(5, 5, 10)), s.FaceAt(api.V(15, 0, 5))}, 1.5, "")
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
			golden.Check(t, filepath.Join("..", "..", "testdata", "solid", name+".tlspr"), saved(t, d))
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

// BenchmarkFilletBox rounds all twelve edges of a block: eight spheres.
func BenchmarkFilletBox(b *testing.B) {
	benchRegen(b, func(d *api.Doc) {
		ex := block(d, 0, 0, 20, 10, 10)
		var edges []api.Entity
		for _, e := range ex.Entities() {
			if e.Kind() == "line" {
				edges = append(edges, e)
			}
		}
		d.Fillet(edges, 2, "")
	})
}

func BenchmarkFilletCircle(b *testing.B) {
	t := &testing.T{}
	benchRegen(b, func(d *api.Doc) {
		d.Circle(api.V(0, 0, 0), 5)
		ex := d.Extrude(10, "")
		d.Fillet([]api.Entity{circleOf(t, ex, 10)}, 2, "")
	})
}

// filletPocket is a block with a pocket in its top, every edge inside
// it filled at radius r: spheres in its corners.
func filletPocket(d *api.Doc, r float64) {
	block(d, 0, 0, 30, 20, 10)
	d.SketchAt(above(d, 10), api.V(1, 0, 0), api.V(0, 1, 0), "")
	d.Rect(api.V(5, 5, 10), api.V(25, 15, 10))
	cut := d.Extrude(-6, "")
	cut.SetCombine(api.Difference)
	s := cut.Solid()
	var edges []api.SolidEdge
	corners := []api.Vec{api.V(5, 5, 4), api.V(25, 5, 4), api.V(25, 15, 4), api.V(5, 15, 4)}
	for i, c := range corners {
		n := corners[(i+1)%4]
		edges = append(edges, s.EdgeAt(c.Plus(n).ScaledBy(0.5)), s.EdgeAt(api.V(c.X, c.Y, 7)))
	}
	d.FilletSolid(edges, r, "")
}

// BenchmarkFilletPocket fills the eight edges inside a pocket: four
// spheres.
func BenchmarkFilletPocket(b *testing.B) {
	benchRegen(b, func(d *api.Doc) { filletPocket(d, 2) })
}

// filletOutline is a block over a rectangle with its corners rounded,
// its top rounded all round from one side picked.
func filletOutline(t *testing.T, d *api.Doc) {
	sides := d.Rect(api.V(0, 0, 0), api.V(30, 20, 0))
	for _, s := range sides {
		d.TangentArc(s.Start(), 4, true)
	}
	ex := d.Extrude(10, "")
	d.Fillet([]api.Entity{lineAlong(t, ex, api.V(4, 0, 10), api.V(26, 0, 10))}, 2, "")
}

// BenchmarkFilletOutline rounds a rounded rectangle's top: four
// cylinders and four tori.
func BenchmarkFilletOutline(b *testing.B) {
	t := &testing.T{}
	benchRegen(b, func(d *api.Doc) { filletOutline(t, d) })
}

// filletUneven is a ramp, its top's front edge and its edge along the
// slope rounded: an uneven mitre.
func filletUneven(d *api.Doc) {
	d.SketchIn(d.Workplane("ZX"), "")
	d.Polyline(true, api.V(0, 0, 0), api.V(20, 0, 0), api.V(15, 0, 10), api.V(0, 0, 10))
	s := d.Extrude(10, "").Solid()
	slope := s.EdgeAt(api.V(15, 5, 10))
	if a, b := slope.Ends(); a.Y < -1 || b.Y < -1 {
		slope = s.EdgeAt(api.V(15, -5, 10))
	}
	d.FilletSolid([]api.SolidEdge{s.EdgeAt(api.V(7, 0, 10)), slope}, 2, "")
}

func BenchmarkFilletUneven(b *testing.B) {
	benchRegen(b, filletUneven)
}

// ribbed is a bracket, a plate 10 thick bent up by 110 degrees round an
// arc of radius 20 and 42 wide, with a rib 10 thick across the bend, a
// triangle from the floor to the wall; and the edges of the rib's foot:
// along each side the floor's, the bend's and the wall's, and the two
// ends of its sloping top.
func ribbed(d *api.Doc) (rib api.Group, edges func(s api.Solid, sides, ends bool) []api.SolidEdge) {
	const thick, bend, reach = 10.0, 20.0, 26.0
	up := api.V(math.Cos(110*math.Pi/180), math.Sin(110*math.Pi/180), 0)
	along := func(p api.Vec, k float64) api.Vec { return api.V(p.X+k*up.X, p.Y+k*up.Y, 0) }
	outer := along(api.V(0, 0, 0), 45)
	corner := api.V(thick*up.Y+(thick+thick*up.X)/up.Y*up.X, thick, 0)
	tangent := bend / math.Tan(55*math.Pi/180)
	onFloor, onWall := api.V(corner.X+tangent, thick, 0), along(corner, tangent)
	inner := api.V(outer.X+thick*up.Y, outer.Y-thick*up.X, 0)
	midBend := api.V(onFloor.X+bend*math.Cos(235*math.Pi/180), thick+bend+bend*math.Sin(235*math.Pi/180), 0)
	lines := []api.Entity{
		d.Line(outer, api.V(0, 0, 0)),
		d.Line(api.V(0, 0, 0), api.V(50, 0, 0)),
		d.Line(api.V(50, 0, 0), api.V(50, thick, 0)),
		d.Line(api.V(50, thick, 0), onFloor),
	}
	arc := d.Arc(api.V(onFloor.X, thick+bend, 0), onWall, onFloor)
	lines = append(lines, d.Line(onWall, inner), d.Line(inner, outer))
	for i := 0; i+1 < 4; i++ {
		d.Coincident(lines[i].End(), lines[i+1].Start())
	}
	d.Coincident(lines[3].End(), arc.End())
	d.Coincident(arc.Start(), lines[4].Start())
	d.Coincident(lines[4].End(), lines[5].Start())
	d.Coincident(lines[5].End(), lines[0].Start())
	d.Extrude(42, "").SetTwoSided(true)
	foot, top := api.V(reach, thick, 0), along(corner, reach-corner.X)
	d.SketchIn(d.Workplane("XY"), "")
	d.Polyline(true, top, corner, foot)
	rib = d.Extrude(thick, "")
	rib.SetTwoSided(true)
	d.Regen()
	edges = func(s api.Solid, sides, ends bool) []api.SolidEdge {
		var out []api.SolidEdge
		for _, z := range []float64{thick / 2, -thick / 2} {
			if !sides {
				break
			}
			a := along(onWall, 0.5)
			out = append(out, s.EdgeAt(api.V(onFloor.X+0.5, thick, z)), s.EdgeAt(api.V(midBend.X, midBend.Y, z)),
				s.EdgeAt(api.V(a.X, a.Y, z)))
		}
		if ends {
			out = append(out, s.EdgeAt(foot), s.EdgeAt(top))
		}
		return out
	}
	return rib, edges
}

// filletRib is the ribbed bracket with the rib's foot filled: along its
// sides, across its ends, or all round, where a side's fillet meets an
// end's below a sharp edge of the rib's top, in an uneven mitre that
// lies outside the rib.
func filletRib(d *api.Doc, r float64, sides, ends bool) api.Group {
	rib, edges := ribbed(d)
	return d.FilletSolid(edges(rib.Solid(), sides, ends), r, "")
}

func BenchmarkFilletRib(b *testing.B) {
	benchRegen(b, func(d *api.Doc) { filletRib(d, 3, true, true) })
}

// filletCone is a frustum turned by a lathe, its top's rim rounded where
// the top meets the cone, and its foot chamfered.
func filletCone(d *api.Doc) {
	d.SketchIn(d.Workplane("ZX"), "")
	axis := d.Line(api.V(0, 0, -2), api.V(0, 0, 10))
	axis.SetConstruction(true)
	d.Polyline(true, api.V(0, 0, 0), api.V(10, 0, 0), api.V(6, 0, 8), api.V(0, 0, 8))
	s := d.Lathe(axis, "").Solid()
	d.FilletSolid([]api.SolidEdge{s.EdgeAt(api.V(6*math.Cos(1), 6*math.Sin(1), 8))}, 1.5, "")
	d.ChamferSolid([]api.SolidEdge{s.EdgeAt(api.V(10*math.Cos(1), 10*math.Sin(1), 0))}, 1, "")
}

func BenchmarkFilletCone(b *testing.B) {
	benchRegen(b, filletCone)
}

// filletTee is a branch pipe joined to a pipe, the closed edge between
// the two cylinders filled round by a rolling ball (roadmap 22.2).
func filletTee(d *api.Doc) {
	d.SketchIn(d.Workplane("XY"), "")
	d.Circle(api.V(0, 0, 0), 10)
	d.Extrude(40, "")
	d.SketchIn(d.Workplane("YZ"), "")
	d.Circle(api.V(0, 0, 20), 5)
	s := d.Extrude(25, "").Solid()
	// The edge where they meet, the one whose points all lie between x 8
	// and 10: at the default chord tolerance its chords pass too far
	// from a point on it for EdgeAt.
	for _, e := range s.Edges() {
		in := true
		for _, p := range e.Points() {
			in = in && p.X > 8 && p.X < 10.01
		}
		if in {
			d.FilletSolid([]api.SolidEdge{e}, 2, "")
			return
		}
	}
}

func BenchmarkFilletTee(b *testing.B) {
	benchRegen(b, filletTee)
}

// filletRadii is a block's top edges rounded at the fillet's radius and
// its foot's at one of their own, in one group.
func filletRadii(t *testing.T, d *api.Doc) {
	ex := block(d, 0, 0, 20, 10, 10)
	var edges []api.Entity
	for _, z := range []float64{10, 0} {
		edges = append(edges,
			lineAlong(t, ex, api.V(0, 0, z), api.V(20, 0, z)), lineAlong(t, ex, api.V(20, 0, z), api.V(20, 10, z)),
			lineAlong(t, ex, api.V(20, 10, z), api.V(0, 10, z)), lineAlong(t, ex, api.V(0, 10, z), api.V(0, 0, z)))
	}
	f := d.Fillet(edges, 2, "")
	for i := 4; i < 8; i++ {
		f.SetEdgeRadius(i, 1)
	}
}

func BenchmarkFilletRadii(b *testing.B) {
	t := &testing.T{}
	benchRegen(b, func(d *api.Doc) { filletRadii(t, d) })
}

func BenchmarkShell(b *testing.B) {
	benchRegen(b, func(d *api.Doc) {
		ex := block(d, 0, 0, 20, 10, 10)
		d.Shell([]api.Entity{ex.Solid().FaceAt(api.V(10, 5, 10)).Entity()}, 1, "")
	})
}

// A cylinder hollowed into a cup (roadmap 15.2).
func BenchmarkShellCup(b *testing.B) {
	benchRegen(b, func(d *api.Doc) {
		d.SketchIn(d.Workplane("XY"), "")
		d.Circle(api.V(0, 0, 0), 5)
		cyl := d.Extrude(10, "")
		d.ShellSolid([]api.SolidFace{cyl.Solid().FaceAt(api.V(0, 0, 10))}, 1, "")
	})
}

// A ring extruded with a draft: cones inside and out (roadmap 15.4).
func BenchmarkDraft(b *testing.B) {
	benchRegen(b, func(d *api.Doc) {
		d.SketchIn(d.Workplane("XY"), "")
		d.Circle(api.V(0, 0, 0), 10)
		d.Circle(api.V(0, 0, 0), 4)
		d.Extrude(10, "").SetDraft(3)
	})
}

func BenchmarkLoft(b *testing.B) {
	benchRegen(b, func(d *api.Doc) {
		bottom := d.SketchIn(d.Workplane("XY"), "")
		d.Circle(api.V(0, 0, 0), 5)
		top := d.SketchAt(above(d, 10), api.V(1, 0, 0), api.V(0, 1, 0), "")
		d.Rect(api.V(-3, -3, 10), api.V(3, 3, 10))
		d.Loft(bottom, top, "").SetTurn(1)
	})
}

func BenchmarkFillet(b *testing.B) {
	t := &testing.T{}
	benchRegen(b, func(d *api.Doc) {
		ex := block(d, 0, 0, 20, 10, 10)
		d.Fillet([]api.Entity{
			lineAlong(t, ex, api.V(0, 0, 10), api.V(20, 0, 10)),
			lineAlong(t, ex, api.V(0, 10, 0), api.V(20, 10, 0)),
		}, 2, "")
	})
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

func BenchmarkPathPattern(b *testing.B) {
	benchRegen(b, func(d *api.Doc) {
		path := patternPath(d, 20, 10, 90)
		d.SketchIn(d.Workplane("XY"), "post-sketch")
		d.Circle(api.V(0, 0, 0), 1)
		d.Extrude(5, "post")
		d.PatternAlongPath(path, 7, "")
	})
}

// Twelve posts round a ring (roadmap 28.5).
func BenchmarkPathPatternRing(b *testing.B) {
	benchRegen(b, func(d *api.Doc) {
		d.SketchIn(d.Workplane("XY"), "ring")
		ring := d.Circle(api.V(0, 0, 0), 20)
		d.SketchIn(d.Workplane("XY"), "post-sketch")
		d.Circle(api.V(20, 0, 0), 1)
		d.Extrude(5, "post")
		d.PatternAlongPath([]api.Entity{ring}, 12, "")
	})
}

// The top edges of a block chamfered by two distances (roadmap 28.1):
// their corners are cut by wedges.
func BenchmarkChamferTwo(b *testing.B) {
	benchRegen(b, func(d *api.Doc) {
		d.Rect(api.V(0, 0, 0), api.V(40, 30, 0))
		ex := d.Extrude(10, "")
		var top []api.Entity
		for _, e := range ex.Entities() {
			if e.Kind() == "line" && e.Start().Pos().Z == 10 && e.End().Pos().Z == 10 {
				top = append(top, e)
			}
		}
		d.Chamfer(top, 1, "").SetSecondDistance(2)
	})
}

// A D drafted all round: cones and planes meeting in conics (roadmap
// 28.2).
func BenchmarkDraftSharpArc(b *testing.B) {
	benchRegen(b, func(d *api.Doc) {
		d.SketchIn(d.Workplane("XY"), "")
		d.Arc(api.V(0, 0, 0), api.V(10, 0, 0), api.V(-10, 0, 0))
		d.Line(api.V(-10, 0, 0), api.V(10, 0, 0))
		d.Extrude(8, "").SetDraft(6)
	})
}

// A rectangle swept round three mitred corners (roadmap 28.4).
func BenchmarkSweepCorners(b *testing.B) {
	benchRegen(b, func(d *api.Doc) {
		d.SketchIn(d.Workplane("ZX"), "path")
		path := []api.Entity{
			d.Line(api.V(0, 0, 0), api.V(0, 0, 10)),
			d.Line(api.V(0, 0, 10), api.V(10, 0, 15)),
			d.Line(api.V(10, 0, 15), api.V(20, 0, 10)),
			d.Line(api.V(20, 0, 10), api.V(20, 0, 0)),
		}
		d.SketchIn(d.Workplane("XY"), "profile")
		d.Circle(api.V(0, 0, 0), 1)
		d.Sweep(path, "")
	})
}

// A circle lofted to a circle held to two guides that widen it: the
// loft's sections moved and scaled, then skinned (roadmap 28.3).
func BenchmarkLoftGuides(b *testing.B) {
	benchRegen(b, func(d *api.Doc) {
		bottom := d.SketchIn(d.Workplane("XY"), "")
		d.Circle(api.V(0, 0, 0), 5)
		d.Sketch3D("")
		g1 := d.Cubic(api.V(5, 0, 0), api.V(9, 0, 4), api.V(9, 0, 8), api.V(5, 0, 12))
		g2 := d.Cubic(api.V(-5, 0, 0), api.V(-6, 0, 4), api.V(-6, 0, 8), api.V(-5, 0, 12))
		top := d.SketchAt(above(d, 12), api.V(1, 0, 0), api.V(0, 1, 0), "")
		d.Circle(api.V(0, 0, 12), 5)
		d.Loft(bottom, top, "").SetGuides([]api.Entity{g1, g2})
	})
}

// A square lofted to a square, square to both ends: one cubic surface
// (roadmap 28.3).
func BenchmarkLoftTangency(b *testing.B) {
	benchRegen(b, func(d *api.Doc) {
		bottom := d.SketchIn(d.Workplane("XY"), "")
		d.Circle(api.V(0, 0, 0), 5)
		top := d.SketchAt(above(d, 10), api.V(1, 0, 0), api.V(0, 1, 0), "")
		d.Rect(api.V(-3, -3, 10), api.V(3, 3, 10))
		d.Loft(bottom, top, "").SetTangency(1, 1)
	})
}
