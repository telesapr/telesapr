package sketch_test

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// cubicCornerProfile draws the profile of the cubic corner grids in the
// XY workplane: 0 a bar 2 across and 4 high, 1 a circle of radius 1, 2
// a bar 4 across and 2 high, 3 the diamond.
func cubicCornerProfile(d *api.Doc, profile float64) {
	d.Features.SketchIn(d.Workplane("XY"), "profile")
	switch profile {
	case 0:
		d.Sketch.Rect(api.V(-1, -2, 0), api.V(1, 2, 0))
	case 1:
		d.Sketch.Circle(api.V(0, 0, 0), 1)
	case 2:
		d.Sketch.Rect(api.V(-2, -1, 0), api.V(2, 1, 0))
	default:
		cubicCornerDiamond(d)
	}
	step(d)
}

// cubicCornerRoundedPath is cubicCornerPath at lead 10 and span 12 with
// a corner between two lines after it, which a corner radius rounds: the
// cubic first, a line of 6 on from the line's end turned a quarter turn
// about y; the line first, a line of 6 on from the cubic's end along its
// tangent and one of 6 turned from it so.
func cubicCornerRoundedPath(d *api.Doc, cubicFirst bool, corner, bend float64) []api.Entity {
	path := cubicCornerPath(d, cubicFirst, corner, bend, 10, 12)
	y := api.V(0, 1, 0)
	var end, dir api.Vec
	if cubicFirst {
		b := bend * math.Pi / 180
		up := api.V(0, 0, 1)
		far := up.RotatedAroundAxis(y, b/2).ScaledBy(12)
		dir = up.RotatedAroundAxis(y, b).RotatedAroundAxis(y, corner*math.Pi/180)
		end = far.Plus(dir.ScaledBy(10))
	} else {
		p := cubicCornerPoints(corner, bend, 10, 12)
		dir = p[4].Minus(p[3]).WithMagnitude(1)
		end = p[4].Plus(dir.ScaledBy(6))
		path = append(path, d.Sketch.Line(p[4], end))
	}
	turned := end.Plus(dir.RotatedAroundAxis(y, math.Pi/2).ScaledBy(6))
	return append(path, d.Sketch.Line(end, turned))
}

// cubicCornerLoop draws, in the ZX workplane, a closed path about a
// sharp corner beside a cubic: cubicCornerPoints' line and cubic at
// lead 10 and span 12, moved down by 5 so the profile on the XY plane
// crosses the line, the line drawn from 8 further down, and from the
// cubic's end lines straight down to the line's foot's level and back
// to its foot. ok is false where the cubic ends too near the line or
// too low for the lines back to make a loop, or arrives going straight
// up, so that the line down would turn back on it.
func cubicCornerLoop(d *api.Doc, corner, bend float64) ([]api.Entity, bool) {
	p := cubicCornerPoints(corner, bend, 10, 12)
	for i := range p {
		p[i] = p[i].Minus(api.V(0, 0, 5))
	}
	foot := api.V(0, 0, -13)
	far := p[4]
	low := api.V(far.X, 0, foot.Z)
	// The cubic arriving straight up would make the line down turn back
	// on it.
	if math.Abs(far.X) < 3 || far.Z < foot.Z+3 || far.Minus(p[3]).WithMagnitude(1).Z > 0.99 {
		return nil, false
	}
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	return []api.Entity{
		d.Sketch.Line(foot, p[1]),
		d.Sketch.Cubic(p[1], p[2], p[3], p[4]),
		d.Sketch.Line(far, low), d.Sketch.Line(low, foot),
	}, true
}

