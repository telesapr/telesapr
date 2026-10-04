package topo

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Name names an edge of a solid so that the name outlives a
// regeneration: by the identities of the two faces it separates, the
// lower first, and its place among the edges between those two faces,
// ordered by where they lie. A face's identity is the sketch entity
// that made it (srf.FaceID), which an edit upstream keeps; positions
// are not names, since an edit moves them.
//
// A name stops resolving, visibly, when a face goes, or when the faces
// meet along another number of edges than they did when it was made:
// it never moves to another edge between other faces, nor to another
// piece where an edit split or joined the edges between the same two.
// The geometric order keeps the rest from moving: edges between two
// flat faces lie on one line, along which an edit keeps their order.
type Name struct {
	A, B srf.FaceID
	N    int
	// Of is how many edges the two faces met along when the name was
	// made; 0 for a name made before names counted them, which
	// resolves by its ordinal alone.
	Of int
}

// String is the name as files and scripts write it: "edge A B N of M",
// handles in hex, or "edge A B N" for a name that doesn't count.
func (n Name) String() string {
	if n.Of > 0 {
		return fmt.Sprintf("edge %x %x %d of %d", uint32(n.A), uint32(n.B), n.N, n.Of)
	}
	return fmt.Sprintf("edge %x %x %d", uint32(n.A), uint32(n.B), n.N)
}

// Less orders names by their faces' identities, then their ordinals.
func (a Name) Less(b Name) bool {
	if a.A != b.A {
		return a.A < b.A
	}
	if a.B != b.B {
		return a.B < b.B
	}
	if a.N != b.N {
		return a.N < b.N
	}
	return a.Of < b.Of
}

// ParseName reads a name as String writes it.
func ParseName(s string) (Name, error) {
	f := strings.Fields(s)
	bad := fmt.Errorf("%q is not an edge name: it is \"edge\", two face handles in hex, an ordinal "+
		"and \"of\" with a count", s)
	if (len(f) != 4 && len(f) != 6) || f[0] != "edge" {
		return Name{}, bad
	}
	a, err1 := strconv.ParseUint(f[1], 16, 32)
	b, err2 := strconv.ParseUint(f[2], 16, 32)
	n, err3 := strconv.Atoi(f[3])
	if err1 != nil || err2 != nil || err3 != nil || n < 0 {
		return Name{}, bad
	}
	name := MakeName(srf.FaceID(a), srf.FaceID(b), n)
	if len(f) == 6 {
		of, err := strconv.Atoi(f[5])
		if f[4] != "of" || err != nil || of <= n {
			return Name{}, bad
		}
		name.Of = of
	}
	return name, nil
}

// MakeName is the name of ordinal n of the edges between faces a and
// b, in either order.
func MakeName(a, b srf.FaceID, n int) Name {
	if b < a {
		a, b = b, a
	}
	return Name{A: a, B: b, N: n}
}

// EdgeFaces are the faces on the two sides of an edge, through its
// first two co-edges; -1 where there is none.
func (t *Topology) EdgeFaces(e int) [2]int {
	out := [2]int{-1, -1}
	for i, ce := range t.Edges[e].CoEdges {
		if i >= 2 {
			break
		}
		if l := t.CoEdges[ce].Loop; l >= 0 {
			out[i] = t.Loops[l].Face
		}
	}
	return out
}

// pairOf is the face identities an edge separates, lower first, and
// whether both have one.
func (t *Topology) pairOf(e int) (a, b srf.FaceID, ok bool) {
	fs := t.EdgeFaces(e)
	if fs[0] < 0 || fs[1] < 0 {
		return 0, 0, false
	}
	a, b = t.Faces[fs[0]].ID, t.Faces[fs[1]].ID
	if b < a {
		a, b = b, a
	}
	return a, b, a != 0 && b != 0 && a != b
}

// between lists the edges between the faces with identities a and b,
// in their geometric order (see sortBetween).
func (t *Topology) between(a, b srf.FaceID) []int {
	return t.byPair[[2]srf.FaceID{a, b}]
}

