package ui

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// requireSameConstraint builds a model twice with build, selects what
// pick names in the window and runs the Constrain menu command, and
// has a script add the same constraint with script; the saved files
// must be the same, which they are only if both reach the same
// Sketch call.
func requireSameConstraint(t *testing.T, cmd Command,
	build func(doc *api.Doc) []api.Entity, pickFrom func(es []api.Entity) []api.Entity,
	script func(doc *api.Doc, es []api.Entity)) {
	t.Helper()
	win, _ := newTestApp(t)
	wdoc := api.NewDoc(win.ScriptHost())
	wes := build(wdoc)
	win.GW.ClearSelection()
	for _, e := range pickFrom(wes) {
		win.GW.MakeSelectedEntity(e.H)
	}
	win.MenuConstrain(cmd)

	scr, _ := newTestApp(t)
	sdoc := api.NewDoc(scr.ScriptHost())
	script(sdoc, build(sdoc))

	got := string(tlspr.Encode(win.SK, tlspr.Options{}))
	want := string(tlspr.Encode(scr.SK, tlspr.Options{}))
	if got != want {
		t.Errorf("the window and the script save different files, at %s", firstDiffLine(got, want))
	}
	if len(win.SK.Constraints) == 0 {
		t.Errorf("the window added no constraint")
	}
}

func all(es []api.Entity) []api.Entity { return es }

// A line segment and an arc that share no end are made tangent at the
// end point selected with them, in the window, and the end point given
// to Tangent in a script.
func TestTangentAtEndMatchesScript(t *testing.T) {
	build := func(doc *api.Doc) []api.Entity {
		line := doc.Sketch.Line(api.V(0, 0, 0), api.V(10, 0, 0))
		arc := doc.Sketch.Arc(api.V(20, 0, 0), api.V(30, 0, 0), api.V(20, 10, 0))
		return []api.Entity{line, arc, arc.End()}
	}
	requireSameConstraint(t, cmdParallel, build, all, func(doc *api.Doc, es []api.Entity) {
		doc.Constrain.Tangent(es[0], es[1], es[2])
	})
}

// Two arcs with no shared end are tangent at the two end points
// selected, and the two a script gives.
func TestTangentCurvesAtEndsMatchesScript(t *testing.T) {
	build := func(doc *api.Doc) []api.Entity {
		a := doc.Sketch.Arc(api.V(0, 0, 0), api.V(10, 0, 0), api.V(0, 10, 0))
		b := doc.Sketch.Arc(api.V(30, 0, 0), api.V(40, 0, 0), api.V(30, 10, 0))
		return []api.Entity{a, b, a.End(), b.Start()}
	}
	requireSameConstraint(t, cmdParallel, build, all, func(doc *api.Doc, es []api.Entity) {
		doc.Constrain.Tangent(es[0], es[1], es[2], es[3])
	})
}

// Two faces are made parallel and perpendicular alike by the window
// and by a script.
func TestFacesMatchScript(t *testing.T) {
	build := func(doc *api.Doc) []api.Entity {
		doc.Sketch.Rect(api.V(0, 0, 0), api.V(10, 10, 0))
		ext := doc.Features.Extrude(5, "")
		faces := ext.Faces()
		doc.Features.SketchIn(doc.Workplane("XY"), "")
		return faces
	}
	requireSameConstraint(t, cmdParallel, build,
		func(es []api.Entity) []api.Entity { return es[:2] },
		func(doc *api.Doc, es []api.Entity) { doc.Constrain.Parallel(es[0], es[1]) })
	requireSameConstraint(t, cmdPerpendicular, build,
		func(es []api.Entity) []api.Entity { return []api.Entity{es[0], es[2]} },
		func(doc *api.Doc, es []api.Entity) { doc.Constrain.Perpendicular(es[0], es[2]) })
}

// A line segment's midpoint in a workplane, and the point at a middle
// that drops its on-line constraint, are the same in both.
func TestMidpointMatchesScript(t *testing.T) {
	build := func(doc *api.Doc) []api.Entity {
		line := doc.Sketch.Line(api.V(0, 0, 0), api.V(10, 0, 0))
		return []api.Entity{line, doc.Workplane("XY")}
	}
	requireSameConstraint(t, cmdAtMidpoint, build, all,
		func(doc *api.Doc, es []api.Entity) { doc.Constrain.Midpoint(es[1], es[0]) })
}

// A whole line segment is symmetric about the workplane's axis and
// loses its horizontal constraint, in the window and in a script.
func TestSymmetricLineMatchesScript(t *testing.T) {
	build := func(doc *api.Doc) []api.Entity {
		doc.Features.SketchIn(doc.Workplane("XY"), "")
		line := doc.Sketch.Line(api.V(-5, 0, 0), api.V(5, 0, 0))
		doc.Constrain.Horizontal(line)
		return []api.Entity{line}
	}
	requireSameConstraint(t, cmdSymmetric, build, all,
		func(doc *api.Doc, es []api.Entity) { doc.Constrain.Symmetric(es[0], api.Entity{}, api.Entity{}) })
}

// A comment on a selected point reads the same in both.
func TestCommentOnPointMatchesScript(t *testing.T) {
	build := func(doc *api.Doc) []api.Entity {
		return []api.Entity{doc.Sketch.Point(api.V(1, 2, 0))}
	}
	requireSameConstraint(t, cmdComment, build, all,
		func(doc *api.Doc, es []api.Entity) {
			doc.Constrain.CommentOn(es[0], "NEW COMMENT -- DOUBLE-CLICK TO EDIT")
		})
}
