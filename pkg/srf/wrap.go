package srf

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/libm"
)

// A sketch in a plane parallel to a cylinder's or a cone's axis is
// wrapped onto it as paper is: lengths in the sketch are lengths on the
// face, and near the line of the face that faces the sketch the map is
// the identity, so the profile seen from the sketch's side looks as it
// was drawn (doc/notes/wrap-emboss.md). On a cylinder a point at s
// across and z along the axis goes to the angle s/r. A cone's sketch is
// laid on its tangent plane along the facing generator, turned there
// about the line across it at the sketch's middle, and rolled on by the
// cone's development. Heights above the face are an affine lift of the
// face's points: on a cylinder a scaling away from the axis, on a cone a
// uniform scaling about the point of the axis on the normal through the
// sketch's middle, which carries the cone onto the parallel one.

// Wrap is the map of one sketch onto a RoundFace.
type Wrap struct {
	Face RoundFace
	// n is the unit direction from the axis towards the sketch, t =
	// Axis × n across.
	n, t geom.Vector
	// A cone's: sin and cos of the half angle, the slant distance of the
	// sketch's middle from the apex, and the centre of the lift and its
	// distance from the face.
	sinA, cosA float64
	rhoC       float64
	q          geom.Vector
	l          float64
}

// wrapFitTol is how far a fitted curve may stray from the true wrapped
// one: a tenth of LengthEps, so that it lies on the face as an exact one
// does.
const wrapFitTol = lengthEps / 10

// wrapFitDepth is how many times a curve is halved to fit it.
const wrapFitDepth = 8

// WrapMaxSpan is the most a loop set (an outer loop and its holes) may
// span round the axis: its caps are one rational quadratic patch each.
const WrapMaxSpan = 170 * math.Pi / 180

// ErrWrapPlane is NewWrap's error for a sketch whose plane is not
// parallel to the face's axis.
var ErrWrapPlane = errors.New("the sketch's plane is not parallel to the face's axis")

// NewWrap is the wrap onto face of the loop sets sets, drawn in the plane
// through planePt with the unit normal planeN.
func NewWrap(face RoundFace, planePt, planeN geom.Vector, sets []SBezierLoopSet) (*Wrap, error) {
	if math.Abs(planeN.Dot(face.Axis)) > 1e-9 {
		return nil, ErrWrapPlane
	}
	w := &Wrap{Face: face}
	// From the axis towards the plane, or along the plane's normal when
	// the plane holds the axis.
	w.n = planeN.WithMagnitude(1)
	if off := planePt.Minus(face.Point).Dot(w.n); off < 0 && math.Abs(off) > lengthEps {
		w.n = w.n.ScaledBy(-1)
	}
	w.t = face.Axis.Cross(w.n)
	if !face.IsCone() {
		return w, nil
	}
	w.sinA, w.cosA = libm.Sin(face.HalfAngle), libm.Cos(face.HalfAngle)
	// The sketch's middle along the axis.
	lo, hi := math.Inf(1), math.Inf(-1)
	for i := range sets {
		for _, l := range sets[i].loops() {
			for _, b := range l.Curves {
				for k := range 9 {
					z := b.PointAt(float64(k) / 8).Minus(face.Point).Dot(face.Axis)
					lo, hi = min(lo, z), max(hi, z)
				}
			}
		}
	}
	if math.IsInf(lo, 0) {
		return nil, errors.New("there is nothing to wrap")
	}
	zc := (lo + hi) / 2
	if zc <= lengthEps {
		return nil, errors.New("the sketch's middle is at or past the cone's apex")
	}
	w.rhoC = zc / w.cosA
	w.l = w.rhoC * w.sinA / w.cosA
	w.q = face.Point.Plus(face.Axis.ScaledBy(w.rhoC / w.cosA))
	return w, nil
}

// coords are p's coordinates in the sketch: across the axis from the
// facing line, and along it.
func (w *Wrap) coords(p geom.Vector) (s, z float64) {
	rel := p.Minus(w.Face.Point)
	return rel.Dot(w.t), rel.Dot(w.Face.Axis)
}

