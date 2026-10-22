package topo_test

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A frustum's flat ends give their faces' frames, its cone the axis:
// the normal along z from the foot's centre up, the origin on the axis.
// Its circular edges give the axis too, at their centres.
func TestDatumFrameFrustum(t *testing.T) {
	t.Parallel()
	sh := frustum(10, 5, 8)
	// The foot, the top and the cone, each one face, as a lathe's
	// profile names them.
	for i := range sh.Surface {
		s := &sh.Surface[i]
		switch p := s.PointAt(0.5, 0.5); {
		case math.Abs(s.NormalAt(0.5, 0.5).WithMagnitude(1).Z) < 1-1e-9:
			s.Face = 3
		case p.Z < 4:
			s.Face = 1
		default:
			s.Face = 2
		}
	}
	tp := build(t, sh)
	seen := map[srf.FaceID]bool{}
	cones := 0
	for _, f := range tp.Faces {
		if seen[f.ID] {
			continue
		}
		seen[f.ID] = true
		fr, _, err := tp.DatumFrame(f.ID)
		if err != nil {
			t.Fatalf("face %x: %v", uint32(f.ID), err)
		}
		if math.Abs(fr.U.Dot(fr.N)) > 1e-12 || math.Abs(fr.U.Magnitude()-1) > 1e-12 {
			t.Errorf("face %x: u %v, n %v", uint32(f.ID), fr.U, fr.N)
		}
		if _, err := tp.FaceFrame(f.ID); err == nil {
			continue
		}
		cones++
		if math.Hypot(fr.Origin.X, fr.Origin.Y) > 1e-9 || math.Abs(fr.Origin.Z) > 1e-9 || math.Abs(fr.N.Z-1) > 1e-9 {
			t.Errorf("the cone's axis at %v along %v", fr.Origin, fr.N)
		}
	}
	if cones != 1 {
		t.Errorf("%d round faces", cones)
	}
	for e := range tp.Edges {
		n, err := tp.NameOf(e)
		if err != nil {
			continue
		}
		fr, _, err := tp.EdgeFrame(n)
		if err != nil {
			// The cone's seam, a line: straight.
			t.Errorf("edge %s: %v", n, err)
			continue
		}
		if p := fr.Origin; math.Abs(math.Abs(fr.N.Z)-1) < 1e-9 && math.Hypot(p.X, p.Y) > 1e-9 {
			t.Errorf("edge %s: a circle's centre %v off the axis", n, p)
		}
	}
	if _, _, err := tp.DatumFrame(0x7fff); err == nil || !strings.Contains(err.Error(), "no longer") {
		t.Errorf("a face that isn't there: %v", err)
	}
}

// A box's faces have no axis, and its edges run along their lines from
// their starts, u out of a face beside them.
func TestDatumFrameBox(t *testing.T) {
	t.Parallel()
	tp := build(t, box())
	if _, err := tp.AxisFrame(1); err == nil || !strings.Contains(err.Error(), "neither flat nor round") {
		t.Errorf("a box's face as an axis: %v", err)
	}
	for e := range tp.Edges {
		n, err := tp.NameOf(e)
		if err != nil {
			t.Fatal(err)
		}
		fr, _, err := tp.EdgeFrame(n)
		if err != nil {
			t.Fatalf("edge %s: %v", n, err)
		}
		ed := tp.Edges[e]
		a, b := tp.Vertices[ed.V[0]].P, tp.Vertices[ed.V[1]].P
		if !fr.Origin.Equals(a) || fr.N.Minus(b.Minus(a).WithMagnitude(1)).Magnitude() > 1e-12 || math.Abs(fr.U.Dot(fr.N)) > 1e-12 {
			t.Errorf("edge %s: %+v from %v to %v", n, fr, a, b)
		}
	}
}

