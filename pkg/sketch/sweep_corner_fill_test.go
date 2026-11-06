package sketch_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// band2 is a piece of a path in the ZX workplane, as (x, z), swept by a
// profile reaching w either side of it in that plane: a line from p
// along the unit d of length l, or an arc about c from the radial r0
// turning by an (positive about +y, negative against).
type band2 struct {
	arc  bool
	p, d [2]float64
	l    float64
	c    [2]float64
	r0   [2]float64
	an   float64
}

// size is the piece's length, or its turn.
func (b band2) size() float64 {
	if b.arc {
		return math.Abs(b.an)
	}
	return b.l
}

// carry is how far the piece is carried past a corner for its fill:
// a quarter turn along an arc, twice its length along a line.
func (b band2) carry() float64 {
	if b.arc {
		return math.Pi / 2
	}
	return 2 * b.l
}

// holds says whether x lies in the piece swept by w carried on from s0
// to s1 of its length or turn.
func (b band2) holds(x [2]float64, w, s0, s1 float64) bool {
	if !b.arc {
		v := [2]float64{x[0] - b.p[0], x[1] - b.p[1]}
		s := v[0]*b.d[0] + v[1]*b.d[1]
		t := v[0]*b.d[1] - v[1]*b.d[0]
		return s >= s0 && s <= s1 && math.Abs(t) <= w
	}
	v := [2]float64{x[0] - b.c[0], x[1] - b.c[1]}
	r := math.Hypot(b.r0[0], b.r0[1])
	// The turn about +y from r0 to v, the way the arc goes, from a
	// quarter turn before its start.
	phi := math.Atan2(b.r0[1]*v[0]-b.r0[0]*v[1], b.r0[0]*v[0]+b.r0[1]*v[1])
	a := math.Copysign(1, b.an) * phi
	a = math.Mod(a+math.Pi/2+4*math.Pi, 2*math.Pi) - math.Pi/2
	return math.Abs(math.Hypot(v[0], v[1])-r) <= w && a >= s0 && a <= s1
}

// arcCornerPath draws, in the ZX workplane, a line of length lead and
// an arc of radius rad turning by turn degrees, about a sharp corner of
// corner degrees between them; the line first, or the arc. turnPath is
// it at a radius of 15.
func arcCornerPath(d *api.Doc, arcFirst bool, corner, turn, lead, rad float64) []api.Entity {
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	y := api.V(0, 1, 0)
	way := math.Copysign(math.Pi/2, turn)
	arc := func(c, from, to api.Vec) api.Entity {
		if turn < 0 {
			return d.Sketch.Arc(c, to, from)
		}
		return d.Sketch.Arc(c, from, to)
	}
	if !arcFirst {
		top := api.V(0, 0, lead)
		dir := api.V(0, 0, 1).RotatedAroundAxis(y, corner*math.Pi/180)
		c := top.Plus(dir.RotatedAroundAxis(y, way).ScaledBy(rad))
		end := c.Plus(top.Minus(c).RotatedAroundAxis(y, turn*math.Pi/180))
		return []api.Entity{d.Sketch.Line(api.V(0, 0, 0), top), arc(c, top, end)}
	}
	o := api.V(0, 0, 0)
	c := api.V(math.Copysign(rad, turn), 0, 0)
	end := c.Plus(o.Minus(c).RotatedAroundAxis(y, turn*math.Pi/180))
	along := end.Minus(c).RotatedAroundAxis(y, way).WithMagnitude(1).RotatedAroundAxis(y, corner*math.Pi/180)
	return []api.Entity{arc(c, o, end), d.Sketch.Line(end, end.Plus(along.ScaledBy(lead)))}
}

