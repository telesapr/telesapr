package sketch_test

import (
	"math"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// fullLengthThread makes a full-length M8 thread on the face of a rod
// of radius 4 and the height, from its upper end down through its
// lower, with o's other options.
func fullLengthThread(t *testing.T, height float64, o sketch.ThreadOptions) (*sketch.Sketch, core.HGroup, core.HGroup) {
	t.Helper()
	sk, part := rodPart(t, 4, height)
	face, _ := roundFace(t, sk, part)
	p, err := sk.ThreadPlaceOnFace(face, false)
	if err != nil {
		t.Fatal(err)
	}
	o.FullLength = true
	res, err := sk.AddThreadOnFace(p, threadCase{"", 4, height, "M8", o}.options(t))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Sound {
		t.Fatal("not sound")
	}
	return sk, part, res.Group
}

// resize makes the rod of part height mm high and regenerates.
func resize(sk *sketch.Sketch, part core.HGroup, height float64) {
	sk.MustParam(part.Param(2)).Val = height / 2
	sk.MarkGroupDirty(part, false)
	sk.GenerateAllTyped(sketch.GenerateAll)
}

// checkFollowed requires the full-length thread on the rod of part to
// be as long as the rod, height mm, sound and solved, as one made on
// the rod at that height is.
func checkFollowed(t *testing.T, sk *sketch.Sketch, hg core.HGroup, height float64) {
	t.Helper()
	got, err := sk.ThreadOf(hg)
	if err != nil {
		t.Fatal(err)
	}
	if got.Length != height || !got.Through || !got.FullLength {
		t.Errorf("at %g: options %+v", height, got)
	}
	if msg := sk.Group(hg).Problem.Msg; msg != "" {
		t.Errorf("at %g: problem %q", height, msg)
	}
	for _, h := range sk.GroupOrder[3:] {
		if g := sk.Group(h); g.Solved.How != core.SolveOkay || g.Solved.Dof != 0 {
			t.Errorf("at %g: group %s solved %v with %d DOF", height, g.Name, g.Solved.How, g.Solved.Dof)
		}
	}
	mesh, bad := threadMesh(t, sk, hg)
	if bad != 0 {
		t.Errorf("at %g: %d naked edges", height, bad)
	}
	if lo, hi := meshBounds(mesh); math.Abs(hi.Z-height) > 1e-6 || math.Abs(lo.Z) > 1e-6 {
		t.Errorf("at %g: the solid reaches from z = %g to %g", height, lo.Z, hi.Z)
	}
	ref, _, refGroup := fullLengthThread(t, height, got)
	refMesh, _ := threadMesh(t, ref, refGroup)
	if v, w := mesh.CalculateVolume(), refMesh.CalculateVolume(); math.Abs(v-w) > 1e-3*w {
		t.Errorf("at %g: volume %g, made at that height %g", height, v, w)
	}
}

// threadMesh is the mesh of the whole thread that hg keeps, at its
// last group, and its count of naked edges.
func threadMesh(t *testing.T, sk *sketch.Sketch, hg core.HGroup) (*srf.SMesh, int) {
	t.Helper()
	hs, err := sk.ThreadGroups(hg)
	if err != nil {
		t.Fatal(err)
	}
	return meshStats(sk, hs[len(hs)-1])
}

// A full-length thread on a face follows the face when the part is
// made longer or shorter after, without being edited: its length is
// the face's, its solid sound and the same as a thread made at that
// length.
func TestThreadFollowsFace(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("thread solids")
	}
	sk, part, hg := fullLengthThread(t, 20, sketch.ThreadOptions{})
	dims := sk.Group(hg).Thread.LengthDims
	if len(dims) == 0 {
		t.Fatal("no length dimensions kept")
	}
	resize(sk, part, 25)
	checkFollowed(t, sk, hg, 25)
	resize(sk, part, 15)
	checkFollowed(t, sk, hg, 15)
	if got := sk.Group(hg).Thread.LengthDims; len(got) != len(dims) {
		t.Errorf("length dimensions %v, were %v", got, dims)
	}
}

// A thread with a run-out and a chamfer follows too: the run-out's
// cone and the chamfer at the far end move with the length.
func TestThreadFollowsFaceChamfered(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("thread solids")
	}
	sk, part, hg := fullLengthThread(t, 20, sketch.ThreadOptions{Chamfer: true})
	resize(sk, part, 24)
	checkFollowed(t, sk, hg, 24)
}

