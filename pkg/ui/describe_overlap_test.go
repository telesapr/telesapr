package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// alsoHere is what the property browser lists under ALSO HERE.
func alsoHere(ss *Application) []string {
	tw := ss.TW
	var rows []string
	in := false
	for r := range tw.rows {
		row := strings.TrimSpace(string(tw.lines[r].text[:]))
		switch {
		case row == "ALSO HERE":
			in = true
		case in && strings.HasPrefix(string(tw.lines[r].text[:]), "   "):
			rows = append(rows, row)
		case in && row != "":
			return rows
		}
	}
	return rows
}

// ALSO HERE lists what Entity.Overlap gives a script, in its order, and
// a click on a row selects that entity, as Edit.Select does.
func TestAlsoHereMatchesScript(t *testing.T) {
	t.Parallel()
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	a := doc.Sketch.Line(api.V(0, 0, 0), api.V(20, 0, 0))
	b := doc.Sketch.Line(api.V(20, 0, 0), api.V(20, 10, 0))
	arc := doc.Sketch.Arc(api.V(30, 0, 0), api.V(20, 0, 0), api.V(30, -10, 0))
	across := doc.Sketch.Line(api.V(10, -5, 0), api.V(10, 5, 0))
	mid := doc.Sketch.Point(api.V(10, 0, 0))

	for _, tc := range []struct {
		name  string
		at    api.Entity
		want  []string
		click int
		then  api.Entity
	}{
		{"corner", a.End(), []string{
			"start of " + b.Request().String(),
			"start of " + arc.Request().String(),
		}, 1, arc.Start()},
		{"crossing", mid, []string{
			a.Request().String() + ", through it",
			across.Request().String() + ", through it",
		}, 1, across},
		{"alone", across.End(), nil, -1, api.Entity{}},
	} {
		ss.GW.ClearSelection()
		ss.GW.MakeSelectedEntity(tc.at.H)
		ss.TW.Show()
		got := alsoHere(ss)
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("%s: ALSO HERE %q, want %q", tc.name, got, tc.want)
			continue
		}
		points, curves := tc.at.Overlap()
		if n := len(points) + len(curves); n != len(got) {
			t.Errorf("%s: %d rows, the script finds %d", tc.name, len(got), n)
		}
		if tc.click < 0 {
			continue
		}
		clickText(t, ss, tc.want[tc.click])
		if sel := doc.Edit.Selection(); len(sel) != 1 || sel[0].H != tc.then.H {
			t.Errorf("%s: clicking %q selects %v, want %v", tc.name, tc.want[tc.click], sel, tc.then)
		}
	}
}

// A hidden group's entities are no rows: the drawing doesn't show
// them. The script's Overlap keeps in step.
func TestAlsoHereHiddenGroup(t *testing.T) {
	t.Parallel()
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	doc.Sketch.Line(api.V(0, 0, 0), api.V(20, 0, 0))
	first := doc.Active()
	doc.Features.SketchIn(doc.Sketch.ActiveWorkplane(), "second")
	b := doc.Sketch.Line(api.V(20, 0, 0), api.V(20, 10, 0))

	ss.GW.ClearSelection()
	ss.GW.MakeSelectedEntity(b.Start().H)
	ss.TW.Show()
	if got := alsoHere(ss); len(got) != 1 {
		t.Fatalf("first group shown: %q", got)
	}
	first.SetVisible(false)
	ss.TW.Show()
	got := alsoHere(ss)
	points, _ := b.Start().Overlap()
	if len(got) != 0 || len(points) != 0 {
		t.Errorf("first group hidden: ALSO HERE %q, the script finds %v", got, points)
	}
}

// The corner of an extruded rectangle is copied to the extrusion with
// its lines, and the group's faces carry it too. The copy is named by
// its line, not by a face.
func TestAlsoHereExtrudedCorner(t *testing.T) {
	t.Parallel()
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	doc.Sketch.Rect(api.V(0, 0, 0), api.V(20, 10, 0))
	doc.Features.Extrude(10, "block")
	var corner api.Entity
	for _, e := range doc.Active().Entities() {
		if e.Kind() == "point" && e.Pos() == api.V(20, 10, 0) {
			corner = e
			break
		}
	}
	if corner.H.V == 0 {
		t.Fatal("no copied corner at (20, 10, 0)")
	}
	ss.GW.ClearSelection()
	ss.GW.MakeSelectedEntity(corner.H)
	ss.TW.Show()
	got := alsoHere(ss)
	if len(got) != 3 || slices.ContainsFunc(got, func(row string) bool { return strings.HasPrefix(row, "point of ") }) {
		t.Errorf("ALSO HERE %q, want the sketch's corner and the end of the other copied line", got)
	}
}
