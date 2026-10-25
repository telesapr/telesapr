package api

import (
	"math"
	"path/filepath"
	"testing"
)

// lens is a prism on a profile of two cubics meeting at two sharp
// corners: its top and bottom rims are each two curved edges that meet
// at the two creases of its wall.
func lens(t *testing.T, d *Doc) Group {
	t.Helper()
	d.Edit.SetChordTolerance(0.01)
	d.Features.SketchIn(d.Workplane("XY"), "")
	a := d.Sketch.Cubic(V(0, 0, 0), V(5, 8, 0), V(15, 8, 0), V(20, 0, 0))
	b := d.Sketch.Cubic(V(20, 0, 0), V(15, -8, 0), V(5, -8, 0), V(0, 0, 0))
	pa, pb := a.Points(), b.Points()
	d.Constrain.Coincident(pa[3], pb[0])
	d.Constrain.Coincident(pb[3], pa[0])
	return d.Features.Extrude(5, "")
}

// rimEdges are the curved edges of g's solid lying wholly at height z.
func rimEdges(g Group, z float64) []SolidEdge {
	var rim []SolidEdge
	for _, e := range g.Solid().Edges() {
		a, b := e.Ends()
		if math.Abs(a.Z-z) < 1e-9 && math.Abs(b.Z-z) < 1e-9 && !e.Straight() {
			rim = append(rim, e)
		}
	}
	return rim
}

// The lens's top rim, its two edges picked together, is filleted as two
// chains mitred at the creases (pkg/topo/rollingchains.go): the solid is
// sound, its mitres run from the creases at the ball's height to where
// OpenCASCADE's makeFillet of the same edges has them, and it comes back
// alike after saving and loading. pkg/topo's TestFilletChainsMitred
// holds the comparison with OpenCASCADE.
func TestFilletChainMitre(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("fillets of several chains over curved faces, slow")
	}
	d := New()
	g := lens(t, d)
	top := rimEdges(g, 5)
	if len(top) != 2 {
		t.Fatalf("the lens's top rim has %d edges", len(top))
	}
	f := d.Features.FilletSolid(top, 1, "")
	if p := f.Problem(); p != "" {
		t.Fatal(p)
	}
	rep := f.Analyze().Check()
	if !rep.Sound() {
		t.Errorf("%s", rep.Summary())
	}
	want := []Vec{V(0, 0, 4), V(1.195537, 0, 5), V(20, 0, 4), V(18.804463, 0, 5)}
	for _, q := range want {
		found := false
		for _, e := range f.Solid().Edges() {
			a, b := e.Ends()
			found = found || a.Minus(q).Magnitude() < 1e-4 || b.Minus(q).Magnitude() < 1e-4
		}
		if !found {
			t.Errorf("no vertex at %v", q)
		}
	}
	path := filepath.Join(t.TempDir(), "lens.tlspr")
	if err := d.File.Save(path); err != nil {
		t.Fatal(err)
	}
	l := New()
	if err := l.File.Load(path); err != nil {
		t.Fatal(err)
	}
	l.Edit.SetChordTolerance(0.01)
	gs := l.Groups()
	if r2 := gs[len(gs)-1].Analyze().Check(); !r2.Sound() || math.Abs(r2.Volume-rep.Volume) > 1e-6*rep.Volume {
		t.Errorf("loaded: %s, volume %g, saved %g", r2.Summary(), r2.Volume, rep.Volume)
	}
}
