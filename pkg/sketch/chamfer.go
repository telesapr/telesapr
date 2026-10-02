package sketch

import (
	"errors"
	"fmt"
	"math"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// A chamfer cuts each of its edges with a wedge: the region between
// the two faces that meet at the edge and the plane through the lines
// on those faces at the chamfer's distance from it. The wedge reaches
// past the faces, so that none of its sides lies on one, and past the
// ends of the edge by twice the distance, so that it cuts a corner
// whose end face leans; for a part that goes on past the end of an
// edge, it cuts that far into it. The wedges are joined, and the
// union is taken from the solid so far.

// NewChamfer starts a chamfer of edges of the solid at the active
// group, line segments along edges between two planar faces, at
// distance mm from each edge on both faces. It is what the interface
// and scripts add with AddGroupAfterActive.
func (sk *Sketch) NewChamfer(edges []core.HEntity, distance float64, name string) (Group, error) {
	g := NewGroup()
	g.Type = GroupChamfer
	g.Name = name
	if name == "" {
		g.Name = "chamfer"
	}
	g.MeshCombine = CombineDifference
	g.ValB = distance
	if len(edges) == 0 {
		return g, errors.New("there are no edges to chamfer")
	}
	if distance <= 0 || math.IsNaN(distance) || math.IsInf(distance, 0) {
		return g, fmt.Errorf("a chamfer's distance must be more than zero, not %g", distance)
	}
	solid := sk.solidAt(sk.Active)
	if solid == nil {
		return g, errors.New("there is no solid to chamfer at the active group")
	}
	var mesh srf.SMesh
	solidMesh(solid, &mesh)
	seen := map[core.HEntity]bool{}
	for _, he := range edges {
		if seen[he] {
			return g, fmt.Errorf("edge %s is selected twice", sk.entityName(he))
		}
		seen[he] = true
		if _, _, err := chamferWedge(sk, &mesh, he, distance); err != nil {
			return g, err
		}
	}
	g.Edges = append([]core.HEntity(nil), edges...)
	return g, nil
}

// solidAt is the group whose running solid is the model at group hg,
// or nil when there is none.
func (sk *Sketch) solidAt(hg core.HGroup) *Group {
	for g := sk.GetGroup(hg); g != nil; g = g.PreviousGroup(sk) {
		if len(g.RunningShell.Surface) > 0 || len(g.RunningMesh.Triangles) > 0 {
			return g
		}
		if g.IsMeshGroup() || g.IsCopyGroup() || g.Type == GroupLinked {
			// A group that makes a solid and has none: the model is
			// empty here.
			return nil
		}
	}
	return nil
}

// solidMesh triangulates a group's running solid.
func solidMesh(g *Group, m *srf.SMesh) {
	g.RunningShell.TriangulateInto(m)
	m.Triangles = append(m.Triangles, g.RunningMesh.Triangles...)
}

// chamferWedge is the wedge that cuts edge he of the solid whose
// triangles are in mesh, at distance d: a triangle at the start of
// the edge and the vector it is swept along.
func chamferWedge(sk *Sketch, mesh *srf.SMesh, he core.HEntity, d float64) (tri [3]geom.Vector, along geom.Vector, err error) {
	e := sk.GetEntity(he)
	if e == nil {
		return tri, along, fmt.Errorf("edge %s doesn't exist", sk.entityName(he))
	}
	if e.Type != core.EntityLineSegment {
		return tri, along, fmt.Errorf("%s is not a line segment: a chamfer cuts straight edges", sk.entityName(he))
	}
	a := sk.MustGetEntity(e.Point[0]).PointGetNum(sk)
	b := sk.MustGetEntity(e.Point[1]).PointGetNum(sk)
	length := b.Minus(a).Magnitude()
	if length < core.LengthEps {
		return tri, along, fmt.Errorf("edge %s has no length", sk.entityName(he))
	}
	ed := b.Minus(a).ScaledBy(1 / length)
	normals := edgeFaceNormals(mesh, a, b)
	if len(normals) != 2 {
		return tri, along, fmt.Errorf("line segment %s is not an edge between two flat faces of the solid (it touches %d)",
			sk.entityName(he), len(normals))
	}
	n1, n2 := normals[0], normals[1]
	// w1 is off plane 1 and in plane 2, w2 the other way round, so
	// that the quarter behind both faces is where both components are
	// negative.
	w1 := ed.Cross(n2)
	w1 = w1.ScaledBy(1 / n1.Dot(w1))
	w2 := ed.Cross(n1)
	w2 = w2.ScaledBy(1 / n2.Dot(w2))
	m := a.Plus(b).ScaledBy(0.5)
	probe := 0.1 * math.Min(d, length)
	at := func(s1, s2 float64) geom.Vector {
		return m.Plus(w1.WithMagnitude(probe * s1)).Plus(w2.WithMagnitude(probe * s2))
	}
	inside := func(p geom.Vector) bool { return mesh.WindingNumber(p) > 0.5 }
	switch {
	case !inside(at(-1, -1)) || inside(at(1, 1)):
		return tri, along, fmt.Errorf("line segment %s is not an edge of the solid", sk.entityName(he))
	case inside(at(1, -1)) || inside(at(-1, 1)):
		return tri, along, fmt.Errorf("edge %s is concave: a chamfer cuts only edges that stick out", sk.entityName(he))
	}
	// Face 1 goes on from the edge against w2, face 2 against w1.
	u1, u2 := w2.WithMagnitude(1).ScaledBy(-1), w1.WithMagnitude(1).ScaledBy(-1)
	// The chamfer's lines on the faces must be on them: just under
	// each is solid, just over it is not.
	for _, f := range []struct{ u, n geom.Vector }{{u1, n1}, {u2, n2}} {
		q := m.Plus(f.u.ScaledBy(d))
		if !inside(q.Minus(f.n.ScaledBy(probe))) || inside(q.Plus(f.n.ScaledBy(probe))) {
			return tri, along, fmt.Errorf("a distance of %g is wider than a face at edge %s", d, sk.entityName(he))
		}
	}
	ext := 2 * d
	p := a.Minus(ed.ScaledBy(ext))
	tri = [3]geom.Vector{
		p.Plus(u1.ScaledBy(d)),
		p.Plus(u2.ScaledBy(d)),
		p.Minus(u1.Plus(u2).ScaledBy(d)),
	}
	return tri, ed.ScaledBy(length + 2*ext), nil
}

// edgeFaceNormals are the distinct outward normals of the triangles
// of mesh that have a side along the segment from a to b.
func edgeFaceNormals(mesh *srf.SMesh, a, b geom.Vector) []geom.Vector {
	const tol = 10 * core.LengthEps
	ab := b.Minus(a)
	l2 := ab.MagSquared()
	onLine := func(p geom.Vector) (float64, bool) {
		t := p.Minus(a).Dot(ab) / l2
		return t, p.Minus(a.Plus(ab.ScaledBy(t))).Magnitude() < tol
	}
	// alongEdge reports whether the side from p to q lies on the line
	// and overlaps the segment.
	alongEdge := func(p, q geom.Vector) bool {
		tp, okp := onLine(p)
		tq, okq := onLine(q)
		if !okp || !okq {
			return false
		}
		lo, hi := math.Min(tp, tq), math.Max(tp, tq)
		eps := tol / math.Sqrt(l2)
		return math.Min(hi, 1)-math.Max(lo, 0) > eps
	}
	var normals []geom.Vector
	for i := range mesh.Triangles {
		t := &mesh.Triangles[i]
		if !alongEdge(t.A, t.B) && !alongEdge(t.B, t.C) && !alongEdge(t.C, t.A) {
			continue
		}
		n := t.Normal().WithMagnitude(1)
		dup := false
		for _, o := range normals {
			if o.Dot(n) > 1-1e-9 {
				dup = true
				break
			}
		}
		if !dup {
			normals = append(normals, n)
		}
	}
	return normals
}

// generateChamferShell joins the wedges of the group's edges into its
// shell, which the pipeline takes from the solid so far.
func (g *Group) generateChamferShell(sk *Sketch) {
	g.MeshCombine = CombineDifference
	if len(g.Edges) == 0 {
		g.setProblem(nil, "the chamfer has no edges")
		return
	}
	d := g.ValB
	if d <= 0 {
		g.setProblem(nil, "a chamfer's distance must be more than zero")
		return
	}
	prev := g.PreviousGroup(sk)
	if prev == nil {
		g.setProblem(nil, "there is no solid to chamfer")
		return
	}
	var mesh srf.SMesh
	solidMesh(prev, &mesh)
	if len(mesh.Triangles) == 0 {
		g.setProblem(nil, "there is no solid to chamfer")
		return
	}
	var wedges []srf.SShell
	for _, he := range g.Edges {
		tri, along, err := chamferWedge(sk, &mesh, he, d)
		if err != nil {
			var at *geom.Vector
			if e := sk.GetEntity(he); e != nil && e.Type == core.EntityLineSegment {
				m := sk.MustGetEntity(e.Point[0]).PointGetNum(sk).Plus(
					sk.MustGetEntity(e.Point[1]).PointGetNum(sk)).ScaledBy(0.5)
				at = &m
			}
			g.setProblem(at, "%v", err)
			return
		}
		var w srf.SShell
		w.MakePrism(tri[:], geom.Vector{}, along, g.Color)
		wedges = append(wedges, w)
	}
	g.ThisShell = unionAll(wedges)
}

// unionAll joins shells a pair at a time.
func unionAll(shells []srf.SShell) srf.SShell {
	for len(shells) > 1 {
		var next []srf.SShell
		for i := 0; i < len(shells); i += 2 {
			if i+1 == len(shells) {
				next = append(next, shells[i])
				continue
			}
			var u srf.SShell
			u.MakeFromUnionOf(&shells[i], &shells[i+1])
			next = append(next, u)
		}
		shells = next
	}
	if len(shells) == 0 {
		return srf.SShell{}
	}
	return shells[0]
}
