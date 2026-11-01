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

// requireLabelsClearOfPoints fails where a label the window draws sits
// on a point it draws: a point is a square seven pixels across.
func requireLabelsClearOfPoints(t *testing.T, ss *Application) {
	t.Helper()
	sk := ss.SK
	const half = 3.5
	for i := range sk.Constraints {
		c := &sk.Constraints[i]
		if !c.HasLabel() {
			continue
		}
		lo, hi := labelScreenBox(ss, c)
		for j := range sk.Entities {
			e := &sk.Entities[j]
			if !e.IsPoint() || !sk.EntityIsVisible(e) {
				continue
			}
			p := ss.GW.ProjectPoint(e.PointGetNum(sk))
			if lo.U < p.U+half && p.U-half < hi.U && lo.V < p.V+half && p.V-half < hi.V {
				t.Errorf("the label %q is on the point %x", ss.constraintLabel(c), e.H.V)
			}
		}
	}
}

// requireSameOffsets fails where the window and a script put a label
// in different places.
func requireSameOffsets(t *testing.T, win, scr *Application) {
	t.Helper()
	if len(win.SK.Constraints) != len(scr.SK.Constraints) {
		t.Fatalf("%d constraints in the window and %d from a script", len(win.SK.Constraints), len(scr.SK.Constraints))
	}
	for i := range win.SK.Constraints {
		w, s := win.SK.Constraints[i].Disp.Offset, scr.SK.Constraints[i].Disp.Offset
		if !w.Equals(s) {
			t.Errorf("label %d at %v in the window and %v from a script", i, w, s)
		}
	}
}

// A minimum and a maximum angle at one corner, made from the Constrain
// menu, put their labels apart and off the corner, as a script that
// makes them does.
func TestAngleLabelsApart(t *testing.T) {
	t.Parallel()
	build := func(doc *api.Doc) (a, b api.Entity) {
		return doc.Sketch.Line(api.V(0, 0, 0), api.V(40, 0, 0)), doc.Sketch.Line(api.V(0, 0, 0), api.V(30, 30, 0))
	}
	win, _ := newTestApp(t)
	la, lb := build(api.NewDoc(win.ScriptHost()))
	for _, cmd := range []Command{cmdMinDistance, cmdMaxDistance} {
		win.GW.ClearSelection()
		win.GW.MakeSelectedEntity(la.H)
		win.GW.MakeSelectedEntity(lb.H)
		win.MenuConstrain(cmd)
	}
	requireLabelsApart(t, win)
	requireLabelsClearOfPoints(t, win)

	scr, _ := newTestApp(t)
	sdoc := api.NewDoc(scr.ScriptHost())
	sa, sb := build(sdoc)
	sdoc.Constrain.Angle(sa, sb, 45).SetLimit(api.Minimum)
	sdoc.Constrain.Angle(sa, sb, 45).SetLimit(api.Maximum)
	requireLabelsApart(t, scr)
	requireSameOffsets(t, win, scr)
}

// A ratio and a difference of the same two line segments put their
// labels apart, from the menu as from a script.
func TestRatioLabelsApart(t *testing.T) {
	t.Parallel()
	build := func(doc *api.Doc) (a, b api.Entity) {
		return doc.Sketch.Line(api.V(0, 0, 0), api.V(40, 0, 0)), doc.Sketch.Line(api.V(0, 10, 0), api.V(20, 10, 0))
	}
	win, _ := newTestApp(t)
	la, lb := build(api.NewDoc(win.ScriptHost()))
	for _, cmd := range []Command{cmdRatio, cmdDifference} {
		win.GW.ClearSelection()
		win.GW.MakeSelectedEntity(la.H)
		win.GW.MakeSelectedEntity(lb.H)
		win.MenuConstrain(cmd)
	}
	requireLabelsApart(t, win)

	scr, _ := newTestApp(t)
	sdoc := api.NewDoc(scr.ScriptHost())
	sa, sb := build(sdoc)
	sdoc.Constrain.Ratio(sa, sb, 2)
	sdoc.Constrain.Difference(sa, sb, 20)
	requireLabelsApart(t, scr)
	requireSameOffsets(t, win, scr)
}

// A circle's diameter keeps its label off the circle's centre, from the
// menu as from a script.
func TestDiameterLabelClearOfCentre(t *testing.T) {
	t.Parallel()
	win, _ := newTestApp(t)
	circle := api.NewDoc(win.ScriptHost()).Sketch.Circle(api.V(0, 0, 0), 5)
	win.GW.ClearSelection()
	win.GW.MakeSelectedEntity(circle.H)
	win.MenuConstrain(cmdDistanceDia)
	requireLabelsClearOfPoints(t, win)

	scr, _ := newTestApp(t)
	sdoc := api.NewDoc(scr.ScriptHost())
	sdoc.Constrain.Diameter(sdoc.Sketch.Circle(api.V(0, 0, 0), 5), 10)
	requireLabelsClearOfPoints(t, scr)
	requireSameOffsets(t, win, scr)
}
