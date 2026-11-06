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

// zxAngle is a point's angle about c in the ZX workplane, from its u
// (z) toward its v (x), the way the workplane's arcs run.
func zxAngle(c, p api.Vec) float64 { return math.Atan2(p.X-c.X, p.Z-c.Z) }

// turnFrom is how far, in [0, 2π), the angle b lies on from a.
func turnFrom(a, b float64) float64 { return math.Mod(b-a+4*math.Pi, 2*math.Pi) }

// pathPart is a line from a to b, or an arc about c from a to b through
// mid, of a path the tests draw in ZX, for their own measure of what the
// profile passes through.
type pathPart struct {
	arc          bool
	a, b, c, mid api.Vec
}

// foot is how far p lies from the part, in the ZX plane, where the line
// from p square to the part meets it inside the part; false where it
// meets it outside, past an end.
func (pp pathPart) foot(x, z float64) (float64, bool) {
	if !pp.arc {
		dx, dz := pp.b.X-pp.a.X, pp.b.Z-pp.a.Z
		l2 := dx*dx + dz*dz
		t := ((x-pp.a.X)*dx + (z-pp.a.Z)*dz) / l2
		if t < 0 || t > 1 {
			return 0, false
		}
		return math.Hypot(x-pp.a.X-t*dx, z-pp.a.Z-t*dz), true
	}
	p := api.V(x, 0, z)
	span := turnFrom(zxAngle(pp.c, pp.a), zxAngle(pp.c, pp.b))
	if turnFrom(zxAngle(pp.c, pp.a), zxAngle(pp.c, pp.mid)) > span {
		// The arc runs the other way round, from b to a.
		span = turnFrom(zxAngle(pp.c, pp.b), zxAngle(pp.c, pp.a))
		if turnFrom(zxAngle(pp.c, pp.b), zxAngle(pp.c, p)) > span {
			return 0, false
		}
	} else if turnFrom(zxAngle(pp.c, pp.a), zxAngle(pp.c, p)) > span {
		return 0, false
	}
	r := pp.a.Minus(pp.c).Magnitude()
	return math.Abs(math.Hypot(x-pp.c.X, z-pp.c.Z) - r), true
}

// draw draws the part in the active ZX sketch.
func (pp pathPart) draw(d *api.Doc) api.Entity {
	if !pp.arc {
		return d.Sketch.Line(pp.a, pp.b)
	}
	if turnFrom(zxAngle(pp.c, pp.a), zxAngle(pp.c, pp.mid)) < turnFrom(zxAngle(pp.c, pp.a), zxAngle(pp.c, pp.b)) {
		return d.Sketch.Arc(pp.c, pp.a, pp.b)
	}
	return d.Sketch.Arc(pp.c, pp.b, pp.a)
}

// bulbParts are the turn at the end of a hairpin in ZX: from a line along
// z at x = 0 ending at z0, heading the way of dir (1 up, -1 down), round
// to a line along z at x = gap heading back. A fillet of radius 4 turns
// away from the other line, a circle of radius 8 goes round, and a
// second fillet turns into the other line, all tangent: the lines lie
// nearer each other than a half circle wider than the profile allows.
func bulbParts(gap, z0, dir float64) []pathPart {
	const f, r = 4, 8
	cx := gap / 2
	c := math.Sqrt((f+r)*(f+r) - (cx+f)*(cx+f))
	f1, f2 := api.V(-f, 0, z0), api.V(gap+f, 0, z0)
	big := api.V(cx, 0, z0+dir*c)
	// Where the fillets meet the circle: on the line between centres, f
	// from the fillet's.
	t1 := f1.Plus(big.Minus(f1).WithMagnitude(f))
	t2 := f2.Plus(big.Minus(f2).WithMagnitude(f))
	a, b := api.V(0, 0, z0), api.V(gap, 0, z0)
	halfway := func(c, p, q api.Vec) api.Vec {
		return c.Plus(p.Minus(c).Plus(q.Minus(c)).WithMagnitude(p.Minus(c).Magnitude()))
	}
	return []pathPart{
		{arc: true, a: a, b: t1, c: f1, mid: halfway(f1, a, t1)},
		{arc: true, a: t1, b: t2, c: big, mid: big.Plus(api.V(0, 0, dir*r))},
		{arc: true, a: t2, b: b, c: f2, mid: halfway(f2, t2, b)},
	}
}

// hairpinParts are a line up the z axis of length h, a bulb round to a
// line gap along x coming back down, and, closed, a bulb at the bottom
// round to the start. Open, the path starts at the origin, where the
// profile is drawn; closed, its first line's middle is there, the
// profile across it.
func hairpinParts(h, gap float64, closed bool) []pathPart {
	lo := 0.0
	if closed {
		lo = -h / 2
	}
	parts := []pathPart{{a: api.V(0, 0, lo), b: api.V(0, 0, lo+h)}}
	parts = append(parts, bulbParts(gap, lo+h, 1)...)
	parts = append(parts, pathPart{a: api.V(gap, 0, lo+h), b: api.V(gap, 0, lo)})
	if closed {
		parts = append(parts, bulbParts(gap, lo, -1)...)
	}
	return parts
}

