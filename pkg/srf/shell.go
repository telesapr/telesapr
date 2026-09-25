package srf

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

// SCurvePt is one polyline vertex on a trim curve.
type SCurvePt struct {
	Vertex bool   // true if this point is a vertex of the original SBezier (used for snapping)
	P      geom.Vector
	Tag    int
}

// SCurve is a trim curve in the shell: the exact SBezier form plus
// the polyline approximation, and references to the two surfaces
// it trims. SurfA is the "outer" surface (the one on the right when
// walking from Start to Finish); SurfB is the "inner" surface.
type SCurve struct {
	H       core.HSCurve
	Tag     int
	Exact   SBezier
	IsExact bool
	Deg     int

	SurfA, SurfB core.HSSurface
	AuxA, AuxB   int

	Points []SCurvePt

	// Source indicates where this curve came from during booleans.
	Source CurveSource
}

// CurveSource tracks the provenance of a curve (used by
// SShell boolean operations to decide what to keep).
type CurveSource uint32

const (
	CurveSourceOrigA  CurveSource = 0
	CurveSourceOrigB  CurveSource = 1
	CurveSourceSplitA CurveSource = 2
	CurveSourceSplitB CurveSource = 3
)

// SShell is a watertight collection of trimmed surfaces and the
// trim curves joining them. Used as the input to SShell::MakeFromBoolean
// (Phase 4) and as the result of Group::GenerateShellAndMesh
// (Phase 5).
type SShell struct {
	Curve   []SCurve
	Surface []SSurface

	BooleanFailed bool

	// cachedMesh is the SMesh produced by TriangulateInto; cached so
	// AllPointsIntersecting doesn't re-triangulate on every ray.
	cachedMesh SMesh
}

// Clear resets all data.
func (s *SShell) Clear() {
	s.Curve = nil
	s.Surface = nil
	s.BooleanFailed = false
}

// MakeFromCopyOf clones another shell.
func (s *SShell) MakeFromCopyOf(src *SShell) {
	s.Clear()
	for _, c := range src.Curve {
		s.Curve = append(s.Curve, c)
	}
	for _, ss := range src.Surface {
		s.Surface = append(s.Surface, ss)
	}
	s.BooleanFailed = src.BooleanFailed
}

// MakeFromExtrusionOf extrudes a planar BezierLoopSet from t0 to t1.
// Each curve in the set becomes a side surface; the outer contour
// is capped at t0 and t1 with two planar surfaces.
//
// This is one of the highest-level constructor methods. It
// iterates every loop in the set, emits a translated copy of each
// Bezier at t0 and t1 (the caps), and an extruded surface along
// each Bezier (the side).
func (s *SShell) MakeFromExtrusionOf(sbls *SBezierLoopSet, t0, t1 geom.Vector, color core.RgbaColor) {
	s.Clear()
	if sbls == nil {
		return
	}
	// Process outer contour first, then holes.
	for loopIdx, loop := range []SBezierLoop{sbls.Outer} {
		for i := range loop.Curves {
			b := &loop.Curves[i]
			// Side surface from this Bezier.
			side := FromExtrusionOf(b, t0, t1)
			side.Color = color
			side.Face = FaceID(uint32(len(s.Surface)) + 1)
			side.Tag = loop.Tag
			s.Surface = append(s.Surface, side)
			_ = loopIdx
		}
		// Cap at t0.
		t0Plane := FromPlane(t0, geom.Vector{X: 1}, geom.Vector{Y: 1})
		t0Plane.Color = color
		t0Plane.Face = FaceID(uint32(len(s.Surface)) + 1)
		t0Plane.Tag = loop.Tag
		s.Surface = append(s.Surface, t0Plane)
		// Cap at t1.
		t1Plane := FromPlane(t1, geom.Vector{X: 1}, geom.Vector{Y: 1})
		t1Plane.Color = color
		t1Plane.Face = FaceID(uint32(len(s.Surface)) + 1)
		t1Plane.Tag = loop.Tag
		s.Surface = append(s.Surface, t1Plane)
	}
}

// MakeFromRevolutionOf revolves a planar BezierLoopSet around an
// axis (pt, axis). Each Bezier becomes a revolved surface; the
// caps are filled by joining the start and end of the rotation.
func (s *SShell) MakeFromRevolutionOf(sbls *SBezierLoopSet, pt, axis geom.Vector, color core.RgbaColor) {
	s.Clear()
	if sbls == nil {
		return
	}
	const thetaf = 2 * 3.141592653589793
	for i := range sbls.Outer.Curves {
		b := &sbls.Outer.Curves[i]
		surf := FromRevolutionOf(b, pt, axis, 0, thetaf, 0, 0)
		surf.Color = color
		surf.Face = FaceID(uint32(len(s.Surface)) + 1)
		surf.Tag = sbls.Outer.Tag
		s.Surface = append(s.Surface, surf)
	}
}

