package sketch_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/srf"
)

// solidBits is the group's solid and mesh as text, to the bit: every
// surface's and curve's numbers, names and trims.
func solidBits(d *api.Doc, g api.Group) string {
	d.Groups() // a read: what is pending is regenerated first
	gr := sketchOf(d).Group(g.H)
	var b strings.Builder
	for _, sh := range []*srf.SShell{&gr.RunningShell, &gr.ThisShell} {
		for _, s := range sh.Surface {
			fmt.Fprintf(&b, "s %d %d %d %v %v %v %v %d %d %v\n", s.H.V, s.DegM, s.DegN, s.Ctrl, s.Weight, s.Trim,
				s.Face, s.Origin, s.Sheet, s.Tol)
		}
		for _, c := range sh.Curve {
			fmt.Fprintf(&b, "c %d %v %d %d %v\n", c.H.V, c.Exact, c.SurfA.V, c.SurfB.V, c.Points)
		}
	}
	fmt.Fprintf(&b, "%v %v %q %q\n", gr.RunningMesh.Triangles, gr.ThisMesh.Triangles, gr.Problem.Msg, gr.Note)
	return b.String()
}

// edgesAt are the edges of g's solid all of whose points lie at height
// z.
func edgesAt(g api.Group, z float64) []api.SolidEdge {
	var out []api.SolidEdge
	for _, e := range g.Solid().Edges() {
		on := true
		for _, p := range e.Points() {
			on = on && math.Abs(p.Z-z) < 1e-9
		}
		if on {
			out = append(out, e)
		}
	}
	return out
}

// A fillet's or a chamfer's trial, which says whether its edges can be
// rounded before the group is added or its edges changed, keeps the
// solid it made, and the regeneration after takes it instead of making
// it again: the same solid as one made afresh, to the bit.
func TestEditTrialTaken(t *testing.T) {
	t.Parallel()
	for _, chamfer := range []bool{false, true} {
		t.Run(fmt.Sprintf("chamfer %v", chamfer), func(t *testing.T) {
			t.Parallel()
			d := api.New()
			d.Sketch.Rect(api.V(0, 0, 0), api.V(40, 30, 0))
			d.Sketch.Circle(api.V(20, 15, 0), 6)
			box := d.Features.Extrude(10, "box")
			top := edgesAt(box, 10)
			if len(top) < 5 {
				t.Fatalf("%d edges on top, want the rectangle's and the hole's", len(top))
			}
			sk := sketchOf(d)
			var g api.Group
			if chamfer {
				g = d.Features.ChamferSolid(top[:2], 1.5, "")
			} else {
				g = d.Features.FilletSolid(top[:2], 1.5, "")
			}
			// Each solid taken from a trial is the one made again from
			// nothing kept.
			again := func(what string, taken int) {
				t.Helper()
				if p := g.Problem(); p != "" {
					t.Fatal(p)
				}
				got := solidBits(d, g)
				if n := sk.EditTrialsTaken(); n != taken {
					t.Errorf("%s, %d trials' solids taken, want %d", what, n, taken)
				}
				sk.NoSolidCache = true
				defer func() { sk.NoSolidCache = false }()
				sk.MarkGroupDirty(g.H, false)
				sk.GenerateAll()
				if solidBits(d, g) != got {
					t.Errorf("%s, the solid taken from the trial is not the one made again", what)
				}
			}
			again("made", 1)
			if chamfer {
				g.AsChamfer().SetEdges(nil, top)
			} else {
				g.AsFillet().SetEdges(nil, top)
			}
			again("edited", 2)
		})
	}
}

// A fillet or chamfer whose foot runs off its face past a curved side,
// made on the solid widened past the side and cut back
// (topo.widenedFillet), is made once: by the trial, whose solid the
// regeneration takes, and that is the one made again from nothing kept,
// to the bit. The disc of radius 8 drafted 5°, a rod of radius 4 off its
// centre whose foot runs past the disc's rim (TestFilletPastConeSide's
// hardest case). -short keeps the fillet, a few seconds each.
func TestEditTrialPastCurvedSide(t *testing.T) {
	t.Parallel()
	for _, chamfer := range []bool{false, true} {
		if chamfer && testing.Short() {
			continue
		}
		t.Run(fmt.Sprintf("chamfer %v", chamfer), func(t *testing.T) {
			t.Parallel()
			d := api.New()
			d.Edit.SetChordTolerance(0.003)
			d.Features.SketchIn(d.Workplane("XY"), "")
			d.Sketch.Circle(api.V(0, 0, 0), 8)
			d.Features.Extrude(4, "disc").AsExtrude().SetDraft(5)
			d.Features.Sketch3D("under")
			d.Features.SketchIn(d.Sketch.NewWorkplane(api.V(0, 0, -6), api.V(1, 0, 0), api.V(0, 1, 0)), "")
			d.Sketch.Circle(api.V(2, 0, -6), 4)
			rod := d.Features.Extrude(16, "rod")
			foot := []api.SolidEdge{rod.Solid().EdgeAt(api.V(6, 0, 4))}
			var g api.Group
			if chamfer {
				g = d.Features.ChamferSolid(foot, 2.5, "")
			} else {
				g = d.Features.FilletSolid(foot, 3, "")
			}
			if p := g.Problem(); p != "" {
				t.Fatal(p)
			}
			sk := sketchOf(d)
			got := solidBits(d, g)
			if n := sk.EditTrialsTaken(); n != 1 {
				t.Errorf("%d trials' solids taken, want 1: the regeneration made it again", n)
			}
			if rep := g.Analyze().Check(); !rep.Sound() {
				t.Fatalf("unsound: %s", rep.Summary())
			}
			sk.NoSolidCache = true
			defer func() { sk.NoSolidCache = false }()
			sk.MarkGroupDirty(g.H, false)
			sk.GenerateAll()
			if solidBits(d, g) != got {
				t.Errorf("the solid taken from the trial is not the one made again")
			}
		})
	}
}
