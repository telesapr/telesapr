package api

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// peakedConePlate is flatConePlate's cone and flat with a plate whose
// top is two planes, from h0 above the foot of its side on the cone at
// x = 0 up to z = 10 at x = 5 and down to h1 above that foot at x = 10:
// low at both ends, so that a blend of the edge runs off the plate's
// side onto its top at x = 10 too, and high between them, where it runs
// off nothing.
func peakedConePlate(d *Doc, c, h0, h1 float64) (SolidEdge, float64) {
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
	d.Features.SketchIn(d.Workplane("ZX"), "")
	d.Sketch.Polyline(true, V(0, 0, 2), V(10, 0, 2), V(10, 0, math.Sqrt(15)+h1), V(5, 0, 10),
		V(0, 0, math.Sqrt(48)+h0))
	plate := d.Features.Extrude(2, "plate")
	plate.SetTwoSided(true)
	return plate.Solid().EdgeAt(V(5, 1, math.Sqrt(5.5*5.5-1))), plate.Analyze().Check().Volume
}

// peakedConeTop is the height of peakedConePlate's top at x.
func peakedConeTop(h0, h1, x float64) float64 {
	if x < 5 {
		z0 := math.Sqrt(48) + h0
		return z0 + (10-z0)*x/5
	}
	z1 := math.Sqrt(15) + h1
	return 10 + (z1-10)*(x-5)/5
}

// mixedEndsOracle is what a blend of size r of peakedConePlate's edge
// adds, measured as bothFacesOracle measures it: the same blend of the
// plate whose top is 4 above the foot at both ends, on the cone uncut,
// which runs off no face, less what of it lies above the top's planes
// at h0 and h1, or past the flat at c and above the cone, by vertical
// lines through its mesh. A column past the flat counts what lies above
// the lower of the two heights, once.
func mixedEndsOracle(chamfer bool, r, c, h0, h1 float64) (float64, error) {
	d := New()
	d.Edit.SetChordTolerance(0.003)
	d.Edit.SetMaxSegments(200)
	edge, before := peakedConePlate(d, 0, 4, 4)
	g, err := blendOrRefusal(d, edge, r, chamfer)
	if err != nil {
		return 0, err
	}
	rep := g.Analyze().Check()
	if !rep.Sound() {
		return 0, fmt.Errorf("the blend that runs off no face: %s", rep.Summary())
	}
	above := volumeAbove(meshOver(g, 0, 10, 1, 8), 0, 10, 1, 8, 1600, func(x, y float64) float64 {
		z := peakedConeTop(h0, h1, x)
		if y > c {
			rx := 7 - 0.3*x
			z = math.Min(z, math.Sqrt(math.Max(0, rx*rx-y*y)))
		}
		return z
	})
	return rep.Volume - before - above, nil
}

// Blends of peakedConePlate's edge, one end running off both its faces
// (the plate's side onto its top and the cone onto the flat, at x = 0)
// and the other one face (the side onto the top, at x = 10). Made since
// 2026-11-18: each end is cut by its own next faces, the end at x = 0 as
// TestBlendRunsOffBothFaces's is, the other as a run-off alone; refused
// before as "runs off both its faces at one end and a face at the
// other". Sound, and adding what mixedEndsOracle measures within 0.1%
// (0.02% measured at most, the fillets of 4). The ordinary run takes a
// sample, -short one chamfer and one fillet; make test-sweeps all.
func TestBlendRunsOffBothFacesAndOne(t *testing.T) {
	t.Parallel()
	type tc struct {
		chamfer bool
		h1, r   float64
	}
	cases := []tc{
		{true, 0.2, 2.15}, {true, 0.5, 2.15}, {true, 1, 2.15},
		{false, 0.2, 3.5}, {false, 0.5, 3.5}, {false, 1, 3.5},
		{false, 0.2, 4}, {false, 0.5, 4}, {false, 1, 4},
	}
	cases = sweep.Pick(cases, 4)
	if (testing.Short() || fixtures.RaceDetector) && !sweep.Full() {
		cases = []tc{{true, 0.5, 2.15}, {false, 0.5, 4}}
	}
	const c, h0 = 3, 1
	for _, tc := range cases {
		kind := "fillet"
		if tc.chamfer {
			kind = "chamfer"
		}
		t.Run(fmt.Sprintf("%s of %g, top %g at the narrow end", kind, tc.r, tc.h1), func(t *testing.T) {
			t.Parallel()
			d := New()
			d.Edit.SetChordTolerance(0.003)
			d.Edit.SetMaxSegments(200)
			edge, before := peakedConePlate(d, c, h0, tc.h1)
			g, err := blendOrRefusal(d, edge, tc.r, tc.chamfer)
			if err != nil {
				t.Fatal(err)
			}
			rep := g.Analyze().Check()
			if !rep.Sound() {
				t.Fatal(rep.Summary())
			}
			want, err := mixedEndsOracle(tc.chamfer, tc.r, c, h0, tc.h1)
			if err != nil {
				t.Fatal(err)
			}
			added := rep.Volume - before
			t.Logf("added %.5f, oracle %.5f (%.3f%%)", added, want, 100*(added-want)/want)
			if math.Abs(added-want) > 0.001*want {
				t.Errorf("adds %.5f, want %.5f", added, want)
			}
		})
	}
}

// A blend of peakedConePlate's edge whose two ends' cuts overlap along
// it, the cut of the end at x = 10 reaching back past where the other's
// begins, is refused by name (runOffApart), not made wrong: a chamfer
// of 2.25 or a fillet of 4.5 (-short the chamfer).
func TestBlendRunsOffBothFacesAndOneOverlapping(t *testing.T) {
	t.Parallel()
	type tc struct {
		chamfer bool
		r       float64
	}
	cases := []tc{{true, 2.25}, {false, 4.5}}
	if testing.Short() && !sweep.Full() {
		cases = cases[:1]
	}
	for _, c := range cases {
		t.Run(fmt.Sprint(c.chamfer, c.r), func(t *testing.T) {
			t.Parallel()
			d := New()
			d.Edit.SetChordTolerance(0.003)
			d.Edit.SetMaxSegments(200)
			edge, _ := peakedConePlate(d, 3, 1, 1)
			_, err := blendOrRefusal(d, edge, c.r, c.chamfer)
			if err == nil || !strings.Contains(err.Error(), "runs off faces at both its ends") {
				t.Fatalf("got %v, want the refusal of ends whose cuts overlap", err)
			}
		})
	}
}
