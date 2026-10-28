package sketch

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A chamfer cuts each of its edges with a wedge: the region between
// the two faces that meet at the edge and the plane through the lines
// on those faces at the chamfer's distance from it. The wedge reaches
// past the faces, so that none of its sides lies on one, and past the
// ends of the edge by twice the distance, so that it cuts a corner
// whose end face leans; for a part that goes on past the end of an
// edge, it cuts that far into it. The wedges are joined, and the
// union is taken from the solid so far.

// NewChamfer starts a chamfer at the active group, of edges of the
// solid so far: line segments along edges between two planar faces,
// and edges of the solid itself, by name, straight and between two
// planar faces; at distance mm from each edge on both faces. It is
// what the interface and scripts add with AddGroupAfterActive.
func (sk *Sketch) NewChamfer(edges []core.HEntity, solidEdges []topo.Name, distance float64, name string) (Group, error) {
	return sk.newChamferTwo(edges, solidEdges, distance, 0, name)
}

// newChamferTwo is NewChamfer with two distances (roadmap 28.1): d1 on
// each edge's first face, d2 on the other. The first face is the one of
// the lower identity, the one the edge's name gives first; swapping the
// distances chamfers the other way round. A d2 of 0 is d1 again.
func (sk *Sketch) newChamferTwo(edges []core.HEntity, solidEdges []topo.Name, distance, d2 float64, name string) (Group, error) {
	g := NewGroup()
	g.Type = GroupChamfer
	g.Name = name
	if name == "" {
		g.Name = "chamfer"
	}
	g.MeshCombine = CombineDifference
	g.ValB = distance
	g.ValC = d2
	if len(edges) == 0 && len(solidEdges) == 0 {
		return g, errors.New("there are no edges to chamfer")
	}
	if distance <= 0 || math.IsNaN(distance) || math.IsInf(distance, 0) {
		return g, fmt.Errorf("a chamfer's distance must be more than zero, not %g", distance)
	}
	if d2 < 0 || math.IsNaN(d2) || math.IsInf(d2, 0) {
		return g, fmt.Errorf("a chamfer's second distance must be more than zero, or zero for the first again, not %g", d2)
	}
	seen := map[core.HEntity]bool{}
	for _, he := range edges {
		if seen[he] {
			return g, fmt.Errorf("edge %s is selected twice", sk.entityName(he))
		}
		seen[he] = true
	}
	seenName := map[topo.Name]bool{}
	for _, n := range solidEdges {
		if seenName[n] {
			return g, fmt.Errorf("%s is selected twice", sk.SolidEdgeName(n))
		}
		seenName[n] = true
	}
	if err := sk.chamferTrial(sk.solidAt(sk.Active), edges, solidEdges, distance, d2, nil, nil); err != nil {
		return g, err
	}
	g.Edges = append([]core.HEntity(nil), edges...)
	g.SolidEdges = append([]topo.Name(nil), solidEdges...)
	return g, nil
}

// SetChamferEdges makes the chamfer hg cut edges and solidEdges, picked
// on the solid before it, instead of the edges it cuts now: a
// chamfer's edges can be changed after it is made (ROADMAP F7). The
// chamfer is tried on a copy of the solid before it with its
// distances, so an edge it can't cut is an error here and nothing
// changes. An edge's own distances stay with it, as a fillet's edge's
// radius does. The caller remembers the undo state.
func (sk *Sketch) SetChamferEdges(hg core.HGroup, edges []core.HEntity, solidEdges []topo.Name) error {
	g, err := sk.inputGroup(hg, GroupChamfer, "edges")
	if err != nil {
		return err
	}
	edges, solidEdges = dedup(edges), dedup(solidEdges)
	if len(edges) == 0 && len(solidEdges) == 0 {
		return errors.New("there are no edges to chamfer")
	}
	if slices.Equal(edges, g.Edges) && slices.Equal(solidEdges, g.SolidEdges) {
		return nil
	}
	if err := sk.edgesBefore(hg, edges, solidEdges); err != nil {
		return err
	}
	radii := carryEdgeValues(g, g.EdgeRadii, edges, solidEdges)
	seconds := carryEdgeValues(g, g.EdgeSeconds, edges, solidEdges)
	if err := sk.chamferTrial(sk.solidBefore(g), edges, solidEdges, g.ValB, g.ValC, radii, seconds); err != nil {
		return err
	}
	g.Edges, g.SolidEdges, g.EdgeRadii, g.EdgeSeconds = edges, solidEdges, radii, seconds
	sk.MarkGroupDirty(hg, false)
	return nil
}

