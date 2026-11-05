package sketch_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// lobedBlockTwoSided is lobedBlock extruded 10 both ways from its
// sketch, z from -5 to 5, each half drafted by draft degrees away from
// the sketch, its neutral plane: each side is two faces, one each way,
// meeting there.
func lobedBlockTwoSided(t *testing.T, draft float64) (*api.Doc, api.Group) {
	t.Helper()
	d := api.New()
	d.Edit.SetChordTolerance(0.005)
	a := d.Sketch.Line(api.V(-10, -10, 0), api.V(10, -10, 0))
	b := d.Sketch.Line(a.End().Pos(), api.V(10, 0, 0))
	c := d.Sketch.Line(b.End().Pos(), api.V(4, 0, 0))
	e := d.Sketch.Line(api.V(-4, 0, 0), api.V(-10, 0, 0))
	d.Sketch.Line(e.End().Pos(), a.Start().Pos())
	d.Sketch.Arc(api.V(0, 0, 0), c.End().Pos(), e.Start().Pos())
	ex := d.Features.Extrude(10, "")
	ex.SetTwoSided(true)
	ex.AsExtrude().SetDraft(draft)
	if p := ex.Problem(); p != "" {
		t.Fatal(p)
	}
	return d, ex
}

// lobedArea is the area of the lobed block's section moved in by d all
// round: the block 20 by 10 less 2d each way, and the lobe of radius
// 4 - d above the top edge moved down by d.
func lobedArea(d float64) float64 {
	rho := 4 - d
	return (20-2*d)*(10-2*d) + rho*rho*math.Acos(-d/rho) + d*math.Sqrt(rho*rho-d*d)
}

// The two-sided drafted lobed block hollowed. Open at its top, every
// side is a wall, and the inside is the section moved in by the
// thickness from the bottom moved in to the top: the drafted shell
// takes it, which it didn't before 2026-11-05 (its sides were probed on
// the neutral plane, where they are two faces' seam and on neither).
// Open at the lobe's faces above the sketch only, the lobe below stays
// a wall and the one above runs out, the walls under the top edge
// either side of it stepping short of the corners as a one-sided
// block's do (TestShellDraftedInsideCorner); open both ways, both
// halves step. Until 2026-11-05 the steep ones were refused: the
// Boolean left the crease where the round sides kept meet open from 7°
// or 9° (srf.TestDroppedSurfaceNamedByNone), and the inside's two halves,
// their lobes cut into patches at unlike turns, didn't join at 10°
// (arcPartBounds). At 12° and a thickness of 1, open at the top, the
// inside's sides meet at a corner of its outline.
func TestShellDraftedTwoSidedLobed(t *testing.T) {
	t.Parallel()
	type tcase struct {
		draft, thick float64
		open         string // "top", "lobe above", "lobe both"
	}
	var cases []tcase
	for _, draft := range []float64{2, 5, 8, 10, 12} {
		for _, open := range []string{"top", "lobe above", "lobe both"} {
			if open == "top" && draft > 10 {
				continue
			}
			cases = append(cases, tcase{draft, 1, open})
		}
	}
	cases = append(cases, tcase{5, 0.5, "lobe above"}, tcase{5, 2, "lobe both"},
		tcase{10, 0.5, "top"}, tcase{10, 0.5, "lobe above"}, tcase{10, 2, "lobe both"})
	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {
			t.Parallel()
			if (testing.Short() || fixtures.RaceDetector) && (c.draft != 5 || c.thick != 1) {
				t.Skip("only the cases of draft 5 and thickness 1, short or under the race detector")
			}
			d, ex := lobedBlockTwoSided(t, c.draft)
			tan := math.Tan(c.draft * math.Pi / 180)
			w := c.thick / math.Cos(c.draft*math.Pi/180)
			lobe := func(z float64) []api.SolidFace {
				r := 4 - math.Abs(z)*tan
				var fs []api.SolidFace
				for _, a := range []float64{math.Pi / 4, 3 * math.Pi / 4} {
					fs = append(fs, ex.Solid().FaceAt(api.V(r*math.Cos(a), r*math.Sin(a), z)))
				}
				return fs
			}
			var open []api.SolidFace
			switch c.open {
			case "top":
				open = []api.SolidFace{ex.Solid().FaceAt(api.V(0, -5, 5))}
			case "lobe above":
				open = lobe(2)
			case "lobe both":
				open = append(lobe(2), lobe(-2)...)
			}
			sh := d.Features.ShellSolid(open, c.thick, "")
			if p := sh.Problem(); p != "" {
				t.Fatal(p)
			}
			r := sh.Analyze().Check()
			if !r.Sound() {
				t.Fatalf("shell: %s", r.String())
			}
			var chord float64
			d.With(func(sk *sketch.Sketch) { chord = sk.Tol().ChordTolMm() })
			// How far the walls under the top edge end further from the
			// lobe than the corner, in a half whose lobe is open: the half's
			// inside runs from the sketch to the end moved in.
			step := lobedCase{draft: c.draft, thick: c.thick}
			back := stepBack(step, chord, 0, 5-c.thick)
			stepped := func(at float64) float64 {
				d := at * tan
				xc := math.Sqrt((4-d)*(4-d) - d*d)
				wr, hr := 20-2*d, 10-2*d
				return wr*hr - (wr-2*w)*(hr-2*w) - 2*(xc+back(at))*w
			}
			const n = 4000
			want := 0.0
			for i := range n {
				z := -5 + 10*(float64(i)+0.5)/n
				at := math.Abs(z)
				dz := at * tan
				var a float64
				switch {
				case z < -5+c.thick, c.open != "top" && z > 5-c.thick:
					a = lobedArea(dz)
				case c.open == "lobe both", c.open == "lobe above" && z > 0:
					a = stepped(at)
				default:
					a = lobedArea(dz) - lobedArea(dz+w)
				}
				want += a * 10 / n
			}
			if math.Abs(r.Volume-want) > 2e-4*want {
				t.Errorf("volume %g, want %g", r.Volume, want)
			}
			if n := sh.Note(); (c.open == "top") != (n == "") || (n != "" && !strings.Contains(n, "short of the inside corner")) {
				t.Errorf("note %q", n)
			}
		})
	}
}

