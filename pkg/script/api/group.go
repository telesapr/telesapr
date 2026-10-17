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

// Dof is how many degrees of freedom the group's constraints leave,
// or -1 where the group is left out of that count, as SetSuppressDof
// asks, and where Doc.Describe says the count was not made.
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
// revolve or a step and repeat rotating, 3 is the angle in radians; for a linked
// part, 0 to 2 are its move in mm and 3 to 6 the quaternion (w, x, y, z) of its turn,
// which LinkGroup.Move, Rotate and SetOrientation set with the group regenerated.
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

// RelaxConstraints reports whether the group's solver keeps only the
// coincident constraints and lets the rest of the sketch move.
func (g Group) RelaxConstraints() (on bool) {
	g.d.read(func(sk *sketch.Sketch) { on = g.get(sk).RelaxConstraints })
	return on
}

// SetRelaxConstraints asks the group's solver to keep only the
// coincident constraints, and to treat every other constraint and
// dimension as a hint the sketch may not follow, or to require all of
// them again. A relaxed group solves where it did not, at the cost of
// the dimensions no longer holding.
func (g Group) SetRelaxConstraints(on bool) {
	g.setSolveOption(func(grp *sketch.Group) { grp.RelaxConstraints = on })
}

// AllowRedundant reports whether the group may solve with redundant
// constraints left in it.
func (g Group) AllowRedundant() (on bool) {
	g.d.read(func(sk *sketch.Sketch) { on = g.get(sk).AllowRedundant })
	return on
}

// SetAllowRedundant lets the group solve with redundant constraints in
// it, or asks for the surplus to be reported as conflicts to remove, as
// its solve-failure screen does.
func (g Group) SetAllowRedundant(on bool) {
	g.setSolveOption(func(grp *sketch.Group) { grp.AllowRedundant = on })
}

// SuppressDof reports whether the group is left out of the model's
// count of free parameters, which a large group makes quicker to
// regenerate.
func (g Group) SuppressDof() (on bool) {
	g.d.read(func(sk *sketch.Sketch) { on = g.get(sk).SuppressDofCalculation })
	return on
}

// SetSuppressDof leaves the group out of the model's count of free
// parameters, which a large group makes quicker to regenerate, or puts
// it back in, so that Group.Dof and the screen report the group again.
func (g Group) SetSuppressDof(on bool) {
	g.setSolveOption(func(grp *sketch.Group) { grp.SuppressDofCalculation = on })
}

// AllDimsReference reports whether every dimension in the group is
// measured without holding the sketch.
func (g Group) AllDimsReference() (on bool) {
	g.d.read(func(sk *sketch.Sketch) { on = g.get(sk).AllDimsReference })
	return on
}

// SetAllDimsReference turns every dimension in the group into a
// reference dimension, which is measured and shown but does not hold
// the sketch, or makes them constrain again.
func (g Group) SetAllDimsReference(on bool) {
	g.setSolveOption(func(grp *sketch.Group) { grp.AllDimsReference = on })
}

// ForceToMesh reports whether the group's NURBS surfaces are turned
// into a triangle mesh.
func (g Group) ForceToMesh() (on bool) {
	g.d.read(func(sk *sketch.Sketch) { on = g.get(sk).ForceToMesh })
	return on
}

// SetForceToMesh makes the group approximate its NURBS surfaces with a
// triangle mesh, which an import that carries no such surface needs,
// or leaves it to keep them. It does nothing where the model is a
// triangle mesh already.
func (g Group) SetForceToMesh(on bool) {
	g.setSolveOption(func(grp *sketch.Group) { grp.ForceToMesh = on })
}

