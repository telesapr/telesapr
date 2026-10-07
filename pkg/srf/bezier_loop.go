package srf

import "github.com/telesapr/telesapr/pkg/geom"

// SBezierLoop is an ordered list of SBezier forming one closed
// (or open) loop. The closing point is implicit (last curve's
// Finish == first curve's Start).
type SBezierLoop struct {
	Tag    int
	AuxA   int
	Curves []SBezier
}

func (l *SBezierLoop) Len() int { return len(l.Curves) }

// MakePwlInto emits the polyline approximation of this loop into
// el, at the tolerance tol.
func (l *SBezierLoop) MakePwlInto(el *SEdgeList, tol Tolerance) {
	for i := range l.Curves {
		l.Curves[i].MakePwlInto(el, tol, 0)
	}
}

// MakePwlIntoContour appends the loop's pwl approximation to sc,
// without doubled points at the joins, and closes it exactly. Mirrors
// C++ SBezierLoop::MakePwlInto(SContour *, double) (curve.cpp:468).
func (l *SBezierLoop) MakePwlIntoContour(sc *SContour, tol Tolerance) {
	for i := range l.Curves {
		l.Curves[i].MakePwlIntoContour(sc, tol, 0)
		// Avoid double points at join between Beziers; except that
		// first and last points should be identical.
		if i+1 < len(l.Curves) {
			sc.Points = sc.Points[:len(sc.Points)-1]
		}
	}
	// Ensure that it's exactly closed, not just within a numerical tolerance.
	if n := len(sc.Points); n > 0 && sc.Points[n-1].P.Equals(sc.Points[0].P) {
		sc.Points[n-1] = sc.Points[0]
	}
}

// Reverse reverses the loop order and every curve in it, so that each
// curve still starts where the previous one finishes. Mirrors C++
// SBezierLoop::Reverse.
func (l *SBezierLoop) Reverse() {
	for i, j := 0, len(l.Curves)-1; i < j; i, j = i+1, j-1 {
		l.Curves[i], l.Curves[j] = l.Curves[j], l.Curves[i]
	}
	for i := range l.Curves {
		l.Curves[i].Reverse()
	}
}

// IsClosed reports whether the loop ends where it starts (C++
// SBezierLoop::IsClosed).
func (l *SBezierLoop) IsClosed() bool {
	if len(l.Curves) == 0 {
		return false
	}
	s := l.Curves[0].Start()
	f := l.Curves[len(l.Curves)-1].Finish()
	return s.Equals(f)
}

// BoundingProjd extends the axis-aligned bounding box.
func (l *SBezierLoop) BoundingProjd(u, orig geom.Vector, pmin, pmax *float64) {
	for i := range l.Curves {
		l.Curves[i].BoundingProjd(u, orig, pmin, pmax)
	}
}

// SBezierLoopFromCurves removes curves from sbl, chaining them
// end-to-start (reversing a curve when needed) into one loop. Only
// curves with the same AuxA (style) as the first are joined. Mirrors
// C++ SBezierLoop::FromCurves (curve.cpp).
func SBezierLoopFromCurves(sbl *SBezierList, allClosed *bool, errorAt *SEdge) SBezierLoop {
	var loop SBezierLoop

	if len(sbl.Curves) < 1 {
		*allClosed = false
		return loop
	}

	first := sbl.Curves[0]
	loop.Curves = append(loop.Curves, first)
	start := first.Start()
	hanging := first.Finish()
	auxA := first.AuxA

	sbl.Curves = append(sbl.Curves[:0], sbl.Curves[1:]...)

	for len(sbl.Curves) > 0 && !hanging.Equals(start) {
		foundNext := false
		for i := range sbl.Curves {
			test := &sbl.Curves[i]

			if test.Finish().Equals(hanging) && test.AuxA == auxA {
				test.Reverse()
				// and let the next test catch it
			}
			if test.Start().Equals(hanging) && test.AuxA == auxA {
				loop.Curves = append(loop.Curves, *test)
				hanging = test.Finish()
				sbl.Curves = append(sbl.Curves[:i], sbl.Curves[i+1:]...)
				foundNext = true
				break
			}
		}
		if !foundNext {
			// The loop completed without finding the hanging edge, so
			// it's an open loop
			if errorAt != nil {
				errorAt.A = hanging
				errorAt.B = start
			}
			*allClosed = false
			return loop
		}
	}
	if hanging.Equals(start) {
		*allClosed = true
	} else {
		// We ran out of edges without forming a closed loop.
		if errorAt != nil {
			errorAt.A = hanging
			errorAt.B = start
		}
		*allClosed = false
	}

	return loop
}
