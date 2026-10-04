package api

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
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
	group := sk.GetGroup(g.H)
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
		if group := sk.GetGroup(g.H); group != nil {
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
	g.d.read(func(sk *sketch.Sketch) { ok = sk.GetGroup(g.H) != nil })
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
		g.get(sk).Color = core.NewRgbaColor(uint8(red), uint8(green), uint8(blue), 255)
		sk.MarkGroupDirty(g.H, false)
	})
}

// Copies is how many copies a step and repeat or a pattern makes.
func (g Group) Copies() (n int) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		if group.Type != sketch.GroupTranslate && group.Type != sketch.GroupRotate && group.Type != sketch.GroupPathPattern {
			fail("%s isn't a step and repeat or a pattern", groupDescription(group))
		}
		n = int(group.ValA)
	})
	return n
}

// SetCopies is how many copies a step and repeat or a pattern makes.
func (g Group) SetCopies(n int) {
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		if group.Type != sketch.GroupTranslate && group.Type != sketch.GroupRotate && group.Type != sketch.GroupPathPattern {
			fail("%s isn't a step and repeat or a pattern", groupDescription(group))
		}
		if n < 1 || n > 999 || (group.Type == sketch.GroupPathPattern && n < 2) {
			fail("can't repeat %d times", n)
		}
		group.ValA = float64(n)
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

// Mesh is the triangle mesh of the model up to and with this group.
// It is the sketch's own, and is replaced when the sketch
// regenerates.
func (g Group) Mesh() (m *srf.SMesh) {
	g.d.read(func(sk *sketch.Sketch) { m = displayMesh(sk, g.get(sk)) })
	return m
}

// Shell is the exact surfaces of the model up to and with this group;
// like Mesh, it is the sketch's own.
func (g Group) Shell() (s *srf.SShell) {
	g.d.read(func(sk *sketch.Sketch) { s = &g.get(sk).RunningShell })
	return s
}

// Volume is the volume in mm³ of the model up to and with this group.
func (g Group) Volume() (v float64) {
	g.d.read(func(sk *sketch.Sketch) { v = displayMesh(sk, g.get(sk)).CalculateVolume() })
	return v
}

// Triangles is how many triangles the model has up to and with this
// group.
func (g Group) Triangles() (n int) {
	g.d.read(func(sk *sketch.Sketch) { n = len(displayMesh(sk, g.get(sk)).Triangles) })
	return n
}

// Bounds are the corners of the box around the model up to and with
// this group.
func (g Group) Bounds() (min, max Vec) {
	g.d.read(func(sk *sketch.Sketch) {
		m := displayMesh(sk, g.get(sk))
		if m.IsEmpty() {
			return
		}
		max, min = m.GetBounding()
	})
	return min, max
}

// Check checks the solid of the model up to and with this group: it
// reports naked and self-intersecting edges, whether the Boolean
// failed, the face and triangle counts, and the volume, centre of mass
// and bounding box, in mm. Report.Sound() says whether the solid is
// closed, does not cross itself, and came from a Boolean that worked,
// and is false too when the group added no solid for a Problem (as an
// extrusion of a sketch that crosses itself), which Report.Problem
// holds; Report.Summary() says it in words. Analyze > Check Solid shows the
// same report.
func (g Group) Check() (r check.Report) {
	g.d.read(func(sk *sketch.Sketch) { r = check.Group(sk, g.get(sk)) })
	return r
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
		if at := sk.GetGroup(sk.Active); at != nil {
			for _, dep := range sk.DependenciesOf(&ng) {
				if on := sk.GetGroup(dep.Group); on != nil && on.Order > at.Order {
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

// Sketch3D adds a group to draw in freely in 3d, and makes it active.
func (d *Doc) Sketch3D(name string) Group {
	return d.add(func(*sketch.Sketch) sketch.Group {
		return newGroup(sketch.GroupDrawing3D, name, "sketch-in-3d")
	}, nil)
}

// SketchIn adds a group to draw in, in a workplane like wp: one of
// the reference workplanes, a workplane that was drawn, or the
// workplane of another group. The new group becomes active, and its
// workplane the one drawn in.
func (d *Doc) SketchIn(wp Entity, name string) Group {
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		g := newGroup(sketch.GroupDrawingWorkplane, name, "sketch-in-plane")
		ent := wp.get(sk)
		if !ent.IsWorkplane() {
			fail("%s isn't a workplane", wp.describe(sk))
		}
		if wp.H.IsFromRequest() {
			g.Subtype = sketch.GroupWorkplaneByPointOrtho
			g.Predef.Origin = ent.Point[0]
			g.Predef.Q = sk.MustGetEntity(ent.Normal).NormalGetNum(sk)
			return g
		}
		from := sk.GetGroup(wp.H.Group())
		g.Subtype = from.Subtype
		g.Predef.Origin = from.Predef.Origin
		switch from.Subtype {
		case sketch.GroupWorkplaneByLineSegments:
			g.Predef.EntityB = from.Predef.EntityB
			g.Predef.EntityC = from.Predef.EntityC
			g.Predef.SwapUV = from.Predef.SwapUV
			g.Predef.NegateU = from.Predef.NegateU
			g.Predef.NegateV = from.Predef.NegateV
		case sketch.GroupWorkplaneByPointOrtho:
			g.Predef.Q = from.Predef.Q
		case sketch.GroupWorkplaneByPointNormal:
			g.Predef.Q = from.Predef.Q
			g.Predef.EntityB = from.Predef.EntityB
		default:
			fail("%s has no workplane to copy", groupDescription(from))
		}
		return g
	}, nil)
}

// SketchAt adds a group to draw in, in a workplane through a point,
// with u to its right and v up; the nearest directions square to each
// other are taken. The new group becomes active.
func (d *Doc) SketchAt(origin Entity, u, v Vec, name string) Group {
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		g := newGroup(sketch.GroupDrawingWorkplane, name, "sketch-in-plane")
		if !origin.get(sk).IsPoint() {
			fail("%s isn't a point", origin.describe(sk))
		}
		if u.Magnitude() == 0 || u.Cross(v).Magnitude() == 0 {
			fail("the directions of a workplane must differ")
		}
		u = u.WithMagnitude(1)
		v = v.Minus(u.ScaledBy(v.Dot(u))).WithMagnitude(1)
		g.Subtype = sketch.GroupWorkplaneByPointOrtho
		g.Predef.Q = geom.QuaternionFromBasis(u, v)
		g.Predef.Origin = origin.H
		return g
	}, nil)
}

// workplaneNormal is the unit normal of the active workplane, which
// there must be.
func workplaneNormal(sk *sketch.Sketch, what string) (core.HEntity, Vec) {
	wp := sk.ActiveWorkplane()
	if core.IsFreeIn3D(wp) {
		fail("%s needs a workplane to be active: the sketch is made into a solid square to it", what)
	}
	return wp, sk.MustGetEntity(wp).NormalEntity(sk).NormalN(sk)
}

// Extrude makes a solid of the active group's sketch by extruding it
// by depth along the normal of its workplane, or against it for a
// negative depth. The depth is where the extrusion starts, as when it
// is dragged; constrain a point of it to keep it. The new group
// becomes active.
func (d *Doc) Extrude(depth float64, name string) Group {
	var n Vec
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		g := newGroup(sketch.GroupExtrude, name, "extrude")
		g.OpA = sk.Active
		g.Predef.EntityB, n = workplaneNormal(sk, "Extrude")
		g.Subtype = sketch.GroupOneSided
		return g
	}, func(sk *sketch.Sketch, hg core.HGroup) {
		// The parameters are the vector to the middle of the extrusion.
		v := n.ScaledBy(depth / 2)
		sk.MustGetParam(hg.Param(0)).Val = v.X
		sk.MustGetParam(hg.Param(1)).Val = v.Y
		sk.MustGetParam(hg.Param(2)).Val = v.Z
	})
}

