package api

import (
	"math"
	"sort"
	"strconv"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// Solid is the solid at a group, as its topology sees it: faces, the
// edges where two faces meet, and the vertices where edges end. Its
// edges have names that outlive a regeneration, so a script can keep
// one and use it after changing the model upstream.
type Solid struct {
	d *Doc
	g core.HGroup
}

// SolidFace is a face of a solid: its identity, the sketch entity that
// made it or the group whose surface it is, and which of the faces
// with that identity it is. A face may be several patches joined
// smoothly, as an extruded circle's side is four.
type SolidFace struct {
	s  Solid
	id srf.FaceID
	n  int
}

// SolidEdge is an edge of a solid, by name.
type SolidEdge struct {
	s    Solid
	name topo.Name
}

// SolidVertex is a vertex of a solid, by position.
type SolidVertex struct {
	s Solid
	p Vec
}

// Solid is the solid at this group: the solid of every group up to it.
func (g Group) Solid() Solid {
	g.d.read(func(sk *sketch.Sketch) { g.get(sk) })
	return Solid{d: g.d, g: g.H}
}

// topology is the solid's topology; it fails when there is no solid
// with surfaces.
func (s Solid) topology(sk *sketch.Sketch) *topo.Topology {
	g := sk.GetGroup(s.g)
	if g == nil {
		fail("the group of the solid is gone")
	}
	src := sk.SolidAt(s.g)
	if src == nil || len(src.RunningShell.Surface) == 0 {
		fail("there is no solid at %s", groupDescription(g))
	}
	if src.IsForcedToMesh(sk) {
		fail("the solid at %s is a triangle mesh, which has no faces and edges to name", groupDescription(g))
	}
	tp, _ := src.Topology()
	return tp
}

// face is the SolidFace of sheet sh.
func (s Solid) face(tp *topo.Topology, sh int) SolidFace {
	id := tp.Sheets[sh].ID
	n := 0
	for _, g := range tp.SheetsWithID(id) {
		if g == sh {
			break
		}
		n++
	}
	return SolidFace{s: s, id: id, n: n}
}

// edge is the SolidEdge that edge e is a piece of, named by its lead
// piece.
func (s Solid) edge(tp *topo.Topology, e int) (SolidEdge, bool) {
	if tp.Edges[e].Seam {
		return SolidEdge{}, false
	}
	n, err := tp.NameOf(tp.Lead(e))
	if err != nil {
		return SolidEdge{}, false
	}
	return SolidEdge{s: s, name: n}, true
}

// Faces are the solid's faces.
func (s Solid) Faces() (faces []SolidFace) {
	s.d.read(func(sk *sketch.Sketch) {
		tp := s.topology(sk)
		for sh := range tp.Sheets {
			faces = append(faces, s.face(tp, sh))
		}
	})
	return faces
}

// Edges are the solid's edges that lie between two faces, in the
// order the topology finds them. The seams between the patches of a
// face are none of them, and an edge they cut, as a cylinder's rim, is
// one.
func (s Solid) Edges() (edges []SolidEdge) {
	s.d.read(func(sk *sketch.Sketch) {
		tp := s.topology(sk)
		for _, e := range tp.SolidEdges() {
			if se, ok := s.edge(tp, e); ok {
				edges = append(edges, se)
			}
		}
	})
	return edges
}

// Vertices are the solid's vertices.
func (s Solid) Vertices() (vs []SolidVertex) {
	s.d.read(func(sk *sketch.Sketch) {
		tp := s.topology(sk)
		for v := range tp.Vertices {
			if tp.SolidVertex(v) {
				vs = append(vs, SolidVertex{s: s, p: tp.Vertices[v].P})
			}
		}
	})
	return vs
}

// Edge is the edge a name, as SolidEdge.Name writes it, names; it fails
// when the name doesn't resolve, saying why. A name may leave out the
// count, "edge A B N" for "edge A B N of M"; the edge has it.
func (s Solid) Edge(name string) (e SolidEdge) {
	n, err := topo.ParseName(name)
	if err != nil {
		fail("%v", err)
	}
	s.d.read(func(sk *sketch.Sketch) {
		tp := s.topology(sk)
		i, err := tp.Resolve(n)
		if err != nil {
			fail("%s: %v", sk.SolidEdgeName(n), err)
		}
		// The edge's own name, which counts the edges between its faces
		// as picking it does, whether or not the name given did.
		if own, err := tp.NameOf(i); err == nil {
			n = own
		}
	})
	return SolidEdge{s: s, name: n}
}

// Face is the face with identity id, as SolidFace.ID gives it: the
// face of which a patch has that identity, whichever patch it is, so a
// face of several patches can be named by any of theirs (an extruded
// circle's side by any quarter's). It fails for an identity no patch
// of the solid has (roadmap 14.1).
func (s Solid) Face(id uint32) (f SolidFace) {
	s.d.read(func(sk *sketch.Sketch) {
		tp := s.topology(sk)
		for _, pf := range tp.FacesWithID(srf.FaceID(id)) {
			f = s.face(tp, tp.SheetOf(pf))
			return
		}
		fail("no face of the solid has the identity %#x", id)
	})
	return f
}

// EdgeAt is the edge nearest the point p, which must be within a
// hundredth of a millimetre of it.
func (s Solid) EdgeAt(p Vec) (e SolidEdge) {
	s.d.read(func(sk *sketch.Sketch) {
		tp := s.topology(sk)
		i, d := tp.EdgeAt(p, true)
		if i < 0 || d > 0.01 {
			fail("no edge of the solid passes through (%g, %g, %g)", p.X, p.Y, p.Z)
		}
		var ok bool
		if e, ok = s.edge(tp, i); !ok {
			fail("the edge through (%g, %g, %g) does not lie between two faces", p.X, p.Y, p.Z)
		}
	})
	return e
}

// FaceAt is the face p lies on, within a hundredth of a millimetre.
func (s Solid) FaceAt(p Vec) (f SolidFace) {
	s.d.read(func(sk *sketch.Sketch) {
		tp := s.topology(sk)
		i, d := tp.FaceAt(p)
		if i < 0 || d > 0.01 {
			fail("no face of the solid passes through (%g, %g, %g)", p.X, p.Y, p.Z)
		}
		f = s.face(tp, tp.SheetOf(i))
	})
	return f
}

// EdgesBetween are the edges where faces with the identities of a and
// b meet, in the order their names number them: both pieces of a face
// another cuts in two count.
func (s Solid) EdgesBetween(a, b SolidFace) (edges []SolidEdge) {
	s.d.read(func(sk *sketch.Sketch) {
		tp := s.topology(sk)
		var found []topo.Name
		for _, e := range tp.SolidEdges() {
			sh := tp.EdgeSheets(e)
			if sh[0] < 0 || sh[1] < 0 {
				continue
			}
			ia, ib := tp.Sheets[sh[0]].ID, tp.Sheets[sh[1]].ID
			if ia == a.id && ib == b.id || ia == b.id && ib == a.id {
				if se, ok := s.edge(tp, e); ok {
					found = append(found, se.name)
				}
			}
		}
		sort.Slice(found, func(i, j int) bool { return found[i].Less(found[j]) })
		for _, n := range found {
			edges = append(edges, SolidEdge{s: s, name: n})
		}
	})
	return edges
}

// EdgesAlong are the straight edges parallel to dir, either way.
func (s Solid) EdgesAlong(dir Vec) (edges []SolidEdge) {
	if dir.Magnitude() == 0 {
		fail("a direction of no length")
	}
	dir = dir.WithMagnitude(1)
	s.d.read(func(sk *sketch.Sketch) {
		tp := s.topology(sk)
		for _, e := range tp.SolidEdges() {
			if !tp.Straight(e) || len(tp.Chain(e)) > 1 {
				continue
			}
			pts := tp.Edges[e].Points
			d := pts[len(pts)-1].Minus(pts[0]).WithMagnitude(1)
			if math.Abs(d.Dot(dir)) > 1-1e-9 {
				if se, ok := s.edge(tp, e); ok {
					edges = append(edges, se)
				}
			}
		}
	})
	return edges
}

// Problems lists what is wrong with the solid's topology: edges not
// shared by two faces, loops that don't close; none for a sound solid.
func (s Solid) Problems() (msgs []string) {
	s.d.read(func(sk *sketch.Sketch) {
		for _, p := range s.topology(sk).Validate() {
			msgs = append(msgs, p.Msg)
		}
	})
	return msgs
}

// Euler is the solid's Euler characteristic, V - E + F less the holes
// in faces: 2 for a solid with no hole through it, 0 for one with one.
func (s Solid) Euler() (chi int) {
	s.d.read(func(sk *sketch.Sketch) { chi = s.topology(sk).Euler() })
	return chi
}

// resolve finds the face's sheet in the current topology.
func (f SolidFace) resolve(sk *sketch.Sketch) (*topo.Topology, int) {
	tp := f.s.topology(sk)
	fs := tp.SheetsWithID(f.id)
	if f.n >= len(fs) {
		fail("%s is no longer in the solid", sk.FaceName(f.id))
	}
	return tp, fs[f.n]
}

// Patches is how many patches the face is made of: one for a flat
// face, four for an extruded circle's side, more for a surface cut
// into patches at its knots.
func (f SolidFace) Patches() (n int) {
	f.s.d.read(func(sk *sketch.Sketch) {
		tp, i := f.resolve(sk)
		n = len(tp.Sheets[i].Faces)
	})
	return n
}

// Name names the face: the entity that made it, or the group.
func (f SolidFace) Name() (name string) {
	f.s.d.read(func(sk *sketch.Sketch) { name = sk.FaceName(f.id) })
	return name
}

// ID is the face's identity: the handle of the entity that made it,
// or for a face no entity names, a number made of the group that made
// it and its place among that group's surfaces.
func (f SolidFace) ID() uint32 { return uint32(f.id) }

// Entity is the face entity that made the face, which constraints can
// use; it fails for a face that no entity names.
func (f SolidFace) Entity() (e Entity) {
	f.s.d.read(func(sk *sketch.Sketch) {
		h := core.HEntity{V: uint32(f.id)}
		if sk.GetEntity(h) == nil {
			fail("%s has no entity", sk.FaceName(f.id))
		}
		e = Entity{d: f.s.d, H: h}
	})
	return e
}

// Edges are the edges round the face.
func (f SolidFace) Edges() (edges []SolidEdge) {
	f.s.d.read(func(sk *sketch.Sketch) {
		tp, i := f.resolve(sk)
		for _, e := range tp.SheetEdges(i) {
			if se, ok := f.s.edge(tp, e); ok {
				edges = append(edges, se)
			}
		}
	})
	return edges
}

// Neighbours are the faces that share an edge with this one.
func (f SolidFace) Neighbours() (faces []SolidFace) {
	f.s.d.read(func(sk *sketch.Sketch) {
		tp, i := f.resolve(sk)
		for _, g := range tp.SheetNeighbours(i) {
			faces = append(faces, f.s.face(tp, g))
		}
	})
	return faces
}

// resolve finds the edge in the current topology.
func (e SolidEdge) resolve(sk *sketch.Sketch) (*topo.Topology, int) {
	tp := e.s.topology(sk)
	i, err := tp.Resolve(e.name)
	if err != nil {
		fail("%s: %v", sk.SolidEdgeName(e.name), err)
	}
	return tp, i
}

// Name is the edge's name, "edge A B N": the two faces' identities in
// hex and its place among the edges between them. Solid.Edge reads it
// back.
func (e SolidEdge) Name() string { return e.name.String() }

// Describe names the edge for a person.
func (e SolidEdge) Describe() (s string) {
	e.s.d.read(func(sk *sketch.Sketch) { s = sk.SolidEdgeName(e.name) })
	return s
}

// Faces are the two faces the edge lies between.
func (e SolidEdge) Faces() (faces [2]SolidFace) {
	e.s.d.read(func(sk *sketch.Sketch) {
		tp, i := e.resolve(sk)
		for k, sh := range tp.EdgeSheets(i) {
			if sh >= 0 {
				faces[k] = e.s.face(tp, sh)
			}
		}
	})
	return faces
}

// Ends are the edge's two ends' positions, one point for an edge that
// closes, as a circle does.
func (e SolidEdge) Ends() (a, b Vec) {
	pts := e.Points()
	return pts[0], pts[len(pts)-1]
}

// Points are points along the edge, from one end to the other.
func (e SolidEdge) Points() (pts []Vec) {
	e.s.d.read(func(sk *sketch.Sketch) {
		tp, i := e.resolve(sk)
		pts = tp.ChainPoints(tp.Chain(i))
	})
	return pts
}

// Length is the edge's length in mm.
func (e SolidEdge) Length() (l float64) {
	pts := e.Points()
	for i := 1; i < len(pts); i++ {
		l += pts[i].Minus(pts[i-1]).Magnitude()
	}
	return l
}

// Pieces is how many pieces the seams of the faces it bounds cut the
// edge into: four for an extruded circle's rim.
func (e SolidEdge) Pieces() (n int) {
	e.s.d.read(func(sk *sketch.Sketch) {
		tp, i := e.resolve(sk)
		n = len(tp.Chain(i))
	})
	return n
}

// Straight reports whether the edge is a line segment.
func (e SolidEdge) Straight() (ok bool) {
	e.s.d.read(func(sk *sketch.Sketch) {
		tp, i := e.resolve(sk)
		ok = len(tp.Chain(i)) == 1 && tp.Straight(i)
	})
	return ok
}

// Pos is the vertex's position.
func (v SolidVertex) Pos() Vec { return v.p }

// Edges are the edges that end at the vertex.
func (v SolidVertex) Edges() (edges []SolidEdge) {
	v.s.d.read(func(sk *sketch.Sketch) {
		tp := v.s.topology(sk)
		for i := range tp.Vertices {
			if !tp.Vertices[i].P.Equals(v.p) {
				continue
			}
			seen := map[topo.Name]bool{}
			for _, e := range tp.Vertices[i].Edges {
				if se, ok := v.s.edge(tp, e); ok && !seen[se.name] {
					seen[se.name] = true
					edges = append(edges, se)
				}
			}
			return
		}
		fail("there is no longer a vertex at (%g, %g, %g)", v.p.X, v.p.Y, v.p.Z)
	})
	return edges
}

// ChamferSolid cuts edges of the solid so far, by name, at distance mm
// from each edge on both faces: any straight edge between two flat
// faces, such as one a Boolean made, or a circular one where a flat
// face meets a cylinder or a cone, picked from Solid. The chamfer
// keeps the edges' names, so it follows them when the model upstream
// changes, and reports when one no longer resolves. The new group
// becomes active.
func (d *Doc) ChamferSolid(edges []SolidEdge, distance float64, name string) Group {
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

// SolidEdges are the edges of the solid a chamfer cuts or a fillet
// rounds by name.
func (g Group) SolidEdges() (edges []SolidEdge) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		if group.Type != sketch.GroupFillet {
			g.mustBe(group, sketch.GroupChamfer, "SolidEdges")
		}
		prev := group.PreviousGroup(sk)
		for _, n := range group.SolidEdges {
			s := Solid{d: g.d}
			if prev != nil {
				s.g = prev.H
			}
			edges = append(edges, SolidEdge{s: s, name: n})
		}
	})
	return edges
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
func (d *Doc) Fillet(edges []Entity, radius float64, name string) Group {
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
func (d *Doc) FilletSolid(edges []SolidEdge, radius float64, name string) Group {
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

// Radius is a fillet's radius in mm.
func (g Group) Radius() (mm float64) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupFillet, "Radius")
		mm = group.ValB
	})
	return mm
}

