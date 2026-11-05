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

// lobedBlock draws a block 20 by 10 whose top edge, y = 0, carries a
// round lobe of radius 4 about the origin, and extrudes it 10 drafted
// by draft degrees: where the lobe
// meets the top edge, at x = ±4, the lobe's cone meets the top's plane
// at an inside corner.
func lobedBlock(t *testing.T, draft float64) (*api.Doc, api.Group) {
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
	ex.AsExtrude().SetDraft(draft)
	if p := ex.Problem(); p != "" {
		t.Fatal(p)
	}
	return d, ex
}

// lobedCase is a shell of the lobed block: its draft in degrees, its
// walls' thickness, and whether the top edge either side of
// the lobe is taken away with it (the two pieces lie on one plane, so
// they are one face).
type lobedCase struct {
	draft, thick float64
	openTop      bool
}

// span is how far the block reaches along z, from lo to hi, and where
// its inside starts and ends, foot to head.
func (c lobedCase) span() (lo, hi, foot, head float64) { return 0, 10, c.thick, 10 - c.thick }

// setBack is how much further from the lobe than the corner the walls
// under the top edge end at distance z from the sketch, as the shell
// steps them (pkg/topo shelldraft_corner.go): the step follows the
// chord of the corner's path from the inside's foot to its head, set
// back by the path's sag from it and a margin.
func (c lobedCase) setBack(chord float64) func(z float64) float64 {
	tan := math.Tan(c.draft * math.Pi / 180)
	w := c.thick / math.Cos(c.draft*math.Pi/180)
	xc := func(z float64) float64 { d := z * tan; return math.Sqrt((4-d)*(4-d) - d*d) }
	_, _, lo, hi := c.span()
	// u, how far the corner has moved toward the lobe since the foot.
	u := func(z float64) float64 { return xc(lo) - xc(z) }
	lin := func(z float64) float64 { return u(hi) * (z - lo) / (hi - lo) }
	sag := 0.0
	for k := 1; k < 64; k++ {
		z := lo + (hi-lo)*float64(k)/64
		sag = max(sag, lin(z)-u(z))
	}
	back := sag + max(0.02*w, 4*chord, 1e-4)
	return func(z float64) float64 { return back - (lin(z) - u(z)) }
}

// volume is what the shell keeps: every section between the ends moved
// in keeps the block's border of width thick/cos α, but not along the
// top edge between the corners, where the lobe opens it, nor along the
// rest of the top edge where that is taken away too (where the corners
// don't step); the
// corners are where the top edge's plane meets the lobe's cone, and
// cut(z) is how much further from the lobe the walls under the top edge
// end than the corner.
func (c lobedCase) volume(cut func(z float64) float64) float64 {
	tan := math.Tan(c.draft * math.Pi / 180)
	w := c.thick / math.Cos(c.draft*math.Pi/180)
	lo, hi, _, head := c.span()
	const n = 4000
	v := 0.0
	for i := range n {
		z := lo + (hi-lo)*(float64(i)+0.5)/n
		at := math.Abs(z)
		d := at * tan
		wr, hr := 20-2*d, 10-2*d
		var a float64
		if z < c.thick || at > head {
			rho := 4 - d
			a = wr*hr + rho*rho*math.Acos(-d/rho) + d*math.Sqrt(rho*rho-d*d)
		} else {
			xc := math.Sqrt((4-d)*(4-d) - d*d)
			a = wr*hr - (wr-2*w)*(hr-2*w) - 2*(xc+cut(at))*w
			if c.openTop {
				a = wr*hr - (wr-2*w)*(hr-2*w) - 2*(10-d-w)*w
			}
		}
		v += a * (hi - lo) / n
	}
	return v
}

// A drafted block with a round lobe on its top edge, hollowed open at
// the lobe's cone: the walls under the top edge either side of the lobe
// end where the lobe meets them, at an inside corner that moves along a
// conic going up, so they stop a little short of it. With the top edge
// either side of the lobe taken away too, the lobe runs out beside it.
func TestShellDraftedInsideCorner(t *testing.T) {
	t.Parallel()
	var cases []lobedCase
	for _, draft := range []float64{0.5, 2, 5, 10} {
		for _, thick := range []float64{0.5, 1, 2} {
			cases = append(cases, lobedCase{draft: draft, thick: thick})
		}
		cases = append(cases, lobedCase{draft: draft, thick: 1, openTop: true})
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {
			t.Parallel()
			if (testing.Short() || fixtures.RaceDetector) && (c.draft != 5 || c.thick != 1) {
				t.Skip("only the cases of draft 5 and thickness 1, short or under the race detector")
			}
			d, ex := lobedBlock(t, c.draft)
			tan := math.Tan(c.draft * math.Pi / 180)
			// The lobe's cone, a face each side of the y axis, at
			// distance 2 from the sketch.
			r2 := 4 - 2*tan
			var open []api.SolidFace
			for _, a := range []float64{math.Pi / 4, 3 * math.Pi / 4} {
				open = append(open, ex.Solid().FaceAt(api.V(r2*math.Cos(a), r2*math.Sin(a), 2)))
			}
			if c.openTop {
				open = append(open, ex.Solid().FaceAt(api.V(7, -2*tan, 2)))
			}
			sh := d.Features.ShellSolid(open, c.thick, "")
			if p := sh.Problem(); p != "" {
				t.Fatal(p)
			}
			r := sh.Analyze().Check()
			if !r.Sound() {
				t.Fatalf("shell: %s", r.String())
			}
			if n := sh.Note(); c.openTop != (n == "") || (n != "" && !strings.Contains(n, "short of the inside corner")) {
				t.Errorf("note %q", n)
			}
			var chord float64
			d.With(func(sk *sketch.Sketch) { chord = sk.Tol().ChordTolMm() })
			want := c.volume(c.setBack(chord))
			if math.Abs(r.Volume-want) > 2e-4*want {
				t.Errorf("volume %g, want %g (the walls ending at the corner: %g)", r.Volume, want,
					c.volume(func(float64) float64 { return 0 }))
			}
		})
	}
}

// A drafted disc with a lobe on its rim, the lobe's cone meeting the
// rim's at inside corners, is still refused open at the lobe: there is
// no plane to step along beside a curved side kept.
func TestShellDraftedInsideCornerBesideCurve(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Edit.SetChordTolerance(0.005)
	// A disc of radius 10 about the origin and a lobe of radius 3 about
	// (10, 0), meeting at x = 9.55.
	cx := (100 - 9 + 100) / 20.0
	cy := math.Sqrt(100 - cx*cx)
	d.Sketch.Arc(api.V(0, 0, 0), api.V(cx, cy, 0), api.V(cx, -cy, 0))
	d.Sketch.Arc(api.V(10, 0, 0), api.V(cx, -cy, 0), api.V(cx, cy, 0))
	ex := d.Features.Extrude(10, "")
	ex.AsExtrude().SetDraft(3)
	if p := ex.Problem(); p != "" {
		t.Fatal(p)
	}
	// The lobe's cone, in as many faces as it was put in as.
	r2 := 3 - 2*math.Tan(3*math.Pi/180)
	var lobe []api.SolidFace
	seen := map[api.SolidFace]bool{}
	for k := -4; k <= 4; k++ {
		a := float64(k) * math.Pi / 8
		f := ex.Solid().FaceAt(api.V(10+r2*math.Cos(a), r2*math.Sin(a), 2))
		if !seen[f] {
			seen[f] = true
			lobe = append(lobe, f)
		}
	}
	refusedWith(t, "inside corner", func() { d.Features.ShellSolid(lobe, 1, "") })
}
