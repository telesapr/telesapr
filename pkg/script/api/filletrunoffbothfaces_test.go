package api

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// flatConePlate is a cone about x from x = 0, radius 7, to x = 14,
// radius 2.8, its side past y = c cut away by a flat face (none where c
// is 0), with a plate 2 wide (y from -1 to 1) standing on it from x = 0
// to 10, its top h0 above the foot of its side on the cone at x = 0 and
// at z = 9 at x = 10. It gives the edge of the plate's side at y = 1
// with the cone, and the volume before. The cone's end at x = 0 and the
// plate's are one flat face; there the cone is widest, so that a blend
// whose foot on the cone stays short of the flat face along the rest of
// the edge reaches past it at that end, past the end of the cone's edge
// with the end face.
func flatConePlate(d *Doc, c, h0 float64) (SolidEdge, float64) {
	d.Features.SketchIn(d.Workplane("ZX"), "")
	axis := d.Sketch.Line(V(-2, 0, 0), V(16, 0, 0))
	axis.SetConstruction(true)
	d.Sketch.Polyline(true, V(0, 0, 0), V(14, 0, 0), V(14, 0, 2.8), V(0, 0, 7))
	d.Features.Lathe(axis, "cone")
	if c > 0 {
		d.Features.Sketch3D("flatplane")
		d.Features.SketchIn(d.Sketch.NewWorkplane(V(0, 0, -8), V(1, 0, 0), V(0, 1, 0)), "")
		d.Sketch.Polyline(true, V(-1, c, -8), V(15, c, -8), V(15, 9, -8), V(-1, 9, -8))
		cut := d.Features.Extrude(16, "flat")
		cut.SetCombine(sketch.CombineDifference)
	}
	foot := math.Sqrt(48)
	d.Features.SketchIn(d.Workplane("ZX"), "")
	d.Sketch.Polyline(true, V(0, 0, 2), V(10, 0, 2), V(10, 0, 9), V(0, 0, foot+h0))
	plate := d.Features.Extrude(2, "plate")
	plate.SetTwoSided(true)
	return plate.Solid().EdgeAt(V(5, 1, math.Sqrt(5.5*5.5-1))), plate.Analyze().Check().Volume
}

// flatConeTop is the height of flatConePlate's top at x, for h0.
func flatConeTop(h0, x float64) float64 {
	z0 := math.Sqrt(48) + h0
	return z0 + (9-z0)*x/10
}

// bothFacesOracle is what a blend of size r of flatConePlate's edge adds
// where its end runs off both its faces, the top at h0 and the flat at
// c: the same blend of the plate whose top is at 4 on the cone uncut,
// which runs off neither, less what of it lies above the top's plane
// at h0 and past the flat's plane, measured by vertical lines through
// its mesh (volumeAbove) above the plane and above the cone's own
// surface. Nothing but the blend lies there: the plate stands at y up
// to 1, the cone under its surface. The two pieces lie apart, one
// beside the plate's top, the other low on the cone.
func bothFacesOracle(chamfer bool, r, c, h0 float64) (float64, error) {
	d := New()
	d.Edit.SetChordTolerance(0.003)
	d.Edit.SetMaxSegments(200)
	edge, before := flatConePlate(d, 0, 4)
	g, err := blendOrRefusal(d, edge, r, chamfer)
	if err != nil {
		return 0, err
	}
	rep := g.Analyze().Check()
	if !rep.Sound() {
		return 0, fmt.Errorf("the blend that runs off no face: %s", rep.Summary())
	}
	const x1 = 5
	top := volumeAbove(meshOver(g, 0, x1, 1, 8), 0, x1, 1, 8, 800, func(x, _ float64) float64 {
		return flatConeTop(h0, x)
	})
	cone := func(x, y float64) float64 {
		rx := 7 - 0.3*x
		return math.Sqrt(math.Max(0, rx*rx-y*y))
	}
	past := volumeAbove(meshOver(g, 0, x1, c, 8), 0, x1, c, 8, 800, cone)
	return rep.Volume - before - top - past, nil
}

