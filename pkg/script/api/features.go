package api

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// FeaturesMenu holds the calls of the Features menu: the groups that
// start a sketch or make a solid. Each adds a group after the active
// one and makes it active. A script reaches it as doc.Features.
type FeaturesMenu struct{ d *Doc }

// BrokenReferences lists what the last regeneration that deleted
// something deleted, as Edit > Broken References shows it. The list
// stays until a regeneration deletes something else, an undo or redo,
// or ClearBrokenReferences. Undo brings the deleted items back.
func (m FeaturesMenu) BrokenReferences() (refs []BrokenReference) {
	d := m.d
	d.read(func(sk *sketch.Sketch) {
		for _, b := range sk.BrokenReferences {
			refs = append(refs, BrokenReference{Group: Group{d, b.Group}, Item: b.Item, Missing: b.Missing, Text: b.String()})
		}
	})
	return refs
}

// ClearBrokenReferences empties the list BrokenReferences gives.
func (m FeaturesMenu) ClearBrokenReferences() {
	d := m.d
	d.h.Run(func() { d.h.Sketch().ClearBrokenReferences() })
}

// Sketch3D adds a group to draw in freely in 3d, and makes it active.
func (m FeaturesMenu) Sketch3D(name string) Group {
	d := m.d
	return d.add(func(*sketch.Sketch) sketch.Group {
		return newGroup(sketch.GroupDrawing3D, name, "sketch-in-3d")
	}, nil)
}

// SketchIn adds a group to draw in, in a workplane like wp: one of
// the reference workplanes, a workplane that was drawn, or the
// workplane of another group. The new group becomes active, and its
// workplane the one drawn in.
func (m FeaturesMenu) SketchIn(wp Entity, name string) Group {
	d := m.d
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		g := newGroup(sketch.GroupDrawingWorkplane, name, "sketch-in-plane")
		ent := wp.get(sk)
		if !ent.IsWorkplane() {
			fail("%s isn't a workplane", wp.describe(sk))
		}
		if wp.H.IsFromRequest() {
			g.Subtype = sketch.GroupWorkplaneByPointOrtho
			g.Predef.Origin = ent.Point[0]
			g.Predef.Q = sk.MustEntity(ent.Normal).NormalGetNum(sk)
			return g
		}
		from := sk.Group(wp.H.Group())
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
		case sketch.GroupWorkplaneByFace, sketch.GroupWorkplaneByDatum:
			g.Predef.Q, g.Predef.At = from.Predef.Q, from.Predef.At
			g.SolidFaces = slices.Clone(from.SolidFaces)
			g.SolidEdges = slices.Clone(from.SolidEdges)
		default:
			fail("%s has no workplane to copy", groupDescription(from))
		}
		return g
	}, nil)
}

// SketchAt adds a group to draw in, in a workplane through a point,
// with u to its right and v up; the nearest directions square to each
// other are taken. The new group becomes active.
func (m FeaturesMenu) SketchAt(origin Entity, u, v Vec, name string) Group {
	d := m.d
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

// SketchOnLines adds a group to draw in, in a workplane through a point
// and parallel to two line segments, as the window makes one from a
// point and two line segments: u runs along lineA and v along lineB, each from its start to its end,
// and the workplane follows the point and the lines when they move.
// The lines must not be parallel. The new group becomes active, and
// its workplane the one drawn in.
func (m FeaturesMenu) SketchOnLines(origin, lineA, lineB Entity, name string) Group {
	d := m.d
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		a, b := lineA.must(sk, "a line segment", isLine), lineB.must(sk, "a line segment", isLine)
		// A line's vector runs from its end to its start; the directions it was drawn in, as the view, keep lineA as u and lineB as v.
		right := sk.MustEntity(a).VectorGetNum(sk).ScaledBy(-1)
		up := sk.MustEntity(b).VectorGetNum(sk).ScaledBy(-1)
		g, err := sk.NewSketchOnLines(origin.must(sk, "a point", isPoint), a, b, right, up, name)
		if err != nil {
			fail("%v", err)
		}
		return g
	}, nil)
}

