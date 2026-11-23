package sketch_test

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// turnedVec is v turned by deg about z.
func turnedVec(v api.Vec, deg float64) api.Vec {
	c, s := math.Cos(deg*math.Pi/180), math.Sin(deg*math.Pi/180)
	return api.V(c*v.X-s*v.Y, s*v.X+c*v.Y, v.Z)
}

// partCircle is a circle of part g, whose normal is the part's.
func partCircle(t *testing.T, g api.Group) api.Entity {
	t.Helper()
	return first(t, g, "circle", func(api.Entity) bool { return true })
}

// A part's normal turned turns the part along its mechanism's freedom:
// the hinged bar's hole's normal turned 30° about z, the hinge's axis,
// turns the bar 30° about its hinge, its far end on its arc; tilted 30°
// about x, which the hinge doesn't let it, the bar stays. Before, the
// part was put where it was turned and solved back.
func TestOrientFollowsHinge(t *testing.T) {
	t.Parallel()
	_, _, bar, far, pivot := hingedBar(t)
	c := partCircle(t, bar)
	r := flat(far.Pos().Minus(pivot)).Magnitude()
	start := angleAbout(far.Pos(), pivot)
	u, w := c.Orientation()
	c.SetOrientation(turnedVec(u, 30), turnedVec(w, 30))
	got := angleAbout(far.Pos(), pivot) - start
	if math.Abs(got-30) > 0.01 || !bar.Solved() {
		t.Errorf("turned 30° about the hinge: the bar turned %.4f°, solved %v", got, bar.Solved())
	}
	if off := flat(far.Pos().Minus(pivot)).Magnitude() - r; math.Abs(off) > 1e-6 {
		t.Errorf("the end is %g off its arc", off)
	}
	if u2, _ := c.Orientation(); u2.Minus(turnedVec(u, 30)).Magnitude() > 1e-4 {
		t.Errorf("the normal's u is %v, want %v", u2, turnedVec(u, 30))
	}
	at := far.Pos()
	u, w = c.Orientation()
	tilt := func(v api.Vec) api.Vec {
		cs, s := math.Cos(math.Pi/6), math.Sin(math.Pi/6)
		return api.V(v.X, cs*v.Y-s*v.Z, s*v.Y+cs*v.Z)
	}
	c.SetOrientation(tilt(u), tilt(w))
	if moved := far.Pos().Minus(at).Magnitude(); moved > 1e-6 || !bar.Solved() {
		t.Errorf("tilted off the hinge, the bar moved %g, solved %v", moved, bar.Solved())
	}
}

// Turning a point of the hinged bar, as Shift or Ctrl does in the
// window, turns the bar about its hinge: about z through its far end,
// a turn the hinge allows only about the pin, it turns 40° about the
// pin, the end leaving the point it was turned about; about an axis
// leaning 45° off z it turns by the share about z, some 28° to 40°;
// and the four-bar's crank turned 150° in one call turns the whole way
// in steps, the loop kept closed, the rocker following.
func TestPullTurnHingedLid(t *testing.T) {
	t.Parallel()
	_, _, bar, far, pivot := hingedBar(t)
	r := flat(far.Pos().Minus(pivot)).Magnitude()
	start := angleAbout(far.Pos(), pivot)
	far.Turn(api.V(0, 0, 1), 40)
	got := angleAbout(far.Pos(), pivot) - start
	if math.Abs(got-40) > 0.01 || !bar.Solved() {
		t.Errorf("turned 40° about z: the bar turned %.4f° about its pin, solved %v", got, bar.Solved())
	}
	if off := flat(far.Pos().Minus(pivot)).Magnitude() - r; math.Abs(off) > 1e-6 {
		t.Errorf("the end is %g off its arc", off)
	}
	start = angleAbout(far.Pos(), pivot)
	far.Turn(api.V(1, 0, 1), 40)
	got = angleAbout(far.Pos(), pivot) - start
	if got < 27 || got > 40 || !bar.Solved() {
		t.Errorf("turned 40° about a leaning axis: the bar turned %.4f° about its pin, solved %v", got, bar.Solved())
	}

	_, _, crank, _, rocker, closeAt := fourBar(t, true)
	end := partCircle(t, crank).Center()
	before := heading(crank)
	r0 := heading(rocker)
	end.Turn(api.V(0, 0, 1), 150)
	turned := math.Mod(heading(crank)-before+720, 360)
	if math.Abs(turned-150) > 0.01 || closeAt() > 1e-6 || !rocker.Solved() || math.Abs(heading(rocker)-r0) < 5 {
		t.Errorf("the crank turned 150°: turned %.4f°, the loop open by %g, solved %v, rocker %.2f° from %.2f°",
			turned, closeAt(), rocker.Solved(), heading(rocker), r0)
	}
}