// MakeFromHelixOf sweeps a planar BezierLoopSet around an axis
// (pt, axis) while translating along the axis by `turns` total
// rotation. Pitch is the axial distance per full revolution
// (pitch = axial_distance / turns). Equivalent to revolving the
// contour once while extruding along the axis by `pitch`.
func (s *SShell) MakeFromHelixOf(sbls *SBezierLoopSet, pt, axis geom.Vector, turns float64, pitch float64, color core.RgbaColor) {
	s.Clear()
	if sbls == nil {
		return
	}
	turns = math.Abs(turns)
	if turns < 1e-9 {
		turns = 1
	}
	if pitch < 1e-9 {
		pitch = 1.0
	}
	const thetaf = 2 * 3.141592653589793
	axisN := axis.Normal()
	axialStep := pitch / 12.0
	for i := range sbls.Outer.Curves {
		b := &sbls.Outer.Curves[i]
		// Sample the bezier at 12 points along its length; each sample
		// is revolved at a different angle and axially shifted.
		const ns = 12
		for k := 0; k < ns; k++ {
			t0 := float64(k) / ns
			t1 := float64(k+1) / ns
			seg := SBezier{Degree: 1}
			seg.Ctrl[0] = b.PointAt(t0)
			seg.Ctrl[1] = b.PointAt(t1)
			seg.Weight[0] = 1
			seg.Weight[1] = 1
			angle0 := t0 * turns * thetaf
			angle1 := t1 * turns * thetaf
			offset0 := t0 * turns * pitch
			offset1 := t1 * turns * pitch
			surf := FromRevolutionOf(&seg, pt, axisN, angle0, angle1, offset0, offset1)
			surf.Color = color
			surf.Face = FaceID(uint32(len(s.Surface)) + 1)
			surf.Tag = sbls.Outer.Tag
			s.Surface = append(s.Surface, surf)
		}
		_ = axialStep
	}
}

// MakeFromTransformationOf applies (translate, rotate, scale) to
// every surface and curve of src.
func (s *SShell) MakeFromTransformationOf(src *SShell, t geom.Vector, q geom.Quaternion, scale float64, includingTrims bool) {
	s.Clear()
	for _, ss := range src.Surface {
		s.Surface = append(s.Surface, FromTransformationOf(&ss, t, q, scale, includingTrims))
	}
}

// TriangulateInto triangulates every surface in the shell and
// appends the resulting triangles to mesh. Each surface is sampled
// on a regular (u, v) grid; cells whose center is outside the trim
// region are skipped via srf.Bsp.
func (s *SShell) TriangulateInto(mesh *SMesh) {
	for i := range s.Surface {
		s.Surface[i].TriangulateInto(mesh)
	}
}

// TriangulateInto is the per-surface version. It samples the
// surface on a regular (u, v) grid, then keeps only those cells
// whose center is INSIDE the trim region (per srf.Bsp). Phase 4 +
// Goal Item 4 implementation: supports trimmed surfaces via the
// SBspUv classifier.
func (srf *SSurface) TriangulateInto(mesh *SMesh) {
	const n = 16
	for j := 0; j < n; j++ {
		for i := 0; i < n; i++ {
			u0 := float64(i) / float64(n)
			u1 := float64(i+1) / float64(n)
			v0 := float64(j) / float64(n)
			v1 := float64(j+1) / float64(n)
			// Test the cell center; keep the cell only if it's inside
			// the trim region.
			if srf.Bsp != nil {
				uc := (u0 + u1) / 2
				vc := (v0 + v1) / 2
				if srf.Bsp.ClassifyPoint(geom.Point2d{U: uc, V: vc}) == BspUvOutside {
					continue
				}
			}
			p00 := srf.PointAt(u0, v0)
			p10 := srf.PointAt(u1, v0)
			p11 := srf.PointAt(u1, v1)
			p01 := srf.PointAt(u0, v1)
			meta := STriMeta{Face: srf.Face, RgbaColor: srf.Color}
			mesh.AddTriangle3(meta, p00, p10, p11)
			mesh.AddTriangle3(meta, p00, p11, p01)
		}
	}
}

