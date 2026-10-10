package api

import (
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// How a group's solid combines with the solid so far.
const (
	Union        = sketch.CombineUnion
	Difference   = sketch.CombineDifference
	Intersection = sketch.CombineIntersection
	Assemble     = sketch.CombineAssemble
)

// Group is a group of the sketch: a drawing in a workplane or in 3d,
// or a step that makes a solid from the group before it, as an
// extrusion does.
type Group struct {
	d *Doc
	H core.HGroup
}

func (g Group) get(sk *sketch.Sketch) *sketch.Group {
	if g.d == nil || g.H.V == 0 {
		fail("no group")
	}
	group := sk.Group(g.H)
	if group == nil {
		fail("group g%03x doesn't exist any more", g.H.V)
	}
	return group
}

// groupDescription is how the property browser names a group.
func groupDescription(g *sketch.Group) string {
	if g == nil {
		return "no group"
	}
	name := g.Name
	if name == "" {
		name = "(unnamed)"
	}
	return fmt.Sprintf("g%03x-%s", g.H.V, name)
}

// markAllDirty has the whole sketch regenerated.
func markAllDirty(sk *sketch.Sketch) {
	for i := range sk.Groups {
		sk.Groups[i].Clean = false
	}
	sk.Unsaved = true
	sk.ScheduleGenerateAll()
}

// String describes the group as the property browser does:
// "g003-extrude".
func (g Group) String() (s string) {
	if g.d == nil || g.H.V == 0 {
		return "no group"
	}
	g.d.read(func(sk *sketch.Sketch) {
		if group := sk.Group(g.H); group != nil {
			s = groupDescription(group)
		} else {
			s = fmt.Sprintf("deleted group g%03x", g.H.V)
		}
	})
	return s
}

// Valid reports whether the group exists.
func (g Group) Valid() (ok bool) {
	if g.d == nil || g.H.V == 0 {
		return false
	}
	g.d.read(func(sk *sketch.Sketch) { ok = sk.Group(g.H) != nil })
	return ok
}

// Name is the group's name.
func (g Group) Name() (name string) {
	g.d.read(func(sk *sketch.Sketch) { name = g.get(sk).Name })
	return name
}

// SetName names the group.
func (g Group) SetName(name string) {
	g.d.change(func(sk *sketch.Sketch) {
		g.get(sk).Name = name
		sk.Unsaved = true
	})
}

// Type is the group's type, as the sketch package names them.
func (g Group) Type() (t sketch.GroupType) {
	g.d.read(func(sk *sketch.Sketch) { t = g.get(sk).Type })
	return t
}

// Kind names what the group is: "sketch-in-plane", "sketch-in-3d",
// "extrude", "lathe", "revolve", "helix", "rotate", "translate",
// "mirror", "chamfer", "sweep", "fillet", "loft", "shell" or "link".
func (g Group) Kind() string { return kindName(g.Type()) }

// kindName is the Kind of a group type.
func kindName(t sketch.GroupType) string {
	switch t {
	case sketch.GroupDrawing3D:
		return "sketch-in-3d"
	case sketch.GroupDrawingWorkplane:
		return "sketch-in-plane"
	case sketch.GroupExtrude:
		return "extrude"
	case sketch.GroupLathe:
		return "lathe"
	case sketch.GroupRevolve:
		return "revolve"
	case sketch.GroupHelix:
		return "helix"
	case sketch.GroupRotate:
		return "rotate"
	case sketch.GroupTranslate:
		return "translate"
	case sketch.GroupMirror:
		return "mirror"
	case sketch.GroupChamfer:
		return "chamfer"
	case sketch.GroupSweep:
		return "sweep"
	case sketch.GroupFillet:
		return "fillet"
	case sketch.GroupLoft:
		return "loft"
	case sketch.GroupShell:
		return "shell"
	case sketch.GroupPathPattern:
		return "pattern"
	case sketch.GroupLinked:
		return "link"
	}
	return "group"
}

// Solved reports whether the solver satisfied the group's constraints.
func (g Group) Solved() (ok bool) {
	g.d.read(func(sk *sketch.Sketch) { ok = g.get(sk).IsSolvedOkay() })
	return ok
}

// Dof is how many degrees of freedom the group's constraints leave.
func (g Group) Dof() (dof int) {
	g.d.read(func(sk *sketch.Sketch) { dof = g.get(sk).Solved.Dof })
	return dof
}

// Requests are what was drawn in the group.
func (g Group) Requests() (requests []Request) {
	g.d.read(func(sk *sketch.Sketch) {
		g.get(sk)
		for i := range sk.Requests {
			if sk.Requests[i].Group == g.H {
				requests = append(requests, Request{g.d, sk.Requests[i].H})
			}
		}
	})
	return requests
}

// Entities are the group's entities, the points and normals among
// them.
func (g Group) Entities() (entities []Entity) {
	g.d.read(func(sk *sketch.Sketch) {
		g.get(sk)
		for i := range sk.Entities {
			if sk.Entities[i].Group == g.H {
				entities = append(entities, Entity{g.d, sk.Entities[i].H})
			}
		}
	})
	return entities
}

// Constraints are the group's constraints.
func (g Group) Constraints() (constraints []Constraint) {
	g.d.read(func(sk *sketch.Sketch) {
		g.get(sk)
		for i := range sk.Constraints {
			if sk.Constraints[i].Group == g.H {
				constraints = append(constraints, Constraint{g.d, sk.Constraints[i].H})
			}
		}
	})
	return constraints
}

// Param is the group's i-th parameter: for an extrusion, 0 to 2 are
// the vector to its middle; for a step and repeat translating, the step; for a
// revolve or a step and repeat rotating, 3 is the angle in radians.
func (g Group) Param(i int) Param {
	return Param{g.d, g.H.Param(i)}
}

// Workplane is the workplane of a group that is drawn in one.
func (g Group) Workplane() (wp Entity) {
	g.d.read(func(sk *sketch.Sketch) {
		if g.get(sk).Type != sketch.GroupDrawingWorkplane {
			fail("%s isn't drawn in a workplane", groupDescription(g.get(sk)))
		}
		wp = Entity{g.d, g.H.Entity(0)}
	})
	return wp
}

// Visible reports whether the group is shown.
func (g Group) Visible() (visible bool) {
	g.d.read(func(sk *sketch.Sketch) { visible = g.get(sk).Visible })
	return visible
}

// SetVisible shows or hides the group.
func (g Group) SetVisible(visible bool) {
	g.d.change(func(sk *sketch.Sketch) {
		g.get(sk).Visible = visible
		sk.MarkGroupDirty(g.H, false)
	})
}

// SuppressMode is how much of a group is left out of the model.
type SuppressMode int

// The suppress modes: nothing left out, the solid only, or the whole
// group with every group that uses it.
const (
	Unsuppressed SuppressMode = iota
	SuppressSolid
	SuppressWhole
)

// SetSuppress leaves the group's solid out of the model with
// SuppressSolid; the whole group, its sketch, solve and solid, with
// SuppressWhole, which also leaves out every group that uses it; and
// puts everything back with Unsuppressed. Nothing is deleted. The
// references can't be suppressed whole.
func (g Group) SetSuppress(mode SuppressMode) {
	if mode < Unsuppressed || mode > SuppressWhole {
		fail("no suppress mode %d", mode)
	}
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		if mode != SuppressWhole {
			group.Suppress = mode == SuppressSolid
		}
		if err := sk.SetSuppressAll(g.H, mode == SuppressWhole); err != nil {
			fail("%v", err)
		}
		sk.MarkGroupDirty(g.H, false)
	})
}

