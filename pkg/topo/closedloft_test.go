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

// ringProfiles are n profiles round the z axis at radius big, each the
// loop of in drawn in the plane through the axis at its angle, x away
// from the axis and y up, its size scaled by size(i).
func ringProfiles(n int, big float64, in func(i int) []srf.SBezier) [][]srf.SBezier {
	var out [][]srf.SBezier
	for i := range n {
		a := 2 * math.Pi * float64(i) / float64(n)
		er := v(math.Cos(a), math.Sin(a), 0)
		to := func(p geom.Vector) geom.Vector {
			return er.ScaledBy(big + p.X).Plus(v(0, 0, p.Y))
		}
		var l []srf.SBezier
		for _, sb := range in(i) {
			for k := 0; k <= sb.Degree; k++ {
				sb.Ctrl[k] = to(sb.Ctrl[k])
			}
			l = append(l, sb)
		}
		out = append(out, l)
	}
	return out
}

// Closed lofts round the z axis: a ring of circles is a torus, of the
// volume Pappus gives, as closely as the profiles' spacing lets the
// cubic across follow the circle round; squares make a square ring.
// No caps, sound, and the surface's slope across the same either side
// of the seam.
func TestClosedLoft(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.005, 100)
	col := core.NewRGBA(100, 100, 100, 255)
	const big, r = 20.0, 4.0
	for _, c := range []struct {
		name  string
		n     int
		loop  func(i int) []srf.SBezier
		want  float64
		allow float64
	}{
		{"torus of 8", 8, func(int) []srf.SBezier { return circle(r, 0, 0) }, 2 * math.Pi * math.Pi * big * r * r, 0.01},
		{"torus of 16", 16, func(int) []srf.SBezier { return circle(r, 0, 0) }, 2 * math.Pi * math.Pi * big * r * r, 0.002},
		{"square ring", 12, func(int) []srf.SBezier { return polygonAt(4, r, 0, 0, 0, math.Pi/4) }, 2 * r * r * 2 * math.Pi * big, 0.005},
		{"three profiles", 3, func(int) []srf.SBezier { return circle(r, 0, 0) }, 0, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			loops := ringProfiles(c.n, big, c.loop)
			sh, err := topo.MakeClosedLoft(loops, col, srf.OriginFor(9, 0), tol)
			if err != nil {
				t.Fatal(err)
			}
			for i := range sh.Surface {
				if s := &sh.Surface[i]; s.DegM == 1 && s.DegN == 1 {
					t.Fatalf("a closed loft has a flat face: a cap")
				}
			}
			var m srf.SMesh
			sh.TriangulateInto(&m)
			rep := check.Solid(sh, &m)
			if !rep.Sound() {
				t.Fatalf("%s", rep.String())
			}
			if c.want != 0 && math.Abs(rep.Volume-c.want) > c.allow*c.want {
				t.Errorf("volume %.4f, want %.4f within %g of it", rep.Volume, c.want, c.allow)
			}
			bs, err := topo.ClosedLoftSurface(loops)
			if err != nil {
				t.Fatal(err)
			}
			for _, u := range []float64{0.1, 0.4, 0.75} {
				_, s0 := partials(bs, u, 0, 0, 1)
				_, s1 := partials(bs, u, 1, 0, 1)
				if d := s0.Minus(s1).Magnitude(); d > 1e-4*s0.Magnitude() {
					t.Errorf("at u %g the slope across is %v at the start, %v at the end", u, s0, s1)
				}
			}
		})
	}
}

// Closed lofts refuse two profiles and profiles on each other; a ring
// drawn turned a little more each time round closes all the same.
func TestClosedLoftRefusals(t *testing.T) {
	t.Parallel()
	col := core.NewRGBA(100, 100, 100, 255)
	ring := ringProfiles(4, 20, func(int) []srf.SBezier { return circle(4, 0, 0) })
	if _, err := topo.MakeClosedLoft(ring[:2], col, 9, srf.Tolerance{}); err == nil {
		t.Error("two profiles made a closed loft")
	}
	// Squares turned a fifth of a quarter more each time round: the
	// closing span takes the turn back, and the ring is still sound.
	twisted := ringProfiles(6, 20, func(i int) []srf.SBezier { return polygonAt(4, 4, 0, 0, 0, float64(i)*0.1*math.Pi) })
	sh, err := topo.MakeClosedLoft(twisted, col, 9, srf.Tol(0.005, 100))
	if err != nil {
		t.Fatalf("a twisted ring: %v", err)
	}
	var m srf.SMesh
	sh.TriangulateInto(&m)
	if rep := check.Solid(sh, &m); !rep.Sound() {
		t.Errorf("a twisted ring: %s", rep.String())
	}
	same := [][]srf.SBezier{circle(4, 0, 0), circle(4, 0, 0), circle(4, 1, 0)}
	if _, err := topo.MakeClosedLoft(same, col, 9, srf.Tolerance{}); err == nil {
		t.Error("profiles on each other made a closed loft")
	}
}
