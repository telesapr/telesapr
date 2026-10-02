package topo_test

import (
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/srf"
	"github.com/spacesolve/spacesolve/pkg/topo"
)

func TestNames(t *testing.T) {
	sh := box()
	tp := build(t, sh)
	top := faceAt(t, tp, v(0, 0, 30), v(0, 0, 1))
	left := faceAt(t, tp, v(0, 0, 0), v(1, 0, 0))
	e := edgeBetween(t, tp, v(0, 0, 30), v(0, 20, 30))
	n, err := tp.NameOf(e)
	if err != nil {
		t.Fatal(err)
	}
	if n != topo.MakeName(tp.Faces[left].ID, tp.Faces[top].ID, 0) {
		t.Errorf("name %v", n)
	}
	back, err := topo.ParseName(n.String())
	if err != nil || back != n {
		t.Errorf("%q reads back as %v, %v", n.String(), back, err)
	}
	if got, err := tp.Resolve(n); err != nil || got != e {
		t.Errorf("resolves to %d, %v", got, err)
	}
	if !tp.Straight(e) || tp.Length(e) != 20 {
		t.Errorf("straight %v, length %g", tp.Straight(e), tp.Length(e))
	}

	// Cut the top in two across: the left side now meets the top along
	// two edges, named in order along y.
	for _, p := range [][2]float64{{0, 30}, {10, 30}} {
		if err := topo.SplitEdge(sh, tp, edgeBetween(t, tp, v(p[0], 0, p[1]), v(p[0], 20, p[1])), v(p[0], 10, p[1])); err != nil {
			t.Fatal(err)
		}
		tp = build(t, sh)
	}
	if _, err := topo.SplitFace(sh, tp, faceAt(t, tp, v(0, 0, 30), v(0, 0, 1)), srf.SBezierFrom2(v(0, 10, 30), v(10, 10, 30))); err != nil {
		t.Fatal(err)
	}
	tp = build(t, sh)
	first, err := tp.Resolve(n)
	if err != nil || first != edgeBetween(t, tp, v(0, 0, 30), v(0, 10, 30)) {
		t.Errorf("ordinal 0 is edge %d, %v", first, err)
	}
	n1 := n
	n1.N = 1
	if second, err := tp.Resolve(n1); err != nil || second != edgeBetween(t, tp, v(0, 10, 30), v(0, 20, 30)) {
		t.Errorf("ordinal 1 is edge %d, %v", second, err)
	}
	n1.N = 2
	if _, err := tp.Resolve(n1); err == nil || !strings.Contains(err.Error(), "meet along 2 edges") {
		t.Errorf("ordinal 2: %v", err)
	}
	// The cut joins two faces with one identity: it has no name.
	if _, err := tp.NameOf(edgeBetween(t, tp, v(0, 10, 30), v(10, 10, 30))); err == nil {
		t.Error("an edge inside a face has a name")
	}
	if _, err := tp.Resolve(topo.MakeName(99, tp.Faces[left].ID, 0)); err == nil || !strings.Contains(err.Error(), "no longer in the solid") {
		t.Errorf("a gone face: %v", err)
	}
	// The left side and the right never met.
	right := faceAt(t, tp, v(10, 0, 0), v(1, 0, 0))
	if _, err := tp.Resolve(topo.MakeName(tp.Faces[left].ID, tp.Faces[right].ID, 0)); err == nil || !strings.Contains(err.Error(), "no longer meet") {
		t.Errorf("faces apart: %v", err)
	}
	for _, s := range []string{"", "edge 1 2", "edge x 2 0", "edge 1 2 -1", "face 1 2 0"} {
		if _, err := topo.ParseName(s); err == nil {
			t.Errorf("%q parsed", s)
		}
	}
	if e, d := tp.EdgeAt(v(5, 0.1, 30.1)); e != edgeBetween(t, tp, v(0, 0, 30), v(10, 0, 30)) || d > 0.2 {
		t.Errorf("edge at: %d, %g", e, d)
	}
	if len(tp.FacesWithID(tp.Faces[top].ID)) != 2 {
		t.Error("the top's two halves")
	}
}

// TestFixtureNames: in every fixture's solid, every edge that has a
// name resolves back to itself.
func TestFixtureNames(t *testing.T) {
	named, edges := 0, 0
	for _, f := range fixtures(t) {
		sk := load(t, f)
		if sk == nil {
			continue
		}
		for _, g := range solidGroups(sk) {
			tp, err := topo.Build(&g.RunningShell)
			if err != nil {
				continue
			}
			for e := range tp.Edges {
				edges++
				n, err := tp.NameOf(e)
				if err != nil {
					continue
				}
				named++
				if got, err := tp.Resolve(n); err != nil || got != e {
					t.Errorf("%s g%x: %v resolves to %d, not %d (%v)", f, g.H.V, n, got, e, err)
				}
			}
		}
	}
	t.Logf("%d of %d edges named", named, edges)
}