// The guide's lid on its driven hinge, kept between 0 and 110°, turned
// by its front corner about x, the hinge's direction, opens as far as
// it is turned and writes the turn into its drive, angle: 60° open, a
// turn of 100° more stops at 110°, and one of 30° back closes it to
// 80°. A turn about x by minus the angle opens it (TestDragDrivenLid).
func TestPullTurnDrivenLid(t *testing.T) {
	t.Parallel()
	d, err := api.Open(filepath.Join("..", "..", "testdata", "ui", "mechanism", "boxlid.tlspr"))
	if err != nil {
		t.Fatal(err)
	}
	gs := d.Groups()
	lid := gs[len(gs)-1]
	var front api.Entity
	for _, e := range lid.Entities() {
		if e.Kind() != "point" {
			continue
		}
		p := e.Pos()
		if front.H.V == 0 || p.Z > front.Pos().Z+1e-6 || math.Abs(p.Z-front.Pos().Z) < 1e-6 && p.Y < front.Pos().Y-1e-6 {
			front = e
		}
	}
	for _, tc := range []struct{ by, want float64 }{{-60, 60}, {-100, 110}, {30, 80}} {
		front.Turn(api.V(1, 0, 0), tc.by)
		p, _ := d.Params.Get("angle")
		if math.Abs(p.Value-tc.want) > 0.01 || !lid.Solved() {
			t.Errorf("turned %g°: angle %g, want %g; solved %v", tc.by, p.Value, tc.want, lid.Solved())
		}
	}
}

// A part that moves in a mechanism but has no mate turns about the
// point it is turned about, as any linked part did before mechanisms;
// a bar on a driven hinge turns about its pin, its drive taking the
// turn; and a turn is refused for a point of a sketch.
func TestPullTurnFixedAndFree(t *testing.T) {
	t.Parallel()
	_, _, bar := twoBars(t)
	far := uprights(t, bar)[60].Start()
	at := far.Pos()
	u0, _ := bar.AsLink().Orientation()
	far.Turn(api.V(0, 0, 1), 30)
	u1, _ := bar.AsLink().Orientation()
	if !near3(far.Pos(), at) || u1.Minus(turnedVec(u0, 30)).Magnitude() > 1e-4 {
		t.Errorf("a free part turned 30° about its corner: the corner went to %v from %v, its x to %v, want %v",
			far.Pos(), at, u1, turnedVec(u0, 30))
	}

	d, ground, driven := twoBars(t)
	d.Sketch.Anywhere3D()
	ug, ub := uprights(t, ground), uprights(t, driven)
	d.Assembly.HingeDriven(ug[60], ub[0])
	corner, pivot := ub[60].Start(), ug[60].Start().Pos()
	start := angleAbout(corner.Pos(), pivot)
	corner.Turn(api.V(0, 0, 1), 20)
	// The pin runs from the top down: a turn anticlockwise seen from
	// above is a negative angle.
	p, _ := d.Params.Get("angle")
	if got := angleAbout(corner.Pos(), pivot) - start; math.Abs(got-20) > 0.01 || math.Abs(p.Value+20) > 0.01 || !driven.Solved() {
		t.Errorf("a driven part turned 20°: it turned %.4f° about its pin, angle %g, solved %v", got, p.Value, driven.Solved())
	}

	s := api.New()
	l := s.Sketch.Line(api.V(0, 0, 0), api.V(10, 0, 0))
	func() {
		defer func() {
			if recover() == nil {
				t.Error("a turn of a sketch's point: no error")
			}
		}()
		l.Start().Turn(api.V(0, 0, 1), 10)
	}()
}