// SetRadius sets a fillet's radius in mm; SetExpr with
// sketch.GroupPitch gives it an expression in the document's unit.
func (g Group) SetRadius(mm float64) {
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupFillet, "SetRadius")
		if mm <= 0 || math.IsNaN(mm) || math.IsInf(mm, 0) {
			fail("a fillet's radius must be more than zero, not %g", mm)
		}
		group.ValB, group.ExprB = mm, ""
		sk.MarkGroupDirty(group.H, false)
	})
}

// EdgeRadius is the radius in mm a fillet gives its edge i, counting
// the entities Edges gives and then the edges SolidEdges gives, from 0:
// the edge's own, or the fillet's.
func (g Group) EdgeRadius(i int) (mm float64) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupFillet, "EdgeRadius")
		if i < 0 || i >= len(group.Edges)+len(group.SolidEdges) {
			fail("the fillet has no edge %d", i)
		}
		mm, _, _ = group.EdgeRadius(i)
	})
	return mm
}

// SetEdgeRadius gives a fillet's edge i, counted as EdgeRadius counts,
// a radius of its own in mm, or with 0 the fillet's again. Rounded
// edges that meet at a corner must have one radius; one that ends where
// no other rounded edge does may have any.
func (g Group) SetEdgeRadius(i int, mm float64) {
	text := ""
	if mm != 0 {
		if mm < 0 || math.IsNaN(mm) || math.IsInf(mm, 0) {
			fail("a fillet's radius must be more than zero, not %g", mm)
		}
		// The text in the document's unit, as the screen takes it.
		var per float64
		g.d.read(func(sk *sketch.Sketch) { per = sk.Units.MmPer() })
		text = strconv.FormatFloat(mm/per, 'g', -1, 64)
	}
	if err := g.SetEdgeRadiusExpr(i, text); err != nil {
		fail("%v", err)
	}
}

