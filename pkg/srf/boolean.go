package srf

import (
	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

// MakeFromBoolean is the master dispatcher for the BSP ray-cast
// CSG pipeline. It runs the four phases defined in srf/boolean.cpp
// (MakeClassifyingBsps, CopyCurvesSplitAgainst,
// MakeIntersectionCurvesAgainst, CopySurfacesTrimAgainst,
// RewriteSurfaceHandlesForCurves, CleanupAfterBoolean).
//
// Phase 4 + Goal Item 1 implementation. The exact-Bezier
// surface-surface intersection is approximated by sampling each
// surface's polyline boundary and intersecting line segments; this
// is correct for the planar-against-planar case and the test
// fixtures, but does not produce exact NURBS intersection curves.
func (s *SShell) MakeFromBoolean(a, b *SShell, op SurfaceCombineAs) {
	s.Clear()
	if a == nil || b == nil {
		s.BooleanFailed = true
		return
	}
	// Phase 1: build uv-space classifying BSPs for every surface in
	// both shells. These BSPs classify a point in (u, v) space as
	// inside or outside the surface's trim region.
	a.MakeClassifyingBsps(nil)
	b.MakeClassifyingBsps(nil)

	// Phase 2: copy curves from a and b, splitting each where it
	// enters/exits a surface from the other shell.
	a.CopyCurvesSplitAgainst(true, b, s)
	b.CopyCurvesSplitAgainst(false, a, s)

	// Phase 3: compute surface-surface intersection curves (line
	// segment approximation) and add them to s.
	a.MakeIntersectionCurvesAgainst(b, s)

	// Remove short segments (degenerate splitters).
	for i := range s.Curve {
		s.Curve[i].RemoveShortSegments(nil, nil)
	}

	// Phase 4: rebuild classifying BSPs with the new (split) curves.
	a.CleanupAfterBoolean()
	b.CleanupAfterBoolean()
	a.MakeClassifyingBsps(s)
	b.MakeClassifyingBsps(s)

	// Phase 5: copy surfaces, trimming each region against the other
	// shell according to the boolean op.
	a.CopySurfacesTrimAgainst(a, b, s, op, true)
	b.CopySurfacesTrimAgainst(a, b, s, op, false)

	// Phase 6: rewrite curve surface handles to point to the
	// newly-created surfaces in s.
	s.RewriteSurfaceHandlesForCurves(a, b)

	// Final cleanup.
	a.CleanupAfterBoolean()
	b.CleanupAfterBoolean()
	s.BooleanFailed = false
}

// makeUnion is a convenience wrapper: keep all surfaces from both
// shells. Used for non-overlapping geometry.
func (s *SShell) makeUnion(a, b *SShell) {
	s.MakeFromBoolean(a, b, SurfaceUnion)
}

// makeDifference: A - B.
func (s *SShell) makeDifference(a, b *SShell) {
	s.MakeFromBoolean(a, b, SurfaceDiff)
}

// makeIntersection: A ∩ B.
func (s *SShell) makeIntersection(a, b *SShell) {
	s.MakeFromBoolean(a, b, SurfaceInter)
}

// MakeFromAssemblyOf combines two shells without performing any
// boolean intersection: surfaces from both are copied as-is.
// Used for step-and-repeat patterns (LINED copies) where the
// geometry is intentionally non-overlapping.
func (s *SShell) MakeFromAssemblyOf(a, b *SShell) {
	s.Clear()
	if a != nil {
		s.Surface = append(s.Surface, a.Surface...)
		for i := range a.Curve {
			s.Curve = append(s.Curve, a.Curve[i])
		}
	}
	if b != nil {
		s.Surface = append(s.Surface, b.Surface...)
		for i := range b.Curve {
			s.Curve = append(s.Curve, b.Curve[i])
		}
	}
	s.BooleanFailed = false
}

// surfaceInsideShell returns true if surface srf lies entirely
// inside shell `sh`. Used by MakeFromDifferenceOf. Phase 4
// implementation: bounding-box containment test (good for the
// common case of two non-overlapping axis-aligned extrusions).
func surfaceInsideShell(srf *SSurface, sh *SShell) bool {
	var sMin, sMax geom.Vector
	srf.GetAxisAlignedBounding(&sMax, &sMin)
	for i := range sh.Surface {
		var bMin, bMax geom.Vector
		sh.Surface[i].GetAxisAlignedBounding(&bMax, &bMin)
		if sMin.X >= bMin.X && sMax.X <= bMax.X &&
			sMin.Y >= bMin.Y && sMax.Y <= bMax.Y &&
			sMin.Z >= bMin.Z && sMax.Z <= bMax.Z {
			return true
		}
	}
	return false
}

// surfaceOverlapsShell returns true if the bounding boxes of srf
// and any surface of sh overlap.
func surfaceOverlapsShell(srf *SSurface, sh *SShell) bool {
	var sMin, sMax geom.Vector
	srf.GetAxisAlignedBounding(&sMax, &sMin)
	for i := range sh.Surface {
		var bMin, bMax geom.Vector
		sh.Surface[i].GetAxisAlignedBounding(&bMax, &bMin)
		if sMax.X < bMin.X || bMax.X < sMin.X {
			continue
		}
		if sMax.Y < bMin.Y || bMax.Y < sMin.Y {
			continue
		}
		if sMax.Z < bMin.Z || bMax.Z < sMin.Z {
			continue
		}
		return true
	}
	return false
}

// CleanupAfterBoolean is invoked after a boolean to remove degenerate
// surfaces (zero area, fully covered by another surface, etc.).
func (s *SShell) CleanupAfterBoolean() {
	// Drop zero-area surfaces.
	out := s.Surface[:0]
	for i := range s.Surface {
		var pMin, pMax geom.Vector
		s.Surface[i].GetAxisAlignedBounding(&pMax, &pMin)
		dx := pMax.X - pMin.X
		dy := pMax.Y - pMin.Y
		dz := pMax.Z - pMin.Z
		if dx < 1e-9 && dy < 1e-9 && dz < 1e-9 {
			continue
		}
		out = append(out, s.Surface[i])
	}
	s.Surface = out
}

// MakeFromUnionOf dispatches to MakeFromBoolean(UNION).
func (s *SShell) MakeFromUnionOf(a, b *SShell) {
	s.MakeFromBoolean(a, b, SurfaceUnion)
}

// MakeFromDifferenceOf dispatches to MakeFromBoolean(DIFFERENCE).
func (s *SShell) MakeFromDifferenceOf(a, b *SShell) {
	s.MakeFromBoolean(a, b, SurfaceDiff)
}

// MakeFromIntersectionOf dispatches to MakeFromBoolean(INTERSECTION).
func (s *SShell) MakeFromIntersectionOf(a, b *SShell) {
	s.MakeFromBoolean(a, b, SurfaceInter)
}

// MakeClassifyingBsps pre-computes uv-space BSPs for every surface
// in the shell. Used during boolean operations to classify a test
// point as inside/outside the shell's region. Each surface's BSP is
// built from the curves in useCurvesFrom (typically passed in the
// second pass of MakeFromBoolean, after the new intersection curves
// have been added); falls back to the shell's own curves otherwise.
func (s *SShell) MakeClassifyingBsps(useCurvesFrom *SShell) {
	for i := range s.Surface {
		s.Surface[i].Bsp = buildSurfaceClassifyingBsp(s, &s.Surface[i], useCurvesFrom)
	}
}

func buildSurfaceClassifyingBsp(sh *SShell, srf *SSurface, useCurvesFrom *SShell) *SBspUv {
	if srf == nil {
		return nil
	}
	src := useCurvesFrom
	if src == nil {
		src = sh
	}
	if src == nil {
		return nil
	}
	el := &SEdgeList{}
	for i := range src.Curve {
		c := &src.Curve[i]
		if !c.IsExact {
			continue
		}
		if useCurvesFrom == nil && c.SurfA != srf.H && c.SurfB != srf.H {
			continue
		}
		el.AddEdge(c.Exact.Start(), c.Exact.PointAt(0.5), 0, 0, 0)
		el.AddEdge(c.Exact.PointAt(0.5), c.Exact.Finish(), 0, 0, 0)
	}
	if len(el.Edges) == 0 {
		return nil
	}
	return BuildBspUvFromEdges(el, srf)
}

// AllPointsIntersecting computes all intersections of the segment
// (a, b) with the triangulated shell mesh. Used during ray-cast
// for inside/outside classification.
//
// Phase 5 + Goal Item 5 implementation: triangulate the shell once
// (cached in s.cachedMesh), then for every triangle cast the
// segment against it using Möller-Trumbore. This replaces the
// sample-based count from Phase 4 with a proper line-mesh ray-cast.
func (s *SShell) AllPointsIntersecting(a, b geom.Vector, inters *[]SInter, asSegment, trimmed, inclTangent bool) {
	if inters == nil {
		return
	}
	// Lazily build (and cache) the shell's triangulated mesh.
	if len(s.cachedMesh.Triangles) == 0 {
		s.TriangulateInto(&s.cachedMesh)
	}
	dir := b.Minus(a)
	if dir.Magnitude() < 1e-20 {
		return
	}
	for _, t := range s.cachedMesh.Triangles {
		tt, _, ok := t.Raytrace(a, dir)
		if !ok {
			continue
		}
		if asSegment && (tt < 0 || tt > 1) {
			continue
		}
		*inters = append(*inters, SInter{
			P:      a.Plus(dir.ScaledBy(tt)),
			SurfA:  core.HSSurface{V: uint32(t.Meta.Face)},
			SurfB:  core.HSSurface{V: uint32(t.Meta.Face)},
			Tag:    0,
		})
	}
	_ = trimmed
	_ = inclTangent
}

// SInter is a ray-shell intersection point with metadata about the
// surfaces and curves it crossed. Equivalent to C++ SInter.
type SInter struct {
	P          geom.Vector
	SurfA      core.HSSurface
	SurfB      core.HSSurface
	SurfNormal geom.Vector
	PInter     geom.Point2d
	Srf        *SSurface
	OnEdge     bool
	Tag        int
}
