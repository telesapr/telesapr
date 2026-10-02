package topo

import (
	"fmt"
	"math"

	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// A fillet rounds a straight edge between two planar faces with a
// cylinder of radius r tangent to both, by local edits: each face is cut
// along the line where the cylinder touches it, each end face along the
// arc where the cylinder meets it, the strips and corners between the
// cuts and the edge are taken out, and the cylinder's patch put in.
// The cylinder is exact: a rational quadratic arc swept along the edge.

// FilletError is why an edge can't be filleted, with where.
type FilletError struct {
	Msg string
	At  geom.Vector
}

func (e *FilletError) Error() string { return e.Msg }

func filletErr(at geom.Vector, format string, args ...any) error {
	return &FilletError{Msg: fmt.Sprintf(format, args...), At: at}
}

// planarNormal is a face's outward normal when its surface is a plane,
// which a SolveSpace surface's normal is.
func (t *Topology) planarNormal(f int) (geom.Vector, bool) {
	s := t.Surface(f)
	if s == nil || s.DegM != 1 || s.DegN != 1 {
		return geom.Vector{}, false
	}
	n := s.NormalAt(0.5, 0.5)
	if n.Magnitude() == 0 {
		return geom.Vector{}, false
	}
	n = n.WithMagnitude(1)
	// All four corners on one plane.
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			if math.Abs(s.Ctrl[i][j].Minus(s.Ctrl[0][0]).Dot(n)) > geom.LengthEps {
				return geom.Vector{}, false
			}
		}
	}
	return n, true
}

// intoFace is the direction, square to the edge and in face f's plane,
// that goes from the edge into the face.
func (t *Topology) intoFace(f int, a, b, n geom.Vector) geom.Vector {
	d := b.Minus(a).WithMagnitude(1)
	u := d.Cross(n).WithMagnitude(1)
	// Which side of the edge the face lies on: test a point just off
	// the edge's middle against the face's outer loop.
	m := a.Plus(b).ScaledBy(0.5)
	step := 1e-3 * b.Minus(a).Magnitude()
	if t.inFace(f, m.Plus(u.ScaledBy(step)), n) {
		return u
	}
	return u.ScaledBy(-1)
}

// inFace reports whether p, on face f's plane, is inside the face.
func (t *Topology) inFace(f int, p, n geom.Vector) bool {
	ax := n.NormalDir(0)
	ay := n.Cross(ax)
	in := false
	for _, l := range t.Faces[f].Loops {
		var poly []geom.Vector
		for _, ce := range t.Loops[l].CoEdges {
			pts := t.Edges[t.CoEdges[ce].Edge].Points
			if t.CoEdges[ce].Reversed {
				pts = reversed(pts)
			}
			for _, q := range pts[:len(pts)-1] {
				poly = append(poly, geom.Vector{X: q.Dot(ax), Y: q.Dot(ay)})
			}
		}
		if pointInPolygon(geom.Vector{X: p.Dot(ax), Y: p.Dot(ay)}, poly) {
			in = !in
		}
	}
	return in
}

// endEdges are the two edges other than e that end at vertex v, and the
// face at v that is neither of e's; an error when v is not where three
// planar faces meet.
func (t *Topology) endEdges(e, v int, fs [2]int) (along [2]int, end int, err error) {
	at := t.Vertices[v].P
	var others []int
	for _, x := range t.Vertices[v].Edges {
		if x != e {
			others = append(others, x)
		}
	}
	if len(others) != 2 {
		return along, -1, filletErr(at, "the edge ends where %d edges meet; a fillet's end needs a corner of three faces", len(others)+1)
	}
	end = -1
	for _, x := range others {
		efs := t.EdgeFaces(x)
		switch {
		case efs[0] == fs[0] || efs[1] == fs[0]:
			along[0] = x
		case efs[0] == fs[1] || efs[1] == fs[1]:
			along[1] = x
		default:
			return along, -1, filletErr(at, "the faces at the edge's end don't meet it as a corner of three faces")
		}
		for _, f := range efs {
			if f != fs[0] && f != fs[1] {
				end = f
			}
		}
	}
	if end < 0 || along[0] == along[1] {
		return along, -1, filletErr(at, "the faces at the edge's end don't meet it as a corner of three faces")
	}
	return along, end, nil
}

