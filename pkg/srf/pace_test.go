package srf

import (
	"reflect"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// TestBooleanPaced checks that a Boolean made into a shell given a pace
// (SShell.PaceBy) calls it between the steps of its loops, on the
// goroutine that made it, makes the shell made without one, and keeps
// the pace until it is taken away.
func TestBooleanPaced(t *testing.T) {
	t.Parallel()
	tol := sweepTol()
	for _, op := range []SurfaceCombineAs{SurfaceUnion, SurfaceDiff, SurfaceInter} {
		want := SShell{Tol: tol}
		want.MakeFromBoolean(lathedTube(0, 5, 0, 20, tol), crossedCylinder(4, 0.7, 45, tol), op)

		got := SShell{Tol: tol}
		paces := 0
		got.PaceBy(func() { paces++ })
		got.MakeFromBoolean(lathedTube(0, 5, 0, 20, tol), crossedCylinder(4, 0.7, 45, tol), op)
		if paces == 0 {
			t.Errorf("%v: the Boolean never paced", op)
		}
		if got.Pace() == nil {
			t.Errorf("%v: the shell lost its pace in the Boolean", op)
		}
		got.PaceBy(nil)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%v: paced, the Boolean made another shell", op)
		}
	}
}

// TestPaceGoesWithShell checks that the pace a shell was given goes to
// the shells made from it: a Boolean of it with another, unpaced, made
// into a shell given none paces and keeps the pace, as do a copy and a
// clone; and that its triangulation paces and meshes as it does unpaced.
func TestPaceGoesWithShell(t *testing.T) {
	t.Parallel()
	tol := sweepTol()
	paces := 0
	pace := func() { paces++ }

	a := lathedTube(0, 5, 0, 20, tol)
	a.PaceBy(pace)
	var out SShell
	out.MakeFromDifferenceOf(a, crossedCylinder(4, 0.7, 45, tol))
	if paces == 0 || out.Pace() == nil {
		t.Fatalf("a Boolean of a paced shell paced %d times, kept pace %t", paces, out.Pace() != nil)
	}

	var cp SShell
	cp.MakeFromCopyOf(&out)
	if cp.Pace() == nil {
		t.Error("a copy of a paced shell has no pace")
	}
	if cl := out.Clone(); cl.Pace() == nil {
		t.Error("a clone of a paced shell has no pace")
	}

	var plain SMesh
	unpaced := out.Clone()
	unpaced.PaceBy(nil)
	unpaced.TriangulateInto(&plain)
	paces = 0
	var paced SMesh
	out.TriangulateInto(&paced)
	if paces < len(out.Surface) {
		t.Errorf("the triangulation paced %d times over %d surfaces", paces, len(out.Surface))
	}
	if !reflect.DeepEqual(plain.Triangles, paced.Triangles) {
		t.Error("paced, the triangulation made another mesh")
	}
}

// TestOutlinesPaced checks that a mesh's outlines and k-d tree, made
// with a pace, call it and come out as made without one.
func TestOutlinesPaced(t *testing.T) {
	t.Parallel()
	tol := sweepTol()
	var sh SShell
	sh.MakeFromDifferenceOf(lathedTube(0, 5, 0, 20, tol), crossedCylinder(4, 0.7, 45, tol))
	var m SMesh
	sh.TriangulateInto(&m)
	var want, got SOutlineList
	m.MakeOutlinesInto(&want, EdgeSharpOrFace)
	paces := 0
	m.MakeOutlinesPacedInto(&got, EdgeSharpOrFace, func() { paces++ })
	if paces == 0 {
		t.Error("the outlines never paced")
	}
	if !reflect.DeepEqual(got, want) {
		t.Error("paced, the outlines came out otherwise")
	}
	// Enough triangles for the tree to pace: the mesh's, moved up thrice.
	var big SMesh
	for k := range 3 {
		for _, tr := range m.Triangles {
			up := geom.Vector{Z: 30 * float64(k)}
			tr.A, tr.B, tr.C = tr.A.Plus(up), tr.B.Plus(up), tr.C.Plus(up)
			big.Triangles = append(big.Triangles, tr)
		}
	}
	paces = 0
	if kd := SKdNodeFromPaced(&big, func() { paces++ }); !reflect.DeepEqual(kd, SKdNodeFrom(&big)) || paces == 0 {
		t.Errorf("paced %d times, the k-d tree came out otherwise", paces)
	}
}