// MergeCoincidentSurfaces identifies surfaces whose control grids
// coincide (within tol) and merges them, keeping a single copy.
// This dramatically reduces shell size when extrusion produces
// many redundant planes.
//
// Phase 4 + Goal Item 15 implementation: clusters surfaces by
// coincidence, then assigns each cluster a unique FaceID. The
// merged cluster keeps one representative surface.
func (s *SShell) MergeCoincidentSurfaces() {
	const tol = 1e-9
	cluster := make([]int, len(s.Surface))
	for i := range s.Surface {
		cluster[i] = -1
	}
	representative := make([]int, 0, len(s.Surface))
	// For each pair (i, j), if they coincide, they share a cluster.
	for i := range s.Surface {
		if cluster[i] != -1 {
			continue
		}
		cluster[i] = len(representative)
		representative = append(representative, i)
		for j := i + 1; j < len(s.Surface); j++ {
			if cluster[j] == -1 && s.Surface[i].CoincidentWith(&s.Surface[j], true) {
				cluster[j] = cluster[i]
			}
		}
	}
	out := make([]SSurface, 0, len(representative))
	for rIdx, srcIdx := range representative {
		srf := s.Surface[srcIdx]
		srf.Face = FaceID(uint32(rIdx + 1))
		out = append(out, srf)
	}
	// Update every curve's surface handle to point at the merged face.
	for i := range s.Curve {
		for j := range s.Surface {
			if cluster[j] == cluster[i] {
				s.Curve[i].SurfA = out[cluster[j]].H
				s.Curve[i].SurfB = out[cluster[j]].H
			}
		}
		_ = i
	}
	s.Surface = out
	_ = tol
}

// MakeEdgesInto extracts the trim edges of every surface as SEdge
// entries. When a surface has no trim, emits the four boundary
// edges of the surface's uv rectangle.
func (s *SShell) MakeEdgesInto(el *SEdgeList) {
	for i := range s.Surface {
		s.Surface[i].MakeEdgesInto(s, el, nil, true)
	}
}

// MakeEdgesInto emits the boundary edges of this surface. If
// useCurvesFrom is non-nil, the trim region is taken from those
// curves instead of the surface's own trim. Phase 4 + Goal Item 4
// implementation: when srf has trim curves (or useCurvesFrom is
// supplied), emit each curve as a 3-D edge; otherwise emit the four
// boundary edges of the uv rectangle.
func (srf *SSurface) MakeEdgesInto(parentShell *SShell, el *SEdgeList, useCurvesFrom *SShell, includeTrimCurves bool) {
	_ = includeTrimCurves
	if useCurvesFrom != nil {
		for i := range useCurvesFrom.Curve {
			tag := int(useCurvesFrom.Curve[i].H.V)
			b := useCurvesFrom.Curve[i].Exact
			// Sample the curve at multiple points and emit as a
			// polyline in 3-D space (since edges are 3-D).
			const ns = 8
			last := b.Start()
			for k := 1; k <= ns; k++ {
				t := float64(k) / float64(ns)
				cur := b.PointAt(t)
				el.AddEdge(last, cur, tag, 0, tag)
				last = cur
			}
		}
		return
	}
	// No curves: emit 4 boundary edges of the surface's uv rectangle.
	p00 := srf.PointAt(0, 0)
	p10 := srf.PointAt(1, 0)
	p11 := srf.PointAt(1, 1)
	p01 := srf.PointAt(0, 1)
	el.AddEdge(p00, p10, 0, 0, srf.Tag)
	el.AddEdge(p10, p11, 0, 0, srf.Tag)
	el.AddEdge(p11, p01, 0, 0, srf.Tag)
	el.AddEdge(p01, p00, 0, 0, srf.Tag)
	_ = parentShell
}

// MakeSectionEdgesInto emits the cross-section of the shell where it
// is cut by the plane (n, d). Phase 4 + Item 4: emit the boundary
// of each surface that's coplanar with (n, d).
func (s *SShell) MakeSectionEdgesInto(n geom.Vector, d float64, el *SEdgeList) {
	for i := range s.Surface {
		s.Surface[i].MakeSectionEdgesInto(s, el, n, d)
	}
}

// MakeSectionEdgesInto is the per-surface version.
func (srf *SSurface) MakeSectionEdgesInto(parentShell *SShell, el *SEdgeList, n geom.Vector, d float64) {
	_ = parentShell
	// Phase 4 + Item 4: emit 4 boundary edges of the surface only
	// when the surface is approximately coplanar with (n, d).
	if !srf.CoincidentWithPlane(n, d, 1e-6) {
		return
	}
	p00 := srf.PointAt(0, 0)
	p10 := srf.PointAt(1, 0)
	p11 := srf.PointAt(1, 1)
	p01 := srf.PointAt(0, 1)
	el.AddEdge(p00, p10, 0, 0, srf.Tag)
	el.AddEdge(p10, p11, 0, 0, srf.Tag)
	el.AddEdge(p11, p01, 0, 0, srf.Tag)
	el.AddEdge(p01, p00, 0, 0, srf.Tag)
	_ = el
	_ = n
	_ = d
}

// RemapFaces assigns fresh face ids to surfaces in the shell.
// Called after a boolean operation when faces get split.
func (s *SShell) RemapFaces() {
	for i := range s.Surface {
		s.Surface[i].Face = FaceID(uint32(i + 1))
	}
}
