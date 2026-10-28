package api

import (
	"math"
	"path/filepath"
	"sync"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
)

// Chamfers on the solids of TestFilletOnOtherSolids, every edge alone at
// the fillets' radius as the distance: one that is made must be sound,
// and the counts made may only get better. Until 2026-10-22 a straight
// edge between flat faces that ends on a curved face was cut by a wedge
// and a Boolean, which left the wedge's end open across the curved face
// (the lid's 4 uprights, unsound with no word of it); it is now the
// rolling ball's, flat, as its fillet is, and where the rolling ball
// refuses, so does the chamfer. Since 2026-10-28 the blob's two rims are
// mitred at their creases, as their fillets are (TestChamferRimMitre).
func TestChamferOnOtherSolids(t *testing.T) {
	if testing.Short() {
		t.Skip("STEP solids at a fine tolerance")
	}
	minMade := map[string]int{"loft": 12, "sweep": 12, "box_hole": 14, "cylinder": 2, "blob_extruded": 3,
		"sphere_bored": 2, "vase_revolved": 2, "lid_degree7": 12}
	t.Parallel()
	var mu sync.Mutex
	total, made := 0, 0
	t.Run("solids", func(t *testing.T) {
		for _, c := range otherSolids() {
			t.Run(c.name, func(t *testing.T) {
				t.Parallel()
				tl := cutEveryEdge(t, c.build, c.r, edgesPerSolid(), true)
				t.Logf("d %g: %s", c.r, tl)
				if tl.unsound > 0 {
					t.Errorf("%d chamfers made but not sound", tl.unsound)
				}
				if !fixtures.RaceDetector && tl.sound < minMade[c.name] {
					t.Errorf("%d chamfers made, %d were", tl.sound, minMade[c.name])
				}
				mu.Lock()
				defer mu.Unlock()
				total += tl.sound + tl.unsound + tl.refused
				made += tl.sound
			})
		}
	})
	t.Logf("in all: %d of %d edges chamfered soundly (%.0f%%)", made, total, 100*float64(made)/float64(total))
}

// lidUprights are the lid's four upright edges, between two flat walls,
// each ending at the top on its curved dome, by a point on each, with
// what OpenCASCADE's makeChamfer takes from each at distances 1 and 2
// (FreeCAD 2026-10-22, as scripts/fillet-step-volumes.py does for
// fillets).
var lidUprights = []struct {
	at     Vec
	d1, d2 float64
}{
	{V(0, 0, 5), 5.093024, 20.696089},
	{V(40, 0, 5), 5.350462, 21.459068},
	{V(40, 30, 5), 4.731403, 19.039396},
	{V(0, 30, 4), 4.239037, 17.358741},
}

// lidStep is the lid read as a solid.
var lidStepFile = filepath.Join("..", "..", "..", "testdata", "step", "degree", "lid_degree7.step")

// sameVolume reports whether the folder and the file are on one drive: a
// sketch saved in the folder links the file relative to it, which it
// can't across drives on Windows (the runner's temporary folder is on
// C: and the checkout on D:).
func sameVolume(t *testing.T, dir, file string) bool {
	t.Helper()
	abs, err := filepath.Abs(file)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.VolumeName(abs) != filepath.VolumeName(dir) {
		t.Logf("%s and %s are on different drives: not saved and loaded", dir, abs)
		return false
	}
	return true
}