// SetEdgeRadiusExpr gives a fillet's edge i, counted as EdgeRadius
// counts, a radius of its own from an expression in the document's
// unit, which may name parameters; "" gives it the fillet's radius
// again.
func (g Group) SetEdgeRadiusExpr(i int, expr string) (err error) {
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupFillet, "SetEdgeRadiusExpr")
		err = sk.SetEdgeRadiusText(g.H, i, expr)
	})
	return err
}

// Shell hollows the solid so far to walls thick mm, open where the faces
// are taken away: face entities, as Group.Faces gives an extrusion's.
// This version hollows solids of flat faces, and refuses a thickness at
// which walls meet. The faces of the cavity have names. The new group
// becomes active.
func (d *Doc) Shell(faces []Entity, thick float64, name string) Group {
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
func (d *Doc) ShellSolid(faces []SolidFace, thick float64, name string) Group {
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

// Thickness is a shell's wall thickness in mm.
func (g Group) Thickness() (mm float64) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupShell, "Thickness")
		mm = group.ValB
	})
	return mm
}

// SetThickness sets a shell's wall thickness in mm; SetExpr with
// sketch.GroupPitch gives it an expression in the document's unit.
func (g Group) SetThickness(mm float64) {
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupShell, "SetThickness")
		if mm <= 0 || math.IsNaN(mm) || math.IsInf(mm, 0) {
			fail("a shell's thickness must be more than zero, not %g", mm)
		}
		group.ValB, group.ExprB = mm, ""
		sk.MarkGroupDirty(group.H, false)
	})
}

