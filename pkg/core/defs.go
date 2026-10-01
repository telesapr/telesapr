package core

import "math"

const (
	AngleCosEps  = 1e-6
	LengthEps    = 1e-6
	VeryPositive = 1e10
	VeryNegative = -1e10
	PI           = math.Pi

	MaxPointsInEntity = 12
	MaxUnknowns       = 2048

	RatPolyEps = LengthEps / 1e2

	WrapSymmetric int = 0
	WrapNotZero   int = 1

	MaxBezierDegree = 3
	MaxBezierCpts   = MaxBezierDegree + 1
)
