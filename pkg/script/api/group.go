package api

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/sketch"
	"github.com/spacesolve/spacesolve/pkg/srf"
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
// "extrude", "lathe", "revolve", "helix", "rotate", "translate" or
// "link".
func (g Group) Kind() string {
	switch g.Type() {
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

// SetSuppress leaves the group's solid out of the model, or puts it
// back.
func (g Group) SetSuppress(suppress bool) {
	g.d.change(func(sk *sketch.Sketch) {
		g.get(sk).Suppress = suppress
		sk.MarkGroupDirty(g.H, false)
	})
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

// SetCopies is how many copies a step and repeat makes.
func (g Group) SetCopies(n int) {
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		if group.Type != sketch.GroupTranslate && group.Type != sketch.GroupRotate {
			fail("%s isn't a step and repeat", groupDescription(group))
		}
		if n < 1 || n > 999 {
			fail("can't repeat %d times", n)
		}
		group.ValA = float64(n)
		sk.MarkGroupDirty(g.H, false)
	})
}

// Delete deletes the group and what depends on it.
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
		hg := d.h.AddGroup(build(sk))
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
		sk.MustGetParam(hg.Param(3)).Val = degrees * math.Pi / 180
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

// Link brings in another sketch file, or an STL file, as a group,
// named after the file if name is empty. The new group becomes
// active.
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