// setSolveOption changes one of the group's solve options and
// regenerates, as the group's screen does.
func (g Group) setSolveOption(set func(grp *sketch.Group)) {
	g.d.change(func(sk *sketch.Sketch) {
		set(g.get(sk))
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
			if mode == SuppressSolid && !group.HasSolidSuppress() {
				fail("%s has no solid of its own to suppress: a %s makes none or edits the solid so far",
					groupDescription(group), kindName(group.Type))
			}
			if err := sk.SetGroupSuppressSolid(g.H, mode == SuppressSolid); err != nil {
				fail("%v", err)
			}
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

// Combine is how the group's solid combines with the solid so far:
// Union, Difference, Intersection or Assemble. A step and repeat, a
// mirror or a pattern takes its source group's mode, so this is the
// source's for those.
func (g Group) Combine() (how sketch.CombineAs) {
	g.d.read(func(sk *sketch.Sketch) { how = g.get(sk).MeshCombine })
	return how
}

// SetCombine says how the group's solid combines with the solid so
// far: Union, Difference, Intersection or Assemble. The extrusion keeps
// the direction the script gave it, where the window turns it round,
// because a script's depth already says which way a cut goes; a mode
// that doesn't exist fails. Only an extrusion, lathe, revolve, helix,
// link, sweep or loft has a mode of its own: the others take their
// source group's or edit the solid so far, and fail, as the group
// screen doesn't offer the choice for them.
func (g Group) SetCombine(how sketch.CombineAs) {
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		if !group.HasCombine() {
			fail("%s has no combine mode of its own: a %s takes its source group's or edits the solid so far",
				groupDescription(group), kindName(group.Type))
		}
		if err := sk.SetGroupCombine(g.H, how, false); err != nil {
			fail("%v", err)
		}
		sk.MarkGroupDirty(g.H, false)
	})
}

// Color is the colour of the group's solid, each of red, green and blue
// 0 to 255.
func (g Group) Color() (red, green, blue int) {
	g.d.read(func(sk *sketch.Sketch) {
		c := g.get(sk).Color
		red, green, blue = int(c.Red), int(c.Green), int(c.Blue)
	})
	return red, green, blue
}

// SetColor colours the group's solid; each of red, green and blue is
// 0 to 255. Its opacity stays. Only an extrusion, lathe, revolve,
// helix, sweep or loft makes faces of its own colour; a link brings its
// own, and a step and repeat, mirror, pattern, chamfer, fillet or shell
// keeps the colours of the solid it works on, so those fail.
func (g Group) SetColor(red, green, blue int) {
	if red < 0 || red > 255 || green < 0 || green > 255 || blue < 0 || blue > 255 {
		fail("a colour is 0 to 255 for each of red, green and blue, not %d, %d, %d", red, green, blue)
	}
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		if !group.HasColor() {
			fail("%s has no colour of its own: a %s doesn't colour its solid", groupDescription(group), kindName(group.Type))
		}
		if err := sk.SetGroupColor(g.H, core.NewRGBA(uint8(red), uint8(green), uint8(blue), 255)); err != nil {
			fail("%v", err)
		}
	})
}

// Opacity is how opaque the group's solid is, 0 (clear) to 1.
func (g Group) Opacity() (alpha float64) {
	g.d.read(func(sk *sketch.Sketch) { alpha = float64(g.get(sk).Color.AlphaF()) })
	return alpha
}

// SetOpacity sets how opaque the group's solid is, from 0 (clear) to 1,
// as the group screen's opacity does. Its colour stays. It fails for a
// value out of range and for a group with no colour of its own, which
// SetColor says.
func (g Group) SetOpacity(alpha float64) {
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		if !group.HasColor() {
			fail("%s has no colour of its own: a %s doesn't colour its solid", groupDescription(group), kindName(group.Type))
		}
		if err := sk.SetGroupOpacity(g.H, alpha); err != nil {
			fail("%v", err)
		}
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

// axisOf is the point and the direction to turn about: the point and
// vector given, or, with a zero point, the line segment vector from its
// first end.
func axisOf(sk *sketch.Sketch, point, vector Entity, what string) (origin, dir core.HEntity) {
	vector.get(sk)
	if point.H.V != 0 {
		point.get(sk)
	}
	origin, dir, err := sk.TurnAxis(point.H, vector.H)
	if err != nil {
		fail("%s: %v", what, err)
	}
	return origin, dir
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