// Draft is an extrusion's draft in degrees: its sides lean into the
// solid by that much from the sketch's plane, out when it is negative.
func (g Group) Draft() (degrees float64) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupExtrude, "Draft")
		degrees = group.ValC
	})
	return degrees
}

// SetDraft leans an extrusion's sides into the solid by degrees from
// the sketch's plane, so that the far end is smaller (larger when
// negative), as a moulded part's; 0 takes the draft away. The profile
// must be of lines and arcs, each arc meeting its neighbours tangent. A
// draft at which the sides meet is a problem on the group, and the
// extrusion is made without it. SetExpr with sketch.GroupValC gives it
// an expression.
func (g Group) SetDraft(degrees float64) {
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupExtrude, "SetDraft")
		if math.IsNaN(degrees) || math.Abs(degrees) >= 90 {
			fail("a draft is between -90 and 90 degrees, not %g", degrees)
		}
		group.ValC, group.ExprC = degrees, ""
		sk.MarkGroupDirty(group.H, false)
	})
}

// Twist is how far a sweep turns its profile about the path over the
// whole of it, in degrees.
func (g Group) Twist() (degrees float64) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupSweep, "Twist")
		degrees = group.ValC
	})
	return degrees
}

// SetTwist turns a sweep's profile about the path by degrees over the
// whole of it, evenly along its length: a square swept along a line and
// twisted by 90 is a twisted bar. A twisted sweep's surface is fitted
// to the true one within a tenth of the chord tolerance, as a sweep
// along a cubic is. SetExpr with sketch.GroupValC gives it an
// expression.
func (g Group) SetTwist(degrees float64) {
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupSweep, "SetTwist")
		if math.IsNaN(degrees) || math.IsInf(degrees, 0) {
			fail("a twist is a number of degrees, not %g", degrees)
		}
		group.ValC, group.ExprC = degrees, ""
		sk.MarkGroupDirty(group.H, false)
	})
}