// Blends of the edge a plate stands on a cone by, whose end runs off
// both its faces at the plate's end: the plate's side past its top,
// which is low there, and the cone past its flat (flatConePlate). Made
// since 2026-11-09 (topo's rollingrunoff_both.go): the end patches are
// cut by both next faces in turn, and the end face grows by the end
// section between their cuts; refused before as running off both its
// faces. Sound, and adding what bothFacesOracle measures: chamfers of
// 1.6 to 2.3 and fillets of 3 to 5.5 within 0.3% (0.15% measured at
// most, the fillet of 5.5; the rest 0.08%: the oracle's columns and the
// meshes' chords). Where the two cuts would cross on the fillet, the two
// next faces meeting across it, the blend is refused (bothRunOffs,
// clipRunOffs); no size here reaches that before its foot on the cone
// runs past the flat all along the edge. The ordinary run takes a
// sample, -short one chamfer and one fillet; make test-sweeps all.
func TestBlendRunsOffBothFaces(t *testing.T) {
	t.Parallel()
	type tc struct {
		chamfer  bool
		c, h0, r float64
	}
	cases := []tc{
		{true, 3, 1, 2.15}, {true, 3, 1, 2.3}, {true, 3, 0.5, 2.3}, {true, 2.5, 0.5, 1.6}, {true, 3, 0.2, 2.3},
		{false, 3, 1, 4}, {false, 3, 0.5, 4}, {false, 2.5, 1, 3.5}, {false, 3, 1, 4.5}, {false, 3, 1, 5}, {false, 2.5, 0.5, 3}, {false, 3, 0.2, 5.5},
	}
	cases = sweep.Pick(cases, 4)
	if (testing.Short() || fixtures.RaceDetector) && !sweep.Full() {
		cases = []tc{{true, 3, 1, 2.15}, {false, 3, 1, 4}}
	}
	for _, c := range cases {
		kind := "fillet"
		if c.chamfer {
			kind = "chamfer"
		}
		t.Run(fmt.Sprintf("%s of %g, flat at %g, top %g", kind, c.r, c.c, c.h0), func(t *testing.T) {
			t.Parallel()
			d := New()
			d.Edit.SetChordTolerance(0.003)
			d.Edit.SetMaxSegments(200)
			edge, before := flatConePlate(d, c.c, c.h0)
			g, err := blendOrRefusal(d, edge, c.r, c.chamfer)
			if err != nil {
				t.Fatal(err)
			}
			rep := g.Analyze().Check()
			if !rep.Sound() {
				t.Fatal(rep.Summary())
			}
			want, err := bothFacesOracle(c.chamfer, c.r, c.c, c.h0)
			if err != nil {
				t.Fatal(err)
			}
			added := rep.Volume - before
			t.Logf("added %.5f, oracle %.5f (%.3f%%)", added, want, 100*(added-want)/want)
			if math.Abs(added-want) > 0.003*want {
				t.Errorf("adds %.5f, want %.5f", added, want)
			}
		})
	}
}

// Each side's run-off of flatConePlate's edge alone is made, onto the
// plate's top (the cone uncut) and onto the flat (the plate's top high).
func TestBlendRunsOffOneOfTwoFaces(t *testing.T) {
	t.Parallel()
	cases := []struct {
		chamfer  bool
		c, h0, r float64
	}{
		{true, 0, 1, 2.15},
		{true, 3, 4, 2.15},
		{false, 3, 4, 4},
	}
	for _, c := range cases {
		kind := "fillet"
		if c.chamfer {
			kind = "chamfer"
		}
		t.Run(fmt.Sprintf("%s of %g, flat at %g, top %g", kind, c.r, c.c, c.h0), func(t *testing.T) {
			t.Parallel()
			d := New()
			d.Edit.SetChordTolerance(0.003)
			d.Edit.SetMaxSegments(200)
			edge, _ := flatConePlate(d, c.c, c.h0)
			g, err := blendOrRefusal(d, edge, c.r, c.chamfer)
			if err != nil {
				t.Fatal(err)
			}
			if rep := g.Analyze().Check(); !rep.Sound() {
				t.Fatal(rep.Summary())
			}
		})
	}
}
