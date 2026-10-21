package api

import (
	"github.com/telesapr/telesapr/pkg/sketch"
)

// RevolveGroup is the view of a revolve or a helix: the angle it turns
// by.
type RevolveGroup struct{ g Group }

// AsRevolve is the view of a revolve or a helix.
func (g Group) AsRevolve() RevolveGroup {
	g.is("AsRevolve", "a revolve or a helix", sketch.GroupRevolve, sketch.GroupHelix)
	return RevolveGroup{g}
}

// Depth is an extrusion's depth in mm, from the sketch's plane to its
// far end along the workplane's normal, negative against it; a
// two-sided extrusion's from end to end. It is what the extrusion has
// now, held or not.
func (v ExtrudeGroup) Depth() (mm float64) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupExtrude, "Depth")
		mm = group.MeasuredExtent(sk)
	})
	return mm
}

// SetDepth holds an extrusion's depth at mm, as Depth measures it, in
// place of a dimension: the extrusion has no freedom along its normal
// left, and a drag doesn't move its end. A dimension that holds it too
// is then redundant. SetExpr with sketch.GroupExtent gives it an
// expression in the document's unit, and FreeDepth frees it again. An
// extrusion of a sketch in 3d has no normal to hold it along.
func (v ExtrudeGroup) SetDepth(mm float64) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupExtrude, "SetDepth")
		if err := sk.SetGroupExtent(group.H, mm); err != nil {
			fail("%v", err)
		}
	})
}

// DepthHeld reports whether SetDepth, or an expression, holds the
// extrusion's depth.
func (v ExtrudeGroup) DepthHeld() (held bool) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupExtrude, "DepthHeld")
		held = group.Extent != 0
	})
	return held
}

// FreeDepth frees an extrusion's depth, which stays where it is, to be
// dragged or dimensioned again.
func (v ExtrudeGroup) FreeDepth() {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupExtrude, "FreeDepth")
		if err := sk.SetGroupExtentHeld(group.H, false); err != nil {
			fail("%v", err)
		}
	})
}

// Angle is the angle in degrees a revolve or a helix turns its sketch
// by, from its first copy to its last, negative the other way round. It
// is what the group has now, held or not.
func (v RevolveGroup) Angle() (degrees float64) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) { degrees = g.get(sk).MeasuredExtent(sk) })
	return degrees
}

// SetAngle holds a revolve's or helix's angle at degrees, as Angle
// measures it, in place of a dimension. A revolve turns by 360 degrees
// at most. SetExpr with sketch.GroupExtent gives it an expression, and
// FreeAngle frees it again.
func (v RevolveGroup) SetAngle(degrees float64) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		if err := sk.SetGroupExtent(g.get(sk).H, degrees); err != nil {
			fail("%v", err)
		}
	})
}

// AngleHeld reports whether SetAngle, or an expression, holds the
// angle.
func (v RevolveGroup) AngleHeld() (held bool) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) { held = g.get(sk).Extent != 0 })
	return held
}

// FreeAngle frees a revolve's or helix's angle, which stays where it is,
// to be dragged or dimensioned again.
func (v RevolveGroup) FreeAngle() {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		if err := sk.SetGroupExtentHeld(g.get(sk).H, false); err != nil {
			fail("%v", err)
		}
	})
}
