package api

import (
	"fmt"
	"math"
	"strings"
	"testing"

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

// Blends of the edge a plate stands on a cone by whose end runs off
// both its faces at the plate's end: the plate's side past its top,
// which is low there, and the cone past its flat (flatConePlate). Each
// side's run-off alone is made, onto the plate's top (the cone uncut)
// and onto the flat (the plate's top high); both at once the end would
// be cut by both next faces and the end face grow by the end section
// between their two cuts. That is not made: the blend is refused at the
// place, saying so. Before 2026-11-09 no test reached this, and the
// refusal said the face past the end "is curved or meets the others
// otherwise".
func TestBlendRunsOffBothFacesRefused(t *testing.T) {
	t.Parallel()
	cases := []struct {
		chamfer   bool
		c, h0, r  float64
		bothFaces bool
	}{
		{true, 3, 1, 2.15, true},
		{false, 3, 1, 4, true},
		{true, 0, 1, 2.15, false},
		{true, 3, 4, 2.15, false},
		{false, 3, 4, 4, false},
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
			if c.bothFaces {
				if err == nil || !strings.Contains(err.Error(), "runs off both its faces") {
					t.Fatalf("%v, want a refusal saying it runs off both its faces", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if rep := g.Analyze().Check(); !rep.Sound() {
				t.Fatal(rep.Summary())
			}
		})
	}
}