// SketchOnNormal adds a group to draw in, in a workplane through a
// point and square to a normal (an entity's Normal, or a workplane,
// whose normal is taken), as the window makes one from a point and a
// normal; the workplane follows the normal when it turns. The new
// group becomes active, and its workplane the one drawn in.
func (m FeaturesMenu) SketchOnNormal(origin, normal Entity, name string) Group {
	d := m.d
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		g, err := sk.NewSketchOnNormal(origin.must(sk, "a point", isPoint), normal.get(sk).H, name)
		if err != nil {
			fail("%v", err)
		}
		return g
	}, nil)
}

// SketchOnFace adds a group to draw in, in a workplane lying in a flat
// face of the solid so far, as Solid.FaceAt or Solid.Face give it: its
// normal out of the solid, its origin on a corner of the face and u
// along an edge of it (for a face with no straight edge, as a
// cylinder's cap, the origin at the centre of its circle). The
// workplane follows the face when an edit upstream moves it; when the
// face is gone, it stays where it was and Problem says so. The new
// group becomes active, and its workplane the one drawn in.
func (m FeaturesMenu) SketchOnFace(face SolidFace, name string) Group {
	d := m.d
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		g, err := sk.NewSketchOnFace(face.id, name)
		if err != nil {
			fail("%v", err)
		}
		return g
	}, nil)
}

// Extrude makes a solid of the active group's sketch by extruding it
// by depth along the normal of its workplane, or against it for a
// negative depth. The depth is where the extrusion starts, as when it
// is dragged; constrain a point of it to keep it. The new group
// becomes active.
func (m FeaturesMenu) Extrude(depth float64, name string) Group {
	d := m.d
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
		sk.SeedParam(hg.Param(0), v.X)
		sk.SeedParam(hg.Param(1), v.Y)
		sk.SeedParam(hg.Param(2), v.Z)
	})
}

// Lathe makes a solid of the active group's sketch by turning it all
// the way around a line segment. The new group becomes active.
func (m FeaturesMenu) Lathe(axis Entity, name string) Group {
	return m.LatheAbout(Entity{}, axis, name)
}

// LatheAbout is Lathe about an axis through a point and parallel to a
// vector, a line segment or a normal, as the window takes a point and
// a line segment or normal.
func (m FeaturesMenu) LatheAbout(point, vector Entity, name string) Group {
	d := m.d
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		g := newGroup(sketch.GroupLathe, name, "lathe")
		workplaneNormal(sk, "Lathe")
		g.Predef.Origin, g.Predef.EntityB = axisOf(sk, point, vector, "Lathe")
		g.OpA = sk.Active
		return g
	}, nil)
}

// Revolve makes a solid of the active group's sketch by turning it by
// an angle in degrees around a line segment. The new group becomes
// active.
func (m FeaturesMenu) Revolve(axis Entity, degrees float64, name string) Group {
	return m.RevolveAbout(Entity{}, axis, degrees, name)
}

// RevolveAbout is Revolve about an axis through a point and parallel to
// a vector, a line segment or a normal.
func (m FeaturesMenu) RevolveAbout(point, vector Entity, degrees float64, name string) Group {
	d := m.d
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		g := newGroup(sketch.GroupRevolve, name, "revolve")
		workplaneNormal(sk, "Revolve")
		g.Predef.Origin, g.Predef.EntityB = axisOf(sk, point, vector, "Revolve")
		g.OpA = sk.Active
		g.ValA = 2
		g.Subtype = sketch.GroupOneSided
		return g
	}, func(sk *sketch.Sketch, hg core.HGroup) {
		// The group turns its copies twice by twice the parameter.
		sk.SeedParam(hg.Param(3), degrees*math.Pi/180/4)
	})
}

// Helix makes a solid of the active group's sketch by sweeping it
// around a line segment by an angle in degrees while moving it along
// the segment's direction by length, as a screw thread does: 360 and a
// length of the pitch make one turn. A negative angle turns the other
// way. The new group becomes active.
func (m FeaturesMenu) Helix(axis Entity, degrees, length float64, name string) Group {
	return m.HelixAbout(Entity{}, axis, degrees, length, name)
}