// chamferTrial cuts edges and solidEdges of solid on a copy at distance
// and d2, or the edges' own (radii, seconds), as generateChamferShell
// will, or says why it can't: by an edit of the solid's faces, failing
// that by wedges and arcs. What NewChamfer and SetChamferEdges check
// before the chamfer is made.
func (sk *Sketch) chamferTrial(solid *Group, edges []core.HEntity, solidEdges []topo.Name, distance, d2 float64, radii, seconds []EdgeRadius) error {
	if solid == nil {
		return errors.New("there is no solid to chamfer")
	}
	segs, err := sk.chamferSegments(solid, edges, solidEdges)
	if err != nil {
		return err
	}
	picks := chamferPicks(segs, d2, radii, seconds)
	if !solid.IsForcedToMesh(sk) && len(solid.RunningShell.Surface) > 0 {
		_, err := chamferByEdit(&solid.RunningShell, picks, distance, 1<<30)
		if err == nil {
			return nil
		}
		if topo.RollingRefusal(err) {
			// No wedge cuts there soundly (generateChamferShell).
			return err
		}
	}
	var mesh srf.SMesh
	solidMesh(solid, &mesh)
	first, err := firstFaces(solid, segs, picks)
	if err != nil {
		return err
	}
	var arcs []topo.FilletPick
	for i, seg := range segs {
		if seg.arc {
			arcs = append(arcs, picks[i])
			continue
		}
		if _, _, err := chamferWedge(&mesh, seg, pickDistance(picks[i], distance), picks[i].D2, first[i]); err != nil {
			return err
		}
	}
	if len(arcs) > 0 {
		if solid.IsForcedToMesh(sk) || len(solid.RunningShell.Surface) == 0 {
			return errors.New("the solid is a triangle mesh, which has no circular edges to cut")
		}
		var sh srf.SShell
		sh.MakeFromCopyOf(&solid.RunningShell)
		if err := topo.FilletPicks(&sh, arcs, distance, 1<<30, true); err != nil {
			return err
		}
	}
	return nil
}

// chamferSegment is an edge a chamfer cuts or a fillet rounds: a
// straight one from a to b, or a circular one through at, and how
// messages name it.
type chamferSegment struct {
	a, b  geom.Vector
	at    geom.Vector
	arc   bool
	label string
}

// pick is the segment as topo picks an edge, with a chamfer's second
// distance d2, or 0.
func (s chamferSegment) pick(d2 float64) topo.FilletPick {
	return topo.FilletPick{A: s.a, B: s.b, At: s.at, Arc: s.arc, D2: d2}
}

// firstFaces is, for each straight segment of a chamfer cut with two
// distances (its pick's D2), the outward normal of the face its first
// distance lies on (topo.Topology.FirstFaceNormal); zero for one of
// one distance.
func firstFaces(solid *Group, segs []chamferSegment, picks []topo.FilletPick) ([]geom.Vector, error) {
	out := make([]geom.Vector, len(segs))
	if !slices.ContainsFunc(picks, func(p topo.FilletPick) bool { return p.D2 != 0 }) {
		return out, nil
	}
	if len(solid.RunningShell.Surface) == 0 {
		return nil, errors.New("a chamfer of two distances needs the solid's faces, and the solid so far is a triangle mesh")
	}
	tp, err := solid.Topology()
	if err != nil {
		return nil, fmt.Errorf("the solid's topology is broken: %w", err)
	}
	for i, seg := range segs {
		if seg.arc || picks[i].D2 == 0 {
			continue
		}
		n, ok := tp.FirstFaceNormal(seg.a, seg.b)
		if !ok {
			return nil, fmt.Errorf("%s is not an edge between two flat faces of the solid", seg.label)
		}
		out[i] = n
	}
	return out, nil
}