// Sharp corners beside a cubic on paths whose frames turn otherwise
// than the corner does, made since 2026-11-09, where before each was
// refused by name: frames held to the corner's axis, a twist (held
// still beside the corner as far as its mitre would reach, and spread
// over the rest), a corner radius of 3 (rounding the path's other
// corners, this one left sharp) and closed paths (their frames turned
// to meet round them). A bar 2 by 4: held, the volume is the sweep's
// with its frames free within 1e-3; twisted, a bar's section keeps its
// area, and the volume is the untwisted one within 3e-3; rounded, with
// the cubic straight, the bar's section times the path's length within
// 1e-3 (the arc's mesh is short of it by its chords); the loops with
// the cubic straight, the section times the loop's length within 1e-6,
// twisted a whole turn within 3e-3. One step of undo takes an
// untwisted loop back. Frames held to another direction, a closed path
// twisted by less than whole turns and a twist with nowhere left to
// turn are refused by name, with no solid.
func TestSweepSharpCornerBesideCubicFrames(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("each sweep joins its pieces by Booleans: a few seconds")
	}
	type corner struct {
		cubicFirst   bool
		corner, bend float64
	}
	corners := []corner{{false, 90, 0}, {false, 90, 40}, {false, -120, -60}, {false, 45, 90}, {true, 90, 40}}
	plain := func(c corner) float64 {
		d := api.New()
		path := cubicCornerPath(d, c.cubicFirst, c.corner, c.bend, 10, 12)
		cubicCornerProfile(d, 0)
		return d.Features.Sweep(path, "").Analyze().Check().Volume
	}
	made := func(name string, d *api.Doc, path []api.Entity, set func(api.SweepGroup)) (float64, bool) {
		before := saved(t, d)
		s := d.Features.Sweep(path, "")
		if set != nil {
			step(d)
			set(s.AsSweep())
		}
		p, r := s.Problem(), s.Analyze().Check()
		if p != "" || !r.Sound() {
			t.Errorf("%s: %q %s, want made sound", name, p, r.String())
			return 0, false
		}
		if set == nil {
			undoRedo(t, d, before)
		}
		return r.Volume, true
	}
	for _, c := range corners {
		name := fmt.Sprintf("cubic first %v, corner %g, bend %g", c.cubicFirst, c.corner, c.bend)
		want := plain(c)
		for _, how := range []string{"held", "twisted"} {
			d := api.New()
			path := cubicCornerPath(d, c.cubicFirst, c.corner, c.bend, 10, 12)
			cubicCornerProfile(d, 0)
			set, rel := func(s api.SweepGroup) { s.SetSweepUp(api.V(0, 1, 0)) }, 1e-3
			if how == "twisted" {
				set, rel = func(s api.SweepGroup) { s.SetTwist(90) }, 3e-3
			}
			if v, ok := made(how+", "+name, d, path, set); ok && math.Abs(v-want) > rel*want {
				t.Errorf("%s, %s: volume %.4f, want %.4f within %g", how, name, v, want, rel)
			}
		}
		d := api.New()
		path := cubicCornerRoundedPath(d, c.cubicFirst, c.corner, c.bend)
		cubicCornerProfile(d, 0)
		v, ok := made("rounded, "+name, d, path, func(s api.SweepGroup) { s.SetCornerRadius(3) })
		if want := 8 * (28 + 1.5*math.Pi); ok && c.bend == 0 && !c.cubicFirst && math.Abs(v-want) > 1e-3*want {
			t.Errorf("rounded, %s: volume %.6f, want %.6f", name, v, want)
		}
	}
	for _, c := range corners[:4] {
		for _, twist := range []float64{0, 360} {
			name := fmt.Sprintf("loop, corner %g, bend %g, twist %g", c.corner, c.bend, twist)
			d := api.New()
			path, ok := cubicCornerLoop(d, c.corner, c.bend)
			if !ok {
				t.Fatalf("%s: the path doesn't close into a loop", name)
			}
			cubicCornerProfile(d, 0)
			var set func(api.SweepGroup)
			if twist != 0 {
				set = func(s api.SweepGroup) { s.SetTwist(twist) }
			}
			v, ok := made(name, d, path, set)
			if !ok || c.bend != 0 {
				continue
			}
			p := cubicCornerPoints(c.corner, 0, 10, 12)
			far := p[4].Minus(api.V(0, 0, 5))
			want, rel := 8*(18+12+(far.Z+13)+math.Abs(far.X)), 1e-6
			if twist != 0 {
				rel = 3e-3
			}
			if math.Abs(v-want) > rel*want {
				t.Errorf("%s: volume %.6f, want %.6f within %g", name, v, want, rel)
			}
		}
	}
	for _, c := range []struct {
		name, want string
		build      func(d *api.Doc) []api.Entity
		set        func(s api.SweepGroup)
	}{
		{"held across the corner's axis", "held to a direction turn otherwise",
			func(d *api.Doc) []api.Entity { return cubicCornerPath(d, false, 90, 40, 10, 12) },
			func(s api.SweepGroup) { s.SetSweepUp(api.V(1, 1, 0)) }},
		{"a loop twisted a quarter turn", "whole turns",
			func(d *api.Doc) []api.Entity { p, _ := cubicCornerLoop(d, 90, 40); return p },
			func(s api.SweepGroup) { s.SetTwist(90) }},
		{"twisted with nowhere to turn", "no stretch of the path left to turn on",
			func(d *api.Doc) []api.Entity { return cubicCornerPath(d, false, 150, 0, 3, 4) },
			func(s api.SweepGroup) { s.SetTwist(90) }},
	} {
		d := api.New()
		path := c.build(d)
		cubicCornerProfile(d, 0)
		s := d.Features.Sweep(path, "")
		c.set(s.AsSweep())
		r := s.Analyze().Check()
		if p := s.Problem(); !strings.Contains(p, c.want) || !r.Empty() {
			t.Errorf("%s: %q, want refused saying %q, with no solid", c.name, p, c.want)
		}
	}
}

