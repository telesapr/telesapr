package topo_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// steppedShaft is a disc of radius 10, 5 high, with a shaft of radius 4
// on it up to z = 20. Its faces are named by where they lie: 1 the
// disc's foot, 2 its rim, 3 the shoulder, 4 the shaft, 5 the shaft's
// cap.
func steppedShaft() *srf.SShell {
	sh := lathed([]geom.Vector{v(0, 0, 0), v(10, 0, 0), v(10, 0, 5), v(4, 0, 5), v(4, 0, 20), v(0, 0, 20)}, srf.Tolerance{})
	for i := range sh.Surface {
		s := &sh.Surface[i]
		p, n := s.PointAt(0.5, 0.5), s.NormalAt(0.5, 0.5).WithMagnitude(1)
		r := math.Hypot(p.X, p.Y)
		switch {
		case math.Abs(n.Z) > 1-1e-9 && p.Z < 1:
			s.Face = 1
		case math.Abs(n.Z) > 1-1e-9 && p.Z > 19:
			s.Face = 5
		case math.Abs(n.Z) > 1-1e-9:
			s.Face = 3
		case r > 9:
			s.Face = 2
		default:
			s.Face = 4
		}
	}
	return sh
}

// A shaft's ends: its cap is free, the shoulder is not, both at the
// shaft's radius and length, and the frame at each points into the
// shaft.
func TestAxisEndsShaft(t *testing.T) {
	t.Parallel()
	tp := build(t, steppedShaft())
	ends, err := tp.AxisEnds(4)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ends {
		if math.Abs(e.Radius-4) > 1e-9 || math.Abs(e.Length-15) > 1e-9 || e.Hole {
			t.Errorf("end at %x: radius %g length %g hole %v", uint32(e.End), e.Radius, e.Length, e.Hole)
		}
		if math.Hypot(e.Frame.Origin.X, e.Frame.Origin.Y) > 1e-9 {
			t.Errorf("end at %x: origin %v off the axis", uint32(e.End), e.Frame.Origin)
		}
		switch e.End {
		case 5:
			if !e.Free || math.Abs(e.Frame.Origin.Z-20) > 1e-9 || e.Frame.N.Z > -1+1e-9 {
				t.Errorf("cap end: free %v, at %v along %v", e.Free, e.Frame.Origin, e.Frame.N)
			}
		case 3:
			if e.Free || math.Abs(e.Frame.Origin.Z-5) > 1e-9 || e.Frame.N.Z < 1-1e-9 {
				t.Errorf("shoulder end: free %v, at %v along %v", e.Free, e.Frame.Origin, e.Frame.N)
			}
		default:
			t.Errorf("an end at face %x", uint32(e.End))
		}
	}
	fr, err := tp.AxisEndFrame(4, 5)
	if err != nil || math.Abs(fr.Origin.Z-20) > 1e-9 {
		t.Errorf("frame at the cap: %v, %v", fr, err)
	}
	if _, err := tp.AxisEndFrame(4, 1); err == nil {
		t.Error("the shaft ends at the disc's foot")
	}

	// The circle where the shaft meets its cap names that end.
	found := false
	for e := range tp.Edges {
		id, end, err := tp.AxisEndAt(e)
		if err != nil {
			continue
		}
		if id == 4 && end.End == 5 {
			found = true
		}
		if id != 4 && id != 2 {
			t.Errorf("edge %d: an end of face %x", e, uint32(id))
		}
	}
	if !found {
		t.Error("no edge names the shaft's cap end")
	}
}

// Flat faces and cones have no ends a thread starts at.
func TestAxisEndsRefused(t *testing.T) {
	t.Parallel()
	tp := build(t, steppedShaft())
	if _, err := tp.AxisEnds(3); err == nil {
		t.Error("the shoulder has ends")
	}
	sh := frustum(10, 5, 8, srf.Tolerance{})
	for i := range sh.Surface {
		sh.Surface[i].Face = 1
		if s := &sh.Surface[i]; math.Abs(s.NormalAt(0.5, 0.5).WithMagnitude(1).Z) < 1-1e-9 {
			s.Face = 2
		}
	}
	if _, err := build(t, sh).AxisEnds(2); err == nil {
		t.Error("a cone has ends")
	}
}

// A hole through a plate is a hole, free at both ends.
func TestAxisEndsHole(t *testing.T) {
	t.Parallel()
	sh := countersunk(4, 4, srf.Tolerance{})
	// The plate's faces are named one by one, the hole's quarters as one.
	const hole = srf.FaceID(100)
	for i := range sh.Surface {
		s := &sh.Surface[i]
		s.Face = srf.FaceID(i + 1)
		if p := s.PointAt(0.5, 0.5); math.Abs(math.Hypot(p.X, p.Y)-4) < 1e-6 {
			s.Face = hole
		}
	}
	tp := build(t, sh)
	ends, err := tp.AxisEnds(hole)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ends {
		if !e.Hole || !e.Free || math.Abs(e.Radius-4) > 1e-6 || math.Abs(e.Length-10) > 1e-6 {
			t.Errorf("end %+v", e)
		}
	}
	if ends[0].Frame.N.Dot(ends[1].Frame.N) > -1+1e-9 {
		t.Errorf("both ends point the same way: %v, %v", ends[0].Frame.N, ends[1].Frame.N)
	}
}
