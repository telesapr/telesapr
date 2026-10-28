package topo_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
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
		{"four profiles", 4, func(int) []srf.SBezier { return circle(r, 0, 0) }, 0, 0},
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

// Closed lofts refuse two profiles, profiles more than a quarter turn
// apart and profiles on each other; a ring drawn turned a little more
// each time round closes all the same.
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
	// Three round the ring turn by a third of a turn each: the sections
	// between would pinch.
	if _, err := topo.MakeClosedLoft(ringProfiles(3, 20, func(int) []srf.SBezier { return circle(4, 0, 0) }), col, 9, srf.Tolerance{}); err == nil {
		t.Error("three profiles a third of a turn apart made a closed loft")
	}
	same := [][]srf.SBezier{circle(4, 0, 0), circle(4, 0, 0), circle(4, 1, 0)}
	if _, err := topo.MakeClosedLoft(same, col, 9, srf.Tolerance{}); err == nil {
		t.Error("profiles on each other made a closed loft")
	}
}

// TestClosedLoftSweep closes rings of circles, squares and blobs, four
// to twelve of them, of one size or swelling and narrowing by turns,
// free, held square to every third profile or guided over their tops,
// and
// lofts through circles stacked aside held square to the profiles
// between their ends, and requires sound solids; a torus of eight or
// more and a stack of one circle moved have the volumes Pappus and
// Cavalieri give. The rate
// allowed was fixed at none before either was written.
func TestClosedLoftSweep(t *testing.T) {
	if testing.Short() {
		t.Skip("a sweep of closed lofts and lofts held square to a middle profile")
	}
	t.Parallel()
	tol := srf.Tol(0.002, 100)
	col := core.NewRGBA(1, 1, 1, 255)
	grid := sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Strings("kind", "ring", "held ring", "guided ring", "middle"),
			sweep.Strings("profile", "circle", "square", "blob"),
			sweep.Floats("profiles", 4, 5, 8, 12),
			sweep.Floats("swell", 0, 0.4),
		},
		MaxFailRate: 0,
	}
	grid.Run(func(p sweep.Point) error {
		n := int(p.Float("profiles"))
		size := func(i int) float64 { return 4 * (1 + p.Float("swell")*float64(i%2)) }
		shape := func(i int, z float64) []srf.SBezier {
			switch p.String("profile") {
			case "square":
				return polygonAt(4, size(i), 0, 0, z, math.Pi/4)
			case "blob":
				return blobAt(size(i), 0.6*size(i), 0.5*size(i), 0, 0, z)
			}
			return circleAt(size(i), 0, 0, z)
		}
		var sh *srf.SShell
		var err error
		switch p.String("kind") {
		case "ring":
			sh, err = topo.MakeClosedLoft(ringProfiles(n, 25, func(i int) []srf.SBezier { return shape(i, 0) }), col, 9, tol)
		case "held ring":
			held := make([]float64, n)
			for j := 0; j < n; j += 3 {
				held[j] = 0.6 + 0.2*float64(j%2)
			}
			sh, err = topo.MakeClosedLoftHeld(ringProfiles(n, 25, func(i int) []srf.SBezier { return shape(i, 0) }), held, col, 9, tol)
		case "guided ring":
			// Circles only, and an even count round, for which the guide
			// over their tops, rising and falling with them, closes.
			if p.String("profile") != "circle" || n%2 == 1 {
				return nil
			}
			swell := p.Float("swell")
			guide := ringGuide(25, 4, func(float64) float64 { return math.Pi / 2 }, func(a float64) float64 {
				return 4*swell*(1-math.Cos(float64(n)*a/2))/2 + 1.2*math.Pow(math.Sin(float64(n)*a/2), 2)
			})
			sh, err = topo.MakeClosedGuidedLoft(ringProfiles(n, 25, func(i int) []srf.SBezier { return shape(i, 0) }), []topo.LoftGuide{guide}, col, 9, tol)
		default:
			var loops [][]srf.SBezier
			for i := range n {
				l := shape(i, 6*float64(i))
				for k := range l {
					for j := range l[k].Ctrl[:l[k].Degree+1] {
						l[k].Ctrl[j].X += 3 * math.Sin(float64(i))
					}
				}
				loops = append(loops, l)
			}
			middle := make([]float64, n-2)
			for j := range middle {
				middle[j] = 0.2 * float64(j%3)
			}
			sh, err = topo.MakeSmoothLoftThrough(loops, 0, topo.LoftEnds{}, topo.LoftSides{}, middle, col, 9, tol)
		}
		if err != nil {
			return err
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		r := check.Solid(sh, &m)
		if !r.Sound() {
			return fmt.Errorf("unsound: %s", r.String())
		}
		if p.Float("swell") != 0 || p.String("profile") != "circle" {
			return nil
		}
		want := math.Pi * 16 * 6 * float64(n-1)
		allow := 0.003
		if p.String("kind") != "middle" {
			// Sections between profiles far apart shrink as the profiles
			// turn: only a ring of many is close to a torus, and less
			// close where it is held square with one slope across, or
			// where its sections are moved up to a guide.
			if n < 8 {
				return nil
			}
			want, allow = 2*math.Pi*math.Pi*25*16, 0.01
			if p.String("kind") != "ring" {
				allow = 0.015
			}
		}
		if math.Abs(r.Volume-want) > allow*want {
			return fmt.Errorf("volume %g, want %g within %g of it", r.Volume, want, allow)
		}
		return nil
	}).Require(t)
}

