package sketch_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// crescent is a disc of radius 10 about the origin less a hollow, a
// disc of radius rc about (14 - rc, 0), whose deepest point, at
// x = 14 - 2rc, carries a lobe of radius rl reaching into the hollow:
// the lobe meets the hollow's side at two inside corners.
type crescent struct{ rc, rl float64 }

// centres are the hollow's centre and the lobe's, on the x axis.
func (c crescent) centres() (cc, lx float64) { return 14 - c.rc, 14 - 2*c.rc }

// meet is where, above the x axis, the circles about (a, 0) of radius
// ra and about (b, 0) of radius rb meet.
func meet(a, ra, b, rb float64) (x, y float64) {
	x = (ra*ra - rb*rb + b*b - a*a) / (2 * (b - a))
	return x, math.Sqrt(ra*ra - (x-a)*(x-a))
}

// draw draws the crescent and extrudes it 10, drafted by draft degrees, at
// the chord tolerance the drawing has (a preference, not saved in its file).
func (c crescent) draw(d *api.Doc, draft float64) api.Group {
	cc, lx := c.centres()
	xr, yr := meet(0, 10, cc, c.rc)
	xl, yl := meet(cc, c.rc, lx, c.rl)
	d.Sketch.Arc(api.V(0, 0, 0), api.V(xr, yr, 0), api.V(xr, -yr, 0))
	d.Sketch.Arc(api.V(cc, 0, 0), api.V(xl, -yl, 0), api.V(xr, -yr, 0))
	d.Sketch.Arc(api.V(lx, 0, 0), api.V(xl, -yl, 0), api.V(xl, yl, 0))
	d.Sketch.Arc(api.V(cc, 0, 0), api.V(xr, yr, 0), api.V(xl, yl, 0))
	ex := d.Features.Extrude(10, "")
	ex.AsExtrude().SetDraft(draft)
	return ex
}

// lobe is the lobe's cone of ex, drafted by tan, in as many faces as it
// was put in as.
func (c crescent) lobe(ex api.Group, tan float64) []api.SolidFace {
	_, lx := c.centres()
	r2 := c.rl - 2*tan
	var faces []api.SolidFace
	seen := map[api.SolidFace]bool{}
	for k := -3; k <= 3; k++ {
		a := float64(k) * math.Pi / 10
		f := ex.Solid().FaceAt(api.V(lx+r2*math.Cos(a), r2*math.Sin(a), 2))
		if !seen[f] {
			seen[f] = true
			faces = append(faces, f)
		}
	}
	return faces
}

// step is where the shell opened at the lobe steps through the hollow's
// wall, as the shell does it (pkg/topo shelldraft_corner.go): the step's
// foot, an angle about the hollow's centre at the inside's foot from,
// set back from the corner along the hollow's side by back; how fast the
// step slides toward the corner along the side's tangent at the foot,
// per unit of height; and the least set-back, the sag of the corner's
// path from its chord to the head to and a margin.
func (c crescent) step(tan, w, chord, from, to, back float64) (foot, slope, least float64) {
	cc, lx := c.centres()
	at := func(z float64) [2]float64 {
		x, y := meet(cc, c.rc+z*tan, lx, c.rl-z*tan)
		return [2]float64{x - cc, y}
	}
	angle := func(p [2]float64) float64 { return math.Atan2(p[1], p[0]) }
	R0 := c.rc + from*tan
	p0 := at(from)
	c0 := angle(p0)
	// Toward the corner along the side at angle th: the way of rising
	// angle about the hollow's centre.
	toward := func(th float64) [2]float64 { return [2]float64{-math.Sin(th), math.Cos(th)} }
	e := toward(c0)
	margin := max(0.02*w, 4*chord, 1e-4)
	for range 2 {
		u := func(z float64) float64 {
			p := at(z)
			return (p[0]-p0[0])*e[0] + (p[1]-p0[1])*e[1]
		}
		h := to - from
		slope = u(to) / h
		sag := 0.0
		for k := 1; k < 64; k++ {
			z := from + h*float64(k)/64
			sag = max(sag, slope*(z-from)-u(z))
		}
		least = sag + margin
		e = toward(c0 - least/R0)
	}
	return c0 - back/R0, slope, least
}

// arcArea is the signed area ∫(x dy - y dx)/2 along the circle about
// (cx, 0) of radius r from angle a to b.
func arcArea(cx, r, a, b float64) float64 {
	return 0.5 * (r*r*(b-a) + r*cx*(math.Sin(b)-math.Sin(a)))
}

// wallArea is the area of the crescent's section at height z, drafted
// by tan, between the ends moved in: its border of width w, open at the
// lobe, the walls under the hollow's side ending at the steps, which
// cross the side square to it at angle foot about the hollow's centre
// and have slid pe toward the corner along the tangent there.
func (c crescent) wallArea(tan, w, z, foot, pe float64) float64 {
	cc, _ := c.centres()
	R, rh := 10-z*tan, c.rc+z*tan
	// The upper half, by Green's theorem round its outline: the rim from
	// the x axis to where it meets the hollow, the hollow's side to the
	// step, the step, the inner side back, the inner rim to the x axis.
	xr, yr := meet(0, R, cc, rh)
	xi, yi := meet(0, R-w, cc, rh+w)
	thR := math.Atan2(yr, xr)
	thHr := math.Atan2(yr, xr-cc)
	thRi := math.Atan2(yi, xi)
	thHi := math.Atan2(yi, xi-cc)
	sOut := foot + math.Asin(pe/rh)
	sIn := foot + math.Asin(pe/(rh+w))
	a := 0.0
	a += arcArea(0, R, math.Pi, thR) // the rim, clockwise from the x axis
	a += arcArea(cc, rh, thHr, sOut)
	pOut := [2]float64{cc + rh*math.Cos(sOut), rh * math.Sin(sOut)}
	pIn := [2]float64{cc + (rh+w)*math.Cos(sIn), (rh + w) * math.Sin(sIn)}
	a += 0.5 * (pOut[0]*pIn[1] - pOut[1]*pIn[0])
	a += arcArea(cc, rh+w, sIn, thHi)
	a += arcArea(0, R-w, thRi, math.Pi)
	return 2 * math.Abs(a)
}

