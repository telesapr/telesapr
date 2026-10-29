package api

import (
	"math"
	"path/filepath"
	"testing"
)

// The corner at the top of an L-shaped block's inside edge, where its
// two convex top edges meet the concave upright between them: all three
// chamfered, of one distance and of three, the top edge along x alone,
// ending at the corner, and the top edge along x with the upright, the
// other top edge left sharp. Each takes OpenCASCADE's makeChamfer's
// volume (scripts/chamfer-setback-fixtures.py's ledgeall, ledgeone and
// ledgetwo, on this block, 2026-10-29), within what the corner's
// bilinear patch's mesh strays at this chord tolerance, or for the last
// within what its plate parts from the hyperbolic paraboloid here
// (TestChamferCornerOracle); the first is saved and loaded, and undone.
func TestChamferMixedCorner(t *testing.T) {
	t.Parallel()
	c := V(10, 10, 20)
	for i, tc := range []struct {
		ends []Vec
		ds   []float64
		want float64
		tol  float64
	}{
		{[]Vec{V(20, 10, 20), V(10, 20, 20), V(10, 10, 0)}, []float64{1, 1, 1}, 6000.333333333, 0.005},
		{[]Vec{V(20, 10, 20), V(10, 20, 20), V(10, 10, 0)}, []float64{2, 1, 1.5}, 5999, 0.005},
		{[]Vec{V(20, 10, 20)}, []float64{1}, 5995, 0.005},
		{[]Vec{V(20, 10, 20), V(10, 10, 0)}, []float64{0.5, 1}, 6008.727896466, 0.15},
	} {
		d := New()
		if i != 0 {
			// The first, of one distance, is filled by a plane.
			d.Edit.SetChordTolerance(0.002)
		}
		d.Sketch.Polyline(true, V(0, 0, 0), V(20, 0, 0), V(20, 10, 0), V(10, 10, 0), V(10, 20, 0), V(0, 20, 0))
		g := d.Features.Extrude(20, "")
		var edges []SolidEdge
		for _, b := range tc.ends {
			edges = append(edges, g.Solid().EdgeAt(c.Plus(b).ScaledBy(0.5)))
		}
		f := d.Features.ChamferSolid(edges, tc.ds[0], "")
		for k, dist := range tc.ds {
			if dist != tc.ds[0] {
				f.AsChamfer().SetEdgeDistance(k, dist)
			}
		}
		if p := f.Problem(); p != "" {
			t.Fatalf("%v: %s", tc.ds, p)
		}
		rep := f.Analyze().Check()
		if !rep.Sound() || math.Abs(rep.Volume-tc.want) > tc.tol {
			t.Errorf("%v: %s, OpenCASCADE's %g", tc.ds, rep.Summary(), tc.want)
		}
		if i != 0 {
			continue
		}
		path := filepath.Join(t.TempDir(), "corner.tlspr")
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