// arcCornerBands are the pieces arcCornerPath draws, in the same order.
func arcCornerBands(arcFirst bool, corner, turn, lead, rad float64) []band2 {
	y := api.V(0, 1, 0)
	way := math.Copysign(math.Pi/2, turn)
	xz := func(v api.Vec) [2]float64 { return [2]float64{v.X, v.Z} }
	if !arcFirst {
		top := api.V(0, 0, lead)
		dir := api.V(0, 0, 1).RotatedAroundAxis(y, corner*math.Pi/180)
		c := top.Plus(dir.RotatedAroundAxis(y, way).ScaledBy(rad))
		return []band2{
			{d: [2]float64{0, 1}, l: lead},
			{arc: true, c: xz(c), r0: xz(top.Minus(c)), an: turn * math.Pi / 180},
		}
	}
	o := api.V(0, 0, 0)
	c := api.V(math.Copysign(rad, turn), 0, 0)
	end := c.Plus(o.Minus(c).RotatedAroundAxis(y, turn*math.Pi/180))
	along := end.Minus(c).RotatedAroundAxis(y, way).WithMagnitude(1).RotatedAroundAxis(y, corner*math.Pi/180)
	return []band2{
		{arc: true, c: xz(c), r0: xz(o.Minus(c)), an: turn * math.Pi / 180},
		{p: xz(end), d: xz(along), l: lead},
	}
}

// joinedBandArea is the area of the region a profile reaching w either
// side sweeps along two pieces about a corner: each piece, and the
// overlap of both carried on past the corner, which fills its outside.
// Integrated by slanted rows, each crossing found by halving.
func joinedBandArea(bands []band2, w float64, rows int) float64 {
	a, b := bands[0], bands[1]
	return regionArea(func(x [2]float64) bool {
		return a.holds(x, w, 0, a.size()) || b.holds(x, w, 0, b.size()) ||
			a.holds(x, w, a.size(), a.size()+a.carry()) && b.holds(x, w, -b.carry(), 0)
	}, rows)
}

// regionArea is the area of the region of the ZX plane within 60 of
// the origin that held says a point is in, integrated by slanted rows,
// each crossing found by halving.
func regionArea(held func([2]float64) bool, rows int) float64 {
	return regionAreaIn(held, rows, 60)
}

// regionAreaIn is regionArea within half of the origin. A row is
// scanned in as many steps as there are rows, only to find where it
// crosses the region's edge, each crossing then halved 40 times: a step
// is under 0.1 mm at the rows the tests take (1500 within 60, 2400
// within 90), where the bands are 2 mm across or more; four times as
// many steps took four times as long and moved no volume past its
// check.
func regionAreaIn(held func([2]float64) bool, rows int, half float64) float64 {
	sn, cs := math.Sincos(0.37)
	inside := func(u, v float64) bool { return held([2]float64{cs*u - sn*v, sn*u + cs*v}) }
	h := 2 * half / float64(rows)
	cols := rows
	dx := 2 * half / float64(cols)
	area := 0.0
	for i := range rows {
		v := -half + (float64(i)+0.5)*h
		prev := inside(-half, v)
		for j := 1; j <= cols; j++ {
			u := -half + float64(j)*dx
			in := inside(u, v)
			if in == prev {
				continue
			}
			l, r := u-dx, u
			for range 40 {
				m := (l + r) / 2
				if inside(m, v) == prev {
					l = m
				} else {
					r = m
				}
			}
			if in {
				area -= l * h
			} else {
				area += l * h
			}
			prev = in
		}
	}
	return area
}

