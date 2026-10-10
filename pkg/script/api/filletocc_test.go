package api

import (
	"math"
	"path/filepath"
	"testing"
)

// Fillets of radius 1 on edges of STEP solids (roadmap 22.5) take the
// volume of an exact construction, or OpenCASCADE's where its fillet is
// right: a box edge (a cylinder: the corner's square less a quarter
// circle along 30), a hole's rims (a torus, by Pappus about the hole's
// axis at the section's centroid, 4 + 0.2234 out; the lower one's ball
// passes a vertex where three of the hole's patches meet), a blob's upright
// corner between two B-spline walls (the section, found in the plane,
// times its height 5: OpenCASCADE takes a fifth less there), a bored
// sphere's rim and a turned vase's foot and lip (OpenCASCADE's makeFillet
// of the same edge, scripts/fillet-step-volumes.py), a degree-7 lid's
// edge where a wall meets its dome (the same). The mesh's chords at the
// chord tolerance below count for about half a percent, and for the
// vase's foot, whose section is small, steep and turned about the axis,
// for about 0.01 mm³.
func TestFilletOnOtherSolidsVolumes(t *testing.T) {
	if testing.Short() {
		t.Skip("STEP solids at a fine tolerance")
	}
	spandrel := 1 - math.Pi/4
	cases := []struct {
		file string
		at   Vec
		took float64
	}{
		{"shapes/box_hole.step", V(0, 0, 15), spandrel * 30},
		{"shapes/box_hole.step", V(12.8060019, 12.8486353, 30), spandrel * 2 * math.Pi * (4 + (10-3*math.Pi)/(12-3*math.Pi))},
		{"shapes/box_hole.step", V(11.8484225, 13.5472996, 0), spandrel * 2 * math.Pi * (4 + (10-3*math.Pi)/(12-3*math.Pi))},
		{"shapes/blob_extruded.step", V(0, 0, 2.5), 5.315751},
		{"shapes/sphere_bored.step", V(0, -3, -6.32455532), 18.564890},
		{"shapes/vase_revolved.step", V(4.33049372, -4.15214177, 0), 0.395511},
		{"shapes/vase_revolved.step", V(5.05224268, -4.8441654, 14), 42.726909},
		{"degree/lid_degree7.step", V(20.3125, 0, 9.59580796), 9.792331},
	}
	for _, c := range cases {
		d := New()
		d.Edit.SetChordTolerance(0.002)
		g, err := d.Assembly.LinkStep(filepath.Join("..", "..", "..", "testdata", "step", c.file), "", true)
		if err != nil {
			t.Fatal(err)
		}
		before := g.Analyze().Check().Volume
		f := d.Features.FilletSolid([]SolidEdge{g.Solid().EdgeAt(c.at)}, 1, "")
		if p := f.Problem(); p != "" {
			t.Errorf("%s at %v: %s", c.file, c.at, p)
			continue
		}
		r := f.Analyze().Check()
		took := before - r.Volume
		t.Logf("%s at %v: took %.6f, want %.6f (%+.2f%%)", c.file, c.at, took, c.took, 100*(took/c.took-1))
		if !r.Sound() || math.Abs(took-c.took) > math.Max(0.01*c.took, 0.02) {
			t.Errorf("%s at %v: %s, took %g, want %g", c.file, c.at, r.Summary(), took, c.took)
		}
	}
}

// The rims of a STEP cylinder filleted at fine chord tolerances: every
// span of the fillet's surface lies just over the tolerance at its first
// sections, and each section put in tipped the span beside it over, one
// a round round the edge, until the rounds ran out (misfit 2.67e-5 at
// 2.4e-5). The late rounds split the spans near the tolerance too.
func TestFilletCylinderFineTolerance(t *testing.T) {
	if testing.Short() {
		t.Skip("STEP solids at a fine tolerance")
	}
	for _, chord := range []float64{0.002, 0.001} {
		for i := range 2 {
			d := New()
			d.Edit.SetChordTolerance(chord)
			g, err := d.Assembly.LinkStep(filepath.Join("..", "..", "..", "testdata", "step", "shapes", "cylinder.step"), "", true)
			if err != nil {
				t.Fatal(err)
			}
			f := d.Features.FilletSolid([]SolidEdge{g.Solid().Edges()[i]}, 1, "")
			if p := f.Problem(); p != "" {
				t.Errorf("chord %g%%, rim %d: %s", chord, i, p)
				continue
			}
			if r := f.Analyze().Check(); !r.Sound() {
				t.Errorf("chord %g%%, rim %d: %s", chord, i, r.Summary())
			}
		}
	}
}
