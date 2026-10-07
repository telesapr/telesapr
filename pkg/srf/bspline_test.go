package srf

import (
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// randomKnots is a knot vector for n control points of degree deg:
// clamped or not, with interior knots spaced unevenly and some
// repeated.
func randomKnots(r *rand.Rand, deg, n int, clamped bool) []float64 {
	k := make([]float64, n+deg+1)
	t := 0.0
	for i := range k {
		switch {
		case clamped && i <= deg:
			k[i] = 0
		case clamped && i >= n:
			k[i] = k[n]
			if i == n {
				t += 0.2 + r.Float64()
				k[i] = t
			}
		default:
			// Repeat a knot now and then, up to the degree's times.
			if i == 0 || r.Intn(4) != 0 || i >= deg && k[i-deg] == t {
				t += 0.2 + r.Float64()
			}
			k[i] = t
		}
	}
	return k
}

func randomCurve(r *rand.Rand, deg, n int, clamped bool) BSplineCurve {
	c := BSplineCurve{Deg: deg, Knots: randomKnots(r, deg, n, clamped)}
	for range n {
		c.Ctrl = append(c.Ctrl, geom.Vector{X: 10 * r.Float64(), Y: 10 * r.Float64(), Z: 10 * r.Float64()})
		c.Weight = append(c.Weight, 0.5+r.Float64())
	}
	return c
}

func randomSurface(r *rand.Rand, du, dv, nu, nv int, clamped bool) BSplineSurface {
	s := BSplineSurface{DegU: du, DegV: dv, KnotsU: randomKnots(r, du, nu, clamped), KnotsV: randomKnots(r, dv, nv, clamped)}
	for i := range nu {
		var row []geom.Vector
		var w []float64
		for j := range nv {
			row = append(row, geom.Vector{X: float64(i) + r.Float64(), Y: float64(j) + r.Float64(), Z: 3 * r.Float64()})
			w = append(w, 0.5+r.Float64())
		}
		s.Ctrl = append(s.Ctrl, row)
		s.Weight = append(s.Weight, w)
	}
	return s
}

// spanOf is the span of breaks holding t and t's place in it, 0 to 1.
func spanOf(bs []float64, t float64) (int, float64) {
	for i := 0; i+1 < len(bs); i++ {
		if t <= bs[i+1] || i+2 == len(bs) {
			return i, (t - bs[i]) / (bs[i+1] - bs[i])
		}
	}
	return 0, 0
}

func TestBSplineCurveBeziers(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	for _, clamped := range []bool{true, false} {
		for deg := 1; deg <= MaxDegree; deg++ {
			for n := deg + 1; n <= deg+5; n += 2 {
				c := randomCurve(r, deg, n, clamped)
				if err := c.Check(); err != nil {
					t.Fatalf("degree %d, %d points: %v (knots %v)", deg, n, err, c.Knots)
				}
				bz, err := c.Beziers()
				if err != nil {
					t.Fatal(err)
				}
				bs := c.Breaks()
				if len(bz) != len(bs)-1 {
					t.Fatalf("%d pieces for %d breaks", len(bz), len(bs))
				}
				lo, hi := c.Domain()
				for k := 0; k <= 40; k++ {
					u := lo + (hi-lo)*float64(k)/40
					i, s := spanOf(bs, u)
					if d := bz[i].PointAt(s).Minus(c.PointAt(u)).Magnitude(); d > 1e-9 {
						t.Errorf("clamped %v degree %d, %d points: piece %d off by %g at %v", clamped, deg, n, i, d, u)
					}
				}
				for i := 1; i < len(bz); i++ {
					if !bz[i-1].Finish().Equals(bz[i].Start()) {
						t.Errorf("pieces %d and %d don't meet", i-1, i)
					}
				}
			}
		}
	}
}

// TestBSplineCircle is the circle of radius 2 as a rational quadratic
// B-spline of nine points: four pieces, each a quarter on the circle.
func TestBSplineCircle(t *testing.T) {
	w := math.Sqrt2 / 2
	c := BSplineCurve{
		Deg:   2,
		Knots: []float64{0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4},
		Ctrl: []geom.Vector{{X: 2}, {X: 2, Y: 2}, {Y: 2}, {X: -2, Y: 2}, {X: -2}, {X: -2, Y: -2},
			{Y: -2}, {X: 2, Y: -2}, {X: 2}},
		Weight: []float64{1, w, 1, w, 1, w, 1, w, 1},
	}
	bz, err := c.Beziers()
	if err != nil || len(bz) != 4 {
		t.Fatalf("%d pieces, %v", len(bz), err)
	}
	for k := 0; k <= 100; k++ {
		u := 4 * float64(k) / 100
		if d := c.PointAt(u).Magnitude(); math.Abs(d-2) > 1e-12 {
			t.Errorf("at %v the circle is %v from its centre", u, d)
		}
	}
	for _, b := range bz {
		var ax, ctr geom.Vector
		var rad float64
		if !b.IsCircle(geom.Vector{Z: 1}, &ctr, &rad) || math.Abs(rad-2) > 1e-12 {
			t.Errorf("a piece is not an arc of the circle: %v %v", ctr, rad)
		}
		_ = ax
	}
}

func TestBSplineSurfacePatches(t *testing.T) {
	r := rand.New(rand.NewSource(4))
	for _, deg := range [][2]int{{1, 1}, {2, 3}, {3, 3}, {MaxDegree, 2}, {4, MaxDegree}} {
		for _, clamped := range []bool{true, false} {
			s := randomSurface(r, deg[0], deg[1], deg[0]+3, deg[1]+2, clamped)
			ps, err := s.Patches()
			if err != nil {
				t.Fatal(err)
			}
			us, vs := s.Breaks()
			if len(ps) != len(us)-1 || len(ps[0]) != len(vs)-1 {
				t.Fatalf("%dx%d patches for %d x %d breaks", len(ps), len(ps[0]), len(us), len(vs))
			}
			u0, u1, v0, v1 := s.Domain()
			for range 60 {
				u := u0 + (u1-u0)*r.Float64()
				v := v0 + (v1-v0)*r.Float64()
				a, x := spanOf(us, u)
				b, y := spanOf(vs, v)
				p := &ps[a][b]
				if p.DegM != deg[0] || p.DegN != deg[1] {
					t.Fatalf("a patch of degree %d by %d", p.DegM, p.DegN)
				}
				if d := p.PointAt(x, y).Minus(s.PointAt(u, v)).Magnitude(); d > 1e-9 {
					t.Errorf("degree %v clamped %v: patch %d,%d off by %g at (%v, %v)", deg, clamped, a, b, d, u, v)
				}
			}
			// Neighbouring patches share their edges.
			for a := range ps {
				for b := range ps[a] {
					if a+1 < len(ps) {
						e, f := ps[a][b].EdgeCurve(1), ps[a+1][b].EdgeCurve(3)
						if !e.Equals(&f) {
							t.Errorf("patches %d,%d and %d,%d don't share their edge", a, b, a+1, b)
						}
					}
					if b+1 < len(ps[a]) {
						e, f := ps[a][b].EdgeCurve(2), ps[a][b+1].EdgeCurve(0)
						if !e.Equals(&f) {
							t.Errorf("patches %d,%d and %d,%d don't share their edge", a, b, a, b+1)
						}
					}
				}
			}
		}
	}
}

func TestBSplineErrors(t *testing.T) {
	r := rand.New(rand.NewSource(5))
	good := randomSurface(r, 3, 3, 5, 5, true)
	high := randomSurface(r, MaxDegree+1, 2, MaxDegree+2, 3, true)
	if err := high.Check(); err != nil {
		t.Fatalf("a surface of degree %d: %v", MaxDegree+1, err)
	}
	if _, err := high.Patches(); err == nil || !strings.Contains(err.Error(), "patches go up to") {
		t.Errorf("degree %d cut into patches: %v", MaxDegree+1, err)
	}
	// It evaluates all the same.
	_ = high.PointAt(0.3, 0.3)
	cases := map[string]func(s *BSplineSurface){
		"knots don't fit": func(s *BSplineSurface) { s.KnotsU = s.KnotsU[1:] },
		"decrease":        func(s *BSplineSurface) { s.KnotsV[5], s.KnotsV[4] = s.KnotsV[4], s.KnotsV[5]+1 },
		"weight":          func(s *BSplineSurface) { s.Weight[1][1] = 0 },
		"row 2":           func(s *BSplineSurface) { s.Ctrl[2] = s.Ctrl[2][1:] },
		"no range":        func(s *BSplineSurface) { s.KnotsU = []float64{0, 0, 0, 1, 1, 1, 1, 2, 2} },
		"too few":         func(s *BSplineSurface) { s.DegU = 5; s.KnotsU = append(s.KnotsU, 1, 1) },
		"not a number":    func(s *BSplineSurface) { s.KnotsV[0] = math.NaN() },
	}
	for want, spoil := range cases {
		s := good
		s.KnotsU = append([]float64(nil), good.KnotsU...)
		s.KnotsV = append([]float64(nil), good.KnotsV...)
		s.Ctrl = make([][]geom.Vector, len(good.Ctrl))
		s.Weight = make([][]float64, len(good.Weight))
		for i := range good.Ctrl {
			s.Ctrl[i] = append([]geom.Vector(nil), good.Ctrl[i]...)
			s.Weight[i] = append([]float64(nil), good.Weight[i]...)
		}
		spoil(&s)
		if err := s.Check(); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", want, err)
		}
		if _, err := s.Patches(); err == nil {
			t.Errorf("%s: cut into patches", want)
		}
	}
}