// cubicCornerFramesRefused is a problem a sweep of the frames grids may
// give by name: cubicCornerRefused's, or one about the corner radius.
func cubicCornerFramesRefused(p string) bool {
	return cubicCornerRefused(p) || strings.Contains(p, "radius")
}

// Sharp corners beside a cubic with the frames held to the corner's
// axis, twisted by 90° or rounded by a radius of 3 elsewhere on the
// path, over both orders, corners ±30° to ±150°, bends −90° to 90° and
// the four profiles: every sweep made sound, or a problem naming the
// corner; none unsound. The counts of the whole grid, 60 corners each,
// are in ROADMAP.md.
func TestSweepSharpCornerBesideCubicFramesSweep(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("each point sweeps and joins: about a second")
	}
	var made, refused [3][4]atomic.Int64
	kinds := []string{"held", "twisted", "rounded"}
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("kind", kinds...),
			sweep.Bools("cubicFirst"),
			sweep.Floats("corner", -150, -90, -30, 30, 90, 150),
			sweep.Floats("bend", -90, -30, 0, 30, 90),
			sweep.Floats("profile", 0, 1, 2, 3),
		},
		MaxFailRate: 0,
		Sample:      36,
	}
	grid.Run(func(p sweep.Point) error {
		d := api.New()
		kind, cf, corner, bend := p.String("kind"), p.Bool("cubicFirst"), p.Float("corner"), p.Float("bend")
		var path []api.Entity
		if kind == "rounded" {
			path = cubicCornerRoundedPath(d, cf, corner, bend)
		} else {
			path = cubicCornerPath(d, cf, corner, bend, 10, 12)
		}
		cubicCornerProfile(d, p.Float("profile"))
		s := d.Features.Sweep(path, "")
		switch kind {
		case "held":
			s.AsSweep().SetSweepUp(api.V(0, 1, 0))
		case "twisted":
			s.AsSweep().SetTwist(90)
		default:
			s.AsSweep().SetCornerRadius(3)
		}
		ki, pi := slices.Index(kinds, kind), int(p.Float("profile"))
		if pr := s.Problem(); pr != "" {
			if !cubicCornerFramesRefused(pr) {
				return fmt.Errorf("refused, but not by name: %q", pr)
			}
			refused[ki][pi].Add(1)
			return nil
		}
		if r := s.Analyze().Check(); !r.Sound() {
			return fmt.Errorf("made unsound, with no problem: %s", r.String())
		}
		made[ki][pi].Add(1)
		return nil
	}).Require(t)
	for ki, k := range kinds {
		for pi := range 4 {
			t.Logf("%s, profile %d: %d made sound, %d refused", k, pi, made[ki][pi].Load(), refused[ki][pi].Load())
		}
	}
}

// Closed paths about a sharp corner beside a cubic, a line up through
// the profile, the cubic, and lines back (cubicCornerLoop), untwisted
// or twisted a whole turn, over corners ±30° to ±150°, bends −90° to 90°
// and the four profiles: every sweep made sound, or a problem naming the
// corner or the path passing through itself; none unsound.
func TestSweepSharpCornerBesideCubicLoopSweep(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("each point sweeps and joins: about a second")
	}
	var made, refused [2][4]atomic.Int64
	var open atomic.Int64
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("twist", 0, 360),
			sweep.Floats("corner", -150, -90, -30, 30, 90, 150),
			sweep.Floats("bend", -90, -30, 0, 30, 90),
			sweep.Floats("profile", 0, 1, 2, 3),
		},
		MaxFailRate: 0,
		Sample:      24,
	}
	grid.Run(func(p sweep.Point) error {
		d := api.New()
		path, ok := cubicCornerLoop(d, p.Float("corner"), p.Float("bend"))
		if !ok {
			open.Add(1)
			return nil
		}
		cubicCornerProfile(d, p.Float("profile"))
		s := d.Features.Sweep(path, "")
		if tw := p.Float("twist"); tw != 0 {
			s.AsSweep().SetTwist(tw)
		}
		ti, pi := 0, int(p.Float("profile"))
		if p.Float("twist") != 0 {
			ti = 1
		}
		if pr := s.Problem(); pr != "" {
			if !cubicCornerRefused(pr) {
				return fmt.Errorf("refused, but not by name: %q", pr)
			}
			refused[ti][pi].Add(1)
			return nil
		}
		if r := s.Analyze().Check(); !r.Sound() {
			return fmt.Errorf("made unsound, with no problem: %s", r.String())
		}
		made[ti][pi].Add(1)
		return nil
	}).Require(t)
	for ti := range 2 {
		for pi := range 4 {
			t.Logf("twist %d, profile %d: %d made sound, %d refused", 360*ti, pi, made[ti][pi].Load(), refused[ti][pi].Load())
		}
	}
	t.Logf("%d no loop", open.Load())
}
