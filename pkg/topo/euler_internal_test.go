package topo

import "testing"

// loopsFace is a topology of one face whose loops run through the
// given vertices in turn, the first its outer loop and the rest holes,
// each co-edge on an edge of its own.
func loopsFace(loops ...[]int) *Topology {
	t := &Topology{Faces: []Face{{}}}
	nv := 0
	for li, vs := range loops {
		l := Loop{Face: 0, Outer: li == 0}
		for i, v := range vs {
			nv = max(nv, v+1)
			t.Edges = append(t.Edges, Edge{V: [2]int{v, vs[(i+1)%len(vs)]}})
			t.CoEdges = append(t.CoEdges, CoEdge{Edge: len(t.Edges) - 1, Loop: li})
			l.CoEdges = append(l.CoEdges, len(t.CoEdges)-1)
		}
		t.Loops = append(t.Loops, l)
		t.Faces[0].Loops = append(t.Faces[0].Loops, li)
	}
	t.Vertices = make([]Vertex, nv)
	return t
}

// A face's holes as the Euler characteristic counts them: a hole apart
// from the outer loop is one; one touching it at a vertex none (where
// two equal tubes crossing touch tangent on a patch's seam, the face is
// a disc whose boundary passes the vertex twice, as its mesh counts it:
// a sweep's runs at a corner beside a cubic, rounded after it, were
// refused for it, TestSweepSharpCornerBesideCubicFramesSweep); one
// touching it at two vertices, or touching another hole, one each (a
// shell's inside touching another along a line is no closed solid,
// TestShellBlockAndFlushPost).
func TestFaceHolesTouching(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		loops [][]int
		want  int
	}{
		{"apart", [][]int{{0, 1, 2, 3}, {4, 5, 6}}, 1},
		{"touching the outer loop", [][]int{{0, 1, 2, 3}, {1, 5, 6}}, 0},
		{"two apart", [][]int{{0, 1, 2, 3}, {4, 5, 6}, {7, 8, 9}}, 2},
		{"two touching each other", [][]int{{0, 1, 2, 3}, {4, 5, 6}, {6, 8, 9}}, 2},
		{"touching the outer loop twice", [][]int{{0, 1, 2, 3}, {1, 2, 6}}, 1},
		{"one touching, one apart", [][]int{{0, 1, 2, 3}, {2, 5, 6}, {7, 8, 9}}, 1},
	} {
		tp := loopsFace(c.loops...)
		if got := tp.faceHoles(tp.Faces[0]); got != c.want {
			t.Errorf("%s: %d holes, want %d", c.name, got, c.want)
		}
	}
}
