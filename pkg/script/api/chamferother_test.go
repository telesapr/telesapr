package api

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"
)

// Chamfers on the solids of TestFilletOnOtherSolids, every edge alone at
// the fillets' radius as the distance: one that is made must be sound,
// and the counts made may only get better. Until 2026-10-23 a straight
// edge between flat faces that ends on a curved face was cut by a wedge
// and a Boolean, which left the wedge's end open across the curved face
// (the lid's 4 uprights, unsound with no word of it); it is now the
// rolling ball's, flat, as its fillet is, and where the rolling ball
// refuses, so does the chamfer.
func TestChamferOnOtherSolids(t *testing.T) {
	if testing.Short() {
		t.Skip("STEP solids at a fine tolerance")
	}
	minMade := map[string]int{"loft": 4, "sweep": 12, "box_hole": 14, "cylinder": 2, "blob_extruded": 1,
		"sphere_bored": 2, "vase_revolved": 2, "lid_degree7": 12}
	total, made := 0, 0
	for _, c := range otherSolids(t) {
		tl := cutEveryEdge(t, c.build, c.r, 16, true)
		t.Logf("%s, d %g: %s", c.name, c.r, tl)
		if tl.unsound > 0 {
			t.Errorf("%s: %d chamfers made but not sound", c.name, tl.unsound)
		}
		if tl.sound < minMade[c.name] {
			t.Errorf("%s: %d chamfers made, %d were", c.name, tl.sound, minMade[c.name])
		}
		total += tl.sound + tl.unsound + tl.refused
		made += tl.sound
	}
	t.Logf("in all: %d of %d edges chamfered soundly (%.0f%%)", made, total, 100*float64(made)/float64(total))
}

// lidUprights are the lid's four upright edges, between two flat walls,
// each ending at the top on its curved dome, by a point on each, with
// what OpenCASCADE's makeChamfer takes from each at distances 1 and 2
// (FreeCAD 2026-10-23, as scripts/fillet-step-volumes.py does for
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
func lidStep(t *testing.T, d *Doc) Group {
	t.Helper()
	g, err := d.Assembly.LinkStep(filepath.Join("..", "..", "..", "testdata", "step", "degree", "lid_degree7.step"), "", true)
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
			path := filepath.Join(t.TempDir(), "lid.tlspr")
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
// side, ending where the sides meet in creases: the rolling ball can't
// fit its end there, and the chamfer says so rather than cutting a wedge
// that would leave the solid open.
func TestChamferRollingRefusal(t *testing.T) {
	d := New()
	d.Edit.SetChordTolerance(0.01)
	g := otherSolids(t)[0].build(d)
	msg := func() (msg string) {
		defer func() {
			if x := recover(); x != nil {
				msg = fmt.Sprint(x)
			}
		}()
		return d.Features.ChamferSolid([]SolidEdge{g.Solid().EdgeAt(V(0, -18, 0))}, 2, "").Problem()
	}()
	if !strings.Contains(msg, "curved face its edge ends at") {
		t.Errorf("the loft's foot chamfered: %q", msg)
	}
}