// drawParts draws the parts in a new ZX sketch, the path.
func drawParts(d *api.Doc, parts []pathPart) []api.Entity {
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	var path []api.Entity
	for _, pp := range parts {
		path = append(path, pp.draw(d))
	}
	return path
}

// passedThrough is the volume a profile passes through swept along the
// parts, a circle of radius rad (square false) or a square of side
// 2 rad with sides along y and across the path in its plane: the union
// of the profile placed square to the path at each of its points, by
// the test's own measure, on a grid in ZX of cells h. A point in the
// plane is in the square placed at a point of the path where the line
// from it square to the path, there, is no longer than rad; above it,
// in the circle to the height that leaves, from the nearest such.
func passedThrough(parts []pathPart, rad float64, square bool, h float64) float64 {
	lo, hi := api.V(math.Inf(1), 0, math.Inf(1)), api.V(math.Inf(-1), 0, math.Inf(-1))
	for _, pp := range parts {
		ps := []api.Vec{pp.a, pp.b}
		if pp.arc {
			r := pp.a.Minus(pp.c).Magnitude()
			ps = append(ps, pp.c.Plus(api.V(-r, 0, -r)), pp.c.Plus(api.V(r, 0, r)))
		}
		for _, p := range ps {
			lo.X, lo.Z, hi.X, hi.Z = min(lo.X, p.X), min(lo.Z, p.Z), max(hi.X, p.X), max(hi.Z, p.Z)
		}
	}
	v := 0.0
	for x := lo.X - rad - 1 + h/2; x < hi.X+rad+1; x += h {
		for z := lo.Z - rad - 1 + h/2; z < hi.Z+rad+1; z += h {
			d := math.Inf(1)
			for _, pp := range parts {
				if f, ok := pp.foot(x, z); ok {
					d = min(d, f)
				}
			}
			switch {
			case d > rad:
			case square:
				v += h * h * 2 * rad
			default:
				v += h * h * 2 * math.Sqrt(rad*rad-d*d)
			}
		}
	}
	return v
}

// loopCubic draws, in ZX, one cubic from (-20, 0, 0) to (20, 0, 0)
// looping up through (0, 0, 45) and crossing itself at (0, 0, 18), its
// tightest bend of radius 8.
func loopCubic(d *api.Doc) []api.Entity {
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	return []api.Entity{d.Sketch.Cubic(api.V(-20, 0, 0), api.V(60, 0, 60), api.V(-60, 0, 60), api.V(20, 0, 0))}
}