// fullArea is the crescent's section at height z, drafted by tan.
func (c crescent) fullArea(tan, z float64) float64 {
	R, rh, rl := 10-z*tan, c.rc+z*tan, c.rl-z*tan
	cc, _ := c.centres()
	return math.Pi*R*R - lensArea(R, rh, cc) + lensArea(rl, rh, c.rc)
}

// A drafted crescent, a disc whose hollow side carries a round lobe,
// hollowed open at the lobe: the hollow's wall either side of the lobe
// ends a little short of the inside corner, as beside a convex side
// (TestShellDraftedInsideCornerBesideCurve), and the way outside it
// back to the lobe run out follows the hollow's circle moved out, not
// its tangent, which runs into the material by x²/2r. It was refused
// before 2026-11-05. The volume is checked against the sections'
// integral, the mesh fine enough to come within 1e-4 of it (4e-5 found). Of the 27,
// one is refused for its walls, not the corner: the hollow of 9 drafted
// 5° leaves the crescent's middle too thin at the head for walls of 2.
// Each case takes some 16 s of processor at that mesh, so the ordinary
// run takes a Latin square of 9 (every hollow, draft and thickness
// three times, every pair of two of them once) and the refused one;
// make test-sweeps takes all 27.
func TestShellDraftedInsideCornerBesideHollow(t *testing.T) {
	t.Parallel()
	type tcase struct{ rc, draft, thick float64 }
	var cases []tcase
	for i, rc := range []float64{4, 6, 9} {
		for j, draft := range []float64{0.5, 3, 5} {
			for k, thick := range []float64{0.5, 1, 2} {
				// The square's thickness for hollow i and draft j keeps
				// hollow 6, draft 3, thickness 1, the short run's case.
				square := k == (i+j+2)%3
				refused := i == 2 && j == 2 && k == 2
				if sweep.Full() || square || refused {
					cases = append(cases, tcase{rc, draft, thick})
				}
			}
		}
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {
			t.Parallel()
			if (testing.Short() || fixtures.RaceDetector) && (c.rc != 6 || c.draft != 3 || c.thick != 1) {
				t.Skip("only the case of hollow 6, draft 3 and thickness 1, short or under the race detector")
			}
			cr := crescent{rc: c.rc, rl: 2}
			d := api.New()
			ex := cr.draw(d, c.draft)
			// Meshed finely: at the usual 20 pieces a curve the mesh's
			// chords lose 1e-4 of the volume.
			d.Edit.SetChordTolerance(0.001)
			d.Edit.SetMaxSegments(400)
			if p := ex.Problem(); p != "" {
				t.Fatal(p)
			}
			tan := math.Tan(c.draft * math.Pi / 180)
			w := c.thick / math.Cos(c.draft*math.Pi/180)
			refused := c.rc == 9 && c.draft == 5 && c.thick == 2
			defer func() {
				e := recover()
				if e == nil || (refused && strings.Contains(fmt.Sprint(e), "takes an arc of the profile away")) {
					return
				}
				t.Fatal(e)
			}()
			sh := d.Features.ShellSolid(cr.lobe(ex, tan), c.thick, "")
			if p := sh.Problem(); p != "" {
				t.Fatal(p)
			}
			if refused {
				t.Fatal("shelled, where the walls meet at the head")
			}
			r := sh.Analyze().Check()
			if !r.Sound() {
				t.Fatalf("shell: %s", r.String())
			}
			n := sh.Note()
			if !strings.Contains(n, "short of the inside corner") {
				t.Fatalf("note %q", n)
			}
			var chord float64
			d.With(func(sk *sketch.Sketch) { chord = sk.Tol().ChordTolMm() })
			from, to := c.thick, 10-c.thick
			var back float64
			if _, err := fmt.Sscanf(n[strings.Index(n, "up to ")+len("up to "):], "%g", &back); err != nil {
				t.Fatalf("note %q: %v", n, err)
			}
			foot, slope, least := cr.step(tan, w, chord, from, to, back)
			if back < least*(1-1e-3)-5e-4 {
				t.Errorf("set back %g, less than the least %g", back, least)
			}
			const nz = 2000
			want := 0.0
			for i := range nz {
				z := 10 * (float64(i) + 0.5) / nz
				a := cr.fullArea(tan, z)
				if z > from && z < to {
					a = cr.wallArea(tan, w, z, foot, slope*(z-from))
				}
				want += a * 10 / nz
			}
			t.Logf("set back up to %g (least %g), volume %g, want %g", back, least, r.Volume, want)
			if math.Abs(r.Volume-want) > 1e-4*want {
				t.Errorf("volume %g, want %g", r.Volume, want)
			}
		})
	}
}
