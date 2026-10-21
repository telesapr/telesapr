package api

import (
	"math"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// WrapGroup is the view of a wrap: how high it raises its profile from
// the face, or how deep it sinks it.
type WrapGroup struct{ g Group }

// Emboss wraps the active sketch's closed loops, text included, onto
// face, a cylinder or a cone of the solid so far picked from Solid, and
// raises them height mm above it. The sketch lies in a workplane
// parallel to the face's axis and is rolled onto the side of the face
// towards it as paper is, so that seen from there it looks as drawn:
// lengths along the sketch are lengths on the face. Lines across or
// along a cylinder's axis wrap exactly, other curves within 1e-7 mm. One
// loop with its holes, such as one letter, may span at most 170° round
// the axis. The new group becomes active.
func (m FeaturesMenu) Emboss(face SolidFace, height float64, name string) Group {
	return m.wrap(face, height, false, name)
}

// Engrave wraps the active sketch's closed loops onto face as Emboss
// does, and sinks them depth mm into it.
func (m FeaturesMenu) Engrave(face SolidFace, depth float64, name string) Group {
	return m.wrap(face, depth, true, name)
}

func (m FeaturesMenu) wrap(face SolidFace, h float64, engrave bool, name string) Group {
	d := m.d
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		g, err := sk.NewWrap(face.id, h, engrave, name)
		if err != nil {
			fail("%v", err)
		}
		return g
	}, nil)
}

// AsWrap is the view of a wrap.
func (g Group) AsWrap() WrapGroup {
	g.is("AsWrap", "a wrap", sketch.GroupWrap)
	return WrapGroup{g}
}

// Height is how far a wrap raises its profile above the face, or sinks
// it into the face, in mm.
func (v WrapGroup) Height() (mm float64) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupWrap, "Height")
		mm = group.ValB
	})
	return mm
}

// SetHeight sets how far a wrap raises or sinks its profile, in mm;
// SetExpr with sketch.GroupPitch gives it an expression in the
// document's unit.
func (v WrapGroup) SetHeight(mm float64) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupWrap, "SetHeight")
		if mm <= 0 || math.IsNaN(mm) || math.IsInf(mm, 0) {
			fail("a wrap's height must be more than zero, not %g", mm)
		}
		group.ValB, group.ExprB = mm, ""
		sk.MarkGroupDirty(group.H, false)
	})
}

// Engraved reports whether the wrap sinks its profile into the face
// rather than raising it: its combine is a difference.
func (v WrapGroup) Engraved() (yes bool) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupWrap, "Engraved")
		yes = group.MeshCombine == sketch.CombineDifference
	})
	return yes
}
