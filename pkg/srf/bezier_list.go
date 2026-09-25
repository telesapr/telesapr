package srf

import "github.com/spacesolve/spacesolve/pkg/geom"

// SBezierList is an ordered slice of SBezier curves. Used to
// represent an open or unclosed collection of curves (a trim-curve
// chain before it gets closed into a loop).
type SBezierList struct {
	Curves []SBezier
}

func (l *SBezierList) Len() int { return len(l.Curves) }

func (l *SBezierList) Clear() {
	l.Curves = l.Curves[:0]
}

func (l *SBezierList) Add(b SBezier) {
	l.Curves = append(l.Curves, b)
}

// AllIntersectionsWith collects all intersections between this list
// and `other`, appending the intersection points to spl.
func (l *SBezierList) AllIntersectionsWith(other *SBezierList, spl *SPointList) {
	for i := range l.Curves {
		for j := range other.Curves {
			l.Curves[i].AllIntersectionsWith(&other.Curves[j], spl)
		}
	}
}

// CullIdenticalBeziers removes duplicate curves from the list. If
// `both` is true, removes both; otherwise keeps one of each.
func (l *SBezierList) CullIdenticalBeziers(both bool) {
	const tol = 1e-9
	out := l.Curves[:0]
	for i := range l.Curves {
		keep := true
		for j := 0; j < len(out); j++ {
			if beziersAlmostEqual(l.Curves[i], out[j], tol) {
				if !both {
					keep = false
				}
				break
			}
		}
		if keep {
			out = append(out, l.Curves[i])
		}
	}
	l.Curves = out
}

// ScaleSelfBy multiplies every control point by s.
func (l *SBezierList) ScaleSelfBy(s float64) {
	for i := range l.Curves {
		l.Curves[i].ScaleSelfBy(s)
	}
}

// GetBoundingProjd extends the box (min, max) to contain the
// projection of every control point onto the axis through orig in
// direction u.
func (l *SBezierList) GetBoundingProjd(u, orig geom.Vector, pmin, pmax *float64) {
	for i := range l.Curves {
		for j := 0; j <= l.Curves[i].Degree; j++ {
			v := l.Curves[i].Ctrl[j]
			proj := v.Minus(orig).Dot(u)
			if proj < *pmin {
				*pmin = proj
			}
			if proj > *pmax {
				*pmax = proj
			}
		}
	}
}

// beziersAlmostEqual reports whether two Beziers are geometrically
// identical (same degree, same control points, same weights) within
// tol.
func beziersAlmostEqual(a, b SBezier, tol float64) bool {
	if a.Degree != b.Degree {
		return false
	}
	for i := 0; i <= a.Degree; i++ {
		if a.Ctrl[i].DistanceTo(b.Ctrl[i]) > tol {
			return false
		}
		if mathAbs(a.Weight[i]-b.Weight[i]) > tol {
			return false
		}
	}
	return true
}
