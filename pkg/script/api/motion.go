package api

// MotionFrame is a step of a motion study: the parameter's value, the
// file its frame went to, if any, whether every group solved, and the
// volume of the solid up to the active group.
type MotionFrame struct {
	Step   int
	Value  float64
	File   string
	Solved bool
	Volume float64
}