// A lens prism's curved sides are neither flat nor round: each gives the
// plane touching it at its middle, on the face, the normal out of the
// solid, square to the first axis. Its cubic edges give the line along
// them at their middles; a sphere's faces give the plane touching them.
func TestDatumFrameFreeForm(t *testing.T) {
	t.Parallel()
	tp := build(t, eachFaceNamed(cubicPrism(lensCubics())))
	centre := v(10, 0, 2.5)
	seen := map[srf.FaceID]bool{}
	sides := 0
	for f, face := range tp.Faces {
		if seen[face.ID] {
			continue
		}
		seen[face.ID] = true
		fr, kind, err := tp.DatumFrame(face.ID)
		if err != nil {
			t.Fatalf("face %x: %v", uint32(face.ID), err)
		}
		if _, flat := tp.PlaneOf(f); flat {
			if kind != topo.DatumPlane {
				t.Errorf("a flat face is a %s", kind)
			}
			continue
		}
		sides++
		if kind != topo.DatumFacePoint {
			t.Errorf("a curved side is a %s", kind)
		}
		s := tp.Surface(f)
		var u, w float64
		s.ClosestPointToUV(fr.Origin, &u, &w, true)
		if d := s.PointAt(u, w).Minus(fr.Origin).Magnitude(); d > 1e-6 {
			t.Errorf("face %x: the origin %v is %g off the face", uint32(face.ID), fr.Origin, d)
		}
		if math.Abs(fr.Origin.Z-2.5) > 1e-6 || fr.N.Dot(fr.Origin.Minus(centre)) <= 0 {
			t.Errorf("face %x: at %v, normal %v not out of the lens", uint32(face.ID), fr.Origin, fr.N)
		}
		if math.Abs(fr.U.Dot(fr.N)) > 1e-12 || math.Abs(fr.N.Magnitude()-1) > 1e-12 {
			t.Errorf("face %x: u %v, n %v", uint32(face.ID), fr.U, fr.N)
		}
		// Found again the same way.
		if again, _, _ := tp.DatumFrame(face.ID); again != fr {
			t.Errorf("face %x: %+v, then %+v", uint32(face.ID), fr, again)
		}
	}
	if sides != 2 {
		t.Errorf("%d curved sides", sides)
	}
	curves := 0
	for e := range tp.Edges {
		n, err := tp.NameOf(e)
		if err != nil {
			continue
		}
		fr, kind, err := tp.EdgeFrame(n)
		if err != nil {
			t.Fatalf("edge %s: %v", n, err)
		}
		if kind != topo.DatumEdgePoint {
			continue
		}
		curves++
		if math.Abs(fr.Origin.Y) < 1 || math.Abs(fr.N.Z) > 1e-9 || math.Abs(math.Abs(fr.N.X)-1) > 1e-9 {
			t.Errorf("edge %s: at %v along %v, not at a cubic's middle", n, fr.Origin, fr.N)
		}
	}
	if curves != 4 {
		t.Errorf("%d cubic edges", curves)
	}

	ball := build(t, eachFaceNamed(sphereShell(5)))
	for _, face := range ball.Faces {
		fr, kind, err := ball.DatumFrame(face.ID)
		if err != nil {
			t.Fatalf("a sphere's face: %v", err)
		}
		if kind == topo.DatumFacePoint && (math.Abs(fr.Origin.Magnitude()-5) > 1e-6 || fr.N.Dot(fr.Origin) < 5-1e-6) {
			t.Errorf("a sphere's face at %v, normal %v", fr.Origin, fr.N)
		}
	}
}

// A datum's kind reads back from its word, and tells the mates that
// take it.
func TestDatumKind(t *testing.T) {
	t.Parallel()
	for _, k := range []topo.DatumKind{topo.DatumPlane, topo.DatumAxis, topo.DatumLine, topo.DatumFacePoint, topo.DatumEdgePoint} {
		if back, ok := topo.ParseDatumKind(k.String()); !ok || back != k {
			t.Errorf("%s reads back as %s", k, back)
		}
	}
	if _, ok := topo.ParseDatumKind("cone"); ok {
		t.Error("cone is a kind")
	}
	if topo.DatumPlane.IsAxis() || !topo.DatumPlane.IsPlane() || topo.DatumEdgePoint.IsPlane() || topo.DatumLine.IsPlane() ||
		!topo.DatumUnknown.IsAxis() || !topo.DatumUnknown.IsPlane() || !topo.DatumFacePoint.IsAxis() {
		t.Error("the mates a kind takes")
	}
}

// eachFaceNamed gives each of sh's surfaces an identity of its own, as
// the entities of a sketch would.
func eachFaceNamed(sh *srf.SShell) *srf.SShell {
	for i := range sh.Surface {
		sh.Surface[i].Face = srf.FaceID(i + 1)
	}
	return sh
}
