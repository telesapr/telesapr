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

// slantConePlate is flatConePlate with its flat at a slant: the cone's
// side past y = c0 + s·x is cut away, so that the flat comes in close
// beside the plate's side at x = 0 and leaves room for the blend's foot
// on the cone further along. A blend whose end at x = 0 runs off both
// its faces then crosses the flat high on its end section, above where
// it crosses the plate's low top: the two cuts cross on the blend.
func slantConePlate(d *Doc, c0, s, h0 float64) (SolidEdge, float64) {
	d.Features.SketchIn(d.Workplane("ZX"), "")
	axis := d.Sketch.Line(V(-2, 0, 0), V(16, 0, 0))
	axis.SetConstruction(true)
	d.Sketch.Polyline(true, V(0, 0, 0), V(14, 0, 0), V(14, 0, 2.8), V(0, 0, 7))
	d.Features.Lathe(axis, "cone")
	d.Features.Sketch3D("flatplane")
	d.Features.SketchIn(d.Sketch.NewWorkplane(V(0, 0, -8), V(1, 0, 0), V(0, 1, 0)), "")
	d.Sketch.Polyline(true, V(-1, c0-s, -8), V(15, c0+15*s, -8), V(15, 9+15*s, -8), V(-1, 9, -8))
	cut := d.Features.Extrude(16, "flat")
	cut.SetCombine(sketch.CombineDifference)
	d.Features.SketchIn(d.Workplane("ZX"), "")
	d.Sketch.Polyline(true, V(0, 0, 2), V(10, 0, 2), V(10, 0, 9), V(0, 0, math.Sqrt(48)+h0))
	plate := d.Features.Extrude(2, "plate")
	plate.SetTwoSided(true)
	return plate.Solid().EdgeAt(V(5, 1, math.Sqrt(5.5*5.5-1))), plate.Analyze().Check().Volume
}

// crossedOracle is what a blend of size r of slantConePlate's edge adds,
// measured as bothFacesOracle measures it: the same blend of the plate
// whose top is at 4 on the cone uncut, which runs off neither face, less
// what of it lies above the top's plane at h0, or past the flat and
// above the cone, by vertical lines through its mesh.
func crossedOracle(chamfer bool, r, c0, s, h0 float64) (float64, error) {
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
	above := volumeAbove(meshOver(g, 0, 10, 1, 8), 0, 10, 1, 8, 1600, func(x, y float64) float64 {
		z := flatConeTop(h0, x)
		if y > c0+s*x {
			rx := 7 - 0.3*x
			z = math.Min(z, math.Sqrt(math.Max(0, rx*rx-y*y)))
		}
		return z
	})
	return rep.Volume - before - above, nil
}