// HelixAbout is Helix about an axis through a point and parallel to a
// vector, a line segment or a normal.
func (m FeaturesMenu) HelixAbout(point, vector Entity, degrees, length float64, name string) Group {
	d := m.d
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		g := newGroup(sketch.GroupHelix, name, "helix")
		workplaneNormal(sk, "Helix")
		g.Predef.Origin, g.Predef.EntityB = axisOf(sk, point, vector, "Helix")
		g.OpA = sk.Active
		g.ValA = 2
		g.Subtype = sketch.GroupOneSided
		return g
	}, func(sk *sketch.Sketch, hg core.HGroup) {
		// As for Revolve, and the copies move twice by the distance.
		sk.SeedParam(hg.Param(3), degrees*math.Pi/180/4)
		sk.SeedParam(hg.Param(7), length/2)
	})
}

// Translate repeats the model so far n times, each a step of delta
// from the one before. The new group becomes active.
func (m FeaturesMenu) Translate(n int, delta Vec, name string) Group {
	d := m.d
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
		// Copy a is moved 2a times the parameters (stepTransform), so
		// they hold half a step.
		sk.SeedParam(hg.Param(0), delta.X/2)
		sk.SeedParam(hg.Param(1), delta.Y/2)
		sk.SeedParam(hg.Param(2), delta.Z/2)
	})
}

// Rotate repeats the model so far n times, each turned by an angle in
// degrees from the one before, about an axis through a point: the
// direction of a line segment or of a normal, or, with no axis
// (Entity{}), the normal of the active workplane. The new group
// becomes active.
func (m FeaturesMenu) Rotate(n int, center, axis Entity, degrees float64, name string) Group {
	d := m.d
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
			g.Predef.EntityB = sk.MustEntity(wp).NormalEntity(sk).H
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
		// As for Revolve: copy a turns by the quaternion of 2a times
		// parameter 3, which is 4a times it, so it holds a quarter step.
		for i, v := range []float64{at.X, at.Y, at.Z, degrees * math.Pi / 180 / 4, dir.X, dir.Y, dir.Z} {
			sk.SeedParam(hg.Param(i), v)
		}
	})
}