// nameEdges lists the edges between every pair of identities, in
// order, once for the topology.
func (t *Topology) nameEdges() {
	t.byPair = map[[2]srf.FaceID][]int{}
	for e := range t.Edges {
		if a, b, ok := t.pairOf(e); ok {
			k := [2]srf.FaceID{a, b}
			t.byPair[k] = append(t.byPair[k], e)
		}
	}
	for k, es := range t.byPair {
		t.byPair[k] = t.sortBetween(es)
	}
}

// sortBetween puts edges between one pair of faces in their geometric
// order: by their midpoints along the direction in which those spread
// most, then along the others.
func (t *Topology) sortBetween(es []int) []int {
	if len(es) < 2 {
		return es
	}
	mids := make([]geom.Vector, len(es))
	lo := geom.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	hi := lo.ScaledBy(-1)
	for i, e := range es {
		mids[i] = midpoint(t.Edges[e].Points)
		lo.X, lo.Y, lo.Z = math.Min(lo.X, mids[i].X), math.Min(lo.Y, mids[i].Y), math.Min(lo.Z, mids[i].Z)
		hi.X, hi.Y, hi.Z = math.Max(hi.X, mids[i].X), math.Max(hi.Y, mids[i].Y), math.Max(hi.Z, mids[i].Z)
	}
	span := hi.Minus(lo)
	// The axes by spread, widest first.
	axes := []int{0, 1, 2}
	sort.SliceStable(axes, func(i, j int) bool { return span.Element(axes[i]) > span.Element(axes[j]) })
	tol := 1e-6 * (1 + span.Magnitude())
	order := make([]int, len(es))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		p, q := mids[order[i]], mids[order[j]]
		for _, ax := range axes {
			if d := p.Element(ax) - q.Element(ax); math.Abs(d) > tol {
				return d < 0
			}
		}
		return false
	})
	out := make([]int, len(es))
	for i, k := range order {
		out[i] = es[k]
	}
	return out
}

// NameOf is the name of edge e. An edge with a face that has no
// identity, such as one a Boolean's tool made, has none, nor has a
// seam. Ordinals count seams all the same, so a name made before seams
// were found names the same edge.
func (t *Topology) NameOf(e int) (Name, error) {
	if t.Edges[e].Seam {
		return Name{}, fmt.Errorf("edge %d is a seam between two patches of one face", e)
	}
	a, b, ok := t.pairOf(e)
	if !ok {
		return Name{}, fmt.Errorf("edge %d does not lie between two faces that come from the sketch", e)
	}
	es := t.between(a, b)
	for i, x := range es {
		if x == e {
			return Name{A: a, B: b, N: i, Of: len(es)}, nil
		}
	}
	return Name{}, fmt.Errorf("edge %d is not between its own faces", e)
}

// Resolve is the edge a name names, or an error that says why there
// is none: a face gone, or fewer edges between the two faces.
func (t *Topology) Resolve(n Name) (int, error) {
	es := t.between(n.A, n.B)
	if n.N < len(es) && (n.Of == 0 || n.Of == len(es)) {
		return es[n.N], nil
	}
	hasA, hasB := false, false
	for _, f := range t.Faces {
		hasA = hasA || f.ID == n.A
		hasB = hasB || f.ID == n.B
	}
	switch {
	case !hasA && !hasB:
		return -1, fmt.Errorf("faces %x and %x are no longer in the solid", uint32(n.A), uint32(n.B))
	case !hasA:
		return -1, fmt.Errorf("face %x is no longer in the solid", uint32(n.A))
	case !hasB:
		return -1, fmt.Errorf("face %x is no longer in the solid", uint32(n.B))
	case len(es) == 0:
		return -1, fmt.Errorf("faces %x and %x no longer meet", uint32(n.A), uint32(n.B))
	case n.Of > 0:
		return -1, fmt.Errorf("faces %x and %x now meet along %d edges, not the %d the name counted, "+
			"so which of them it names is not known", uint32(n.A), uint32(n.B), len(es), n.Of)
	}
	return -1, fmt.Errorf("faces %x and %x now meet along %d edges, so there is no edge %d of them",
		uint32(n.A), uint32(n.B), len(es), n.N)
}

