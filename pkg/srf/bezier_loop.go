package srf

import "github.com/spacesolve/spacesolve/pkg/geom"

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
// el, with a chord tolerance of chordTol. Used by SShell's
// edge extraction.
func (l *SBezierLoop) MakePwlInto(el *SEdgeList, chordTol float64) {
	for i := range l.Curves {
		l.Curves[i].MakePwlInto(el, chordTol, 0)
	}
}

// Reverse reverses every curve in the loop and reverses the loop
// order.
func (l *SBezierLoop) Reverse() {
	for i := range l.Curves {
		l.Curves[i].Reverse()
	}
	for i, j := 0, len(l.Curves)-1; i < j; i, j = i+1, j-1 {
		l.Curves[i], l.Curves[j] = l.Curves[j], l.Curves[i]
	}
}

// GetBoundingProjd extends the axis-aligned bounding box.
func (l *SBezierLoop) GetBoundingProjd(u, orig geom.Vector, pmin, pmax *float64) {
	for i := range l.Curves {
		l.Curves[i].GetBoundingProjd(u, orig, pmin, pmax)
	}
}
