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

// bernstein evaluates B_{k,deg}(t): from the tables up to degree 3,
// so those results stay what they were, and as C(deg, k) t^k
// (1-t)^(deg-k) above.
func bernstein(k, deg int, t float64) float64 {
	if deg > 3 {
		return bernsteinHigh(k, deg, t)
	}
	c := bernsteinCoeff[deg][k]
	return ((c[3]*t+c[2])*t+c[1])*t + c[0]
}

// bernsteinDerivative evaluates B'_{k,deg}(t).
func bernsteinDerivative(k, deg int, t float64) float64 {
	if deg > 3 {
		// deg (B_{k-1,deg-1} - B_{k,deg-1}), a term outside 0..deg-1
		// being 0.
		d := 0.0
		if k > 0 {
			d += bernsteinHigh(k-1, deg-1, t)
		}
		if k < deg {
			d -= bernsteinHigh(k, deg-1, t)
		}
		return float64(deg) * d
	}
	c := bernsteinDerivativeCoeff[deg][k]
	return ((c[2]*t)+c[1])*t + c[0]
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

func bernsteinHigh(k, deg int, t float64) float64 {
	r := binomial[deg][k]
	s := 1 - t
	for i := 0; i < k; i++ {
		r *= t
	}
	for i := k; i < deg; i++ {
		r *= s
	}
	return r
}
