package api

import "github.com/telesapr/telesapr/pkg/sketch"

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