// EdgeAt is the edge whose points come nearest p, and that distance;
// with lead, the lead piece of the solid's edge it is a piece of,
// seams not counting.
func (t *Topology) EdgeAt(p geom.Vector, lead bool) (int, float64) {
	best, bestD := -1, math.Inf(1)
	for e := range t.Edges {
		if lead && t.Edges[e].Seam {
			continue
		}
		if d := distanceToPolyline(p, t.Edges[e].Points); d < bestD {
			best, bestD = e, d
		}
	}
	if lead && best >= 0 {
		best = t.Lead(best)
	}
	return best, bestD
}

// FaceAt is the face nearest p that p lies over, its closest point on
// the face's surface inside the face's outer loop and outside its
// holes, and that distance; -1 when p lies over no face.
func (t *Topology) FaceAt(p geom.Vector) (int, float64) {
	best, bestD := -1, math.Inf(1)
	for f := range t.Faces {
		s := t.Surface(f)
		if s == nil {
			continue
		}
		var u, v float64
		s.ClosestPointToUV(p, &u, &v, true)
		d := s.PointAt(u, v).Minus(p).Magnitude()
		if d >= bestD {
			continue
		}
		uv := geom.Vector{X: u, Y: v}
		inside := true
		for _, l := range t.Faces[f].Loops {
			if pointInPolygon(uv, t.loopUV(s, t.Loops[l].CoEdges)) != t.Loops[l].Outer {
				inside = false
				break
			}
		}
		if inside {
			best, bestD = f, d
		}
	}
	return best, bestD
}

// Straight reports whether an edge is a line segment.
func (t *Topology) Straight(e int) bool {
	pts := t.Edges[e].Points
	if len(pts) < 2 {
		return false
	}
	a, b := pts[0], pts[len(pts)-1]
	d := b.Minus(a)
	if d.Magnitude() < geom.LengthEps {
		return false
	}
	for _, p := range pts[1 : len(pts)-1] {
		if p.DistanceToLine(a, d) > 10*geom.LengthEps {
			return false
		}
	}
	return true
}

// Length is an edge's length along its points.
func (t *Topology) Length(e int) float64 {
	l := 0.0
	pts := t.Edges[e].Points
	for i := 1; i < len(pts); i++ {
		l += pts[i].Minus(pts[i-1]).Magnitude()
	}
	return l
}

// FaceEdges are the edges round a face, each once.
func (t *Topology) FaceEdges(f int) []int {
	var out []int
	seen := map[int]bool{}
	for _, l := range t.Faces[f].Loops {
		for _, ce := range t.Loops[l].CoEdges {
			if e := t.CoEdges[ce].Edge; !seen[e] {
				seen[e] = true
				out = append(out, e)
			}
		}
	}
	return out
}

// Neighbours are the faces that share an edge with face f.
func (t *Topology) Neighbours(f int) []int {
	var out []int
	seen := map[int]bool{f: true}
	for _, e := range t.FaceEdges(f) {
		for _, g := range t.EdgeFaces(e) {
			if g >= 0 && !seen[g] {
				seen[g] = true
				out = append(out, g)
			}
		}
	}
	return out
}

// FacesWithID are the faces that carry identity id.
func (t *Topology) FacesWithID(id srf.FaceID) []int {
	var out []int
	for f := range t.Faces {
		if t.Faces[f].ID == id {
			out = append(out, f)
		}
	}
	return out
}

// EdgeCurve is edge e's exact curve from its V[0] to its V[1], or for a
// Boolean's curve with no exact one, the arc through its points when
// they lie on one; false when there is neither.
func (t *Topology) EdgeCurve(e int) (srf.SBezier, bool) {
	ed := &t.Edges[e]
	a, b := t.Vertices[ed.V[0]].P, t.Vertices[ed.V[1]].P
	if c := t.shell.FindCurve(ed.Curve); c != nil && c.IsExact {
		sb := c.Exact
		switch {
		case sb.Start().Equals(a) && sb.Finish().Equals(b):
			return sb, true
		case sb.Start().Equals(b) && sb.Finish().Equals(a):
			sb.Reverse()
			return sb, true
		}
		return srf.SBezier{}, false
	}
	if n, ok := planeNormalOf(ed.Points); ok {
		return arcThrough(ed.Points, n)
	}
	return srf.SBezier{}, false
}