// polar is where p goes on the face: the angle about the axis from n,
// and how far along (along the axis on a cylinder, along the generator
// from the apex on a cone). False for a point past a cone's apex.
func (w *Wrap) polar(p geom.Vector) (theta, along float64, ok bool) {
	s, z := w.coords(p)
	if !w.Face.IsCone() {
		return s / w.Face.Radius, z, true
	}
	// The sketch along the generator from the middle: z - zc, laid at
	// rhoC + that from the apex in the development.
	x := w.rhoC + (z - w.rhoC*w.cosA)
	if x <= lengthEps {
		return 0, 0, false
	}
	return libm.Atan2(s, x) / w.sinA, libm.Hypot(s, x), true
}

// at is the point of the face at the angle theta and along.
func (w *Wrap) at(theta, along float64) geom.Vector {
	radial := w.n.ScaledBy(libm.Cos(theta)).Plus(w.t.ScaledBy(libm.Sin(theta)))
	if !w.Face.IsCone() {
		return w.Face.Point.Plus(w.Face.Axis.ScaledBy(along)).Plus(radial.ScaledBy(w.Face.Radius))
	}
	return w.Face.Point.Plus(w.Face.Axis.ScaledBy(along * w.cosA)).Plus(radial.ScaledBy(along * w.sinA))
}

// onFace is where the sketch's point p goes on the face.
func (w *Wrap) onFace(p geom.Vector) geom.Vector {
	theta, along, _ := w.polar(p)
	return w.at(theta, along)
}

// LiftFactor is the scaling that lifts the face h along its outward
// normal; at or below zero the lift reaches the axis or the centre.
func (w *Wrap) LiftFactor(h float64) float64 {
	if !w.Face.IsCone() {
		return (w.Face.Radius + w.Face.Outward*h) / w.Face.Radius
	}
	return 1 + w.Face.Outward*h/w.l
}

// lift moves the point x of the face (or of a curve or patch on it) to
// h along the face's outward normal; the map is affine, so it carries
// control points.
func (w *Wrap) lift(x geom.Vector, h float64) geom.Vector {
	k := w.LiftFactor(h)
	if !w.Face.IsCone() {
		rel := x.Minus(w.Face.Point)
		radial := rel.Minus(w.Face.Axis.ScaledBy(rel.Dot(w.Face.Axis)))
		return x.Plus(radial.ScaledBy(k - 1))
	}
	return w.q.Plus(x.Minus(w.q).ScaledBy(k))
}

// liftedCurve is b lifted h.
func (w *Wrap) liftedCurve(b SBezier, h float64) SBezier {
	for i := 0; i <= b.Degree; i++ {
		b.Ctrl[i] = w.lift(b.Ctrl[i], h)
	}
	return b
}

// liftedSurface is s lifted h.
func (w *Wrap) liftedSurface(s SSurface, h float64) SSurface {
	for i := 0; i <= s.DegM; i++ {
		for j := 0; j <= s.DegN; j++ {
			s.Ctrl[i][j] = w.lift(s.Ctrl[i][j], h)
		}
	}
	return s
}

// MapCurve is the sketch curve b wrapped onto the face: exact pieces
// where the wrapped curve is a line or a circle, fitted quintics
// elsewhere, each carrying b's Sheet and Entity.
func (w *Wrap) MapCurve(b *SBezier) ([]SBezier, error) {
	a, z := b.Start(), b.Finish()
	if _, _, ok := w.polar(a); !ok {
		return nil, errors.New("the sketch reaches past the cone's apex")
	}
	if b.Degree == 1 {
		if out, ok := w.exactLine(b); ok {
			return out, nil
		}
	}
	for k := range 17 {
		if _, _, ok := w.polar(b.PointAt(float64(k) / 16)); !ok {
			return nil, errors.New("the sketch reaches past the cone's apex")
		}
	}
	f := func(t float64) geom.Vector { return w.onFace(b.PointAt(t)) }
	pieces, ok := fitCurve(f, 0, 1, wrapFitDepth)
	if !ok {
		return nil, fmt.Errorf("a curve from %v to %v can't be wrapped within %g mm", a, z, wrapFitTol)
	}
	for i := range pieces {
		pieces[i].Sheet, pieces[i].Entity = b.Sheet, b.Entity
	}
	return pieces, nil
}