// A sharp corner beside an arc, 150° or 170°, that the kernel can't
// join: the line's wall passes outside the arc's at some of the
// profile's points, so their paths never cross. Made of Booleans, the
// path cut at the corner and the outside of the turn filled by where
// the two pieces carried on past it overlap, the solid is sound and its
// volume the joined region's, integrated by rows, times the bar's
// height 4 (within 0.3%: the rows and the mesh's chords). The arc turning
// against the corner, the overlap runs on as far as the pieces are
// carried: the sides never meet past the corner, and it is refused,
// naming the corner radius, with no solid. One step of undo takes the
// sweep back; the arcs' ends are no round numbers, so the group count
// says so (TestSweepCrossesItself).
func TestSweepSharpCornerBesideArc(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("the volumes are integrated by rows: a few seconds")
	}
	for _, c := range []struct {
		arcFirst                bool
		corner, turn, lead, rad float64
		made                    bool
	}{
		{false, 150, 30, 10, 12, true},
		{true, -150, -100, 10, 12, true},
		{false, 170, 100, 10, 4, true},
		{false, 170, -30, 10, 12, true},
		{true, -170, 30, 10, 12, true},
		{false, 150, -30, 10, 12, false},
		{true, 150, -100, 10, 4, false},
	} {
		name := fmt.Sprintf("arc first %v, corner %g, turn %g, radius %g", c.arcFirst, c.corner, c.turn, c.rad)
		d := api.New()
		path := arcCornerPath(d, c.arcFirst, c.corner, c.turn, c.lead, c.rad)
		d.Features.SketchIn(d.Workplane("XY"), "profile")
		d.Sketch.Rect(api.V(-1, -2, 0), api.V(1, 2, 0))
		step(d)
		groups := len(d.Groups())
		s := d.Features.Sweep(path, "")
		p, r := s.Problem(), s.Analyze().Check()
		if !c.made {
			if !strings.Contains(p, "never meet") || !strings.Contains(p, "corner radius") || !r.Empty() {
				t.Errorf("%s: %q %s, want refused, naming the corner radius, with no solid", name, p, r.String())
			}
			continue
		}
		if p != "" || !r.Sound() {
			t.Errorf("%s: %q %s, want made sound", name, p, r.String())
			continue
		}
		want := 4 * joinedBandArea(arcCornerBands(c.arcFirst, c.corner, c.turn, c.lead, c.rad), 1, 1500)
		if math.Abs(r.Volume-want) > 3e-3*want {
			t.Errorf("%s: volume %.4f, want %.4f", name, r.Volume, want)
		}
		if !d.Edit.Undo() || len(d.Groups()) != groups {
			t.Errorf("%s: undo left %d groups, want %d", name, len(d.Groups()), groups)
		}
		if !d.Edit.Redo() || len(d.Groups()) != groups+1 {
			t.Errorf("%s: redo didn't bring the sweep back", name)
		}
	}
}

// Sharp corners beside arcs over both orders, corners of 150° and 170°
// either way, arcs turning either way, two leads, three radii and three
// profiles: every sweep made sound, or a problem naming the corner
// radius or the sweep passing through itself; none unsound. Before
// 2026-11-04 the kernel refused every corner of these it couldn't join
// (at radius 4 and 12, all of them); of the 64 rectangles of radius 4
// and 12 at lead 10, 48 are made and 16 refused, the sides never meeting.
func TestSweepSharpCornerBesideArcSweep(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("each point sweeps and joins: about half a second")
	}
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Bools("arcFirst"),
			sweep.Floats("corner", -170, -150, 150, 170),
			sweep.Floats("turn", -100, -30, 30, 100),
			sweep.Floats("lead", 10, 25),
			sweep.Floats("rad", 4, 12, 15),
			sweep.Floats("profile", 0, 1, 2),
		},
		MaxFailRate: 0,
		Sample:      24,
	}
	grid.Run(func(p sweep.Point) error {
		d := api.New()
		path := arcCornerPath(d, p.Bool("arcFirst"), p.Float("corner"), p.Float("turn"), p.Float("lead"), p.Float("rad"))
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
			if !strings.Contains(pr, "corner radius") && !strings.Contains(pr, "passes through itself") &&
				!strings.Contains(pr, "too tight") {
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

// BenchmarkSweepSharpArc makes the sweep of a bar along a line and an
// arc at a corner of 150°, too sharp to join: two pieces, the corner's
// fill twice over and three Booleans.
func BenchmarkSweepSharpArc(b *testing.B) {
	for b.Loop() {
		d := api.New()
		path := arcCornerPath(d, false, 150, 100, 20, 12)
		d.Features.SketchIn(d.Workplane("XY"), "profile")
		d.Sketch.Rect(api.V(-1, -1.5, 0), api.V(1, 1.5, 0))
		if s := d.Features.Sweep(path, ""); s.Problem() != "" {
			b.Fatal(s.Problem())
		}
	}
}