// Mirror reflects the active group in plane, a workplane or a planar
// face: its entities are copied reflected, and its solid is reflected
// and joined with the original, combined with the model before it as
// the active group's solid is, so a mirrored hole is cut too. The
// copies follow the plane when it moves. The new group becomes
// active; SetKeepOriginal(false) leaves the original out of the
// solid.
func (m FeaturesMenu) Mirror(plane Entity, name string) Group {
	d := m.d
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		g, err := sk.NewMirror(plane.get(sk).H, name)
		if err != nil {
			fail("%v", err)
		}
		return g
	}, nil)
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
func (m FeaturesMenu) Chamfer(edges []Entity, distance float64, name string) Group {
	d := m.d
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
// it, as Features > Pattern Along Path does. The path starts at its
// end nearest the solid. A path that closes, a circle alone or pieces
// that come back to their start meeting it tangent, has the copies
// shared out evenly round it, starting at its point nearest the
// solid. Copies and SetCopies read and change the
// count; the sketch's entities are not copied.
func (m FeaturesMenu) PatternAlongPath(path []Entity, copies int, name string) Group {
	d := m.d
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

// Sweep moves the closed loops of the active group, a sketch in a
// workplane, along path, line segments, arcs and cubics joined end to
// end and meeting tangent, as a pipe follows its bends. The sweep
// starts at the end of the path nearest the sketch's plane, with the
// sketch where it is drawn, and keeps its attitude to the path, turning
// as little as the path lets it. Along lines and arcs its surfaces are
// exact; along cubics they are fitted within a tenth of the chord
// tolerance (see also SetTwist and SetSweepUp). Two line segments may
// meet at a corner, where the sweep is mitred: each piece cut by the
// plane that halves the corner. A path that closes, a circle or pieces
// that come back to where they began, is swept round from where the
// sketch's plane crosses it, nearest the sketch, back to the sketch,
// with no caps: a ring, or a frame mitred at its corners; the sketch
// must cross the path where it is smooth, and a twist must be whole
// turns. The new group becomes active.
func (m FeaturesMenu) Sweep(path []Entity, name string) Group {
	d := m.d
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

// SweepEdges is Sweep along a chain of edges of the solid so far, by
// name (Solid, SolidEdge), and the line segments, arcs and cubics of
// path with them, one chain: a straight edge is a line, a circular one
// an arc, any other a curve as a cubic is. The sweep keeps the edges'
// names, so it follows them when the model upstream changes.
func (m FeaturesMenu) SweepEdges(edges []SolidEdge, path []Entity, name string) Group {
	d := m.d
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		hs := make([]core.HEntity, len(path))
		for i, e := range path {
			hs[i] = e.get(sk).H
		}
		names := make([]topo.Name, len(edges))
		for i, e := range edges {
			names[i] = e.name
		}
		g, err := sk.NewSweepAlong(hs, names, name)
		if err != nil {
			fail("%v", err)
		}
		return g
	}, nil)
}

// Thread makes a screw thread on a circle or an arc, about its axis
// and from its plane, as Features > Screw Thread does: four groups,
// a sketch and a lathe that turn the part down (or bore it), and a
// ridge sketch and a helix that sweeps it, then with LevelStarts a
// ridge sketch and a helix per further start; a Cosmetic thread, a
// sketch and, where the part is cut, a lathe. It returns the helix
// group (a cosmetic thread's sketch), which keeps what made the thread
// (AsThread); the thread's last
// group becomes active, and AsThread().Finished() is that group, whose
// solid is the part with the whole thread (with level starts the helix
// group's has the first start only). Its Check says whether the solid
// came out sound at the chord tolerance of the moment. A thread longer
// than the solid so far reaches along it, run off it the other way (see
// Reverse), or on a rod thinner than its root, is refused, saying so;
// one whose part is made shorter later says so as the Problem of the
// group that keeps it.
func (m FeaturesMenu) Thread(circle Entity, o ThreadOptions) (g Group) {
	d := m.d
	d.change(func(sk *sketch.Sketch) {
		if d.h.Sketch().RegenerationPending() {
			d.h.Regenerate()
			sk = d.h.Sketch()
		}
		c := circle.get(sk)
		if !c.IsCircle() {
			fail("a thread is made on a circle or an arc, and %s isn't one", circle.describe(sk))
		}
		opts := o.sketchOptions(sk, 2*c.CircleGetRadiusNum(sk))
		res, err := sk.AddThread(circle.H, opts)
		if err != nil {
			fail("%v", err)
		}
		hs, err := sk.ThreadGroups(res.Group)
		if err != nil {
			fail("%v", err)
		}
		d.h.Activate(hs[len(hs)-1])
		g = Group{d, res.Group}
	})
	return g
}

// ChamferSolid cuts edges of the solid so far, by name, at distance mm
// from each edge on both faces: any straight edge between two flat
// faces, such as one a Boolean made, or a circular one where a flat
// face meets a cylinder or a cone, picked from Solid. The chamfer
// keeps the edges' names, so it follows them when the model upstream
// changes, and reports when one no longer resolves. The new group
// becomes active.
func (m FeaturesMenu) ChamferSolid(edges []SolidEdge, distance float64, name string) Group {
	d := m.d
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		names := make([]topo.Name, len(edges))
		for i, e := range edges {
			names[i] = e.name
		}
		g, err := sk.NewChamfer(nil, names, distance, name)
		if err != nil {
			fail("%v", err)
		}
		return g
	}, nil)
}