// pointAlong is the point of edge x, which leaves vertex v into a face,
// at distance s from the filleted edge: into is the direction in the
// face square to the filleted edge.
func (t *Topology) pointAlong(x, v int, into geom.Vector, s float64) (geom.Vector, error) {
	ed := &t.Edges[x]
	from := t.Vertices[v].P
	to := t.Vertices[ed.V[1]].P
	if ed.V[1] == v {
		to = t.Vertices[ed.V[0]].P
	}
	if !t.Straight(x) {
		return geom.Vector{}, filletErr(from, "an edge at the end of the fillet is curved")
	}
	d := to.Minus(from)
	l := d.Magnitude()
	d = d.ScaledBy(1 / l)
	k := d.Dot(into)
	if k < 1e-9 {
		return geom.Vector{}, filletErr(from, "an edge at the end of the fillet runs back along it")
	}
	k = s / k
	if k >= l-geom.LengthEps {
		return geom.Vector{}, filletErr(from, "a radius this large is wider than a face at the edge's end")
	}
	return from.Plus(d.ScaledBy(k)), nil
}

// edgeAtPoint is the edge through p, within LengthEps.
func (t *Topology) edgeAtPoint(p geom.Vector) int {
	e, d := t.EdgeAt(p)
	if d > 10*geom.LengthEps {
		return -1
	}
	return e
}

// vertexNear is the vertex at p, or -1.
func (t *Topology) vertexNear(p geom.Vector) int {
	for v := range t.Vertices {
		if t.Vertices[v].P.Equals(p) {
			return v
		}
	}
	return -1
}

// faceOnPlaneAt is the face incident to vertex v whose plane has
// normal n, or -1.
func (t *Topology) faceOnPlaneAt(v int, n geom.Vector) int {
	for _, e := range t.Vertices[v].Edges {
		for _, f := range t.EdgeFaces(e) {
			if f < 0 {
				continue
			}
			if fn, ok := t.planarNormal(f); ok && fn.Dot(n) > 1-1e-9 {
				return f
			}
		}
	}
	return -1
}

