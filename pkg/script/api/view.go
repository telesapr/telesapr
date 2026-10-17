package api

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// ViewMenu holds the calls of the View menu that change what the
// drawing shows: the section, the units of dimensions and the
// appearance of faces. A script reaches it as doc.View.
type ViewMenu struct{ d *Doc }

// SetFaceAppearance gives one face of the solid its own appearance, or
// with "" takes it away, so that the face has its group's. The face is
// kept by its identity, in the group that made it.
func (m ViewMenu) SetFaceAppearance(face SolidFace, name string) {
	d := m.d
	checkAppearance(name)
	d.change(func(sk *sketch.Sketch) {
		if err := sk.SetFaceAppearance(face.id, name); err != nil {
			fail("%v", err)
		}
	})
}

// FaceAppearance is the appearance the face has: its own, its group's,
// or "" for the default.
func (m ViewMenu) FaceAppearance(face SolidFace) (name string) {
	d := m.d
	d.read(func(sk *sketch.Sketch) { name = sk.AppearanceOf(face.id) })
	return name
}

// MmPerUnit is the size in mm of the unit the user interface shows
// lengths in.
func (m ViewMenu) MmPerUnit() (mm float64) {
	d := m.d
	d.h.Run(func() { mm = d.h.MmPerUnit() })
	return mm
}

// Units is the document's unit of length: what lengths typed as text,
// in Constraint.SetExpr and Group.SetExpr and in the user interface,
// are in. Every other length this package takes or gives is in mm.
func (m ViewMenu) Units() (u core.Unit) {
	d := m.d
	d.read(func(sk *sketch.Sketch) { u = sk.Units })
	return u
}

// SetUnits makes u the document's unit, keeping every size, as View >
// Dimension Units does: a length expression is rewritten to give the
// same length in u, "(1/4)*25.4" for "1/4" from inches to mm. It
// returns the parameters that lengths are computed from, which now
// count in u.
func (m ViewMenu) SetUnits(u core.Unit) (check []string) {
	d := m.d
	d.change(func(sk *sketch.Sketch) { check = sk.SetUnits(u) })
	return check
}

// SetSection turns the section view on: the solid is drawn cut by a
// plane parallel to plane, a workplane or a planar face, offset along
// its normal (a number or an expression in the document's unit, ""
// for none), the side the normal points to taken away, or the other
// side with flip. It is a way of looking, as View > Section View is:
// not saved and not undone. Render draws it too.
func (m ViewMenu) SetSection(plane Entity, offset string, flip bool) {
	d := m.d
	d.h.Run(func() {
		sk := d.h.Sketch()
		if err := sk.SetSection(plane.get(sk).H, offset, flip); err != nil {
			fail("%v", err)
		}
	})
}

// ClearSection turns the section view off.
func (m ViewMenu) ClearSection() {
	d := m.d
	d.h.Run(func() { d.h.Sketch().ClearSection() })
}

// Section is the section view's state and its cut of the active
// group's solid.
func (m ViewMenu) Section() (s Section) {
	d := m.d
	d.read(func(sk *sketch.Sketch) {
		sec := sk.Section
		if sec == nil {
			return
		}
		s.On = true
		s.Plane = Entity{d: d, H: sec.Plane}
		s.PlaneName = sk.SectionPlaneName()
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
