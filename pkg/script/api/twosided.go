package api

import "github.com/telesapr/telesapr/pkg/sketch"

// mustBeSided fails unless the group is one the text window offers
// one-sided and two-sided for.
func mustBeSided(g Group, group *sketch.Group, what string) {
	switch group.Type {
	case sketch.GroupExtrude, sketch.GroupRotate, sketch.GroupTranslate, sketch.GroupRevolve, sketch.GroupHelix:
		return
	}
	fail("%s is for an extrude, step and repeat, revolve or helix group, and %s is a %s",
		what, groupDescription(group), kindName(group.Type))
}

// TwoSided reports whether an extrusion, step and repeat, revolve or
// helix goes both ways from its sketch, as the text window's
// "two-sided" says.
func (g Group) TwoSided() (two bool) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		mustBeSided(g, group, "TwoSided")
		two = group.Subtype == sketch.GroupTwoSided || group.Subtype == sketch.GroupTwoSkewed
	})
	return two
}

// SetTwoSided makes an extrusion, step and repeat, revolve or helix go
// both ways from its sketch, or one way when two is false, as the text
// window's "two-sided" and "one-sided" do. An extrusion keeps its whole
// depth: Extrude(10, ...) then SetTwoSided(true) runs from -5 to 5
// along the normal, the sketch in the middle. Copies of a step and
// repeat are laid on both sides of the original; a revolve turns as far
// each way.
func (g Group) SetTwoSided(two bool) {
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		mustBeSided(g, group, "SetTwoSided")
		switch {
		case two && group.Subtype == sketch.GroupOneSided:
			group.Subtype = sketch.GroupTwoSided
		case two && group.Subtype == sketch.GroupOneSkewed:
			group.Subtype = sketch.GroupTwoSkewed
		case !two && group.Subtype == sketch.GroupTwoSided:
			group.Subtype = sketch.GroupOneSided
		case !two && group.Subtype == sketch.GroupTwoSkewed:
			group.Subtype = sketch.GroupOneSkewed
		default:
			return
		}
		sk.MarkGroupDirty(group.H, false)
	})
}
