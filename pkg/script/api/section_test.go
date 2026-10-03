package api

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// sectionBox is a 20 x 10 x 30 box standing on the sketch's workplane.
func sectionBox(t *testing.T) (d *Doc, wp Entity, ext Group) {
	t.Helper()
	d = New()
	wp = d.Groups()[0].Workplane()
	d.Rect(V(0, 0, 0), V(20, 10, 0))
	ext = d.Extrude(30, "")
	return d, wp, ext
}

func TestSection(t *testing.T) {
	d, wp, ext := sectionBox(t)
	if s := d.Section(); s.On {
		t.Fatalf("the section view is on in a new sketch: %+v", s)
	}

	d.SetSection(wp, "12", false)
	s := d.Section()
	if !s.On || s.Plane.H != wp.H || s.Offset != "12" || s.Flip || s.Err != "" {
		t.Errorf("state %+v", s)
	}
	if !s.Normal.Equals(V(0, 0, 1)) || !s.Point.Equals(V(0, 0, 12)) {
		t.Errorf("cut %v through %v, want +z through z = 12", s.Normal, s.Point)
	}
	if !near(s.Area, 200) || s.Loops != 1 || !s.Closed {
		t.Errorf("cap %v mm² in %d loops (closed %v), want 200 in 1", s.Area, s.Loops, s.Closed)
	}

	d.SetSection(wp, "12", true)
	if s := d.Section(); !s.Flip || !s.Normal.Equals(V(0, 0, -1)) || !near(s.Area, 200) {
		t.Errorf("flipped: %+v", s)
	}

	// Above the box, nothing is cut.
	d.SetSection(wp, "40", false)
	if s := d.Section(); s.Area != 0 || s.Loops != 0 || !s.Closed {
		t.Errorf("above the box: %+v", s)
	}

	// Each face, offset into the solid, gives the box's cross-section
	// parallel to it: face normals are taken to point out.
	areas := map[float64]int{}
	for _, f := range ext.Faces() {
		d.SetSection(f, "-1", false)
		s := d.Section()
		if !s.Closed || s.Loops != 1 {
			t.Errorf("%v: %+v", f, s)
		}
		areas[math.Round(s.Area)]++
	}
	if areas[200] != 2 || areas[300] != 2 || areas[600] != 2 {
		t.Errorf("cross-sections parallel to the faces %v, want two each of 200, 300 and 600", areas)
	}

	d.ClearSection()
	if s := d.Section(); s.On {
		t.Errorf("still on after ClearSection: %+v", s)
	}
}

// An offset may be an expression in the document's unit, and follows
// the parameters it names.
func TestSectionOffsetExpression(t *testing.T) {
	d, wp, _ := sectionBox(t)
	if err := d.SetParameter("h", "10"); err != nil {
		t.Fatal(err)
	}
	d.SetSection(wp, "h + 2", false)
	if s := d.Section(); !s.Point.Equals(V(0, 0, 12)) || s.Offset != "h + 2" {
		t.Errorf("offset h + 2: %+v", s)
	}
	if err := d.SetParameter("h", "20"); err != nil {
		t.Fatal(err)
	}
	if s := d.Section(); !s.Point.Equals(V(0, 0, 22)) {
		t.Errorf("after h = 20, the cut is through %v", s.Point)
	}
	d.SetUnits(core.UnitInches)
	d.SetSection(wp, "1", false)
	if s := d.Section(); !near(s.Point.Z, 25.4) {
		t.Errorf("an offset of 1 in inches cuts at z = %v", s.Point.Z)
	}
	// A parameter the offset names goes away: the view stays on but
	// can't cut, and says why.
	d.SetUnits(core.UnitMM)
	d.SetSection(wp, "h", false)
	d.With(func(sk *sketch.Sketch) { sk.Parameters = nil })
	if s := d.Section(); !s.On || s.Err == "" || s.Area != 0 {
		t.Errorf("with h gone: %+v", s)
	}
}

// A section on a face of a group that is deleted can't cut, and says
// so.
func TestSectionPlaneGone(t *testing.T) {
	d, _, ext := sectionBox(t)
	d.SetSection(ext.Faces()[0], "", false)
	ext.Delete()
	s := d.Section()
	if !s.On || !strings.Contains(s.Err, "gone") {
		t.Errorf("after deleting the face's group: %+v", s)
	}
}

func TestSectionMisuse(t *testing.T) {
	d, wp, ext := sectionBox(t)
	face := ext.Faces()[0]
	for name, fn := range map[string]func(){
		"a normal":          func() { d.SetSection(wp.Normal(), "", false) },
		"a bad expression":  func() { d.SetSection(wp, "1+", false) },
		"unknown parameter": func() { d.SetSection(wp, "q", false) },
		"not a number":      func() { d.SetSection(wp, "1/0", false) },
		"deleted entity":    func() { ext.Delete(); d.SetSection(face, "", false) },
	} {
		func() {
			defer func() {
				if _, ok := recover().(error); !ok {
					t.Errorf("%s: no error", name)
				}
			}()
			fn()
		}()
	}
	if s := d.Section(); s.On {
		t.Errorf("a refused section turned the view on: %+v", s)
	}
}
