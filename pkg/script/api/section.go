package api

import (
	"math"

	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Section is the state of the section view, as its screen shows it,
// and the cut of the active group's solid.
type Section struct {
	// On is whether the section view is on; the rest is zero when it
	// is off.
	On bool
	// Plane is the workplane or face the cut is parallel to.
	Plane Entity
	// Offset is the cut's distance from Plane along its normal, as
	// typed, in the document's unit.
	Offset string
	// Flip is set when the side the normal points to is kept.
	Flip bool
	// Err says why the view can't cut, as when Plane was deleted.
	Err string
	// Normal points to the side cut away, and Point is on the cut.
	Normal, Point Vec
	// Area is the area in mm² of the cap, the cut through the active
	// group's solid; Loops is how many loops bound it, and Closed is
	// false when one of them is open because the solid is not closed.
	Area   float64
	Loops  int
	Closed bool
}

// SetSection turns the section view on: the solid is drawn cut by a
// plane parallel to plane, a workplane or a planar face, offset along
// its normal (a number or an expression in the document's unit, ""
// for none), the side the normal points to taken away, or the other
// side with flip. It is a way of looking, as View > Section View is:
// not saved and not undone. Render draws it too.
func (d *Doc) SetSection(plane Entity, offset string, flip bool) {
	d.h.Run(func() {
		sk := d.h.Sketch()
		if err := sk.SetSection(plane.get(sk).H, offset, flip); err != nil {
			fail("%v", err)
		}
	})
}

// ClearSection turns the section view off.
func (d *Doc) ClearSection() {
	d.h.Run(func() { d.h.Sketch().ClearSection() })
}

// Section is the section view's state and its cut of the active
// group's solid.
func (d *Doc) Section() (s Section) {
	d.read(func(sk *sketch.Sketch) {
		sec := sk.Section
		if sec == nil {
			return
		}
		s.On = true
		s.Plane = Entity{d: d, H: sec.Plane}
		s.Offset, s.Flip = sec.Offset, sec.Flip
		keep, point, ok, err := sk.SectionCut()
		if !ok {
			if err != nil {
				s.Err = err.Error()
			}
			return
		}
		s.Normal, s.Point = keep.N, point
		g := sk.Group(sk.ActiveGroup())
		if g == nil {
			return
		}
		var poly srf.SPolygon
		s.Closed = displayMesh(sk, g).MakeSectionPolygonInto(keep, &poly)
		s.Loops = len(poly.Contours)
		poly.FixContourDirections()
		s.Area = math.Abs(poly.SignedArea())
	})
	return s
}

// Faces are the planar faces this group made, which constraints and
// the section view can refer to.
func (g Group) Faces() (faces []Entity) {
	g.d.read(func(sk *sketch.Sketch) {
		h := g.get(sk).H
		for i := range sk.Entities {
			if e := &sk.Entities[i]; e.Group == h && e.IsFace() {
				faces = append(faces, Entity{d: g.d, H: e.H})
			}
		}
	})
	return faces
}