// A cosmetic thread's turning follows the face.
func TestThreadFollowsFaceCosmetic(t *testing.T) {
	t.Parallel()
	sk, part := rodPart(t, 5, 20)
	face, _ := roundFace(t, sk, part)
	p, err := sk.ThreadPlaceOnFace(face, false)
	if err != nil {
		t.Fatal(err)
	}
	o := threadCase{"", 5, 20, "M8", sketch.ThreadOptions{FullLength: true, Cosmetic: true}}.options(t)
	res, err := sk.AddThreadOnFace(p, o)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Sound {
		t.Fatal("not sound")
	}
	resize(sk, part, 30)
	got, _ := sk.ThreadOf(res.Group)
	if got.Length != 30 || !got.Cosmetic {
		t.Errorf("options %+v", got)
	}
	mesh, bad := threadMesh(t, sk, res.Group)
	if bad != 0 {
		t.Errorf("%d naked edges", bad)
	}
	// Turned down to M8's major diameter along the whole rod.
	if v, w := mesh.CalculateVolume(), math.Pi*16*30; math.Abs(v-w) > 1e-2*w {
		t.Errorf("volume %g, want about %g", v, w)
	}
}

// A face too short for the thread is reported on the thread's group,
// which keeps its length; made long enough again, the thread follows.
func TestThreadFollowsFaceRefused(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("thread solids")
	}
	sk, part, hg := fullLengthThread(t, 20, sketch.ThreadOptions{})
	resize(sk, part, 2)
	got, _ := sk.ThreadOf(hg)
	if got.Length != 20 {
		t.Errorf("followed to %g", got.Length)
	}
	if msg := sk.Group(hg).Problem.Msg; !strings.Contains(msg, "the face is now 2 mm long") || !strings.Contains(msg, "edit the thread") {
		t.Errorf("problem %q", msg)
	}
	resize(sk, part, 22)
	checkFollowed(t, sk, hg, 22)
}

// A thread whose length dimension was deleted by hand doesn't follow,
// and says so.
func TestThreadFollowsFaceDimensionGone(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("thread solids")
	}
	sk, part, hg := fullLengthThread(t, 20, sketch.ThreadOptions{})
	gone := sk.Group(hg).Thread.LengthDims[0]
	sk.Constraints = slices.DeleteFunc(sk.Constraints, func(c core.Constraint) bool { return c.H == gone })
	resize(sk, part, 25)
	if got, _ := sk.ThreadOf(hg); got.Length != 20 {
		t.Errorf("followed to %g", got.Length)
	}
	if msg := sk.Group(hg).Problem.Msg; !strings.Contains(msg, "a dimension of its length is gone") {
		t.Errorf("problem %q", msg)
	}
}

// Undoing the change to the part takes the thread back with it, and
// redo brings the followed thread again.
func TestThreadFollowsFaceUndo(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("thread solids")
	}
	sk, part, hg := fullLengthThread(t, 20, sketch.ThreadOptions{})
	// The solver settles a fraction of a nanometre apart from one solve
	// to the next, so the text of the file is not compared.
	state := func() (float64, float64) {
		sk.GenerateAll()
		got, err := sk.ThreadOf(hg)
		if err != nil {
			t.Fatal(err)
		}
		mesh, bad := threadMesh(t, sk, hg)
		if bad != 0 {
			t.Errorf("%d naked edges", bad)
		}
		_, hi := meshBounds(mesh)
		return got.Length, hi.Z
	}
	sk.UndoRemember()
	resize(sk, part, 25)
	if l, z := state(); l != 25 || math.Abs(z-25) > 1e-6 {
		t.Errorf("grown: length %g, top at %g", l, z)
	}
	sk.UndoUndo()
	if l, z := state(); l != 20 || math.Abs(z-20) > 1e-6 {
		t.Errorf("undone: length %g, top at %g", l, z)
	}
	sk.UndoRedo()
	if l, z := state(); l != 25 || math.Abs(z-25) > 1e-6 {
		t.Errorf("redone: length %g, top at %g", l, z)
	}
}

// The dimensions a thread follows its face by are saved with it: a
// thread read from a file follows the face as the one saved did.
func TestThreadFollowsFaceRoundTrip(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("thread solids")
	}
	sk, _, hg := fullLengthThread(t, 20, sketch.ThreadOptions{})
	path := filepath.Join(t.TempDir(), "rod.tlspr")
	if err := tlspr.SaveFile(sk, "", path, tlspr.Options{}); err != nil {
		t.Fatal(err)
	}
	doc, err := tlspr.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	back := doc.Sketch
	back.GenerateAll()
	if got, want := back.Group(hg).Thread.LengthDims, sk.Group(hg).Thread.LengthDims; len(got) != len(want) {
		t.Fatalf("read back %v length dimensions, saved %v", got, want)
	}
	part := back.GroupOrder[2]
	resize(back, part, 25)
	checkFollowed(t, back, hg, 25)
}
