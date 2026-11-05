package topo_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// profilesAt are the loops of in drawn in the planes through the z axis
// at the angles at, on the curve round the axis that place gives (x
// away from the axis, y up, about the point place(a), facing along the
// curve's tangent there).
func profilesAt(at []float64, place func(a float64) (geom.Vector, geom.Vector), in func(i int) []srf.SBezier) [][]srf.SBezier {
	var out [][]srf.SBezier
	for i, a := range at {
		p, tangent := place(a)
		er := tangent.Cross(v(0, 0, 1)).WithMagnitude(1)
		to := func(q geom.Vector) geom.Vector {
			return p.Plus(er.ScaledBy(q.X)).Plus(v(0, 0, q.Y))
		}
		var l []srf.SBezier
		for _, sb := range in(i) {
			for k := 0; k <= sb.Degree; k++ {
				sb.Ctrl[k] = to(sb.Ctrl[k])
			}
			l = append(l, sb)
		}
		out = append(out, l)
	}
	return out
}

// onCircle and onEllipse place profiles round the z axis.
func onCircle(r float64) func(a float64) (geom.Vector, geom.Vector) {
	return func(a float64) (geom.Vector, geom.Vector) {
		return v(r*math.Cos(a), r*math.Sin(a), 0), v(-math.Sin(a), math.Cos(a), 0)
	}
}

func onEllipse(ra, rb float64) func(a float64) (geom.Vector, geom.Vector) {
	return func(a float64) (geom.Vector, geom.Vector) {
		return v(ra*math.Cos(a), rb*math.Sin(a), 0), v(-ra*math.Sin(a), rb*math.Cos(a), 0).WithMagnitude(1)
	}
}

func degs(ds ...float64) []float64 {
	out := make([]float64, len(ds))
	for i, d := range ds {
		out[i] = d * math.Pi / 180
	}
	return out
}

// Closed lofts whose profiles lie more than a third of a turn apart
// round the ring: three circles on a circle, with gaps of 150 degrees,
// of half a turn, or one of 300 (three bunched on one side), drawn
// either way round or facing either way, make the torus; on an ellipse
// they make a sound ring.
func TestClosedLoftWide(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.005, 100)
	col := core.NewRGBA(100, 100, 100, 255)
	const big, r = 20.0, 4.0
	torus := 2 * math.Pi * math.Pi * big * r * r
	round := func(int) []srf.SBezier { return circle(r, 0, 0) }
	flip := func(loops [][]srf.SBezier, which ...int) [][]srf.SBezier {
		for _, i := range which {
			l := loops[i]
			out := make([]srf.SBezier, len(l))
			for j := range l {
				sb := l[len(l)-1-j]
				sb.Reverse()
				out[j] = sb
			}
			loops[i] = out
		}
		return loops
	}
	for _, c := range []struct {
		name  string
		loops [][]srf.SBezier
		want  float64
	}{
		{"150 90 120", profilesAt(degs(0, 150, 240), onCircle(big), round), torus},
		{"150 60 150", profilesAt(degs(0, 150, 210), onCircle(big), round), torus},
		{"half turn", profilesAt(degs(0, 90, 180), onCircle(big), round), torus},
		{"one long gap", profilesAt(degs(0, 30, 60), onCircle(big), round), torus},
		{"one long gap of four", profilesAt(degs(10, 40, 70, 100), onCircle(big), round), torus},
		{"clockwise", profilesAt(degs(0, -30, -60), onCircle(big), round), torus},
		{"facing both ways", flip(profilesAt(degs(0, 30, 60), onCircle(big), round), 1), torus},
		{"squares", profilesAt(degs(0, 150, 210), onCircle(big), func(int) []srf.SBezier { return polygonAt(4, r, 0, 0, 0, math.Pi/4) }), 2 * r * r * 2 * math.Pi * big},
		{"ellipse", profilesAt(degs(0, 150, 210), onEllipse(25, 18), round), 0},
		{"ellipse, long gap", profilesAt(degs(-30, 0, 30), onEllipse(25, 18), round), 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			sh, err := topo.MakeClosedLoft(c.loops, col, srf.OriginFor(9, 0), tol)
			if err != nil {
				t.Fatal(err)
			}
			var m srf.SMesh
			sh.TriangulateInto(&m)
			rep := check.Solid(sh, &m)
			if !rep.Sound() {
				t.Fatalf("%s", rep.String())
			}
			if c.want != 0 && math.Abs(rep.Volume-c.want) > 0.002*c.want {
				t.Errorf("volume %.4f, want %.4f", rep.Volume, c.want)
			}
		})
	}
}