// Closed lofts held square to profiles: a ring of eight circles held at
// one profile, at the seam's, at two and at every one is sound, one face
// of a piece from each held profile round to the next, the slope across
// at a held profile along the way round there and the same either side
// of it, and the volume near Pappus's. Held at every profile it is
// furthest off: one slope for the whole profile carries its inside as
// fast as its outside, which the torus's own doesn't.
func TestClosedLoftHeld(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.005, 100)
	col := core.NewRGBA(100, 100, 100, 255)
	const n, big, r = 8, 20.0, 4.0
	want := 2 * math.Pi * math.Pi * big * r * r
	for _, c := range []struct {
		name  string
		held  []float64
		allow float64
	}{
		{"one", []float64{0, 0, 1}, 0.01},
		{"the seam's", []float64{1}, 0.01},
		{"two", []float64{0, 0.8, 0, 0, 0, 1.2}, 0.03},
		{"every one", []float64{1, 1, 1, 1, 1, 1, 1, 1}, 0.05},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			loops := ringProfiles(n, big, func(int) []srf.SBezier { return circle(r, 0, 0) })
			bss, err := topo.ClosedLoftSurfaces(loops, c.held)
			if err != nil {
				t.Fatal(err)
			}
			var at []int
			for j, x := range c.held {
				if x != 0 {
					at = append(at, j)
				}
			}
			if len(bss) != len(at) {
				t.Fatalf("%d pieces, want %d", len(bss), len(at))
			}
			for p, bs := range bss {
				a := 2 * math.Pi * float64(at[p]) / n
				way := v(-math.Sin(a), math.Cos(a), 0)
				prev := bss[(p+len(bss)-1)%len(bss)]
				for _, u := range []float64{0.1, 0.4, 0.75} {
					_, s0 := partials(bs, u, 0, 0, 1)
					if d := s0.WithMagnitude(1).Minus(way).Magnitude(); d > 1e-4 {
						t.Errorf("piece %d at u %g leaves along %v, not %v", p, u, s0, way)
					}
					// The piece before reaches it the same way, scaled
					// by the ratio of their parameters' lengths.
					_, s1 := partials(prev, u, 1, 0, 1)
					if d := s1.WithMagnitude(1).Minus(way).Magnitude(); d > 1e-4 {
						t.Errorf("piece %d at u %g reaches along %v, not %v", p, u, s1, way)
					}
				}
			}
			sh, err := topo.MakeClosedLoftHeld(loops, c.held, col, srf.OriginFor(9, 0), tol)
			if err != nil {
				t.Fatal(err)
			}
			var m srf.SMesh
			sh.TriangulateInto(&m)
			rep := check.Solid(sh, &m)
			if !rep.Sound() {
				t.Fatalf("%s", rep.String())
			}
			if math.Abs(rep.Volume-want) > c.allow*want {
				t.Errorf("volume %.4f, want %.4f within %g of it", rep.Volume, want, c.allow)
			}
		})
	}
	loops := ringProfiles(n, big, func(int) []srf.SBezier { return circle(r, 0, 0) })
	if _, err := topo.ClosedLoftSurfaces(loops, make([]float64, n+1)); err == nil {
		t.Error("held at more profiles than the ring has")
	}
	if _, err := topo.ClosedLoftSurfaces(loops, []float64{math.NaN()}); err == nil {
		t.Error("held by NaN")
	}
}
