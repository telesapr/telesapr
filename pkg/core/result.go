package core

type SolveResult int

const (
	SolveOkay SolveResult = iota
	SolveDidntConverge
	SolveRedundantOkay
	SolveRedundantDidntConverge
	SolveTooManyUnknowns
)

type PolyError int

const (
	PolyGood PolyError = iota
	PolyNotClosed
	PolyNotCoplanar
	PolySelfIntersecting
	PolyZeroLenEdge
)