// SweepUp is the direction a sweep holds its frames to, or zero when
// they turn as little as the path lets them.
func (g Group) SweepUp() (up Vec) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupSweep, "SweepUp")
		up = group.SweepUp
	})
	return up
}

// SetSweepUp holds a sweep's frames to the direction up, as far as the
// path's tangent allows, rather than turning them as little as the path
// lets them: a profile swept along a helix with up its axis keeps
// facing the axis, as a thread's does. The path must never run along
// up. Zero turns them least again.
func (g Group) SetSweepUp(up Vec) {
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupSweep, "SetSweepUp")
		if math.IsNaN(up.X+up.Y+up.Z) || math.IsInf(up.X+up.Y+up.Z, 0) {
			fail("an up direction is a vector of numbers, not %v", up)
		}
		if up.Magnitude() > 0 {
			up = up.WithMagnitude(1)
		}
		group.SweepUp = up
		sk.MarkGroupDirty(group.H, false)
	})
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

// Lathe makes a solid of the active group's sketch by turning it all
// the way around a line segment. The new group becomes active.
func (d *Doc) Lathe(axis Entity, name string) Group {
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		g := newGroup(sketch.GroupLathe, name, "lathe")
		workplaneNormal(sk, "Lathe")
		g.Predef.Origin, g.Predef.EntityB = axisOf(sk, axis, "Lathe")
		g.OpA = sk.Active
		return g
	}, nil)
}

// Revolve makes a solid of the active group's sketch by turning it by
// an angle in degrees around a line segment. The new group becomes
// active.
func (d *Doc) Revolve(axis Entity, degrees float64, name string) Group {
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		g := newGroup(sketch.GroupRevolve, name, "revolve")
		workplaneNormal(sk, "Revolve")
		g.Predef.Origin, g.Predef.EntityB = axisOf(sk, axis, "Revolve")
		g.OpA = sk.Active
		g.ValA = 2
		g.Subtype = sketch.GroupOneSided
		return g
	}, func(sk *sketch.Sketch, hg core.HGroup) {
		// The group turns its copies twice by twice the parameter.
		sk.MustGetParam(hg.Param(3)).Val = degrees * math.Pi / 180 / 4
	})
}

// Helix makes a solid of the active group's sketch by sweeping it
// around a line segment by an angle in degrees while moving it along
// the segment's direction by length, as a screw thread does: 360 and a
// length of the pitch make one turn. A negative angle turns the other
// way. The new group becomes active.
func (d *Doc) Helix(axis Entity, degrees, length float64, name string) Group {
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		g := newGroup(sketch.GroupHelix, name, "helix")
		workplaneNormal(sk, "Helix")
		g.Predef.Origin, g.Predef.EntityB = axisOf(sk, axis, "Helix")
		g.OpA = sk.Active
		g.ValA = 2
		g.Subtype = sketch.GroupOneSided
		return g
	}, func(sk *sketch.Sketch, hg core.HGroup) {
		// As for Revolve, and the copies move twice by the distance.
		sk.MustGetParam(hg.Param(3)).Val = degrees * math.Pi / 180 / 4
		sk.MustGetParam(hg.Param(7)).Val = length / 2
	})
}

// Translate repeats the model so far n times, each a step of delta
// from the one before. The new group becomes active.
func (d *Doc) Translate(n int, delta Vec, name string) Group {
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		g := newGroup(sketch.GroupTranslate, name, "translate")
		if n < 1 || n > 999 {
			fail("can't repeat %d times", n)
		}
		g.OpA = sk.Active
		g.ValA = float64(n)
		g.Subtype = sketch.GroupOneSided
		g.Predef.EntityB = sk.ActiveWorkplane()
		g.ActiveWorkplane = sk.ActiveWorkplane()
		return g
	}, func(sk *sketch.Sketch, hg core.HGroup) {
		sk.MustGetParam(hg.Param(0)).Val = delta.X
		sk.MustGetParam(hg.Param(1)).Val = delta.Y
		sk.MustGetParam(hg.Param(2)).Val = delta.Z
	})
}

