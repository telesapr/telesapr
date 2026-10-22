package topo_test

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/srf"
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
		fr, err := tp.DatumFrame(f.ID)
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
		fr, err := tp.EdgeFrame(n)
		if err != nil {
			// The cone's seam, a line: straight.
			t.Errorf("edge %s: %v", n, err)
			continue
		}
		if p := fr.Origin; math.Abs(math.Abs(fr.N.Z)-1) < 1e-9 && math.Hypot(p.X, p.Y) > 1e-9 {
			t.Errorf("edge %s: a circle's centre %v off the axis", n, p)
		}
	}
	if _, err := tp.DatumFrame(0x7fff); err == nil || !strings.Contains(err.Error(), "no longer") {
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
		fr, err := tp.EdgeFrame(n)
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
