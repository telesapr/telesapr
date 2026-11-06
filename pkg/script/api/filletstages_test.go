package api

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
)

// A block's upright edges rounded at one radius and its top edges at a
// smaller one, in one fillet with radii per edge (roadmap 22.4): the
// corners are rounded in stages, sound and of the volume by formula; one
// step of undo; the same after saving and loading. The radii the other
// way round are set back at the corners (roadmap 22.4), sound, and gone
// with one undo (and in a full sweep the same after saving and loading,
// the first case's way through the file again); two top edges of
// different radii over an upright left sharp are refused, saying why,
// marked at the corner. The three side by side.
func TestFilletStagesScript(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("a long script")
	}
	if fixtures.RaceDetector {
		// Twelve minutes there; its fillets' stages run on the goroutines
		// TestFilletOnOtherSolids's do, and it checks the script path.
		t.Skip("a long script, minutes under the race detector")
	}
	const w, l, h = stagesW, stagesL, stagesH
	build := func(d *Doc) (Group, []SolidEdge, int) {
		d.Edit.SetChordTolerance(0.002)
		d.Features.SketchIn(d.Workplane("XY"), "")
		d.Sketch.Rect(V(0, 0, 0), V(w, l, 0))
		block := d.Features.Extrude(h, "block")
		edges := block.Solid().EdgesAlong(V(0, 0, 1))
		n := len(edges)
		for _, e := range block.Solid().Edges() {
			if a, b := e.Ends(); near(a.Z, h) && near(b.Z, h) {
				edges = append(edges, e)
			}
		}
		return block, edges, n
	}
	t.Run("stages", func(t *testing.T) {
		t.Parallel()
		filletStages(t, build)
	})
	t.Run("set back", func(t *testing.T) {
		t.Parallel()
		filletSetBack(t, build)
	})
	t.Run("mitre", func(t *testing.T) {
		t.Parallel()
		filletTwoRadiiMitre(t, build)
	})
}

// The block of TestFilletStagesScript, 30 by 20 by 15, the radii of its
// uprights' and its top's fillets.
const stagesW, stagesL, stagesH, stagesUpright, stagesTop = 30.0, 20.0, 15.0, 5.0, 2.0

// filletStages is TestFilletStagesScript's uprights rounded at 5 and the
// top at 2: the volume by formula, the file, the undo.
func filletStages(t *testing.T, build func(d *Doc) (Group, []SolidEdge, int)) {
	const w, l, h, upright, top = stagesW, stagesL, stagesH, stagesUpright, stagesTop
	d := New()
	_, edges, n := build(d)
	if n != 4 || len(edges) != 8 {
		t.Fatalf("%d upright edges and %d in all", n, len(edges))
	}
	fill := d.Features.FilletSolid(edges, upright, "rounded")
	for i := n; i < len(edges); i++ {
		fill.AsFillet().SetEdgeRadius(i, top)
	}
	if p := fill.Problem(); p != "" {
		t.Fatal(p)
	}
	spandrel := func(r float64) float64 { return r * r * (1 - math.Pi/4) }
	c := top * (10 - 3*math.Pi) / (12 - 3*math.Pi)
	want := w*l*h - 4*spandrel(upright)*h - spandrel(top)*2*(w+l-4*upright) - spandrel(top)*2*math.Pi*(upright-c)
	r := fill.Analyze().Check()
	if !r.Sound() || math.Abs(r.Volume-want) > 1e-4*want {
		t.Fatalf("%s, volume %g, want %g", r.Summary(), r.Volume, want)
	}
	path := filepath.Join(t.TempDir(), "casing.tlspr")
	if err := d.File.Save(path); err != nil {
		t.Fatal(err)
	}
	e := New()
	if err := e.File.Load(path); err != nil {
		t.Fatal(err)
	}
	e.Edit.SetChordTolerance(0.002)
	g := e.Groups()
	if r2 := g[len(g)-1].Analyze().Check(); !r2.Sound() || math.Abs(r2.Volume-r.Volume) > 1e-6*r.Volume {
		t.Errorf("loaded: %s, volume %g, saved %g", r2.Summary(), r2.Volume, r.Volume)
	}
	if !d.Edit.Undo() {
		t.Fatal("no undo")
	}
	if g := d.Groups(); g[len(g)-1].Name() == "rounded" {
		t.Fatal("the fillet is still there after undo")
	}
}

// filletSetBack is TestFilletStagesScript's top larger than the
// uprights it would go round: set back.
func filletSetBack(t *testing.T, build func(d *Doc) (Group, []SolidEdge, int)) {
	const w, l, h, upright, top = stagesW, stagesL, stagesH, stagesUpright, stagesTop
	f := New()
	_, edges, n := build(f)
	other := f.Features.FilletSolid(edges, top, "rounded")
	for i := n; i < len(edges); i++ {
		other.AsFillet().SetEdgeRadius(i, upright)
	}
	if p := other.Problem(); p != "" {
		t.Errorf("the radii the other way round: %q", p)
	} else if r := other.Analyze().Check(); !r.Sound() || r.Volume >= w*l*h {
		t.Errorf("the radii the other way round: %s, volume %g", r.Summary(), r.Volume)
	} else {
		// The same after saving and loading (regenerating it again: a
		// full sweep's), and gone with one undo.
		if sweep.Full() {
			path := filepath.Join(t.TempDir(), "setback.tlspr")
			if err := f.File.Save(path); err != nil {
				t.Fatal(err)
			}
			e := New()
			if err := e.File.Load(path); err != nil {
				t.Fatal(err)
			}
			e.Edit.SetChordTolerance(0.002)
			g := e.Groups()
			if r2 := g[len(g)-1].Analyze().Check(); !r2.Sound() || math.Abs(r2.Volume-r.Volume) > 1e-6*r.Volume {
				t.Errorf("set back, loaded: %s, volume %g, saved %g", r2.Summary(), r2.Volume, r.Volume)
			}
		}
		if !f.Edit.Undo() {
			t.Fatal("no undo")
		}
		if g := f.Groups(); g[len(g)-1].Name() == "rounded" {
			t.Error("the set back fillet is still there after undo")
		}
	}
}

// filletTwoRadiiMitre is TestFilletStagesScript's two top edges of
// different radii over an upright left sharp: the smaller ends on the
// larger's cylinder.
func filletTwoRadiiMitre(t *testing.T, build func(d *Doc) (Group, []SolidEdge, int)) {
	const w, upright, top = stagesW, stagesUpright, stagesTop
	g2 := New()
	_, edges, n := build(g2)
	var two []SolidEdge
	for _, e := range edges[n:] {
		a, b := e.Ends()
		if near(a.Y, 0) && near(b.Y, 0) || near(a.X, w) && near(b.X, w) {
			two = append(two, e)
		}
	}
	if len(two) != 2 {
		t.Fatalf("%d top edges at the corner", len(two))
	}
	pair := g2.Features.FilletSolid(two, top, "rounded")
	pair.AsFillet().SetEdgeRadius(1, upright)
	if p := pair.Problem(); p != "" {
		t.Errorf("a mitre of two radii: %q", p)
	}
}