// Rotate repeats the model so far n times, each turned by an angle in
// degrees from the one before, about an axis through a point: the
// direction of a line segment or of a normal, or, with no axis
// (Entity{}), the normal of the active workplane. The new group
// becomes active.
func (d *Doc) Rotate(n int, center, axis Entity, degrees float64, name string) Group {
	var at, dir Vec
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		g := newGroup(sketch.GroupRotate, name, "rotate")
		if n < 1 || n > 999 {
			fail("can't repeat %d times", n)
		}
		c := center.get(sk)
		if !c.IsPoint() {
			fail("%s isn't a point", center.describe(sk))
		}
		at = c.PointGetNum(sk)
		g.Predef.Origin = center.H
		if axis.H.V == 0 {
			wp, normal := workplaneNormal(sk, "Rotate about a point alone")
			g.Predef.EntityB = sk.MustGetEntity(wp).NormalEntity(sk).H
			g.ActiveWorkplane = wp
			dir = normal
		} else {
			a := axis.get(sk)
			if !a.HasVector() {
				fail("%s has no direction to turn about", axis.describe(sk))
			}
			g.Predef.EntityB = axis.H
			dir = a.VectorGetNum(sk).WithMagnitude(1)
		}
		g.OpA = sk.Active
		g.ValA = float64(n)
		g.Subtype = sketch.GroupOneSided
		return g
	}, func(sk *sketch.Sketch, hg core.HGroup) {
		for i, v := range []float64{at.X, at.Y, at.Z, degrees * math.Pi / 180, dir.X, dir.Y, dir.Z} {
			sk.MustGetParam(hg.Param(i)).Val = v
		}
	})
}

// BillOfMaterials is the parts the model places by linking files, one
// item a file: each link group places one, and a step and repeat, a
// mirror or a pattern of a link group its copies, but for one where the
// part already is. Groups left out of the model place none.
func (d *Doc) BillOfMaterials() (items []sketch.BOMItem) {
	d.read(func(sk *sketch.Sketch) { items = sk.BillOfMaterials() })
	return items
}

// ExportBOM writes the bill of materials to a CSV file: part, count,
// file and the groups that place them.
func (d *Doc) ExportBOM(path string) error {
	var b bytes.Buffer
	if err := sketch.WriteBOMCSV(&b, d.BillOfMaterials()); err != nil {
		return err
	}
	return os.WriteFile(path, b.Bytes(), 0o644)
}

// Link brings in another sketch file, a STEP file, or a mesh file (STL
// or OBJ), as
// a group, named after the file if name is empty. The new group becomes
// active. A STEP file comes in as a reference mesh, its faces
// tessellated, with a point at each vertex and at the centre of each
// circular edge for constraints to use; LinkInfo and LinkProblem say
// what was read.
func (d *Doc) Link(path, name string) (g Group, err error) {
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(error)
			if !ok {
				panic(r)
			}
			err = e
		}
	}()
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		g := newGroup(sketch.GroupLinked, name, "")
		g.MeshCombine = sketch.CombineAssemble
		abs, err := filepath.Abs(path)
		if err != nil {
			fail("%v", err)
		}
		g.LinkFile = abs
		if name == "" {
			stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			g.Name = strings.Map(func(r rune) rune {
				if r >= 0x80 || unicode.IsLetter(r) || unicode.IsDigit(r) {
					return r
				}
				return '-'
			}, stem)
		}
		return g
	}, nil), nil
}

// LinkStep links a STEP file as Link does, as a solid when asSolid is
// set: its faces exact, so the solid takes part in Booleans, fillets
// and the rest as a solid made here does. A file that can't be read as
// a solid comes in as a reference mesh, and LinkProblem says why.
func (d *Doc) LinkStep(path, name string, asSolid bool) (Group, error) {
	g, err := d.Link(path, name)
	if err != nil || !asSolid {
		return g, err
	}
	g.SetLinkAsSolid(true)
	return g, nil
}

// LinkAsSolid reports whether a link group reads its STEP file as a
// solid rather than as a reference mesh.
func (g Group) LinkAsSolid() (on bool) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupLinked, "LinkAsSolid")
		on = group.LinkAsSolid
	})
	return on
}

