package topo_test

import (
	"math"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/check"
	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/srf"
	"github.com/spacesolve/spacesolve/pkg/topo"
)

func v(x, y, z float64) geom.Vector { return geom.Vector{X: x, Y: y, Z: z} }

// box is a 10 x 20 x 30 box, each face given an identity from 1 up.
func box() *srf.SShell {
	var sh srf.SShell
	sh.MakePrism([]geom.Vector{v(0, 0, 0), v(10, 0, 0), v(10, 20, 0), v(0, 20, 0)}, v(0, 0, 0), v(0, 0, 30),
		core.NewRgbaColor(100, 100, 100, 255))
	for i := range sh.Surface {
		sh.Surface[i].Face = srf.FaceID(i + 1)
	}
	return &sh
}

func build(t *testing.T, sh *srf.SShell) *topo.Topology {
	t.Helper()
	tp, err := topo.Build(sh)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range tp.Validate() {
		t.Errorf("%s at %v", p.Msg, p.At)
	}
	return tp
}

// solid checks the shell's mesh: closed, with the volume wanted, and
// the topology's Euler characteristic the mesh's.
func solid(t *testing.T, sh *srf.SShell, tp *topo.Topology, volume float64) {
	t.Helper()
	var m srf.SMesh
	sh.TriangulateInto(&m)
	r := check.Solid(sh, &m)
	if r.NakedEdges != 0 || r.SelfIntersecting != 0 || math.Abs(r.Volume-volume) > 1e-6*volume {
		t.Errorf("naked %d, crossing %d, volume %g, want %g", r.NakedEdges, r.SelfIntersecting, r.Volume, volume)
	}
	if e, want := tp.Euler(), topo.MeshEuler(&m); e != want {
		t.Errorf("Euler characteristic %d, the mesh's %d", e, want)
	}
}

// faceAt is the face whose outer loop has its vertices all on the
// plane through p with normal n.
func faceAt(t *testing.T, tp *topo.Topology, p, n geom.Vector) int {
	t.Helper()
	for f := range tp.Faces {
		ok := true
		for _, l := range tp.Faces[f].Loops {
			for _, ce := range tp.Loops[l].CoEdges {
				if math.Abs(tp.Vertices[tp.Start(ce)].P.Minus(p).Dot(n)) > 1e-9 {
					ok = false
				}
			}
		}
		if ok {
			return f
		}
	}
	t.Fatalf("no face on the plane at %v", p)
	return -1
}

func edgeBetween(t *testing.T, tp *topo.Topology, a, b geom.Vector) int {
	t.Helper()
	for e := range tp.Edges {
		pa, pb := tp.Vertices[tp.Edges[e].V[0]].P, tp.Vertices[tp.Edges[e].V[1]].P
		if pa.Equals(a) && pb.Equals(b) || pa.Equals(b) && pb.Equals(a) {
			return e
		}
	}
	t.Fatalf("no edge from %v to %v", a, b)
	return -1
}

func TestBox(t *testing.T) {
	sh := box()
	tp := build(t, sh)
	if len(tp.Vertices) != 8 || len(tp.Edges) != 12 || len(tp.Faces) != 6 || tp.Euler() != 2 {
		t.Errorf("V %d E %d F %d", len(tp.Vertices), len(tp.Edges), len(tp.Faces))
	}
	solid(t, sh, tp, 6000)
	for f := range tp.Faces {
		if len(tp.Neighbours(f)) != 4 || len(tp.FaceEdges(f)) != 4 {
			t.Errorf("face %d: %d neighbours, %d edges", f, len(tp.Neighbours(f)), len(tp.FaceEdges(f)))
		}
	}
}

func TestSplitEdge(t *testing.T) {
	sh := box()
	tp := build(t, sh)
	e := edgeBetween(t, tp, v(0, 0, 0), v(0, 0, 30))
	if err := topo.SplitEdge(sh, tp, e, v(0, 0, 12)); err != nil {
		t.Fatal(err)
	}
	tp = build(t, sh)
	if len(tp.Vertices) != 9 || len(tp.Edges) != 13 {
		t.Errorf("V %d E %d", len(tp.Vertices), len(tp.Edges))
	}
	solid(t, sh, tp, 6000)
	if err := topo.SplitEdge(sh, tp, edgeBetween(t, tp, v(0, 0, 0), v(0, 0, 12)), v(5, 5, 5)); err != topo.ErrNotOnEdge {
		t.Errorf("split off the edge: %v", err)
	}
	if err := topo.SplitEdge(sh, tp, edgeBetween(t, tp, v(0, 0, 0), v(0, 0, 12)), v(0, 0, 0)); err != topo.ErrNotOnEdge {
		t.Errorf("split at an end: %v", err)
	}
}

func TestSplitFace(t *testing.T) {
	sh := box()
	tp := build(t, sh)
	// Across the top from the middle of one long side to the other.
	for _, p := range [][2]geom.Vector{{v(0, 0, 30), v(0, 20, 30)}, {v(10, 0, 30), v(10, 20, 30)}} {
		if err := topo.SplitEdge(sh, tp, edgeBetween(t, tp, p[0], p[1]), p[0].Plus(p[1]).ScaledBy(0.5)); err != nil {
			t.Fatal(err)
		}
		tp = build(t, sh)
	}
	top := faceAt(t, tp, v(0, 0, 30), v(0, 0, 1))
	h, err := topo.SplitFace(sh, tp, top, srf.SBezierFrom2(v(0, 10, 30), v(10, 10, 30)))
	if err != nil {
		t.Fatal(err)
	}
	tp = build(t, sh)
	if len(tp.Faces) != 7 || h.V == 0 {
		t.Errorf("%d faces", len(tp.Faces))
	}
	solid(t, sh, tp, 6000)
	// The two halves carry the top's identity: the edges between them
	// and the front face, and the back, are named apart.
	if _, err := topo.SplitFace(sh, tp, top, srf.SBezierFrom2(v(0, 3, 30), v(10, 3, 30))); err == nil {
		t.Error("a cut between points that are not vertices")
	}
}

