package api

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
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

// takesSideRefused are the blends of TestBlendTakesSideFaceWhole that
// are refused: the end section is cut by both slopes, the far one's
// carried on past the ridge, which the next faces' growing over the
// fillet up to the ridge doesn't follow.
var takesSideRefused = map[string]bool{
	"fillet l 1.5 r 2.2": true, "chamfer l 1.5 r 1.4": true, "chamfer l 2 r 1.8": true,
	"chamfer l 2.5 r 2.2": true,
}

// Blends of the edge a short plate with a gabled top stands on a rod by,
// so large that the ball's contact with the plate's side lies above one
// slope of the top or the other all along: the blend takes the side
// whole (topo's consumedRunOff). Both slopes cut the blend, meeting
// across it on the ridge carried on; each slope grows by its cut and by
// the ridge's line out to where the cuts meet, the ends by the end
// sections, and the side goes. Refused before 2026-11-04 as running off
// "past the next one too" or "all along"; OpenCASCADE refuses every
// one. Each is sound and adds what gabledAdded integrates: chamfers
// within 0.3%, fillets within 0.03, as a fillet of this edge whose top
// is too tall to run off does at 1.8 (0.02 short), the fillet's mesh
// lying inside its surface.
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
	if testing.Short() || fixtures.RaceDetector {
		cases = []tc{{false, 2.5, 2.2}, {true, 3, 1.4}}
	}
	for _, c := range cases {
		kind := "fillet"
		if c.chamfer {
			kind = "chamfer"
		}
		name := fmt.Sprintf("%s l %g r %g", kind, c.l, c.r)
		d := New()
		d.Edit.SetChordTolerance(0.003)
		d.Edit.SetMaxSegments(200)
		hMid := 0.58 + c.l/4
		edge, before := gabledPlateOnRod(d, 10, c.l, 0.58, hMid)
		g, err := blendOrRefusal(d, edge, c.r, c.chamfer)
		if takesSideRefused[name] {
			if err == nil || !strings.Contains(err.Error(), "runs off the faces at both its ends") {
				t.Errorf("%s: %v, want a refusal", name, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		rep := g.Analyze().Check()
		if !rep.Sound() {
			t.Errorf("%s: %s", name, rep.Summary())
			continue
		}
		added, want := rep.Volume-before, gabledAdded(c.l, 0.58, hMid, c.r, c.chamfer)
		tol := 0.03
		if c.chamfer {
			tol = 0.003 * want
		}
		if math.Abs(added-want) > tol {
			t.Errorf("%s added %g, %g integrated", name, added, want)
		}
	}
}
