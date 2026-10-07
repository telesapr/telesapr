package srf

// bernsteinCoeff[k][deg] is the polynomial coefficients for
// B_{k,deg}(t) evaluated as ((c[3]*t+c[2])*t+c[1])*t+c[0].
// Matches the table in srf/ratpoly.cpp.
var bernsteinCoeff = [4][4][4]float64{
	{
		{1.0, 0.0, 0.0, 0.0},
		{1.0, 0.0, 0.0, 0.0},
		{1.0, 0.0, 0.0, 0.0},
		{1.0, 0.0, 0.0, 0.0},
	},
	{
		{1.0, -1.0, 0.0, 0.0},
		{0.0, 1.0, 0.0, 0.0},
		{0.0, 0.0, 0.0, 0.0},
		{0.0, 0.0, 0.0, 0.0},
	},
	{
		{1.0, -2.0, 1.0, 0.0},
		{0.0, 2.0, -2.0, 0.0},
		{0.0, 0.0, 1.0, 0.0},
		{0.0, 0.0, 0.0, 0.0},
	},
	{
		{1.0, -3.0, 3.0, -1.0},
		{0.0, 3.0, -6.0, 3.0},
		{0.0, 0.0, 3.0, -3.0},
		{0.0, 0.0, 0.0, 1.0},
	},
}

// bernsteinDerivativeCoeff[k][deg] is the polynomial coefficients
// for B'_{k,deg}(t).
var bernsteinDerivativeCoeff = [4][4][3]float64{
	{
		{0.0, 0.0, 0.0},
		{0.0, 0.0, 0.0},
		{0.0, 0.0, 0.0},
		{0.0, 0.0, 0.0},
	},
	{
		{-1.0, 0.0, 0.0},
		{1.0, 0.0, 0.0},
		{0.0, 0.0, 0.0},
		{0.0, 0.0, 0.0},
	},
	{
		{-2.0, 2.0, 0.0},
		{2.0, -4.0, 0.0},
		{0.0, 2.0, 0.0},
		{0.0, 0.0, 0.0},
	},
	{
		{-3.0, 6.0, -3.0},
		{3.0, -12.0, 9.0},
		{0.0, 6.0, -9.0},
		{0.0, 0.0, 3.0},
	},
}

// bernsteinPoly[deg][k] are the coefficients of B_{k,deg}(t) in
// powers of t, for every degree up to MaxDegree: those of degree up to
// 3 are bernsteinCoeff's, those above C(n,k) C(n-k,m-k) (-1)^(m-k) for
// t^m. bernsteinPolyD are B'_{k,deg}(t)'s.
var bernsteinPoly, bernsteinPolyD = func() (b [MaxDegree + 1][MaxDegree + 1][MaxDegree + 1]float64,
	d [MaxDegree + 1][MaxDegree + 1][MaxDegree]float64) {
	for n := 0; n <= MaxDegree; n++ {
		for k := 0; k <= n; k++ {
			for m := k; m <= n; m++ {
				c := binomial[n][k] * binomial[n-k][m-k]
				if (m-k)%2 == 1 {
					c = -c
				}
				b[n][k][m] = c
			}
			if n <= 3 {
				for m := 0; m <= 3; m++ {
					b[n][k][m] = bernsteinCoeff[n][k][m]
				}
			}
			for m := range MaxDegree {
				d[n][k][m] = float64(m+1) * b[n][k][m+1]
			}
			if n <= 3 {
				for m := 0; m <= 2; m++ {
					d[n][k][m] = bernsteinDerivativeCoeff[n][k][m]
				}
			}
		}
	}
	return b, d
}()

// The Horner forms below are written out for MaxDegree 5; this fails
// to compile for any other.
var _ = [1]struct{}{}[MaxDegree-5]

// bernstein evaluates B_{k,deg}(t), by Horner's rule in powers of t.
// The coefficients above a polynomial's degree are zero, so for degrees
// up to 3 every step adds exactly what the cubic form
// ((c3 t + c2) t + c1) t + c0 does, and the results are those.
func bernstein(k, deg int, t float64) float64 {
	c := &bernsteinPoly[deg][k]
	return ((((c[5]*t+c[4])*t+c[3])*t+c[2])*t+c[1])*t + c[0]
}

// bernsteinDerivative evaluates B'_{k,deg}(t).
func bernsteinDerivative(k, deg int, t float64) float64 {
	c := &bernsteinPolyD[deg][k]
	return (((c[4]*t+c[3])*t+c[2])*t+c[1])*t + c[0]
}

// binomial[n][k] is C(n, k) for n up to MaxDegree.
var binomial = func() (b [MaxDegree + 1][MaxDegree + 1]float64) {
	for n := 0; n <= MaxDegree; n++ {
		b[n][0] = 1
		for k := 1; k <= n; k++ {
			b[n][k] = b[n-1][k-1]
			if k < n {
				b[n][k] += b[n-1][k]
			}
		}
	}
	return b
}()