// The ring with one long gap held square to a profile, and guided over
// its circles' tops by a guide rising between them: sound, the held
// ring the torus.
func TestClosedLoftWideHeldGuided(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.005, 100)
	col := core.NewRGBA(100, 100, 100, 255)
	const big, r = 25.0, 4.0
	loops := func() [][]srf.SBezier {
		return profilesAt(degs(0, 30, 60), onCircle(big), func(int) []srf.SBezier { return circle(r, 0, 0) })
	}
	guide := ringGuide(big, r, func(float64) float64 { return math.Pi / 2 }, func(a float64) float64 {
		// Over the profiles' tops, rising in the long gap.
		if a <= math.Pi/3 {
			return 0
		}
		return 1.5 * math.Pow(math.Sin(math.Pi*(a-math.Pi/3)/(5*math.Pi/3)), 2)
	})
	for _, c := range []struct {
		name string
		make func() (*srf.SShell, error)
		want float64
	}{
		{"held", func() (*srf.SShell, error) { return topo.MakeClosedLoftHeld(loops(), []float64{0, 1}, col, 9, tol) }, 2 * math.Pi * math.Pi * big * r * r},
		{"guided", func() (*srf.SShell, error) {
			return topo.MakeClosedGuidedLoft(loops(), []topo.LoftGuide{guide}, col, 9, tol)
		}, 0},
		{"guided held", func() (*srf.SShell, error) {
			return topo.MakeClosedGuidedLoftHeld(loops(), []float64{1}, []topo.LoftGuide{guide}, col, 9, tol)
		}, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			sh, err := c.make()
			if err != nil {
				t.Fatal(err)
			}
			var m srf.SMesh
			sh.TriangulateInto(&m)
			rep := check.Solid(sh, &m)
			if !rep.Sound() {
				t.Fatalf("%s", rep.String())
			}
			if c.want != 0 && math.Abs(rep.Volume-c.want) > 0.002*c.want {
				t.Errorf("volume %.4f, want %.4f", rep.Volume, c.want)
			}
		})
	}
}

// TestClosedLoftWideSweep closes rings of three to five profiles at
// angles round the ring with gaps of a sixth to most of a turn, on a
// circle or an ellipse, flat or rising and falling and leaning with it,
// of circles, squares and blobs, and requires sound solids. None of
// the whole grid, 126, failed when written.
func TestClosedLoftWideSweep(t *testing.T) {
	if testing.Short() {
		t.Skip("a sweep of closed lofts with profiles far apart round the ring")
	}
	t.Parallel()
	tol := srf.Tol(0.005, 100)
	col := core.NewRGBA(1, 1, 1, 255)
	grid := sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Strings("gaps", "150 90 120", "60 60 240", "20 20 320", "100 50 30 180", "40 170 40 110", "30 30 30 30 240", "72 144 72 36 36"),
			sweep.Strings("path", "circle", "ellipse", "wavy"),
			sweep.Strings("profile", "circle", "square", "blob"),
			sweep.Floats("start", 0, 100),
		},
	}
	grid.Run(func(p sweep.Point) error {
		if fixtures.RaceDetector && p.String("path") != "circle" {
			// Under the race detector, on top of the grid's fifth, the
			// circle only: the ellipse and the wavy path loft on the
			// same goroutines.
			return nil
		}
		var at []float64
		a := p.Float("start")
		gaps := splitFloats(p.String("gaps"))
		for _, g := range gaps {
			at = append(at, a*math.Pi/180)
			a += g
		}
		base := onCircle(25)
		if p.String("path") != "circle" {
			base = onEllipse(25, 18)
		}
		place := base
		if p.String("path") == "wavy" {
			place = func(a float64) (geom.Vector, geom.Vector) {
				q, tg := base(a)
				q.Z = 4 * math.Sin(2*a)
				return q, tg.Plus(v(0, 0, 0.3*math.Cos(2*a))).WithMagnitude(1)
			}
		}
		loops := profilesAt(at, place, func(int) []srf.SBezier {
			switch p.String("profile") {
			case "square":
				return polygonAt(4, 4, 0, 0, 0, math.Pi/4)
			case "blob":
				return blobAt(4, 2.4, 2, 0, 0, 0)
			}
			return circle(4, 0, 0)
		})
		sh, err := topo.MakeClosedLoft(loops, col, 9, tol)
		if err != nil {
			return err
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		if r := check.Solid(sh, &m); !r.Sound() {
			return fmt.Errorf("unsound: %s", r.String())
		}
		return nil
	}).Require(t)
}

// splitFloats are the numbers of s, separated by spaces.
func splitFloats(s string) []float64 {
	var out []float64
	for f := range strings.FieldsSeq(s) {
		var x float64
		if _, err := fmt.Sscan(f, &x); err == nil {
			out = append(out, x)
		}
	}
	return out
}
