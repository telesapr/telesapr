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

// bernstein evaluates B_{k,deg}(t).
func bernstein(k, deg int, t float64) float64 {
	c := bernsteinCoeff[deg][k]
	return ((c[3]*t+c[2])*t+c[1])*t + c[0]
}

// bernsteinDerivative evaluates B'_{k,deg}(t).
func bernsteinDerivative(k, deg int, t float64) float64 {
	c := bernsteinDerivativeCoeff[deg][k]
	return ((c[2]*t)+c[1])*t + c[0]
}
