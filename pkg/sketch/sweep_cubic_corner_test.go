package sketch_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// cubicCornerPath draws, in the ZX workplane, a line up the z axis of
// length lead and a cubic leaving its top at a sharp corner of corner
// degrees, its legs bent by bend degrees from its chord of length span;
// with cubicFirst, a cubic up from the origin bent by bend and a line of
// length lead leaving its end at the corner. With bend 0 the cubic is
// straight.
func cubicCornerPath(d *api.Doc, cubicFirst bool, corner, bend, lead, span float64) []api.Entity {
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	if cubicFirst {
		y := api.V(0, 1, 0)
		b := bend * math.Pi / 180
		up := api.V(0, 0, 1)
		far := up.RotatedAroundAxis(y, b/2).ScaledBy(span)
		out := up.RotatedAroundAxis(y, b)
		c2 := far.Minus(out.ScaledBy(span / 3))
		end := far.Plus(out.RotatedAroundAxis(y, corner*math.Pi/180).ScaledBy(lead))
		return []api.Entity{
			d.Sketch.Cubic(api.V(0, 0, 0), up.ScaledBy(span/3), c2, far),
			d.Sketch.Line(far, end),
		}
	}
	p := cubicCornerPoints(corner, bend, lead, span)
	return []api.Entity{d.Sketch.Line(p[0], p[1]), d.Sketch.Cubic(p[1], p[2], p[3], p[4])}
}

// cubicCornerPoints are the line's foot [0] and top [1], which the
// cubic starts from, the cubic's inner points [2] and [3], and its end
// [4].
func cubicCornerPoints(corner, bend, lead, span float64) []api.Vec {
	y := api.V(0, 1, 0)
	top := api.V(0, 0, lead)
	dir := api.V(0, 0, 1).RotatedAroundAxis(y, corner*math.Pi/180)
	b := bend * math.Pi / 180
	c1 := top.Plus(dir.ScaledBy(span / 3))
	far := top.Plus(dir.RotatedAroundAxis(y, b/2).ScaledBy(span))
	c2 := far.Minus(dir.RotatedAroundAxis(y, b).ScaledBy(span / 3))
	return []api.Vec{api.V(0, 0, 0), top, c1, c2, far}
}

// cornerCubicLength is the length of the cubic of cubicCornerPath, the line
// first, summed over fine chords.
func cornerCubicLength(corner, bend, lead, span float64) float64 {
	p := cubicCornerPoints(corner, bend, lead, span)
	at := func(t float64) api.Vec {
		s := 1 - t
		return p[1].ScaledBy(s * s * s).Plus(p[2].ScaledBy(3 * s * s * t)).Plus(p[3].ScaledBy(3 * s * t * t)).Plus(p[4].ScaledBy(t * t * t))
	}
	const n = 20000
	l, prev := 0.0, at(0)
	for i := 1; i <= n; i++ {
		q := at(float64(i) / n)
		l += q.Minus(prev).Magnitude()
		prev = q
	}
	return l
}

// cubicCornerRefused is a problem a sweep at a sharp corner beside a
// cubic may give by name: the corner, unjoined, or a path too tight or
// passing through itself.
func cubicCornerRefused(p string) bool {
	for _, s := range []string{"next to a cubic", "passes through itself", "too tight", "too short"} {
		if strings.Contains(p, s) {
			return true
		}
	}
	return false
}