// chamferSegments are the segments of a chamfer's edges, on the solid
// whose running shell is solid's: the line segment entities, then the
// edges of the solid by name.
func (sk *Sketch) chamferSegments(solid *Group, edges []core.HEntity, names []topo.Name) ([]chamferSegment, error) {
	var out []chamferSegment
	for _, he := range edges {
		e := sk.Entity(he)
		if e == nil {
			return nil, fmt.Errorf("edge %s doesn't exist", sk.entityName(he))
		}
		if e.IsCircle() {
			// A circle or an arc of one: a circular edge, by a point on it.
			var sbl srf.SBezierList
			GenerateEntityBeziers(e, &sbl, &Runtime{Sketch: sk})
			if len(sbl.Curves) == 0 {
				return nil, fmt.Errorf("%s has no curve", sk.entityName(he))
			}
			out = append(out, chamferSegment{at: sbl.Curves[0].PointAt(0.5), arc: true,
				label: "circle " + sk.entityName(he)})
			continue
		}
		if e.Type != core.EntityLineSegment {
			return nil, fmt.Errorf("%s is neither a line segment nor a circle: a chamfer cuts straight and circular edges", sk.entityName(he))
		}
		out = append(out, chamferSegment{
			a:     sk.MustEntity(e.Point[0]).PointGetNum(sk),
			b:     sk.MustEntity(e.Point[1]).PointGetNum(sk),
			label: "line segment " + sk.entityName(he),
		})
	}
	if len(names) == 0 {
		return out, nil
	}
	if solid == nil || solid.IsForcedToMesh(sk) || len(solid.RunningShell.Surface) == 0 {
		return nil, errors.New("the solid so far is a triangle mesh, whose edges have no names")
	}
	tp, _ := solid.Topology()
	for _, n := range names {
		e, err := tp.Resolve(n)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", sk.SolidEdgeName(n), err)
		}
		ed := &tp.Edges[e]
		if !tp.Straight(e) {
			out = append(out, chamferSegment{at: ed.Points[len(ed.Points)/2], arc: true, label: sk.SolidEdgeName(n)})
			continue
		}
		out = append(out, chamferSegment{
			a:     tp.Vertices[ed.V[0]].P,
			b:     tp.Vertices[ed.V[1]].P,
			label: sk.SolidEdgeName(n),
		})
	}
	return out, nil
}

// SolidEdgeName names an edge of a solid for a person: the faces it
// lies between.
func (sk *Sketch) SolidEdgeName(n topo.Name) string {
	s := fmt.Sprintf("the edge between %s and %s", sk.FaceName(n.A), sk.FaceName(n.B))
	if n.N > 0 {
		s += fmt.Sprintf(" (number %d of them)", n.N+1)
	}
	return s
}

// FaceName names a face of a solid by its identity: the entity that
// made it, or for a face no entity names, the group that made it.
func (sk *Sketch) FaceName(id srf.FaceID) string {
	he := core.HEntity{V: uint32(id)}
	if g := faceGroup(id); id&(1<<30) != 0 && id&(1<<31) == 0 {
		return fmt.Sprintf("face %d of %s", uint32(id)&0xffff, sk.anyGroupName(g))
	}
	return "face " + sk.entityName(he)
}

// SolidAt is the group whose running solid is the model at group hg,
// or nil when there is none: the solid whose edges a chamfer there
// cuts and the interface lets the user pick.
func (sk *Sketch) SolidAt(hg core.HGroup) *Group { return sk.solidAt(hg) }

