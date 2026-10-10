package api

import (
	"slices"
	"strings"
	"testing"
)

// mustFail runs fn and reports unless it stops with an error that
// says want.
func mustFail(t *testing.T, name, want string, fn func()) {
	t.Helper()
	defer func() {
		t.Helper()
		err, ok := recover().(error)
		if !ok || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: recovered %v, want an error saying %q", name, err, want)
		}
	}()
	fn()
}

// A length ratio and a length difference hold two line segments, and
// a line segment and an arc in either order.
func TestRatioAndDifference(t *testing.T) {
	t.Parallel()
	d := New()
	a := d.Sketch.Line(V(0, 0, 0), V(15, 0, 0))
	b := d.Sketch.Line(V(0, 10, 0), V(8, 10, 0))
	d.Constrain.Length(b, 10)
	d.Constrain.Ratio(a, b, 2)
	if l := a.Length(); !near(l, 20) {
		t.Errorf("a is %g long at twice b, want 20", l)
	}

	d = New()
	a = d.Sketch.Line(V(0, 0, 0), V(15, 0, 0))
	b = d.Sketch.Line(V(0, 10, 0), V(8, 10, 0))
	d.Constrain.Length(b, 10)
	d.Constrain.Difference(a, b, 5)
	if l := a.Length(); !near(l, 15) {
		t.Errorf("a is %g long at b and 5, want 15", l)
	}

	// An arc first is taken as the window takes it: the ratio is the
	// line's over the arc's.
	d = New()
	line := d.Sketch.Line(V(0, -20, 0), V(10, -20, 0))
	arc := d.Sketch.Arc(V(0, 0, 0), V(10, 0, 0), V(0, 10, 0))
	d.Constrain.Radius(arc, 10)
	c := d.Constrain.Ratio(arc, line, 1)
	if l, want := line.Length(), arc.Length(); !near(l, want) {
		t.Errorf("the line is %g long, want the arc's %g", l, want)
	}
	if v := c.Value(); !near(v, 1) {
		t.Errorf("the ratio is %g, want 1", v)
	}

	mustFail(t, "ratio of a circle", "length ratio", func() {
		d.Constrain.Ratio(d.Sketch.Circle(V(0, 30, 0), 2), line, 2)
	})
	mustFail(t, "difference of points", "length difference", func() {
		d.Constrain.Difference(line.Start(), line, 2)
	})
}

// Same orientation turns a workplane drawn in 3d to the XY plane's,
// and takes only normals.
func TestSameOrientation(t *testing.T) {
	t.Parallel()
	d := New()
	d.Features.Sketch3D("free")
	wp := d.Sketch.NewWorkplane(V(5, 5, 5), V(0, 1, 0), V(0, 0, 1))
	xy := d.Workplane("XY")
	d.Constrain.SameOrientation(wp.Normal(), xy.Normal())
	g := d.Active()
	if !g.Solved() {
		t.Fatalf("the sketch didn't solve")
	}
	if n, want := wp.Normal().Dir(), xy.Normal().Dir(); !near(n.Dot(want), 1) {
		t.Errorf("the workplane's normal is %v, want %v", n, want)
	}
	mustFail(t, "a line", "a normal", func() {
		line := d.Sketch.Line(V(0, 0, 0), V(1, 0, 0))
		d.Constrain.SameOrientation(line, xy.Normal())
	})
}

// A view of a group of another kind stops the script, naming both
// kinds.
func TestViewOfAnotherKind(t *testing.T) {
	t.Parallel()
	d := New()
	d.Sketch.Rect(V(0, 0, 0), V(10, 10, 0))
	ext := d.Features.Extrude(5, "box")
	ext.AsExtrude()
	for name, view := range map[string]func(){
		"a sweep":                         func() { ext.AsSweep() },
		"a step and repeat or a pattern":  func() { ext.AsRepeat() },
		"a mirror":                        func() { ext.AsMirror() },
		"a chamfer":                       func() { ext.AsChamfer() },
		"a fillet":                        func() { ext.AsFillet() },
		"a shell":                         func() { ext.AsShell() },
		"a loft":                          func() { ext.AsLoft() },
		"a linked part":                   func() { ext.AsLink() },
		"an extrude, and g002-sketch-in-": func() { d.Group("g002").AsExtrude() },
	} {
		mustFail(t, name, "is for "+name, view)
	}
	if v := ext.Analyze().Volume(); !near(v, 500) {
		t.Errorf("the box's volume is %g, want 500", v)
	}
	if ext.History().UsedBy() != nil {
		t.Errorf("the last group is used by others")
	}
}

// A menu entry is found however its path is written, and a call
// knows its entries.
func TestMenuCalls(t *testing.T) {
	t.Parallel()
	for _, entry := range []string{"Features > Fillet Edges", "Features/Fillet Edges", "features › fillet edges"} {
		path, calls, why, ok := MenuEntryCalls(entry)
		if !ok || path != "Features/Fillet Edges" || !slices.Equal(calls, []string{"Features.Fillet"}) || why != "" {
			t.Errorf("%q is %q, %v, %q, %v", entry, path, calls, why, ok)
		}
	}
	if path, calls, why, ok := MenuEntryCalls("Edit > Parameters"); !ok || path != "Edit/Parameters..." || len(calls) == 0 || why != "" {
		t.Errorf("Edit > Parameters is %q, %v, %q, %v", path, calls, why, ok)
	}
	if _, calls, why, ok := MenuEntryCalls("View > Zoom In"); !ok || calls != nil || why != "the window's view" {
		t.Errorf("View > Zoom In is %v, %q, %v", calls, why, ok)
	}
	if _, _, _, ok := MenuEntryCalls("Features > Nothing"); ok {
		t.Errorf("an entry there isn't was found")
	}
	if got := MenuEntriesOf("Sketch.Line"); !slices.Equal(got, []string{"Sketch > Construction Line Segment", "Sketch > Line Segment"}) {
		t.Errorf("Sketch.Line is %v", got)
	}
	if got := MenuEntriesOf("Sketch.UV"); got != nil {
		t.Errorf("Sketch.UV is %v, want no entry", got)
	}
}
