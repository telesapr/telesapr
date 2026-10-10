package api

import (
	"math"
	"slices"

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
	g := sk.Group(s.g)
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
		slices.SortFunc(found, func(a, b topo.Name) int {
			less := func(a, b topo.Name) bool { return a.Less(b) }
			switch {
			case less(a, b):
				return -1
			case less(b, a):
				return 1
			}
			return 0
		})
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
		if sk.Entity(h) == nil {
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

// Smooth reports whether the edge is a seam where its two faces go on
// into each other without a crease, as the patches of a loft or a
// surface's pieces do: there is no corner there for a fillet or a
// chamfer to round.
func (e SolidEdge) Smooth() (ok bool) {
	e.s.d.read(func(sk *sketch.Sketch) {
		tp, i := e.resolve(sk)
		ok = tp.Smooth(i)
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

// SolidEdges are the edges of the solid a chamfer cuts, a fillet rounds
// or a sweep follows, by name.
func (g Group) SolidEdges() (edges []SolidEdge) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		if group.Type != sketch.GroupFillet && group.Type != sketch.GroupSweep {
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
