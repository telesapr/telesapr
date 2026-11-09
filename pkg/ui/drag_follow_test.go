package ui

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// armApp is a window showing a two-link arm, hinged at the origin, its
// links 40 and 20 long, its elbow at 30° and bent back by 60°.
func armApp(t *testing.T) (*Application, []api.Entity) {
	t.Helper()
	ss := New(headless.New())
	ss.Init()
	doc := api.NewDoc(ss.ScriptHost())
	elbow := api.V(40*math.Cos(math.Pi/6), 40*math.Sin(math.Pi/6), 0)
	hand := api.V(elbow.X+20*math.Cos(-math.Pi/6), elbow.Y+20*math.Sin(-math.Pi/6), 0)
	var links []api.Entity
	doc.Batch(func() {
		links = doc.Sketch.Polyline(false, api.V(0, 0, 0), elbow, hand)
		doc.Constrain.Lock(links[0].Start())
		doc.Constrain.Length(links[0], 40)
		doc.Constrain.Length(links[1], 20)
	})
	doc.Edit.Regen()
	w := ss.GW.window.(*headless.Window)
	w.Width, w.Height = 1200, 900
	ss.GW.ZoomToFit(false, false)
	return ss, links
}

// bend is twice the signed area of the arm's base, elbow and hand: its
// sign is the side the elbow is bent to.
func bend(links []api.Entity) float64 {
	a, b, c := links[0].Start().Pos(), links[0].End().Pos(), links[1].End().Pos()
	return (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
}

// The window's drag reaches the sketch's drag (Sketch.MovePointTo): the
// arm's hand dragged across to the far side in one move of the mouse is
// followed there in steps, and the elbow stays bent the way it was. In
// one step, as before, it bends the other way.
func TestDragKeepsElbow(t *testing.T) {
	t.Parallel()
	for _, noFollow := range []bool{true, false} {
		ss, links := armApp(t)
		ss.SK.NoFollow = noFollow
		before := bend(links)
		hand := links[1].End()
		q := ss.GW.ProjectPoint(hand.Pos())
		to := ss.GW.ProjectPoint(api.V(42*math.Cos(5*math.Pi/6), 42*math.Sin(5*math.Pi/6), 0))
		startDrag(ss, hand.H, q)
		ss.GW.dragPoints(to.U, to.V, false, false)
		ss.Refresh()
		ss.GW.MouseLeftUp(to.U, to.V, false, false)
		kept := bend(links)*before > 0
		switch {
		case noFollow && kept:
			t.Error("in one step the elbow kept its side: the drag shows nothing")
		case !noFollow && !kept:
			t.Errorf("the elbow bent the other way (%g, was %g)", bend(links), before)
		case !noFollow && ss.SK.Followed == 0:
			t.Error("the drag was not followed")
		}
	}
}

// The on-screen editor of a dimension reaches the sketch's edit
// (Sketch.SetConstraintText): a link made twice as long is followed
// there in steps.
func TestEditDimensionFollows(t *testing.T) {
	t.Parallel()
	ss, links := armApp(t)
	c := ss.SK.Constraints[len(ss.SK.Constraints)-1].H
	ss.GW.EditConstraint(c)
	ss.GW.EditControlDone("40")
	ss.Refresh()
	if ss.SK.Followed != 1 || ss.SK.FollowSteps == 0 {
		t.Errorf("the edit followed %d solves in %d steps, want 1 in some", ss.SK.Followed, ss.SK.FollowSteps)
	}
	if l := links[1].Length(); math.Abs(l-40) > 1e-6 {
		t.Errorf("the link is %g long, want 40", l)
	}
}
