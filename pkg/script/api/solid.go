package api

import (
	"github.com/telesapr/telesapr/pkg/sketch"
)

// Problem says what the last regeneration found wrong with the
// group's input, as an edge a chamfer can't cut, a sweep path with
// a corner, or a sketch that crosses itself, is open, is not flat or
// has a curve of no length, with the point where; or "" when there is
// nothing. A group with a problem adds no solid. A sketch says what is
// wrong with its contours only when a solid is made from it, since a
// sweep's path may be open and not flat; the group making the solid
// says it too. It also says when the Boolean that joins the group's
// solid to the solid so far failed: the group's solid is then
// unsound, and Check says where. When the solid so far holds parts
// assembled side by side, which may stand in each other, it says so:
// a Boolean with such a solid is not defined.
func (g Group) Problem() (msg string) {
	g.d.read(func(sk *sketch.Sketch) { msg = problemOf(sk, g.get(sk)) })
	return msg
}

// problemOf is the group's problem, or that its Boolean failed.
func problemOf(sk *sketch.Sketch, group *sketch.Group) string {
	switch {
	case group.Problem.Msg != "":
		return group.Problem.Msg
	case group.BooleanFailed && group.JoinsAssembly(sk):
		return "the Boolean with the solid so far failed, so the solid is unsound: the solid so far " +
			"holds parts assembled side by side, and where they stand in each other the Boolean is not " +
			"defined (Assembly.Interferences finds where); join or cut each part before it is " +
			"assembled, or force NURBS surfaces to triangle mesh to join them as meshes"
	case group.BooleanFailed:
		return "the Boolean with the solid so far failed, so the solid is unsound; " +
			"the option to force NURBS surfaces to triangle mesh may join them as meshes"
	}
	return ""
}

// mustBe fails unless the group is of type t.
func (g Group) mustBe(group *sketch.Group, t sketch.GroupType, what string) {
	if group.Type != t {
		fail("%s is for a %s group, and %s is a %s", what, kindName(t), groupDescription(group), kindName(group.Type))
	}
}

// Edges are the edges a chamfer cuts or a fillet rounds, or the path a
// sweep or a pattern follows.
func (g Group) Edges() (edges []Entity) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		if group.Type != sketch.GroupChamfer && group.Type != sketch.GroupSweep && group.Type != sketch.GroupFillet &&
			group.Type != sketch.GroupPathPattern {
			fail("Edges is for a chamfer, a fillet, a sweep or a pattern, and %s is a %s", groupDescription(group), kindName(group.Type))
		}
		for _, he := range group.Edges {
			edges = append(edges, Entity{d: g.d, H: he})
		}
	})
	return edges
}

// Interference is two parts of an assembly that overlap, and the
// volume they share in mm³. Approximate is set when the volume is
// that of the parts' meshes, within the chord tolerance of their
// surfaces, the Boolean of their shells having failed, or that of a
// part before a cut that failed on it.
type Interference struct {
	A, B        Group
	Volume      float64
	Approximate bool
}
