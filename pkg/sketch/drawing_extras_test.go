package sketch

import (
	"errors"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// boxMesh is a mesh whose points span x 0 to 100, y 0 to 60 and z 0 to
// 40: enough for the extents a section and a detail are checked
// against, which are all that is read of it.
func boxMesh() *srf.SMesh {
	var m srf.SMesh
	m.AddTriangle(srf.STriangle{A: geom.Vector{}, B: geom.Vector{X: 100}, C: geom.Vector{Y: 60}})
	m.AddTriangle(srf.STriangle{A: geom.Vector{Z: 40}, B: geom.Vector{X: 100, Z: 40}, C: geom.Vector{Y: 60, Z: 40}})
	return &m
}

// A section must cut the solid and a detail's middle be on its view;
// both are refused on a view there is none of or with no solid.
func TestDrawingAddExtras(t *testing.T) {
	t.Parallel()
	solid := boxMesh()
	d := DefaultDrawing()
	// The front view looks along z, 40 deep: 20 either side of the middle.
	if err := d.AddSection("front", 19.5, solid); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		view   string
		offset float64
		solid  *srf.SMesh
	}{{"front", 20, solid}, {"front", -25, solid}, {"top", 30, solid}, {"iso", 0, solid},
		{"sideways", 0, solid}, {"front", 0, nil}} {
		if err := d.AddSection(c.view, c.offset, c.solid); err == nil {
			t.Errorf("a section of %s at %g was added", c.view, c.offset)
		}
	}
	if err := d.AddSection("front", 0, &srf.SMesh{}); !errors.Is(err, ErrNoDrawingSolid) {
		t.Errorf("with no solid: %v", err)
	}
	// The right view has z to its left: 20 either side, y 30 up or down.
	if err := d.AddDetail("right", -20, 30, 5, 2, solid); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		view           string
		x, y, r, times float64
	}{{"right", 21, 0, 5, 2}, {"right", 0, -31, 5, 2}, {"right", 0, 0, 0, 2}, {"right", 0, 0, 5, -1},
		{"sideways", 0, 0, 5, 2}} {
		if err := d.AddDetail(c.view, c.x, c.y, c.r, c.times, solid); err == nil {
			t.Errorf("a detail %+v was added", c)
		}
	}
	if err := d.AddDetail("iso", 0, 0, 5, 2, nil); !errors.Is(err, ErrNoDrawingSolid) {
		t.Errorf("a detail with no solid: %v", err)
	}
	if len(d.Sections) != 1 || len(d.Details) != 1 || d.Check() != nil {
		t.Errorf("the drawing: %+v, %v", d, d.Check())
	}
}

// Adding a section moves the details' letters on and removing one
// moves those after it back, the dimensions on them with them; one
// with a dimension on it stays.
func TestDrawingExtrasLettered(t *testing.T) {
	t.Parallel()
	solid := boxMesh()
	d := DefaultDrawing()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(d.AddSection("front", 0, solid))           // A
	must(d.AddDetail("front", 10, 10, 5, 2, solid)) // B
	d.Dimensions = []DrawingDimension{{View: "B", Kind: "length"}, {View: "front", Kind: "length"}}
	must(d.AddSection("top", 0, solid)) // B, the detail C
	if d.Dimensions[0].View != "C" || d.Dimensions[1].View != "front" {
		t.Errorf("after a section was added: %+v", d.Dimensions)
	}
	if i, section, ok := d.ExtraIndex("C"); !ok || section || i != 0 {
		t.Errorf("C is %d, %v, %v", i, section, ok)
	}
	if _, _, ok := d.ExtraIndex("front"); ok {
		t.Error("front named a section or detail")
	}
	if err := d.RemoveDetail(0); err == nil {
		t.Error("a detail with a dimension on it was removed")
	}
	must(d.RemoveSection(0))
	if d.Sections[0].View != "top" || d.Dimensions[0].View != "B" {
		t.Errorf("after section A was removed: %+v, %+v", d.Sections, d.Dimensions)
	}
	if err := d.RemoveSection(1); err == nil {
		t.Error("a section past the last was removed")
	}
	if err := d.RemoveDetail(-1); err == nil {
		t.Error("a detail before the first was removed")
	}
	d.Dimensions = d.Dimensions[1:]
	must(d.RemoveDetail(0))
	if len(d.Details) != 0 || len(d.Sections) != 1 {
		t.Errorf("the drawing: %+v", d)
	}
}
