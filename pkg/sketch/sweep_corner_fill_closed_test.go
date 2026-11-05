package sketch_test

import (
	"fmt"
	"math"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// closedCornerPath draws, in the ZX workplane, a closed path about a
// sharp corner beside an arc: a line up the z axis of length lead, its
// middle at the origin, the corner of corner degrees, an arc of radius
// rad turning by turn degrees, and, from the arc's end, a line on along
// its tangent down to the line's foot's level and one across to the
// foot, both corners mitred.
// closedCornerBands are its pieces, in order.
func closedCornerPath(d *api.Doc, corner, turn, lead, rad float64) []api.Entity {
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	foot, top, c, end, low, _ := closedCornerPoints(corner, turn, lead, rad)
	arc := func() api.Entity {
		if turn < 0 {
			return d.Sketch.Arc(c, end, top)
		}
		return d.Sketch.Arc(c, top, end)
	}
	return []api.Entity{
		d.Sketch.Line(foot, top), arc(),
		d.Sketch.Line(end, low), d.Sketch.Line(low, foot),
	}
}

// closedCornerPoints are closedCornerPath's: the line's foot and top,
// the arc's centre and end, and the corner below the arc's end; ok is
// false where the arc ends too near the first line, or too low, for the
// lines back to make a loop.
func closedCornerPoints(corner, turn, lead, rad float64) (foot, top, c, end, low api.Vec, ok bool) {
	y := api.V(0, 1, 0)
	foot, top = api.V(0, 0, -lead/2), api.V(0, 0, lead/2)
	dir := api.V(0, 0, 1).RotatedAroundAxis(y, corner*math.Pi/180)
	c = top.Plus(dir.RotatedAroundAxis(y, math.Copysign(math.Pi/2, turn)).ScaledBy(rad))
	end = c.Plus(top.Minus(c).RotatedAroundAxis(y, turn*math.Pi/180))
	// On from the arc's end along its tangent there, down to the foot's
	// level.
	h := api.V(0, 0, 1).RotatedAroundAxis(y, (corner+turn)*math.Pi/180)
	if h.Z > -0.2 || end.Z < foot.Z+6 {
		return foot, top, c, end, end, false
	}
	low = end.Plus(h.ScaledBy((foot.Z - end.Z) / h.Z))
	return foot, top, c, end, low, math.Abs(low.X) > 6 && math.Abs(end.X) > 3
}

func closedCornerBands(corner, turn, lead, rad float64) []band2 {
	foot, top, c, end, low, _ := closedCornerPoints(corner, turn, lead, rad)
	xz := func(v api.Vec) [2]float64 { return [2]float64{v.X, v.Z} }
	line := func(a, b api.Vec) band2 {
		return band2{p: xz(a), d: xz(b.Minus(a).WithMagnitude(1)), l: b.Minus(a).Magnitude()}
	}
	return []band2{
		line(foot, top),
		{arc: true, c: xz(c), r0: xz(top.Minus(c)), an: turn * math.Pi / 180},
		line(end, low), line(low, foot),
	}
}

// closedBandArea is the area a profile reaching w either side sweeps
// round a closed path of the bands: each band, and at each corner the
// overlap of the band before carried on past it and the band after
// carried back before it, which is the mitre between two lines too.
// Each is carried half as far as the kernel's fill carries it: an arc
// carried back by a quarter turn can cross the line carried on a second
// time, far from the corner, as at a corner of 150° beside an arc of
// radius 20 turning against it, an overlap that is no part of the
// corner's fill.
func closedBandArea(bands []band2, w float64, rows int) float64 {
	return regionArea(func(x [2]float64) bool {
		for k, a := range bands {
			b := bands[(k+1)%len(bands)]
			if a.holds(x, w, 0, a.size()) ||
				a.holds(x, w, a.size(), a.size()+a.carry()/2) && b.holds(x, w, -b.carry()/2, 0) {
				return true
			}
		}
		return false
	}, rows)
}

// closedCornerCase is one closed path about a sharp corner beside an
// arc and its profile: 0 a bar 2 across the path in its plane and 4
// high, 1 a circle of radius 1, 2 a bar 4 across and 2 high.
type closedCornerCase struct {
	corner, turn, lead, rad, profile float64
}

func (c closedCornerCase) build(tolPct float64) (*api.Doc, []api.Entity) {
	d := api.New()
	if tolPct > 0 {
		d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = tolPct })
	}
	path := closedCornerPath(d, c.corner, c.turn, c.lead, c.rad)
	d.Features.SketchIn(d.Workplane("XY"), "profile")
	switch c.profile {
	case 0:
		d.Sketch.Rect(api.V(-1, -2, 0), api.V(1, 2, 0))
	case 1:
		d.Sketch.Circle(api.V(0, 0, 0), 1)
	default:
		d.Sketch.Rect(api.V(-2, -1, 0), api.V(2, 1, 0))
	}
	step(d)
	return d, path
}