// A sharp corner beside a cubic, which the kernel's fitted sweep can't
// mitre, is made of Booleans: the path cut at the corner, each side
// swept, and the outside of the turn filled by where the two sides,
// carried on past the corner along their tangents, overlap. A bar 2
// wide and 4 high: with the cubic straight it is the mitre of two
// lines, the bar's length times its section; bent, the same within
// 0.5%, the cubic's length summed by chords (a band of constant width
// along a curve has its length times its width, and near the corner the
// cubic's curve makes the inside's overlap and the outside's fill differ
// from a mitre's by a little: 0.05% at 45° and 90°, 0.3% at 120°). One step of undo takes the
// sweep back. A twist or a corner radius leaves it refused, by name,
// with no solid.
func TestSweepSharpCornerBesideCubic(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("each sweep joins its pieces by Booleans: a few seconds")
	}
	bar := func(d *api.Doc) {
		d.Features.SketchIn(d.Workplane("XY"), "profile")
		d.Sketch.Rect(api.V(-1, -2, 0), api.V(1, 2, 0))
		step(d)
	}
	for _, c := range []struct {
		cubicFirst         bool
		corner, bend, lead float64
	}{
		{false, 90, 0, 10},
		{false, 60, 0, 10},
		{false, 90, 40, 10},
		{false, -120, -60, 10},
		{false, 45, 90, 10},
		{true, 90, 40, 10},
	} {
		name := fmt.Sprintf("cubic first %v, corner %g, bend %g", c.cubicFirst, c.corner, c.bend)
		d := api.New()
		path := cubicCornerPath(d, c.cubicFirst, c.corner, c.bend, c.lead, 12)
		bar(d)
		groups := len(d.Groups())
		s := d.Features.Sweep(path, "")
		p, r := s.Problem(), s.Analyze().Check()
		if p != "" || !r.Sound() {
			t.Errorf("%s: %q %s, want made sound", name, p, r.String())
			continue
		}
		switch {
		case c.bend == 0:
			if want := 8 * (c.lead + 12); math.Abs(r.Volume-want) > 1e-6*want {
				t.Errorf("%s: volume %.6f, want %.6f", name, r.Volume, want)
			}
		case !c.cubicFirst:
			want := 8 * (c.lead + cornerCubicLength(c.corner, c.bend, c.lead, 12))
			if math.Abs(r.Volume-want) > 5e-3*want {
				t.Errorf("%s: volume %.4f, want %.4f", name, r.Volume, want)
			}
		}
		if !d.Edit.Undo() || len(d.Groups()) != groups {
			t.Errorf("%s: undo left %d groups, want %d", name, len(d.Groups()), groups)
		}
		if !d.Edit.Redo() || len(d.Groups()) != groups+1 {
			t.Errorf("%s: redo didn't bring the sweep back", name)
		}
	}
	// Twisted or rounded: refused by name.
	for _, how := range []string{"twist", "radius"} {
		d := api.New()
		path := cubicCornerPath(d, false, 90, 40, 10, 12)
		bar(d)
		s := d.Features.Sweep(path, "")
		switch how {
		case "twist":
			s.AsSweep().SetTwist(90)
		case "radius":
			s.AsSweep().SetCornerRadius(2)
		}
		r := s.Analyze().Check()
		if p := s.Problem(); !strings.Contains(p, "next to a cubic") || !r.Empty() {
			t.Errorf("%s: %q, want refused naming the cubic, with no solid", how, p)
		}
	}
}

// Sharp corners beside cubics over both orders, corners either way,
// cubics bent either way, two leads and three profiles: every sweep made
// sound, or a problem naming the corner; none unsound. Bars of 2 by 4
// are made at every corner and bend of these the default sample takes;
// circles are refused (their tubes' joins fail).
func TestSweepSharpCornerBesideCubicSweep(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("each point sweeps and joins: about half a second")
	}
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Bools("cubicFirst"),
			sweep.Floats("corner", -150, -90, -30, 30, 90, 150),
			sweep.Floats("bend", -90, -30, 0, 30, 90),
			sweep.Floats("lead", 5, 20),
			sweep.Floats("profile", 0, 1, 2),
		},
		MaxFailRate: 0,
		Sample:      24,
	}
	grid.Run(func(p sweep.Point) error {
		d := api.New()
		path := cubicCornerPath(d, p.Bool("cubicFirst"), p.Float("corner"), p.Float("bend"), p.Float("lead"), 12)
		d.Features.SketchIn(d.Workplane("XY"), "profile")
		switch p.Float("profile") {
		case 0:
			d.Sketch.Rect(api.V(-1, -2, 0), api.V(1, 2, 0))
		case 1:
			d.Sketch.Circle(api.V(0, 0, 0), 1)
		default:
			d.Sketch.Rect(api.V(-2, -1, 0), api.V(2, 1, 0))
		}
		s := d.Features.Sweep(path, "")
		if pr := s.Problem(); pr != "" {
			if !cubicCornerRefused(pr) {
				return fmt.Errorf("refused, but not by name: %q", pr)
			}
			return nil
		}
		if r := s.Analyze().Check(); !r.Sound() {
			return fmt.Errorf("made unsound, with no problem: %s", r.String())
		}
		return nil
	}).Require(t)
}
