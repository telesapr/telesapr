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

	// Phase 7: merge coincident surfaces that result from per-surface
	// trim (e.g. boolean_coplanar_union's stacked cuboids share side
	// faces that need to fuse into single shells — see PORTING.md
	// §"Boolean-CSG parity gaps"). MergeCoincidentSurfaces reassigns
	// unique face ids and rewrites curve surface handles to point at
	// the merged cluster representatives.
	s.MergeCoincidentSurfaces()

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
// surfaces (zero area, fully covered by another surface, etc.) and
// to clear the temporary `Edges` calculation aid that
// `MakeClassifyingBsps` populates (matches C++
// `SShell::CleanupAfterBoolean`, boolean.cpp:780).
func (s *SShell) CleanupAfterBoolean() {
	for i := range s.Surface {
		s.Surface[i].Edges = SEdgeList{}
	}
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
//
// PORTING.md §"Remaining work" item 5: a real bsp classifies a point
// against this surface's own *trim* boundary (the STrimBy loop from
// the surface's originating profile) — it is never split by a
// cross-shell intersection curve, because that curve isn't a trim
// edge of this (unmodified) operand surface; it only marks where the
// *other* shell touches it, and is a trim edge of the freshly-copied
// surface built later in the per-surface trim pass
// (makeCopyTrimAgainst's own origBsp). The Go port doesn't populate
// SSurface.Trim for surfaces produced by extrusion (a separate,
// larger gap — see PORTING.md), so there is no real boundary data to
// build a bsp from here: buildSurfaceClassifyingBsp always returns
// nil for these fixtures, and ClassifyEdge's case-3 nil-bsp default
// (below) supplies the correct semantic instead of a synthesized
// one — see that default's doc comment.
//
// (An earlier version of this function tried to approximate a bsp by
// splitting the surface's uv-rectangle along the cross-shell
// intersection curves that reference it. That produces an
// *unclosed* partition — it has no representation of the surface's
// actual outer trim boundary, only the internal split line — and
// empirically made classification worse, not better: it dropped
// boolean_knife_edge's ratio from 0.76 to as low as 0.26 while only
// inconsistently helping the tangent_* fixtures, because the
// SurfA/SurfB "which side is which" convention it relied on isn't
// well-defined for every intersection branch (plane-plane vs
// extrusion-extrusion vs marching-squares). Reverted in favor of the
// case-3 default fix.)
func (s *SShell) MakeClassifyingBsps(useCurvesFrom *SShell) {
	for i := range s.Surface {
		srf := &s.Surface[i]
		srf.Bsp = buildSurfaceClassifyingBsp(s, srf, useCurvesFrom)
		// Mirrors the second half of C++ `SSurface::MakeClassifyingBsp`
		// (boolean.cpp:909-918): besides the uv-space bsp, it also
		// rebuilds `edges` as the *xyz* trim boundary of this surface.
		// `SShell::ClassifyEdge`'s edge-on-edge (case 1) and knife-edge
		// (case 2) branches read `srf.Edges` to detect when the probe
		// point lies exactly on another surface's trim boundary; with
		// `Edges` always empty those branches could never fire and
		// every such probe fell through to the raycast fallback
		// (case 4), which is far less precise for the axis-aligned
		// coincident-edge geometry the boolean fixtures are full of.
		srf.Edges = buildSurfaceClassifyingEdgesXYZ(s, srf, useCurvesFrom)
	}
}

func buildSurfaceClassifyingBsp(sh *SShell, srf *SSurface, useCurvesFrom *SShell) *SBspUv {
	if srf == nil || len(srf.Trim) == 0 {
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
	for i := range srf.Trim {
		stb := &srf.Trim[i]
		c := src.FindCurve(stb.Curve)
		if c == nil {
			continue
		}
		addCurveUvEdgesForBsp(el, srf, c, stb.Backwards)
	}
	if len(el.Edges) == 0 {
		return nil
	}
	return BuildBspUvFromEdges(el, srf)
}

// buildSurfaceClassifyingEdgesXYZ builds the xyz-space trim boundary
// of srf from the same STrimBy list buildSurfaceClassifyingBsp uses,
// but keeps the curve samples in 3-D instead of projecting them to
// (u, v). Mirrors the `MakeEdgesInto(shell, &edges, MakeAs::XYZ,
// useCurvesFrom)` call in C++ `SSurface::MakeClassifyingBsp`
// (boolean.cpp:918).
func buildSurfaceClassifyingEdgesXYZ(sh *SShell, srf *SSurface, useCurvesFrom *SShell) SEdgeList {
	var el SEdgeList
	if srf == nil || len(srf.Trim) == 0 {
		return el
	}
	src := useCurvesFrom
	if src == nil {
		src = sh
	}
	if src == nil {
		return el
	}
	for i := range srf.Trim {
		stb := &srf.Trim[i]
		c := src.FindCurve(stb.Curve)
		if c == nil {
			continue
		}
		addCurveXyzEdgesForClassify(&el, c, stb.Backwards)
	}
	return el
}

// addCurveXyzEdgesForClassify appends curve c's polyline (or, absent
// polyline points, samples of its exact form) to el as raw xyz
// edges, tagged with the curve's handle so ClassifyEdge's edge-on-
// edge matching (`se.A.Equals(...)`) can identify which curve an
// edge came from. reverse walks Finish->Start, matching
// STrimBy.Backwards.
func addCurveXyzEdgesForClassify(el *SEdgeList, c *SCurve, reverse bool) {
	pts := make([]geom.Vector, 0, 3)
	if len(c.Points) >= 2 {
		for i := range c.Points {
			pts = append(pts, c.Points[i].P)
		}
	} else {
		pts = append(pts, c.Exact.Start(), c.Exact.PointAt(0.5), c.Exact.Finish())
	}
	if reverse {
		for i, j := 0, len(pts)-1; i < j; i, j = i+1, j-1 {
			pts[i], pts[j] = pts[j], pts[i]
		}
	}
	tag := int(c.H.V)
	for i := 1; i < len(pts); i++ {
		el.AddEdge(pts[i-1], pts[i], tag, 0, tag)
	}
}

// addCurveUvEdgesForBsp projects curve c's polyline (or, absent
// polyline points, samples of its exact form) into srf's (u, v)
// space and appends the resulting edges to el, matching the
// `MakeAs::UV` branch of the C++ SSurface::MakeTrimEdgesInto: the
// stored edge endpoints are `{u, v, 0}`, not the curve's raw xyz
// samples, so the BSP's classifyPointUv operates in the same (u, v)
// space as the ClassifyPoint / ClassifyEdge callers. reverse walks
// the curve Finish->Start instead, matching STrimBy.Backwards.
func addCurveUvEdgesForBsp(el *SEdgeList, srf *SSurface, c *SCurve, reverse bool) {
	toUV := func(p geom.Vector) geom.Vector {
		var puv geom.Point2d
		srf.ClosestPointTo(p, &puv, false)
		return geom.Vector{X: puv.U, Y: puv.V}
	}
	pts := make([]geom.Vector, 0, 3)
	if len(c.Points) >= 2 {
		for i := range c.Points {
			pts = append(pts, toUV(c.Points[i].P))
		}
	} else {
		pts = append(pts, toUV(c.Exact.Start()), toUV(c.Exact.PointAt(0.5)), toUV(c.Exact.Finish()))
	}
	if reverse {
		for i, j := 0, len(pts)-1; i < j; i, j = i+1, j-1 {
			pts[i], pts[j] = pts[j], pts[i]
		}
	}
	for i := 1; i < len(pts); i++ {
		el.AddEdge(pts[i-1], pts[i], 0, 0, 0)
	}
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