func lidStep(t *testing.T, d *Doc) Group {
	t.Helper()
	g, err := d.Assembly.LinkStep(lidStepFile, "", true)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// A chamfer of an upright of the lid ends on its dome: the rolling ball,
// flat, its end on the dome fitted to where the chamfer meets it, its
// way there along the chamfer's own surface (a fillet's is the wall's).
// It takes what OpenCASCADE's makeChamfer takes within 0.05% (ours
// within 1.2e-4 at this chord tolerance); the first comes back alike
// after saving and loading and goes with one undo.
func TestChamferEndsOnCurvedFace(t *testing.T) {
	t.Parallel()
	cases := lidUprights
	if testing.Short() {
		cases = cases[:1]
	}
	for i, c := range cases {
		for _, dist := range []float64{1, 2} {
			if testing.Short() && dist != 1 {
				continue
			}
			want := c.d1
			if dist == 2 {
				want = c.d2
			}
			d := New()
			d.Edit.SetChordTolerance(0.002)
			g := lidStep(t, d)
			before := g.Analyze().Check().Volume
			f := d.Features.ChamferSolid([]SolidEdge{g.Solid().EdgeAt(c.at)}, dist, "")
			if p := f.Problem(); p != "" {
				t.Fatalf("at %v, %g: %s", c.at, dist, p)
			}
			rep := f.Analyze().Check()
			took := before - rep.Volume
			if !rep.Sound() || math.Abs(took-want) > 5e-4*want {
				t.Errorf("at %v, %g: %s, took %g, OpenCASCADE %g", c.at, dist, rep.Summary(), took, want)
			}
			if i != 0 || dist != 1 {
				continue
			}
			if dir := t.TempDir(); sameVolume(t, dir, lidStepFile) {
				path := filepath.Join(dir, "lid.tlspr")
				if err := d.File.Save(path); err != nil {
					t.Fatal(err)
				}
				l := New()
				if err := l.File.Load(path); err != nil {
					t.Fatal(err)
				}
				l.Edit.SetChordTolerance(0.002)
				gs := l.Groups()
				if r2 := gs[len(gs)-1].Analyze().Check(); !r2.Sound() || math.Abs(r2.Volume-rep.Volume) > 1e-6*rep.Volume {
					t.Errorf("loaded: %s, volume %g, saved %g", r2.Summary(), r2.Volume, rep.Volume)
				}
			}
			n := len(d.Groups())
			if !d.Edit.Undo() || len(d.Groups()) >= n {
				t.Errorf("undo left %d groups of %d", len(d.Groups()), n)
			}
		}
	}
}

// The lid's four uprights chamfered together: each alone, one after
// another, as their chamfers don't meet (topo.filletApart), taking what
// OpenCASCADE takes from each alone within 0.05%.
func TestChamferEndsOnCurvedFaceApart(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("chamfers over curved faces, slow")
	}
	d := New()
	d.Edit.SetChordTolerance(0.002)
	g := lidStep(t, d)
	before := g.Analyze().Check().Volume
	var edges []SolidEdge
	want := 0.0
	for _, c := range lidUprights {
		edges = append(edges, g.Solid().EdgeAt(c.at))
		want += c.d1
	}
	f := d.Features.ChamferSolid(edges, 1, "")
	if p := f.Problem(); p != "" {
		t.Fatal(p)
	}
	rep := f.Analyze().Check()
	if took := before - rep.Volume; !rep.Sound() || math.Abs(took-want) > 5e-4*want {
		t.Errorf("%s, took %g, OpenCASCADE %g", rep.Summary(), took, want)
	}
}

// A chamfer of the loft's foot edge, between its flat foot and a curved
// side, ending where the sides meet in creases: its rulings twist near
// the ends, which the rolling ball's sections there are fitted to, each
// end's way along the chamfer from its foot's own ruling. Exported as
// STEP and read by OpenCASCADE (2026-10-22), the solid is valid and the
// chamfer of 2 took 40.142 mm³; OpenCASCADE's own makeChamfer of the edge
// takes 40.134 and leaves a solid it finds invalid.
func TestChamferLoftFoot(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("rolling-ball chamfer of a loft, slow")
	}
	d := New()
	d.Edit.SetChordTolerance(0.01)
	g := otherSolids()[0].build(t, d)
	f := d.Features.ChamferSolid([]SolidEdge{g.Solid().EdgeAt(V(0, -18, 0))}, 2, "")
	if p := f.Problem(); p != "" {
		t.Fatal(p)
	}
	if rep := f.Analyze().Check(); !rep.Sound() {
		t.Errorf("%s", rep.Summary())
	}
}

// The blob's rims chamfered at distance 1, each one closed edge turning
// at its wall's crease: the chain runs from the crease round to it, and
// its ends meet in a mitre where the chamfer's surfaces either way,
// carried on, meet, as the fillet's do (TestFilletRimMitre). Its
// vertices are OpenCASCADE's makeChamfer's within 3e-6 mm, and every
// point of either solid lies within 4.2e-5 mm of the other (sampled
// 2026-10-28; its Volume of our STEP export is 2% off, which surfaces
// that close can't be). Both rims are sound.
func TestChamferRimMitre(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("chamfers over curved faces, slow")
	}
	blob := filepath.Join("..", "..", "..", "testdata", "step", "shapes", "blob_extruded.step")
	for _, z := range []float64{5, 0} {
		d := New()
		d.Edit.SetChordTolerance(0.01)
		g, err := d.Assembly.LinkStep(blob, "", true)
		if err != nil {
			t.Fatal(err)
		}
		var rim SolidEdge
		for _, e := range g.Solid().Edges() {
			if a, b := e.Ends(); a == b && math.Abs(a.Z-z) < 1e-9 {
				rim = e
			}
		}
		f := d.Features.ChamferSolid([]SolidEdge{rim}, 1, "")
		if p := f.Problem(); p != "" {
			t.Fatalf("rim at z %g: %s", z, p)
		}
		if rep := f.Analyze().Check(); !rep.Sound() {
			t.Errorf("rim at z %g: %s", z, rep.Summary())
		}
		dz := 1.0
		if z == 0 {
			dz = -1
		}
		low, high := V(0, 0, z-dz), V(0.084219, 2.400768, z)
		var ends [2]bool
		for _, e := range f.Solid().Edges() {
			a, b := e.Ends()
			for i, q := range []Vec{low, high} {
				ends[i] = ends[i] || a.Minus(q).Magnitude() < 1e-4 || b.Minus(q).Magnitude() < 1e-4
			}
		}
		if !ends[0] || !ends[1] {
			t.Errorf("rim at z %g: the mitre's ends %v and %v are not both vertices (%v)", z, low, high, ends)
		}
	}
}
