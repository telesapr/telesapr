package srf

import "github.com/telesapr/telesapr/pkg/geom"

// SPoint is a single vertex on a planar polygon. Used by SContour
// during ear-clipping triangulation. The Tag field is preserved
// across edge assembly so callers can trace points back to source
// edges.
type SPoint struct {
	Tag int

	Ear EarType

	P    geom.Vector
	AuxV geom.Vector
}

// EarType classifies a vertex for ear-clipping triangulation. An ear
// is a vertex that can be removed as part of ear-clipping; non-ears
// must remain until adjacent ears are clipped.
type EarType uint32

const (
	EarUnknown EarType = 0
	EarNotEar  EarType = 1
	EarIsEar   EarType = 2
)

// SPointList is an ordered list of points, each with a tag (C++
// SPointList).
type SPointList struct {
	Points []SPoint
}

func (l *SPointList) Clear() {
	l.Points = l.Points[:0]
}

// IndexForPoint is the index of the list's point equal to pt, or -1
// (C++ SPointList::IndexForPoint).
func (l *SPointList) IndexForPoint(pt geom.Vector) int {
	for i := range l.Points {
		if pt.Equals(l.Points[i].P) {
			return i
		}
	}
	// Not found, so return negative to indicate that.
	return -1
}

// ContainsPoint reports whether pt is one of the list's points (C++
// SPointList::ContainsPoint).
func (l *SPointList) ContainsPoint(pt geom.Vector) bool {
	return l.IndexForPoint(pt) >= 0
}

// IncrementTagFor counts pt: it increments the tag of the point equal
// to pt, or adds pt with tag 1 (C++ SPointList::IncrementTagFor).
func (l *SPointList) IncrementTagFor(pt geom.Vector) {
	for i := range l.Points {
		if pt.Equals(l.Points[i].P) {
			l.Points[i].Tag++
			return
		}
	}
	l.Points = append(l.Points, SPoint{P: pt, Tag: 1})
}

// Add appends pt with tag 0 (C++ SPointList::Add).
func (l *SPointList) Add(pt geom.Vector) {
	l.Points = append(l.Points, SPoint{P: pt})
}

// removeTagged drops the points for which tagged is true, keeping the
// order of the rest (C++ List::RemoveTagged, after the caller tags).
func (l *SPointList) removeTagged(tagged func(*SPoint) bool) {
	out := l.Points[:0]
	for i := range l.Points {
		if !tagged(&l.Points[i]) {
			out = append(out, l.Points[i])
		}
	}
	l.Points = out
}
