package topo

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A shell hollows a solid, leaving walls of one thickness, open where
// faces are taken away. It takes solids of planar faces, and prisms
// whose sides are flat or extrusions of arcs (shellprism.go). Of flat
// faces, the inner solid has the outer one's topology, each face moved in by the
// thickness but those taken away, and each vertex where its faces'
// moved planes meet. The shell is the solid less the inner one, by a
// Boolean.

// ShellError is why a solid can't be hollowed, with where.
type ShellError struct {
	Msg string
	At  geom.Vector
}

func (e *ShellError) Error() string { return e.Msg }

func shellErr(at geom.Vector, format string, args ...any) error {
	return &ShellError{Msg: fmt.Sprintf(format, args...), At: at}
}

// ShellSolid hollows sh to walls thick, taking away faces open (indices
// into the solid's topology) to leave it open there. The faces of the
// cavity get origins from origin up.
func ShellSolid(sh *srf.SShell, open []int, thick float64, origin uint32) (*srf.SShell, error) {
	if thick <= 0 || math.IsNaN(thick) || math.IsInf(thick, 0) {
		return nil, fmt.Errorf("a shell's thickness must be more than zero, not %g", thick)
	}
	t, err := Build(sh)
	if err != nil {
		return nil, err
	}
	if probs := t.Validate(); len(probs) > 0 {
		return nil, shellErr(probs[0].At, "the solid is not closed: %s", probs[0].Msg)
	}
	if len(open) == 0 {
		return nil, errors.New("there are no faces to take away: a shell is open where faces are taken away")
	}
	opened := map[int]bool{}
	for _, f := range open {
		if f < 0 || f >= len(t.Faces) {
			return nil, fmt.Errorf("there is no face %d of the solid", f)
		}
		opened[f] = true
	}
	if len(opened) == len(t.Faces) {
		return nil, errors.New("every face is taken away, which leaves no wall")
	}
	if out, ok, err := shellPrism(t, sh, opened, thick, origin); ok {
		return out, err
	}

	// Each face's plane, moved in by the thickness.
	normal := make([]geom.Vector, len(t.Faces))
	offset := make([]float64, len(t.Faces))
	for f := range t.Faces {
		n, ok := t.planarNormal(f)
		if !ok {
			return nil, shellErr(t.faceAnchor(f), "a face of the solid is curved; a shell hollows solids of flat faces, and prisms of lines and arcs")
		}
		// A face taken away stays where it is: the inside meets it, and
		// the Boolean takes it away with the inside, as a cut flush with
		// a face does. Moved out, it would slide its neighbours' corners
		// along, turning short edges round outside the solid.
		d := -thick
		if opened[f] {
			d = 0
		}
		normal[f] = n
		offset[f] = n.Dot(t.Surface(f).Ctrl[0][0]) + d
	}
	for e := range t.Edges {
		if !t.Straight(e) {
			return nil, shellErr(t.Edges[e].Points[0], "an edge of the solid is curved; a shell hollows solids of flat faces, and prisms of lines and arcs")
		}
	}

	// Each vertex where its faces' moved planes meet: one point, or at a
	// vertex of four faces whose planes don't meet in one, two points
	// joined by a new edge (roadmap 15.3).
	moved := make([]movedPoints, len(t.Vertices))
	for v := range t.Vertices {
		mp, err := t.movedVertex(v, normal, offset)
		if err != nil {
			return nil, err
		}
		moved[v] = mp
	}
	// at is where vertex v goes in face f beside face g.
	at := func(v, f, g int) geom.Vector { return moved[v].in(f, g) }
	other := func(e, f int) int {
		fs := t.EdgeFaces(e)
		if fs[0] == f {
			return fs[1]
		}
		return fs[0]
	}

	// A thickness that turns an edge round takes a face away.
	for e := range t.Edges {
		ed := &t.Edges[e]
		fs := t.EdgeFaces(e)
		was := t.Vertices[ed.V[1]].P.Minus(t.Vertices[ed.V[0]].P)
		now := at(ed.V[1], fs[0], fs[1]).Minus(at(ed.V[0], fs[0], fs[1]))
		if now.Magnitude() < geom.LengthEps || now.Dot(was) <= 0 {
			return nil, shellErr(t.Vertices[ed.V[0]].P,
				"a thickness of %g makes the walls meet, closing up a face", thick)
		}
	}

	inner := &srf.SShell{}
	for f := range t.Faces {
		var loops [][]srf.SBezier
		var pts []geom.Vector
		for _, l := range t.outerFirst(f) {
			var loop []srf.SBezier
			ces := t.Loops[l].CoEdges
			for k, ce := range ces {
				g := other(t.CoEdges[ce].Edge, f)
				a, b := at(t.Start(ce), f, g), at(t.End(ce), f, g)
				loop = append(loop, srf.SBezierFrom2(a, b))
				pts = append(pts, a)
				// Where the vertex was split, the new edge between
				// this side's point and the next one's.
				next := ces[(k+1)%len(ces)]
				if c := at(t.End(ce), f, other(t.CoEdges[next].Edge, f)); !c.Equals(b) {
					loop = append(loop, srf.SBezierFrom2(b, c))
					pts = append(pts, b)
				}
			}
			loops = append(loops, loop)
		}
		s := planeOver(pts, normal[f])
		s.Color = t.Surface(f).Color
		s.Origin = origin + uint32(f)
		if _, err := AddFace(inner, s, loops); err != nil {
			return nil, shellErr(t.faceAnchor(f), "the inner surface can't be built: %v", err)
		}
	}
	it, err := Build(inner)
	if err != nil {
		return nil, err
	}
	if probs := it.Validate(); len(probs) > 0 {
		return nil, shellErr(probs[0].At, "the inner surface is not closed: %s", probs[0].Msg)
	}

	out := &srf.SShell{}
	out.MakeFromDifferenceOf(sh, inner)
	if out.BooleanFailed {
		return nil, errors.New("the Boolean that takes the inside away failed")
	}
	return out, nil
}

