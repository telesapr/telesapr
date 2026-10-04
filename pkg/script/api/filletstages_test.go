package api

import (
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// A block's upright edges rounded at one radius and its top edges at a
// smaller one, in one fillet with radii per edge (roadmap 22.4): the
// corners are rounded in stages, sound and of the volume by formula; one
// step of undo; the same after saving and loading; and the radii the
// other way round refused, saying why, marked at the corner.
func TestFilletStagesScript(t *testing.T) {
	const w, l, h, upright, top = 30.0, 20.0, 15.0, 5.0, 2.0
	build := func(d *Doc) (Group, []SolidEdge, int) {
		d.SetChordTolerance(0.002)
		d.SketchIn(d.Workplane("XY"), "")
		d.Rect(V(0, 0, 0), V(w, l, 0))
		block := d.Extrude(h, "block")
		edges := block.Solid().EdgesAlong(V(0, 0, 1))
		n := len(edges)
		for _, e := range block.Solid().Edges() {
			if a, b := e.Ends(); near(a.Z, h) && near(b.Z, h) {
				edges = append(edges, e)
			}
		}
		return block, edges, n
	}
	d := New()
	_, edges, n := build(d)
	if n != 4 || len(edges) != 8 {
		t.Fatalf("%d upright edges and %d in all", n, len(edges))
	}
	fill := d.FilletSolid(edges, upright, "rounded")
	for i := n; i < len(edges); i++ {
		fill.SetEdgeRadius(i, top)
	}
	if p := fill.Problem(); p != "" {
		t.Fatal(p)
	}
	spandrel := func(r float64) float64 { return r * r * (1 - math.Pi/4) }
	c := top * (10 - 3*math.Pi) / (12 - 3*math.Pi)
	want := w*l*h - 4*spandrel(upright)*h - spandrel(top)*2*(w+l-4*upright) - spandrel(top)*2*math.Pi*(upright-c)
	r := fill.Check()
	if !r.Sound() || math.Abs(r.Volume-want) > 1e-4*want {
		t.Fatalf("%s, volume %g, want %g", r.Summary(), r.Volume, want)
	}
	path := filepath.Join(t.TempDir(), "casing.tlspr")
	if err := d.Save(path); err != nil {
		t.Fatal(err)
	}
	e := New()
	if err := e.Load(path); err != nil {
		t.Fatal(err)
	}
	e.SetChordTolerance(0.002)
	g := e.Groups()
	if r2 := g[len(g)-1].Check(); !r2.Sound() || math.Abs(r2.Volume-r.Volume) > 1e-6*r.Volume {
		t.Errorf("loaded: %s, volume %g, saved %g", r2.Summary(), r2.Volume, r.Volume)
	}
	if !d.Undo() {
		t.Fatal("no undo")
	}
	if g := d.Groups(); g[len(g)-1].Name() == "rounded" {
		t.Fatal("the fillet is still there after undo")
	}

	// The top larger than the uprights it would go round.
	f := New()
	_, edges, n = build(f)
	wrong := f.FilletSolid(edges, top, "rounded")
	for i := n; i < len(edges); i++ {
		wrong.SetEdgeRadius(i, upright)
	}
	p := wrong.Problem()
	if !strings.Contains(p, "one radius at a time") {
		t.Errorf("the radii the other way round: %q", p)
	}
	wrong.d.read(func(sk *sketch.Sketch) {
		if pr := wrong.get(sk).Problem; !pr.HasPoint || !near(pr.At.Z, h) {
			t.Errorf("the refusal is marked at %v (%v), not at a top corner", pr.At, pr.HasPoint)
		}
	})
}
