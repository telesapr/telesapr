package srf

// Tolerance is how closely the piecewise-linear and triangulated forms
// follow the curves and surfaces they are made from: the chord
// tolerance, in mm, and the most segments one curve is cut into. The
// zero value is unset: a shell made with it takes the tolerance of the
// shells it is made from, and one with none at all the process-wide
// setting (SetChordTolMm, SetMaxSegments).
type Tolerance struct {
	chordTolMm  float64
	maxSegments int
	set         bool
}

// Tol is the tolerance of chordTolMm and maxSegments. A chord
// tolerance of 0 leaves the segment limit as the only bound.
func Tol(chordTolMm float64, maxSegments int) Tolerance {
	return Tolerance{chordTolMm: chordTolMm, maxSegments: maxSegments, set: true}
}

// IsSet reports whether t was made with Tol.
func (t Tolerance) IsSet() bool { return t.set }

// Or is t if it is set, and o otherwise.
func (t Tolerance) Or(o Tolerance) Tolerance {
	if t.set {
		return t
	}
	return o
}

// resolved is t, or the process-wide setting when t is unset.
func (t Tolerance) resolved() Tolerance {
	if t.set {
		return t
	}
	return Tol(ChordTolMm(), MaxSegments())
}

// ChordTolMm is the chord tolerance in mm.
func (t Tolerance) ChordTolMm() float64 { return t.resolved().chordTolMm }

// MaxSegments is the most segments one curve is cut into.
func (t Tolerance) MaxSegments() int { return t.resolved().maxSegments }

// tolOf is the tolerance of a surface, which may be nil.
func tolOf(s *SSurface) Tolerance {
	if s == nil {
		return Tolerance{}
	}
	return s.Tol
}