// Blends of slantConePlate's edge whose end at x = 0 runs off both its
// faces and whose two cuts cross on the blend: its end section lies
// wholly above the plate's top or past the flat. Made since 2026-11-23
// (topo's rollingrunoff_crossed.go): the top and the flat meet across
// the blend's end, along the line their planes share from where their
// cuts cross to the end face, the blend's patches past there cut away
// and the end face taking no end section; refused before as "the faces
// past them meet across its end". Sound, and adding what crossedOracle
// measures within 0.1% (0.052% measured at most; the oracle's columns
// and the meshes' chords put the blends that don't cross as far off).
// The ordinary run takes a sample, -short one chamfer and one fillet;
// make test-sweeps all.
func TestBlendRunsOffBothFacesCutsCrossing(t *testing.T) {
	t.Parallel()
	type tc struct {
		chamfer      bool
		c0, s, h0, r float64
	}
	cases := []tc{
		{false, 1.6, 0.1, 0.2, 2.5}, {false, 1.6, 0.1, 0.2, 3}, {false, 1.6, 0.1, 0.2, 4}, {false, 1.6, 0.1, 0.5, 4},
		{false, 1.6, 0.2, 0.2, 2.5}, {false, 1.6, 0.2, 0.2, 3}, {false, 1.6, 0.2, 0.2, 4}, {false, 2, 0.1, 0.2, 4},
		{true, 1.6, 0.1, 0.2, 1.5}, {true, 1.6, 0.2, 0.2, 2}, {true, 1.6, 0.2, 0.5, 3}, {true, 2, 0.1, 0.2, 1.5},
		{true, 2, 0.2, 0.2, 2.5}, {true, 2, 0.2, 0.5, 3}, {true, 2.4, 0.1, 0.2, 2.5}, {true, 2.4, 0.2, 0.5, 2.5},
	}
	cases = sweep.Pick(cases, 6)
	if (testing.Short() || fixtures.RaceDetector) && !sweep.Full() {
		cases = []tc{{false, 1.6, 0.2, 0.2, 3}, {true, 2, 0.2, 0.2, 2.5}}
	}
	for _, c := range cases {
		kind := "fillet"
		if c.chamfer {
			kind = "chamfer"
		}
		t.Run(fmt.Sprintf("%s of %g, flat at %g+%gx, top %g", kind, c.r, c.c0, c.s, c.h0), func(t *testing.T) {
			t.Parallel()
			d := New()
			d.Edit.SetChordTolerance(0.003)
			d.Edit.SetMaxSegments(200)
			edge, before := slantConePlate(d, c.c0, c.s, c.h0)
			g, err := blendOrRefusal(d, edge, c.r, c.chamfer)
			if err != nil {
				t.Fatal(err)
			}
			rep := g.Analyze().Check()
			if !rep.Sound() {
				t.Fatal(rep.Summary())
			}
			want, err := crossedOracle(c.chamfer, c.r, c.c0, c.s, c.h0)
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

// shortConePlate is peakedConePlate's plate, low at both ends, on a cone
// that ends with it at x = 10 (radius 7 at x = 0, 4 at x = 10), whose
// side past the flats y = c0 + (cm-c0)·x/5 up to x = 5 and y = cm +
// (c1-cm)·(x-5)/5 on from there is cut away (none where c0 is 0): the
// flats come in close beside the plate at both ends, so that a blend of
// the edge runs off both its faces at both ends, the plate's side onto
// its top and the cone onto a flat, the end faces at x = 0 and x = 10
// each one plane across the plate and the cone.
func shortConePlate(d *Doc, c0, cm, c1, h0, h1 float64) (SolidEdge, float64) {
	d.Features.SketchIn(d.Workplane("ZX"), "")
	axis := d.Sketch.Line(V(-2, 0, 0), V(16, 0, 0))
	axis.SetConstruction(true)
	d.Sketch.Polyline(true, V(0, 0, 0), V(10, 0, 0), V(10, 0, 4), V(0, 0, 7))
	d.Features.Lathe(axis, "cone")
	if c0 > 0 {
		d.Features.Sketch3D("flatplane")
		d.Features.SketchIn(d.Sketch.NewWorkplane(V(0, 0, -8), V(1, 0, 0), V(0, 1, 0)), "")
		s0, s1 := (cm-c0)/5, (c1-cm)/5
		d.Sketch.Polyline(true, V(-1, c0-s0, -8), V(5, cm, -8), V(15, cm+10*s1, -8), V(15, 9, -8), V(-1, 9, -8))
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

// bothEndsOracle is what a blend of size r of shortConePlate's edge
// adds, measured as mixedEndsOracle measures it: the same blend of the
// plate whose top is 4 above the foot at both ends, on the cone cut by
// a flat at y = 20, which takes nothing of it, less what of it lies
// above the top's planes, or past the flats and above the cone, by
// vertical lines through its mesh. The flat that takes nothing keeps
// the cone's faces and chords as the measured solid has them: on the
// cone uncut the blend's own volume moves by up to 0.16% (a fillet of 2
// at a chord tolerance of 0.003, 3.9315 uncut, 3.9253 cut at 20; 3.9218
// and 3.9230 at 0.001), and the oracle with it.
func bothEndsOracle(chamfer bool, r, c0, cm, c1, h0, h1 float64) (float64, error) {
	d := New()
	d.Edit.SetChordTolerance(0.003)
	d.Edit.SetMaxSegments(200)
	edge, before := shortConePlate(d, 20, 20, 20, 4, 4)
	g, err := blendOrRefusal(d, edge, r, chamfer)
	if err != nil {
		return 0, err
	}
	rep := g.Analyze().Check()
	if !rep.Sound() {
		return 0, fmt.Errorf("the blend that runs off no face: %s", rep.Summary())
	}
	flat := func(x float64) float64 {
		if x < 5 {
			return c0 + (cm-c0)*x/5
		}
		return cm + (c1-cm)*(x-5)/5
	}
	above := volumeAbove(meshOver(g, 0, 10, 1, 8), 0, 10, 1, 8, 1600, func(x, y float64) float64 {
		z := peakedConeTop(h0, h1, x)
		if y > flat(x) {
			rx := 7 - 0.3*x
			z = math.Min(z, math.Sqrt(math.Max(0, rx*rx-y*y)))
		}
		return z
	})
	return rep.Volume - before - above, nil
}

// Blends of shortConePlate's edge, both of whose ends run off both its
// faces. Made since 2026-11-23: each end is cut by its own next faces,
// as TestBlendRunsOffBothFaces's end is; refused before as "runs off
// both its faces at both its ends". Sound, and adding what
// bothEndsOracle measures within 0.1% (0.044% measured at most). The
// ordinary run takes a sample, -short one chamfer and one fillet; make
// test-sweeps all.
func TestBlendRunsOffBothFacesAtBothEnds(t *testing.T) {
	t.Parallel()
	type tc struct {
		chamfer   bool
		c1, cm, r float64
	}
	cases := []tc{
		{false, 1.4, 4, 1.5}, {false, 1.4, 5, 4}, {false, 1.7, 4, 2.5}, {false, 1.7, 5, 3}, {false, 2, 4, 2}, {false, 2, 5, 4},
		{true, 1.4, 4, 1.5}, {true, 1.4, 5, 3}, {true, 1.7, 4, 2}, {true, 1.7, 5, 2.5}, {true, 2, 4, 3}, {true, 2, 5, 1.5},
	}
	cases = sweep.Pick(cases, 4)
	if (testing.Short() || fixtures.RaceDetector) && !sweep.Full() {
		cases = []tc{{false, 1.7, 4, 2.5}, {true, 1.7, 4, 2}}
	}
	const c0, h = 2.4, 0.5
	for _, c := range cases {
		kind := "fillet"
		if c.chamfer {
			kind = "chamfer"
		}
		t.Run(fmt.Sprintf("%s of %g, flats %g, %g, %g", kind, c.r, c0, c.cm, c.c1), func(t *testing.T) {
			t.Parallel()
			d := New()
			d.Edit.SetChordTolerance(0.003)
			d.Edit.SetMaxSegments(200)
			edge, before := shortConePlate(d, c0, c.cm, c.c1, h, h)
			g, err := blendOrRefusal(d, edge, c.r, c.chamfer)
			if err != nil {
				t.Fatal(err)
			}
			rep := g.Analyze().Check()
			if !rep.Sound() {
				t.Fatal(rep.Summary())
			}
			want, err := bothEndsOracle(c.chamfer, c.r, c0, c.cm, c.c1, h, h)
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

// Chamfers of 4 of shortConePlate's edge are refused by name, not made
// wrong: with the middle flat at 5 the two ends' cuts overlap along the
// edge (runOffApart), at 4 the chamfer reaches over the plate's side
// all along (consumedRunOff), which with ends running off both faces
// it can't follow (-short the first).
func TestBlendRunsOffBothFacesAtBothEndsRefused(t *testing.T) {
	t.Parallel()
	cases := []struct {
		cm   float64
		want string
	}{
		{5, "runs off faces at both its ends"},
		{4, "takes the face on side 1 whole and runs off both its faces at one end"},
	}
	if testing.Short() && !sweep.Full() {
		cases = cases[:1]
	}
	for _, c := range cases {
		t.Run(fmt.Sprint(c.cm), func(t *testing.T) {
			t.Parallel()
			d := New()
			d.Edit.SetChordTolerance(0.003)
			d.Edit.SetMaxSegments(200)
			edge, _ := shortConePlate(d, 2.4, c.cm, 1.4, 0.5, 0.5)
			_, err := blendOrRefusal(d, edge, 4, true)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want the refusal %q", err, c.want)
			}
		})
	}
}
