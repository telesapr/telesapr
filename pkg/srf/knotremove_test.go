package srf

import (
	"math"
	"math/rand"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// refined is s with every inner knot inserted until it is there deg
// times along each way, the surface its Bezier patches joined.
func refined(s BSplineSurface) BSplineSurface {
	net := make([][]geom.Vector4, len(s.Ctrl))
	for i := range s.Ctrl {
		for j := range s.Ctrl[i] {
			net[i] = append(net[i], homogeneous(s.Ctrl[i][j], s.Weight[i][j]))
		}
	}
	full := func(deg int, knots []float64, net [][]geom.Vector4) ([]float64, [][]geom.Vector4) {
		for _, b := range breaks(deg, knots) {
			for {
				count := 0
				for _, k := range knots {
					if k == b {
						count++
					}
				}
				if count >= deg || b == knots[0] || b == knots[len(knots)-1] {
					break
				}
				var nk []float64
				out := make([][]geom.Vector4, len(net[0]))
				for j := range net[0] {
					col := make([]geom.Vector4, len(net))
					for i := range net {
						col[i] = net[i][j]
					}
					nk, out[j] = insertKnot(deg, knots, col, b)
				}
				knots, net = nk, transpose4(out)
			}
		}
		return knots, net
	}
	out := s
	ku, net := full(s.DegU, append([]float64(nil), s.KnotsU...), net)
	kv, tr := full(s.DegV, append([]float64(nil), s.KnotsV...), transpose4(net))
	net = transpose4(tr)
	out.KnotsU, out.KnotsV = ku, kv
	out.Ctrl, out.Weight = nil, nil
	for i := range net {
		var row []geom.Vector
		var w []float64
		for _, q := range net[i] {
			row = append(row, q.PerspectiveProject())
			w = append(w, q.W)
		}
		out.Ctrl = append(out.Ctrl, row)
		out.Weight = append(out.Weight, w)
	}
	return out
}

func distinctInner(deg int, knots []float64) int {
	return len(breaks(deg, knots)) - 2
}

// TestRemoveKnotsGivesBack refines random rational B-spline surfaces
// to Bezier patches joined, a knot of full multiplicity between each
// two, and takes the knots out again: the surface's own knots come
// back, and its points stay where they were.
func TestRemoveKnotsGivesBack(t *testing.T) {
	r := rand.New(rand.NewSource(27))
	for trial := range 40 {
		du, dv := 1+r.Intn(4), 1+r.Intn(4)
		nu, nv := du+1+r.Intn(4), dv+1+r.Intn(4)
		s := randomSurface(r, du, dv, nu, nv, true)
		f := refined(s)
		if len(f.KnotsU) < len(s.KnotsU) || len(f.KnotsV) < len(s.KnotsV) {
			t.Fatalf("refining lost knots")
		}
		g := f
		n := g.RemoveKnots(1e-7)
		if err := g.Check(); err != nil {
			t.Fatal(err)
		}
		want := len(f.KnotsU) - len(s.KnotsU) + len(f.KnotsV) - len(s.KnotsV)
		if n != want || len(g.KnotsU) != len(s.KnotsU) || len(g.KnotsV) != len(s.KnotsV) {
			t.Errorf("trial %d: degree %d,%d: took %d knots out of %d, %d; want %d, back to %d, %d",
				trial, du, dv, n, len(f.KnotsU), len(f.KnotsV), want, len(s.KnotsU), len(s.KnotsV))
		}
		u0, u1, v0, v1 := s.Domain()
		for range 20 {
			u, v := u0+(u1-u0)*r.Float64(), v0+(v1-v0)*r.Float64()
			if d := g.PointAt(u, v).Minus(s.PointAt(u, v)).Magnitude(); d > 1e-6 {
				t.Errorf("trial %d: the surface moved %g at (%g, %g)", trial, d, u, v)
			}
		}
	}
}

// TestRemoveKnotsKeepsCreases moves a control point beside a knot of
// a refined surface: that knot stays, and the surface with it.
func TestRemoveKnotsKeepsCreases(t *testing.T) {
	r := rand.New(rand.NewSource(28))
	s := randomSurface(r, 3, 2, 6, 4, true)
	f := refined(s)
	// The point at the first inner joint along u, row 1.
	c := f.DegU
	f.Ctrl[c][1] = f.Ctrl[c][1].Plus(geom.Vector{X: 0.01})
	g := f
	g.RemoveKnots(1e-7)
	if distinctInner(g.DegU, g.KnotsU) != distinctInner(f.DegU, f.KnotsU) {
		t.Errorf("a creased knot was taken out: %v from %v", g.KnotsU, f.KnotsU)
	}
	u0, u1, v0, v1 := f.Domain()
	for range 50 {
		u, v := u0+(u1-u0)*r.Float64(), v0+(v1-v0)*r.Float64()
		if d := g.PointAt(u, v).Minus(f.PointAt(u, v)).Magnitude(); d > 1e-6 {
			t.Errorf("the surface moved %g", d)
		}
	}
}

// TestRemoveKnotsCircle keeps the knots of a cylinder of four quarter
// circles: the rational circle is smooth, but not its homogeneous
// points, so no knot goes.
func TestRemoveKnotsCircle(t *testing.T) {
	w := math.Sqrt(0.5)
	ring := [][2]float64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	s := BSplineSurface{DegU: 2, DegV: 1,
		KnotsU: []float64{0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4},
		KnotsV: []float64{0, 0, 1, 1}}
	for i, p := range ring {
		wt := 1.0
		if i%2 == 1 {
			wt = w
		}
		s.Ctrl = append(s.Ctrl, []geom.Vector{{X: 10 * p[0], Y: 10 * p[1]}, {X: 10 * p[0], Y: 10 * p[1], Z: 5}})
		s.Weight = append(s.Weight, []float64{wt, wt})
	}
	if err := s.Check(); err != nil {
		t.Fatal(err)
	}
	if n := s.RemoveKnots(1e-7); n != 0 {
		t.Errorf("%d knots taken out of a circle: %v", n, s.KnotsU)
	}
	for k := 0; k <= 40; k++ {
		p := s.PointAt(4*float64(k)/40, 0.5)
		if d := math.Abs(math.Hypot(p.X, p.Y) - 10); d > 1e-9 {
			t.Errorf("off the circle by %g", d)
		}
	}
}