// Drafted both ways steeply, a solid whose round sides are kept meets
// itself on the neutral plane in a crease of twice the draft. Until
// 2026-11-05 the Boolean taking the inside away left that crease open
// from about 8°, and the shell was refused (a disc and a slot at 10°,
// made unsound with no problem on the group before that): the crease's
// curves named the caps the join of the two halves had dropped, by
// handles the next Boolean gave to other patches
// (srf.TestDroppedSurfaceNamedByNone). A disc of radius 6 and a slot of
// half-width 4 and straight length 10, from z = -5 to 5, open at the
// top, walls 1 thick: each half is a frustum, or a slot narrowing by
// z tan α, the inside's narrower by sec α from the bottom moved in by 1.
func TestShellDraftedTwoSidedSteep(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		shape string
		draft float64
	}{{"disc", 10}, {"disc", 15}, {"disc", 20}, {"slot", 10}, {"slot", 15}} {
		t.Run(fmt.Sprintf("%s %g", c.shape, c.draft), func(t *testing.T) {
			t.Parallel()
			if (testing.Short() || fixtures.RaceDetector) && c.draft != 10 {
				t.Skip("only the drafts of 10°, short or under the race detector")
			}
			d := api.New()
			d.Edit.SetChordTolerance(0.005)
			tan, sec := math.Tan(c.draft*math.Pi/180), 1/math.Cos(c.draft*math.Pi/180)
			// half is the volume of one half, from the neutral plane up h,
			// of the section of half-width w0 there.
			var half func(w0, h float64) float64
			if c.shape == "disc" {
				d.Sketch.Circle(api.V(0, 0, 0), 6)
				half = func(w0, h float64) float64 { return frustum(h, w0, w0-h*tan) }
			} else {
				a := d.Sketch.Line(api.V(-5, -4, 0), api.V(5, -4, 0))
				b := d.Sketch.Line(api.V(5, 4, 0), api.V(-5, 4, 0))
				d.Sketch.Arc(api.V(5, 0, 0), a.End().Pos(), b.Start().Pos())
				d.Sketch.Arc(api.V(-5, 0, 0), b.End().Pos(), a.Start().Pos())
				half = func(w0, h float64) float64 {
					w1 := w0 - h*tan
					return 20*h*(w0+w1)/2 + math.Pi*h/3*(w0*w0+w0*w1+w1*w1)
				}
			}
			w := 6.0
			if c.shape == "slot" {
				w = 4
			}
			ex := d.Features.Extrude(10, "")
			ex.SetTwoSided(true)
			ex.AsExtrude().SetDraft(c.draft)
			if p := ex.Problem(); p != "" {
				t.Fatal(p)
			}
			top := ex.Solid().FaceAt(api.V(0, 0, 5))
			sh := d.Features.ShellSolid([]api.SolidFace{top}, 1, "")
			if p := sh.Problem(); p != "" {
				t.Fatal(p)
			}
			want := 2*half(w, 5) - half(w-sec, 5) - half(w-sec, 4)
			if r := sh.Analyze().Check(); !r.Sound() || math.Abs(r.Volume-want) > 2e-4*want {
				t.Fatalf("shell: %s, want volume %g", r.String(), want)
			}
		})
	}
}

// stepBack is lobedCase.setBack for the inside running from foot to
// head along z.
func stepBack(c lobedCase, chord, foot, head float64) func(z float64) float64 {
	tan := math.Tan(c.draft * math.Pi / 180)
	w := c.thick / math.Cos(c.draft*math.Pi/180)
	xc := func(z float64) float64 { d := z * tan; return math.Sqrt((4-d)*(4-d) - d*d) }
	u := func(z float64) float64 { return xc(foot) - xc(z) }
	lin := func(z float64) float64 { return u(head) * (z - foot) / (head - foot) }
	sag := 0.0
	for k := 1; k < 64; k++ {
		z := foot + (head-foot)*float64(k)/64
		sag = max(sag, lin(z)-u(z))
	}
	back := sag + max(0.02*w, 4*chord, 1e-4)
	return func(z float64) float64 { return back - (lin(z) - u(z)) }
}
