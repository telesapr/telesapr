package ui

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/res"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// labelScreenBox is the box the window draws c's label in, in pixels on
// the screen: its lower left and upper right corners.
func labelScreenBox(ss *Application, c *core.Constraint) (lo, hi geom.Point2d) {
	gw := ss.GW
	camera := gw.Camera()
	corner := gw.ProjectPoint(gw.constraintLabelPos(c, &camera))
	th := ss.styleTextHeight(constraintStyle(c))
	w := res.BuiltinVectorFont().Width(th, ss.constraintLabel(c))
	return corner, geom.Point2d{U: corner.U + w, V: corner.V + th}
}

func labelsOverlap(ss *Application, a, b *core.Constraint) bool {
	alo, ahi := labelScreenBox(ss, a)
	blo, bhi := labelScreenBox(ss, b)
	return math.Max(alo.U, blo.U) < math.Min(ahi.U, bhi.U) &&
		math.Max(alo.V, blo.V) < math.Min(ahi.V, bhi.V)
}

func requireLabelsApart(t *testing.T, ss *Application) {
	t.Helper()
	var labelled []*core.Constraint
	for i := range ss.SK.Constraints {
		if c := &ss.SK.Constraints[i]; c.HasLabel() {
			labelled = append(labelled, c)
		}
	}
	if len(labelled) < 2 {
		t.Fatalf("%d labelled constraints, want two", len(labelled))
	}
	for i, a := range labelled {
		for _, b := range labelled[i+1:] {
			if labelsOverlap(ss, a, b) {
				t.Errorf("the labels %q and %q overlap", ss.constraintLabel(a), ss.constraintLabel(b))
			}
		}
	}
}

// A minimum and a maximum on one line segment, made from the Constrain
// menu, put their labels apart, as a script that makes them does.
func TestBoundLabelsApart(t *testing.T) {
	t.Parallel()
	build := func(doc *api.Doc) []api.Entity {
		return []api.Entity{doc.Sketch.Line(api.V(0, 0, 0), api.V(40, 0, 0))}
	}
	win, _ := newTestApp(t)
	line := build(api.NewDoc(win.ScriptHost()))[0]
	for _, cmd := range []Command{cmdMinDistance, cmdMaxDistance} {
		win.GW.ClearSelection()
		win.GW.MakeSelectedEntity(line.H)
		win.MenuConstrain(cmd)
	}
	requireLabelsApart(t, win)

	scr, _ := newTestApp(t)
	sdoc := api.NewDoc(scr.ScriptHost())
	sline := build(sdoc)[0]
	sdoc.Constrain.MinDistance(sline, api.Entity{}, 40)
	sdoc.Constrain.MaxDistance(sline, api.Entity{}, 40)
	requireLabelsApart(t, scr)
	for i := range win.SK.Constraints {
		w, s := win.SK.Constraints[i].Disp.Offset, scr.SK.Constraints[i].Disp.Offset
		if !w.Equals(s) {
			t.Errorf("label %d at %v in the window and %v from a script", i, w, s)
		}
	}
}

// A distance from a circle's centre to a line, added after the circle's
// diameter, keeps its label off the diameter's.
func TestDistanceLabelClearOfDiameter(t *testing.T) {
	t.Parallel()
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	circle := doc.Sketch.Circle(api.V(0, 0, 0), 3)
	line := doc.Sketch.Line(api.V(-20, -2, 0), api.V(20, -2, 0))
	doc.Constrain.Diameter(circle, 6)
	ss.GW.ClearSelection()
	ss.GW.MakeSelectedEntity(circle.Center().H)
	ss.GW.MakeSelectedEntity(line.H)
	ss.MenuConstrain(cmdDistanceDia)
	requireLabelsApart(t, ss)
}
