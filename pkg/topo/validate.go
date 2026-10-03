package topo

import (
	"fmt"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Problem is a way the topology falls short of a closed, manifold
// solid, with a point where it does.
type Problem struct {
	Msg string
	At  geom.Vector
	// Edge is the edge at fault, -1 for none, for the user interface
	// to mark it.
	Edge int
}

// Validate lists what is wrong with the topology: an edge not used
// exactly twice, in opposite directions, by two loops; a loop that
// doesn't close; a co-edge in no loop. Euler's formula is checked
// apart, against the mesh, by Euler and MeshEuler.
func (t *Topology) Validate() []Problem {
	var out []Problem
	for i := range t.Edges {
		e := &t.Edges[i]
		at := t.Vertices[e.V[0]].P
		switch len(e.CoEdges) {
		case 2:
			a, b := &t.CoEdges[e.CoEdges[0]], &t.CoEdges[e.CoEdges[1]]
			if a.Reversed == b.Reversed {
				out = append(out, Problem{fmt.Sprintf("edge %d is run the same way by both its faces", i), at, i})
			}
		case 0, 1:
			out = append(out, Problem{fmt.Sprintf("edge %d is used by %d co-edges, not 2", i, len(e.CoEdges)), at, i})
		default:
			// Where two parts of a solid touch along a line, as two holes
			// at a common corner do, or two blocks edge to edge: a
			// non-manifold edge, which no feature can work on (roadmap
			// 14.3).
			b := t.Vertices[e.V[1]].P
			out = append(out, Problem{fmt.Sprintf("edge %d is used by %d co-edges, not 2: %d faces meet along "+
				"it, from (%.4g, %.4g, %.4g) to (%.4g, %.4g, %.4g), where two parts of the solid touch",
				i, len(e.CoEdges), len(e.CoEdges), at.X, at.Y, at.Z, b.X, b.Y, b.Z), at, i})
		}
	}
	for i := range t.Loops {
		l := &t.Loops[i]
		for k, ce := range l.CoEdges {
			next := l.CoEdges[(k+1)%len(l.CoEdges)]
			if t.End(ce) != t.Start(next) {
				out = append(out, Problem{fmt.Sprintf("loop %d does not close", i), t.Vertices[t.End(ce)].P, -1})
				break
			}
		}
	}
	for i := range t.CoEdges {
		if t.CoEdges[i].Loop < 0 {
			out = append(out, Problem{fmt.Sprintf("co-edge %d is in no loop", i), t.Vertices[t.Start(i)].P, -1})
		}
	}
	return out
}

// Euler is V - E + F - H, the Euler characteristic of the solid by the
// Euler-Poincaré formula, H being the holes in faces: twice the number
// of shells less their genus, the same as MeshEuler gives the mesh.
func (t *Topology) Euler() int {
	holes := 0
	for _, l := range t.Loops {
		if !l.Outer {
			holes++
		}
	}
	return len(t.Vertices) - len(t.Edges) + len(t.Faces) - holes
}

// MeshEuler is V - E + F of a triangle mesh, its vertices merged where
// they are within LengthEps.
func MeshEuler(m *srf.SMesh) int {
	t := &Topology{}
	index := map[vertexKey][]int{}
	type ek struct{ a, b int }
	edges := map[ek]bool{}
	for _, tr := range m.Triangles {
		v := [3]int{t.vertexAt(tr.A, index), t.vertexAt(tr.B, index), t.vertexAt(tr.C, index)}
		for k := 0; k < 3; k++ {
			a, b := v[k], v[(k+1)%3]
			edges[ek{min(a, b), max(a, b)}] = true
		}
	}
	return len(t.Vertices) - len(edges) + len(m.Triangles)
}
