package topo_test

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Every face of a box has a frame: its normal out of the box, its
// origin on a corner, u along an edge, and the face beside the origin
// on the side of u and v.
func TestFaceFrameBox(t *testing.T) {
	t.Parallel()
	sh := box()
	tp := build(t, sh)
	centre := v(5, 10, 15)
	for id := srf.FaceID(1); id <= 6; id++ {
		fr, err := tp.FaceFrame(id)
		if err != nil {
			t.Fatalf("face %d: %v", id, err)
		}
		corner := false
		for _, vx := range tp.Vertices {
			corner = corner || vx.P.Equals(fr.Origin)
		}
		in := fr.Origin.Plus(fr.U.ScaledBy(0.5)).Plus(fr.V().ScaledBy(0.5))
		f, d := tp.FaceAt(in)
		switch {
		case !corner:
			t.Errorf("face %d: origin %v is no corner", id, fr.Origin)
		case fr.Origin.Minus(centre).Dot(fr.N) <= 0:
			t.Errorf("face %d: normal %v points into the box", id, fr.N)
		case math.Abs(fr.U.Magnitude()-1) > 1e-12 || math.Abs(fr.U.Dot(fr.N)) > 1e-12:
			t.Errorf("face %d: u %v", id, fr.U)
		case f < 0 || d > 1e-9 || tp.Faces[f].ID != id:
			t.Errorf("face %d: %v, half a unit along u and v, is not on it", id, in)
		}
		if q := fr.Quaternion(); !q.RotationN().Equals(fr.N) || !q.RotationU().Equals(fr.U) {
			t.Errorf("face %d: quaternion %v turns to %v, %v", id, q, q.RotationU(), q.RotationN())
		}
	}
}

// A frame is the same on a moved solid, moved: it comes from the
// face's edges by name, not from where they are.
func TestFaceFrameMoves(t *testing.T) {
	t.Parallel()
	sh := box()
	var moved srf.SShell
	moved.MakeFromTransformationOf(sh, v(3, -4, 7), geom.QuaternionIdentity, 1)
	a, b := build(t, sh), build(t, &moved)
	for id := srf.FaceID(1); id <= 6; id++ {
		fa, errA := a.FaceFrame(id)
		fb, errB := b.FaceFrame(id)
		if errA != nil || errB != nil || !fb.Origin.Equals(fa.Origin.Plus(v(3, -4, 7))) || !fb.U.Equals(fa.U) {
			t.Errorf("face %d: %+v (%v), moved %+v (%v)", id, fa, errA, fb, errB)
		}
	}
}

// A plate's top with a hole takes its frame from its outer boundary; a
// cylinder's cap from its circle's centre; its side and a face that
// isn't there have none.
func TestFaceFrameHoleCapAndRefusals(t *testing.T) {
	t.Parallel()
	plate := plateWithHole(5, 10)
	tp := build(t, plate)
	top, d := tp.FaceAt(v(15, 15, 10))
	if top < 0 || d > 1e-9 {
		t.Fatal("no top face")
	}
	fr, err := tp.FaceFrame(tp.Faces[top].ID)
	if err != nil || math.Abs(math.Abs(fr.Origin.X)-20) > 1e-9 || math.Abs(math.Abs(fr.Origin.Y)-20) > 1e-9 ||
		!fr.N.Equals(v(0, 0, 1)) {
		t.Fatalf("plate's top: %+v, %v", fr, err)
	}

	cyl := build(t, named(cylinder(5, 10)))
	fr, err = cyl.FaceFrame(1)
	if err != nil || !fr.Origin.Equals(v(0, 0, 10)) || !fr.N.Equals(v(0, 0, 1)) || math.Abs(fr.U.Magnitude()-1) > 1e-12 {
		t.Fatalf("cylinder's cap: %+v, %v", fr, err)
	}
	if _, err := cyl.FaceFrame(7); err == nil || !strings.Contains(err.Error(), "not flat") {
		t.Errorf("cylinder's side: %v", err)
	}
	if _, err := cyl.FaceFrame(99); err == nil || !strings.Contains(err.Error(), "no longer in the solid") {
		t.Errorf("a face that isn't there: %v", err)
	}
}

// An extruded oval of four cubics has caps with no straight and no
// circular edge: the frame runs from a vertex along the chord of its
// least-named edge.
func TestFaceFrameOval(t *testing.T) {
	t.Parallel()
	const a, b, k = 8.0, 5.0, 0.5523
	p := []geom.Vector{v(a, 0, 0), v(0, b, 0), v(-a, 0, 0), v(0, -b, 0)}
	var ls srf.SBezierLoopSet
	for i := range 4 {
		p0, p1 := p[i], p[(i+1)%4]
		// The tangents at the ends run along the axes.
		t0 := v(-p0.Y*a/b, p0.X*b/a, 0).ScaledBy(k)
		t1 := v(p1.Y*a/b, -p1.X*b/a, 0).ScaledBy(k)
		ls.Outer.Curves = append(ls.Outer.Curves, srf.SBezierFrom4(p0, p0.Plus(t0), p1.Plus(t1), p1))
	}
	ls.Normal = v(0, 0, -1)
	var sh srf.SShell
	sh.MakeFromExtrusionOf(&ls, v(0, 0, 0), v(0, 0, 3), core.NewRGBA(100, 100, 100, 255))
	tp := build(t, named(&sh))
	fr, err := tp.FaceFrame(1)
	if err != nil {
		t.Fatal(err)
	}
	corner := false
	for _, q := range p {
		corner = corner || fr.Origin.Equals(q.Plus(v(0, 0, 3)))
	}
	if !corner || !fr.N.Equals(v(0, 0, 1)) || math.Abs(fr.U.Magnitude()-1) > 1e-12 || math.Abs(fr.U.Z) > 1e-12 {
		t.Fatalf("oval's cap: %+v", fr)
	}
}
