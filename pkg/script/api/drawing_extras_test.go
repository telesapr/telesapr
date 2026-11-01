package api

import "testing"

// A script adds and removes a drawing's sections and details by the
// calls the drawing screen's links reach, each one step of undo that
// comes back on redo, and fails on a view there is none of or a section
// or detail off the solid, leaving the drawing as it was.
func TestDrawingSectionsAndDetails(t *testing.T) {
	t.Parallel()
	d := New()
	d.Sketch.Rect(V(0, 0, 0), V(120, 60, 0))
	d.Features.Extrude(40, "block")
	step := func() { d.h.(*SketchHost).BeginEval() }

	step()
	if l := d.Drawing.AddSection("front", 5); l != "A" {
		t.Errorf("the section is %q", l)
	}
	step()
	if l := d.Drawing.AddDetail("top", 50, 10, 8, 2); l != "B" {
		t.Errorf("the detail is %q", l)
	}
	step()
	d.Drawing.AddDimension("B", "length", 0, d.Groups()[len(d.Groups())-1].Solid().Edges()[0])
	step()
	if l := d.Drawing.AddSection("top", -10); l != "B" {
		t.Errorf("the second section is %q", l)
	}
	dr := d.Drawing.Get()
	if len(dr.Sections) != 2 || len(dr.Details) != 1 || dr.Sections[0] != (DrawingSection{View: "front", Offset: 5}) ||
		dr.Details[0] != (DrawingDetail{View: "top", CX: 50, CY: 10, R: 8, Scale: 2}) || dr.Dimensions[0].View != "C" {
		t.Errorf("the drawing: %+v", dr)
	}
	// Undo takes the second section back, the dimension's letter with
	// it; redo brings both.
	if !d.Edit.Undo() {
		t.Fatal("nothing to undo")
	}
	if dr := d.Drawing.Get(); len(dr.Sections) != 1 || dr.Dimensions[0].View != "B" {
		t.Errorf("after undo: %+v", dr)
	}
	if !d.Edit.Redo() {
		t.Fatal("nothing to redo")
	}
	if dr := d.Drawing.Get(); len(dr.Sections) != 2 || dr.Dimensions[0].View != "C" {
		t.Errorf("after redo: %+v", dr)
	}

	for _, c := range []struct {
		want string
		fn   func()
	}{
		{"no section of", func() { d.Drawing.AddSection("iso", 0) }},
		{"within 20 mm", func() { d.Drawing.AddSection("front", 25) }},
		{"within 30 mm", func() { d.Drawing.AddSection("top", 31) }},
		{"within 60 mm", func() { d.Drawing.AddDetail("front", 70, 0, 5, 2) }},
		{"radius and scale", func() { d.Drawing.AddDetail("front", 0, 0, 0, 2) }},
		{"no detail of", func() { d.Drawing.AddDetail("aslant", 0, 0, 5, 2) }},
		{"no section \"C\"", func() { d.Drawing.RemoveSection("C") }},
		{"no detail \"A\"", func() { d.Drawing.RemoveDetail("A") }},
		{"has a dimension on it", func() { d.Drawing.RemoveDetail("C") }},
	} {
		before := d.Drawing.Get()
		failsWith(t, c.want, c.fn)
		after := d.Drawing.Get()
		if len(after.Sections) != len(before.Sections) || len(after.Details) != len(before.Details) {
			t.Errorf("%q changed the drawing", c.want)
		}
	}

	// Removing section A moves the detail and its dimension back to B.
	step()
	d.Drawing.RemoveSection("A")
	if dr := d.Drawing.Get(); len(dr.Sections) != 1 || dr.Sections[0].View != "top" || dr.Dimensions[0].View != "B" {
		t.Errorf("after A was removed: %+v", dr)
	}
	step()
	dr = d.Drawing.Get()
	dr.Dimensions = nil
	d.Drawing.Set(dr)
	step()
	d.Drawing.RemoveDetail("B")
	if dr := d.Drawing.Get(); len(dr.Details) != 0 {
		t.Errorf("after the detail was removed: %+v", dr)
	}
	if !d.Edit.Undo() || len(d.Drawing.Get().Details) != 1 {
		t.Error("undo did not bring the detail back")
	}
}

// A sketch with no solid has nothing to cut.
func TestDrawingSectionNoSolid(t *testing.T) {
	t.Parallel()
	d := New()
	d.Sketch.Rect(V(0, 0, 0), V(10, 10, 0))
	failsWith(t, "no solid", func() { d.Drawing.AddSection("front", 0) })
	if len(d.Drawing.Get().Sections) != 0 {
		t.Error("a section was added")
	}
}
