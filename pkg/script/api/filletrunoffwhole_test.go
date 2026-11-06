package api

import (
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// gabledAdded is what a fillet (or chamfer) of size r of the edge of
// gabledPlateOnRod adds, integrated: the blend's section across the
// edge, between the plate's side, the rod and the ball's arc (or the
// chamfer's line from r up the plate to the point of the rod r from the
// foot), is the same along it, and is cut by the plate's ends and by the
// slopes of its top, whose height depends on x alone. It matches
// OpenCASCADE's makeFillet and makeChamfer to the last digit printed
// where they make the blend (fillets of 1 on plates 2 long, 1.8 on 3.5;
// chamfers of 1 on 2 and 1.4 on 3.5).
func gabledAdded(l, hEnd, hMid, r float64, chamfer bool) float64 {
	foot := math.Sqrt(24)
	top := func(x float64) float64 {
		return foot + hEnd + (hMid-hEnd)*math.Min(x, l-x)/(l/2)
	}
	upper, y1 := blendSectionTop(r, chamfer)
	const nx, ny = 600, 2000
	sum := 0.0
	for i := range nx {
		x := (float64(i) + 0.5) / nx * l
		h := top(x)
		for j := range ny {
			y := 1 + (float64(j)+0.5)/ny*(y1-1)
			if d := math.Min(upper(y), h) - math.Sqrt(25-y*y); d > 0 {
				sum += d
			}
		}
	}
	return sum * (y1 - 1) / ny * l / nx
}

// blendSectionTop is the top of the section of a blend of size r of the
// edge a plate's side at y = 1 stands on a rod of radius 5 along x by,
// as a height over y, and how far out in y it reaches.
func blendSectionTop(r float64, chamfer bool) (func(y float64) float64, float64) {
	foot := math.Sqrt(24)
	if chamfer {
		a := math.Atan2(foot, 1) - 2*math.Asin(r/10)
		qy, qz := 5*math.Cos(a), 5*math.Sin(a)
		return func(y float64) float64 { return foot + r + (qz-foot-r)*(y-1)/(qy-1) }, qy
	}
	zc := math.Sqrt((5+r)*(5+r) - (1+r)*(1+r))
	return func(y float64) float64 { return zc - math.Sqrt(math.Max(0, r*r-(y-1-r)*(y-1-r))) }, 5 * (1 + r) / (5 + r)
}

// Blends of the edge a short plate with a gabled top stands on a rod by,
// so large that the ball's contact with the plate's side lies above one
// slope of the top or the other all along: the blend takes the side
// whole (topo's consumedRunOff). Both slopes cut the blend, meeting
// across it on the ridge carried on; each slope grows by its cut and by
// the ridge's line out to where the cuts meet, the ends by the end
// sections, and the side goes. Refused before 2026-11-04 as running off
// "past the next one too" or "all along"; OpenCASCADE refuses every
// one. Where the end section reaches above the far slope's plane
// carried on past the ridge, as the largest blends of the shortest plates
// do, it reaches above the near slope there too, which cuts that part
// away first: refused as running off "at both its ends" before
// 2026-11-05. Each is sound and adds what gabledAdded integrates: chamfers
// within 0.3%, fillets within six times the chord tolerance (they come
// out 2 to 3.6 times it over, the meshes' chords; a fillet of this edge
// whose top is too tall to run off converges on its section's integral
// as the chord tolerance is made finer). Under the ear clipper, which
// meshed curved faces before the constrained Delaunay mesher, the
// fillets came out up to 0.02 short, at any chord tolerance: its
// triangles strayed past the tolerance. The ordinary run takes the
// largest blend of each kind on each plate, and the smallest on the
// shortest: 12 of the 20, side by side; make test-sweeps all of them.
func TestBlendTakesSideFaceWhole(t *testing.T) {
	t.Parallel()
	type tc struct {
		chamfer bool
		l, r    float64
	}
	cases := []tc{
		{false, 1.5, 1.4}, {false, 1.5, 1.8}, {false, 1.5, 2.2}, {false, 2, 1.8}, {false, 2, 2.2},
		{false, 2.5, 1.8}, {false, 2.5, 2.2}, {false, 3, 2.2}, {false, 3.5, 2.2},
		{true, 1.5, 1}, {true, 1.5, 1.4}, {true, 2, 1.4}, {true, 2, 1.8}, {true, 2.5, 1.4}, {true, 2.5, 1.8},
		{true, 2.5, 2.2}, {true, 3, 1.4}, {true, 3, 1.8}, {true, 3.5, 1.8}, {true, 3.5, 2.2},
	}
	switch {
	case sweep.Full():
	case testing.Short() || fixtures.RaceDetector:
		cases = []tc{{false, 2.5, 2.2}, {true, 3, 1.4}}
	default:
		type kl struct {
			chamfer bool
			l       float64
		}
		largest, smallest := map[kl]float64{}, map[bool]float64{}
		for _, c := range cases {
			largest[kl{c.chamfer, c.l}] = max(largest[kl{c.chamfer, c.l}], c.r)
			if c.l == 1.5 && (smallest[c.chamfer] == 0 || c.r < smallest[c.chamfer]) {
				smallest[c.chamfer] = c.r
			}
		}
		cases = slices.DeleteFunc(cases, func(c tc) bool {
			return c.r != largest[kl{c.chamfer, c.l}] && (c.l != 1.5 || c.r != smallest[c.chamfer])
		})
	}
	for _, c := range cases {
		kind := "fillet"
		if c.chamfer {
			kind = "chamfer"
		}
		name := fmt.Sprintf("%s l %g r %g", kind, c.l, c.r)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			blendTakesSideFaceWhole(t, c.chamfer, c.l, c.r)
		})
	}
}

// blendTakesSideFaceWhole is a case of TestBlendTakesSideFaceWhole: the
// blend of size r on the plate l long, a chamfer or a fillet.
func blendTakesSideFaceWhole(t *testing.T, chamfer bool, l, r float64) {
	d := New()
	d.Edit.SetChordTolerance(0.003)
	d.Edit.SetMaxSegments(200)
	hMid := 0.58 + l/4
	edge, before := gabledPlateOnRod(d, 10, l, 0.58, hMid)
	g, err := blendOrRefusal(d, edge, r, chamfer)
	if err != nil {
		t.Fatal(err)
	}
	rep := g.Analyze().Check()
	if !rep.Sound() {
		t.Fatal(rep.Summary())
	}
	added, want := rep.Volume-before, gabledAdded(l, 0.58, hMid, r, chamfer)
	var chord float64
	d.With(func(sk *sketch.Sketch) { chord = sk.Tol().ChordTolMm() })
	tol := 6 * chord
	if chamfer {
		tol = 0.003 * want
	}
	if math.Abs(added-want) > tol {
		t.Errorf("added %g, %g integrated", added, want)
	}
}