// solidAt is SolidAt.
func (sk *Sketch) solidAt(hg core.HGroup) *Group {
	for g := sk.Group(hg); g != nil; g = g.PreviousGroup(sk) {
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
// the edge and the vector it is swept along. With a second distance d2,
// d lies on the face whose outward normal is first and d2 on the other.
func chamferWedge(mesh *srf.SMesh, seg chamferSegment, d, d2 float64, first geom.Vector) (tri [3]geom.Vector, along geom.Vector, err error) {
	a, b := seg.a, seg.b
	length := b.Minus(a).Magnitude()
	if length < core.LengthEps {
		return tri, along, fmt.Errorf("%s has no length", seg.label)
	}
	ed := b.Minus(a).ScaledBy(1 / length)
	normals := edgeFaceNormals(mesh, a, b)
	if len(normals) != 2 {
		return tri, along, fmt.Errorf("%s is not an edge between two flat faces of the solid (it touches %d)",
			seg.label, len(normals))
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
		return tri, along, fmt.Errorf("%s is not an edge of the solid", seg.label)
	case inside(at(1, -1)) || inside(at(-1, 1)):
		return tri, along, fmt.Errorf("%s is concave: a chamfer cuts only edges that stick out", seg.label)
	}
	// Face 1 goes on from the edge against w2, face 2 against w1.
	u1, u2 := w2.WithMagnitude(1).ScaledBy(-1), w1.WithMagnitude(1).ScaledBy(-1)
	d1, dd2 := d, d
	if d2 != 0 {
		// The first distance on the first face.
		if n1.Dot(first) > n2.Dot(first) {
			dd2 = d2
		} else {
			d1 = d2
		}
	}
	// The chamfer's lines on the faces must be on them: just under
	// each is solid, just over it is not.
	for _, f := range []struct {
		u, n geom.Vector
		d    float64
	}{{u1, n1, d1}, {u2, n2, dd2}} {
		q := m.Plus(f.u.ScaledBy(f.d))
		if !inside(q.Minus(f.n.ScaledBy(probe))) || inside(q.Plus(f.n.ScaledBy(probe))) {
			return tri, along, fmt.Errorf("a distance of %g is wider than a face at %s", f.d, seg.label)
		}
	}
	ext := 2 * math.Max(d1, dd2)
	p := a.Minus(ed.ScaledBy(ext))
	tri = [3]geom.Vector{
		p.Plus(u1.ScaledBy(d1)),
		p.Plus(u2.ScaledBy(dd2)),
		p.Minus(u1.ScaledBy(d1).Plus(u2.ScaledBy(dd2))),
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

// generateChamferShell joins the wedges of the group's straight edges
// into its shell, which the pipeline takes from the solid so far. With
// circular edges it makes the solid itself, and says so: a copy of the
// solid so far with those edges cut by cones (topo.FilletPicks), less
// the wedges.
func (g *Group) generateChamferShell(sk *Sketch) (done bool) {
	g.MeshCombine = CombineDifference
	if len(g.Edges) == 0 && len(g.SolidEdges) == 0 {
		g.setProblem(nil, "the chamfer has no edges")
		return false
	}
	d, d2 := g.ValB, g.ValC
	if d <= 0 {
		g.setProblem(nil, "a chamfer's distance must be more than zero")
		return false
	}
	if d2 < 0 {
		g.setProblem(nil, "a chamfer's second distance must be more than zero, or zero for the first again")
		return false
	}
	prev := g.PreviousGroup(sk)
	if prev == nil {
		g.setProblem(nil, "there is no solid to chamfer")
		return false
	}
	var mesh srf.SMesh
	solidMesh(prev, &mesh)
	if len(mesh.Triangles) == 0 {
		g.setProblem(nil, "there is no solid to chamfer")
		return false
	}
	segs, err := sk.chamferSegments(prev, g.Edges, g.SolidEdges)
	if err != nil {
		g.setProblem(nil, "%v", err)
		return false
	}
	picks := chamferPicks(segs, d2, g.EdgeRadii, g.EdgeSeconds)
	// On a shell every edge is cut by the fillet's local edit, flat:
	// straight ones as circular ones, so concave edges and chains through
	// smooth joins come too (roadmap 15.1). Where that can't be done, as
	// at a corner where a concave edge meets the cut, and on a solid
	// forced to a mesh, which has no faces to edit, straight edges are
	// cut by wedges and a Boolean, as before.
	if !prev.IsForcedToMesh(sk) && len(prev.RunningShell.Surface) > 0 {
		sh, err := chamferByEdit(&prev.RunningShell, picks, d, srf.OriginFor(g.H.V, 0))
		if err == nil {
			g.RunningShell = sh
			g.ThisShell.MakeFromCopyOf(&sh)
			return true
		}
		if topo.RollingRefusal(err) {
			// A wedge would stand open across a curved face: the
			// edit's reason is the group's.
			g.RunningShell.MakeFromCopyOf(&prev.RunningShell)
			if fe, ok := errors.AsType[*topo.FilletError](err); ok {
				at := fe.At
				g.setProblem(&at, "%v", err)
			} else {
				g.setProblem(nil, "%v", err)
			}
			return true
		}
	}
	first, err := firstFaces(prev, segs, picks)
	if err != nil {
		g.setProblem(nil, "%v", err)
		return false
	}
	var arcs []topo.FilletPick
	var straight []chamferSegment
	var firstOf []geom.Vector
	var pickOf []topo.FilletPick
	for i, seg := range segs {
		if seg.arc {
			arcs = append(arcs, picks[i])
		} else {
			straight = append(straight, seg)
			firstOf = append(firstOf, first[i])
			pickOf = append(pickOf, picks[i])
		}
	}
	var wedges []srf.SShell
	for i, seg := range straight {
		tri, along, err := chamferWedge(&mesh, seg, pickDistance(pickOf[i], d), pickOf[i].D2, firstOf[i])
		if err != nil {
			m := seg.a.Plus(seg.b).ScaledBy(0.5)
			g.setProblem(&m, "%v", err)
			return false
		}
		w := srf.SShell{Tol: sk.tol}
		w.MakePrism(tri[:], geom.Vector{}, along, g.Color)
		wedges = append(wedges, w)
	}
	if len(arcs) == 0 {
		g.ThisShell = unionAll(wedges)
		return false
	}
	g.RunningShell.MakeFromCopyOf(&prev.RunningShell)
	if prev.IsForcedToMesh(sk) || len(prev.RunningShell.Surface) == 0 {
		g.setProblem(nil, "the solid so far is a triangle mesh, which has no circular edges to cut")
		return true
	}
	var sh srf.SShell
	sh.MakeFromCopyOf(&prev.RunningShell)
	if err := topo.FilletPicks(&sh, arcs, d, srf.OriginFor(g.H.V, 0), true); err != nil {
		if fe, ok := errors.AsType[*topo.FilletError](err); ok {
			at := fe.At
			g.setProblem(&at, "%v", err)
		} else {
			g.setProblem(nil, "%v", err)
		}
		return true
	}
	if len(wedges) > 0 {
		cutter := unionAll(wedges)
		var out srf.SShell
		out.MakeFromDifferenceOf(&sh, &cutter)
		sh = out
	}
	g.RunningShell = sh
	g.ThisShell.MakeFromCopyOf(&sh)
	return true
}

// chamferByEdit cuts every pick of a copy of solid at its own distance
// or d, and its D2 on the second face when that is not 0, by the
// fillet's edit, flat.
func chamferByEdit(solid *srf.SShell, picks []topo.FilletPick, d float64, origin uint32) (srf.SShell, error) {
	var sh srf.SShell
	sh.MakeFromCopyOf(solid)
	err := topo.FilletPicks(&sh, picks, d, origin, true)
	return sh, err
}

// pickDistance is the distance a chamfer cuts pick p at: its own, or
// the group's d.
func pickDistance(p topo.FilletPick, d float64) float64 {
	if p.R != 0 {
		return p.R
	}
	return d
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