// TestLocalChamfer chamfers the edge between the top and the front of
// the box with the local edits alone: the faces on both sides of the
// edge and at its ends are cut along the chamfer's lines, the strips
// along the edge taken out, and the chamfer's face put in.
func TestLocalChamfer(t *testing.T) {
	const d = 2.0
	sh := box()
	tp := build(t, sh)
	// The chamfer's corners on the edges at the ends of the edge.
	for _, s := range []struct{ a, b, at geom.Vector }{
		{v(0, 0, 30), v(0, 20, 30), v(0, d, 30)},
		{v(10, 0, 30), v(10, 20, 30), v(10, d, 30)},
		{v(0, 0, 0), v(0, 0, 30), v(0, 0, 30-d)},
		{v(10, 0, 0), v(10, 0, 30), v(10, 0, 30-d)},
	} {
		if err := topo.SplitEdge(sh, tp, edgeBetween(t, tp, s.a, s.b), s.at); err != nil {
			t.Fatal(err)
		}
		tp = build(t, sh)
	}
	// The cuts: along the top, down the front, and across each end.
	cuts := []struct {
		p, n   geom.Vector
		cut    srf.SBezier
		strip  geom.Vector // a point of the part to take out
		normal geom.Vector
	}{
		{v(0, 0, 30), v(0, 0, 1), srf.SBezierFrom2(v(0, d, 30), v(10, d, 30)), v(5, d/2, 30), v(0, 0, 1)},
		{v(0, 0, 0), v(0, 1, 0), srf.SBezierFrom2(v(0, 0, 30-d), v(10, 0, 30-d)), v(5, 0, 30-d/2), v(0, 1, 0)},
		{v(0, 0, 0), v(1, 0, 0), srf.SBezierFrom2(v(0, d, 30), v(0, 0, 30-d)), v(0, d/3, 30-d/3), v(1, 0, 0)},
		{v(10, 0, 0), v(1, 0, 0), srf.SBezierFrom2(v(10, d, 30), v(10, 0, 30-d)), v(10, d/3, 30-d/3), v(1, 0, 0)},
	}
	for _, c := range cuts {
		f := faceAt(t, tp, c.p, c.n)
		if _, err := topo.SplitFace(sh, tp, f, c.cut); err != nil {
			t.Fatal(err)
		}
		tp = build(t, sh)
	}
	solid(t, sh, tp, 6000)
	// Take out the four small pieces at the edge.
	var gone []int
	for _, c := range cuts {
		// The piece is the face whose vertices' centroid is nearest the
		// point.
		best, bestD := -1, math.Inf(1)
		for f := range tp.Faces {
			var sum geom.Vector
			n := 0
			for _, l := range tp.Faces[f].Loops {
				for _, ce := range tp.Loops[l].CoEdges {
					sum = sum.Plus(tp.Vertices[tp.Start(ce)].P)
					n++
				}
			}
			if dd := sum.ScaledBy(1 / float64(n)).Minus(c.strip).Magnitude(); dd < bestD {
				best, bestD = f, dd
			}
		}
		gone = append(gone, best)
	}
	topo.RemoveFaces(sh, tp, gone)
	tp, _ = topo.Build(sh)
	if open := len(tp.Validate()); open == 0 {
		t.Error("taking faces out left the shell closed")
	}
	plane := srf.FromPlane(v(0, d, 30), v(1, 0, 0), v(0, -d, -d).WithMagnitude(1))
	if _, err := topo.AddFace(sh, plane, [][]srf.SBezier{{
		srf.SBezierFrom2(v(0, d, 30), v(10, d, 30)),
		srf.SBezierFrom2(v(10, d, 30), v(10, 0, 30-d)),
		srf.SBezierFrom2(v(10, 0, 30-d), v(0, 0, 30-d)),
		srf.SBezierFrom2(v(0, 0, 30-d), v(0, d, 30)),
	}}); err != nil {
		t.Fatal(err)
	}
	tp = build(t, sh)
	solid(t, sh, tp, 6000-d*d/2*10)
	if len(tp.Faces) != 7 || len(tp.Edges) != 15 || len(tp.Vertices) != 10 {
		t.Errorf("V %d E %d F %d", len(tp.Vertices), len(tp.Edges), len(tp.Faces))
	}
}

func TestReplaceSurface(t *testing.T) {
	sh := box()
	tp := build(t, sh)
	top := faceAt(t, tp, v(0, 0, 30), v(0, 0, 1))
	// The same plane, its parameters the other way: it is turned to face
	// as the old one did.
	if err := topo.ReplaceSurface(sh, tp, top, srf.FromPlane(v(0, 0, 30), v(0, 1, 0), v(1, 0, 0))); err != nil {
		t.Fatal(err)
	}
	tp = build(t, sh)
	solid(t, sh, tp, 6000)
}