// Suppression is the group's suppress mode, as SetSuppress set it.
func (g Group) Suppression() (mode SuppressMode) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		switch {
		case group.SuppressAll:
			mode = SuppressWhole
		case group.Suppress:
			mode = SuppressSolid
		}
	})
	return mode
}

// LeftOut says why the group is left out of the model, or "" if it is
// in: "suppressed" for a group suppressed whole, or "uses g004-sketch,
// which is suppressed" for one that uses such a group. A group left out
// has no entities and no solid of its own, and can't be made active.
func (g Group) LeftOut() (why string) {
	g.d.read(func(sk *sketch.Sketch) {
		g.get(sk)
		why = sk.SuppressedReason(g.H)
	})
	return why
}

// SetCombine says how the group's solid combines with the solid so
// far: Union, Difference, Intersection or Assemble.
func (g Group) SetCombine(how sketch.CombineAs) {
	g.d.change(func(sk *sketch.Sketch) {
		g.get(sk).MeshCombine = how
		sk.MarkGroupDirty(g.H, false)
	})
}

// SetColor colours the group's solid; each of red, green and blue is
// 0 to 255.
func (g Group) SetColor(red, green, blue int) {
	g.d.change(func(sk *sketch.Sketch) {
		g.get(sk).Color = core.NewRGBA(uint8(red), uint8(green), uint8(blue), 255)
		sk.MarkGroupDirty(g.H, false)
	})
}

// Delete deletes the group and what depends on it. If the active
// group goes, being this one or one that used it, the last group
// before it that is left becomes active, so that new groups come
// where the history ends; Doc.Active says which it is.
func (g Group) Delete() {
	g.d.change(func(sk *sketch.Sketch) {
		g.get(sk)
		if g.H.V == core.HGroupReferences {
			fail("the references can't be deleted")
		}
		g.d.h.DeleteGroup(g.H)
	})
}

