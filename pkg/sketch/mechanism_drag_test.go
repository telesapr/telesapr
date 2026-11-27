package sketch_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// planPulls is how many pulls of d's mechanisms its plan took.
func planPulls(d *api.Doc) int {
	n := 0
	d.With(func(sk *sketch.Sketch) { n = sk.PlanPulls })
	return n
}

// nearAsOld reports whether a point pulled toward to by the plan, at,
// is where the null space's pull put it, was, where that reached to;
// and no farther from to than was, give or take 1e-4, where it didn't,
// the target off the point's path: each stops where its steps stop
// gaining, along the path.
func nearAsOld(at, was, to api.Vec) bool {
	if was.Minus(to).Magnitude() < 1e-3 {
		return at.Minus(was).Magnitude() < 1e-4
	}
	return at.Minus(to).Magnitude() <= was.Minus(to).Magnitude()+1e-4
}

// Each example mechanism dragged in steps (dragCases), with its plan and
// along the null space as before: the plan takes every step, and puts
// the dragged point where the null space's pull and solve do, or
// nearer, on its mates, every part solved.
func TestDragByPlanAgrees(t *testing.T) {
	t.Parallel()
	for _, tc := range dragCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d, pt, ts := tc.build(t)
			old, oldPt, _ := tc.build(t)
			old.With(func(sk *sketch.Sketch) { sk.NoMechanismPlan = true })
			for i, to := range ts {
				pt.MoveTo(to)
				oldPt.MoveTo(to)
				if !nearAsOld(pt.Pos(), oldPt.Pos(), to) {
					t.Errorf("step %d to %v: by the plan at %v, along the null space at %v", i, to, pt.Pos(), oldPt.Pos())
				}
			}
			if n := planPulls(d); n != len(ts) {
				t.Errorf("the plan took %d of %d steps", n, len(ts))
			}
			if n := planPulls(old); n != 0 {
				t.Errorf("with the plan off it took %d steps", n)
			}
			for _, g := range d.Groups() {
				if !g.Solved() {
					t.Errorf("%s not solved", g.Name())
				}
			}
		})
	}
}

// ballOnFace is the hinged bar's ground and bar held by a ball at the
// bar's origin's top corner on the ground's far one, the bar's far top
// corner on the ground's top face: the bar turns about the ball, its
// far corner on a circle round it in the top's plane, and spins about
// the line from the one corner to the other. It returns the far corner
// and the ball's centre.
func ballOnFace(t testing.TB) (d *api.Doc, bar api.Group, far api.Entity, ball api.Vec) {
	t.Helper()
	d, ground, bar := twoBars(t)
	ug, ub := uprights(t, ground), uprights(t, bar)
	d.Assembly.Ball(ug[60].Start(), ub[0].Start())
	far = ub[60].Start()
	_, top := ends(ground)
	d.Constrain.On(far, top)
	if !bar.Solved() || bar.Dof() != 2 {
		t.Fatalf("ball on a face: solved %v, %d DOF", bar.Solved(), bar.Dof())
	}
	return d, bar, far, ug[60].Start().Pos()
}

// A part held by points alone, the bar on its ball with its far corner
// on a face, is turned by the plan: its far corner pulled round goes
// round its circle, 60 from the ball and in the face's plane, as near
// the pull as the null space's pull and solve put it.
func TestDragBallOnFaceByPlan(t *testing.T) {
	t.Parallel()
	d, bar, far, ball := ballOnFace(t)
	old, _, oldFar, _ := ballOnFace(t)
	old.With(func(sk *sketch.Sketch) { sk.NoMechanismPlan = true })
	z := far.Pos().Z
	for k := 1; k <= 6; k++ {
		to := turned(far.Pos(), ball, 10)
		to.Z += 3 // off the face: the corner stays on it
		far.MoveTo(to)
		oldFar.MoveTo(to)
		p := far.Pos()
		if r := p.Minus(ball).Magnitude(); math.Abs(r-60) > 1e-6 || math.Abs(p.Z-z) > 1e-6 || !bar.Solved() {
			t.Errorf("pull %d: the corner at %v, %g from the ball, %g off the face; solved %v", k, p, r, p.Z-z, bar.Solved())
		}
		if !nearAsOld(p, oldFar.Pos(), to) {
			t.Errorf("pull %d to %v: by the plan at %v, along the null space at %v", k, to, p, oldFar.Pos())
		}
	}
	if got := angleAbout(far.Pos(), ball); math.Abs(got-60) > 0.5 {
		t.Errorf("six pulls of 10°: the corner at %.3f° round the ball, want 60", got)
	}
	if n := planPulls(d); n != 6 {
		t.Errorf("the plan took %d of 6 pulls", n)
	}
}

// The plan places a part held by points alone, which it kept turned as
// it was before: the bar on its ball, its far corner put on a face it
// is 5 above, is turned down onto it by the plan, not the solve.
func TestPlanTurnsBall(t *testing.T) {
	t.Parallel()
	d, ground, bar := twoBars(t)
	ug, ub := uprights(t, ground), uprights(t, bar)
	d.Assembly.Ball(ug[60].Start(), ub[0].Start())
	far := ub[60].Start()
	_, top := ends(ground)
	// The far corner lifted 5 off the top's plane, turned about the ball.
	ball := ug[60].Start().Pos()
	far.MoveTo(ball.Plus(api.V(math.Sqrt(3600-25), 0, 5)))
	if math.Abs(far.Pos().Z-ball.Z-5) > 1e-6 {
		t.Fatalf("the corner at %v, not lifted", far.Pos())
	}
	var placed int
	d.With(func(sk *sketch.Sketch) { placed = sk.PlanPlaced })
	d.Constrain.On(far, top)
	p := far.Pos() // a read regenerates
	d.With(func(sk *sketch.Sketch) { placed = sk.PlanPlaced - placed })
	if math.Abs(p.Z-ball.Z) > 1e-6 || math.Abs(p.Minus(ball).Magnitude()-60) > 1e-6 || !bar.Solved() || placed == 0 {
		t.Errorf("on the face: the corner at %v, ball at %v, solved %v, placed by the plan %d times", p, ball, bar.Solved(), placed)
	}
}

// Where the plan doesn't cover a drag, the null space takes it as
// before: a bound that is no drive's (TestPullStopsAtBound's), and a
// part no mate holds (TestPullFreePartSlides').
func TestDragFallsBackFromPlan(t *testing.T) {
	t.Parallel()
	d, ground, _, far, pivot := hingedBar(t)
	d.Constrain.Distance(far, uprights(t, ground)[0].Start(), 100).SetLimit(api.Minimum)
	far.MoveTo(turned(far.Pos(), pivot, 30))
	if n := planPulls(d); n != 0 {
		t.Errorf("with a bound: the plan took %d pulls", n)
	}
	free, _, bar := twoBars(t)
	c := uprights(t, bar)[60].Start()
	c.MoveTo(c.Pos().Plus(api.V(0, -40, 0)))
	if n := planPulls(free); n != 0 {
		t.Errorf("a part no mate holds: the plan took %d pulls", n)
	}
}
