package topo_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A point picked on a lens's curved side or cubic edge is kept as a
// place on it: the frame at that place is at the point, and with no
// place at the middle, as before. A flat face and a place on it give the
// face's own frame.
func TestDatumFrameAtPlace(t *testing.T) {
	t.Parallel()
	tp := build(t, eachFaceNamed(cubicPrism(lensCubics(), srf.Tolerance{})))
	sides, curves := 0, 0
	for f, face := range tp.Faces {
		if _, flat := tp.PlaneOf(f); flat {
			place, err := tp.FacePlace(face.ID, tp.Vertices[0].P)
			if err != nil {
				t.Fatal(err)
			}
			at, kind, err := tp.DatumFrameAt(face.ID, place)
			if mid, _, _ := tp.DatumFrame(face.ID); err != nil || kind != topo.DatumPlane || at != mid {
				t.Errorf("a flat face picked anywhere: %+v %s %v, not %+v", at, kind, err, mid)
			}
			continue
		}
		sides++
		s := tp.Surface(f)
		for _, uv := range [][2]float64{{0.2, 0.3}, {0.8, 0.7}} {
			p := s.PointAt(uv[0], uv[1])
			place, err := tp.FacePlace(face.ID, p)
			if err != nil {
				t.Fatal(err)
			}
			fr, kind, err := tp.DatumFrameAt(face.ID, place)
			if err != nil || kind != topo.DatumFacePoint {
				t.Fatalf("face %x: %s %v", uint32(face.ID), kind, err)
			}
			if d := fr.Origin.Minus(p).Magnitude(); d > 1e-6 {
				t.Errorf("face %x picked at %v: the frame at %v", uint32(face.ID), p, fr.Origin)
			}
			if n := s.NormalAt(uv[0], uv[1]).WithMagnitude(1); math.Abs(math.Abs(n.Dot(fr.N))-1) > 1e-9 {
				t.Errorf("face %x: normal %v, the face's %v", uint32(face.ID), fr.N, n)
			}
		}
	}
	for e, ed := range tp.Edges {
		n, err := tp.NameOf(e)
		if err != nil {
			continue
		}
		if _, kind, _ := tp.EdgeFrame(n); kind != topo.DatumEdgePoint {
			continue
		}
		curves++
		p := ed.Points[len(ed.Points)/4]
		place, err := tp.EdgePlace(n, p)
		if err != nil {
			t.Fatal(err)
		}
		if place[0] <= 0 || place[0] >= 0.5 {
			t.Errorf("edge %s: a quarter of its points along is at %g of its length", n, place[0])
		}
		fr, _, err := tp.EdgeFrameAt(n, place)
		if err != nil {
			t.Fatal(err)
		}
		if d := fr.Origin.Minus(p).Magnitude(); d > 1e-3 {
			t.Errorf("edge %s picked at %v: the frame at %v", n, p, fr.Origin)
		}
		if mid, _, _ := tp.EdgeFrameAt(n, nil); mid.Origin.Minus(fr.Origin).Magnitude() < 1 {
			t.Errorf("edge %s: the frame at the place is the middle's, %v", n, mid.Origin)
		}
	}
	if sides != 2 || curves != 4 {
		t.Errorf("%d curved sides, %d cubic edges", sides, curves)
	}
	if _, err := tp.FacePlace(0x7fff, geom.Vector{}); err == nil {
		t.Error("a place on a face not in the solid")
	}
	if _, err := tp.EdgePlace(topo.Name{A: 0x7fff, B: 0x7ffe}, geom.Vector{}); err == nil {
		t.Error("a place on an edge not in the solid")
	}
}

// A place on a cubic edge whose frame was last found at the place's
// mirror along the edge, as after an edit that turned the edge round,
// is counted from the other end: the frame comes back where it was last
// found and the place is returned as the edge now counts it. Found
// where it was, the place stays. A straight edge's frame takes no place.
func TestEdgeFrameNear(t *testing.T) {
	t.Parallel()
	tp := build(t, eachFaceNamed(cubicPrism(lensCubics(), srf.Tolerance{})))
	checked := 0
	for e := range tp.Edges {
		n, err := tp.NameOf(e)
		if err != nil {
			continue
		}
		if _, kind, _ := tp.EdgeFrame(n); kind != topo.DatumEdgePoint {
			if fr, _, place, err := tp.EdgeFrameNear(n, []float64{0.2}, topo.FaceFrame{}); err != nil || len(place) != 1 || place[0] != 0.2 {
				t.Errorf("edge %s: %+v %v %v", n, fr, place, err)
			}
			continue
		}
		checked++
		here, _, _ := tp.EdgeFrameAt(n, []float64{0.2})
		there, _, _ := tp.EdgeFrameAt(n, []float64{0.8})
		fr, _, place, err := tp.EdgeFrameNear(n, []float64{0.2}, there)
		if err != nil || place[0] != 0.8 || fr.Origin != there.Origin {
			t.Errorf("edge %s turned round: %v at %v, not %v", n, place, fr.Origin, there.Origin)
		}
		if _, _, place, _ := tp.EdgeFrameNear(n, []float64{0.2}, here); place[0] != 0.2 {
			t.Errorf("edge %s where it was: %v", n, place)
		}
	}
	if checked != 4 {
		t.Errorf("%d cubic edges", checked)
	}
	if _, _, _, err := tp.EdgeFrameNear(topo.Name{A: 0x7fff, B: 0x7ffe}, []float64{0.2}, topo.FaceFrame{}); err == nil {
		t.Error("a frame on an edge not in the solid")
	}
}
