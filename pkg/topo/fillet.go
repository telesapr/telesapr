package topo

import (
	"fmt"
	"math"

	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// A fillet rounds a straight edge between two planar faces with a
// cylinder of radius r tangent to both: the faces are cut back along the
// lines where the cylinder touches them, an end face along the arc where
// the cylinder meets it, and the cylinder's patch put in
// (FilletNetwork). The cylinder is exact: a rational quadratic arc swept
// along the edge.

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
// which a SolveSpace surface's normal is: a patch of degree 1 both ways,
// or of a higher degree with every control point on one plane, as a
// lathe's flat faces are, quarters of a disc or a ring.
func (t *Topology) planarNormal(f int) (geom.Vector, bool) {
	s := t.Surface(f)
	if s == nil {
		return geom.Vector{}, false
	}
	n := s.NormalAt(0.5, 0.5)
	if n.Magnitude() == 0 {
		return geom.Vector{}, false
	}
	n = n.WithMagnitude(1)
	for i := 0; i <= s.DegM; i++ {
		for j := 0; j <= s.DegN; j++ {
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
// face square to the filleted edge. With back, the point may lie on the
// edge's line behind v: a concave edge's fillet, filling, runs on past
// a face that ends it on the outside of a corner, as a boss's foot does
// at the boss's side.
func (t *Topology) pointAlong(x, v int, into geom.Vector, s float64, back bool) (geom.Vector, error) {
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
	if back && k < -1e-9 {
		return from.Plus(d.ScaledBy(s / k)), nil
	}
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

// FilletEdge rounds the edge from a to b of the shell with radius r,
// as FilletNetwork does one edge: straight, between two planar faces
// that meet at less than a straight angle with the solid between them,
// ending at corners where it meets one more planar face. The new face's
// surface gets origin as its Origin, so its edges have names.
func FilletEdge(sh *srf.SShell, a, b geom.Vector, r float64, origin uint32) error {
	return FilletNetwork(sh, [][2]geom.Vector{{a, b}}, r, origin)
}

// arc is the rational quadratic Bezier from p to q with control c and
// middle weight w: a circular arc when the angle at c is right for it.
func arc(p, c, q geom.Vector, w float64) srf.SBezier {
	sb := srf.SBezierFrom3(p, c, q)
	sb.Weight[1] = w
	return sb
}

// FilletEdges fillets several edges, each a segment from a to b, with
// radius r, as one edit (FilletNetwork): edges that meet at a corner
// are blended there. Origins are origin, origin+1, and so on.
func FilletEdges(sh *srf.SShell, segs [][2]geom.Vector, r float64, origin uint32) error {
	return FilletNetwork(sh, segs, r, origin)
}
