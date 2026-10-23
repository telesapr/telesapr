package slvs

import (
	"errors"
	"math"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/golden"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// rodPart is a rod of the radius, extruded height up from a circle at
// the origin, with nothing on it: the sketch and the extrusion.
func rodPart(t testing.TB, radius, height float64) (*sketch.Sketch, core.HGroup) {
	t.Helper()
	sk := sketch.NewSketch()
	sk.NewFile()
	sk.GenerateAll()
	g1 := sk.ActiveGroup()
	sk.Active = g1
	sk.Group(g1).ActiveWorkplane = g1.Entity(0)
	circle := sk.AddRequestOfType(sketch.RequestCircle, false)
	sk.MustEntity(circle.Entity(1)).PointForceTo(sk, geom.Vector{})
	sk.MustEntity(circle.Entity(64)).DistanceForceTo(sk, radius)
	sk.GenerateAllTyped(sketch.GenerateAll)
	ext := sketch.Group{Type: sketch.GroupExtrude, Subtype: sketch.GroupOneSided, Name: "extrude",
		OpA: g1, Visible: true, Scale: 1, Order: len(sk.Groups), Color: core.NewRGBA(100, 100, 100, 255)}
	ext.Predef.EntityB = g1.Entity(0)
	part := sk.AddGroup(ext)
	sk.Active = part
	sk.GenerateAllTyped(sketch.GenerateAll)
	sk.MustParam(part.Param(0)).Val = 0
	sk.MustParam(part.Param(1)).Val = 0
	sk.MustParam(part.Param(2)).Val = height / 2
	sk.GenerateAllTyped(sketch.GenerateAll)
	return sk, part
}

// roundFace is the identity of the one cylindrical face of the solid
// at group hg, and of a flat face.
func roundFace(t testing.TB, sk *sketch.Sketch, hg core.HGroup) (round, flat srf.FaceID) {
	t.Helper()
	tp, err := sk.Group(hg).Topology()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range tp.Faces {
		if _, err := tp.AxisEnds(f.ID); err == nil {
			round, found = f.ID, true
		} else {
			flat = f.ID
		}
	}
	if !found {
		t.Fatal("no cylindrical face")
	}
	return round, flat
}

// A thread made on a rod's face is the thread made on a circle at the
// rod's upper end: its own datum and circle there, the same volume,
// sound, at 0 DOF. It follows the face when the rod is made longer.
func TestThreadOnFace(t *testing.T) {
	sk, part := rodPart(t, 4, 30)
	face, _ := roundFace(t, sk, part)
	p, err := sk.ThreadPlaceOnFace(face, false)
	if err != nil {
		t.Fatal(err)
	}
	if p.At.Hole || math.Abs(p.At.Frame.Origin.Z-30) > 1e-9 || p.At.Frame.N.Z > -1+1e-9 || !p.Far.Free {
		t.Fatalf("the thread starts at %v along %v, hole %v", p.At.Frame.Origin, p.At.Frame.N, p.At.Hole)
	}
	if o, err := sk.ThreadPlaceOnFace(face, true); err != nil || math.Abs(o.At.Frame.Origin.Z) > 1e-9 {
		t.Errorf("the other end: %v, %v", o.At.Frame.Origin, err)
	}
	tc := threadCase{"", 4, 30, "M8", sketch.ThreadOptions{Length: 12}}
	o := tc.options(t)
	res, err := sk.AddThreadOnFace(p, o)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Sound {
		t.Fatal("the thread on the face is not sound")
	}
	got, _ := sk.ThreadOf(res.Group)
	if got.Internal || got.Reverse || got.Length != 12 {
		t.Errorf("options %+v", got)
	}
	for _, hg := range sk.GroupOrder[3:] {
		if g := sk.Group(hg); g.Solved.How != core.SolveOkay || g.Solved.Dof != 0 {
			t.Errorf("group %s: solved %v with %d DOF", g.Name, g.Solved.How, g.Solved.Dof)
		}
	}
	mesh, bad := meshStats(sk, res.Group)
	if bad != 0 {
		t.Errorf("%d naked edges", bad)
	}
	ref, _, refRes := tc.build(t, 0)
	refMesh, _ := meshStats(ref, refRes.Group)
	if v, w := mesh.CalculateVolume(), refMesh.CalculateVolume(); math.Abs(v-w) > 1e-3*w {
		t.Errorf("volume %g, on a circle %g", v, w)
	}
	if lo, _ := meshBounds(mesh); lo.Z > 17.5 {
		t.Errorf("the thread ends at z = %g, above 18", lo.Z)
	}

	// The rod made 40 long carries the thread up with its end.
	sk.MustParam(part.Param(2)).Val = 20
	sk.MarkGroupDirty(part, false)
	sk.GenerateAllTyped(sketch.GenerateAll)
	mesh, bad = meshStats(sk, res.Group)
	if bad != 0 {
		t.Errorf("after the rod grew: %d naked edges", bad)
	}
	if _, hi := meshBounds(mesh); math.Abs(hi.Z-40) > 1e-6 {
		t.Errorf("after the rod grew, the solid reaches z = %g", hi.Z)
	}
	if v, w := mesh.CalculateVolume(), refMesh.CalculateVolume()+math.Pi*16*10; math.Abs(v-w) > 1e-2*w {
		t.Errorf("after the rod grew: volume %g, want about %g", v, w)
	}
}

// A full-length thread on a face runs its whole length, through where
// the far end is the part's, and is measured again when it is edited
// after the face changed.
func TestThreadOnFaceFullLength(t *testing.T) {
	sk, part := rodPart(t, 4, 20)
	face, _ := roundFace(t, sk, part)
	p, err := sk.ThreadPlaceOnFace(face, false)
	if err != nil {
		t.Fatal(err)
	}
	o := threadCase{"", 4, 20, "M8", sketch.ThreadOptions{FullLength: true, Length: 5}}.options(t)
	res, err := sk.AddThreadOnFace(p, o)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := sk.ThreadOf(res.Group)
	if got.Length != 20 || !got.Through || !got.FullLength {
		t.Errorf("options %+v", got)
	}
	if !res.Sound {
		t.Error("not sound")
	}
	sk.MustParam(part.Param(2)).Val = 12.5
	sk.MarkGroupDirty(part, false)
	sk.GenerateAllTyped(sketch.GenerateAll)
	if _, err := sk.EditThread(res.Group, got); err != nil {
		t.Fatal(err)
	}
	if got, _ := sk.ThreadOf(res.Group); got.Length != 25 {
		t.Errorf("edited after the rod grew to 25: length %g", got.Length)
	}
}

// A thread is refused on a flat face, and adds nothing then.
func TestThreadOnFaceRefused(t *testing.T) {
	sk, part := rodPart(t, 4, 20)
	_, flat := roundFace(t, sk, part)
	groups := len(sk.Groups)
	if _, err := sk.ThreadPlaceOnFace(flat, false); !errors.Is(err, sketch.ErrNotCylinder) {
		t.Errorf("a flat face: %v", err)
	}
	face, _ := roundFace(t, sk, part)
	p, err := sk.ThreadPlaceOnFace(face, false)
	if err != nil {
		t.Fatal(err)
	}
	o := threadCase{"", 4, 20, "M8", sketch.ThreadOptions{Length: 1}}.options(t)
	if _, err := sk.AddThreadOnFace(p, o); err == nil {
		t.Error("a thread too short to turn was made")
	}
	if len(sk.Groups) != groups {
		t.Error("groups were added")
	}
}

// meshBounds is the box a mesh lies in.
func meshBounds(m *srf.SMesh) (lo, hi geom.Vector) {
	lo = geom.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	hi = lo.ScaledBy(-1)
	for _, tr := range m.Triangles {
		for _, p := range []geom.Vector{tr.A, tr.B, tr.C} {
			lo = geom.Vector{X: min(lo.X, p.X), Y: min(lo.Y, p.Y), Z: min(lo.Z, p.Z)}
			hi = geom.Vector{X: max(hi.X, p.X), Y: max(hi.Y, p.Y), Z: max(hi.Z, p.Z)}
		}
	}
	return lo, hi
}

// TestThreadOnFaceGolden saves a thread made on a rod's face from its
// lower end, opens it and saves it again, and requires the golden,
// testdata/thread/face_m8.tlspr: the datum's two faces and the thread's
// full-length. TELESAPR_REGEN_GOLDEN=1 rewrites it.
func TestThreadOnFaceGolden(t *testing.T) {
	if testing.Short() {
		t.Skip("thread solids")
	}
	sk, part := rodPart(t, 4, 30)
	face, _ := roundFace(t, sk, part)
	p, err := sk.ThreadPlaceOnFace(face, true)
	if err != nil {
		t.Fatal(err)
	}
	o := threadCase{"", 4, 30, "M8", sketch.ThreadOptions{FullLength: true}}.options(t)
	if _, err := sk.AddThreadOnFace(p, o); err != nil {
		t.Fatal(err)
	}
	sk.GenerateAll()
	path := filepath.Join(t.TempDir(), "face_m8.tlspr")
	if err := tlspr.SaveFile(sk, "", path, tlspr.Options{}); err != nil {
		t.Fatal(err)
	}
	doc, err := tlspr.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Sketch.GenerateAll()
	doc.Sketch.GenerateAll()
	golden.Check(t, fixture("thread/face_m8.tlspr"), tlspr.Encode(doc.Sketch, tlspr.Options{}))
}
