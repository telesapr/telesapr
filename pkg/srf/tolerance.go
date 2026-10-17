package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// Tolerance is how closely the piecewise-linear and triangulated forms
// follow the curves and surfaces they are made from: the chord
// tolerance, in mm, and the most segments one curve is cut into. The
// zero value is unset: a shell made with it takes the tolerance of the
// shells it is made from, and one with none at all DefaultTolerance.
type Tolerance struct {
	chordTolMm  float64
	maxSegments int
	mesher      Mesher
	angleDeg    float64
	set         bool
}

// Mesher is how a curved surface's trimmed region is triangulated.
type Mesher uint8

const (
	// MesherEarClip grids the region and ear-clips what the grid leaves
	// along the trims (UvGridTriangulateInto).
	MesherEarClip Mesher = iota
	// MesherCDT fills the region with points sized by the surface's sag
	// and the trim's segments and triangulates them Delaunay, the trim's
	// segments kept as edges (cdtTriangulateInto; roadmap K4).
	MesherCDT
)

// WithMesher is t meshing curved surfaces with m.
func (t Tolerance) WithMesher(m Mesher) Tolerance {
	t = t.resolved()
	t.mesher = m
	return t
}

// WithAngle is t with an angular tolerance of deg degrees: the most a
// curve's tangent or a surface's normal may turn across one piece, 0
// for none. It is applied by the piecewise-linear curves and the surface
// grids, not by the marched intersection curves of surfinter.go, which
// step by chord alone.
func (t Tolerance) WithAngle(deg float64) Tolerance {
	t = t.resolved()
	t.angleDeg = deg
	return t
}

// AngleDeg is the angular tolerance in degrees, 0 for none.
func (t Tolerance) AngleDeg() float64 { return t.resolved().angleDeg }

// angleCos is the cosine of the angular tolerance, and whether there is
// one.
func (t Tolerance) angleCos() (c float64, on bool) {
	t = t.resolved()
	if t.angleDeg <= 0 {
		return 1, false
	}
	return math.Cos(t.angleDeg * math.Pi / 180), true
}

// turnsLittle reports whether the tangent of b at ta, at tb and halfway
// between stays within the angular tolerance of each other. A tangent
// too short to have a direction passes.
func (t Tolerance) turnsLittle(b *SBezier, ta, tb float64) bool {
	c, on := t.angleCos()
	if !on {
		return true
	}
	dir := func(at float64) (geom.Vector, bool) {
		d := b.TangentAt(at)
		m := d.Magnitude()
		if m < lengthEps {
			return d, false
		}
		return d.ScaledBy(1 / m), true
	}
	da, oka := dir(ta)
	dm, okm := dir((ta + tb) / 2)
	db, okb := dir(tb)
	return (!oka || !okm || da.Dot(dm) >= c) &&
		(!okm || !okb || dm.Dot(db) >= c) &&
		(!oka || !okb || da.Dot(db) >= c)
}

// Mesher is how curved surfaces are triangulated at t.
func (t Tolerance) Mesher() Mesher { return t.resolved().mesher }

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

// DefaultTolerance is what an unset tolerance resolves to: 0.1 mm and
// 20 segments, as a new document's display tolerance would be. A
// sketch gives every shell it builds a tolerance of its own, so the
// program never writes this; tests that build shells with none set it
// through SetChordTolMm and SetMaxSegments, and so cannot run in
// parallel.
var DefaultTolerance = Tol(0.1, 20)

// ChordTolMm is DefaultTolerance's chord tolerance in mm.
func ChordTolMm() float64 { return DefaultTolerance.chordTolMm }

// MaxSegments is DefaultTolerance's segment limit.
func MaxSegments() int { return DefaultTolerance.maxSegments }

// SetChordTolMm sets DefaultTolerance's chord tolerance; for tests.
func SetChordTolMm(t float64) { DefaultTolerance = Tol(t, DefaultTolerance.maxSegments) }

// SetMesher sets DefaultTolerance's mesher; for tests.
func SetMesher(m Mesher) { DefaultTolerance = DefaultTolerance.WithMesher(m) }

// SetMaxSegments sets DefaultTolerance's segment limit; for tests.
func SetMaxSegments(n int) { DefaultTolerance = Tol(DefaultTolerance.chordTolMm, n) }

// resolved is t, or DefaultTolerance when t is unset.
func (t Tolerance) resolved() Tolerance {
	if t.set {
		return t
	}
	return DefaultTolerance
}

// SetTolerance gives the shell and its surfaces the tolerance t.
func (s *SShell) SetTolerance(t Tolerance) {
	s.Tol = t
	for i := range s.Surface {
		s.Surface[i].Tol = t
	}
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
