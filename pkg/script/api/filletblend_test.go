package api

import (
	"math"
	"path/filepath"
	"testing"
)

// The corner at the top of an L-shaped block's inside edge, its top edge
// along x rounded, convex, and the concave upright below it, the other
// top edge left sharp: a blend corner, one patch. Each takes
// OpenCASCADE's makeFillet's volume (the ledge of
// scripts/fillet-blend-fixtures.py on this block, 2026-10-29) within
// what the two patches differ by; the first is saved and loaded, and
// undone.
func TestFilletBlendCorner(t *testing.T) {
	t.Parallel()
	c := V(10, 10, 10)
	for i, tc := range []struct {
		radii []float64
		want  float64
	}{
		{[]float64{1, 1}, 3000.09219},
		{[]float64{1, 2}, 3006.62749},
	} {
		d := New()
		if i != 0 {
			// The first is saved, and a file keeps no chord tolerance.
			d.Edit.SetChordTolerance(0.002)
		}
		d.Sketch.Polyline(true, V(0, 0, 0), V(20, 0, 0), V(20, 10, 0), V(10, 10, 0), V(10, 20, 0), V(0, 20, 0))
		g := d.Features.Extrude(10, "")
		edges := []SolidEdge{g.Solid().EdgeAt(c.Plus(V(20, 10, 10)).ScaledBy(0.5)), g.Solid().EdgeAt(c.Plus(V(10, 10, 0)).ScaledBy(0.5))}
		f := d.Features.FilletSolid(edges, tc.radii[0], "")
		if tc.radii[1] != tc.radii[0] {
			f.AsFillet().SetEdgeRadius(1, tc.radii[1])
		}
		if p := f.Problem(); p != "" {
			t.Fatalf("%v: %s", tc.radii, p)
		}
		rep := f.Analyze().Check()
		if !rep.Sound() || math.Abs(rep.Volume-tc.want) > 0.2 {
			t.Errorf("%v: %s, OpenCASCADE's %g", tc.radii, rep.Summary(), tc.want)
		}
		if i != 0 {
			continue
		}
		path := filepath.Join(t.TempDir(), "blend.tlspr")
		if err := d.File.Save(path); err != nil {
			t.Fatal(err)
		}
		l := New()
		if err := l.File.Load(path); err != nil {
			t.Fatal(err)
		}
		gs := l.Groups()
		if r2 := gs[len(gs)-1].Analyze().Check(); !r2.Sound() || math.Abs(r2.Volume-rep.Volume) > 1e-9 {
			t.Errorf("loaded: %s, saved %g", r2.Summary(), rep.Volume)
		}
		n := len(d.Groups())
		if !d.Edit.Undo() || len(d.Groups()) >= n {
			t.Errorf("undo left %d groups of %d", len(d.Groups()), n)
		}
	}
}
