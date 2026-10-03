package topo

import (
	"errors"
	"fmt"
	"math"

	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// A shell hollows a solid, leaving walls of one thickness, open where
// faces are taken away. This version takes solids of planar faces: the
// inner solid has the outer one's topology, each face moved in by the
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

	// Each face's plane, moved in by the thickness.
	normal := make([]geom.Vector, len(t.Faces))
	offset := make([]float64, len(t.Faces))
	for f := range t.Faces {
		n, ok := t.planarNormal(f)
		if !ok {
			return nil, shellErr(t.faceAnchor(f), "a face of the solid is curved; a shell hollows solids of flat faces in this version")
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
			return nil, shellErr(t.Edges[e].Points[0], "an edge of the solid is curved; a shell hollows solids of flat faces in this version")
		}
	}

	// Each vertex where its faces' moved planes meet.
	moved := make([]geom.Vector, len(t.Vertices))
	for v := range t.Vertices {
		p, err := t.movedVertex(v, normal, offset)
		if err != nil {
			return nil, err
		}
		moved[v] = p
	}

	// A thickness that turns an edge round takes a face away.
	for e := range t.Edges {
		ed := &t.Edges[e]
		was := t.Vertices[ed.V[1]].P.Minus(t.Vertices[ed.V[0]].P)
		now := moved[ed.V[1]].Minus(moved[ed.V[0]])
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
			for _, ce := range t.Loops[l].CoEdges {
				a, b := moved[t.Start(ce)], moved[t.End(ce)]
				loop = append(loop, srf.SBezierFrom2(a, b))
				pts = append(pts, a)
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

// movedVertex is where the moved planes of the faces round vertex v
// meet. Three faces meet in a point; two, along a line, on which the
// vertex keeps its place along the edge; more must meet in one point
// again, or the shell would need new edges, which it doesn't make.
func (t *Topology) movedVertex(v int, normal []geom.Vector, offset []float64) (geom.Vector, error) {
	p := t.Vertices[v].P
	var fs []int
	seen := map[int]bool{}
	for _, e := range t.Vertices[v].Edges {
		for _, f := range t.EdgeFaces(e) {
			if f >= 0 && !seen[f] {
				seen[f] = true
				fs = append(fs, f)
			}
		}
	}
	switch {
	case len(fs) < 2:
		return p, shellErr(p, "a vertex of the solid lies on fewer than two faces")
	case len(fs) == 2:
		a, b := fs[0], fs[1]
		along := normal[a].Cross(normal[b])
		if along.Magnitude() < 1e-9 {
			return p, shellErr(p, "two faces meet at a vertex in one plane")
		}
		return meet3(normal[a], offset[a], normal[b], offset[b], along.WithMagnitude(1), along.WithMagnitude(1).Dot(p)), nil
	}
	// The three most independent planes fix the point; the rest must
	// pass through it.
	best, bi := 0.0, [3]int{}
	for i := 0; i < len(fs); i++ {
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
		return p, shellErr(p, "the faces at a vertex don't meet in a point")
	}
	q := meet3(normal[bi[0]], offset[bi[0]], normal[bi[1]], offset[bi[1]], normal[bi[2]], offset[bi[2]])
	for _, f := range fs {
		if math.Abs(normal[f].Dot(q)-offset[f]) > 10*geom.LengthEps {
			return p, shellErr(p, "%d faces meet at a vertex and, moved in, don't meet in one point; a shell needs vertices of three faces", len(fs))
		}
	}
	return q, nil
}

// meet3 is the point on the three planes n·x = d.
func meet3(n1 geom.Vector, d1 float64, n2 geom.Vector, d2 float64, n3 geom.Vector, d3 float64) geom.Vector {
	den := n1.Dot(n2.Cross(n3))
	return n2.Cross(n3).ScaledBy(d1).
		Plus(n3.Cross(n1).ScaledBy(d2)).
		Plus(n1.Cross(n2).ScaledBy(d3)).
		ScaledBy(1 / den)
}