// A sweep round a closed path, or a fitted one, twisted, held to a
// direction or along a cubic, that comes back within the profile's reach
// of where it has been is made in spans each clear of itself, joined, as
// one along lines and arcs is: a hairpin whose legs lie nearer than the
// profile is wide, its bends wider; the same with its ends joined round
// in a ring; a cubic looping across itself. Each is sound and, along
// lines and arcs, of the volume the profile passes through by the test's
// own measure (twisted, the square turning about the path, no such
// measure is at hand). Where the spans don't join it is refused, naming
// it, with no solid (2026-11-04): a round profile held to a direction,
// whose fitted skins cross where the legs overlap, fail to unite at this
// tolerance (at the default one they do); a square held, its sides on
// both legs in one plane, fitted skins lying on each other; and a
// twisted ring, the join of its spans left open.
func TestSweepCrossesItselfFittedOrClosed(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("fitted sweeps made in spans and joined, tens of seconds")
	}
	circle := func(rad float64) func(d *api.Doc) {
		return func(d *api.Doc) {
			d.Features.SketchIn(d.Workplane("XY"), "profile")
			d.Sketch.Circle(api.V(0, 0, 0), rad)
		}
	}
	square := func(d *api.Doc) { squareProfile(d, 4) }
	for _, c := range []struct {
		name    string
		parts   []pathPart
		profile func(d *api.Doc)
		rad     float64
		square  bool
		set     func(s api.SweepGroup)
		made    bool
	}{
		{"hairpin", hairpinParts(20, 4, false), circle(2.5), 2.5, false, nil, true},
		{"ring", hairpinParts(20, 4, true), circle(2.5), 2.5, false, nil, true},
		{"hairpin, square, twisted", hairpinParts(20, 3.5, false), square, 0, true,
			func(s api.SweepGroup) { s.SetTwist(90) }, true},
		{"loop, square, twisted", nil, func(d *api.Doc) {
			d.Features.SketchIn(d.Workplane("XY"), "profile")
			squareAt(d, api.V(-20, 0, 0), 4)
		}, 0, true, func(s api.SweepGroup) { s.SetTwist(90) }, true},
		{"hairpin, held", hairpinParts(20, 4, false), circle(2.5), 0, false,
			func(s api.SweepGroup) { s.SetSweepUp(api.V(0, 1, 0)) }, false},
		{"hairpin, square, held", hairpinParts(20, 3.5, false), square, 0, true,
			func(s api.SweepGroup) { s.SetSweepUp(api.V(0, 1, 0)) }, false},
		{"ring, square, twisted", hairpinParts(20, 3.5, true), square, 0, true,
			func(s api.SweepGroup) { s.SetTwist(360) }, false},
	} {
		// Under the race detector, minutes a case, one fitted sweep made
		// in spans and joined stands for the rest: the spans are made and
		// joined on the same goroutines whatever the path.
		if fixtures.RaceDetector && c.name != "hairpin, square, twisted" {
			continue
		}
		// The round profile held refuses after 80 s of processor, four
		// times the next case; the square held is refused the same way
		// in 5, so the round one waits for make test-sweeps.
		if c.name == "hairpin, held" && !sweep.Full() {
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			d := api.New()
			fineChord(d, 0.002)
			var path []api.Entity
			if c.parts != nil {
				path = drawParts(d, c.parts)
			} else {
				path = loopCubic(d)
			}
			c.profile(d)
			step(d)
			groups := len(d.Groups())
			s := d.Features.Sweep(path, "")
			if c.set != nil {
				c.set(s.AsSweep())
			}
			p, r := s.Problem(), s.Analyze().Check()
			if !c.made {
				if !strings.Contains(p, sketch.ErrSweepCrossesItself.Error()) || !r.Empty() {
					t.Fatalf("%q %s, want refused as passing through itself, with no solid", p, r.String())
				}
				return
			}
			if p != "" || !r.Sound() {
				t.Fatalf("%q %s, want made sound", p, r.String())
			}
			if c.rad > 0 {
				want := passedThrough(c.parts, c.rad, c.square, 0.02)
				if math.Abs(r.Volume-want) > 2e-3*want {
					t.Errorf("volume %.3f, want %.3f", r.Volume, want)
				}
			}
			if !d.Edit.Undo() || len(d.Groups()) != groups {
				t.Errorf("undo left %d groups, want %d", len(d.Groups()), groups)
			}
		})
	}
}

// Fitted sweeps and closed ones whose paths come back over themselves,
// every one sound or a problem: none is made crossing itself. Hairpins
// and rings whose legs lie nearer than the profile reaches, rigid, held
// to a direction or twisted, and a cubic looping across itself, of a
// round profile, a smaller one and a square. On 2026-11-04, of the
// grid's 54 points, 33 are made sound and 21 are a problem, where before
// spans 21 were made (those that don't come back over themselves) and
// 33 refused; TestSweepCrossesItselfFittedOrClosed says which kinds are
// still refused. The loop has no gap, so 9 of the 54 were the same
// sweep again: of the 45 left, 31 are made sound and 14 a problem
// (2026-11-06).
func TestSweepCrossesItselfFittedSweep(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("fitted sweeps made in spans and joined, a minute")
	}
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("path", "hairpin", "ring", "cubic"),
			sweep.Strings("frames", "rigid", "held", "twisted"),
			sweep.Strings("profile", "circle", "small", "square"),
			sweep.Floats("gap", 3.5, 4.5),
		},
		MaxFailRate: 0,
		Sample:      12,
		Parallel:    true,
	}
	var made, refused atomic.Int64
	rep := grid.Run(func(p sweep.Point) error {
		d := api.New()
		var path []api.Entity
		at := api.V(0, 0, 0)
		switch p.String("path") {
		case "cubic":
			// The loop has no gap: its second gap is the first's point
			// again, left out.
			if p.Float("gap") != 3.5 {
				return nil
			}
			path, at = loopCubic(d), api.V(-20, 0, 0)
		default:
			path = drawParts(d, hairpinParts(20, p.Float("gap"), p.String("path") == "ring"))
		}
		d.Features.SketchIn(d.Workplane("XY"), "profile")
		switch p.String("profile") {
		case "circle":
			d.Sketch.Circle(at, 2.5)
		case "small":
			d.Sketch.Circle(at, 1.5)
		default:
			squareAt(d, at, 4)
		}
		s := d.Features.Sweep(path, "")
		switch p.String("frames") {
		case "held":
			s.AsSweep().SetSweepUp(api.V(0, 1, 0))
		case "twisted":
			turn := 90.0
			if p.String("path") == "ring" {
				turn = 360
			}
			s.AsSweep().SetTwist(turn)
		}
		if s.Problem() != "" {
			refused.Add(1)
			return nil
		}
		if r := s.Analyze().Check(); !r.Sound() {
			return fmt.Errorf("made unsound, with no problem: %s", r.String())
		}
		made.Add(1)
		return nil
	})
	t.Logf("%d made sound, %d a problem", made.Load(), refused.Load())
	rep.Require(t)
}
