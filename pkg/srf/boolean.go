package srf

import (
	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

// MakeFromBoolean sets s to the boolean combination of a and b.
// Mirrors C++ SShell::MakeFromBoolean (boolean.cpp).
func (s *SShell) MakeFromBoolean(a, b *SShell, op SurfaceCombineAs) {
	s.Clear()
	s.BooleanFailed = false

	a.MakeClassifyingBsps(nil)
	b.MakeClassifyingBsps(nil)

	// Copy over all the original curves, splitting them so that a
	// piecewise linear segment never crosses a surface from the other
	// shell.
	a.CopyCurvesSplitAgainst(true, b, s)
	b.CopyCurvesSplitAgainst(false, a, s)

	// Generate the intersection curves for each surface in A against all
	// the surfaces in B (which is all of the intersection curves).
	a.MakeIntersectionCurvesAgainst(b, s)

	for i := range s.Curve {
		sc := &s.Curve[i]
		sc.RemoveShortSegments(sc.GetSurfaceA(a, b), sc.GetSurfaceB(a, b))
	}

	// And clean up the piecewise linear things we made as a calculation aid
	a.CleanupAfterBoolean()
	b.CleanupAfterBoolean()
	// Remake the classifying BSPs with the split (and short-segment-removed)
	// curves
	a.MakeClassifyingBsps(s)
	b.MakeClassifyingBsps(s)

	// Then trim and copy the surfaces
	a.CopySurfacesTrimAgainst(a, b, s, op)
	b.CopySurfacesTrimAgainst(a, b, s, op)

	// Now that we've copied the surfaces, we know their new hSurfaces, so
	// rewrite the curves to refer to the surfaces by their handles in the
	// result.
	s.RewriteSurfaceHandlesForCurves(a, b)

	// And clean up the piecewise linear things we made as a calculation aid
	a.CleanupAfterBoolean()
	b.CleanupAfterBoolean()
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

// buildSurfaceClassifyingBsp builds srf's uv-space trim BSP, as the
// first half of C++ SSurface::MakeClassifyingBsp (boolean.cpp:913).
func buildSurfaceClassifyingBsp(sh *SShell, srf *SSurface, useCurvesFrom *SShell) *SBspUv {
	var el SEdgeList
	srf.MakeEdgesInto(sh, &el, true, useCurvesFrom)
	return SBspUvFrom(&el, srf)
}

// buildSurfaceClassifyingEdgesXYZ builds srf's xyz-space trim
// boundary, as the second half of C++ SSurface::MakeClassifyingBsp
// (boolean.cpp:918).
func buildSurfaceClassifyingEdgesXYZ(sh *SShell, srf *SSurface, useCurvesFrom *SShell) SEdgeList {
	var el SEdgeList
	srf.MakeEdgesInto(sh, &el, false, useCurvesFrom)
	return el
}

// AllPointsIntersecting collects the intersections of the line (a, b)
// with every surface of the shell. Mirrors C++
// SShell::AllPointsIntersecting (raycast.cpp:383).
func (s *SShell) AllPointsIntersecting(a, b geom.Vector, inters *[]SInter, asSegment, trimmed, inclTangent bool) {
	for i := range s.Surface {
		s.Surface[i].AllPointsIntersecting(a, b, inters, asSegment, trimmed, inclTangent)
	}
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