// FilletEdge rounds the edge from a to b of the shell with radius r.
// The edge must be straight, between two planar faces that meet at
// less than a straight angle with the solid between them, and end at
// corners where it meets one more planar face. The new face's surface
// gets origin as its Origin, so its edges have names.
func FilletEdge(sh *srf.SShell, a, b geom.Vector, r float64, origin uint32) error {
	if !(r > 0) || math.IsInf(r, 0) {
		return filletErr(a, "a fillet's radius must be more than zero")
	}
	t, err := Build(sh)
	if err != nil {
		return filletErr(a, "the solid's topology is broken: %v", err)
	}
	m := a.Plus(b).ScaledBy(0.5)
	e := t.edgeAtPoint(m)
	if e < 0 {
		return filletErr(m, "no edge of the solid runs there")
	}
	va, vb := t.vertexNear(a), t.vertexNear(b)
	ed := &t.Edges[e]
	if va < 0 || vb < 0 || !(ed.V[0] == va && ed.V[1] == vb || ed.V[0] == vb && ed.V[1] == va) {
		return filletErr(m, "the segment is not a whole edge of the solid")
	}
	if !t.Straight(e) {
		return filletErr(m, "the edge is curved: a fillet here rounds straight edges")
	}
	fs := t.EdgeFaces(e)
	if fs[0] < 0 || fs[1] < 0 || len(ed.CoEdges) != 2 {
		return filletErr(m, "the edge is not between two faces")
	}
	n1, ok1 := t.planarNormal(fs[0])
	n2, ok2 := t.planarNormal(fs[1])
	if !ok1 || !ok2 {
		return filletErr(m, "a face at the edge is curved: a fillet here rounds edges between flat faces")
	}
	dir := b.Minus(a).WithMagnitude(1)
	u1 := t.intoFace(fs[0], a, b, n1)
	u2 := t.intoFace(fs[1], a, b, n2)
	if u1.Dot(n2) > -1e-9 || u2.Dot(n1) > -1e-9 {
		return filletErr(m, "the edge is concave or flat: a fillet here rounds edges that stick out")
	}
	beta := math.Acos(math.Max(-1, math.Min(1, u1.Dot(u2))))
	s := r / math.Tan(beta/2)
	w := math.Sin(beta / 2)

	// Where the cuts meet the edges at each end.
	type end struct {
		v      int
		t1, t2 geom.Vector
		face   int
	}
	var ends [2]end
	for k, v := range []int{va, vb} {
		along, endFace, err := t.endEdges(e, v, fs)
		if err != nil {
			return err
		}
		if _, ok := t.planarNormal(endFace); !ok {
			return filletErr(t.Vertices[v].P, "the face at the edge's end is curved")
		}
		t1, err := t.pointAlong(along[0], v, u1, s)
		if err != nil {
			return err
		}
		t2, err := t.pointAlong(along[1], v, u2, s)
		if err != nil {
			return err
		}
		ends[k] = end{v: v, t1: t1, t2: t2, face: endFace}
	}
	endNormals := [2]geom.Vector{}
	for k := range ends {
		endNormals[k], _ = t.planarNormal(ends[k].face)
	}

	// Split the four edges at the ends.
	for _, en := range ends {
		for _, p := range []geom.Vector{en.t1, en.t2} {
			x := t.edgeAtPoint(p)
			if x < 0 {
				return filletErr(p, "no edge where the fillet meets the end")
			}
			if err := SplitEdge(sh, t, x, p); err != nil {
				return filletErr(p, "the fillet's edge can't be split: %v", err)
			}
			if t, err = Build(sh); err != nil {
				return filletErr(p, "%v", err)
			}
		}
	}
	// Cut the two faces along the tangent lines, the end faces along
	// the arcs.
	cuts := []struct {
		at  geom.Vector // the vertex the face is found at
		n   geom.Vector
		cut srf.SBezier
	}{
		{a, n1, srf.SBezierFrom2(ends[0].t1, ends[1].t1)},
		{a, n2, srf.SBezierFrom2(ends[0].t2, ends[1].t2)},
		{a, endNormals[0], arc(ends[0].t1, a, ends[0].t2, w)},
		{b, endNormals[1], arc(ends[1].t1, b, ends[1].t2, w)},
	}
	for _, c := range cuts {
		v := t.vertexNear(c.at)
		f := -1
		if v >= 0 {
			f = t.faceOnPlaneAt(v, c.n)
		}
		if f < 0 {
			return filletErr(c.at, "a face at the edge is gone")
		}
		if _, err := SplitFace(sh, t, f, c.cut); err != nil {
			return filletErr(c.cut.Start(), "the fillet runs into another edge of a face: %v", err)
		}
		if t, err = Build(sh); err != nil {
			return filletErr(c.cut.Start(), "%v", err)
		}
	}
	// Take out every face at the edge's ends: the strips and corners.
	var gone []int
	seen := map[int]bool{}
	for _, p := range []geom.Vector{a, b} {
		v := t.vertexNear(p)
		if v < 0 {
			continue
		}
		for _, x := range t.Vertices[v].Edges {
			for _, f := range t.EdgeFaces(x) {
				if f >= 0 && !seen[f] {
					seen[f] = true
					gone = append(gone, f)
				}
			}
		}
	}
	color := t.Surface(fs[0]).Color
	RemoveFaces(sh, t, gone)

	// The cylinder: the cross section's arc swept along the edge, from
	// a little before it to a little past.
	margin := s + r
	p0 := a.Minus(dir.ScaledBy(margin))
	section := arc(p0.Plus(u1.ScaledBy(s)), p0, p0.Plus(u2.ScaledBy(s)), w)
	cyl := srf.FromExtrusionOf(&section, geom.Vector{}, dir.ScaledBy(b.Minus(a).Magnitude()+2*margin))
	cyl.Color = color
	cyl.Origin = origin
	_, err = AddFace(sh, cyl, [][]srf.SBezier{{
		srf.SBezierFrom2(ends[0].t1, ends[1].t1),
		arc(ends[1].t1, b, ends[1].t2, w),
		srf.SBezierFrom2(ends[1].t2, ends[0].t2),
		arc(ends[0].t2, a, ends[0].t1, w),
	}})
	if err != nil {
		return filletErr(m, "%v", err)
	}
	if t, err = Build(sh); err != nil {
		return filletErr(m, "%v", err)
	}
	if probs := t.Validate(); len(probs) > 0 {
		return filletErr(probs[0].At, "the fillet leaves the solid open: %s", probs[0].Msg)
	}
	return nil
}

// arc is the rational quadratic Bezier from p to q with control c and
// middle weight w: a circular arc when the angle at c is right for it.
func arc(p, c, q geom.Vector, w float64) srf.SBezier {
	sb := srf.SBezierFrom3(p, c, q)
	sb.Weight[1] = w
	return sb
}

// FilletEdges fillets several edges, each a segment from a to b, with
// radius r, one after another. Edges that meet at a vertex are refused:
// a corner where fillets meet needs a blend of its own. Origins are
// origin, origin+1, and so on.
func FilletEdges(sh *srf.SShell, segs [][2]geom.Vector, r float64, origin uint32) error {
	for i := range segs {
		for j := i + 1; j < len(segs); j++ {
			for _, p := range segs[i] {
				for _, q := range segs[j] {
					if p.Equals(q) {
						return filletErr(p, "two of the edges meet here: fillets that meet at a corner are not made yet")
					}
				}
			}
		}
	}
	for i, sg := range segs {
		if err := FilletEdge(sh, sg[0], sg[1], r, origin+uint32(i)); err != nil {
			return err
		}
	}
	return nil
}
