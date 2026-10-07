package srf

import (
	"math/rand"
	"testing"
)

// TestLowDegreeCurve approximates random rational B-spline curves of
// degree 6 to 9 by degree 5: within the tolerance at the same
// parameters, the ends the same, Beziers that kernel code can use.
func TestLowDegreeCurve(t *testing.T) {
	r := rand.New(rand.NewSource(272))
	const tol = 1e-4
	for trial := range 30 {
		deg := 6 + r.Intn(4)
		c := randomCurve(r, deg, deg+1+r.Intn(6), true)
		lc, err := c.LowDegree(tol)
		if err != nil {
			t.Fatalf("trial %d: %v", trial, err)
		}
		if lc.Deg != MaxDegree {
			t.Fatalf("trial %d: degree %d", trial, lc.Deg)
		}
		if _, err := lc.Beziers(); err != nil {
			t.Fatal(err)
		}
		lo, hi := c.Domain()
		for k := range 201 {
			x := lo + (hi-lo)*float64(k)/200
			if d := lc.PointAt(x).Minus(c.PointAt(x)).Magnitude(); d > tol {
				t.Errorf("trial %d: %g off at %g", trial, d, x)
				break
			}
		}
		if !lc.PointAt(lo).Equals(c.PointAt(lo)) || !lc.PointAt(hi).Equals(c.PointAt(hi)) {
			t.Errorf("trial %d: the ends moved", trial)
		}
	}
	// A curve of degree 5 or less is itself.
	c := randomCurve(r, 3, 6, true)
	if lc, _ := c.LowDegree(tol); lc != &c {
		t.Error("a cubic was approximated")
	}
}

// TestLowDegreeSurface approximates random rational B-spline surfaces
// of a degree above 5 one way or both.
func TestLowDegreeSurface(t *testing.T) {
	r := rand.New(rand.NewSource(273))
	const tol = 1e-4
	for trial := range 12 {
		du, dv := 6+r.Intn(3), 2+r.Intn(6)
		if trial%2 == 1 {
			du, dv = dv, du
		}
		s := randomSurface(r, du, dv, du+1+r.Intn(3), dv+1+r.Intn(3), true)
		ls, err := s.LowDegree(tol)
		if err != nil {
			t.Fatalf("trial %d: %v", trial, err)
		}
		if ls.DegU > MaxDegree || ls.DegV > MaxDegree {
			t.Fatalf("trial %d: degree %d by %d", trial, ls.DegU, ls.DegV)
		}
		if _, err := ls.Patches(); err != nil {
			t.Fatal(err)
		}
		u0, u1, v0, v1 := s.Domain()
		for range 400 {
			u, v := u0+(u1-u0)*r.Float64(), v0+(v1-v0)*r.Float64()
			if d := ls.PointAt(u, v).Minus(s.PointAt(u, v)).Magnitude(); d > tol {
				t.Errorf("trial %d (degree %d,%d): %g off at (%g, %g)", trial, du, dv, d, u, v)
				break
			}
		}
	}
}
