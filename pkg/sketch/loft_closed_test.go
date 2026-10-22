package sketch_test

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// ringOfCircles draws n circles of radius r round the z axis at radius
// big, each in the upright plane through the axis at its angle, and
// returns their sketches in order.
func ringOfCircles(d *api.Doc, n int, big, r float64) []api.Group {
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0.01 })
	var out []api.Group
	for i := range n {
		a := 2 * math.Pi * float64(i) / float64(n)
		er := api.V(math.Cos(a), math.Sin(a), 0)
		d.Features.Sketch3D("")
		o := d.Sketch.Point(er.ScaledBy(big))
		out = append(out, d.Features.SketchAt(o, er, api.V(0, 0, 1), ""))
		d.Sketch.Circle(er.ScaledBy(big), r)
	}
	return out
}

// A closed loft through a ring of circles is a torus: no caps, sound,
// of the volume Pappus gives as closely as eight profiles let the loft
// follow the circle round. Closing it is one undo step, saved, and
// written by a script that builds it again.
func TestLoftClosed(t *testing.T) {
	t.Parallel()
	d := api.New()
	const big, r = 20.0, 4.0
	ps := ringOfCircles(d, 8, big, r)
	l := d.Features.LoftThrough(ps, "ring")
	step(d)
	before := saved(t, d)
	l.AsLoft().SetClosed(true)
	if p := l.Problem(); p != "" {
		t.Fatal(p)
	}
	rep := l.Analyze().Check()
	want := 2 * math.Pi * math.Pi * big * r * r
	if !rep.Sound() || math.Abs(rep.Volume-want) > 0.01*want {
		t.Fatalf("%s, want volume %g", rep.String(), want)
	}
	if !l.AsLoft().Closed() {
		t.Error("Closed is false")
	}
	if n := len(l.Solid().Faces()); n != 1 {
		t.Errorf("%d faces, want the ring's one", n)
	}
	undoRedo(t, d, before)
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0 })
	back := roundTrip(t, d)
	if !back.Group("ring").AsLoft().Closed() {
		t.Error("reopened, the loft isn't closed")
	}
}

// A loft that can't close says why: two profiles; and once closed, its
// ends, turn and middle profiles can't be set, each a problem on the
// group or a refused call.
func TestLoftClosedProblems(t *testing.T) {
	t.Parallel()
	d := api.New()
	bottom, top := twoProfiles(d, 10, 4, 6)
	two := d.Features.Loft(bottom, top, "")
	if !refused(func() { two.AsLoft().SetClosed(true) }) {
		t.Error("a loft of two profiles closed")
	}
	d = api.New()
	ps := ringOfCircles(d, 4, 20, 3)
	l := d.Features.LoftThrough(ps, "")
	l.AsLoft().SetClosed(true)
	if p := l.Problem(); p != "" {
		t.Fatal(p)
	}
	if !refused(func() { l.AsLoft().SetAlongSides(true, false) }) {
		t.Error("a closed loft took a solid's sides at its ends")
	}
	if !refused(func() { l.AsLoft().SetProfileTangency(ps[1], 1) }) {
		t.Error("a closed loft was held square to a profile")
	}
	for _, c := range []struct {
		set  func()
		want string
	}{
		{func() { l.AsLoft().SetTangency(1, 0) }, "no ends"},
		{func() { l.AsLoft().SetTurn(1) }, "can't turn"},
	} {
		c.set()
		if p := l.Problem(); !strings.Contains(p, c.want) {
			t.Errorf("%q, want a problem saying %q", p, c.want)
		}
		l.AsLoft().SetTangency(0, 0)
		l.AsLoft().SetTurn(0)
	}
	l.AsLoft().SetClosed(false)
	if p := l.Problem(); p != "" || l.AsLoft().Closed() {
		t.Errorf("opened again: %q", p)
	}
}

// A loft through three circles, the middle one moved aside, held square
// to it: it passes the middle straight up, its horizontal sections the
// circle moved, so its volume is the area times the height. Setting it
// is one undo step, and saved.
func TestLoftProfileTangency(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0.001 })
	var ps []api.Group
	for i, x := range []float64{0, 8, 0} {
		z := 10 * float64(i)
		ps = append(ps, d.Features.SketchAt(above(d, z), api.V(1, 0, 0), api.V(0, 1, 0), ""))
		d.Sketch.Circle(api.V(x, 0, z), 5)
	}
	l := d.Features.LoftThrough(ps, "waist")
	step(d)
	before := saved(t, d)
	l.AsLoft().SetProfileTangency(ps[1], 1)
	if p := l.Problem(); p != "" {
		t.Fatal(p)
	}
	rep := l.Analyze().Check()
	if want := math.Pi * 25 * 20; !rep.Sound() || math.Abs(rep.Volume-want) > 0.003*want {
		t.Fatalf("%s, want volume %g", rep.String(), want)
	}
	if got := l.AsLoft().ProfileTangency(ps[1]); got != 1 {
		t.Errorf("ProfileTangency %g", got)
	}
	if got := l.AsLoft().ProfileTangency(ps[0]); got != 0 {
		t.Errorf("an end's ProfileTangency %g", got)
	}
	undoRedo(t, d, before)
	if !refused(func() { l.AsLoft().SetProfileTangency(ps[0], 1) }) {
		t.Error("an end profile took a middle tangency")
	}
	if !refused(func() { l.AsLoft().SetProfileTangency(ps[1], math.NaN()) }) {
		t.Error("a NaN tangency was taken")
	}
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0 })
	back := roundTrip(t, d)
	bl := back.Group("waist")
	if got := bl.AsLoft().ProfileTangency(bl.AsLoft().Profiles()[1]); got != 1 {
		t.Errorf("reopened, ProfileTangency %g", got)
	}
	// Free again: one surface, no tangency kept.
	l.AsLoft().SetProfileTangency(ps[1], 0)
	d.With(func(sk *sketch.Sketch) {
		if g := sk.Group(l.H); g.LoftThroughTangency != nil {
			t.Errorf("free again, tangencies %v kept", g.LoftThroughTangency)
		}
	})
}

// A loft drawn from the top of a drafted post, held by a guide, leaves
// along the post's sides and still holds the guide.
func TestLoftGuidesAlongSides(t *testing.T) {
	t.Parallel()
	d := api.New()
	bottom, top, rTop := draftedPost(d, 10)
	d.Features.Sketch3D("guide")
	tan := math.Tan(10 * math.Pi / 180)
	guide := d.Sketch.Cubic(api.V(rTop, 0, 10), api.V(rTop-3*tan, 0, 13), api.V(5, 0, 17), api.V(2, 0, 20))
	l := d.Features.Loft(bottom, top, "")
	l.AsLoft().SetGuides([]api.Entity{guide})
	step(d)
	before := saved(t, d)
	l.AsLoft().SetAlongSides(true, false)
	if p := l.Problem(); p != "" {
		t.Fatal(p)
	}
	if r := l.Analyze().Check(); !r.Sound() {
		t.Fatal(r.String())
	}
	const h = 0.005
	slope := (loftRadius(t, d, l, 10+2*h) - loftRadius(t, d, l, 10+h)) / h
	if math.Abs(slope+tan) > 0.02 {
		t.Errorf("the guided loft leaves at slope %.4f, want %.4f along the sides", slope, -tan)
	}
	// The guide's middle, at height 15 and 3.545 out, is where the
	// loft is along x.
	if r := loftRadius(t, d, l, 15); math.Abs(r-3.545) > 0.01 {
		t.Errorf("at the guide's middle the loft is %.4f out, want 3.545", r)
	}
	undoRedo(t, d, before)
}