// A closed path with a corner beside an arc of 170°, either way, too
// sharp for the kernel to join: the corner is filled as an open path's
// is, the sweep going round from the profile, drawn across the first
// line, and back to it with no caps. Such a corner was refused on a
// closed path before 2026-11-04. Corners of 150° beside an arc of radius
// 20, which the kernel joins, are measured the same way beside them. The
// solids are sound and their volumes the region's the profile sweeps,
// integrated by rows, times the bar's height (within 0.3%, at a chord
// tolerance of 0.05%).
func TestSweepClosedSharpCornerBesideArc(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("the volumes are integrated by rows: a few seconds")
	}
	cases := []closedCornerCase{
		{150, -30, 30, 20, 0},
		{-150, 30, 30, 20, 0},
		{170, -60, 20, 12, 0},
		{-170, -60, 30, 12, 2},
	}
	if fixtures.RaceDetector {
		// A minute there; the corner of 170° is the one filled.
		cases = cases[2:3]
	}
	for _, c := range cases {
		name := fmt.Sprintf("corner %g, turn %g, radius %g", c.corner, c.turn, c.rad)
		if _, _, _, _, _, ok := closedCornerPoints(c.corner, c.turn, c.lead, c.rad); !ok {
			t.Fatalf("%s: the path doesn't close into a loop", name)
		}
		d, path := c.build(0.05)
		before := saved(t, d)
		s := d.Features.Sweep(path, "")
		p, r := s.Problem(), s.Analyze().Check()
		if p != "" || !r.Sound() {
			t.Errorf("%s: %q %s, want made sound", name, p, r.String())
			continue
		}
		w, height := 1.0, 4.0
		if c.profile == 2 {
			w, height = 2, 2
		}
		want := height * closedBandArea(closedCornerBands(c.corner, c.turn, c.lead, c.rad), w, 1500)
		if math.Abs(r.Volume-want) > 3e-3*want {
			t.Errorf("%s: volume %.4f, want %.4f", name, r.Volume, want)
		}
		undoRedo(t, d, before)
	}
}

// Closed paths about a sharp corner beside an arc, corners of 150° and
// 170° either way, the arc turning with the corner or against it, three
// radii, two leads and three profiles: every sweep made sound, or a
// problem naming the corner radius or the sweep passing through itself;
// none unsound.
func TestSweepClosedSharpCornerBesideArcSweep(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("each point sweeps and joins: about half a second")
	}
	var made, refused, open atomic.Int64
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("corner", -170, -150, 150, 170),
			sweep.Floats("turn", -100, -60, -30, -10, 10, 30),
			sweep.Floats("rad", 4, 12, 20),
			sweep.Floats("lead", 20, 30),
			sweep.Floats("profile", 0, 1, 2),
		},
		MaxFailRate: 0,
		Sample:      24,
	}
	grid.Run(func(p sweep.Point) error {
		c := closedCornerCase{p.Float("corner"), p.Float("turn"), p.Float("lead"), p.Float("rad"), p.Float("profile")}
		if _, _, _, _, _, ok := closedCornerPoints(c.corner, c.turn, c.lead, c.rad); !ok {
			open.Add(1)
			return nil
		}
		d, path := c.build(0)
		s := d.Features.Sweep(path, "")
		if pr := s.Problem(); pr != "" {
			if !strings.Contains(pr, "corner radius") && !strings.Contains(pr, "passes through itself") {
				return fmt.Errorf("refused, but not by name: %q", pr)
			}
			refused.Add(1)
			return nil
		}
		if r := s.Analyze().Check(); !r.Sound() {
			return fmt.Errorf("made unsound, with no problem: %s", r.String())
		}
		made.Add(1)
		return nil
	}).Require(t)
	t.Logf("%d made sound, %d refused, %d no loop", made.Load(), refused.Load(), open.Load())
}
