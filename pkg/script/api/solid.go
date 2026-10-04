package api

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// Mirror reflects the active group in plane, a workplane or a planar
// face: its entities are copied reflected, and its solid is reflected
// and joined with the original, combined with the model before it as
// the active group's solid is, so a mirrored hole is cut too. The
// copies follow the plane when it moves. The new group becomes
// active; SetKeepOriginal(false) leaves the original out of the
// solid.
func (d *Doc) Mirror(plane Entity, name string) Group {
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		g, err := sk.NewMirror(plane.get(sk).H, name)
		if err != nil {
			fail("%v", err)
		}
		return g
	}, nil)
}

// KeepOriginal reports whether a mirror's solid has the original as
// well as the reflection.
func (g Group) KeepOriginal() (keep bool) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupMirror, "KeepOriginal")
		keep = !group.SkipFirst
	})
	return keep
}

// SetKeepOriginal has a mirror's solid keep the original beside the
// reflection, or not, which leaves the reflection alone.
func (g Group) SetKeepOriginal(keep bool) {
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupMirror, "SetKeepOriginal")
		group.SkipFirst = !keep
		sk.MarkGroupDirty(group.H, false)
	})
}

// MirrorSolid reports whether a mirror reflects the whole solid so far,
// up to and with its source group, rather than the source group's own
// solid.
func (g Group) MirrorSolid() (whole bool) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupMirror, "MirrorSolid")
		whole = group.MirrorSolid
	})
	return whole
}

// SetMirrorSolid has a mirror reflect the whole solid so far, as a body
// made of several groups needs, or only its source group's solid. With
// the whole solid, the mirror's result is the solid so far: with its
// reflection when it keeps the original, or the reflection alone.
func (g Group) SetMirrorSolid(whole bool) {
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupMirror, "SetMirrorSolid")
		group.MirrorSolid = whole
		sk.MarkGroupDirty(group.H, false)
	})
}

// Plane is the workplane or face a mirror reflects in.
func (g Group) Plane() (plane Entity) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupMirror, "Plane")
		plane = Entity{d: g.d, H: group.Predef.EntityB}
	})
	return plane
}

// Problem says what the last regeneration found wrong with the
// group's input, as an edge a chamfer can't cut, a sweep path with
// a corner, or a sketch that crosses itself, is open, is not flat or
// has a curve of no length, with the point where; or "" when there is
// nothing. A group with a problem adds no solid. A sketch says what is
// wrong with its contours only when a solid is made from it, since a
// sweep's path may be open and not flat; the group making the solid
// says it too.
func (g Group) Problem() (msg string) {
	g.d.read(func(sk *sketch.Sketch) { msg = g.get(sk).Problem.Msg })
	return msg
}

// mustBe fails unless the group is of type t.
func (g Group) mustBe(group *sketch.Group, t sketch.GroupType, what string) {
	if group.Type != t {
		fail("%s is for a %s group, and %s is a %s", what, kindName(t), groupDescription(group), kindName(group.Type))
	}
}

// Chamfer cuts edges of the solid so far, at distance mm from each
// edge on both faces that meet there, a 45 degree chamfer where they
// are square. An edge is a line segment that lies along an edge
// between two flat faces and sticks out, as the copies of the sketch
// at the ends of an extrusion and the lines along its sides do; or a
// circle or arc where a flat face meets a cylinder or a cone about the
// circle's axis, as an extruded circle's or a lathed frustum's, which is
// cut all round by a cone. The new group
// becomes active.
func (d *Doc) Chamfer(edges []Entity, distance float64, name string) Group {
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		hs := make([]core.HEntity, len(edges))
		for i, e := range edges {
			hs[i] = e.get(sk).H
		}
		g, err := sk.NewChamfer(hs, nil, distance, name)
		if err != nil {
			fail("%v", err)
		}
		return g
	}, nil)
}

// PatternAlongPath repeats the active group's solid along path, line
// segments and arcs joined end to end and meeting tangent, as a sweep's
// path, copies times in all: the first where the solid is and the last
// at the far end of the path, spaced evenly along it, each turned with
// it, as New Group > Pattern Along Path does. The path starts at its
// end nearest the solid. Copies and SetCopies read and change the
// count; the sketch's entities are not copied.
func (d *Doc) PatternAlongPath(path []Entity, copies int, name string) Group {
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		hs := make([]core.HEntity, len(path))
		for i, e := range path {
			hs[i] = e.get(sk).H
		}
		g, err := sk.NewPathPattern(hs, copies, name)
		if err != nil {
			fail("%v", err)
		}
		return g
	}, nil)
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

// Distance is a chamfer's distance in mm.
func (g Group) Distance() (mm float64) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupChamfer, "Distance")
		mm = group.ValB
	})
	return mm
}

// SetDistance sets a chamfer's distance in mm; SetExpr with
// sketch.GroupPitch gives it an expression in the document's unit.
func (g Group) SetDistance(mm float64) {
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupChamfer, "SetDistance")
		if mm <= 0 || math.IsNaN(mm) || math.IsInf(mm, 0) {
			fail("a chamfer's distance must be more than zero, not %g", mm)
		}
		group.ValB, group.ExprB = mm, ""
		sk.MarkGroupDirty(group.H, false)
	})
}

// Sweep moves the closed loops of the active group, a sketch in a
// workplane, along path, line segments, arcs and cubics joined end to
// end and meeting tangent, as a pipe follows its bends. The sweep
// starts at the end of the path nearest the sketch's plane, with the
// sketch where it is drawn, and keeps its attitude to the path, turning
// as little as the path lets it. Along lines and arcs its surfaces are
// exact; along cubics they are fitted within a tenth of the chord
// tolerance (see also SetTwist and SetSweepUp). The new group becomes
// active.
func (d *Doc) Sweep(path []Entity, name string) Group {
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		hs := make([]core.HEntity, len(path))
		for i, e := range path {
			hs[i] = e.get(sk).H
		}
		g, err := sk.NewSweep(hs, name)
		if err != nil {
			fail("%v", err)
		}
		return g
	}, nil)
}

// Interference is two parts of an assembly that overlap, and the
// volume they share in mm³.
type Interference struct {
	A, B   Group
	Volume float64
}

// Interferences checks the parts of the assembly up to the active
// group against each other by their shells (Analyze > Show Interfering
// Parts): each group that combines as an assembly is a part, and so is
// the solid made before the first of them. It lists the pairs that
// overlap, not those that only touch. A part with no shell, as a
// linked mesh or STEP file, can't be checked so; meshOnly lists them.
func (d *Doc) Interferences() (found []Interference, meshOnly []Group) {
	d.read(func(sk *sketch.Sketch) {
		fs, mo := sk.Interferences(sk.ActiveGroup())
		for _, f := range fs {
			if f.Failed {
				fail("the Boolean between %s and %s failed", sk.GroupName(f.A), sk.GroupName(f.B))
			}
			found = append(found, Interference{A: Group{d, f.A}, B: Group{d, f.B}, Volume: f.Volume})
		}
		for _, h := range mo {
			meshOnly = append(meshOnly, Group{d, h})
		}
	})
	return found, meshOnly
}
