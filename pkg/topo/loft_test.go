package topo_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// square is the loop of a square of side s centred on (0, 0, z).
func square(s, z float64) []srf.SBezier {
	h := s / 2
	p := []geom.Vector{v(-h, -h, z), v(h, -h, z), v(h, h, z), v(-h, h, z)}
	var out []srf.SBezier
	for i := range p {
		out = append(out, srf.SBezierFrom2(p[i], p[(i+1)%4]))
	}
	return out
}

// circle is a circle of radius r about the z axis at height z, as four
// quarter arcs, starting at angle start.
func circle(r, z, start float64) []srf.SBezier {
	var out []srf.SBezier
	for k := range 4 {
		a0 := start + float64(k)*math.Pi/2
		a1 := a0 + math.Pi/2
		p0 := v(r*math.Cos(a0), r*math.Sin(a0), z)
		p1 := v(r*math.Cos(a1), r*math.Sin(a1), z)
		c := v(r*math.Sqrt2*math.Cos(a0+math.Pi/4), r*math.Sqrt2*math.Sin(a0+math.Pi/4), z)
		sb := srf.SBezierFrom3(p0, c, p1)
		sb.Weight[1] = math.Sqrt2 / 2
		out = append(out, sb)
	}
	return out
}

func loftVolume(t *testing.T, a, b []srf.SBezier) (float64, *topo.Topology) {
	t.Helper()
	sh, err := topo.MakeRuledLoft(a, b, core.NewRGBA(100, 100, 100, 255), 1<<30)
	if err != nil {
		t.Fatal(err)
	}
	var m srf.SMesh
	sh.TriangulateInto(&m)
	r := check.Solid(sh, &m)
	if !r.Sound() {
		t.Fatalf("unsound: %s", r.String())
	}
	tp, _ := topo.Build(sh)
	return r.Volume, tp
}

func TestLoftPrismAndFrustum(t *testing.T) {
	vol, tp := loftVolume(t, square(10, 0), square(10, 20))
	if math.Abs(vol-2000) > 1e-9 || len(tp.Faces) != 6 || tp.Euler() != 2 {
		t.Errorf("prism: volume %g, %d faces", vol, len(tp.Faces))
	}
	// A frustum: h/3 (A1 + A2 + sqrt(A1 A2)).
	vol, _ = loftVolume(t, square(10, 0), square(4, 6))
	if want := 6.0 / 3 * (100 + 16 + 40); math.Abs(vol-want) > 1e-9 {
		t.Errorf("frustum: volume %g, want %g", vol, want)
	}
	// Given either way round, and from the top down.
	vol, _ = loftVolume(t, square(4, 6), square(10, 0))
	if want := 6.0 / 3 * (100 + 16 + 40); math.Abs(vol-want) > 1e-9 {
		t.Errorf("frustum, the other way: volume %g", vol)
	}
}

func TestLoftCircles(t *testing.T) {
	old := srf.ChordTolMm()
	srf.SetChordTolMm(0.0005)
	defer srf.SetChordTolMm(old)
	vol, tp := loftVolume(t, circle(5, 0, 0), circle(2, 9, 0))
	want := math.Pi * 9 / 3 * (25 + 4 + 10)
	if math.Abs(vol-want) > 1e-3*want || len(tp.Faces) != 6 {
		t.Errorf("cone frustum: volume %g, want %g, %d faces", vol, want, len(tp.Faces))
	}
	// A square to a circle: between the frustum of the inscribed and of
	// the circumscribed polygons.
	vol, _ = loftVolume(t, square(10, 0), circle(5, 10, -3*math.Pi/4))
	if vol < 10*(100+math.Pi*25)/2*0.9 || vol > 1000 {
		t.Errorf("square to circle: volume %g", vol)
	}
}

func TestLoftRefusals(t *testing.T) {
	cases := map[string][2][]srf.SBezier{
		"counts":   {square(10, 0), circle(5, 10, 0)[:3]},
		"open":     {square(10, 0)[:3], square(10, 10)[:3]},
		"same":     {square(10, 0), square(10, 0)},
		"one only": {square(10, 0)[:1], square(10, 5)[:1]},
	}
	for name, c := range cases {
		if _, err := topo.MakeRuledLoft(c[0], c[1], core.RGBA{}, 0); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// A square to a smaller one turned by a quarter: each side face is a
// saddle, a patch of degree 1 across that twists along, which must mesh
// as one rather than as the two flat triangles across its corners;
// those would make the frustum. Its volume is h (A0 + A1) / 3, the mixed
// area of a loop and its quarter turn being zero.
func TestLoftTwistedSides(t *testing.T) {
	srf.SetChordTolMm(0.01)
	a, b := square(40, 0), square(20, 30)
	b = append(b[1:], b[:1]...)
	vol, tp := loftVolume(t, a, b)
	want := 30 * (1600 + 400) / 3.0
	if math.Abs(vol-want) > 0.005*want || len(tp.Faces) != 6 {
		t.Errorf("volume %g, want %g within the mesh's error; %d faces", vol, want, len(tp.Faces))
	}
}
