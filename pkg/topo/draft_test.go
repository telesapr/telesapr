package topo_test

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// polyLoop is the loop of lines through pts.
func polyLoop(pts ...geom.Vector) []srf.SBezier {
	var l []srf.SBezier
	for i := range pts {
		l = append(l, srf.SBezierFrom2(pts[i], pts[(i+1)%len(pts)]))
	}
	return l
}

// roundedLoop is a w x h rectangle about the origin, at z = 0, its
// corners rounded to radius r, counter-clockwise.
func roundedLoop(w, h, r float64) []srf.SBezier {
	var l []srf.SBezier
	x, y := w/2-r, h/2-r
	corners := []geom.Vector{v(x, y, 0), v(-x, y, 0), v(-x, -y, 0), v(x, -y, 0)}
	for k, c := range corners {
		a0 := float64(k) * math.Pi / 2
		p0 := c.Plus(v(r*math.Cos(a0), r*math.Sin(a0), 0))
		p1 := c.Plus(v(r*math.Cos(a0+math.Pi/2), r*math.Sin(a0+math.Pi/2), 0))
		mid := c.Plus(v(r*math.Sqrt2*math.Cos(a0+math.Pi/4), r*math.Sqrt2*math.Sin(a0+math.Pi/4), 0))
		sb := srf.SBezierFrom3(p0, mid, p1)
		sb.Weight[1] = math.Sqrt2 / 2
		l = append(l, sb)
		next := corners[(k+1)%4]
		a1 := a0 + math.Pi/2
		l = append(l, srf.SBezierFrom2(p1, next.Plus(v(r*math.Cos(a1), r*math.Sin(a1), 0))))
	}
	return l
}

// simpson integrates a quadratic exactly.
func simpson(h float64, area func(z float64) float64) float64 {
	return h / 6 * (area(0) + 4*area(h/2) + area(h))
}

// Extrusions drafted (roadmap 15.4): each cross-section is the profile
// offset by the height times the angle's tangent, so the volume is the
// integral of its area, a quadratic in the height.
func TestDraftedPrism(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.001, 20)
	square := polyLoop(v(0, 0, 0), v(20, 0, 0), v(20, 10, 0), v(0, 10, 0))
	hole := circle(3, 0, 0)
	for i := range hole {
		hole[i] = translatedBy(hole[i], v(10, 5, 0))
	}
	tri := polyLoop(v(0, 0, 0), v(30, 0, 0), v(0, 20, 0))
	for _, c := range []struct {
		name  string
		loops [][]srf.SBezier
		h     float64
		deg   float64
		area  func(x float64) float64 // of the section moved in by x
		round float64                 // the length of the round sides
	}{
		{"block", [][]srf.SBezier{square}, 10, 5, func(x float64) float64 { return (20 - 2*x) * (10 - 2*x) }, 0},
		{"block leaning out", [][]srf.SBezier{square}, 10, -5, func(x float64) float64 { return (20 - 2*x) * (10 - 2*x) }, 0},
		{"triangle", [][]srf.SBezier{tri}, 8, 3, func(x float64) float64 {
			// The inradius is 30 * 20 / (30 + 20 + hyp).
			rin := 600 / (50 + math.Hypot(30, 20))
			s := 1 - x/rin
			return 300 * s * s
		}, 0},
		{"cylinder", [][]srf.SBezier{circle(5, 0, 0)}, 10, 10, func(x float64) float64 { return math.Pi * (5 - x) * (5 - x) }, 2 * math.Pi * 5},
		{"rounded box", [][]srf.SBezier{roundedLoop(20, 10, 3)}, 10, 7, func(x float64) float64 {
			return roundedArea(20-2*x, 10-2*x, 3-x)
		}, 2 * math.Pi * 3},
		{"block with a hole", [][]srf.SBezier{square, hole}, 10, 4, func(x float64) float64 {
			return (20-2*x)*(10-2*x) - math.Pi*(3+x)*(3+x)
		}, 2 * math.Pi * 3},
	} {
		tan := math.Tan(c.deg * math.Pi / 180)
		sh, err := topo.DraftedPrism(c.loops, v(0, 0, c.h), c.deg*math.Pi/180, core.NewRGBA(100, 100, 100, 255), 1<<30, tol)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		want := simpson(c.h, func(z float64) float64 { return c.area(z * tan) })
		if got := sound(t, sh); math.Abs(got-want) > 0.001*c.round*c.h+1e-6*want {
			t.Errorf("%s: volume %g, want %g", c.name, got, want)
		}
		// Downwards, the same solid upside down.
		sh, err = topo.DraftedPrism(c.loops, v(0, 0, -c.h), c.deg*math.Pi/180, core.NewRGBA(100, 100, 100, 255), 0, tol)
		if err != nil {
			t.Errorf("%s downwards: %v", c.name, err)
			continue
		}
		if got := sound(t, sh); math.Abs(got-want) > 0.001*c.round*c.h+1e-6*want {
			t.Errorf("%s downwards: volume %g, want %g", c.name, got, want)
		}
	}
}

