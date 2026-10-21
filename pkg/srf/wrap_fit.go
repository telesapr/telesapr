package srf

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
)

// fitSamples is how many intervals a piece is fitted over, and
// fitChecks how many it is checked at.
const (
	fitSamples = 24
	fitChecks  = 64
)

// fitCurves fits each of fs over [t0, t1] with polynomial Beziers of
// MaxDegree, each within wrapFitTol of its function at the same
// parameter, halving the range up to depth times where one doesn't fit,
// all of them together, so that the i-th pieces of each cover the same
// range. The pieces meet end to end at the functions' own points. False
// when a piece still doesn't fit at the last depth.
func fitCurves(fs []func(t float64) geom.Vector, t0, t1 float64, depth int) ([][]SBezier, bool) {
	out := make([][]SBezier, len(fs))
	fits := true
	for i, f := range fs {
		b, err := fitQuintic(f, t0, t1)
		out[i] = []SBezier{b}
		fits = fits && err <= wrapFitTol
	}
	if fits {
		return out, true
	}
	if depth == 0 {
		return nil, false
	}
	tm := (t0 + t1) / 2
	a, ok := fitCurves(fs, t0, tm, depth-1)
	if !ok {
		return nil, false
	}
	c, ok := fitCurves(fs, tm, t1, depth-1)
	if !ok {
		return nil, false
	}
	for i := range fs {
		out[i] = slices.Concat(a[i], c[i])
	}
	return out, true
}

// fitQuintic is the quintic through f(t0) and f(t1) whose inner control
// points fit f over [t0, t1] in least squares, and the most it strays
// from f at the same parameter.
func fitQuintic(f func(t float64) geom.Vector, t0, t1 float64) (SBezier, float64) {
	const deg = MaxDegree
	at := func(u float64) geom.Vector { return f(t0 + (t1-t0)*u) }
	var b SBezier
	b.Degree = deg
	for i := range deg + 1 {
		b.Weight[i] = 1
	}
	b.Ctrl[0], b.Ctrl[deg] = at(0), at(1)

	// Normal equations for the deg-1 inner points, one system for all
	// three coordinates.
	const n = deg - 1
	var m [n][n]float64
	var rhs [n]geom.Vector
	for k := range fitSamples + 1 {
		u := float64(k) / fitSamples
		var bs [deg + 1]float64
		for i := range deg + 1 {
			bs[i] = bernstein(i, deg, u)
		}
		r := at(u).Minus(b.Ctrl[0].ScaledBy(bs[0])).Minus(b.Ctrl[deg].ScaledBy(bs[deg]))
		for i := range n {
			for j := range n {
				m[i][j] += bs[i+1] * bs[j+1]
			}
			rhs[i] = rhs[i].Plus(r.ScaledBy(bs[i+1]))
		}
	}
	x, ok := solveNormal(m, rhs)
	if !ok {
		return b, math.Inf(1)
	}
	for i := range n {
		b.Ctrl[i+1] = x[i]
	}
	worst := 0.0
	for k := range fitChecks + 1 {
		u := float64(k) / fitChecks
		worst = max(worst, b.PointAt(u).Minus(at(u)).Magnitude())
	}
	return b, worst
}

// solveNormal solves the symmetric system m x = rhs, x a vector per
// unknown, by Gaussian elimination with partial pivoting.
func solveNormal(m [MaxDegree - 1][MaxDegree - 1]float64, rhs [MaxDegree - 1]geom.Vector) ([MaxDegree - 1]geom.Vector, bool) {
	const n = MaxDegree - 1
	var x [n]geom.Vector
	for c := range n {
		p := c
		for r := c + 1; r < n; r++ {
			if math.Abs(m[r][c]) > math.Abs(m[p][c]) {
				p = r
			}
		}
		if math.Abs(m[p][c]) < 1e-14 {
			return x, false
		}
		m[c], m[p] = m[p], m[c]
		rhs[c], rhs[p] = rhs[p], rhs[c]
		for r := c + 1; r < n; r++ {
			k := m[r][c] / m[c][c]
			for j := c; j < n; j++ {
				m[r][j] -= k * m[c][j]
			}
			rhs[r] = rhs[r].Minus(rhs[c].ScaledBy(k))
		}
	}
	for c := n - 1; c >= 0; c-- {
		v := rhs[c]
		for j := c + 1; j < n; j++ {
			v = v.Minus(x[j].ScaledBy(m[c][j]))
		}
		x[c] = v.ScaledBy(1 / m[c][c])
	}
	return x, true
}
