package api

import (
	"strings"
	"testing"
)

// A point is coincident while a constraint says so, and not after the
// constraint is taken away.
func TestEntityIsCoincident(t *testing.T) {
	d := New()
	a := d.Sketch.Point(V(0, 0, 0))
	b := d.Sketch.Point(V(3, 4, 0))
	d.Constrain.Lock(a)
	if a.IsCoincident() {
		t.Error("a point with nothing on it is coincident")
	}
	d.Constrain.Coincident(a, b)
	if !a.IsCoincident() || !b.IsCoincident() {
		t.Error("the two points are held together, and neither says so")
	}
	a.DeleteCoincidences()
	if a.IsCoincident() {
		t.Error("the coincidence is still there after it is deleted")
	}
	// Only a point can be coincident with another.
	line := d.Sketch.Line(V(0, 0, 0), V(1, 0, 0))
	defer func() {
		if _, ok := recover().(error); !ok {
			t.Error("a line segment is coincident with no error")
		}
	}()
	line.IsCoincident()
}

// An entity is drawn while its group is shown and View > Show lets it
// be, as the window draws it.
func TestEntityVisible(t *testing.T) {
	d := New()
	line := d.Sketch.Line(V(0, 0, 0), V(10, 0, 0))
	if !line.Visible() {
		t.Error("a line in the active group is not drawn")
	}
	g := d.Active()
	g.SetVisible(false)
	if line.Visible() {
		t.Error("a line in a hidden group is drawn")
	}
	g.SetVisible(true)
	if !line.Visible() {
		t.Error("the group is shown again, and the line is not drawn")
	}
}

// The section view names the plane it cuts by, as its screen does: a
// workplane by its group, a face by the group it is of.
func TestSectionPlaneName(t *testing.T) {
	d := New()
	d.Sketch.Rect(V(0, 0, 0), V(20, 10, 0))
	d.Features.Extrude(10, "plate")
	plate := d.Active()
	wp := d.Workplane("XY")
	d.View.SetSection(wp, "5", false)
	if name := d.View.Section().PlaneName; name == "" || !strings.Contains(name, "XY") {
		t.Errorf("the workplane is named %q", name)
	}
	for _, f := range plate.Faces() {
		d.View.SetSection(f, "-1", false)
		if name := d.View.Section().PlaneName; name == "" || !strings.Contains(name, "plate") {
			t.Errorf("the face is named %q", name)
		}
	}
}
