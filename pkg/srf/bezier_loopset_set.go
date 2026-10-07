package srf

import "github.com/telesapr/telesapr/pkg/geom"

// SBezierLoopSetSet is the top-level container for an entire sketch:
// a list of SBezierLoopSet (one per outer contour) plus an optional
// list of open paths.
type SBezierLoopSetSet struct {
	Sets []SBezierLoopSet
}

func (s *SBezierLoopSetSet) Clear() {
	s.Sets = s.Sets[:0]
}

// AddOuter appends a new empty SBezierLoopSet and returns a pointer
// to it.
func (s *SBezierLoopSetSet) AddOuter() *SBezierLoopSet {
	s.Sets = append(s.Sets, SBezierLoopSet{})
	return &s.Sets[len(s.Sets)-1]
}

// AddOpenPath adds a set holding a loop of just b (C++
// SBezierLoopSetSet::AddOpenPath).
func (s *SBezierLoopSetSet) AddOpenPath(b SBezier) {
	s.Sets = append(s.Sets, SBezierLoopSet{Outer: SBezierLoop{Curves: []SBezier{b}}})
}

// FindOuterFacesFrom assembles the curves of sbl into closed loops and
// groups each outer loop with its holes into one SBezierLoopSet. The
// pwl of every loop goes into spxyz. If srfuv is nil, the curves must
// all be coplanar and a plane through them is used. Leftover and open
// loops go to openContours (when non-nil). Mirrors C++
// SBezierLoopSetSet::FindOuterFacesFrom (curve.cpp:575).
func (s *SBezierLoopSetSet) FindOuterFacesFrom(sbl *SBezierList, spxyz *SPolygon,
	srfuv *SSurface, tol Tolerance,
	allClosed *bool, notClosedAt *SEdge,
	allCoplanar *bool, notCoplanarAt *geom.Vector,
	openContours *[]SBezierLoop) {

	var srfPlane SSurface
	if srfuv == nil {
		var p, u, v geom.Vector
		*allCoplanar = sbl.PlaneContainingBeziers(&p, &u, &v, notCoplanarAt)
		if !*allCoplanar {
			// Don't even try to assemble them into loops if they're not
			// all coplanar.
			if openContours != nil {
				for _, sb := range sbl.Curves {
					*openContours = append(*openContours, SBezierLoop{Curves: []SBezier{sb}})
				}
			}
			return
		}
		// All the curves lie in a plane through p with basis vectors u and v.
		srfPlane = FromPlane(p, u, v)
		srfuv = &srfPlane
	}

	// Assemble the Bezier trim curves into closed loops; we also get the
	// piecewise linearization of the curves (in the SPolygon spxyz), as a
	// calculation aid for the loop direction.
	sbls, _, _ := sBezierLoopSetFrom(sbl, spxyz, tol, allClosed, notClosedAt, openContours)
	if len(sbls) != len(spxyz.Contours) {
		return
	}

	// Convert the xyz piecewise linear to uv piecewise linear.
	var spuv SPolygon
	for i := range spxyz.Contours {
		spuv.AddEmptyContour()
		for _, pt := range spxyz.Contours[i].Points {
			var u, v float64
			srfuv.ClosestPointToUV(pt.P, &u, &v, true)
			spuv.Contours[len(spuv.Contours)-1].AddPoint(geom.Vector{X: u, Y: v})
		}
	}
	spuv.Normal = geom.Vector{Z: 1} // must be, since it's in xy plane now

	const (
		outerLoop = 10
		innerLoop = 20
		usedLoop  = 30
	)
	// Fix the contour directions; we do this properly, in uv space, so it
	// works for curved surfaces too (important for STEP export).
	spuv.FixContourDirections()
	for i := range spuv.Contours {
		contour := &spuv.Contours[i]
		bl := &sbls[i]
		if contour.Tag != 0 {
			// This contour got reversed in the polygon to make the directions
			// consistent, so the same must be necessary for the Bezier loop.
			bl.Reverse()
		}
		if contour.IsClockwiseProjdToNormal(spuv.Normal) {
			bl.Tag = innerLoop
		} else {
			bl.Tag = outerLoop
		}
	}

	loopsRemaining := true
	for loopsRemaining {
		loopsRemaining = false
		for i := range sbls {
			loop := &sbls[i]
			if loop.Tag != outerLoop {
				continue
			}

			// Check if this contour contains any outer loops; if it does, then
			// we should do those "inner outer loops" first; otherwise we
			// will steal their holes, since their holes also lie inside this
			// contour.
			j := 0
			for ; j < len(sbls); j++ {
				outer := &sbls[j]
				if i == j {
					continue
				}
				if outer.Tag != outerLoop {
					continue
				}

				p := spuv.Contours[j].AnyEdgeMidpoint()
				if spuv.Contours[i].ContainsPointProjdToNormal(spuv.Normal, p) {
					break
				}
			}
			if j < len(sbls) {
				// It does, can't do this one yet.
				continue
			}

			loopsRemaining = true
			loop.Tag = usedLoop
			outerAndInners := SBezierLoopSet{Outer: *loop}
			auxA := 0
			if len(loop.Curves) > 0 {
				auxA = loop.Curves[0].AuxA
			}

			for j := range sbls {
				inner := &sbls[j]
				if inner.Tag != innerLoop {
					continue
				}
				if len(inner.Curves) < 1 {
					continue
				}
				if inner.Curves[0].AuxA != auxA {
					continue
				}

				p := spuv.Contours[j].AnyEdgeMidpoint()
				if spuv.Contours[i].ContainsPointProjdToNormal(spuv.Normal, p) {
					outerAndInners.Holes = append(outerAndInners.Holes, *inner)
					inner.Tag = usedLoop
				}
			}

			outerAndInners.Point = srfuv.PointAt(0, 0)
			outerAndInners.Normal = srfuv.NormalAt(0, 0)
			s.Sets = append(s.Sets, outerAndInners)
		}
	}

	// If we have poorly-formed loops--for example, overlapping zero-area
	// stuff--then we can end up with leftovers. We use this function to
	// group stuff into closed paths for export when possible, so it's bad
	// to screw up on that stuff. So just add them onto the open curve list.
	// Very ugly, but better than losing curves.
	for i := range sbls {
		if sbls[i].Tag == usedLoop {
			continue
		}
		if openContours != nil {
			*openContours = append(*openContours, sbls[i])
		}
	}
}
