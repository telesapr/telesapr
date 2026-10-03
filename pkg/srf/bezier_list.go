package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

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

// AllIntersectionsWith adds to spl the points where any curve of this
// list crosses any of other (C++ SBezierList::AllIntersectionsWith).
func (l *SBezierList) AllIntersectionsWith(other *SBezierList, spl *SPointList) {
	for i := range l.Curves {
		for j := range other.Curves {
			other.Curves[j].AllIntersectionsWith(&l.Curves[i], spl)
		}
	}
}

// CullIdenticalBeziers removes the curves identical to an earlier one,
// either way round, and with both the earlier one too (C++
// SBezierList::CullIdenticalBeziers).
func (l *SBezierList) CullIdenticalBeziers(both bool) {
	for i := range l.Curves {
		l.Curves[i].Tag = 0
	}
	for i := range l.Curves {
		bi := &l.Curves[i]
		bir := *bi
		bir.Reverse()

		for j := i + 1; j < len(l.Curves); j++ {
			bj := &l.Curves[j]
			if bj.Equals(bi) || bj.Equals(&bir) {
				if both {
					bi.Tag = 1
				}
				bj.Tag = 1
			}
		}
	}
	kept := l.Curves[:0]
	for _, sb := range l.Curves {
		if sb.Tag == 0 {
			kept = append(kept, sb)
		}
	}
	l.Curves = kept
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

// GetPlaneContainingBeziers finds a plane that contains all the curves in
// this list. If the curves are all colinear (or coincident, or empty),
// then that plane is not exactly determined but we choose the additional
// degree(s) of freedom arbitrarily. Returns true if all curves are
// coplanar, otherwise false.
func (l *SBezierList) GetPlaneContainingBeziers(p, u, v *geom.Vector, notCoplanarAt *geom.Vector) bool {
	var pt, ptFar, ptOffLine geom.Vector
	if l.Len() > 0 {
		pt = l.Curves[0].Start()
	} else {
		pt = geom.Vector{}
	}
	ptFar = pt
	ptOffLine = pt

	// Get the point farthest from our arbitrary point.
	farMax := -math.MaxFloat64
	for i := range l.Curves {
		b := &l.Curves[i]
		for j := 0; j <= b.Degree; j++ {
			m := pt.Minus(b.Ctrl[j]).Magnitude()
			if m > farMax {
				ptFar = b.Ctrl[j]
				farMax = m
			}
		}
	}
	if ptFar.Equals(pt) {
		// All points coincident.
		*p = pt
		*u = geom.Vector{X: 1}
		*v = geom.Vector{Y: 1}
		return true
	}

	// Get the point farthest from line between pt and ptFar.
	dp := ptFar.Minus(pt)
	offLineMax := -math.MaxFloat64
	for i := range l.Curves {
		b := &l.Curves[i]
		for j := 0; j <= b.Degree; j++ {
			m := b.Ctrl[j].DistanceToLine(pt, dp)
			if m > offLineMax {
				ptOffLine = b.Ctrl[j]
				offLineMax = m
			}
		}
	}

	*p = pt
	if offLineMax < geom.LengthEps {
		// The points are all colinear; so choose the second basis vector
		// arbitrarily.
		*u = ptFar.Minus(pt).WithMagnitude(1)
		*v = u.NormalDir(0).WithMagnitude(1)
	} else {
		// The points actually define a plane.
		n := ptFar.Minus(pt).Cross(ptOffLine.Minus(pt))
		*u = n.NormalDir(0).WithMagnitude(1)
		*v = n.NormalDir(1).WithMagnitude(1)
	}

	// Verify all points are in this plane.
	n := u.Cross(*v).WithMagnitude(1)
	d := p.Dot(n)
	for i := range l.Curves {
		b := &l.Curves[i]
		for j := 0; j <= b.Degree; j++ {
			if math.Abs(n.Dot(b.Ctrl[j])-d) > geom.LengthEps {
				if notCoplanarAt != nil {
					*notCoplanarAt = b.Ctrl[j]
				}
				return false
			}
		}
	}
	return true
}