// outerFirst are a face's loops, its outer one first.
func (t *Topology) outerFirst(f int) []int {
	var out []int
	for _, l := range t.Faces[f].Loops {
		if t.Loops[l].Outer {
			out = append([]int{l}, out...)
		} else {
			out = append(out, l)
		}
	}
	return out
}

// faceAnchor is a point of face f, a vertex of its first loop.
func (t *Topology) faceAnchor(f int) geom.Vector {
	for _, l := range t.Faces[f].Loops {
		if ces := t.Loops[l].CoEdges; len(ces) > 0 {
			return t.Vertices[t.Start(ces[0])].P
		}
	}
	return geom.Vector{}
}

// movedPoints is where a vertex goes: one point, or, split, two, each
// on the planes of three of its four faces.
type movedPoints struct {
	p     [2]geom.Vector
	faces [2][3]int
	split bool
}

// in is the point face f takes beside face g: the one on both planes.
func (m movedPoints) in(f, g int) geom.Vector {
	if !m.split {
		return m.p[0]
	}
	for k := range 2 {
		hasF, hasG := false, false
		for _, x := range m.faces[k] {
			hasF = hasF || x == f
			hasG = hasG || x == g
		}
		if hasF && hasG {
			return m.p[k]
		}
	}
	return m.p[0]
}