// Loft joins the closed loops of two sketches in workplanes, from and
// to, with ruled faces, curve by curve, and closes the ends with the
// sketches' planes: each loop must be one closed loop; a circle counts
// as four curves, and a loop with fewer curves than the other has its
// longest split until they match. The solid joins the solid so far as
// an extrusion's does. The new group becomes active.
func (d *Doc) Loft(from, to Group, name string) Group {
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
func (d *Doc) LoftThrough(profiles []Group, name string) Group {
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

// Profiles are the sketches a loft joins, in order.
func (g Group) Profiles() (out []Group) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupLoft, "Profiles")
		for _, h := range group.LoftProfiles() {
			out = append(out, Group{g.d, h})
		}
	})
	return out
}

// Turn is how many curves a loft turns its second profile by before
// joining curve by curve; a smooth loft turns each after the first.
func (g Group) Turn() (n int) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupLoft, "Turn")
		n = int(group.ValA)
	})
	return n
}

// SetTurn turns a loft's second profile by n curves, which joins its
// curves to others of the first: a square lofted to a square turned by
// one twists by a quarter.
func (g Group) SetTurn(n int) {
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupLoft, "SetTurn")
		group.ValA, group.ExprA = float64(n), ""
		sk.MarkGroupDirty(group.H, false)
	})
}