func TestDraftRefused(t *testing.T) {
	t.Parallel()
	square := polyLoop(v(0, 0, 0), v(20, 0, 0), v(20, 10, 0), v(0, 10, 0))
	spline := []srf.SBezier{srf.SBezierFrom4(v(0, 0, 0), v(5, 5, 0), v(10, 5, 0), v(15, 0, 0)), srf.SBezierFrom2(v(15, 0, 0), v(0, 0, 0))}
	color := core.NewRGBA(100, 100, 100, 255)
	for name, c := range map[string]struct {
		loops [][]srf.SBezier
		along geom.Vector
		deg   float64
		want  string
	}{
		"sides meet":      {[][]srf.SBezier{square}, v(0, 0, 60), 5, "meet"},
		"cone to a point": {[][]srf.SBezier{circle(5, 0, 0)}, v(0, 0, 60), 10, "takes an arc"},
		"a spline":        {[][]srf.SBezier{spline}, v(0, 0, 5), 5, "neither a line nor an arc"},
		"skewed":          {[][]srf.SBezier{square}, v(1, 0, 5), 5, "square"},
		"flat":            {[][]srf.SBezier{square}, v(0, 0, 5), 90, "flat"},
	} {
		_, err := topo.DraftedPrism(c.loops, c.along, c.deg*math.Pi/180, color, 0, srf.Tolerance{})
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want an error saying %q", name, err, c.want)
		}
		var se *topo.ShellError
		if name == "cone to a point" && !errors.As(err, &se) {
			t.Errorf("%s: %v has no place", name, err)
		}
	}
}

// translatedBy is sb moved by by.
func translatedBy(sb srf.SBezier, by geom.Vector) srf.SBezier {
	for i := 0; i <= sb.Degree; i++ {
		sb.Ctrl[i] = sb.Ctrl[i].Plus(by)
	}
	return sb
}

// Profiles drafted at several angles, both ways, over several heights:
// sound, of the volume the sections give, or refused because the sides
// meet when the offset reaches the profile's narrowest.
func TestDraftSweep(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.001, 20)
	type shape struct {
		loops [][]srf.SBezier
		area  func(x float64) float64
		limit float64 // the offset at which the sides meet
		round float64
	}
	hole := circle(2, 0, 0)
	for i := range hole {
		hole[i] = translatedBy(hole[i], v(10, 5, 0))
	}
	square := polyLoop(v(0, 0, 0), v(20, 0, 0), v(20, 10, 0), v(0, 10, 0))
	hexagon := regularLoop(6, 10)
	shapes := map[string]shape{
		"square": {[][]srf.SBezier{square}, func(x float64) float64 { return (20 - 2*x) * (10 - 2*x) }, 5, 0},
		"hexagon": {[][]srf.SBezier{hexagon}, func(x float64) float64 {
			a := 10 * math.Sqrt(3) / 2
			s := (a - x) / a
			return 6 * 100 * math.Sqrt(3) / 4 * s * s
		}, 10 * math.Sqrt(3) / 2, 0},
		"cylinder": {[][]srf.SBezier{circle(5, 0, 0)}, func(x float64) float64 { return math.Pi * (5 - x) * (5 - x) }, 5, 2 * math.Pi * 5},
		"rounded box": {[][]srf.SBezier{roundedLoop(20, 10, 3)}, func(x float64) float64 {
			return roundedArea(20-2*x, 10-2*x, 3-x)
		}, 3, 2 * math.Pi * 3},
		"block with a hole": {[][]srf.SBezier{square, hole}, func(x float64) float64 {
			return (20-2*x)*(10-2*x) - math.Pi*(2+x)*(2+x)
		}, 1, 2 * math.Pi * 2},
	}
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("shape", "square", "hexagon", "cylinder", "rounded box", "block with a hole"),
			sweep.Floats("degrees", -10, -3, 1, 5, 20),
			sweep.Floats("height", 3, 15, 60),
			sweep.Strings("way", "up", "down"),
		},
		MaxFailRate: 0,
		Sample:      30,
		Parallel:    true,
	}
	grid.Run(func(p sweep.Point) error {
		s := shapes[p.String("shape")]
		h, deg := p.Float("height"), p.Float("degrees")
		x := h * math.Tan(deg*math.Pi/180)
		along := v(0, 0, h)
		if p.String("way") == "down" {
			along = v(0, 0, -h)
		}
		sh, err := topo.DraftedPrism(s.loops, along, deg*math.Pi/180, core.NewRGBA(100, 100, 100, 255), 0, tol)
		meets := x >= s.limit-1e-9
		if p.String("shape") == "block with a hole" {
			// Between the hole and the long side, 3 wide at the
			// sketch; leaning out, the outside grows as the hole
			// shrinks, and the hole closes at its radius.
			meets = x >= 1.5-1e-9 || x <= -2+1e-9
		}
		if meets {
			if err == nil {
				return fmt.Errorf("an offset of %g, past %g, is accepted", x, s.limit)
			}
			return nil
		}
		if err != nil {
			return err
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		r := check.Solid(sh, &m)
		want := simpson(h, func(z float64) float64 { return s.area(z / h * x) })
		if !r.Sound() {
			return fmt.Errorf("unsound: %s", r.String())
		}
		// The chords cut inside the round sides, which grow as they
		// lean out; a curve takes at most 20 chords (srf's
		// maxSegments), so on a wide cone they cut deeper than the
		// tolerance asks: the error stays at 3.4 mm³ on the cylinder
		// leaning out 3° over 60 from a tolerance of 0.001 down.
		round := s.round
		if round > 0 {
			round += 2 * math.Pi * math.Abs(x)
		}
		if math.Abs(r.Volume-want) > 0.002*round*h+1e-6*want {
			return fmt.Errorf("volume %g, want %g", r.Volume, want)
		}
		return nil
	}).Require(t)
}

// regularLoop is the regular polygon of n sides about the origin, its
// corners r from it.
func regularLoop(n int, r float64) []srf.SBezier {
	var pts []geom.Vector
	for i := range n {
		a := 2 * math.Pi * float64(i) / float64(n)
		pts = append(pts, v(r*math.Cos(a), r*math.Sin(a), 0))
	}
	return polyLoop(pts...)
}