// SetLinkAsSolid reads a link group's STEP file as a solid, or as a
// reference mesh, and reads it again.
func (g Group) SetLinkAsSolid(on bool) {
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupLinked, "SetLinkAsSolid")
		group.LinkAsSolid = on
		if sk.ReloadAllLinked != nil {
			sk.ReloadAllLinked(sk)
		}
		// A mesh forces its group to one; a solid that was one no
		// longer.
		if on && len(group.ImpShell.Surface) > 0 {
			group.ForceToMesh = false
		}
		sk.MarkGroupDirty(group.H, false)
	})
}

// LinkInfo describes the file a link group links, a line each, as the
// group's screen shows it: for a STEP file its schema and unit, how
// many shapes, faces and triangles it gave, and the chord tolerance it
// was tessellated to. A sketch or STL file gives none.
func (g Group) LinkInfo() (lines []string) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupLinked, "LinkInfo")
		lines = append(lines, group.LinkInfo...)
	})
	return lines
}

// LinkProblem says what of the file a link group links could not be
// read, such as the faces of a STEP file on a kind of surface the
// reader does not support, or that the file holds no solid it can
// read; "" when all of it was.
func (g Group) LinkProblem() (msg string) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupLinked, "LinkProblem")
		msg = group.LinkProblem
	})
	return msg
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
			if sk.GetConstraint(hc) != nil {
				constraints = append(constraints, Constraint{g.d, hc})
			}
		}
	})
	return constraints
}

// DependsOn lists the groups this group refers to, in history order:
// the group it extrudes or copies, the workplane it is drawn in, and the
// groups whose entities its sketch and constraints name. The references
// are left out.
func (g Group) DependsOn() (groups []Group) {
	g.d.read(func(sk *sketch.Sketch) {
		g.get(sk)
		for _, dep := range sk.GroupDependencies(g.H) {
			if dep.Group.V != core.HGroupReferences {
				groups = append(groups, Group{g.d, dep.Group})
			}
		}
	})
	return groups
}

// UsedBy lists the groups that refer to this group directly, in
// history order.
func (g Group) UsedBy() (groups []Group) {
	g.d.read(func(sk *sketch.Sketch) {
		g.get(sk)
		for _, h := range sk.GroupDependents(g.H) {
			groups = append(groups, Group{g.d, h})
		}
	})
	return groups
}

// MoveBefore moves the group in the history to just before other, as
// one undo step. A move that would put a group ahead of a group it
// uses is refused with an error naming both, and nothing changes:
// an extrusion can't move before its sketch, nor a sketch after its
// extrusion. The references stay first.
func (g Group) MoveBefore(other Group) error {
	if other.H.V == 0 {
		fail("no group to move before")
	}
	other.d = g.d
	g.d.read(func(sk *sketch.Sketch) { other.get(sk) })
	return g.move(other.H)
}

// MoveToEnd moves the group to the end of the history, as MoveBefore
// does. Only this group moves: the sketches it uses stay where they
// are, as they come before it already. MoveToEndWithSources moves them
// along.
func (g Group) MoveToEnd() error {
	return g.move(core.HGroup{})
}

// OwnSources lists the group with the groups only it needs, in history
// order: those it uses, directly or through each other, that no other
// group uses, as an extrusion's sketch. These are what
// MoveToEndWithSources moves.
func (g Group) OwnSources() (groups []Group) {
	g.d.read(func(sk *sketch.Sketch) {
		g.get(sk)
		for _, h := range sk.OwnSources(g.H) {
			groups = append(groups, Group{g.d, h})
		}
	})
	return groups
}

// MoveToEndWithSources moves the group to the end of the history
// together with its OwnSources, keeping their order, as one undo step:
// an extrusion moves with its sketch. It returns the groups moved, in
// their new order. A move that would put one of them ahead of a group
// it uses, or another group ahead of one of them that it uses, is
// refused with an error naming both, and nothing changes.
func (g Group) MoveToEndWithSources() ([]Group, error) {
	return g.moveWithSources(core.HGroup{})
}

// MoveWithSourcesBefore moves the group with its OwnSources to just
// before other, as MoveToEndWithSources moves them to the end.
func (g Group) MoveWithSourcesBefore(other Group) ([]Group, error) {
	if other.H.V == 0 {
		fail("no group to move before")
	}
	other.d = g.d
	g.d.read(func(sk *sketch.Sketch) { other.get(sk) })
	return g.moveWithSources(other.H)
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