// movedVertex is where the moved planes of the faces round vertex v
// meet. Three faces meet in a point; two, along a line, on which the
// vertex keeps its place along the edge. Four faces whose moved planes
// don't meet in one point, as at an off-centre pyramid's apex, meet in
// two: two opposite faces of the four come to share a new edge between
// them, each end on the planes of three faces, the pair taken whose
// ends lie inside the fourth face's plane. More faces must meet in one
// point again.
func (t *Topology) movedVertex(v int, normal []geom.Vector, offset []float64) (movedPoints, error) {
	p := t.Vertices[v].P
	var fs []int
	seen := map[int]bool{}
	adjacent := map[[2]int]bool{}
	for _, e := range t.Vertices[v].Edges {
		ef := t.EdgeFaces(e)
		adjacent[[2]int{ef[0], ef[1]}], adjacent[[2]int{ef[1], ef[0]}] = true, true
		for _, f := range ef {
			if f >= 0 && !seen[f] {
				seen[f] = true
				fs = append(fs, f)
			}
		}
	}
	one := func(q geom.Vector) movedPoints { return movedPoints{p: [2]geom.Vector{q, q}} }
	switch {
	case len(fs) < 2:
		return one(p), shellErr(p, "a vertex of the solid lies on fewer than two faces")
	case len(fs) == 2:
		a, b := fs[0], fs[1]
		along := normal[a].Cross(normal[b])
		if along.Magnitude() < 1e-9 {
			return one(p), shellErr(p, "two faces meet at a vertex in one plane")
		}
		return one(meet3(normal[a], offset[a], normal[b], offset[b], along.WithMagnitude(1), along.WithMagnitude(1).Dot(p))), nil
	}
	// The three most independent planes fix the point; the rest must
	// pass through it.
	best, bi := 0.0, [3]int{}
	for i := range len(fs) {
		for j := i + 1; j < len(fs); j++ {
			for k := j + 1; k < len(fs); k++ {
				d := math.Abs(normal[fs[i]].Dot(normal[fs[j]].Cross(normal[fs[k]])))
				if d > best {
					best, bi = d, [3]int{fs[i], fs[j], fs[k]}
				}
			}
		}
	}
	if best < 1e-9 {
		return one(p), shellErr(p, "the faces at a vertex don't meet in a point")
	}
	q := meet3(normal[bi[0]], offset[bi[0]], normal[bi[1]], offset[bi[1]], normal[bi[2]], offset[bi[2]])
	on := func(f int, x geom.Vector) float64 { return normal[f].Dot(x) - offset[f] }
	meets := true
	for _, f := range fs {
		if math.Abs(on(f, q)) > 10*geom.LengthEps {
			meets = false
		}
	}
	if meets {
		return one(q), nil
	}
	if len(fs) != 4 {
		return one(p), shellErr(p, "%d faces meet at a vertex and, moved in, don't meet in one point; "+
			"a shell splits vertices of four faces only", len(fs))
	}
	// The two pairs of opposite faces; for each, the ends of the new
	// edge, and whether they lie inside the plane of the face they
	// leave out.
	for i := 1; i < 4; i++ {
		if adjacent[[2]int{fs[0], fs[i]}] {
			continue
		}
		a, c := fs[0], fs[i]
		var bd []int
		for _, f := range fs[1:] {
			if f != c {
				bd = append(bd, f)
			}
		}
		for _, pair := range [][2]int{{a, c}, {bd[0], bd[1]}} {
			x, y := pair[0], pair[1]
			var rest []int
			for _, f := range fs {
				if f != x && f != y {
					rest = append(rest, f)
				}
			}
			p0 := meet3(normal[x], offset[x], normal[y], offset[y], normal[rest[0]], offset[rest[0]])
			p1 := meet3(normal[x], offset[x], normal[y], offset[y], normal[rest[1]], offset[rest[1]])
			if on(rest[1], p0) <= 10*geom.LengthEps && on(rest[0], p1) <= 10*geom.LengthEps {
				return movedPoints{
					p:     [2]geom.Vector{p0, p1},
					faces: [2][3]int{{x, y, rest[0]}, {x, y, rest[1]}},
					split: true,
				}, nil
			}
		}
		break
	}
	return one(p), shellErr(p, "4 faces meet at a vertex and, moved in, meet in no edge a shell can make")
}

// meet3 is the point on the three planes n·x = d.
func meet3(n1 geom.Vector, d1 float64, n2 geom.Vector, d2 float64, n3 geom.Vector, d3 float64) geom.Vector {
	den := n1.Dot(n2.Cross(n3))
	return n2.Cross(n3).ScaledBy(d1).
		Plus(n3.Cross(n1).ScaledBy(d2)).
		Plus(n1.Cross(n2).ScaledBy(d3)).
		ScaledBy(1 / den)
}
