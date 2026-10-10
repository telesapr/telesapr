package export

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// biarcMaxDepth bounds the halving of a curve that biarcs don't follow:
// a span of 1/4096 of the curve is cut as a straight line.
const biarcMaxDepth = 12

// biarcSamples is how many points inside a span its biarc is measured
// against.
const biarcSamples = 16

// fitBiarcs passes to add the curve sb, which lies in the xy plane, as
// circular arcs, two to a span, each pair meeting tangentially and
// tangent to the curve at the span's ends, so that the cuts join
// smoothly. A span whose biarc strays from the curve by more than tol
// at any of its samples is halved; nearly straight arcs are lines.
func fitBiarcs(sb *srf.SBezier, tol float64, add func(gcodeMove)) {
	fitBiarcSpan(sb, 0, 1, tol, 0, add)
}

func fitBiarcSpan(sb *srf.SBezier, t0, t1, tol float64, depth int, add func(gcodeMove)) {
	p0, p1 := sb.PointAt(t0), sb.PointAt(t1)
	if depth >= biarcMaxDepth {
		add(gcodeMove{a: p0, b: p1})
		return
	}
	if m, n, ok := biarc(p0, curveTangent(sb, t0, t1, false), p1, curveTangent(sb, t0, t1, true)); ok {
		fits := true
		for k := 1; k < biarcSamples && fits; k++ {
			p := sb.PointAt(t0 + (t1-t0)*float64(k)/biarcSamples)
			fits = min(m.distanceTo(p), n.distanceTo(p)) <= tol
		}
		if fits {
			add(m)
			add(n)
			return
		}
	}
	tm := (t0 + t1) / 2
	fitBiarcSpan(sb, t0, tm, tol, depth+1, add)
	fitBiarcSpan(sb, tm, t1, tol, depth+1, add)
}

// curveTangent is the unit direction of the curve at the span's start,
// or at its end when atEnd. Where the derivative vanishes, as at a
// cubic whose first two control points coincide, the direction is
// taken from a point a little way into the span.
func curveTangent(sb *srf.SBezier, t0, t1 float64, atEnd bool) geom.Vector {
	t := t0
	if atEnd {
		t = t1
	}
	d := sb.TangentAt(t)
	d.Z = 0
	if d.Magnitude() > 1e-9 {
		return d.WithMagnitude(1)
	}
	h := (t1 - t0) / 1024
	var chord geom.Vector
	if atEnd {
		chord = sb.PointAt(t1).Minus(sb.PointAt(t1 - h))
	} else {
		chord = sb.PointAt(t0 + h).Minus(sb.PointAt(t0))
	}
	chord.Z = 0
	if chord.Magnitude() < 1e-12 {
		return geom.Vector{}
	}
	return chord.WithMagnitude(1)
}

// biarc is the pair of arcs from p0, leaving along the unit tangent d0,
// to p1, arriving along d1, that meet tangentially at a point as far
// along each arc's tangent from the arcs' far ends (the equal-length
// biarc). It fails where the two ends meet, or no such pair turns the
// right way.
func biarc(p0, d0, p1, d1 geom.Vector) (m, n gcodeMove, ok bool) {
	v := p1.Minus(p0)
	vv := v.Dot(v)
	if vv < geom.LengthEps*geom.LengthEps || d0.Magnitude() == 0 || d1.Magnitude() == 0 {
		return m, n, false
	}
	// The tangent lengths d from p0 along d0 and back from p1 along d1
	// are equal, and the two control points are 2d apart:
	// |v - d(d0+d1)|² = 4d², a quadratic in d.
	vt := v.Dot(d0.Plus(d1))
	den := 2 * (1 - d0.Dot(d1))
	var d float64
	if den < 1e-12 {
		// Parallel tangents: the quadratic term vanishes.
		vd := v.Dot(d1)
		if vd <= 0 {
			return m, n, false
		}
		d = vv / (4 * vd)
	} else {
		d = (-vt + math.Sqrt(vt*vt+den*vv)) / den
	}
	if !(d > 0) || math.IsInf(d, 0) {
		return m, n, false
	}
	q0 := p0.Plus(d0.ScaledBy(d))
	q1 := p1.Minus(d1.ScaledBy(d))
	j := q0.Plus(q1).ScaledBy(0.5)
	m = arcFrom(p0, d0, j)
	// The second arc, run backwards from p1, leaves along -d1.
	n = arcFrom(p1, d1.ScaledBy(-1), j).reversed()
	return m, n, true
}

// arcFrom is the arc that leaves a along the unit tangent d and ends at
// b, or the line from a to b where that arc would be straight within
// LengthEps.
func arcFrom(a, d, b geom.Vector) gcodeMove {
	w := b.Minus(a)
	ww := w.Dot(w)
	left := geom.Vector{X: -d.Y, Y: d.X}
	s := left.Dot(w)
	// The arc's height over its chord is ww/(8|r|) at most for the
	// shorter arc, with r = ww/(2s).
	if math.Abs(s)/4 < geom.LengthEps {
		return gcodeMove{a: a, b: b}
	}
	r := ww / (2 * s)
	return gcodeMove{a: a, b: b, arc: true, c: a.Plus(left.ScaledBy(r)), ccw: s > 0}
}

// distanceTo is how far p lies from the cut.
func (m gcodeMove) distanceTo(p geom.Vector) float64 {
	if !m.arc {
		ab := m.b.Minus(m.a)
		t := 0.0
		if l := ab.Dot(ab); l > 0 {
			t = max(0, min(1, p.Minus(m.a).Dot(ab)/l))
		}
		return p.DistanceTo(m.a.Plus(ab.ScaledBy(t)))
	}
	r := m.a.Minus(m.c).Magnitude()
	if m.sweepTo(p) <= m.sweepTo(m.b) {
		return math.Abs(p.Minus(m.c).Magnitude() - r)
	}
	return min(p.Minus(m.a).Magnitude(), p.Minus(m.b).Magnitude())
}

// sweepTo is the angle, in [0, 2π), that the arc's direction of turning
// takes from its start to the direction of p from its centre.
func (m gcodeMove) sweepTo(p geom.Vector) float64 {
	u, v := m.a.Minus(m.c), p.Minus(m.c)
	ang := math.Atan2(u.X*v.Y-u.Y*v.X, u.X*v.X+u.Y*v.Y)
	if !m.ccw {
		ang = -ang
	}
	if ang < 0 {
		ang += 2 * math.Pi
	}
	return ang
}
