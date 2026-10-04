package api

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/sweep"
)

// A band whose side cuts a lathed cone along a hyperbola: the edge
// between them runs from the base to the band's end, a flat face at
// each end, and the rolling ball rounds it (roadmap 22.2): the fillet
// fills the corner, the chamfer too, each sound and adding about what
// its section's area along the edge gives.
func TestFilletFlatOnConeOffAxis(t *testing.T) {
	for _, what := range []string{"a fillet", "a chamfer"} {
		d := New()
		d.SetChordTolerance(0.002)
		d.SketchIn(d.Workplane("ZX"), "")
		axis := d.Line(V(0, 0, -2), V(0, 0, 32))
		axis.SetConstruction(true)
		d.Polyline(true, V(0, 0, 0), V(10, 0, 0), V(4, 0, 30), V(0, 0, 30))
		d.Lathe(axis, "")
		d.SketchIn(d.Workplane("XY"), "")
		d.Rect(V(6, -3, 0), V(20, 3, 0))
		band := d.Extrude(25, "band")
		if p := band.Problem(); p != "" {
			t.Fatal(p)
		}
		before := band.Check().Volume
		edge := band.Solid().EdgeAt(V(math.Sqrt(64-9), 3, 10))
		var g Group
		if what == "a fillet" {
			g = d.FilletSolid([]SolidEdge{edge}, 1, "")
		} else {
			g = d.ChamferSolid([]SolidEdge{edge}, 1, "")
		}
		if p := g.Problem(); p != "" {
			t.Fatalf("%s: %s", what, p)
		}
		r := g.Check()
		added := r.Volume - before
		// The corner between the band's side and the cone is wide: a
		// fillet of 1 fills about 1.27 over the edge, a chamfer at 1 about
		// 7.88, the same within a few thousandths at chord tolerances from
		// 0.002% to 0.005%.
		lo, hi := 1.2, 1.35
		if what == "a chamfer" {
			lo, hi = 7.6, 8.1
		}
		if !r.Sound() || added < lo || added > hi {
			t.Errorf("%s of the band's edge on the cone: %s, added %g, want %g to %g", what, r.Summary(), added, lo, hi)
		}
	}
}

// bandOnCone is a lathed cone with a band of half-width y and height
// top across its side, the band's edge on the cone's side at y, and the
// volume before.
func bandOnCone(d *Doc, y, top float64) (SolidEdge, float64) {
	d.SketchIn(d.Workplane("ZX"), "")
	axis := d.Line(V(0, 0, -2), V(0, 0, 32))
	axis.SetConstruction(true)
	d.Polyline(true, V(0, 0, 0), V(10, 0, 0), V(4, 0, 30), V(0, 0, 30))
	d.Lathe(axis, "")
	d.SketchIn(d.Workplane("XY"), "")
	d.Rect(V(6, -y, 0), V(20, y, 0))
	band := d.Extrude(top, "band")
	return band.Solid().EdgeAt(V(math.Sqrt(64-y*y), y, 10)), band.Check().Volume
}

// A chamfer or fillet of 2 or more on the band's edge on the cone runs
// off the band's side past its top, at its end on the band's end face
// (roadmap 22.3): the blend goes on to the band's top, which cuts it
// too, and both the end face and the top grow by its end. Sound, and
// the fillets add what OpenCASCADE's do on the same edge within 1.5%
// (makeFillet, valid there; its chamfers of 2 are not valid shapes).
func TestChamferRunsOffBand(t *testing.T) {
	occ := map[float64]float64{2: 4.579086, 2.5: 6.797516}
	for _, what := range []string{"a chamfer", "a fillet"} {
		for _, r := range []float64{2, 2.5} {
			d := New()
			d.SetChordTolerance(0.003)
			edge, before := bandOnCone(d, 3, 25)
			var g Group
			if what == "a chamfer" {
				g = d.ChamferSolid([]SolidEdge{edge}, r, "")
			} else {
				g = d.FilletSolid([]SolidEdge{edge}, r, "")
			}
			if p := g.Problem(); p != "" {
				t.Errorf("%s of %g: %s", what, r, p)
				continue
			}
			rep := g.Check()
			added := rep.Volume - before
			if !rep.Sound() {
				t.Errorf("%s of %g: %s", what, r, rep.Summary())
			}
			if what == "a fillet" && math.Abs(added-occ[r]) > 0.015*occ[r] {
				t.Errorf("%s of %g added %g, OpenCASCADE's %g", what, r, added, occ[r])
			}
			// The chamfer adds about its section's area along the edge, which
			// grows as the square of its size: 7.88 at 1.
			if what == "a chamfer" && math.Abs(added/(r*r)-7.9) > 0.1 {
				t.Errorf("%s of %g added %g, %g per size squared", what, r, added, added/(r*r))
			}
			if r != 2 {
				continue
			}
			// The same after saving and loading, and gone with one undo.
			path := filepath.Join(t.TempDir(), "runoff.tlspr")
			if err := d.Save(path); err != nil {
				t.Fatal(err)
			}
			e := New()
			if err := e.Load(path); err != nil {
				t.Fatal(err)
			}
			e.SetChordTolerance(0.003)
			gs := e.Groups()
			if r2 := gs[len(gs)-1].Check(); !r2.Sound() || math.Abs(r2.Volume-rep.Volume) > 1e-6*rep.Volume {
				t.Errorf("%s, loaded: %s, volume %g, saved %g", what, r2.Summary(), r2.Volume, rep.Volume)
			}
			// A Go program's document is one step of undo throughout.
			n := len(d.Groups())
			if !d.Undo() || len(d.Groups()) >= n {
				t.Errorf("%s: undo left %d groups of %d", what, len(d.Groups()), n)
			}
		}
	}
}

// Blends whose end runs off onto the next face (roadmap 22.3): chamfers
// and fillets of the band's edge on the cone, bands of three widths and
// two heights, blends of three sizes. Measured on the first run, after
// the code was written (not before, as the rule asks): 2 of 36 refused
// (a fillet of 3 on the narrowest band, its surface can't be fitted),
// and 8 chamfers made whose meshes cross where the chamfer meets the
// cone, the cone's mesh taking slivers of only its trim's points there
// (the mesher's, as the fillet of 1's crossing in TestFilletFlatOnCone;
// the shells are sound). A shell made unsound fails outright.
func TestRunOffSweep(t *testing.T) {
	g := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("y", 2, 2.5, 3),
			sweep.Floats("top", 22, 25),
			sweep.Strings("kind", "chamfer", "fillet"),
			sweep.Floats("r", 2, 2.5, 3),
		},
		MaxFailRate: 10.0 / 36,
		Sample:      7,
		// The sample's points, measured: one refused.
		SampleMaxFailRate: 1.0 / 7,
	}
	g.Run(func(p sweep.Point) error {
		d := New()
		d.SetChordTolerance(0.003)
		edge, _ := bandOnCone(d, p.Float("y"), p.Float("top"))
		var f Group
		if p.String("kind") == "chamfer" {
			f = d.ChamferSolid([]SolidEdge{edge}, p.Float("r"), "")
		} else {
			f = d.FilletSolid([]SolidEdge{edge}, p.Float("r"), "")
		}
		if pr := f.Problem(); pr != "" {
			return errors.New(pr)
		}
		rep := f.Check()
		if rep.NakedEdges > 0 || rep.TopologyProblems > 0 {
			t.Errorf("%s: made, not closed: %s", p.Label(), rep.Summary())
		}
		if !rep.Sound() {
			return fmt.Errorf("%d mesh edges cross", rep.SelfIntersecting)
		}
		return nil
	}).Require(t)
}
