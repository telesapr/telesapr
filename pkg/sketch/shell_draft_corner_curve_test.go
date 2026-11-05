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

// lobedDisc draws a disc of radius 10 about the origin with a lobe of
// radius 3 about (10, 0) on its rim, meeting it at inside corners at
// x = 9.55, and extrudes it 10 drafted by draft degrees.
func lobedDisc(t *testing.T, draft float64) (*api.Doc, api.Group) {
	t.Helper()
	d := api.New()
	d.Edit.SetChordTolerance(0.005)
	cx := (100 - 9 + 100) / 20.0
	cy := math.Sqrt(100 - cx*cx)
	d.Sketch.Arc(api.V(0, 0, 0), api.V(cx, cy, 0), api.V(cx, -cy, 0))
	d.Sketch.Arc(api.V(10, 0, 0), api.V(cx, -cy, 0), api.V(cx, cy, 0))
	ex := d.Features.Extrude(10, "")
	ex.AsExtrude().SetDraft(draft)
	if p := ex.Problem(); p != "" {
		t.Fatal(p)
	}
	return d, ex
}

// lensArea is the area two circles of radii a and b, centres d apart,
// share.
func lensArea(a, b, d float64) float64 {
	ca := (d*d + a*a - b*b) / (2 * d * a)
	cb := (d*d + b*b - a*a) / (2 * d * b)
	return a*a*math.Acos(ca) + b*b*math.Acos(cb) -
		0.5*math.Sqrt((-d+a+b)*(d+a-b)*(d-a+b)*(d+a+b))
}

// discCorner is the lobed disc's upper inside corner at distance z from
// the sketch, drafted by tan: where the rim of radius 10 - z tan meets
// the lobe's circle of radius 3 - z tan about (10, 0); as an angle about
// the origin.
func discCorner(z, tan float64) float64 {
	R, r := 10-z*tan, 3-z*tan
	return math.Acos((100 + R*R - r*r) / (20 * R))
}

// discStep is where the shell of the lobed disc opened at its lobe
// steps through the rim's wall, as the shell does it
// (pkg/topo shelldraft_corner.go): the step's foot, an angle about the
// origin at the inside's foot from, set back from the corner along the
// rim by back; how fast the step slides toward the corner along the
// rim's tangent at the foot, per unit of height; and the least set-back,
// the sag of the corner's path from its chord to the head to and a
// margin. The shell sets the step further back where the way outside the
// rim, sliding with the step along a tangent, would leave too little
// room in the wedge between the rim and the lobe, and says how far back
// in its note.
func discStep(tan, w, chord, from, to, back float64) (foot, slope, least float64) {
	R0 := 10 - from*tan
	c0 := discCorner(from, tan)
	at := func(z float64) [2]float64 {
		th, R := discCorner(z, tan), 10-z*tan
		return [2]float64{R * math.Cos(th), R * math.Sin(th)}
	}
	p0 := at(from)
	// Toward the corner along the rim at angle th: the way of falling
	// angle.
	toward := func(th float64) [2]float64 { return [2]float64{math.Sin(th), -math.Cos(th)} }
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
		e = toward(c0 + least/R0)
	}
	return c0 + back/R0, slope, least
}

// A drafted disc with a round lobe on its rim, hollowed open at the
// lobe: the rim's wall either side of the lobe ends a little short of
// the inside corner, on a plane crossing it square at the inside's foot
// and sliding along the rim's tangent there going up, as a block's flat
// side does beside a lobe (TestShellDraftedInsideCorner). It was
// refused before 2026-11-05. The volume is checked against the sections'
// integral: the rim's ring of width thick/cos α, less the arc of it on
// the corner's side of each step, between the ends moved in.
func TestShellDraftedInsideCornerBesideCurve(t *testing.T) {
	t.Parallel()
	type tcase struct{ draft, thick float64 }
	var cases []tcase
	for _, draft := range []float64{0.5, 3, 6, 8} {
		for _, thick := range []float64{0.5, 1, 2} {
			cases = append(cases, tcase{draft, thick})
		}
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {
			t.Parallel()
			if (testing.Short() || fixtures.RaceDetector) && (c.draft != 3 || c.thick != 1) {
				t.Skip("only the case of draft 3 and thickness 1, short or under the race detector")
			}
			d, ex := lobedDisc(t, c.draft)
			tan := math.Tan(c.draft * math.Pi / 180)
			w := c.thick / math.Cos(c.draft*math.Pi/180)
			// The lobe's cone, in as many faces as it was put in as.
			r2 := 3 - 2*tan
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
			sh := d.Features.ShellSolid(lobe, c.thick, "")
			if p := sh.Problem(); p != "" {
				t.Fatal(p)
			}
			r := sh.Analyze().Check()
			if !r.Sound() {
				t.Fatalf("shell: %s", r.String())
			}
			if n := sh.Note(); !strings.Contains(n, "short of the inside corner") {
				t.Errorf("note %q", n)
			}
			var chord float64
			d.With(func(sk *sketch.Sketch) { chord = sk.Tol().ChordTolMm() })
			from, to := c.thick, 10-c.thick
			var back float64
			n := sh.Note()
			if _, err := fmt.Sscanf(n[strings.Index(n, "up to ")+len("up to "):], "%g", &back); err != nil {
				t.Fatalf("note %q: %v", n, err)
			}
			foot, slope, least := discStep(tan, w, chord, from, to, back)
			if back < least*(1-1e-3)-5e-4 {
				t.Errorf("set back %g, less than the least %g", back, least)
			}
			const nz, nr = 2000, 64
			want, reach := 0.0, 0.0
			for i := range nz {
				z := 10 * (float64(i) + 0.5) / nz
				R, rl := 10-z*tan, 3-z*tan
				var a, ar float64
				if z < from || z > to {
					a = math.Pi*R*R + math.Pi*rl*rl - lensArea(R, rl, 10)
					ar = a
				} else {
					// Walls reaching the corner, for the log: the ring cut
					// on the radial through it.
					ar = (math.Pi - discCorner(z, tan)) * (R*R - (R-w)*(R-w))
					// The ring between R - w and R, from where the step crosses
					// each circle round the far side: the step is the line
					// along the radial at the foot, slid along the tangent by
					// slope (z - from) toward the corner.
					pe := slope * (z - from)
					for j := range nr {
						rr := R - w + w*(float64(j)+0.5)/nr
						cut := foot - math.Pi/2 + math.Acos(pe/rr)
						a += 2 * (math.Pi - cut) * rr * w / nr
					}
				}
				want += a * 10 / nz
				reach += ar * 10 / nz
			}
			t.Logf("set back up to %g (least %g), volume %g, %.3g%% short of walls reaching the corner",
				back, least, r.Volume, 100*(reach-r.Volume)/reach)
			if math.Abs(r.Volume-want) > 2e-4*want {
				t.Errorf("volume %g, want %g", r.Volume, want)
			}
		})
	}
}
