package sketch_test

import (
	"errors"
	"math"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// errTurned is a case whose sketch turned over without a note on its
// group.
var errTurned = errors.New("turned over, and no note says so")

// errNoted is a case that turned over, with a note on its group that
// says so.
var errNoted = errors.New("turned over, noted")

// errUnsolved is a case whose sketch did not solve after the edit, which
// no orientation is asked of: an edit past what the triangles allow.
var errUnsolved = errors.New("did not solve")

// dragHooks is the Hooks of a sketch dragged as the window drags it:
// the params it lists are held as near where they were put as the
// constraints let them.
type dragHooks struct {
	sketch.NoHooks
	dragged []core.HParam
}

func (h *dragHooks) DraggedParams() []core.HParam { return h.dragged }

// followDoc is a new drawing, in a workplane, whose sketch follows edits
// and drags or not (Sketch.NoFollow), with hooks to drag by.
func followDoc(noFollow bool) (*api.Doc, *sketch.Sketch, *dragHooks) {
	sk := sketch.NewSketch()
	sk.NewFile()
	h := &dragHooks{}
	sk.Hooks = h
	sk.NoFollow = noFollow
	return api.NewDoc(api.NewSketchHost(sk, "")), sk, h
}

// activeSolved reports whether the sketch's active group solved.
func activeSolved(sk *sketch.Sketch) bool {
	how := sk.Group(sk.ActiveGroup()).Solved.How
	return how == core.SolveOkay || how == core.SolveRedundantOkay
}

// cross2 is the z of (b-a)×(c-a), twice the signed area of triangle abc.
func cross2(a, b, c api.Vec) float64 {
	return (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
}

// dragTo drags point p to `to` in one move of the mouse, as the window
// does, and regenerates.
func dragTo(d *api.Doc, sk *sketch.Sketch, h *dragHooks, p api.Entity, to api.Vec) {
	e := sk.Entity(p.H)
	h.dragged = []core.HParam{e.Param[0], e.Param[1]}
	p.MoveTo(to)
	d.Edit.Regen()
	h.dragged = nil
}

// orientationKept is a case's result: nil where it kept its orientation, errNoted
// where it turned over and its group says so, errTurned where it turned
// over unsaid.
func orientationKept(sk *sketch.Sketch, over bool) error {
	switch {
	case !over:
		return nil
	case sk.Group(sk.ActiveGroup()).SolveNote != "":
		return errNoted
	}
	return errTurned
}

// fanCase edits one dimension of a 12-gon triangulated as a fan from its
// first corner, every side and diagonal dimensioned, its corners a
// little off a regular polygon by seed, to times its value, in one edit
// (Constraint.Set), and reports whether a triangle of the fan turned
// over.
func fanCase(noFollow bool, seed, dim, times float64) error {
	d, sk, _ := followDoc(noFollow)
	const n = 12
	pts := make([]api.Vec, n)
	for i := range n {
		r := 50 * (1 + 0.15*math.Sin(seed*float64(i*i+1)))
		a := 2 * math.Pi * float64(i) / n
		pts[i] = api.V(r*math.Cos(a), r*math.Sin(a), 0)
	}
	var dims []api.Constraint
	var corner []api.Entity
	d.Batch(func() {
		sides := d.Sketch.Polyline(true, pts...)
		for _, s := range sides {
			corner = append(corner, s.Start())
			dims = append(dims, d.Constrain.Length(s, s.Length()))
		}
		for i := 2; i < n-1; i++ {
			l := d.Sketch.Line(pts[0], pts[i])
			d.Constrain.Coincident(l.Start(), corner[0])
			d.Constrain.Coincident(l.End(), corner[i])
			dims = append(dims, d.Constrain.Length(l, l.Length()))
		}
		d.Constrain.Lock(corner[0])
	})
	d.Edit.Regen()
	if !activeSolved(sk) {
		return errors.New("the fan does not solve before the edit")
	}
	areas := func() []float64 {
		var s []float64
		for i := 1; i < n-1; i++ {
			s = append(s, cross2(corner[0].Pos(), corner[i].Pos(), corner[i+1].Pos()))
		}
		return s
	}
	before := areas()
	c := dims[int(dim)%len(dims)]
	c.Set(c.Value() * times)
	d.Edit.Regen()
	if !activeSolved(sk) {
		return errUnsolved
	}
	over := false
	for i, a := range areas() {
		over = over || a*before[i] < 0
	}
	return orientationKept(sk, over)
}

// armCase drags the hand of a two-link arm, hinged at the origin, its
// links 40 and 40·ratio long and bent by bend degrees, to a point at
// angle at and a fraction reach of its full stretch, in one move of the
// mouse, and reports whether the elbow bent the other way.
func armCase(noFollow bool, ratio, bend, at, reach float64) error {
	d, sk, h := followDoc(noFollow)
	l1, l2 := 40.0, 40*ratio
	a1 := 30 * math.Pi / 180
	b := a1 + bend*math.Pi/180
	elbow := api.V(l1*math.Cos(a1), l1*math.Sin(a1), 0)
	hand := api.V(elbow.X+l2*math.Cos(b), elbow.Y+l2*math.Sin(b), 0)
	var links []api.Entity
	d.Batch(func() {
		links = d.Sketch.Polyline(false, api.V(0, 0, 0), elbow, hand)
		d.Constrain.Lock(links[0].Start())
		d.Constrain.Length(links[0], l1)
		d.Constrain.Length(links[1], l2)
	})
	d.Edit.Regen()
	if !activeSolved(sk) {
		return errors.New("the arm does not solve before the drag")
	}
	bent := func() float64 { return cross2(links[0].Start().Pos(), links[0].End().Pos(), links[1].End().Pos()) }
	before := bent()
	r, t := reach*(l1+l2), at*math.Pi/180
	dragTo(d, sk, h, links[1].End(), api.V(r*math.Cos(t), r*math.Sin(t), 0))
	if !activeSolved(sk) {
		return errUnsolved
	}
	// An arm stretched out or folded flat, toward a point out of its
	// reach, bends to neither side.
	after := bent()
	if math.Abs(after) < 0.05*l1*l2 {
		return nil
	}
	return orientationKept(sk, after*before < 0)
}

// arcCase drags the end of an arc of radius 30 about the origin, 15° on
// from its start at 0°, to past degrees short of the start, at reach
// times the radius, in one move of the mouse, its start locked or free
// on the circle, and reports whether the arc's sweep wrapped round past
// its start.
func arcCase(noFollow bool, past, reach float64, locked bool) error {
	d, sk, h := followDoc(noFollow)
	const r = 30.0
	start := api.V(r, 0, 0)
	end := api.V(r*math.Cos(15*math.Pi/180), r*math.Sin(15*math.Pi/180), 0)
	var arc api.Entity
	d.Batch(func() {
		arc = d.Sketch.Arc(api.V(0, 0, 0), start, end)
		d.Constrain.Lock(arc.Center())
		d.Constrain.Radius(arc, r)
		if locked {
			d.Constrain.Lock(arc.Start())
		}
	})
	d.Edit.Regen()
	sweep := func() float64 {
		_, _, s := sk.MustEntity(arc.H).ArcGetAngles(sk)
		return s
	}
	if !activeSolved(sk) || sweep() > math.Pi/6 {
		return errors.New("the arc does not solve before the drag")
	}
	t := -past * math.Pi / 180
	dragTo(d, sk, h, arc.End(), api.V(reach*r*math.Cos(t), reach*r*math.Sin(t), 0))
	if !activeSolved(sk) {
		return errUnsolved
	}
	return orientationKept(sk, sweep() > math.Pi)
}

// The grids of the cases SolidCraft measured its sketch solver on (issue
// #203): a fan-triangulated 12-gon with each dimension edited to 0.5 to
// 2.5 times its value, a two-link arm whose hand is dragged far in one
// move, an arc whose end is dragged past its start.
var (
	fanGrid = sweep.Grid{Axes: []sweep.Axis{
		sweep.Floats("seed", 1, 2, 3),
		sweep.Floats("dim", 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20),
		sweep.Floats("times", 0.5, 0.75, 1.5, 2, 2.5),
	}, Sample: 60, Parallel: true}
	armGrid = sweep.Grid{Axes: []sweep.Axis{
		sweep.Floats("ratio", 0.5, 0.75, 1),
		sweep.Floats("bend", -60, 60),
		sweep.Floats("at", 0, 30, 60, 90, 120, 150, 180, 210, 240, 270, 300, 330),
		sweep.Floats("reach", 0.3, 0.5, 0.7, 0.9),
	}, Sample: 60, Parallel: true}
	arcGrid = sweep.Grid{Axes: []sweep.Axis{
		sweep.Floats("past", 5, 15, 30, 60, 90),
		sweep.Floats("reach", 0.8, 1, 1.5),
		sweep.Bools("locked"),
	}, Parallel: true}
)

// flipCases are the three grids and how each case is run.
var flipCases = []struct {
	name string
	grid sweep.Grid
	run  func(noFollow bool, p sweep.Point) error
}{
	{"fan", fanGrid, func(noFollow bool, p sweep.Point) error {
		return fanCase(noFollow, p.Float("seed"), p.Float("dim"), p.Float("times"))
	}},
	{"arm", armGrid, func(noFollow bool, p sweep.Point) error {
		return armCase(noFollow, p.Float("ratio"), p.Float("bend"), p.Float("at"), p.Float("reach"))
	}},
	{"arc", arcGrid, func(noFollow bool, p sweep.Point) error {
		return arcCase(noFollow, p.Float("past"), p.Float("reach"), p.Bool("locked"))
	}},
}

// A sketch keeps its orientation through a large dimension edit and a
// long drag: no triangle of the fan turns over, no elbow bends the other
// way, and an arc's end dragged past its start takes the start along
// rather than wrap the arc round; where nothing can keep it, an arc
// whose start is locked, the group says so. A case that does not solve
// is no failure.
func TestFollowKeepsOrientation(t *testing.T) {
	t.Parallel()
	for _, fc := range flipCases {
		t.Run(fc.name, func(t *testing.T) {
			t.Parallel()
			var noted atomic.Int64
			r := fc.grid.Run(func(p sweep.Point) error {
				err := fc.run(false, p)
				switch {
				case errors.Is(err, errNoted):
					noted.Add(1)
					if fc.name != "arc" || !p.Bool("locked") {
						return err
					}
					return nil
				case errors.Is(err, errUnsolved):
					return nil
				}
				return err
			})
			r.Require(t)
			t.Logf("%d noted", noted.Load())
		})
	}
}

// TestFollowCounts counts, over the whole of each grid, the cases that
// turn over solved in one step, as before continuation.go
// (Sketch.NoFollow), and following the edit or drag, for
// doc/dev-notes/solver-literature.md. It runs in a full sweep only
// (make test-sweeps).
func TestFollowCounts(t *testing.T) {
	if !sweep.Full() {
		t.Skip("counts the flips over the whole grids; TELESAPR_SWEEP=1 runs it")
	}
	t.Parallel()
	for _, fc := range flipCases {
		for _, noFollow := range []bool{true, false} {
			var over, noted, unsolved atomic.Int64
			r := fc.grid.RunAll(func(p sweep.Point) error {
				switch err := fc.run(noFollow, p); {
				case errors.Is(err, errTurned):
					over.Add(1)
				case errors.Is(err, errNoted):
					over.Add(1)
					noted.Add(1)
				case errors.Is(err, errUnsolved):
					unsolved.Add(1)
				case err != nil:
					return err
				}
				return nil
			})
			for _, f := range r.Failed {
				t.Errorf("%s %s: %v", fc.name, f.Point.Label(), f.Err)
			}
			t.Logf("%s, following %v: %d of %d turned over (%d of them noted), %d did not solve",
				fc.name, !noFollow, over.Load(), r.Runs, noted.Load(), unsolved.Load())
		}
	}
}

// A regeneration with nothing edited or dragged never follows: every
// fixture, opened and regenerated twice, solves in one step at each
// solve, so that no golden moves for continuation.go.
func TestRegenerateNeverFollows(t *testing.T) {
	t.Parallel()
	all, err := fixtures.Walk(".tlspr")
	if err != nil {
		t.Fatal(err)
	}
	// The thread variants cost the most, and differ only in their
	// thread's options; under -short every tenth model.
	files := sweep.Without(all, fixtures.ThreadVariant)
	for i, f := range files {
		if testing.Short() && i%10 != 0 {
			continue
		}
		t.Run(f, func(t *testing.T) {
			t.Parallel()
			doc, err := tlspr.LoadFile(fixtures.Path(f))
			if err != nil {
				t.Skipf("does not load: %v", err)
			}
			sk := doc.Sketch
			sk.GenerateAll()
			sk.GenerateAllTyped(sketch.GenerateAll)
			if sk.Followed != 0 || sk.FollowSteps != 0 {
				t.Errorf("regeneration followed %d solves in %d steps", sk.Followed, sk.FollowSteps)
			}
			for _, hg := range sk.GroupOrder {
				if n := sk.Group(hg).SolveNote; n != "" {
					t.Errorf("group %08x: %s", hg.V, n)
				}
			}
		})
	}
}

// An edit is followed once, at the solve after it, and undo puts the
// sketch back where it was, following nothing: the regenerations after
// solve in one step.
func TestFollowOnceAndUndo(t *testing.T) {
	t.Parallel()
	d, sk, _ := followDoc(false)
	var side api.Constraint
	var pts []api.Entity
	d.Batch(func() {
		ls := d.Sketch.Polyline(true, api.V(0, 0, 0), api.V(40, 0, 0), api.V(15, 25, 0))
		for i, l := range ls {
			c := d.Constrain.Length(l, l.Length())
			if i == 0 {
				side = c
			}
			pts = append(pts, l.Start())
		}
		d.Constrain.Lock(pts[0])
		d.Constrain.Horizontal(ls[0])
	})
	d.Edit.Regen()
	was := pts[2].Pos()
	// A bare sketch's host remembers the first change of a script's step
	// only; this test's steps are its own.
	sk.UndoRemember()
	side.Set(70)
	d.Edit.Regen()
	if sk.Followed != 1 || sk.FollowSteps == 0 {
		t.Fatalf("the edit to 70 followed %d solves in %d steps, want 1 in some", sk.Followed, sk.FollowSteps)
	}
	if cross2(pts[0].Pos(), pts[1].Pos(), pts[2].Pos()) <= 0 {
		t.Error("the triangle turned over")
	}
	steps := sk.FollowSteps
	d.Edit.Regen()
	if !sk.UndoUndo() {
		t.Fatal("no undo")
	}
	if sk.Followed != 1 || sk.FollowSteps != steps {
		t.Errorf("regenerating and undoing followed: %d solves in %d steps", sk.Followed, sk.FollowSteps)
	}
	if now := pts[2].Pos(); now != was {
		t.Errorf("undo put the apex at %v, was %v", now, was)
	}
	// A small edit is one solve, checked but not stepped.
	side.Set(42)
	d.Edit.Regen()
	if sk.Followed != 2 || sk.FollowSteps != steps {
		t.Errorf("the edit to 42 followed %d solves in %d steps, want 2 in %d", sk.Followed, sk.FollowSteps, steps)
	}
}

// The note on a group that a drag turned over names what turned, and a
// drag after that turns nothing over clears it.
func TestFollowNote(t *testing.T) {
	t.Parallel()
	d, sk, h := followDoc(false)
	const r = 30.0
	var arc api.Entity
	d.Batch(func() {
		arc = d.Sketch.Arc(api.V(0, 0, 0), api.V(r, 0, 0), api.V(r*math.Cos(0.25), r*math.Sin(0.25), 0))
		d.Constrain.Lock(arc.Center())
		d.Constrain.Radius(arc, r)
		d.Constrain.Lock(arc.Start())
	})
	d.Edit.Regen()
	dragTo(d, sk, h, arc.End(), api.V(r*math.Cos(-0.5), r*math.Sin(-0.5), 0))
	note := sk.Group(sk.ActiveGroup()).SolveNote
	if !strings.Contains(note, "arc-of-circle") {
		t.Errorf("note %q names no arc", note)
	}
	gs := d.Groups()
	if got := gs[len(gs)-1].Note(); !strings.Contains(got, note) {
		t.Errorf("the script's Group.Note is %q, want it to hold %q", got, note)
	}
	dragTo(d, sk, h, arc.End(), api.V(r*math.Cos(-0.6), r*math.Sin(-0.6), 0))
	if n := sk.Group(sk.ActiveGroup()).SolveNote; n != "" {
		t.Errorf("a drag that turned nothing over left the note %q", n)
	}
}