// Fillet rounds edges of the solid so far with radius mm: line segments
// along edges between two flat faces, as the copies of the sketch at
// the ends of an extrusion and the lines along its sides are. Each edge
// must end at corners of three flat faces. An edge that sticks out is
// rounded off, and one that goes in, as a pocket's, is filled. Two
// rounded edges that meet at a corner are mitred, their cylinders
// meeting along an ellipse, the one that turns further going on past it
// when the faces don't lie alike about them; three get a sphere. A
// circle or arc where a flat face meets a cylinder or a cone about the
// circle's axis, as an extruded circle's, a hole's rim, a boss's foot or
// a lathed frustum's top, is rounded all round by a torus. An edge
// takes with it the edges it meets smoothly, as round a rectangle whose
// corners are rounded. An edge may have a radius of its own
// (SetEdgeRadius), but edges that meet at a corner need one. The faces
// a fillet makes are exact cylinders, spheres and tori, and their edges
// have names. The new group becomes active.
func (m FeaturesMenu) Fillet(edges []Entity, radius float64, name string) Group {
	d := m.d
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		hs := make([]core.HEntity, len(edges))
		for i, e := range edges {
			hs[i] = e.get(sk).H
		}
		g, err := sk.NewFillet(hs, nil, radius, name)
		if err != nil {
			fail("%v", err)
		}
		return g
	}, nil)
}

// FilletSolid rounds edges of the solid so far, by name, with radius
// mm, as Fillet does: any straight edge between two flat faces, such
// as one a Boolean made, or a circular one, picked from Solid. The fillet
// keeps the names, so it follows the edges when the model upstream
// changes. The new group becomes active.
func (m FeaturesMenu) FilletSolid(edges []SolidEdge, radius float64, name string) Group {
	d := m.d
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		names := make([]topo.Name, len(edges))
		for i, e := range edges {
			names[i] = e.name
		}
		g, err := sk.NewFillet(nil, names, radius, name)
		if err != nil {
			fail("%v", err)
		}
		return g
	}, nil)
}

// Shell hollows the solid so far to walls thick mm, open where the faces
// are taken away: face entities, as Group.Faces gives an extrusion's.
// This version hollows solids of flat faces, prisms of lines and arcs
// open at their ends, and solids made by one lathe or revolve of lines
// and arcs, and refuses a thickness at which walls meet. The faces of the cavity have names. The new group
// becomes active.
func (m FeaturesMenu) Shell(faces []Entity, thick float64, name string) Group {
	d := m.d
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		ids := make([]srf.FaceID, len(faces))
		for i, f := range faces {
			e := f.get(sk)
			if !e.IsFace() {
				fail("entity %x is not a face", e.H.V)
			}
			ids[i] = srf.FaceID(e.H.V)
		}
		g, err := sk.NewShell(ids, thick, name)
		if err != nil {
			fail("%v", err)
		}
		return g
	}, nil)
}

// ShellSolid hollows the solid so far as Shell does, taking away faces
// picked from Solid, such as one a Boolean or a loft made.
func (m FeaturesMenu) ShellSolid(faces []SolidFace, thick float64, name string) Group {
	d := m.d
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		ids := make([]srf.FaceID, len(faces))
		for i, f := range faces {
			ids[i] = f.id
		}
		g, err := sk.NewShell(ids, thick, name)
		if err != nil {
			fail("%v", err)
		}
		return g
	}, nil)
}

// Loft joins the closed loops of two sketches in workplanes, from and
// to, with ruled faces, curve by curve, and closes the ends with the
// sketches' planes: each loop must be one closed loop; a circle counts
// as four curves, and a loop with fewer curves than the other has its
// longest split until they match. The solid joins the solid so far as
// an extrusion's does. The new group becomes active.
func (m FeaturesMenu) Loft(from, to Group, name string) Group {
	d := m.d
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		g, err := sk.NewLoft(from.get(sk).H, to.get(sk).H, name)
		if err != nil {
			fail("%v", err)
		}
		return g
	}, nil)
}

// LoftThrough joins the closed loops of sketches in workplanes, three
// or more, in order, by one smooth surface through them all, and closes
// the ends with the first and last sketches' planes. Loops of fewer
// curves have their longest split until all match, and each loop after
// the first starts at the curve nearest where the one before it does.
// The new group becomes active.
func (m FeaturesMenu) LoftThrough(profiles []Group, name string) Group {
	d := m.d
	return d.add(func(sk *sketch.Sketch) sketch.Group {
		hs := make([]core.HGroup, len(profiles))
		for i, p := range profiles {
			hs[i] = p.get(sk).H
		}
		g, err := sk.NewSmoothLoft(hs, name)
		if err != nil {
			fail("%v", err)
		}
		return g
	}, nil)
}
