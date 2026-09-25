package srf

import "github.com/spacesolve/spacesolve/pkg/geom"

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

// SPointList is an ordered list of SPoint with tag-based lookup.
type SPointList struct {
	Points []SPoint
	TagCtr int
}

func (l *SPointList) Clear() {
	l.Points = l.Points[:0]
	l.TagCtr = 0
}

func (l *SPointList) Len() int { return len(l.Points) }

func (l *SPointList) AddTag() int {
	l.TagCtr++
	return l.TagCtr
}

func (l *SPointList) Contains(idx int) bool {
	return idx >= 0 && idx < len(l.Points)
}

func (l *SPointList) IndexOf(p geom.Vector) int {
	for i, sp := range l.Points {
		if sp.P.DistanceTo(p) < 1e-12 {
			return i
		}
	}
	return -1
}
