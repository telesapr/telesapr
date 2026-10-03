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
// group's input, as an edge a chamfer can't cut or a sweep path with
// a corner, or "" when there is nothing; a group with a problem adds
// no solid.
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

// Edges are the edges a chamfer cuts or a fillet rounds, or the path a
// sweep follows.
func (g Group) Edges() (edges []Entity) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		if group.Type != sketch.GroupChamfer && group.Type != sketch.GroupSweep && group.Type != sketch.GroupFillet {
			fail("Edges is for a chamfer, a fillet or a sweep, and %s is a %s", groupDescription(group), kindName(group.Type))
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
// workplane, along path, line segments and arcs joined end to end and
// meeting tangent, as a pipe follows its bends. The sweep starts at
// the end of the path nearest the sketch's plane, with the sketch
// where it is drawn, and keeps its attitude to the path. The new group
// becomes active.
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