// displayMesh is the triangles of the model up to and with a group,
// which are made when they are first wanted.
func displayMesh(sk *sketch.Sketch, group *sketch.Group) *srf.SMesh {
	group.GenerateDisplayItems(sk)
	return &group.DisplayMesh
}

// newGroup starts a group of a type, named name or else what the
// menu would name it.
func newGroup(typ sketch.GroupType, name, defaultName string) sketch.Group {
	g := sketch.NewGroup()
	g.Type = typ
	g.Name = name
	if name == "" {
		g.Name = defaultName
	}
	return g
}

// add adds a group made by build, which runs on the sketch.
func (d *Doc) add(build func(sk *sketch.Sketch) sketch.Group, after func(sk *sketch.Sketch, hg core.HGroup)) (g Group) {
	d.change(func(sk *sketch.Sketch) {
		// What the new group is made from must be up to date.
		if sk.RegenerationPending() {
			d.h.Regenerate()
			sk = d.h.Sketch()
		}
		sk.Active = sk.ActiveGroup()
		ng := build(sk)
		// A group refers only to what is made before it: one made with
		// an earlier group active goes in after that one (roadmap 16.4).
		if at := sk.Group(sk.Active); at != nil {
			for _, dep := range sk.DependenciesOf(&ng) {
				if on := sk.Group(dep.Group); on != nil && on.Order > at.Order {
					fail("the new group would refer to %s (%s), which comes after the active group %s in the history",
						sk.GroupName(dep.Group), dep.Why, sk.GroupName(at.H))
				}
			}
		}
		hg := d.h.AddGroup(ng)
		sk = d.h.Sketch()
		if after != nil {
			after(sk, hg)
			sk.MarkGroupDirty(hg, false)
			d.h.Regenerate()
		}
		g = Group{d, hg}
	})
	return g
}

// Face is the face of the solid so far that a sketch made by
// SketchOnFace lies in.
func (g Group) Face() (f SolidFace) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		if group.Type != sketch.GroupDrawingWorkplane || group.Subtype != sketch.GroupWorkplaneByFace {
			fail("Face is for a sketch on a face, and %s is not one", groupDescription(group))
		}
		before := group.PreviousGroup(sk)
		if before == nil || len(group.SolidFaces) == 0 {
			fail("%s has no face", groupDescription(group))
		}
		f = SolidFace{s: Solid{d: g.d, g: before.H}, id: group.SolidFaces[0]}
	})
	return f
}

// workplaneNormal is the unit normal of the active workplane, which
// there must be.
func workplaneNormal(sk *sketch.Sketch, what string) (core.HEntity, Vec) {
	wp := sk.ActiveWorkplane()
	if core.IsFreeIn3D(wp) {
		fail("%s needs a workplane to be active: the sketch is made into a solid square to it", what)
	}
	return wp, sk.MustEntity(wp).NormalEntity(sk).NormalN(sk)
}

// axisOf is the point and the direction a line segment gives to turn
// about.
func axisOf(sk *sketch.Sketch, axis Entity, what string) (origin, dir core.HEntity) {
	ent := axis.get(sk)
	if ent.Type != core.EntityLineSegment {
		fail("%s turns about a line segment, and %s isn't one", what, axis.describe(sk))
	}
	return ent.Point[0], axis.H
}

// Conflicts lists the constraints that keep the group from solving,
// as its solve-failure screen does, any one of which could be removed
// to fix it; none when the group solved.
func (g Group) Conflicts() (constraints []Constraint) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		if group.IsSolvedOkay() {
			return
		}
		for _, hc := range group.Solved.Remove {
			if sk.Constraint(hc) != nil {
				constraints = append(constraints, Constraint{g.d, hc})
			}
		}
	})
	return constraints
}

func (g Group) moveWithSources(before core.HGroup) (moved []Group, err error) {
	g.d.read(func(sk *sketch.Sketch) {
		g.get(sk)
		err = sk.CheckMoveGroupWithSources(g.H, before)
	})
	if err != nil {
		return nil, err
	}
	var hs []core.HGroup
	g.d.change(func(sk *sketch.Sketch) { hs, err = sk.MoveGroupWithSources(g.H, before) })
	for _, h := range hs {
		moved = append(moved, Group{g.d, h})
	}
	return moved, err
}

func (g Group) move(before core.HGroup) (err error) {
	g.d.read(func(sk *sketch.Sketch) {
		g.get(sk)
		err = sk.CheckMoveGroup(g.H, before)
	})
	if err != nil {
		return err
	}
	g.d.change(func(sk *sketch.Sketch) { err = sk.MoveGroup(g.H, before) })
	return err
}