// exactLine wraps the line b exactly where it can: on a cylinder a line
// along the axis stays a line and one across it becomes a circular arc;
// on a cone a line through the developed apex is a generator.
func (w *Wrap) exactLine(b *SBezier) ([]SBezier, bool) {
	p0, p1 := b.Start(), b.Finish()
	th0, al0, _ := w.polar(p0)
	th1, al1, ok := w.polar(p1)
	if !ok {
		return nil, false
	}
	keep := func(c SBezier) SBezier {
		c.Sheet, c.Entity = b.Sheet, b.Entity
		return c
	}
	if math.Abs(th1-th0)*max(w.Face.Radius, al0*w.sinA, al1*w.sinA) < lengthEps/100 {
		return []SBezier{keep(SBezierFrom2(w.at(th0, al0), w.at(th1, al1)))}, true
	}
	if w.Face.IsCone() || math.Abs(al1-al0) > lengthEps/100 {
		return nil, false
	}
	// An arc about the axis, in quarters or less.
	n := int(math.Ceil(math.Abs(th1-th0) / (math.Pi / 2)))
	out := make([]SBezier, 0, n)
	for i := range n {
		ta := th0 + (th1-th0)*float64(i)/float64(n)
		tb := th0 + (th1-th0)*float64(i+1)/float64(n)
		half := (tb - ta) / 2
		c := libm.Cos(half)
		mid := w.n.ScaledBy(libm.Cos(ta + half)).Plus(w.t.ScaledBy(libm.Sin(ta + half)))
		arc := SBezierFrom3(w.at(ta, al0),
			w.Face.Point.Plus(w.Face.Axis.ScaledBy(al0)).Plus(mid.ScaledBy(w.Face.Radius/c)),
			w.at(tb, al0))
		arc.Weight[1] = c
		arc.IsCircleHint = true
		out = append(out, keep(arc))
	}
	return out, true
}

// capPatch is one patch of the face, unlifted, holding the loop set
// sbls once it is wrapped, with a margin all round; an error when the
// set spans more than WrapMaxSpan round the axis or reaches past a
// cone's apex.
func (w *Wrap) capPatch(sbls *SBezierLoopSet) (SSurface, error) {
	thLo, thHi := math.Inf(1), math.Inf(-1)
	alLo, alHi := math.Inf(1), math.Inf(-1)
	for _, l := range sbls.loops() {
		for _, b := range l.Curves {
			for k := range 33 {
				th, al, ok := w.polar(b.PointAt(float64(k) / 32))
				if !ok {
					return SSurface{}, errors.New("the sketch reaches past the cone's apex")
				}
				thLo, thHi = min(thLo, th), max(thHi, th)
				alLo, alHi = min(alLo, al), max(alHi, al)
			}
		}
	}
	if thHi-thLo > WrapMaxSpan {
		return SSurface{}, fmt.Errorf("a loop of the sketch wraps through %.0f° of the face, more than %.0f°: split it",
			(thHi-thLo)*180/math.Pi, WrapMaxSpan*180/math.Pi)
	}
	dth := 0.02*(thHi-thLo) + 0.002
	dal := 0.02*(alHi-alLo) + 1e-3
	thLo, thHi = thLo-dth, thHi+dth
	alLo, alHi = alLo-dal, alHi+dal
	if w.Face.IsCone() {
		alLo = max(alLo, (alLo+dal)/2)
	}
	gen := SBezierFrom2(w.at(0, alLo), w.at(0, alHi))
	return FromRevolutionOf(&gen, w.Face.Point, w.Face.Axis, thLo, thHi, 0, 0), nil
}
