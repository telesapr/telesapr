package sketch

import "testing"

// Views are shown, moved and hidden on a drawing whose views are
// placed; the last one and one with a dimension stay.
func TestDrawingViews(t *testing.T) {
	t.Parallel()
	d := DefaultDrawing()
	if err := d.PlaceView("front", 1, 2); err == nil {
		t.Error("a view was placed on a drawing with the standard views")
	}
	d.Views = []DrawingView{{Kind: "front", X: 50, Y: 60}, {Kind: "top", X: 50, Y: 150}}
	if err := d.PlaceView("front", 70, 80); err != nil || d.Views[0].X != 70 || d.Views[0].Y != 80 || len(d.Views) != 2 {
		t.Errorf("moving the front view: %v, %+v", err, d.Views)
	}
	if err := d.PlaceView("iso", 200, 100); err != nil || d.ViewIndex("iso") != 2 {
		t.Errorf("showing the iso view: %v, %+v", err, d.Views)
	}
	if err := d.PlaceView("sideways", 0, 0); err == nil {
		t.Error("a view of no kind was placed")
	}
	d.Dimensions = []DrawingDimension{{View: "top", Kind: "length"}}
	if err := d.RemoveView("top"); err == nil {
		t.Error("a view with a dimension was removed")
	}
	d.Dimensions = nil
	if err := d.RemoveView("top"); err != nil || d.ViewIndex("top") >= 0 {
		t.Errorf("removing the top view: %v, %+v", err, d.Views)
	}
	if err := d.RemoveView("top"); err == nil {
		t.Error("a view not shown was removed")
	}
	if err := d.RemoveView("iso"); err != nil {
		t.Error(err)
	}
	if err := d.RemoveView("front"); err == nil {
		t.Error("the last view was removed")
	}

	d.Sections = []DrawingSection{{View: "top"}}
	d.Details = []DrawingDetail{{View: "front", R: 1, Scale: 2}}
	if err := d.PlaceSection(0, 10, 20); err != nil || d.Sections[0].X != 10 || d.Sections[0].Y != 20 {
		t.Errorf("placing the section: %v, %+v", err, d.Sections)
	}
	if err := d.PlaceDetail(0, 30, 40); err != nil || d.Details[0].X != 30 || d.Details[0].Y != 40 {
		t.Errorf("placing the detail: %v, %+v", err, d.Details)
	}
	if d.PlaceSection(1, 0, 0) == nil || d.PlaceDetail(-1, 0, 0) == nil {
		t.Error("a section or detail that isn't there was placed")
	}
}
